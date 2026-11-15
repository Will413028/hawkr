// Package postops 是代行貼文的 use case：代行 session 以代行店家的身分建立、修改、發布、下架、排程與讀取貼文。每個寫入在同一個 transaction 內先寫單段的
// SUCCEEDED 稽核（冪等鍵是 request_id），再呼叫 social；social 失敗時稽核一併 rollback。
package postops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Audit 是單段稽核（audit.Module.RunOperation）。
type Audit interface {
	RunOperation(ctx context.Context, in model.OperationAuditInput, do func(ctx context.Context) (model.OperationOutcome, error)) (model.AuditLog, bool, error)
}

// Social 是代行貼文需要的 social 公開 API（*social.Module 滿足它）；店家由 delegation 以 session 的目標店家讀出後交給 social。
type Social interface {
	CreatePost(ctx context.Context, store seller.Store, in social.CreatePostInput) (social.Post, error)
	UpdatePost(ctx context.Context, store seller.Store, in social.UpdatePostInput) (social.Post, error)
	PublishPost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) (social.Post, error)
	UnpublishPost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) (social.Post, error)
	SchedulePost(ctx context.Context, store seller.Store, in social.SchedulePostInput) (social.ScheduledPost, error)
	MyPost(ctx context.Context, store seller.Store, id uuid.UUID) (social.Post, error)
	ListMyPosts(ctx context.Context, store seller.Store, status social.PostStatus, after *social.OwnPostCursor, size int32) (social.OwnPostPage, error)
	PendingSchedule(ctx context.Context, storeID int64, postID uuid.UUID) (social.Schedule, error)
}

// Deps 是 use case 的依賴。Store 讀代行店家此刻的狀態：店家不存在或不可代行時回拒絕（delegation 的 DelegationDenial）。
type Deps struct {
	Audit  Audit
	Social Social
	Store  func(ctx context.Context, storeID int64) (seller.Store, error)
}

type Module struct{ d Deps }

func New(d Deps) *Module { return &Module{d: d} }

// Caller 是發起請求的代行：Session 是傳輸層以憑證解出的 session；CorrelationID 與 UserAgent 寫進稽核列。
type Caller struct {
	Session       model.DelegationSession
	CorrelationID string
	UserAgent     string
}

// Operation 是每個代行寫入共同的稽核資料。
type Operation struct {
	// RequestID 是 request_id（AIP-155），同一個 session 內去重。
	RequestID uuid.UUID
	// Reason 是操作原因，去除首尾空白後 1–100 字，不得含控制字元。
	Reason       string
	NotifySeller bool
}

type CreatePostInput struct {
	Op         Operation
	Body       string
	ImageToken string
	ProductID  *int64
	Hashtags   []string
}

type UpdatePostInput struct {
	Op     Operation
	Update social.UpdatePostInput
}

// PostTransitionInput 是發布與下架：貼文與讀取時的版本。
type PostTransitionInput struct {
	Op      Operation
	PostID  uuid.UUID
	Version int64
}

type SchedulePostInput struct {
	Op        Operation
	PostID    uuid.UUID
	Version   int64
	PublishAt time.Time
}

// PostResult 是寫入的結果：貼文（重送時是此刻的樣子）、稽核列 ID、是否是重送，排程時另有排定的發布時間。
type PostResult struct {
	Post social.Post
	// Store 是代行店家（交給回應的組裝）。
	Store      seller.Store
	AuditLogID uuid.UUID
	Replayed   bool
	PublishAt  *time.Time
}

// PostPageResult 是一頁貼文與代行店家。
type PostPageResult struct {
	Page  social.OwnPostPage
	Store seller.Store
}

// Gate 檢查 session 此刻還能用（ACTIVE）並帶有能力 c：傳輸層的 interceptor 與每個 use case 都呼叫它（use case 是自己的前置條件，模組的其他呼叫端不能繞過）。
func Gate(s model.DelegationSession, c model.Capability) error {
	switch s.Status {
	case model.DelegationRevoked:
		return model.Deny(model.DenialSessionRevoked, "代行工作階段已被撤銷")
	case model.DelegationExpired:
		return model.Deny(model.DenialSessionExpired, "代行工作階段已逾時")
	case model.DelegationActive:
	}
	if !slices.Contains(s.Capabilities, c) {
		return model.Deny(model.DenialCapabilityDenied, "此工作階段未取得執行該操作的能力")
	}
	return nil
}

func hashOf(parts ...any) string {
	raw, _ := json.Marshal(parts)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func preview(body string) string {
	if r := []rune(strings.TrimSpace(body)); len(r) > 60 {
		return string(r[:60])
	}
	return strings.TrimSpace(body)
}

type spec struct {
	action  model.AuditAction
	cap     model.Capability
	op      Operation
	hash    string
	postID  string
	version *int64
	name    string
	do      func(ctx context.Context, st seller.Store) (social.Post, *time.Time, error)
}

func (m *Module) run(ctx context.Context, c Caller, sp spec) (PostResult, error) {
	if err := Gate(c.Session, sp.cap); err != nil {
		return PostResult{}, err
	}
	st, err := m.d.Store(ctx, c.Session.Target.StoreID)
	if err != nil {
		return PostResult{}, err
	}
	var (
		post      social.Post
		publishAt *time.Time
	)
	log, replayed, err := m.d.Audit.RunOperation(ctx, model.OperationAuditInput{
		SessionID: c.Session.ID, Capability: sp.cap, Action: sp.action, ResourceType: model.ResourcePost,
		ResourceID: sp.postID, ResourceName: sp.name, ResourceVersionBefore: sp.version,
		OperationReason: strings.TrimSpace(sp.op.Reason), NotifySeller: sp.op.NotifySeller,
		RequestID: sp.op.RequestID.String(), CorrelationID: c.CorrelationID, UserAgent: c.UserAgent, PayloadHash: sp.hash,
	}, func(ctx context.Context) (model.OperationOutcome, error) {
		p, at, err := sp.do(ctx, st)
		if err != nil {
			return model.OperationOutcome{}, err
		}
		post, publishAt = p, at
		return model.OperationOutcome{ResourceID: p.ID.String(), ResourceName: preview(p.Body), VersionAfter: p.Version}, nil
	})
	if err != nil {
		return PostResult{}, err
	}
	if replayed {
		if log.ResourceID == nil {
			return PostResult{}, fmt.Errorf("delegation：重送的稽核列 %s 沒有資源 ID", log.ID)
		}
		id, err := uuid.Parse(*log.ResourceID)
		if err != nil {
			return PostResult{}, fmt.Errorf("delegation：稽核列 %s 的資源 ID %q：%w", log.ID, *log.ResourceID, err)
		}
		if post, err = m.d.Social.MyPost(ctx, st, id); err != nil {
			return PostResult{}, err
		}
		if sp.action == model.AuditPostSchedule {
			if s, err := m.d.Social.PendingSchedule(ctx, st.StoreID, id); err == nil {
				at := s.PublishAt
				publishAt = &at
			} else if apperr.KindOf(err) != apperr.KindNotFound {
				return PostResult{}, err
			}
		}
	}
	return PostResult{Post: post, Store: st, AuditLogID: log.ID, Replayed: replayed, PublishAt: publishAt}, nil
}

// CreatePost 以代行店家建立草稿貼文（能力 posts.write，稽核 POST_CREATE）。
func (m *Module) CreatePost(ctx context.Context, c Caller, in CreatePostInput) (PostResult, error) {
	return m.run(ctx, c, spec{
		action: model.AuditPostCreate, cap: model.CapabilityPostsWrite, op: in.Op, name: preview(in.Body),
		hash: hashOf(model.AuditPostCreate, in.Body, in.ImageToken, in.ProductID, in.Hashtags),
		do: func(ctx context.Context, st seller.Store) (social.Post, *time.Time, error) {
			p, err := m.d.Social.CreatePost(ctx, st, social.CreatePostInput{Body: in.Body, ImageToken: in.ImageToken, ProductID: in.ProductID, Hashtags: in.Hashtags})
			return p, nil, err
		},
	})
}

// UpdatePost 修改貼文（能力 posts.write，稽核 POST_UPDATE）；變更的欄位以鎖住的列為準，在 social 內合併。
func (m *Module) UpdatePost(ctx context.Context, c Caller, in UpdatePostInput) (PostResult, error) {
	u := in.Update
	return m.run(ctx, c, spec{
		action: model.AuditPostUpdate, cap: model.CapabilityPostsWrite, op: in.Op, postID: u.PostID.String(), version: &u.Version,
		hash: hashOf(model.AuditPostUpdate, u.PostID, u.Version, u.Body, u.ImageToken, u.ClearImage, u.SetProduct, u.ProductID, u.Hashtags),
		do: func(ctx context.Context, st seller.Store) (social.Post, *time.Time, error) {
			p, err := m.d.Social.UpdatePost(ctx, st, u)
			return p, nil, err
		},
	})
}

// PublishPost 立即發布（能力 posts.publish，稽核 POST_PUBLISH）。
func (m *Module) PublishPost(ctx context.Context, c Caller, in PostTransitionInput) (PostResult, error) {
	return m.run(ctx, c, spec{
		action: model.AuditPostPublish, cap: model.CapabilityPostsPublish, op: in.Op, postID: in.PostID.String(), version: &in.Version,
		hash: hashOf(model.AuditPostPublish, in.PostID, in.Version),
		do: func(ctx context.Context, st seller.Store) (social.Post, *time.Time, error) {
			p, err := m.d.Social.PublishPost(ctx, st, in.PostID, in.Version)
			return p, nil, err
		},
	})
}

// UnpublishPost 下架（能力 posts.publish，稽核 POST_UNPUBLISH）。
func (m *Module) UnpublishPost(ctx context.Context, c Caller, in PostTransitionInput) (PostResult, error) {
	return m.run(ctx, c, spec{
		action: model.AuditPostUnpublish, cap: model.CapabilityPostsPublish, op: in.Op, postID: in.PostID.String(), version: &in.Version,
		hash: hashOf(model.AuditPostUnpublish, in.PostID, in.Version),
		do: func(ctx context.Context, st seller.Store) (social.Post, *time.Time, error) {
			p, err := m.d.Social.UnpublishPost(ctx, st, in.PostID, in.Version)
			return p, nil, err
		},
	})
}

// SchedulePost 排程發布（能力 posts.publish，稽核一律先寫 POST_SCHEDULE：到點發布的稽核繼承它的理由與通知設定）；發起者種類 DELEGATION，ref 是 session ID。
func (m *Module) SchedulePost(ctx context.Context, c Caller, in SchedulePostInput) (PostResult, error) {
	by := social.ScheduledBy{Kind: social.ScheduleByDelegation, Ref: strconv.FormatInt(c.Session.ID, 10)}
	return m.run(ctx, c, spec{
		action: model.AuditPostSchedule, cap: model.CapabilityPostsPublish, op: in.Op, postID: in.PostID.String(), version: &in.Version,
		hash: hashOf(model.AuditPostSchedule, in.PostID, in.Version, in.PublishAt.UTC()),
		do: func(ctx context.Context, st seller.Store) (social.Post, *time.Time, error) {
			s, err := m.d.Social.SchedulePost(ctx, st, social.SchedulePostInput{PostID: in.PostID, Version: in.Version, PublishAt: in.PublishAt, By: by})
			at := s.Schedule.PublishAt
			return s.Post, &at, err
		},
	})
}

// GetPost 讀代行店家的一篇貼文（能力 posts.read）；別家的貼文與不存在一樣回 NotFound。讀取不寫稽核。
func (m *Module) GetPost(ctx context.Context, c Caller, id uuid.UUID) (PostResult, error) {
	if err := Gate(c.Session, model.CapabilityPostsRead); err != nil {
		return PostResult{}, err
	}
	st, err := m.d.Store(ctx, c.Session.Target.StoreID)
	if err != nil {
		return PostResult{}, err
	}
	p, err := m.d.Social.MyPost(ctx, st, id)
	return PostResult{Post: p, Store: st}, err
}

// ListPosts 列出代行店家的貼文（能力 posts.read）。讀取不寫稽核。
func (m *Module) ListPosts(ctx context.Context, c Caller, status social.PostStatus, after *social.OwnPostCursor, size int32) (PostPageResult, error) {
	if err := Gate(c.Session, model.CapabilityPostsRead); err != nil {
		return PostPageResult{}, err
	}
	st, err := m.d.Store(ctx, c.Session.Target.StoreID)
	if err != nil {
		return PostPageResult{}, err
	}
	page, err := m.d.Social.ListMyPosts(ctx, st, status, after, size)
	return PostPageResult{Page: page, Store: st}, err
}

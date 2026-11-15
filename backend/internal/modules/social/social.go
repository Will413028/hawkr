// Package social 是 social 模組的公開 API：貼文、留言、讚、店家討論區、hashtag、計數、瀏覽與分享、買家的社群處分與隱藏。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md，行為規格見 backend/docs/modules/social.md。互動者 (kind, id) 由伺服器依資源歸屬推導（model.Caller）；店家身分的寫入收已經解析好的
// seller.Store，能力在模組內以 seller.Authorize 檢查。
package social

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"slices"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/actor"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/comment"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/engage"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/forum"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/post"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/readcache"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/sanction"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/schedule"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/stats"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Buyers 是買家身分的檢查需要的 buyer 公開 API（買家要是營運中）。
type Buyers = actor.Buyers

// Stores 是讀店家（快取失效要 store_code）的 seller 公開 API。
type Stores = readcache.Stores

// StoreBlocks 是店家封鎖檢查（買家身分對某店的寫入）需要的 seller 公開 API：只有判斷「店家是否封鎖了買家」的方法。
type StoreBlocks = actor.StoreBlocks

// ProductVerifier 檢查貼文帶的商品：存在、上架中而且屬於這家店。它是跨行程的呼叫（B4 前的 product-service），在 transaction 之前呼叫；
// 由 cmd/hawkr 注入。
type ProductVerifier = post.ProductVerifier

// 商品驗證 port 的結果（port 的實作回傳它們，句子由 social 的目錄給）。
var (
	ErrProductNotAvailable = post.ErrProductNotAvailable
	ErrProductNotOwned     = post.ErrProductNotOwned
)

// Deps 是模組需要的外部依賴，全部在組裝點提供。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Media  media.Media
	Jobs   jobs.Enqueuer
	Buyers Buyers
	Stores Stores
	// StoreBlocks 檢查店家是否封鎖了買家；正式組裝傳 seller.Module。
	StoreBlocks StoreBlocks
	Products    ProductVerifier
	// ScheduleAuthorizers 是排程到點時依發起者種類重驗授權的實作（單體規則 4：social 不 import 發起者的模組，實作由 cmd/hawkr 注入）。
	// OWNER 由 social 自己提供；其他每一種種類都要在這裡給，缺一種 New 就 panic（啟動失敗）。
	ScheduleAuthorizers map[ScheduleKind]ScheduleAuthorizer
	// NewID 產生實體的主鍵，組裝點給 uuid.NewV7（ADR 2026-09-25-row-identity-by-data-kind）。
	NewID func() uuid.UUID
	// Now 是目前時間；測試注入固定值。
	Now    func() time.Time
	Logger *slog.Logger
}

type (
	Post            = model.Post
	PostStatus      = model.PostStatus
	PostStats       = model.PostStats
	Hashtag         = model.Hashtag
	HashtagCount    = model.HashtagCount
	PostCursor      = model.PostCursor
	OwnPostCursor   = model.OwnPostCursor
	PostPage        = model.PostPage
	OwnPostPage     = model.OwnPostPage
	CreatePostInput = model.CreatePostInput
	UpdatePostInput = model.UpdatePostInput
	Actor           = model.Actor
	ActorKind       = model.ActorKind
	Caller          = model.Caller
	Comment         = model.Comment
	CommentState    = model.CommentState
	CommentCursor   = model.CommentCursor
	CommentPage     = model.CommentPage
	CommentInput    = model.CommentInput
	// 排程發布：Schedule 是一次排程，ScheduledBy 是發起者（種類加發起者擁有格式的不透明 ref），ScheduleAuthorizer 是到點重驗授權的 port。
	Schedule           = model.Schedule
	ScheduledPost      = model.ScheduledPost
	ScheduleKind       = model.ScheduleKind
	ScheduleState      = model.ScheduleState
	CancelReason       = model.CancelReason
	ScheduledBy        = model.ScheduledBy
	SchedulePostInput  = model.SchedulePostInput
	ScheduleCursor     = model.ScheduleCursor
	SchedulePage       = model.SchedulePage
	ScheduledPublish   = model.ScheduledPublish
	ScheduleAuthorizer = model.ScheduleAuthorizer
	PostOrigin         = model.PostOrigin
	ForumEntry         = model.ForumEntry
	ForumPage          = model.ForumPage
	ForumInput         = model.ForumInput
	Target             = model.Target
	TargetKind         = model.TargetKind
	HideInput          = model.HideInput
	RestoreInput       = model.RestoreInput
	Hide               = model.Hide
	WarnInput          = model.WarnInput
	Sanction           = model.Sanction
	SanctionKind       = model.SanctionKind
	SanctionCursor     = model.SanctionCursor
	SanctionPage       = model.SanctionPage
	HiddenContent      = model.HiddenContent
	HiddenCursor       = model.HiddenCursor
	HiddenPage         = model.HiddenPage
	ContentOwner       = model.ContentOwner
	ContentSnapshot    = model.ContentSnapshot
	// TakedownContentError 是內容下架的對象不成立（不存在、已刪除、不是措施的店家寫的）；OnMeasureCreated 回這個錯誤讓措施回滾。
	TakedownContentError = stats.TakedownContentError
	Viewer               = engage.Viewer
	RecountReport        = stats.Report
	CountDrift           = stats.Drift
)

const (
	StatusDraft       = model.StatusDraft
	StatusPublished   = model.StatusPublished
	StatusUnpublished = model.StatusUnpublished
	ActorBuyer        = model.ActorBuyer
	ActorStore        = model.ActorStore
	CommentVisible    = model.CommentVisible
	CommentDeleted    = model.CommentDeleted
	CommentHidden     = model.CommentHidden
	TargetComment     = model.TargetComment
	TargetForumEntry  = model.TargetForumEntry
	TargetPost        = model.TargetPost
	SanctionWarning   = model.SanctionWarning
	// MaxTopPosts 是 TopPosts 能回的篇數上限。
	MaxTopPosts = model.MaxTopPosts
	// DefaultTrendingWindow 是熱門標籤呼叫端沒給時間窗時用的長度，也是時間窗的上限。
	DefaultTrendingWindow = model.DefaultTrendingWindow

	ScheduleByOwner      = model.ScheduleByOwner
	ScheduleByDelegation = model.ScheduleByDelegation
	SchedulePending      = model.SchedulePending
	SchedulePublished    = model.SchedulePublished
	ScheduleCanceled     = model.ScheduleCanceled
	CancelAuthorityLost  = model.CancelAuthorityLost
	CancelDeleted        = model.CancelDeleted
	CancelTakenDown      = model.CancelTakenDown
	CancelNotPublishable = model.CancelNotPublishable
	CancelRetryExhausted = model.CancelRetryExhausted
	CancelExpired        = model.CancelExpired
	CancelReplaced       = model.CancelReplaced
	CancelOwnerCanceled  = model.CancelOwnerCanceled
	OriginDirect         = model.OriginDirect
	OriginScheduled      = model.OriginScheduled
	// MinScheduleLead 與 MaxScheduleLead 是排程發布時點距現在的區間。
	MinScheduleLead = model.MinScheduleLead
	MaxScheduleLead = model.MaxScheduleLead
	// ScheduleExpiry 是到點後仍可補發的期限，超過取消為 EXPIRED。
	ScheduleExpiry = model.ScheduleExpiry
)

// ScheduleAuthorizerFunc 讓一個函式當排程授權實作（組裝點與測試用）。
type ScheduleAuthorizerFunc func(ctx context.Context, p ScheduledPublish) error

// AuthorizeScheduledPublish 實作 ScheduleAuthorizer。
func (f ScheduleAuthorizerFunc) AuthorizeScheduledPublish(ctx context.Context, p ScheduledPublish) error {
	return f(ctx, p)
}

// ErrScheduleDenied 是授權實作回報「現在不能發布」：排程轉 CANCELED（AUTHORITY_LOST）、不重試；其他錯誤讓 job 重試。可以用 fmt.Errorf("%w") 包裝。
var ErrScheduleDenied = model.ErrScheduleDenied

// ScheduleKindValues 回傳全部排程發起者種類。
func ScheduleKindValues() []ScheduleKind { return model.ScheduleKindValues() }

// ScheduleStateValues 回傳全部排程狀態。
func ScheduleStateValues() []ScheduleState { return model.ScheduleStateValues() }

// CancelReasonValues 回傳全部排程取消原因。
func CancelReasonValues() []CancelReason { return model.CancelReasonValues() }

// PostStatusValues 回傳全部貼文狀態。
func PostStatusValues() []PostStatus { return model.PostStatusValues() }

// ActorKindValues 回傳全部互動者種類。
func ActorKindValues() []ActorKind { return model.ActorKindValues() }

// SanctionKindValues 回傳全部處分種類。
func SanctionKindValues() []SanctionKind { return model.SanctionKindValues() }

// Module 是 social 模組。
type Module struct {
	post      *post.Service
	comment   *comment.Service
	forum     *forum.Service
	engage    *engage.Service
	sanction  *sanction.Service
	stats     *stats.Service
	purge     *purge.Service
	mediaSync *media.RefSyncer
	jobs      jobs.Enqueuer
	cancel    schedule.Canceler
	now       func() time.Time
}

// New 建立模組。
func New(d Deps) *Module {
	if d.Now == nil || d.NewID == nil {
		panic("social: Now 與 NewID 是必要的")
	}
	if d.StoreBlocks == nil {
		panic("social: StoreBlocks 是必要的")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	actors := actor.Resolver{Buyers: d.Buyers, Blocks: d.StoreBlocks}
	cache := readcache.Cache{Jobs: d.Jobs, Stores: d.Stores}
	authorizers := map[ScheduleKind]ScheduleAuthorizer{ScheduleByOwner: post.OwnerAuthorizer(d.Stores)}
	maps.Copy(authorizers, d.ScheduleAuthorizers)
	if err := post.ValidateAuthorizers(authorizers); err != nil {
		panic(err)
	}
	cancel := schedule.Canceler{DB: d.DB, Events: d.Events}
	st := stats.New(stats.Deps{DB: d.DB, Logger: d.Logger, Now: d.Now, Cache: cache, Cancel: cancel})
	return &Module{
		post: post.New(post.Deps{
			DB: d.DB, Media: d.Media, Jobs: d.Jobs, Events: d.Events, Products: d.Products, Cache: cache, Cancel: cancel, Authorizers: authorizers,
			NewID: d.NewID, Now: d.Now, Logger: d.Logger,
		}),
		comment:   comment.New(comment.Deps{DB: d.DB, Events: d.Events, Actors: actors, NewID: d.NewID, Now: d.Now}),
		forum:     forum.New(forum.Deps{DB: d.DB, Actors: actors, NewID: d.NewID, Now: d.Now}),
		engage:    engage.New(engage.Deps{DB: d.DB, Events: d.Events, Actors: actors, Now: d.Now}),
		sanction:  sanction.New(sanction.Deps{DB: d.DB, Cache: cache, NewID: d.NewID, Now: d.Now}),
		stats:     st,
		purge:     purge.New(purge.Deps{DB: d.DB, Jobs: d.Jobs, Stats: st, Cache: cache, Cancel: cancel, Now: d.Now}),
		mediaSync: mediaref.NewSyncer(d.DB, d.Media, d.Jobs, d.Now, d.Logger),
		jobs:      d.Jobs,
		cancel:    cancel,
		now:       d.Now,
	}
}

// CreatePost 新增貼文；店家要有 CapMarketing。Publish 為真時同一個動作發布（公開要件在這裡驗證）。
func (m *Module) CreatePost(ctx context.Context, store seller.Store, in CreatePostInput) (Post, error) {
	return m.post.Create(ctx, store, in)
}

// UpdatePost 修改貼文（帶讀取時的版本）；店家要有 CapMarketing。
func (m *Module) UpdatePost(ctx context.Context, store seller.Store, in UpdatePostInput) (Post, error) {
	return m.post.Update(ctx, store, in)
}

// PublishPost 發布貼文；店家要有 CapMarketing。
func (m *Module) PublishPost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) (Post, error) {
	return m.post.Publish(ctx, store, postID, version)
}

// UnpublishPost 下架貼文；店家要有 CapContentContract。
func (m *Module) UnpublishPost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) (Post, error) {
	return m.post.Unpublish(ctx, store, postID, version)
}

// DeletePost 軟刪貼文（冪等）；店家要有 CapContentContract。version 為 0 表示不檢查版本。
func (m *Module) DeletePost(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) error {
	return m.post.Delete(ctx, store, postID, version)
}

// SchedulePost 排程發布：貼文要是 DRAFT 或 UNPUBLISHED，in.PublishAt 距現在 5 分鐘到 90 天，仍要滿足公開要件；貼文已有待發布的排程時先把它轉 REPLACED。
// in.By 是發起者：OWNER 的 ref 由伺服器填，DELEGATION 的 ref 由呼叫方給（代行 session 的 ID）；店家要有 CapMarketing。排程與到點的 job 在同一個 transaction 寫入。
func (m *Module) SchedulePost(ctx context.Context, store seller.Store, in SchedulePostInput) (ScheduledPost, error) {
	return m.post.Schedule(ctx, store, in)
}

// CancelPostSchedule 是店主取消貼文待發布的排程（OWNER_CANCELED）；version 是讀取時的版本；回傳取消後貼文的新版本（etag）；沒有待發布的排程時什麼都不做（不檢查版本，回傳目前的版本）；店家要有 CapMarketing。
func (m *Module) CancelPostSchedule(ctx context.Context, store seller.Store, postID uuid.UUID, version int64) (int64, error) {
	return m.post.CancelSchedule(ctx, store, postID, version)
}

// CancelScheduledBy 取消某個發起者（種類加一批 ref，例如代行的 session ID）所有待發布的排程，原因凍結為 reason，回傳被取消的排程 ID；空的 refs 是 no-op。
// 每個被取消的排程發一個 PostScheduleCanceled。會依 ID 由小到大鎖相關貼文列（social 的鎖順序：貼文列 → 排程列），所以別的模組要在自己的 transaction 內、
// 鎖或改自己的列「之前」呼叫它（單體規則 4：social 在 delegation 之前）。
func (m *Module) CancelScheduledBy(ctx context.Context, kind ScheduleKind, refs []string, reason CancelReason) ([]uuid.UUID, error) {
	if err := checkCancelArgs(kind, reason); err != nil {
		return nil, err
	}
	return m.cancel.ByScheduler(ctx, kind, refs, reason, m.now().UTC())
}

// CancelDelegatedSchedulesOfStore 取消一家店所有發起者是代行（DELEGATION）、待發布的排程（撤回店家授權時，不必列舉 session），回傳被取消的排程 ID。
// 呼叫時機與鎖順序同 CancelScheduledBy。
func (m *Module) CancelDelegatedSchedulesOfStore(ctx context.Context, storeID int64, reason CancelReason) ([]uuid.UUID, error) {
	if err := checkCancelArgs(ScheduleByDelegation, reason); err != nil {
		return nil, err
	}
	return m.cancel.OfStoreByKind(ctx, storeID, ScheduleByDelegation, reason, m.now().UTC())
}

// PendingSchedulerRefs 回傳某種發起者目前有待發布排程的 ref（去重）；筆數以待發布的排程為界。發起者的模組用它和自己的資料求交集，找出該取消的排程。
func (m *Module) PendingSchedulerRefs(ctx context.Context, kind ScheduleKind) ([]string, error) {
	return m.post.PendingSchedulerRefs(ctx, kind)
}

func checkCancelArgs(kind ScheduleKind, reason CancelReason) error {
	if !slices.Contains(ScheduleKindValues(), kind) {
		return fmt.Errorf("social：不認得的排程發起者種類 %q", kind)
	}
	if !slices.Contains(CancelReasonValues(), reason) {
		return fmt.Errorf("social：不認得的排程取消原因 %q", reason)
	}
	return nil
}

// PendingSchedule 讀貼文待發布的排程；貼文不是這家店的或沒有待發布的排程回 NotFound。
func (m *Module) PendingSchedule(ctx context.Context, storeID int64, postID uuid.UUID) (Schedule, error) {
	return m.post.PendingSchedule(ctx, storeID, postID)
}

// ListPendingSchedules 列出店家待發布的排程（含各種發起者）：發布時間早的在前，keyset。
func (m *Module) ListPendingSchedules(ctx context.Context, storeID int64, after *ScheduleCursor, size int32) (SchedulePage, error) {
	return m.post.ListPendingSchedules(ctx, storeID, after, size)
}

// PublishScheduledAttempt 是 social.publish_scheduled job 的一次執行：final 為真是 River 的最後一次嘗試，失敗時把排程取消為 RETRY_EXHAUSTED，並仍回傳原錯誤。
func (m *Module) PublishScheduledAttempt(ctx context.Context, scheduleID uuid.UUID, final bool) error {
	return m.post.RunScheduledAttempt(ctx, scheduleID, final)
}

// PublishScheduled 是 social.publish_scheduled job 的內容，給 worker 與測試呼叫：排程不是 PENDING 就什麼都不做（冪等），否則依
// 貼文列 → 排程列的順序鎖住、做 social 自己的檢查、問發起者種類的授權實作，通過才發布（授權被拒就取消、原因 AUTHORITY_LOST、不重試）。
func (m *Module) PublishScheduled(ctx context.Context, scheduleID uuid.UUID) error {
	return m.post.RunScheduled(ctx, scheduleID)
}

// OwnPost 讀店家自己的貼文（任何狀態）。
func (m *Module) OwnPost(ctx context.Context, storeID int64, id uuid.UUID) (Post, error) {
	return m.post.Own(ctx, storeID, id)
}

// PublicPost 讀觀看者可讀的貼文（c.BuyerID 是觀看者，0 是未登入；觀看者封鎖的店的貼文讀不到）；其他一律 NotFound。讀取不計瀏覽數（RecordView 另外呼叫）。
func (m *Module) PublicPost(ctx context.Context, c Caller, id uuid.UUID) (Post, error) {
	return m.post.Public(ctx, c, id)
}

// ListFeed 是動態牆（觀看者可讀的貼文，最新發布的在前；c.BuyerID 封鎖的店不出現，過濾在分頁之前）。
func (m *Module) ListFeed(ctx context.Context, c Caller, after *PostCursor, size int32) (PostPage, error) {
	return m.post.ListFeed(ctx, c, after, size)
}

// ListStorePosts 是店家頁的貼文。
func (m *Module) ListStorePosts(ctx context.Context, c Caller, storeID int64, after *PostCursor, size int32) (PostPage, error) {
	return m.post.ListStore(ctx, c, storeID, after, size)
}

// ListHashtagPosts 列出某個標籤的觀看者可讀的貼文。
func (m *Module) ListHashtagPosts(ctx context.Context, c Caller, tag string, after *PostCursor, size int32) (PostPage, error) {
	return m.post.ListByHashtag(ctx, c, tag, after, size)
}

// ListOwnPosts 是店主自己的貼文（含草稿與下架）；status 為空字串不篩選。
func (m *Module) ListOwnPosts(ctx context.Context, storeID int64, status PostStatus, after *OwnPostCursor, size int32) (OwnPostPage, error) {
	return m.post.ListOwn(ctx, storeID, status, after, size)
}

// TopPosts 是「本週主題」：最近發布的 30 篇觀看者可讀的貼文中瀏覽數最高的 n 篇（n 最多 30，同分以發布時間、ID 排），不分頁。
func (m *Module) TopPosts(ctx context.Context, c Caller, n int32) ([]Post, error) {
	return m.post.Top(ctx, c, n)
}

// SuggestHashtags 是標籤的建議（前綴，或 contains 為真時包含）。全域彙總，不依觀看者。
func (m *Module) SuggestHashtags(ctx context.Context, q string, contains bool, limit int32) ([]HashtagCount, error) {
	return m.post.SuggestHashtags(ctx, q, contains, limit)
}

// TrendingHashtags 是熱門標籤：發布時間在 [from, to) 內的公開貼文上各標籤的貼文數。全域彙總，不依觀看者。
func (m *Module) TrendingHashtags(ctx context.Context, from, to time.Time, limit int32) ([]HashtagCount, error) {
	return m.post.TrendingHashtags(ctx, from, to, limit)
}

// CreateComment 新增留言或回覆（互動者依貼文歸屬推導；回覆只限貼文所屬的店家）。
func (m *Module) CreateComment(ctx context.Context, c Caller, in CommentInput) (Comment, error) {
	return m.comment.Create(ctx, c, in)
}

// UpdateComment 修改自己的留言（被隱藏的不能修改）。
func (m *Module) UpdateComment(ctx context.Context, c Caller, id uuid.UUID, body string) (Comment, error) {
	return m.comment.Update(ctx, c, id, body)
}

// DeleteComment 刪除留言：作者或貼文所屬的店；冪等。
func (m *Module) DeleteComment(ctx context.Context, c Caller, id uuid.UUID) error {
	return m.comment.Delete(ctx, c, id)
}

// ListComments 列出貼文的頂層留言（刪除或隱藏而底下有可見回覆的是佔位項目；與 c.BuyerID 任一方向有個人封鎖的買家的留言連同回覆整則不出現，不留佔位）；
// 店主讀自己的貼文不必公開，也不套個人封鎖。
func (m *Module) ListComments(ctx context.Context, c Caller, postID uuid.UUID, after *CommentCursor, size int32) (CommentPage, error) {
	return m.comment.ListTopLevel(ctx, c, postID, after, size)
}

// ListReplies 列出頂層留言的回覆。
func (m *Module) ListReplies(ctx context.Context, c Caller, parentID uuid.UUID, after *CommentCursor, size int32) (CommentPage, error) {
	return m.comment.ListReplies(ctx, c, parentID, after, size)
}

// CreateForumEntry 在店家的討論區新增留言或回覆。
func (m *Module) CreateForumEntry(ctx context.Context, c Caller, in ForumInput) (ForumEntry, error) {
	return m.forum.Create(ctx, c, in)
}

// UpdateForumEntry 修改自己的討論區留言。
func (m *Module) UpdateForumEntry(ctx context.Context, c Caller, id uuid.UUID, body string) (ForumEntry, error) {
	return m.forum.Update(ctx, c, id, body)
}

// DeleteForumEntry 刪除討論區留言：作者或該店；冪等。
func (m *Module) DeleteForumEntry(ctx context.Context, c Caller, id uuid.UUID) error {
	return m.forum.Delete(ctx, c, id)
}

// ListForumEntries 列出店家討論區的頂層留言（觀看者封鎖了這家店，或與作者任一方向有個人封鎖時，對應的內容不出現；店主讀自己的討論區不套個人封鎖）。
func (m *Module) ListForumEntries(ctx context.Context, c Caller, storeID int64, after *CommentCursor, size int32) (ForumPage, error) {
	return m.forum.ListTopLevel(ctx, c, storeID, after, size)
}

// ListForumReplies 列出頂層討論區留言的回覆。
func (m *Module) ListForumReplies(ctx context.Context, c Caller, parentID uuid.UUID, after *CommentCursor, size int32) (ForumPage, error) {
	return m.forum.ListReplies(ctx, c, parentID, after, size)
}

// LikePost 對公開可讀的貼文按讚（買家）；已經按過回 false。
func (m *Module) LikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error) {
	return m.engage.LikePost(ctx, buyerID, postID)
}

// UnlikePost 取消讚；沒有按過回 false。
func (m *Module) UnlikePost(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error) {
	return m.engage.UnlikePost(ctx, buyerID, postID)
}

// PostsLikedBy 回傳買家按過讚的貼文（批次）。
func (m *Module) PostsLikedBy(ctx context.Context, buyerID int64, postIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	return m.engage.PostsLikedBy(ctx, buyerID, postIDs)
}

// LikeComment 對公開可讀的留言按讚。
func (m *Module) LikeComment(ctx context.Context, c Caller, commentID uuid.UUID) (bool, error) {
	return m.engage.LikeComment(ctx, c, commentID)
}

// UnlikeComment 取消留言的讚。
func (m *Module) UnlikeComment(ctx context.Context, c Caller, commentID uuid.UUID) (bool, error) {
	return m.engage.UnlikeComment(ctx, c, commentID)
}

// CommentsLikedBy 回傳互動者按過讚的留言（批次）。
func (m *Module) CommentsLikedBy(ctx context.Context, a Actor, commentIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	return m.engage.CommentsLikedBy(ctx, a, commentIDs)
}

// RecordShare 記一次分享（買家；同一位買家同一篇只算一次）。
func (m *Module) RecordShare(ctx context.Context, buyerID int64, postID uuid.UUID) (bool, error) {
	return m.engage.RecordShare(ctx, buyerID, postID)
}

// RecordView 記一次瀏覽（只計公開可讀的貼文；同一篇、同一個台北日、同一個觀看者只算一次；匿名沒有 IP 時不計）。回傳這次有沒有計入。
// 店主與代行的讀取由呼叫方不呼叫。
func (m *Module) RecordView(ctx context.Context, postID uuid.UUID, v Viewer) (bool, error) {
	return m.engage.RecordView(ctx, postID, v)
}

// ContentOwner 回傳內容的歸屬（存在、作者、所屬店家、是否公開可讀），給 moderation 在建立處分前驗證。
func (m *Module) ContentOwner(ctx context.Context, t Target) (ContentOwner, error) {
	return m.sanction.Owner(ctx, t)
}

// ContentSnapshot 回傳內容此刻的歸屬（同 ContentOwner，含是否公開可讀）、本文與媒體參照，給 moderation 在檢舉送出時凍結（唯讀，不鎖）；內容不存在或已刪除回 Exists 為假。
func (m *Module) ContentSnapshot(ctx context.Context, t Target) (ContentSnapshot, error) {
	return m.sanction.Snapshot(ctx, t)
}

// Hide 隱藏一則買家寫的留言或討論區留言（會在 moderation 的 transaction 內被呼叫）；已有未還原的隱藏時回既有的（冪等）。
func (m *Module) Hide(ctx context.Context, in HideInput) (Hide, error) {
	return m.sanction.Hide(ctx, in)
}

// Restore 是管理員更正：還原目前的隱藏，內容立即可見。
func (m *Module) Restore(ctx context.Context, in RestoreInput) (Hide, error) {
	return m.sanction.Restore(ctx, in)
}

// Warn 對買家寫一筆警告。
func (m *Module) Warn(ctx context.Context, in WarnInput) (Sanction, error) {
	return m.sanction.Warn(ctx, in)
}

// MySanctions 是買家自己的處分紀錄。
func (m *Module) MySanctions(ctx context.Context, buyerID int64, after *SanctionCursor, size int32) (SanctionPage, error) {
	return m.sanction.MySanctions(ctx, buyerID, after, size)
}

// MyHiddenContent 是買家自己被隱藏的內容與理由。
func (m *Module) MyHiddenContent(ctx context.Context, buyerID int64, after *HiddenCursor, size int32) (HiddenPage, error) {
	return m.sanction.MyHiddenContent(ctx, buyerID, after, size)
}

// RecountAll 對全部可重算的計數跑重算與覆寫並回報漂移（手動全量重算：子命令 social-recount；改了計數定義的 migration 在同一步跑一次）。
func (m *Module) RecountAll(ctx context.Context) (RecountReport, error) {
	return m.stats.RecountAll(ctx)
}

// Reconcile 是每日對帳：全量重算，漂移不為 0 時記 warn log。它是 social.reconcile_counts job 的內容，給 worker 與測試呼叫。
func (m *Module) Reconcile(ctx context.Context) (RecountReport, error) { return m.stats.Reconcile(ctx) }

// PurgeBuyerContent 清除買家的社群內容（軟刪留言與討論區留言、刪讚與分享、重算計數）。它是 social.purge_buyer_content job 的內容，
// 給 worker 與測試呼叫；冪等。
func (m *Module) PurgeBuyerContent(ctx context.Context, buyerID int64) error {
	return m.purge.PurgeBuyer(ctx, buyerID)
}

// PurgeStoreContent 清除退場店家的社群內容（軟刪貼文並釋放圖片引用、軟刪討論區留言與店家身分的留言、重算計數）。它是
// social.purge_store_content job 的內容；冪等。
func (m *Module) PurgeStoreContent(ctx context.Context, storeID int64) error {
	return m.purge.PurgeStore(ctx, storeID)
}

// CleanupViews 清掉瀏覽去重用的舊 salt 與去重列。它是 social.cleanup_views job 的內容。
func (m *Module) CleanupViews(ctx context.Context) error { return m.purge.CleanupViews(ctx) }

// OnBuyerDeletionRequested 是買家進入刪除中時同 transaction 的訂閱者（單體規則 4 的原子狀態變更）：把「這位買家不再營運中」記進自己的 inactive_authors，
// 並讓他的內容在同一個 transaction 重新求值（旗標與計數一起改，資料庫與 API 的讀取立即看不到他的內容，沒有 job 的延遲），再排實體清除的 job；網站的快取頁由這個 job 失效（同 transaction 排入、立即可跑，晚秒級，快取 TTL 保底）。
// 事實的複本是 social 自己的列（buyer 擁有事實，buyer.pub_inactive_buyers 留給每日對帳核對）；鎖的順序 identity → buyer → seller → social：
// 訂閱者在買家的事件裡鎖 social 的列，所以 accountdeletion 在派發事件之前先鎖完買家與店家（TestDeletionLocksBuyerAndSellerBeforeSocialRows 量這個順序）。
func (m *Module) OnBuyerDeletionRequested(ctx context.Context, e buyer.BuyerDeletionRequested) error {
	if _, err := m.stats.MarkAuthorInactive(ctx, e.BuyerID, e.OccurredAt); err != nil {
		return err
	}
	return purge.EnqueueBuyer(ctx, m.jobs, e.BuyerID)
}

// OnBuyerDeleted 是買家關閉時同 transaction 的訂閱者：同 OnBuyerDeletionRequested（冪等）。沒有開啟中的訂單時買家直接從營運中關閉，
// 只發這個事件、沒有 BuyerDeletionRequested，所以兩個事件都要訂、都要寫複本與排清除 job（TestDirectCloseStillHidesUncountsAndPurges）。
func (m *Module) OnBuyerDeleted(ctx context.Context, e buyer.BuyerDeleted) error {
	if _, err := m.stats.MarkAuthorInactive(ctx, e.BuyerID, e.OccurredAt); err != nil {
		return err
	}
	return purge.EnqueueBuyer(ctx, m.jobs, e.BuyerID)
}

// OnMeasureCreated 是 seller 建立措施時同 transaction 的訂閱者（單體規則 4 的原子狀態變更）：只處理內容下架（Contents 不空），把對象寫進自己的
// content_takedowns，讓被下架的留言與討論區留言重新求值，並排受影響的貼文與店面頁的快取失效（貼文的公開讀取直接探測複本）。內容不存在、已刪除或不是措施的店家寫的時回 *TakedownContentError（歸屬核對在這裡，每條建立入口共用），
// 措施跟著回滾。鎖的順序 identity → buyer → seller → social：訂閱者在 seller 的事件裡鎖 social 的列。
func (m *Module) OnMeasureCreated(ctx context.Context, e seller.MeasureCreated) error {
	if len(e.Contents) == 0 {
		return nil
	}
	return m.stats.ApplyTakedowns(ctx, e.MeasureID, e.StoreID, e.OccurredAt, takedownContents(e.Contents))
}

// OnMeasureRevoked 是 seller 撤銷措施時同 transaction 的訂閱者：刪除該措施的複本並讓內容重新求值，內容立即回到公開讀取。同一則內容被另一個未撤銷的措施
// 下架時仍然隱藏。
func (m *Module) OnMeasureRevoked(ctx context.Context, e seller.MeasureRevoked) error {
	if len(e.Contents) == 0 {
		return nil
	}
	return m.stats.ReleaseTakedowns(ctx, e.MeasureID)
}

func takedownContents(refs []seller.ContentRef) []stats.Content {
	out := make([]stats.Content, len(refs))
	for i, r := range refs {
		out[i] = stats.Content{Kind: r.Kind, ID: r.ID}
	}
	return out
}

// OnStoreRetired 是店家退場時同 transaction 的訂閱者：只排清除 job；可見性由 seller.pub_exposable_stores 在讀取時擋住。
func (m *Module) OnStoreRetired(ctx context.Context, e seller.StoreRetired) error {
	return purge.EnqueueStore(ctx, m.jobs, e.StoreID)
}

// SyncMedia 同步一個 media ref（貼文圖，ref_id 是貼文的 UUID）的引用：Link 貼文目前的圖，Unlink 退場的圖。它是 social.media_sync job
// 的內容，給 worker 與測試呼叫；回傳是否要再跑一輪。
func (m *Module) SyncMedia(ctx context.Context, ref media.Ref) (media.SyncResult, error) {
	return m.mediaSync.Sync(ctx, ref)
}

// SweepMedia 執行 media 引用的保底掃描；回傳補排的 ref 數。它是 social.media_sweep job 的內容。
func (m *Module) SweepMedia(ctx context.Context) (int, error) { return m.mediaSync.Sweep(ctx) }

// AddWorkers 登記本模組的 River worker：media 引用、清除、瀏覽資料清理、每日對帳與排程發布。
func (m *Module) AddWorkers(workers *river.Workers) {
	mediaref.AddWorkers(workers, m.mediaSync)
	purge.AddWorkers(workers, m.purge)
	stats.AddWorkers(workers, m.stats)
	post.AddWorkers(workers, m.post)
}

// PeriodicJobs 回傳週期 job：media 引用的保底掃描、瀏覽資料清理、每日對帳。
func (m *Module) PeriodicJobs() []*river.PeriodicJob {
	out := mediaref.PeriodicJobs()
	out = append(out, purge.PeriodicJobs()...)
	return append(out, stats.PeriodicJobs()...)
}

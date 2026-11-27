package delegation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
)


// DelegatedMethod 是一個 Delegated RPC 宣告的代行資料：需要的能力，以及稽核列的動作與資源種類（被拒絕的讀取用 *_READ 動作）。宣告表在資源模組的 httpapi 套件。
type DelegatedMethod struct {
	Capability Capability
	Action     AuditAction
	Resource   AuditResourceType
}

// DelegatedCaller 是 interceptor 為這個請求驗證過的代行：session、被呼叫的方法宣告，與寫進稽核列的請求資料。
type DelegatedCaller struct {
	Session       DelegationSession
	Method        DelegatedMethod
	CorrelationID string
	UserAgent     string
}

// DelegatedOperation 是每個代行寫入共同的稽核資料。
type DelegatedOperation struct {
	// RequestID 是 request_id（AIP-155），同一個 session 內去重。
	RequestID uuid.UUID
	// Reason 是操作原因，去除首尾空白後 1–100 字，不得含控制字元。
	Reason       string
	NotifySeller bool
}

// DelegatedWrite 描述一次代行寫入的稽核內容。
type DelegatedWrite struct {
	Op DelegatedOperation
	// ResourceID 是資源自己的 ID；建立類在寫入前沒有，留空，寫入的事件到了由訂閱者補上。
	ResourceID string
	// ResourceName 寫入失敗或重送前就要記的名稱（建立類的預覽）；寫入成功後以事件為準。
	ResourceName string
	// Version 是請求帶的資源版本（寫入前的版本）。
	Version *int64
	// PayloadHash 綁定 request_id 與請求內容（DelegatedPayloadHash）：同一把 request_id、不同內容不是重送。
	PayloadHash string
}

// DelegatedWriteResult 是 DelegatedWrite 的結果。Replayed 為真時 do 沒有被呼叫，ResourceID 是第一次寫入的資源，呼叫方據它讀回資源目前的樣子。
type DelegatedWriteResult struct {
	// Store 是代行店家（交給 do 的同一份）。
	Store      seller.Store
	AuditLogID uuid.UUID
	ResourceID string
	Replayed   bool
}

// DelegatedPayloadHash 是請求內容的雜湊：parts 依序編成 JSON，動作與所有影響結果的欄位都要放進去。
func DelegatedPayloadHash(parts ...any) string {
	raw, _ := json.Marshal(parts)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (m *Module) operableStore(ctx context.Context, storeID int64) (seller.Store, error) {
	st, state, err := readStore(ctx, m.seller, storeID)
	switch {
	case err != nil:
		return seller.Store{}, err
	case !state.Found:
		return seller.Store{}, model.Deny(model.DenialStoreNotDelegable, "店家不存在")
	case state.Closed:
		return seller.Store{}, model.Deny(model.DenialStoreClosed, "%s", state.UnavailableText)
	case !state.Operable:
		return seller.Store{}, model.Deny(model.DenialStoreNotOperable, "%s", state.UnavailableText)
	}
	return st, nil
}

// AuthorizeDelegated 是傳輸層在處理請求之前的授權：session 此刻還能用並帶有方法宣告的能力。被拒絕時另開 transaction 寫一列 FAILED 的稽核（讀取也寫），再回拒絕。
func (m *Module) AuthorizeDelegated(ctx context.Context, c DelegatedCaller) error {
	err := model.CheckSession(c.Session, c.Method.Capability)
	if err != nil {
		m.recordFailure(ctx, c, DelegatedWrite{}, err)
	}
	return err
}

// DelegatedRead 在代行店家上執行讀取 read：這裡自己檢查 session 此刻還能用並帶有能力（公開 API 的縱深防禦，interceptor 的 AuthorizeDelegated 是第一道），
// 再讀店家此刻的狀態（不可代行拒絕、不撤銷），任何拒絕都寫 FAILED 的稽核。讀取成功不稽核；read 自己的失敗（找不到等）原樣回傳、不稽核。
func (m *Module) DelegatedRead(ctx context.Context, c DelegatedCaller, read func(ctx context.Context, st seller.Store) error) error {
	err := model.CheckSession(c.Session, c.Method.Capability)
	var st seller.Store
	if err == nil {
		st, err = m.operableStore(ctx, c.Session.Target.StoreID)
	}
	if err != nil {
		m.recordFailure(ctx, c, DelegatedWrite{}, err)
		return err
	}
	return read(ctx, st)
}

// DelegatedWrite 在代行店家上執行一次寫入 do（A7-D26）：不開 transaction。先查重送，再檢查 session 此刻還能用並帶有能力（公開 API 的縱深防禦：資源模組的業務驗證不得先於授權），再讀店家此刻可不可代行，然後把代行者（platform/delegatedactor，明確的參數、不經 ctx）交給 do；
// 資源模組在自己的 transaction 內為寫入發帶代行者的事件，delegation 的訂閱者（OnPostCreated 等，依事件種類決定稽核動作）同 transaction 檢查 session 與能力、再次確認店家可代行、寫一列 SUCCEEDED
// （request_id 對到 (session_id, idempotency_key)）。do 失敗時資源模組的 transaction rollback，稽核列跟著消失，所以「有 SUCCEEDED」等於「寫入成功」。
//
// 同一個 session 以同一個 request_id 重送（內容相同）不呼叫 do，回第一次的結果（不再檢查店家此刻可不可代行）；內容不同回 DenialIdempotencyKeyReused。並行的重送：後到的訂閱者撞稽核表唯一鍵、
// 回 ErrOperationKeyTaken 讓資源模組的 transaction rollback，這裡再讀那一列，收斂成重送。任何失敗（被拒絕或 do 的業務失敗）都另開 transaction 寫一列 FAILED。
//
// do 不得假設自己在 transaction 內：外部 provider 的呼叫與 transaction 由資源模組自己安排（單體規則 3、5）。
func (m *Module) DelegatedWrite(ctx context.Context, c DelegatedCaller, w DelegatedWrite, do func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error) (DelegatedWriteResult, error) {
	if w.PayloadHash == "" {
		err := model.Deny(model.DenialInvalidCommand, "請求內容雜湊為必填")
		m.recordFailure(ctx, c, w, err)
		return DelegatedWriteResult{}, err
	}
	res, err := m.delegatedWrite(ctx, c, w, do)
	if err != nil {
		var rr replayReadError
		if !errors.As(err, &rr) {
			m.recordFailure(ctx, c, w, err)
		}
		return DelegatedWriteResult{}, err
	}
	return res, nil
}

type replayReadError struct{ error }

func (e replayReadError) Unwrap() error { return e.error }

func auditPayloadHash(w DelegatedWrite) string {
	return DelegatedPayloadHash(w.PayloadHash, strings.TrimSpace(w.Op.Reason), w.Op.NotifySeller)
}

func auditInput(c DelegatedCaller, w DelegatedWrite) model.OperationAuditInput {
	return model.OperationAuditInput{
		SessionID: c.Session.ID, Capability: c.Method.Capability, Action: c.Method.Action, ResourceType: c.Method.Resource,
		ResourceID: w.ResourceID, ResourceName: w.ResourceName, ResourceVersionBefore: w.Version,
		OperationReason: strings.TrimSpace(w.Op.Reason), NotifySeller: w.Op.NotifySeller,
		RequestID: w.Op.RequestID.String(), CorrelationID: c.CorrelationID, UserAgent: c.UserAgent, PayloadHash: auditPayloadHash(w),
	}
}

func (m *Module) delegatedWrite(ctx context.Context, c DelegatedCaller, w DelegatedWrite, do func(ctx context.Context, st seller.Store, actor *delegatedactor.Actor) error) (DelegatedWriteResult, error) {
	in := auditInput(c, w)
	log, found, err := m.audit.LookupOperation(ctx, in)
	if err != nil {
		return DelegatedWriteResult{}, err
	}
	if found {
		return m.replayResult(ctx, c, log)
	}
	if err := model.CheckSession(c.Session, c.Method.Capability); err != nil {
		return DelegatedWriteResult{}, err
	}
	st, err := m.operableStore(ctx, c.Session.Target.StoreID)
	if err != nil {
		return DelegatedWriteResult{}, err
	}
	err = do(ctx, st, actorOf(in, c))
	if errors.Is(err, model.ErrOperationKeyTaken) {
		if log, found, err = m.audit.LookupOperation(ctx, in); err != nil {
			return DelegatedWriteResult{}, err
		}
		if !found {
			return DelegatedWriteResult{}, fmt.Errorf("delegation：並行的請求 %s 寫下的稽核列讀不到", in.RequestID)
		}
		return m.replayResult(ctx, c, log)
	}
	if err != nil {
		return DelegatedWriteResult{}, err
	}
	log, found, err = m.audit.LookupOperation(ctx, in)
	if err != nil {
		return DelegatedWriteResult{}, err
	}
	if !found {
		return DelegatedWriteResult{}, fmt.Errorf("delegation：代行寫入 %s 成功，卻沒有稽核列（不變式被破壞：資源模組沒發事件？）", in.RequestID)
	}
	out := DelegatedWriteResult{Store: st, AuditLogID: log.ID}
	if log.ResourceID != nil {
		out.ResourceID = *log.ResourceID
	}
	return out, nil
}

func (m *Module) replayResult(ctx context.Context, c DelegatedCaller, log model.AuditLog) (DelegatedWriteResult, error) {
	st, state, err := readStore(ctx, m.seller, c.Session.Target.StoreID)
	if err != nil {
		return DelegatedWriteResult{}, replayReadError{err}
	}
	if !state.Found {
		return DelegatedWriteResult{}, replayReadError{fmt.Errorf("delegation：重送的稽核列 %s 的店家 %d 不存在", log.ID, c.Session.Target.StoreID)}
	}
	out := DelegatedWriteResult{Store: st, AuditLogID: log.ID, Replayed: true}
	if log.ResourceID != nil {
		out.ResourceID = *log.ResourceID
	}
	if out.ResourceID == "" {
		return DelegatedWriteResult{}, replayReadError{fmt.Errorf("delegation：重送的稽核列 %s 沒有資源 ID", log.ID)}
	}
	return out, nil
}

func actorOf(in model.OperationAuditInput, c DelegatedCaller) *delegatedactor.Actor {
	return &delegatedactor.Actor{
		SessionID: c.Session.ID, AdminID: c.Session.Actor.AdminID, StoreID: c.Session.Target.StoreID,
		RequestID: in.RequestID, Reason: in.OperationReason, NotifySeller: in.NotifySeller, PayloadHash: in.PayloadHash,
		ResourceVersionBefore: in.ResourceVersionBefore, CorrelationID: in.CorrelationID, UserAgent: in.UserAgent,
	}
}

const auditNameRunes = 60

type postWrite struct {
	capability Capability
	action     AuditAction
}

var (
	postCreateWrite    = postWrite{CapabilityPostsWrite, AuditPostCreate}
	postUpdateWrite    = postWrite{CapabilityPostsWrite, AuditPostUpdate}
	postPublishWrite   = postWrite{CapabilityPostsPublish, AuditPostPublish}
	postUnpublishWrite = postWrite{CapabilityPostsPublish, AuditPostUnpublish}
	postScheduleWrite  = postWrite{CapabilityPostsPublish, AuditPostSchedule}
)

// PostWriteAudit 是一種貼文寫入事件在稽核上的能力與動作。
type PostWriteAudit struct {
	Capability Capability
	Action     AuditAction
}

// PostWriteFor 回傳 social 貼文寫入事件 e（值，不是指標）對應的能力與動作，不是代行會寫入的事件回 false。這是代行寫入宣告的唯一來源：
// 訂閱者寫稽核列用它，httpapi 的 DelegatedDeclaration 也由它衍生每個寫入方法的能力與動作，兩邊不會各寫一份而不一致。
func PostWriteFor(e any) (PostWriteAudit, bool) {
	var w postWrite
	switch e.(type) {
	case social.PostCreated:
		w = postCreateWrite
	case social.PostUpdated:
		w = postUpdateWrite
	case social.PostPublished, social.PostPublishRepeated:
		w = postPublishWrite
	case social.PostUnpublished, social.PostUnpublishRepeated:
		w = postUnpublishWrite
	case social.PostScheduled:
		w = postScheduleWrite
	default:
		return PostWriteAudit{}, false
	}
	return PostWriteAudit{Capability: w.capability, Action: w.action}, true
}

// OnPostCreated、OnPostUpdated、OnPostUnpublished、OnPostScheduled、OnPostPublishRepeated、OnPostUnpublishRepeated 訂閱 social 的同名事件（組裝點以 events.SubscribeNamed 與 delegatedactor.AuditHandler 訂閱）：
// 事件帶代行者時，代行寫入在 social 的 transaction 內，依事件種類決定稽核動作，再寫一列 SUCCEEDED；沒有代行者（店主自己的寫入）略過。
func (m *Module) OnPostCreated(ctx context.Context, e social.PostCreated) error {
	return m.recordPostWrite(ctx, postCreateWrite, e.PostWritten)
}

func (m *Module) OnPostUpdated(ctx context.Context, e social.PostUpdated) error {
	return m.recordPostWrite(ctx, postUpdateWrite, e.PostWritten)
}

func (m *Module) OnPostUnpublished(ctx context.Context, e social.PostUnpublished) error {
	return m.recordPostWrite(ctx, postUnpublishWrite, e.PostWritten)
}

func (m *Module) OnPostScheduled(ctx context.Context, e social.PostScheduled) error {
	return m.recordPostWrite(ctx, postScheduleWrite, e.PostWritten)
}

func (m *Module) OnPostPublishRepeated(ctx context.Context, e social.PostPublishRepeated) error {
	return m.recordPostWrite(ctx, postPublishWrite, e.PostWritten)
}

func (m *Module) OnPostUnpublishRepeated(ctx context.Context, e social.PostUnpublishRepeated) error {
	return m.recordPostWrite(ctx, postUnpublishWrite, e.PostWritten)
}

func (m *Module) recordPostWrite(ctx context.Context, w postWrite, e social.PostWritten) error {
	a := e.Actor
	if a == nil {
		return nil
	}
	return m.recordPostWriteBy(ctx, w, a, e.PostID, e.StoreID, e.Preview, e.Version)
}

func (m *Module) recordPostWriteBy(ctx context.Context, w postWrite, a *delegatedactor.Actor, postID uuid.UUID, storeID int64, preview string, version int64) error {
	if storeID != a.StoreID {
		return fmt.Errorf("delegation：貼文寫入事件的店家 %d 與代行者的店家 %d 不一致", storeID, a.StoreID)
	}
	if _, err := m.operableStore(ctx, a.StoreID); err != nil {
		return err
	}
	name := strings.TrimSpace(preview)
	if r := []rune(name); len(r) > auditNameRunes {
		name = string(r[:auditNameRunes])
	}
	return m.audit.RecordOperation(ctx, model.OperationAuditInput{
		SessionID: a.SessionID, Capability: w.capability, Action: w.action, ResourceType: ResourcePost,
		ResourceID: postID.String(), ResourceName: name, ResourceVersionBefore: a.ResourceVersionBefore,
		OperationReason: a.Reason, NotifySeller: a.NotifySeller,
		RequestID: a.RequestID, CorrelationID: a.CorrelationID, UserAgent: a.UserAgent, PayloadHash: a.PayloadHash,
	}, model.OperationOutcome{ResourceID: postID.String(), ResourceName: name, VersionAfter: version})
}

const failureTimeout = 5 * time.Second

func (m *Module) recordFailure(ctx context.Context, c DelegatedCaller, w DelegatedWrite, cause error) {
	requestID := w.Op.RequestID.String()
	if w.Op.RequestID == (uuid.UUID{}) {
		requestID = c.CorrelationID
	}
	if requestID == "" {
		requestID = uuid.New().String()
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureTimeout)
	defer cancel()
	err := m.audit.RecordFailure(ctx, model.FailureAuditInput{
		Session: c.Session, Capability: c.Method.Capability, Action: c.Method.Action, ResourceType: c.Method.Resource,
		ResourceID: w.ResourceID, ResourceName: w.ResourceName, ResourceVersionBefore: w.Version, OperationReason: w.Op.Reason,
		RequestID: requestID, CorrelationID: c.CorrelationID, UserAgent: c.UserAgent, Cause: cause,
	})
	if err != nil {
		logger := m.logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.ErrorContext(ctx, "delegation：失敗的代行請求沒有寫進稽核", "session_id", c.Session.ID, "action", c.Method.Action, "err", err)
	}
}

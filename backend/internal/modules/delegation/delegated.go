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

// 代行 Connect 的共用元件（ADR 2026-09-27-delegation-module「代行 Connect」修訂）：資源模組擁有 `<模組>DelegatedService` 與資源的寫入，delegation 提供
// 能力檢查、店家可代行檢查與稽核，每個資源模組的 handler 以 DelegatedRead、DelegatedWrite 帶著自己的讀寫（do）呼叫，不重寫稽核、冪等與失敗紀錄。

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

// operableStore 是代行要交給資源模組的店家：不存在、已關閉或此刻不可代行時回代行的拒絕（句子與 StartDelegation 被拒時同一句）。只拒絕，不撤銷 session。
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

// DelegatedWrite 在代行店家上執行一次寫入 do（A7-D26）：不開 transaction。先查重送，再讀店家此刻可不可代行，然後帶著代行者（platform/delegatedactor）呼叫 do；
// 資源模組在自己的 transaction 內為寫入發帶代行者的事件，delegation 的訂閱者（OnPostWrittenByActor）同 transaction 檢查 session 與能力、再次確認店家可代行、寫一列 SUCCEEDED
// （request_id 對到 (session_id, idempotency_key)）。do 失敗時資源模組的 transaction rollback，稽核列跟著消失，所以「有 SUCCEEDED」等於「寫入成功」。
//
// 同一個 session 以同一個 request_id 重送（內容相同）不呼叫 do，回第一次的結果（不再檢查店家此刻可不可代行）；內容不同回 DenialIdempotencyKeyReused。並行的重送：後到的訂閱者撞稽核表唯一鍵、
// 回 ErrOperationKeyTaken 讓資源模組的 transaction rollback，這裡再讀那一列，收斂成重送。任何失敗（被拒絕或 do 的業務失敗）都另開 transaction 寫一列 FAILED。
//
// do 不得假設自己在 transaction 內：外部 provider 的呼叫與 transaction 由資源模組自己安排（單體規則 3、5）。
func (m *Module) DelegatedWrite(ctx context.Context, c DelegatedCaller, w DelegatedWrite, do func(ctx context.Context, st seller.Store) error) (DelegatedWriteResult, error) {
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

// replayReadError 是重送時讀回店家失敗：request_id 已經有 SUCCEEDED 的列，不再為它寫 FAILED。
type replayReadError struct{ error }

func (e replayReadError) Unwrap() error { return e.error }

// auditPayloadHash 是寫進稽核列的內容雜湊：資源模組算的請求內容，加上操作原因與 notify_seller（它們也影響結果：稽核內容與賣家通知），
// 同一把 request_id 帶不同的原因或通知設定不是重送（AIP-155）。
func auditPayloadHash(w DelegatedWrite) string {
	return DelegatedPayloadHash(w.PayloadHash, strings.TrimSpace(w.Op.Reason), w.Op.NotifySeller)
}

// auditInput 是請求這一側知道的稽核資料（重送比對用）；寫入後才知道的資源 ID 與版本由事件補上。
func auditInput(c DelegatedCaller, w DelegatedWrite) model.OperationAuditInput {
	return model.OperationAuditInput{
		SessionID: c.Session.ID, Capability: c.Method.Capability, Action: c.Method.Action, ResourceType: c.Method.Resource,
		ResourceID: w.ResourceID, ResourceName: w.ResourceName, ResourceVersionBefore: w.Version,
		OperationReason: strings.TrimSpace(w.Op.Reason), NotifySeller: w.Op.NotifySeller,
		RequestID: w.Op.RequestID.String(), CorrelationID: c.CorrelationID, UserAgent: c.UserAgent, PayloadHash: auditPayloadHash(w),
	}
}

func (m *Module) delegatedWrite(ctx context.Context, c DelegatedCaller, w DelegatedWrite, do func(ctx context.Context, st seller.Store) error) (DelegatedWriteResult, error) {
	in := auditInput(c, w)
	log, found, err := m.audit.LookupOperation(ctx, in)
	if err != nil {
		return DelegatedWriteResult{}, err
	}
	if found {
		return m.replayResult(ctx, c, log)
	}
	st, err := m.operableStore(ctx, c.Session.Target.StoreID)
	if err != nil {
		return DelegatedWriteResult{}, err
	}
	err = do(delegatedactor.With(ctx, actorOf(c, in)), st)
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
		return DelegatedWriteResult{}, fmt.Errorf("delegation：代行寫入 %s 成功，卻沒有稽核列（資源模組沒發事件？）", in.RequestID)
	}
	out := DelegatedWriteResult{Store: st, AuditLogID: log.ID}
	if log.ResourceID != nil {
		out.ResourceID = *log.ResourceID
	}
	return out, nil
}

// replayResult 回第一次寫入的結果，與店家此刻的狀態無關（AIP-155）：不可代行的檢查只屬於第一次寫入。這裡只讀店家本身，交給呼叫方讀回資源。
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

func actorOf(c DelegatedCaller, in model.OperationAuditInput) delegatedactor.Actor {
	return delegatedactor.Actor{
		SessionID: c.Session.ID, AdminID: c.Session.Actor.AdminID, StoreID: c.Session.Target.StoreID,
		Capability: string(in.Capability), Action: string(in.Action), Resource: string(in.ResourceType),
		RequestID: in.RequestID, Reason: in.OperationReason, NotifySeller: in.NotifySeller, PayloadHash: in.PayloadHash,
		ResourceVersionBefore: in.ResourceVersionBefore, CorrelationID: in.CorrelationID, UserAgent: in.UserAgent,
	}
}

// auditNameRunes 是稽核列的資源名稱長度（貼文內文去除首尾空白後的前 N 個字元），與 handler 寫 FAILED 列時的預覽一致。
const auditNameRunes = 60

// OnPostWrittenByActor 訂閱 social 的 PostWrittenByActor：代行寫入在 social 的 transaction 內，檢查事件與代行者一致（店家、動作）、session 此刻還能用且帶有能力、
// 店家此刻可代行（第一次寫入；重送在呼叫 social 之前就回了），然後寫一列 SUCCEEDED 的稽核。任何一步失敗 social 的 transaction 整個 rollback。組裝點以 events.Subscribe 訂閱。
func (m *Module) OnPostWrittenByActor(ctx context.Context, e social.PostWrittenByActor) error {
	a := e.Actor
	action, ok := postWriteAction(e.Kind)
	if !ok || string(action) != a.Action || e.StoreID != a.StoreID {
		return fmt.Errorf("delegation：貼文寫入事件 %s（店家 %d）與代行者宣告的 %s（店家 %d）不一致", e.Kind, e.StoreID, a.Action, a.StoreID)
	}
	if _, err := m.operableStore(ctx, a.StoreID); err != nil {
		return err
	}
	name := strings.TrimSpace(e.Preview)
	if r := []rune(name); len(r) > auditNameRunes {
		name = string(r[:auditNameRunes])
	}
	return m.audit.RecordOperation(ctx, model.OperationAuditInput{
		SessionID: a.SessionID, Capability: Capability(a.Capability), Action: action, ResourceType: AuditResourceType(a.Resource),
		ResourceID: e.PostID.String(), ResourceName: name, ResourceVersionBefore: a.ResourceVersionBefore,
		OperationReason: a.Reason, NotifySeller: a.NotifySeller,
		RequestID: a.RequestID, CorrelationID: a.CorrelationID, UserAgent: a.UserAgent, PayloadHash: a.PayloadHash,
	}, model.OperationOutcome{ResourceID: e.PostID.String(), ResourceName: name, VersionAfter: e.Version})
}

func postWriteAction(k social.PostWriteKind) (AuditAction, bool) {
	switch k {
	case social.PostWriteCreated:
		return AuditPostCreate, true
	case social.PostWriteUpdated:
		return AuditPostUpdate, true
	case social.PostWritePublished:
		return AuditPostPublish, true
	case social.PostWriteUnpublished:
		return AuditPostUnpublish, true
	case social.PostWriteScheduled:
		return AuditPostSchedule, true
	}
	return "", false
}

// failureTimeout 是寫 FAILED 稽核的期限：請求的 ctx 已取消或逾時時仍要留下一列。
const failureTimeout = 5 * time.Second

// recordFailure 另開 transaction 寫一列 FAILED 的稽核。寫不進去只記 log、不改變回給呼叫方的錯誤（原本的拒絕或失敗才是請求的結果）。
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

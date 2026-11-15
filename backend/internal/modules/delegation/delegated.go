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
	// ResourceID 是資源自己的 ID；建立類在寫入前沒有，留空，寫入後由 DelegatedOutcome.ResourceID 補上。
	ResourceID string
	// ResourceName 寫入失敗或重送前就要記的名稱（建立類的預覽）；寫入成功後以 DelegatedOutcome 為準。
	ResourceName string
	// Version 是請求帶的資源版本（寫入前的版本）。
	Version *int64
	// PayloadHash 綁定 request_id 與請求內容（DelegatedPayloadHash）：同一把 request_id、不同內容不是重送。
	PayloadHash string
}

// DelegatedOutcome 是寫入完成後補進稽核列的事實。
type DelegatedOutcome = model.OperationOutcome

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

// DelegatedWrite 在代行店家上執行一次寫入 do，在同一個 transaction 內：檢查 session 與能力（interceptor 已在請求進來時檢查過一次，這裡是 transaction 內的第二次，對的是不同時間點的 session）、讀店家此刻可不可代行（只在第一次寫入）、寫一列 SUCCEEDED 的稽核（request_id 對到 (session_id, idempotency_key)）、
// 呼叫 do、把資源與版本補進稽核列。do 失敗時整個 transaction rollback（稽核列跟著消失，所以「有 SUCCEEDED」等於「寫入成功」）。同一個 session 以同一個 request_id 重送
// （內容相同）不呼叫 do，回第一次的結果（不再檢查店家此刻可不可代行）。任何失敗（被拒絕或 do 的業務失敗）都在 transaction 之外另寫一列 FAILED。
//
// do 在 transaction 內執行，不得呼叫外部 provider（單體規則 5）；do 拿到的 ctx 帶著這個 transaction。
func (m *Module) DelegatedWrite(ctx context.Context, c DelegatedCaller, w DelegatedWrite, do func(ctx context.Context, st seller.Store) (DelegatedOutcome, error)) (DelegatedWriteResult, error) {
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

func (m *Module) delegatedWrite(ctx context.Context, c DelegatedCaller, w DelegatedWrite, do func(ctx context.Context, st seller.Store) (DelegatedOutcome, error)) (DelegatedWriteResult, error) {
	var st seller.Store
	log, replayed, err := m.audit.RunOperation(ctx, model.OperationAuditInput{
		SessionID: c.Session.ID, Capability: c.Method.Capability, Action: c.Method.Action, ResourceType: c.Method.Resource,
		ResourceID: w.ResourceID, ResourceName: w.ResourceName, ResourceVersionBefore: w.Version,
		OperationReason: strings.TrimSpace(w.Op.Reason), NotifySeller: w.Op.NotifySeller,
		RequestID: w.Op.RequestID.String(), CorrelationID: c.CorrelationID, UserAgent: c.UserAgent, PayloadHash: auditPayloadHash(w),
	}, func(ctx context.Context) (model.OperationOutcome, error) {
		var err error
		if st, err = m.operableStore(ctx, c.Session.Target.StoreID); err != nil {
			return model.OperationOutcome{}, err
		}
		return do(ctx, st)
	})
	if err != nil {
		return DelegatedWriteResult{}, err
	}
	if replayed {
		// 重送回第一次的結果，與店家此刻的狀態無關（AIP-155）：不可代行的檢查只屬於第一次寫入（在 transaction 內）。這裡只讀店家本身，交給呼叫方讀回資源。
		var state model.StoreState
		if st, state, err = readStore(ctx, m.seller, c.Session.Target.StoreID); err != nil {
			return DelegatedWriteResult{}, replayReadError{err}
		}
		if !state.Found {
			return DelegatedWriteResult{}, replayReadError{fmt.Errorf("delegation：重送的稽核列 %s 的店家 %d 不存在", log.ID, c.Session.Target.StoreID)}
		}
	}
	out := DelegatedWriteResult{Store: st, AuditLogID: log.ID, Replayed: replayed}
	if log.ResourceID != nil {
		out.ResourceID = *log.ResourceID
	}
	if replayed && out.ResourceID == "" {
		return DelegatedWriteResult{}, fmt.Errorf("delegation：重送的稽核列 %s 沒有資源 ID", log.ID)
	}
	return out, nil
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

package audit

import (
	"context"
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// RunOperation 是代行互動寫入的單段稽核：在同一個 transaction 內先寫一列 SUCCEEDED 的稽核（冪等鍵是 in.RequestID），再執行 do（被稽核的寫入），
// 最後把資源與版本補進那一列。do 失敗時整個 transaction rollback，稽核列跟著消失，所以「稽核成功」一定對應「寫入成功」。
//
// 同一個 session 以同一把 RequestID 重送（內容相同）時不執行 do，回傳第一次的那一列與 replayed 為 true，呼叫方據 ResourceID 讀回資源目前的樣子；
// 內容不同（動作、資源種類、資源 ID 或內容雜湊不同）回 DenialIdempotencyKeyReused。session 已撤銷或到期寫不進稽核（排程到點的例外不走這條路）。
// 稽核列要通知賣家時，完成事件在補上資源之後、同一個 transaction 內排入送出。
//
// 鎖：只有 INSERT（外鍵對 session 列的 FOR KEY SHARE 與所有 session 寫入的 NO KEY UPDATE 相容）與 UPDATE 自己剛寫的列，沒有先鎖 delegation 既有的列；
// 呼叫 social 在這之後，social 的貼文列鎖仍然在 delegation 的任何會與之衝突的列鎖之前。
func (m *Module) RunOperation(ctx context.Context, in model.OperationAuditInput, do func(ctx context.Context) (model.OperationOutcome, error)) (model.AuditLog, bool, error) {
	if err := validateOperation(in); err != nil {
		return model.AuditLog{}, false, err
	}
	var (
		out      model.AuditLog
		replayed bool
	)
	err := m.d.DB.InTx(ctx, func(ctx context.Context) error {
		now := m.d.Now()
		s, err := m.d.Session.SessionForAudit(ctx, model.AuditSessionInput{ID: in.SessionID, Origin: model.AuditInteractive, Now: now})
		if err != nil {
			return err
		}
		q := m.q(ctx)
		notify := in.NotifySeller && in.Action.MayNotifySeller()
		row, err := q.InsertSucceededAuditLog(ctx, store.InsertSucceededAuditLogParams{
			ID:                    m.d.NewID(),
			SessionID:             s.ID,
			ActorAdminID:          s.Actor.AdminID,
			ActorDisplayName:      s.Actor.DisplayName,
			TargetStoreID:         s.Target.StoreID,
			TargetSellerAccountID: s.Target.SellerAccountID,
			TargetStoreName:       s.Target.StoreName,
			Capability:            string(in.Capability),
			Action:                string(in.Action),
			ResourceType:          string(in.ResourceType),
			ResourceID:            model.Optional(in.ResourceID),
			ResourceName:          model.Optional(truncateRunes(in.ResourceName, model.ResourceNameMaxRunes)),
			OperationReason:       model.Optional(in.OperationReason),
			CaseReference:         s.CaseReference,
			RequestID:             in.RequestID,
			CorrelationID:         in.CorrelationID,
			IdempotencyKey:        in.RequestID,
			PayloadHash:           model.Optional(in.PayloadHash),
			ResourceVersionBefore: in.ResourceVersionBefore,
			UserAgent:             model.Optional(in.UserAgent),
			RiskLevel:             string(in.Action.RiskLevel()),
			NotifySeller:          notify,
			Now:                   now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			existing, err := q.GetDelegationAuditLogByIdempotencyKey(ctx, store.GetDelegationAuditLogByIdempotencyKeyParams{
				SessionID: in.SessionID, IdempotencyKey: model.Optional(in.RequestID),
			})
			if err != nil {
				return fmt.Errorf("delegation：讀取稽核的冪等鍵：%w", err)
			}
			if !sameOperation(existing, in) {
				return model.Deny(model.DenialIdempotencyKeyReused, "request_id 已用於不同的請求內容")
			}
			out, replayed = toAuditLog(existing), true
			return nil
		}
		if err != nil {
			return fmt.Errorf("delegation：寫入代行稽核：%w", err)
		}
		outcome, err := do(ctx)
		if err != nil {
			return err
		}
		var after *int64
		if outcome.VersionAfter != 0 {
			after = &outcome.VersionAfter
		}
		row, err = q.SetAuditLogOutcome(ctx, store.SetAuditLogOutcomeParams{
			ID:                   row.ID,
			ResourceID:           model.Optional(outcome.ResourceID),
			ResourceName:         model.Optional(truncateRunes(outcome.ResourceName, model.ResourceNameMaxRunes)),
			ResourceVersionAfter: after,
		})
		if err != nil {
			return fmt.Errorf("delegation：補上代行稽核的資源：%w", err)
		}
		if row.NotifiedAt != nil {
			if err := events.Publish(ctx, m.d.Events, delegationCompleted(row, s)); err != nil {
				return err
			}
		}
		out = toAuditLog(row)
		return nil
	})
	if err != nil {
		return model.AuditLog{}, false, err
	}
	return out, replayed, nil
}

// sameOperation 回傳既有的稽核列是不是同一個請求寫的：動作、資源種類與內容雜湊相同，請求帶了資源 ID 時也要相同。
func sameOperation(l store.DelegationAdminOperationAuditLog, in model.OperationAuditInput) bool {
	if l.Action != string(in.Action) || l.ResourceType != string(in.ResourceType) {
		return false
	}
	if in.ResourceID != "" && (l.ResourceID == nil || *l.ResourceID != in.ResourceID) {
		return false
	}
	return l.PayloadHash != nil && *l.PayloadHash == in.PayloadHash
}

func validateOperation(in model.OperationAuditInput) error {
	switch {
	case !in.Action.Known() || in.Action.ReadDenial():
		return model.Deny(model.DenialInvalidCommand, "action 不是代行的寫入動作")
	case !in.ResourceType.Valid():
		return model.Deny(model.DenialInvalidCommand, "resourceType 不正確")
	case !in.Capability.Valid():
		return model.Deny(model.DenialInvalidCommand, "capability 不正確")
	case in.RequestID == "" || in.PayloadHash == "":
		return model.Deny(model.DenialInvalidCommand, "requestId 與請求內容雜湊為必填")
	case in.OperationReason == "":
		return model.Deny(model.DenialInvalidCommand, "請填寫本次操作的原因")
	case utf8.RuneCountInString(in.OperationReason) > 100:
		return model.Deny(model.DenialInvalidCommand, "操作原因不得超過 100 字")
	}
	for _, r := range in.OperationReason {
		if unicode.IsControl(r) {
			return model.Deny(model.DenialInvalidCommand, "操作原因不得包含控制字元")
		}
	}
	return nil
}

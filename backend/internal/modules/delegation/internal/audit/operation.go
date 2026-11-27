package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// LookupOperation 查同一個 session 的同一把 request_id 是否已有 SUCCEEDED 的稽核列（呼叫資源模組之前的重送偵測，在任何 transaction 之外）：沒有回 found 為 false；
// 內容相同（動作、資源種類、資源 ID 或內容雜湊）回那一列，呼叫方據 ResourceID 讀回資源目前的樣子；內容不同回 DenialIdempotencyKeyReused。
func (m *Module) LookupOperation(ctx context.Context, in model.OperationAuditInput) (model.AuditLog, bool, error) {
	if err := validateOperation(in); err != nil {
		return model.AuditLog{}, false, err
	}
	existing, err := m.q(ctx).GetDelegationAuditLogByIdempotencyKey(ctx, store.GetDelegationAuditLogByIdempotencyKeyParams{
		SessionID: in.SessionID, IdempotencyKey: model.Optional(in.RequestID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AuditLog{}, false, nil
	}
	if err != nil {
		return model.AuditLog{}, false, fmt.Errorf("delegation：讀取稽核的冪等鍵：%w", err)
	}
	if !sameOperation(existing, in) {
		return model.AuditLog{}, false, model.Deny(model.DenialIdempotencyKeyReused, "request_id 已用於不同的請求內容")
	}
	out := toAuditLog(existing)
	if model.AuditAction(existing.Action) == model.AuditProductBatchUnpublish {
		items, err := m.q(ctx).ListDelegationAuditLogItems(ctx, existing.ID)
		if err != nil {
			return model.AuditLog{}, false, fmt.Errorf("delegation：讀取批次稽核的商品清單：%w", err)
		}
		for _, it := range items {
			out.Items = append(out.Items, model.AuditItem{
				ResourceID: it.ResourceID, ResourceName: deref(it.ResourceName),
				VersionBefore: deref(it.ResourceVersionBefore), VersionAfter: deref(it.ResourceVersionAfter),
			})
		}
	}
	return out, true, nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// RecordOperation 是代行互動寫入的單段稽核，由資源模組的事件訂閱者在寫入的同一個 transaction 內呼叫（A7-D26）：檢查 session 此刻還能用且帶有 in.Capability
// （model.CheckSession，讀的是這個 transaction 內的 session），寫一列 SUCCEEDED（冪等鍵是 in.RequestID，資源與版本寫入後才知道，由 outcome 給），
// 稽核列要通知賣家時，完成事件在同一個 transaction 內排入送出。呼叫方的寫入失敗時整個 transaction rollback，稽核列跟著消失，所以「稽核成功」一定對應「寫入成功」。
// 同一把 request_id 已有列時不寫、回 ErrOperationKeyTaken（呼叫方回傳它讓 transaction rollback）。必須在 transaction 內呼叫。
//
// 鎖：只有 INSERT（外鍵對 session 列的 FOR KEY SHARE 與所有 session 寫入的 NO KEY UPDATE 相容）與 UPDATE 自己剛寫的列，沒有先鎖 delegation 既有的列；
// 資源模組的貼文列鎖在這之前，仍在 delegation 的任何會與之衝突的列鎖之前。
func (m *Module) RecordOperation(ctx context.Context, in model.OperationAuditInput, outcome model.OperationOutcome) error {
	if err := validateOperation(in); err != nil {
		return err
	}
	now := m.d.Now()
	s, err := m.d.Session.SessionForAudit(ctx, model.AuditSessionInput{ID: in.SessionID, Origin: model.AuditInteractive, Now: now})
	if err != nil {
		return err
	}
	if err := model.CheckSession(s, in.Capability); err != nil {
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
		return model.ErrOperationKeyTaken
	}
	if err != nil {
		return fmt.Errorf("delegation：寫入代行稽核：%w", err)
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
	if len(outcome.Items) > 0 && in.Action != model.AuditProductBatchUnpublish {
		return fmt.Errorf("delegation：只有批次動作的稽核列帶商品清單，卻收到 %s 的 %d 項", in.Action, len(outcome.Items))
	}
	for _, it := range outcome.Items {
		before, after := it.VersionBefore, it.VersionAfter
		if err := q.InsertDelegationAuditLogItem(ctx, store.InsertDelegationAuditLogItemParams{
			AuditLogID: row.ID, ResourceID: it.ResourceID, ResourceName: model.Optional(truncateRunes(it.ResourceName, model.ResourceNameMaxRunes)),
			ResourceVersionBefore: &before, ResourceVersionAfter: &after,
		}); err != nil {
			return fmt.Errorf("delegation：寫入批次稽核的商品 %s：%w", it.ResourceID, err)
		}
	}
	if row.NotifiedAt != nil {
		return events.Publish(ctx, m.d.Events, delegationCompleted(row, s))
	}
	return nil
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
	case !in.Action.Known() || in.Action.DenialOnly():
		return model.Deny(model.DenialInvalidCommand, "action 不是代行的寫入動作")
	case !in.ResourceType.Valid():
		return model.Deny(model.DenialInvalidCommand, "resourceType 不正確")
	case !in.Capability.Valid():
		return model.Deny(model.DenialInvalidCommand, "capability 不正確")
	case in.RequestID == "" || in.PayloadHash == "":
		return model.Deny(model.DenialInvalidCommand, "requestId 與請求內容雜湊為必填")
	}
	if _, ok := model.LegalOperationReason(in.OperationReason); !ok {
		return model.Deny(model.DenialInvalidCommand, "操作原因必須是去除首尾空白後 1 到 100 字、不含控制字元的文字")
	}
	return nil
}

// RecordFailure 寫一列 FAILED 的稽核（失敗的代行請求：被拒絕或寫入的業務失敗），在自己的 transaction 內：被稽核的寫入若已 rollback，這一列仍要留下。
// 沒有冪等鍵，不占用 request_id。呼叫方不得在 transaction 內呼叫（那會與外層一起 rollback）。
func (m *Module) RecordFailure(ctx context.Context, in model.FailureAuditInput) error {
	if !in.Action.Known() || !in.ResourceType.Valid() || !in.Capability.Valid() {
		return fmt.Errorf("delegation：失敗稽核的動作、資源種類或能力不在登錄的值域內：%s %s %s", in.Action, in.ResourceType, in.Capability)
	}
	s := in.Session
	reason, _ := model.LegalOperationReason(in.OperationReason)
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := m.q(ctx).InsertFailedAuditLog(ctx, store.InsertFailedAuditLogParams{
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
			FailureReason:         model.FailureReasonOf(in.Cause),
			OperationReason:       model.Optional(reason),
			CaseReference:         s.CaseReference,
			RequestID:             in.RequestID,
			CorrelationID:         in.CorrelationID,
			ResourceVersionBefore: in.ResourceVersionBefore,
			UserAgent:             model.Optional(in.UserAgent),
			RiskLevel:             string(in.Action.RiskLevel()),
			Now:                   m.d.Now(),
		})
		if err != nil {
			return fmt.Errorf("delegation：寫入失敗的代行稽核：%w", err)
		}
		return nil
	})
}

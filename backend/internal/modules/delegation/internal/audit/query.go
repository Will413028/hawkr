package audit

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
)

// DelegationAuditLogs 列出稽核：帶 SessionID 是單一 session 的時間線（session 不存在是 9277），不帶是跨 session 的清單
// （以管理員、店家篩選）。篩選的值不合法是 9260，不靜默忽略（前端會以為篩選生效，實際拿到全集）。
func (m *Module) DelegationAuditLogs(ctx context.Context, f model.AuditLogFilter, p model.DelegationPaging) (model.AuditLogPage, error) {
	q := m.q(ctx)
	if f.SessionID != nil {
		_, err := m.d.Session.Session(ctx, *f.SessionID)
		if err != nil {
			return model.AuditLogPage{}, err
		}
	}
	switch {
	case f.AdminID != nil && *f.AdminID <= 0:
		return model.AuditLogPage{}, model.Deny(model.DenialInvalidCommand, "actorAdminId 不正確")
	case f.StoreID != nil && *f.StoreID <= 0:
		return model.AuditLogPage{}, model.Deny(model.DenialInvalidCommand, "targetStoreId 不正確")
	case f.Action != "" && !f.Action.Known():
		return model.AuditLogPage{}, model.Deny(model.DenialInvalidCommand, "action 篩選值不正確")
	case f.ResourceType != "" && !f.ResourceType.Valid():
		return model.AuditLogPage{}, model.Deny(model.DenialInvalidCommand, "resourceType 篩選值不正確")
	case f.Result != "" && f.Result != model.AuditPending && f.Result != model.AuditSucceeded && f.Result != model.AuditFailed:
		return model.AuditLogPage{}, model.Deny(model.DenialInvalidCommand, "result 篩選值不正確")
	}
	if err := model.ValidateRange(f.From, f.To); err != nil {
		return model.AuditLogPage{}, err
	}
	p, err := p.Normalize()
	if err != nil {
		return model.AuditLogPage{}, err
	}
	rows, err := q.ListDelegationAuditLogs(ctx, store.ListDelegationAuditLogsParams{
		SessionID: f.SessionID, ActorAdminID: f.AdminID, TargetStoreID: f.StoreID, Action: string(f.Action),
		ResourceType: string(f.ResourceType), Result: string(f.Result), DateFrom: f.From, DateTo: f.To,
		RowOffset: p.Offset(), RowLimit: p.PageSize,
	})
	if err != nil {
		return model.AuditLogPage{}, fmt.Errorf("delegation：列出代行稽核：%w", err)
	}
	total, err := q.CountDelegationAuditLogs(ctx, store.CountDelegationAuditLogsParams{
		SessionID: f.SessionID, ActorAdminID: f.AdminID, TargetStoreID: f.StoreID, Action: string(f.Action),
		ResourceType: string(f.ResourceType), Result: string(f.Result), DateFrom: f.From, DateTo: f.To,
	})
	if err != nil {
		return model.AuditLogPage{}, fmt.Errorf("delegation：計算代行稽核數：%w", err)
	}
	page := model.AuditLogPage{Total: total, Paging: p, Logs: []model.AuditLog{}}
	for _, r := range rows {
		page.Logs = append(page.Logs, toAuditLog(r))
	}
	return page, nil
}

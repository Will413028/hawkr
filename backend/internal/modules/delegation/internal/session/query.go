package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

func revokerDisplayName(admins map[int64]identity.Admin, id *int64) *string {
	if id == nil {
		return nil
	}
	if admin, ok := admins[*id]; ok {
		return &admin.Email
	}
	return nil
}

func delegationSessionRow(ctx context.Context, q *store.Queries, id int64) (store.DelegationDelegatedOperationSession, error) {
	s, err := q.GetDelegationSession(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, model.Deny(model.DenialSessionNotFound, "找不到指定的代行工作階段")
	}
	if err != nil {
		return s, fmt.Errorf("delegation：讀取代行：%w", err)
	}
	return s, nil
}

func (m *Module) Session(ctx context.Context, id int64) (model.DelegationSession, error) {
	row, err := delegationSessionRow(ctx, m.q(ctx), id)
	if err != nil {
		return model.DelegationSession{}, err
	}
	return toDelegationSession(row, m.d.Now()), nil
}

func (m *Module) SessionForAudit(ctx context.Context, in model.AuditSessionInput) (model.DelegationSession, error) {
	row, err := delegationSessionRow(ctx, m.q(ctx), in.ID)
	if err != nil {
		return model.DelegationSession{}, err
	}
	s := toDelegationSession(row, in.Now)
	switch s.Status {
	case model.DelegationRevoked:
		return s, model.Deny(model.DenialSessionRevoked, "代行工作階段已被撤銷")
	case model.DelegationExpired:
		if in.Origin != model.AuditScheduled {
			return s, model.Deny(model.DenialSessionExpired, "代行工作階段已逾時")
		}
	case model.DelegationActive:
	}
	return s, nil
}

func (m *Module) ActiveSessionsOfStore(ctx context.Context, storeID int64, now time.Time) (int64, error) {
	return m.q(ctx).CountActiveDelegationSessionsOfStore(ctx, store.CountActiveDelegationSessionsOfStoreParams{
		TargetStoreID: storeID, Now: now, IdleSince: model.IdleSince(now),
	})
}

// 結束摘要的「履約、金流」計數以子字串比對動作（設計書 56 §8.1.0 的禁區清單）：要偵測的是只有封鎖被打穿之後才會出現
// 的動作，名稱今天不存在，列舉登錄的 19 個動作只會得到恆為零的計數。
var (
	fulfillmentAreaTerms = []string{"fulfillment", "shipment", "cancel"}
	financeAreaTerms     = []string{"refund", "payment", "finance"}
)

func likePatterns(terms []string) []string {
	out := make([]string, len(terms))
	for i, t := range terms {
		out[i] = "%" + t + "%"
	}
	return out
}

// DelegationSessionSummary 回傳結束摘要（「結束本次代行？」的對話框；以 ID 查任意 session 與以 credential 查自己的
// session 共用這一個）。
func (m *Module) DelegationSessionSummary(ctx context.Context, id int64) (model.DelegationSummary, error) {
	q := m.q(ctx)
	s, err := delegationSessionRow(ctx, q, id)
	if err != nil {
		return model.DelegationSummary{}, err
	}
	c, err := q.DelegationAuditSummary(ctx, store.DelegationAuditSummaryParams{
		FulfillmentPatterns: likePatterns(fulfillmentAreaTerms), FinancePatterns: likePatterns(financeAreaTerms), SessionID: id,
	})
	if err != nil {
		return model.DelegationSummary{}, fmt.Errorf("delegation：計算代行摘要：%w", err)
	}
	now := m.d.Now()
	// 三個候選取最早的：撤銷或主動結束的時刻、閒置逾時時的最後活動、hard 到期。都沒有時是現在（還在進行）。
	end := now
	if s.EndedAt != nil && s.EndedAt.Before(end) {
		end = *s.EndedAt
	}
	if idleExpired(s, now) && s.LastActiveAt.Before(end) {
		end = s.LastActiveAt
	}
	if s.ExpiresAt.Before(end) {
		end = s.ExpiresAt
	}
	duration := max(end.Sub(s.IssuedAt).Truncate(time.Second), 0)
	session := toDelegationSession(s, now)
	if s.RevokedByAdminID != nil {
		admins, err := m.d.Identity.Admins(ctx, []int64{*s.RevokedByAdminID})
		if err != nil {
			return model.DelegationSummary{}, fmt.Errorf("delegation：讀取撤銷的管理員：%w", err)
		}
		session.RevokedByDisplayName = revokerDisplayName(admins, s.RevokedByAdminID)
	}
	return model.DelegationSummary{
		Session:                session,
		Duration:               duration,
		OperationCount:         c.SucceededCount,
		RiskOperationCount:     c.HighRiskCount,
		PendingAuditCount:      c.PendingCount,
		NotifiedSeller:         c.NotifiedCount > 0,
		FulfillmentActionCount: c.FulfillmentCount,
		FinanceActionCount:     c.FinanceCount,
		AuditRetentionDays:     model.DelegationAuditRetentionDays,
	}, nil
}

// DelegationSessions 列出代行（「這段期間誰代行了哪些店」）。整頁的狀態以同一個時刻推導。
func (m *Module) DelegationSessions(ctx context.Context, f model.DelegationSessionFilter, p model.DelegationPaging) (model.DelegationSessionPage, error) {
	switch {
	case f.AdminID != nil && *f.AdminID <= 0:
		return model.DelegationSessionPage{}, model.Deny(model.DenialInvalidCommand, "actorAdminId 不正確")
	case f.StoreID != nil && *f.StoreID <= 0:
		return model.DelegationSessionPage{}, model.Deny(model.DenialInvalidCommand, "targetStoreId 不正確")
	case f.DisplayCode != "" && !validDisplayCode(f.DisplayCode):
		// 打錯的代號回 9260，不是一頁空白（讀起來像「沒有這個代行」）。
		return model.DelegationSessionPage{}, model.Deny(model.DenialInvalidCommand, "displayCode 格式不正確")
	}
	if err := model.ValidateRange(f.From, f.To); err != nil {
		return model.DelegationSessionPage{}, err
	}
	p, err := p.Normalize()
	if err != nil {
		return model.DelegationSessionPage{}, err
	}
	q := m.q(ctx)
	rows, err := q.ListDelegationSessions(ctx, store.ListDelegationSessionsParams{
		ActorAdminID: f.AdminID, TargetStoreID: f.StoreID, DisplayCode: f.DisplayCode, DateFrom: f.From, DateTo: f.To,
		RowOffset: p.Offset(), RowLimit: p.PageSize,
	})
	if err != nil {
		return model.DelegationSessionPage{}, fmt.Errorf("delegation：列出代行：%w", err)
	}
	total, err := q.CountDelegationSessions(ctx, store.CountDelegationSessionsParams{
		ActorAdminID: f.AdminID, TargetStoreID: f.StoreID, DisplayCode: f.DisplayCode, DateFrom: f.From, DateTo: f.To,
	})
	if err != nil {
		return model.DelegationSessionPage{}, fmt.Errorf("delegation：計算代行數：%w", err)
	}
	now := m.d.Now()
	page := model.DelegationSessionPage{Total: total, Paging: p, Sessions: []model.DelegationSession{}}
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		if r.RevokedByAdminID != nil {
			ids = append(ids, *r.RevokedByAdminID)
		}
	}
	admins, err := m.d.Identity.Admins(ctx, ids)
	if err != nil {
		return model.DelegationSessionPage{}, fmt.Errorf("delegation：讀取撤銷的管理員：%w", err)
	}
	for _, r := range rows {
		s := toDelegationSession(r, now)
		s.RevokedByDisplayName = revokerDisplayName(admins, s.RevokedByAdminID)
		page.Sessions = append(page.Sessions, s)
	}
	return page, nil
}

// validDisplayCode 是 display_code 的形狀（DB 的 delegated_operation_sessions_display_code_check）。
func validDisplayCode(s string) bool {
	if len(s) != 17 || s[:4] != "OPS-" {
		return false
	}
	for i := 4; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

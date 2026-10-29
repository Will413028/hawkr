package measure

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/suspension"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Revoke admin 撤銷措施（原因要是管理員能選的 ManualRevokeReasonValues，由模組自己檢查，不依賴傳輸層的 AdminRevokeReason；接受申覆只由裁定申覆寫入，連帶撤銷只由連帶撤銷寫入）；版本要與
// 讀到的相符（不符回 VersionConflict，先於狀態檢查，撤銷已經生效後帶舊版本再撤銷一樣回 VersionConflict，不寫入）。申覆待裁定時
// 要先裁定。RevokeFindings 為 true 時一併撤銷這個措施自己建立的認定，並連帶撤銷引用那些認定的其他措施（只限依據認定的措施）。
// 同一個 transaction 解除措施的效果、重算停權狀態，發出 MeasureRevoked。撤銷終止合作時，它關閉的店家在申覆期間內於同一個 transaction 恢復營業，
// 以它為來源的關聯終止一併撤銷（見 revoke）。
func (s *Service) Revoke(ctx context.Context, in model.RevokeMeasureInput) (model.Measure, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.Measure{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Measure{}, errExpectedVersion()
	}
	if err := model.OneOf(in.Reason, model.ManualRevokeReasonValues(), "Reason", "撤銷原因不合法；接受申覆請以裁定申覆撤銷"); err != nil {
		return model.Measure{}, err
	}
	var result model.Measure
	err := s.retryScope(ctx, func() error {
		return s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			now := s.now()
			scope, row, err := s.lockForRevoke(ctx, q, in.MeasureID, in.RevokeFindings)
			if err != nil {
				return err
			}
			if row.Version != in.ExpectedVersion {
				return errMeasureVersion()
			}
			current, err := s.readRow(ctx, q, row, detailList, now)
			if err != nil {
				return err
			}
			if !current.CanRevoke() {
				return errActionUnavailable(current, model.MeasureActionRevoke)
			}
			if in.RevokeFindings && !current.CanRevokeFindings() {
				return apperr.InvalidInputOn([]string{"RevokeFindings"}, "這個措施沒有可以撤銷的認定（只有依據認定、而且自己建立了尚未撤銷的認定的措施可以一併撤銷認定）")
			}
			plan, err := s.planRevoke(ctx, q, scope, row, in.RevokeFindings, now)
			if err != nil {
				return err
			}
			revoked, err := s.revoke(ctx, q, plan, row, in.AdminID, in.Reason, now)
			if err != nil {
				return err
			}
			result, err = s.readRow(ctx, q, revoked, detailAdmin, now)
			return err
		})
	})
	return result, err
}

// errScopeChanged 是撤銷或賣家請求要鎖的帳號在不加鎖的預讀與鎖住之間變了（剛好有新的關聯終止或引用）：整個 transaction 回滾重來。
var errScopeChanged = errors.New("seller/measure：要鎖的帳號範圍變了")

// retryScope 在 errScopeChanged 時把整個 transaction 重跑（每次重新預讀、依升序鎖住），有限次數；仍然變動就回 Unavailable 請呼叫方重試。
func (s *Service) retryScope(ctx context.Context, run func() error) error {
	var err error
	for range 5 {
		if err = run(); !errors.Is(err, errScopeChanged) {
			return err
		}
	}
	return apperr.Unavailable("%s", text.MeasureRetryExhausted(uitext.LocaleFrom(ctx)))
}

// revokeScope 是撤銷一筆措施會碰到的帳號與鎖：措施自己的帳號、以它（與連帶撤銷的措施）為來源的關聯終止所在的帳號。涉及終止合作時帳號列也要先鎖
// （恢復營業要授予 SELLER，identity 的帳號列鎖要在店家列之前）。
type revokeScope struct {
	accounts []int64
	identity bool
}

func (sc revokeScope) covers(accountID int64) bool { return slices.Contains(sc.accounts, accountID) }

// lockForRevoke 依固定的順序鎖住撤銷要碰的帳號：identity 帳號列（涉及終止合作時）依 account_id 升序，再鎖這些帳號全部的 stint（升序），最後鎖措施列
// （鎖順序：帳號列、店家列、措施列；跨帳號時一律升序，與關聯終止的建立不會互鎖）。要碰哪些帳號在鎖之前先不加鎖讀一次（措施、它的連帶對象和關聯終止），
// 鎖住之後 planRevoke 重讀並確認沒有超出已鎖的範圍，超出（讀與鎖之間剛好有新的關聯終止或引用）回 Unavailable 請重試。
func (s *Service) lockForRevoke(ctx context.Context, q *store.Queries, measureID uuid.UUID, revokeFindings bool) (revokeScope, store.SellerMeasure, error) {
	current, err := q.MeasureByID(ctx, measureID)
	if errors.Is(err, pgx.ErrNoRows) {
		return revokeScope{}, current, errMeasureNotFound()
	}
	if err != nil {
		return revokeScope{}, current, err
	}
	candidates := []store.SellerMeasure{current}
	if revokeFindings {
		cascaded, err := s.citingMeasures(ctx, q, current)
		if err != nil {
			return revokeScope{}, current, err
		}
		candidates = append(candidates, cascaded...)
	}
	scope := revokeScope{accounts: []int64{current.AccountID}}
	var sources []uuid.UUID
	for _, m := range candidates {
		if m.Kind == string(model.MeasureTermination) {
			scope.identity = true
			sources = append(sources, m.ID)
		}
	}
	if len(sources) > 0 {
		linked, err := q.LinkedMeasureAccounts(ctx, sources)
		if err != nil {
			return revokeScope{}, current, err
		}
		scope.accounts = append(scope.accounts, linked...)
	}
	slices.Sort(scope.accounts)
	scope.accounts = slices.Compact(scope.accounts)
	if scope.identity {
		for _, id := range scope.accounts {
			if err := s.d.Identity.LockAccount(ctx, id); err != nil {
				return revokeScope{}, current, err
			}
		}
	}
	if err := stint.LockAccounts(ctx, q, scope.accounts); err != nil {
		return revokeScope{}, current, err
	}
	row, err := q.LockMeasure(ctx, measureID)
	return scope, row, err
}

// citingMeasures 是撤銷 origin 並一併撤銷認定時會連帶撤銷的措施：引用 origin 自己建立、仍生效的認定的其他未撤銷措施（同一個帳號：認定屬於帳號）。
func (s *Service) citingMeasures(ctx context.Context, q *store.Queries, origin store.SellerMeasure) ([]store.SellerMeasure, error) {
	owned, err := q.OriginFindings(ctx, origin.ID)
	if err != nil || len(owned) == 0 {
		return nil, err
	}
	findingIDs := make([]uuid.UUID, len(owned))
	for i, f := range owned {
		findingIDs[i] = f.ID
	}
	return q.MeasuresCitingFindings(ctx, store.MeasuresCitingFindingsParams{ViolationIds: findingIDs, ExceptID: origin.ID})
}

// revokePlan 是一次撤銷要做的全部事，在改動任何東西之前決定好（含恢復營業與否：恢復與否要看撤銷之前的資料，否則同一個 transaction 裡結案的待處理申覆
// 會讓 legal hold 在裁定的那一刻消失）：自己建立的認定、連帶撤銷的措施（引用那些認定，FINDING_REVOKED）、關聯終止（以撤銷的終止為來源，
// SOURCE_REVOKED），以及哪些措施要恢復它們關閉的店家。
type revokePlan struct {
	findings []uuid.UUID
	cascaded []store.SellerMeasure
	linked   []store.SellerMeasure
	reopen   []int64
	accounts []int64
}

// planRevoke 只讀（措施列與關聯終止的列在這裡鎖住）：措施與帳號都已由 lockForRevoke 鎖住。
func (s *Service) planRevoke(ctx context.Context, q *store.Queries, scope revokeScope, row store.SellerMeasure, revokeFindings bool, now time.Time) (revokePlan, error) {
	var plan revokePlan
	if revokeFindings {
		owned, err := q.OriginFindings(ctx, row.ID)
		if err != nil {
			return plan, err
		}
		for _, f := range owned {
			plan.findings = append(plan.findings, f.ID)
		}
		if plan.cascaded, err = s.citingMeasures(ctx, q, row); err != nil {
			return plan, err
		}
	}
	revoking := append([]store.SellerMeasure{row}, plan.cascaded...)
	var sources []uuid.UUID
	for _, m := range revoking {
		if m.Kind == string(model.MeasureTermination) {
			sources = append(sources, m.ID)
		}
	}
	if len(sources) > 0 {
		linked, err := q.LockLinkedMeasures(ctx, sources)
		if err != nil {
			return plan, err
		}
		plan.linked = linked
	}
	plan.accounts = []int64{row.AccountID}
	for _, m := range append(revoking, plan.linked...) {
		if !scope.covers(m.AccountID) || m.Kind == string(model.MeasureTermination) && !scope.identity {
			return plan, errScopeChanged
		}
		plan.accounts = append(plan.accounts, m.AccountID)
	}
	slices.Sort(plan.accounts)
	plan.accounts = slices.Compact(plan.accounts)
	all := append(revoking, plan.linked...)
	revokingIDs := measureIDs(all)
	planned := map[int64]bool{}
	for _, m := range all {
		if m.Kind != string(model.MeasureTermination) || planned[m.AccountID] {
			continue
		}
		planned[m.AccountID] = true
		storeID, err := s.d.Closing.ReopenPlan(ctx, q, m.AccountID, revokingIDs, now)
		if err != nil {
			return plan, err
		}
		if storeID != 0 {
			plan.reopen = append(plan.reopen, storeID)
		}
	}
	return plan, nil
}

// revoke 是撤銷措施在同一個 transaction 要做的事（Revoke 與接受申覆共用）：撤銷措施、要求時撤銷自己建立的認定並連帶撤銷引用它們的其他措施
// （FINDING_REVOKED）、連帶撤銷以撤銷的終止為來源的關聯終止（SOURCE_REVOKED，與來源同一個 transaction）、把它們待處理的請求結案、恢復它們在
// 申覆期間內關閉的店家（plan.reopen）、重算每個受影響帳號的停權狀態（每個帳號只重算一次）。呼叫方已鎖住帳號與措施列，且已確認版本與狀態。
func (s *Service) revoke(ctx context.Context, q *store.Queries, plan revokePlan, row store.SellerMeasure, adminID int64, reason model.RevokeReason, now time.Time) (store.SellerMeasure, error) {
	revoked, err := q.RevokeMeasure(ctx, store.RevokeMeasureParams{ID: row.ID, ExpectedVersion: row.Version, Reason: string(reason), AdminID: adminID, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return revoked, errMeasureVersion()
	}
	if err != nil {
		return revoked, err
	}
	affected := []store.SellerMeasure{revoked}
	if err := s.publishRevoked(ctx, q, revoked, reason, now); err != nil {
		return revoked, err
	}
	if len(plan.findings) > 0 {
		if _, err := q.RevokeViolations(ctx, store.RevokeViolationsParams{Ids: plan.findings, Reason: string(reason), AdminID: adminID, Now: now}); err != nil {
			return revoked, err
		}
	}
	for _, cascade := range []struct {
		measures []store.SellerMeasure
		reason   model.RevokeReason
	}{{plan.cascaded, model.RevokeFindingRevoked}, {plan.linked, model.RevokeSourceRevoked}} {
		done, err := s.cascadeRevoke(ctx, q, cascade.measures, cascade.reason, adminID, now)
		if err != nil {
			return revoked, err
		}
		affected = append(affected, done...)
	}
	if err := s.closePendingOfRevoked(ctx, q, affected, adminID, now); err != nil {
		return revoked, err
	}
	for _, storeID := range plan.reopen {
		if err := s.d.Closing.Reopen(ctx, q, storeID, now); err != nil {
			return revoked, err
		}
	}
	for _, accountID := range plan.accounts {
		if err := suspension.Recompute(ctx, q, s.d.Events, s.d.Jobs, s.d.Exposure, accountID, now); err != nil {
			return revoked, err
		}
	}
	return revoked, nil
}

// cascadeRevoke 連帶撤銷這些措施（原因 reason，作者是動作的管理員），每個發 MeasureRevoked；回傳實際撤銷的。
func (s *Service) cascadeRevoke(ctx context.Context, q *store.Queries, measures []store.SellerMeasure, reason model.RevokeReason, adminID int64, now time.Time) ([]store.SellerMeasure, error) {
	if len(measures) == 0 {
		return nil, nil
	}
	cascaded, err := q.CascadeRevokeMeasures(ctx, store.CascadeRevokeMeasuresParams{Ids: measureIDs(measures), Reason: string(reason), AdminID: adminID, Now: now})
	if err != nil {
		return nil, err
	}
	for _, m := range cascaded {
		if err := s.publishRevoked(ctx, q, m, reason, now); err != nil {
			return nil, err
		}
	}
	return cascaded, nil
}

// closePendingOfRevoked 把剛撤銷的這些措施上待處理的請求以 CLOSED（REVOKED，帶執行撤銷的管理員）結案，每筆在同一個 transaction 發
// MeasureRequestClosed。
func (s *Service) closePendingOfRevoked(ctx context.Context, q *store.Queries, measures []store.SellerMeasure, adminID int64, now time.Time) error {
	closed, err := q.CloseRequests(ctx, store.CloseRequestsParams{MeasureIds: measureIDs(measures), AdminID: adminID, Now: now})
	if err != nil {
		return err
	}
	return s.publishClosed(ctx, measures, closed, model.CloseCauseRevoked, now)
}

// closeEndedRemediations 把這些措施（已結束）上失去標的的待處理解除請求以 CLOSED（ENDED，系統結案沒有管理員）結案，結案時間是措施的結束時間
// （model.Measure.EffectiveRequest 的輸出在結案前後相同），每筆在同一個 transaction 發 MeasureRequestClosed；回傳結案的請求。
func (s *Service) closeEndedRemediations(ctx context.Context, q *store.Queries, measures []store.SellerMeasure, now time.Time) ([]store.SellerMeasureRequest, error) {
	closed, err := q.CloseEndedRemediations(ctx, measureIDs(measures))
	if err != nil {
		return nil, err
	}
	return closed, s.publishClosed(ctx, measures, closed, model.CloseCauseEnded, now)
}

func measureIDs(measures []store.SellerMeasure) []uuid.UUID {
	ids := make([]uuid.UUID, len(measures))
	for i, m := range measures {
		ids[i] = m.ID
	}
	return ids
}

func (s *Service) publishClosed(ctx context.Context, measures []store.SellerMeasure, closed []store.SellerMeasureRequest, cause model.RequestCloseCause, now time.Time) error {
	owners := make(map[uuid.UUID]store.SellerMeasure, len(measures))
	for _, m := range measures {
		owners[m.ID] = m
	}
	for _, request := range closed {
		owner := owners[request.MeasureID]
		if err := events.Publish(ctx, s.d.Events, model.MeasureRequestClosed{
			EventID: uuid.NewV7(), OccurredAt: now, MeasureID: owner.ID, RequestID: request.ID, Kind: model.RequestKind(request.Kind),
			Cause: cause, StoreID: owner.StoreID, AccountID: owner.AccountID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) publishRevoked(ctx context.Context, q *store.Queries, row store.SellerMeasure, reason model.RevokeReason, now time.Time) error {
	event := model.MeasureRevoked{
		EventID: uuid.NewV7(), OccurredAt: now, MeasureID: row.ID, StoreID: row.StoreID, AccountID: row.AccountID, Reason: reason,
	}
	if row.Kind == string(model.MeasureContentTakedown) {
		contents, err := q.MeasureContents(ctx, []uuid.UUID{row.ID})
		if err != nil {
			return err
		}
		for _, c := range contents {
			event.Contents = append(event.Contents, model.ContentRef{Kind: model.ContentKind(c.ContentKind), ID: c.ContentID})
		}
	}
	return events.Publish(ctx, s.d.Events, event)
}

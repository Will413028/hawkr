package measure

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// linkedAccounts 是措施的身分快照反查：與這筆措施的帳號使用同一個身分證明文件或統一編號的其他帳號（W9）。只有留有身分快照的措施有結果（停權與非預告
// 的終止合作；快照在措施撤銷或結束時刪除），其他措施沒有。命中的判斷與建立關聯終止時伺服器的再驗證是同一支比對（LinkedIdentities.LinkedAccountHits）；每一列的 CanCreateLinkedTermination 與建立時的前置條件是同一組 model 規則
// （目標店家是帳號目前的店家，沒有時是最近的 stint，所以一定是帳號最近的一個）。
func (s *Service) linkedAccounts(ctx context.Context, q *store.Queries, m store.SellerMeasure) ([]model.LinkedAccount, error) {
	if m.Status != string(model.MeasureStatusEnforced) || !model.MeasureKind(m.Kind).Restrictive() {
		return nil, nil
	}
	hits, err := s.d.Linked.LinkedAccountHits(ctx, q, m.ID, m.AccountID, nil)
	if err != nil || len(hits) == 0 {
		return nil, err
	}
	matched := map[int64][]model.LinkedIdentity{}
	var accounts []int64
	for _, h := range hits {
		if _, ok := matched[h.AccountID]; !ok {
			accounts = append(accounts, h.AccountID)
		}
		matched[h.AccountID] = append(matched[h.AccountID], h.Identity)
	}
	linked, err := q.LinkedMeasuresOf(ctx, &m.ID)
	if err != nil {
		return nil, err
	}
	linkedMeasure := make(map[int64]uuid.UUID, len(linked))
	for _, l := range linked {
		linkedMeasure[l.AccountID] = l.ID
	}
	slices.Sort(accounts)
	out := make([]model.LinkedAccount, 0, len(accounts))
	for _, accountID := range accounts {
		target, err := q.CurrentStore(ctx, accountID)
		if errors.Is(err, pgx.ErrNoRows) {
			target, err = q.LatestStoreOfAccount(ctx, accountID)
		}
		if err != nil {
			return nil, err
		}
		entry := model.LinkedAccount{AccountID: accountID, StoreID: target.StoreID, StoreCode: target.StoreCode, Matched: firstMatched(matched[accountID])}
		id, alreadyLinked := linkedMeasure[accountID]
		if alreadyLinked {
			entry.LinkedMeasureID = &id
		}
		entry.CanCreateLinkedTermination = model.CanCreateLinkedTermination(
			model.LinkedSourceOf(m.Status, m.Kind, m.TerminationGround, m.AccountID), accountID, alreadyLinked,
			model.TerminationTarget{Closed: target.Status == string(model.Closed), LatestOfAccount: true},
		)
		if entry.CanCreateLinkedTermination {
			effect := model.NewTerminationEffect(model.GroundLinkedAccount, s.now())
			entry.TerminationEffect = &effect
		}
		out = append(out, entry)
	}
	return out, nil
}

// firstMatched 在同時命中多種身分時取 LinkedIdentityValues 的第一種。
func firstMatched(found []model.LinkedIdentity) model.LinkedIdentity {
	for _, kind := range model.LinkedIdentityValues() {
		if slices.Contains(found, kind) {
			return kind
		}
	}
	return ""
}

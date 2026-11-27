package measure

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

func (s *Service) linkedAccounts(ctx context.Context, q *store.Queries, m store.SellerMeasure) ([]model.LinkedAccount, error) {
	if m.Status != string(model.MeasureStatusEnforced) || !model.MeasureKind(m.Kind).Restrictive() {
		return nil, nil
	}
	hits, err := s.d.Linked.LinkedAccountHits(ctx, q, s.now(), m.ID, m.AccountID, nil)
	if err != nil || len(hits) == 0 {
		return nil, err
	}
	matched := map[int64][]model.LinkedIdentity{}
	ground := map[int64]bool{}
	var accounts []int64
	for _, h := range hits {
		if _, ok := matched[h.AccountID]; !ok {
			accounts = append(accounts, h.AccountID)
		}
		matched[h.AccountID] = append(matched[h.AccountID], h.Identity)
		ground[h.AccountID] = ground[h.AccountID] || h.Identity.CountsAsGround()
	}
	linked, err := q.LinkedMeasuresOf(ctx, &m.ID)
	if err != nil {
		return nil, err
	}
	linkedMeasure := make(map[int64]uuid.UUID, len(linked))
	for _, l := range linked {
		linkedMeasure[l.AccountID] = l.ID
	}
	pendingRows, err := q.SharedBankAccountPendingAccounts(ctx, store.SharedBankAccountPendingAccountsParams{At: s.now(), MeasureID: m.ID})
	if err != nil {
		return nil, err
	}
	pending := make(map[int64]bool, len(pendingRows))
	for _, id := range pendingRows {
		pending[id] = true
	}
	reviewRows, err := q.MeasureLinkedReviews(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	reviewedAt := make(map[int64]time.Time, len(reviewRows))
	for _, r := range reviewRows {
		reviewedAt[r.AccountID] = r.ReviewedAt
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
		entry.SharedBankAccountPending = pending[accountID]
		if at, ok := reviewedAt[accountID]; ok {
			entry.ReviewedAt = &at
		}
		id, alreadyLinked := linkedMeasure[accountID]
		if alreadyLinked {
			entry.LinkedMeasureID = &id
		}
		entry.CanCreateLinkedTermination = model.CanCreateLinkedTermination(
			model.LinkedSourceOf(m.Status, m.Kind, m.TerminationGround, m.AccountID), accountID, ground[accountID], alreadyLinked,
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

func firstMatched(found []model.LinkedIdentity) model.LinkedIdentity {
	for _, kind := range model.LinkedIdentityValues() {
		if slices.Contains(found, kind) {
			return kind
		}
	}
	return ""
}

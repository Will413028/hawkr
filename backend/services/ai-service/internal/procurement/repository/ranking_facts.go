package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	// seller_ranking_facts/seller_ranking_processed_events are declared in
	// db/schema/schema.sql (the general ai-service schema), not
	// db/schema/procurement.sql — Task 10 put them there deliberately (子系統 1,
	// a local ranking cache ai-service owns, not a procurement-domain table).
	// sqlc's procurement entry (db/sqlc.yaml) only scans procurement.sql, so it
	// has no model for this table; the queries (db/queries/seller_ranking_facts.sql,
	// top-level — same reason) compile through the general entry into this
	// package instead. Both entries' pools point at the same hawkr_ai
	// database (see newProcurementPool's comment in module.go), so querying
	// this table through procurement's own pool is safe at runtime — this is
	// purely a Go-level code-generation boundary, not a connectivity one.
	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// RankingFactsRepository is procurement's read/write port onto
// seller_ranking_facts — consumed by candidate_platform_lane.go (Task 14) to
// rank/cap PLATFORM-lane candidates by seller membership tier.
type RankingFactsRepository interface {
	BatchGet(ctx context.Context, storeIDs []identity.StoreID) (map[identity.StoreID]domain.SellerRankingFact, error)
	// UpsertOrderCompleted claims eventID for idempotency (returns immediately,
	// no-op, if already claimed) then increments/resets the store's monthly
	// count in one transaction.
	UpsertOrderCompleted(ctx context.Context, storeID identity.StoreID, period, eventID string) error
	// UpsertTier only applies when effectiveAt is not older than the value
	// already stored for this store (or nothing is stored yet) — a guard
	// against RabbitMQ retry/requeue delivering an older
	// seller.membership_changed event after a newer one already landed.
	UpsertTier(ctx context.Context, storeID identity.StoreID, tierRank, monthlyOrderCap int, effectiveAt time.Time) error
}

type rankingFactsRepository struct {
	pool      *pgxpool.Pool
	txManager database.TxManager
}

func NewRankingFactsRepository(pool *pgxpool.Pool, txManager database.TxManager) RankingFactsRepository {
	return &rankingFactsRepository{pool: pool, txManager: txManager}
}

func (r *rankingFactsRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, r.pool))
}

func (r *rankingFactsRepository) BatchGet(ctx context.Context, storeIDs []identity.StoreID) (map[identity.StoreID]domain.SellerRankingFact, error) {
	ids := make([]int64, len(storeIDs))
	for i, id := range storeIDs {
		ids[i] = int64(id)
	}
	rows, err := r.q(ctx).BatchGetSellerRankingFacts(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[identity.StoreID]domain.SellerRankingFact, len(rows))
	for _, row := range rows {
		fact := domain.SellerRankingFact{
			StoreID:                identity.StoreID(row.StoreID),
			MonthlyCompletedOrders: int(row.MonthlyCompletedOrders),
			// count_period 是 NOT NULL 的 text，sqlc 直接給 string；讀端要靠它
			// 判斷計數是否還屬於當月（見 domain.SellerRankingFact.CountPeriod）。
			CountPeriod: row.CountPeriod,
		}
		// sqlc emits plain Go pointers for nullable columns here
		// (sqlc.yaml's emit_pointers_for_null_types: true), not pgtype.Int4 or
		// sql.NullInt32 — verified from the generated
		// internal/repository/postgres/gen/seller_ranking_facts.sql.go /
		// models.go after running `make sqlc`.
		if row.TierRank != nil {
			fact.TierRank = int(*row.TierRank)
		}
		if row.MonthlyOrderCap != nil {
			cap := int(*row.MonthlyOrderCap)
			fact.MonthlyOrderCap = &cap
		}
		out[fact.StoreID] = fact
	}
	return out, nil
}

func (r *rankingFactsRepository) UpsertOrderCompleted(ctx context.Context, storeID identity.StoreID, period, eventID string) error {
	return r.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := r.q(txCtx)
		claimed, err := q.ClaimSellerRankingProcessedEvent(txCtx, eventID)
		if err != nil {
			return err
		}
		if claimed == 0 {
			return nil // already processed this event, no-op
		}
		return q.UpsertSellerRankingOrderCount(txCtx, gen.UpsertSellerRankingOrderCountParams{
			StoreID: int64(storeID), CountPeriod: period,
		})
	})
}

func (r *rankingFactsRepository) UpsertTier(ctx context.Context, storeID identity.StoreID, tierRank, monthlyOrderCap int, effectiveAt time.Time) error {
	tr := int32(tierRank)
	cap := int32(monthlyOrderCap)
	return r.q(ctx).UpsertSellerRankingTier(ctx, gen.UpsertSellerRankingTierParams{
		StoreID: int64(storeID), TierRank: &tr, MonthlyOrderCap: &cap, TierEffectiveAt: &effectiveAt,
	})
}

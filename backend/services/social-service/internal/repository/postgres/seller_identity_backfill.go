package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SingleStintSellerMapping map[int64]int64

type SellerIdentityBackfillResult struct {
	CmsCommentUserID     int64
	CmsSellerForumUserID int64
	ReportsReporterID    int64
	UserBlocksBlockerID  int64
}

// Total sums all counters — a quick "did anything happen" check.
func (r SellerIdentityBackfillResult) Total() int64 {
	return r.CmsCommentUserID +
		r.CmsSellerForumUserID + r.ReportsReporterID + r.UserBlocksBlockerID
}

func BackfillSingleStintSellerIdentity(ctx context.Context, pool *pgxpool.Pool, mapping SingleStintSellerMapping) (SellerIdentityBackfillResult, error) {
	var result SellerIdentityBackfillResult
	if len(mapping) == 0 {
		return result, nil
	}

	accountIDs := make([]int64, 0, len(mapping))
	storeIDs := make([]int64, 0, len(mapping))
	for accountID, storeID := range mapping {
		if accountID == storeID {
			continue // nothing to rekey — grandfathered account, already its own store_id
		}
		accountIDs = append(accountIDs, accountID)
		storeIDs = append(storeIDs, storeID)
	}
	if len(accountIDs) == 0 {
		return result, nil
	}

	var err error

	if result.CmsCommentUserID, err = execRekeyUpdate(ctx, pool,
		`UPDATE cms_comment AS t
		 SET user_id = m.store_id
		 FROM unnest($1::bigint[], $2::bigint[]) AS m(account_id, store_id)
		 WHERE t.user_id = m.account_id`,
		accountIDs, storeIDs); err != nil {
		return result, fmt.Errorf("backfill cms_comment.user_id: %w", err)
	}

	if result.CmsSellerForumUserID, err = execRekeyUpdate(ctx, pool,
		`UPDATE cms_seller_forum AS t
		 SET user_id = m.store_id
		 FROM unnest($1::bigint[], $2::bigint[]) AS m(account_id, store_id)
		 WHERE t.user_id = m.account_id`,
		accountIDs, storeIDs); err != nil {
		return result, fmt.Errorf("backfill cms_seller_forum.user_id: %w", err)
	}

	if result.ReportsReporterID, err = execRekeyUpdate(ctx, pool,
		`UPDATE reports AS t
		 SET reporter_id = m.store_id
		 FROM unnest($1::bigint[], $2::bigint[]) AS m(account_id, store_id)
		 WHERE t.reporter_id = m.account_id`,
		accountIDs, storeIDs); err != nil {
		return result, fmt.Errorf("backfill reports.reporter_id: %w", err)
	}

	// user_blocks: UNIQUE (blocker_id, blocked_id) + CHECK blocker_id <>
	// blocked_id — guard against both a colliding row and a self-block.
	if result.UserBlocksBlockerID, err = execRekeyUpdate(ctx, pool,
		`UPDATE user_blocks AS t
		 SET blocker_id = m.store_id
		 FROM unnest($1::bigint[], $2::bigint[]) AS m(account_id, store_id)
		 WHERE t.blocker_id = m.account_id
		   AND m.store_id <> t.blocked_id
		   AND NOT EXISTS (
		     SELECT 1 FROM user_blocks x
		     WHERE x.blocked_id = t.blocked_id AND x.blocker_id = m.store_id
		   )`,
		accountIDs, storeIDs); err != nil {
		return result, fmt.Errorf("backfill user_blocks.blocker_id: %w", err)
	}

	return result, nil
}

func execRekeyUpdate(ctx context.Context, pool *pgxpool.Pool, sql string, accountIDs, storeIDs []int64) (int64, error) {
	tag, err := pool.Exec(ctx, sql, accountIDs, storeIDs)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

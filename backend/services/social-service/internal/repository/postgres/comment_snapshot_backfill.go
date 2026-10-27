package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BrokenCommentSnapshot struct {
	UserName   string
	UserAvatar string
}

// ListBrokenSellerCommentUserIDs returns the distinct user_id values with at
// least one seller-authored (role) comment/reply whose display snapshot is
// NULL. cms_comment.user_id itself is left untouched by this package — it's
// a separate ownership field (see cmd/backfill-seller-content-identity),
// unrelated to the display bug this backfill fixes.
func ListBrokenSellerCommentUserIDs(ctx context.Context, pool *pgxpool.Pool, role int16) ([]int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT user_id FROM cms_comment
		WHERE role = $1 AND deleted_at IS NULL
		  AND (user_name IS NULL OR user_avatar IS NULL)`,
		role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// BackfillCommentSnapshots writes the resolved name/avatar for every user_id
// in mapping back onto their still-broken cms_comment rows, in one
// statement.
//
// Idempotent: a row already holding non-NULL user_name AND user_avatar
// matches nothing on a later pass. Callers must omit any user_id that failed
// to resolve to a genuine (non-empty) name from mapping rather than writing
// an empty string — an empty string is NOT NULL, so it would stop matching
// this UPDATE's WHERE clause on a future retry while still displaying as
// blank, i.e. permanently un-retryable instead of just "not fixed yet".
func BackfillCommentSnapshots(ctx context.Context, pool *pgxpool.Pool, role int16, mapping map[int64]BrokenCommentSnapshot) (int64, error) {
	if len(mapping) == 0 {
		return 0, nil
	}

	userIDs := make([]int64, 0, len(mapping))
	// *string, not string: an empty value must round-trip as SQL NULL, not
	// '', to match CreateComment's own pgtype.Text{Valid: x != ""} write
	// path (see comment_repository.go) — a real seller with no logo
	// uploaded yet is expected to have a NULL avatar even on a fresh,
	// correctly-written comment, so this backfill must reproduce that
	// exact "blank means NULL" convention rather than introducing a
	// second, inconsistent representation of "no avatar".
	names := make([]*string, 0, len(mapping))
	avatars := make([]*string, 0, len(mapping))
	for userID, snap := range mapping {
		userIDs = append(userIDs, userID)
		names = append(names, nullIfEmpty(snap.UserName))
		avatars = append(avatars, nullIfEmpty(snap.UserAvatar))
	}

	tag, err := pool.Exec(ctx, `
		UPDATE cms_comment AS t
		SET user_name = m.user_name, user_avatar = m.user_avatar, update_time = NOW()
		FROM unnest($1::bigint[], $2::text[], $3::text[]) AS m(user_id, user_name, user_avatar)
		WHERE t.user_id = m.user_id AND t.role = $4 AND t.deleted_at IS NULL
		  AND (t.user_name IS NULL OR t.user_avatar IS NULL)`,
		userIDs, names, avatars, role)
	if err != nil {
		return 0, fmt.Errorf("backfill cms_comment snapshot: %w", err)
	}
	return tag.RowsAffected(), nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

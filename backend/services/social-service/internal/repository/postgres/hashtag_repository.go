package postgres

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HashtagRepository wraps cms_hashtag + cms_post_hashtag sqlc queries.
// Mutating methods accept gen.DBTX so callers may pass a pgx.Tx for
// transactional flows; nil falls back to the underlying pool.
type HashtagRepository struct {
	pool *pgxpool.Pool
}

func NewHashtagRepository(pool *pgxpool.Pool) *HashtagRepository {
	return &HashtagRepository{pool: pool}
}

// qOn returns a *gen.Queries bound to q if non-nil, otherwise to the pool.
func (r *HashtagRepository) qOn(q gen.DBTX) *gen.Queries {
	if q == nil {
		return gen.New(r.pool)
	}
	return gen.New(q)
}

func (r *HashtagRepository) UpsertByName(ctx context.Context, q gen.DBTX, id int64, canonical string) (int64, error) {
	return r.qOn(q).UpsertHashtagByName(ctx, gen.UpsertHashtagByNameParams{ID: id, Name: canonical})
}

func (r *HashtagRepository) AdjustPostCount(ctx context.Context, q gen.DBTX, hashtagID int64, delta int32) error {
	return r.qOn(q).AdjustHashtagPostCount(ctx, gen.AdjustHashtagPostCountParams{ID: hashtagID, PostCount: delta})
}

func (r *HashtagRepository) InsertBinding(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error {
	return r.qOn(q).InsertPostHashtag(ctx, gen.InsertPostHashtagParams{
		PostID:      postID,
		HashtagID:   hashtagID,
		DisplayName: displayName,
		Position:    position,
	})
}

func (r *HashtagRepository) UpdateBinding(ctx context.Context, q gen.DBTX, postID, hashtagID int64, displayName string, position int16) error {
	return r.qOn(q).UpdatePostHashtagBinding(ctx, gen.UpdatePostHashtagBindingParams{
		PostID:      postID,
		HashtagID:   hashtagID,
		DisplayName: displayName,
		Position:    position,
	})
}

func (r *HashtagRepository) DeleteBindings(ctx context.Context, q gen.DBTX, postID int64, hashtagIDs []int64) error {
	if len(hashtagIDs) == 0 {
		return nil
	}
	return r.qOn(q).DeletePostHashtags(ctx, gen.DeletePostHashtagsParams{PostID: postID, HashtagIds: hashtagIDs})
}

func (r *HashtagRepository) DeleteAllBindings(ctx context.Context, q gen.DBTX, postID int64) error {
	return r.qOn(q).DeleteAllPostHashtags(ctx, postID)
}

func (r *HashtagRepository) ListHashtagIDsByPostID(ctx context.Context, q gen.DBTX, postID int64) ([]int64, error) {
	return r.qOn(q).ListHashtagIDsByPostID(ctx, postID)
}

func (r *HashtagRepository) ListByPostID(ctx context.Context, q gen.DBTX, postID int64) ([]gen.CmsPostHashtag, error) {
	return r.qOn(q).ListPostHashtags(ctx, postID)
}

func (r *HashtagRepository) ListByPostIDs(ctx context.Context, postIDs []int64) (map[int64][]gen.CmsPostHashtag, error) {
	if len(postIDs) == 0 {
		return nil, nil
	}
	rows, err := r.qOn(nil).ListPostHashtagsByPostIDs(ctx, postIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64][]gen.CmsPostHashtag, len(postIDs))
	for _, row := range rows {
		out[row.PostID] = append(out[row.PostID], row)
	}
	return out, nil
}

func (r *HashtagRepository) ListHashtagsByIDs(ctx context.Context, ids []int64) (map[int64]gen.CmsHashtag, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.qOn(nil).ListHashtagsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]gen.CmsHashtag, len(rows))
	for _, h := range rows {
		out[h.ID] = h
	}
	return out, nil
}

func (r *HashtagRepository) SearchByPrefix(ctx context.Context, prefix string, limit int32) ([]gen.SearchHashtagsByPrefixRow, error) {
	return r.qOn(nil).SearchHashtagsByPrefix(ctx, gen.SearchHashtagsByPrefixParams{Prefix: prefix, Lim: limit})
}

func (r *HashtagRepository) SearchByContain(ctx context.Context, query string, limit int32) ([]gen.SearchHashtagsByContainRow, error) {
	return r.qOn(nil).SearchHashtagsByContain(ctx, gen.SearchHashtagsByContainParams{Query: query, Lim: limit})
}

func (r *HashtagRepository) PopularInRange(ctx context.Context, start, end time.Time, limit int32) ([]gen.SelectPopularHashtagsInRangeRow, error) {
	return r.qOn(nil).SelectPopularHashtagsInRange(ctx, gen.SelectPopularHashtagsInRangeParams{
		StartAt:         timestamptzParam(start),
		EndAt:           timestamptzParam(end),
		PublishedStatus: domain.PostPublishStatusPublished,
		Lim:             limit,
	})
}

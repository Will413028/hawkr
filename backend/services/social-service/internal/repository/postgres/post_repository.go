package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostRepository struct {
	q    *gen.Queries
	pool *pgxpool.Pool
}

func NewPostRepository(pool *pgxpool.Pool) *PostRepository {
	return &PostRepository{q: gen.New(pool), pool: pool}
}

func (r *PostRepository) GetByID(ctx context.Context, id int64) (*gen.CmsPost, error) {
	row, err := r.q.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetByIDTx is GetByID bound to q instead of the pool (domain.PostRepository;
// see that interface's doc comment for why it exists). q must not be nil for
// the real UpdatePost call path — the mock in usecase tests is the only
// caller that ever passes nil, and it never touches q at all.
func (r *PostRepository) GetByIDTx(ctx context.Context, q gen.DBTX, id int64) (*gen.CmsPost, error) {
	row, err := gen.New(q).GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *PostRepository) List(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsPost, error) {
	rows, err := r.q.ListPosts(ctx, gen.ListPostsParams{
		Column1:         sellerID,
		Limit:           int32(limit),
		Offset:          int32(offset),
		PublishedStatus: domain.PostPublishStatusPublished,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsPost, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *PostRepository) Count(ctx context.Context, sellerID int64) (int64, error) {
	return r.q.CountPosts(ctx, gen.CountPostsParams{
		Column1:         sellerID,
		PublishedStatus: domain.PostPublishStatusPublished,
	})
}

// noVersionCAS is the sentinel SetPostPublishStatus and SchedulePostPublish
// read as "do not compare versions" — both still have a legitimate nil caller
// (the seller's own publish/unpublish/schedule path). UpdatePost's own query
// no longer honors this sentinel (#2772): passing it there just fails the
// version-equality predicate and comes back as a version conflict instead of
// a silent unconditional write. Safe because chk_cms_post_version_nonneg
// keeps every stored version >= 0, so the sentinel can never match a real row
// on any of the three queries.
const noVersionCAS int64 = -1

// Create 在單一交易內建立貼文並執行 sameTx。
//
// sameTx 是 hashtag 的落點。#2773：原本 hashtag 跑在 INSERT 之後的獨立自動提交裡，
// 於是「API 回失敗但貼文已提交」——使用者看到建立失敗，卻在自己的列表裡看到那篇貼文。
// nil sameTx 表示沒有要一起做的事。
func (r *PostRepository) Create(ctx context.Context, id, sellerID int64, content string,
	imageURL string, imageAssetID *int64, productID *int64, publishStatus int16,
	sameTx func(ctx context.Context, q gen.DBTX) error) (*gen.CmsPost, error) {

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row, err := gen.New(tx).CreatePost(ctx, gen.CreatePostParams{
		ID:              id,
		SellerID:        sellerID,
		Content:         nullableText(content),
		Image:           nullableText(imageURL),
		ProductID:       productID,
		PublishStatus:   publishStatus,
		PublishedStatus: domain.PostPublishStatusPublished,
		ImageAssetID:    imageAssetID,
	})
	if err != nil {
		return nil, err
	}
	if sameTx != nil {
		if err := sameTx(ctx, tx); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &row, nil
}

// Update writes the post body inside its own transaction and runs sameTx —
// the hashtag work's landing point, same shape as Create/SetPublishStatus.
//
// #2772: CAS is mandatory here, not optional. The caller must always pass the
// version it read on entry; rows = 0 is unconditionally a version conflict —
// there is no more "expectedVersion == nil disables the check" branch at this
// layer. Without this gate, two concurrent image swaps both commit, and
// whichever's broad Release lands second deletes the reference the other one
// just linked.
//
// image_asset_id is the row's declared media truth and this is the only
// column this path owns; there is no bookkeeping column to keep in step with
// it. The media reconciler replays every managed row each round, so the next
// round reads the new value and releases the superseded reference — see
// posts.sql's UpdatePost comment.
func (r *PostRepository) Update(ctx context.Context, id int64, content string, imageURL string,
	imageAssetID *int64, productID *int64, expectedVersion *int64,
	sameTx func(ctx context.Context, q gen.DBTX) error) error {

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	params := gen.UpdatePostParams{
		ID:              id,
		Content:         nullableText(content),
		Image:           nullableText(imageURL),
		ImageAssetID:    imageAssetID,
		ProductID:       productID,
		ExpectedVersion: casVersion(expectedVersion),
	}
	rows, err := gen.New(tx).UpdatePost(ctx, params)
	if err != nil {
		return err
	}
	// rows == 0 conflates two causes: a genuine version mismatch, or the row
	// having been soft-deleted between the caller's GetByID and this UPDATE
	// (the query's WHERE also carries deleted_at IS NULL). Both surface as the
	// same 9266 version conflict rather than a distinct not-found — fail-closed
	// and the window is narrow, but it is newly reachable on the seller's own
	// path now that this path always supplies a real expectedVersion.
	if rows == 0 {
		return domain.ErrVersionConflict
	}
	if sameTx != nil {
		if err := sameTx(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// casVersion converts the optional expectedVersion into the query's sentinel.
func casVersion(expectedVersion *int64) int64 {
	if expectedVersion != nil {
		return *expectedVersion
	}
	return noVersionCAS
}

// SetPublishStatus performs the interactive publish / unpublish (56 §6.1.9).
//
// The transaction is opened here rather than left to the caller because sameTx
// must run if and only if the CAS won: hashtag counters follow visibility, and
// moving them for a write that never happened is exactly as wrong as not moving
// them for one that did. Same shape as the scheduler's terminal write.
func (r *PostRepository) SetPublishStatus(
	ctx context.Context,
	id int64,
	publishStatus int16,
	now time.Time,
	expectedVersion *int64,
	sameTx func(ctx context.Context, q gen.DBTX) error,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := gen.New(tx).SetPostPublishStatus(ctx, gen.SetPostPublishStatusParams{
		PublishStatus:   publishStatus,
		PublishedStatus: domain.PostPublishStatusPublished,
		Now:             timestamptzParam(now),
		ID:              id,
		ExpectedVersion: casVersion(expectedVersion),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		if expectedVersion != nil {
			return domain.ErrVersionConflict
		}
		return nil
	}
	if sameTx != nil {
		if err := sameTx(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SchedulePublish writes the pending schedule (56 §7.6.1 T0).
func (r *PostRepository) SchedulePublish(
	ctx context.Context,
	id int64,
	scheduledAt time.Time,
	sessionID int64,
	now time.Time,
	expectedVersion *int64,
) error {
	rows, err := r.q.SchedulePostPublish(ctx, gen.SchedulePostPublishParams{
		ScheduledPublishAt:   timestamptzParam(scheduledAt),
		ScheduledBySessionID: sessionID,
		Now:                  timestamptzParam(now),
		ID:                   id,
		PublishedStatus:      domain.PostPublishStatusPublished,
		ExpectedVersion:      casVersion(expectedVersion),
	})
	if err != nil {
		return err
	}
	if rows == 0 && expectedVersion != nil {
		return domain.ErrVersionConflict
	}
	return nil
}

func (r *PostRepository) SoftDelete(ctx context.Context, id, sellerID int64) (int64, error) {
	return r.q.SoftDeletePost(ctx, gen.SoftDeletePostParams{ID: id, SellerID: sellerID})
}

func (r *PostRepository) IncrViewCount(ctx context.Context, id int64) error {
	return r.q.IncrPostViewCount(ctx, id)
}

func (r *PostRepository) IncrShareCount(ctx context.Context, id int64) error {
	return r.q.IncrPostShareCount(ctx, id)
}

func (r *PostRepository) IncrLikeCount(ctx context.Context, id int64) error {
	return r.q.IncrPostLikeCount(ctx, id)
}

func (r *PostRepository) DecrLikeCount(ctx context.Context, id int64) error {
	return r.q.DecrPostLikeCount(ctx, id)
}

func (r *PostRepository) IncrCommentCount(ctx context.Context, id int64) error {
	return r.q.IncrPostCommentCount(ctx, id)
}

func (r *PostRepository) DecrCommentCount(ctx context.Context, id int64) error {
	return r.q.DecrPostCommentCount(ctx, id)
}

func (r *PostRepository) GetLike(ctx context.Context, postID int64, kind int16, userID int64) (*gen.CmsPostLike, error) {
	row, err := r.q.GetPostLike(ctx, gen.GetPostLikeParams{PostID: postID, ActorKind: kind, UserID: userID})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *PostRepository) CreateLike(ctx context.Context, postID int64, kind int16, userID int64) error {
	return r.q.CreatePostLike(ctx, gen.CreatePostLikeParams{PostID: postID, ActorKind: kind, UserID: userID})
}

func (r *PostRepository) DeleteLike(ctx context.Context, postID int64, kind int16, userID int64) error {
	return r.q.DeletePostLike(ctx, gen.DeletePostLikeParams{PostID: postID, ActorKind: kind, UserID: userID})
}

func (r *PostRepository) BatchGetLikedPostIDs(ctx context.Context, postIDs []int64, kind int16, userID int64) ([]int64, error) {
	return r.q.BatchGetPostLikes(ctx, gen.BatchGetPostLikesParams{Column1: postIDs, ActorKind: kind, UserID: userID})
}

func (r *PostRepository) DeleteLikesByActor(ctx context.Context, kind int16, userID int64) error {
	return r.q.DeletePostLikesByUserID(ctx, gen.DeletePostLikesByUserIDParams{ActorKind: kind, UserID: userID})
}

func (r *PostRepository) SoftDeleteBySellerID(ctx context.Context, sellerID int64) error {
	return r.q.SoftDeletePostsBySellerID(ctx, sellerID)
}

// ListPostIDsBySellerID 讀該賣家全部貼文 id（domain.PostRepository），供
// seller.deleted handler 批次釋放媒體引用。述詞刻意不看 deleted_at，重投時回同一個
// 集合——理由見 posts.sql 的 ListPostIDsBySellerID 查詢註解。
func (r *PostRepository) ListPostIDsBySellerID(ctx context.Context, sellerID int64) ([]int64, error) {
	return r.q.ListPostIDsBySellerID(ctx, sellerID)
}

func (r *PostRepository) ReconcileLikeCounts(ctx context.Context) error {
	return r.q.ReconcilePostLikeCounts(ctx)
}

func (r *PostRepository) ReconcileCommentCounts(ctx context.Context) error {
	return r.q.ReconcilePostCommentCounts(ctx)
}

func (r *PostRepository) ListMy(ctx context.Context, sellerID int64, status string, limit, offset int64) ([]*gen.CmsPost, error) {
	rows, err := r.q.ListMyPosts(ctx, gen.ListMyPostsParams{
		SellerID:        sellerID,
		Column2:         status,
		Limit:           int32(limit),
		Offset:          int32(offset),
		PublishedStatus: domain.PostPublishStatusPublished,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsPost, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *PostRepository) CountMy(ctx context.Context, sellerID int64, status string) (int64, error) {
	return r.q.CountMyPosts(ctx, gen.CountMyPostsParams{
		SellerID:        sellerID,
		Column2:         status,
		PublishedStatus: domain.PostPublishStatusPublished,
	})
}

func (r *PostRepository) AdminList(ctx context.Context, status string, limit, offset int64) ([]gen.AdminListPostsRow, error) {
	return r.q.AdminListPosts(ctx, gen.AdminListPostsParams{
		Column1:         status,
		Limit:           int32(limit),
		Offset:          int32(offset),
		PublishedStatus: domain.PostPublishStatusPublished,
	})
}

func (r *PostRepository) AdminCount(ctx context.Context, status string) (int64, error) {
	return r.q.AdminCountPosts(ctx, gen.AdminCountPostsParams{
		Column1:         status,
		PublishedStatus: domain.PostPublishStatusPublished,
	})
}

func (r *PostRepository) HidePost(ctx context.Context, id int64, reason string) error {
	return r.q.HidePost(ctx, gen.HidePostParams{
		ID:           id,
		HiddenReason: nullableText(reason),
	})
}

func (r *PostRepository) RestorePost(ctx context.Context, id int64) error {
	return r.q.RestorePost(ctx, id)
}

func (r *PostRepository) ListByHashtag(ctx context.Context, tagName string, limit, offset int64) ([]*gen.CmsPost, error) {
	rows, err := r.q.ListPostsByHashtagName(ctx, gen.ListPostsByHashtagNameParams{
		Name:            tagName,
		Limit:           int32(limit),
		Offset:          int32(offset),
		PublishedStatus: domain.PostPublishStatusPublished,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsPost, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *PostRepository) CountByHashtag(ctx context.Context, tagName string) (int64, error) {
	return r.q.CountPostsByHashtagName(ctx, gen.CountPostsByHashtagNameParams{
		Name:            tagName,
		PublishedStatus: domain.PostPublishStatusPublished,
	})
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

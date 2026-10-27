package postgres

import (
	"context"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CommentRepository struct {
	q *gen.Queries
}

func NewCommentRepository(pool *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{q: gen.New(pool)}
}

func (r *CommentRepository) GetByID(ctx context.Context, id int64) (*gen.CmsComment, error) {
	row, err := r.q.GetComment(ctx, id)
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *CommentRepository) ListTopLevel(ctx context.Context, postID, limit, offset int64) ([]*gen.CmsComment, error) {
	rows, err := r.q.ListTopLevelComments(ctx, gen.ListTopLevelCommentsParams{
		PostID: postID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsComment, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *CommentRepository) CountTopLevel(ctx context.Context, postID int64) (int64, error) {
	return r.q.CountTopLevelComments(ctx, postID)
}

func (r *CommentRepository) ListReplies(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsComment, error) {
	rows, err := r.q.ListReplies(ctx, gen.ListRepliesParams{
		ParentID: parentID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsComment, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *CommentRepository) CountReplies(ctx context.Context, parentID int64) (int64, error) {
	return r.q.CountReplies(ctx, parentID)
}

func (r *CommentRepository) GetLatestReplies(ctx context.Context, parentID, limit int64) ([]*gen.CmsComment, error) {
	rows, err := r.q.GetLatestReplies(ctx, gen.GetLatestRepliesParams{
		ParentID: parentID,
		Limit:    int32(limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsComment, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *CommentRepository) BatchGetLatestReplies(ctx context.Context, parentIDs []int64, limit int32) ([]*gen.CmsComment, error) {
	rows, err := r.q.BatchGetLatestReplies(ctx, gen.BatchGetLatestRepliesParams{
		ParentIds:  parentIDs,
		ReplyLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsComment, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *CommentRepository) Create(ctx context.Context, id, postID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsComment, error) {
	row, err := r.q.CreateComment(ctx, gen.CreateCommentParams{
		ID:          id,
		PostID:      postID,
		UserID:      userID,
		UserName:    nullableText(userName),
		UserAvatar:  nullableText(userAvatar),
		Content:     textPointer(content),
		ParentID:    parentID,
		ReplyToID:   replyToID,
		ReplyToName: nullableText(replyToName),
		Role:        role,
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *CommentRepository) UpdateContent(ctx context.Context, id int64, content string) (*gen.CmsComment, error) {
	row, err := r.q.UpdateCommentContent(ctx, gen.UpdateCommentContentParams{
		ID:      id,
		Content: textPointer(content),
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *CommentRepository) SoftDelete(ctx context.Context, id int64) error {
	return r.q.SoftDeleteComment(ctx, id)
}

func (r *CommentRepository) HideComment(ctx context.Context, id int64, reason string) error {
	return r.q.HideComment(ctx, gen.HideCommentParams{
		ID:           id,
		HiddenReason: nullableText(reason),
	})
}

func (r *CommentRepository) IncrReplyCount(ctx context.Context, id int64) error {
	return r.q.IncrCommentReplyCount(ctx, id)
}

func (r *CommentRepository) DecrReplyCount(ctx context.Context, id int64) error {
	return r.q.DecrCommentReplyCount(ctx, id)
}

func (r *CommentRepository) IncrLikeCount(ctx context.Context, id int64) error {
	return r.q.IncrCommentLikeCount(ctx, id)
}

func (r *CommentRepository) DecrLikeCount(ctx context.Context, id int64) error {
	return r.q.DecrCommentLikeCount(ctx, id)
}

func (r *CommentRepository) GetLike(ctx context.Context, commentID int64, kind int16, userID int64) (*gen.CmsCommentLike, error) {
	row, err := r.q.GetCommentLike(ctx, gen.GetCommentLikeParams{CommentID: commentID, ActorKind: kind, UserID: userID})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *CommentRepository) CreateLike(ctx context.Context, commentID int64, kind int16, userID int64) error {
	return r.q.CreateCommentLike(ctx, gen.CreateCommentLikeParams{CommentID: commentID, ActorKind: kind, UserID: userID})
}

func (r *CommentRepository) DeleteLike(ctx context.Context, commentID int64, kind int16, userID int64) error {
	return r.q.DeleteCommentLike(ctx, gen.DeleteCommentLikeParams{CommentID: commentID, ActorKind: kind, UserID: userID})
}

func (r *CommentRepository) BatchGetLikedCommentIDs(ctx context.Context, commentIDs []int64, kind int16, userID int64) ([]int64, error) {
	return r.q.BatchGetCommentLikes(ctx, gen.BatchGetCommentLikesParams{Column1: commentIDs, ActorKind: kind, UserID: userID})
}

func (r *CommentRepository) DeleteLikesByActor(ctx context.Context, kind int16, userID int64) error {
	return r.q.DeleteCommentLikesByUserID(ctx, gen.DeleteCommentLikesByUserIDParams{ActorKind: kind, UserID: userID})
}

func (r *CommentRepository) SoftDeleteByActor(ctx context.Context, kind int16, userID int64) error {
	return r.q.SoftDeleteCommentsByUserID(ctx, gen.SoftDeleteCommentsByUserIDParams{Role: kind, UserID: userID})
}

func (r *CommentRepository) ReconcileLikeCounts(ctx context.Context) error {
	return r.q.ReconcileCommentLikeCounts(ctx)
}

func (r *CommentRepository) ReconcileReplyCounts(ctx context.Context) error {
	return r.q.ReconcileCommentReplyCounts(ctx)
}

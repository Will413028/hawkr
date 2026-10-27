package postgres

import (
	"context"

	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ForumRepository struct {
	q *gen.Queries
}

func NewForumRepository(pool *pgxpool.Pool) *ForumRepository {
	return &ForumRepository{q: gen.New(pool)}
}

func (r *ForumRepository) GetByID(ctx context.Context, id int64) (*gen.CmsSellerForum, error) {
	row, err := r.q.GetForumEntry(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *ForumRepository) ListTopLevel(ctx context.Context, sellerID, limit, offset int64) ([]*gen.CmsSellerForum, error) {
	rows, err := r.q.ListTopLevelForum(ctx, gen.ListTopLevelForumParams{
		SellerID: sellerID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsSellerForum, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *ForumRepository) CountTopLevel(ctx context.Context, sellerID int64) (int64, error) {
	return r.q.CountTopLevelForum(ctx, sellerID)
}

func (r *ForumRepository) ListReplies(ctx context.Context, parentID, limit, offset int64) ([]*gen.CmsSellerForum, error) {
	rows, err := r.q.ListForumReplies(ctx, gen.ListForumRepliesParams{
		ParentID: parentID,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*gen.CmsSellerForum, len(rows))
	for i := range rows {
		r2 := rows[i]
		result[i] = &r2
	}
	return result, nil
}

func (r *ForumRepository) CountReplies(ctx context.Context, parentID int64) (int64, error) {
	return r.q.CountForumReplies(ctx, parentID)
}

func (r *ForumRepository) Create(ctx context.Context, id, sellerID, userID int64, userName, userAvatar, content string, parentID, replyToID int64, replyToName string, role int16) (*gen.CmsSellerForum, error) {
	row, err := r.q.CreateForumEntry(ctx, gen.CreateForumEntryParams{
		ID:          id,
		SellerID:    sellerID,
		UserID:      userID,
		UserName:    nullableText(userName),
		UserAvatar:  nullableText(userAvatar),
		Content:     content,
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

func (r *ForumRepository) UpdateContent(ctx context.Context, id int64, content string) (*gen.CmsSellerForum, error) {
	row, err := r.q.UpdateForumContent(ctx, gen.UpdateForumContentParams{
		ID:      id,
		Content: content,
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *ForumRepository) SoftDelete(ctx context.Context, id int64) error {
	return r.q.SoftDeleteForumEntry(ctx, id)
}

func (r *ForumRepository) HideEntry(ctx context.Context, id int64, reason string) error {
	return r.q.HideForumEntry(ctx, gen.HideForumEntryParams{
		ID:           id,
		HiddenReason: nullableText(reason),
	})
}

func (r *ForumRepository) IncrReplyCount(ctx context.Context, id int64) error {
	return r.q.IncrForumReplyCount(ctx, id)
}

func (r *ForumRepository) DecrReplyCount(ctx context.Context, id int64) error {
	return r.q.DecrForumReplyCount(ctx, id)
}

func (r *ForumRepository) SoftDeleteByActor(ctx context.Context, kind int16, userID int64) error {
	return r.q.SoftDeleteForumByUserID(ctx, gen.SoftDeleteForumByUserIDParams{Role: kind, UserID: userID})
}

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/review-service/internal/domain"
	"github.com/Will413028/hawkr/review-service/internal/repository/postgres/gen"
)

type reviewRepo struct {
	pool *pgxpool.Pool
}

func NewReviewRepository(pool *pgxpool.Pool) domain.ReviewRepository {
	return &reviewRepo{pool: pool}
}

func (r *reviewRepo) Insert(ctx context.Context, rev *domain.Review) (*domain.Review, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.InsertReview(ctx, gen.InsertReviewParams{
		ID:           rev.ID,
		OrderNo:      rev.OrderNo,
		OrderItemID:  rev.OrderItemID,
		BuyerID:      rev.BuyerID,
		StoreID:      rev.StoreID,
		ProductID:    rev.ProductID,
		Rating:       int16(rev.Rating),
		Content:      nullableText(rev.Content),
		ImageKeys:    rev.ImageKeys,
		SkuName:      rev.SkuName,
		ItemImageKey: rev.ItemImageKey,
		Price:        rev.Price,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, domain.ErrAlreadyReviewed
		}
		return nil, err
	}
	return toReview(row), nil
}

func (r *reviewRepo) GetByID(ctx context.Context, id int64) (*domain.Review, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetReviewByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toReview(row), nil
}

func (r *reviewRepo) GetByOrderItemID(ctx context.Context, orderItemID int64) (*domain.Review, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetReviewByOrderItemID(ctx, orderItemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toReview(row), nil
}

func (r *reviewRepo) ListBySellerID(ctx context.Context, storeID int64, rating int32, limit, offset int32) ([]*domain.Review, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListSellerReviews(ctx, gen.ListSellerReviewsParams{
		StoreID: storeID,
		Column2: int16(rating),
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	return toReviews(rows), nil
}

func (r *reviewRepo) CountBySellerID(ctx context.Context, storeID int64, rating int32) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountSellerReviews(ctx, gen.CountSellerReviewsParams{
		StoreID: storeID,
		Column2: int16(rating),
	})
}

func (r *reviewRepo) GetStatsBySellerID(ctx context.Context, storeID int64) (*domain.SellerStats, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetSellerStats(ctx, storeID)
	if err != nil {
		return nil, err
	}

	return &domain.SellerStats{
		TotalCount:   row.TotalCount,
		AvgRating:    row.AvgRating,
		Rating5Count: row.Rating5Count,
		Rating4Count: row.Rating4Count,
		Rating3Count: row.Rating3Count,
		Rating2Count: row.Rating2Count,
		Rating1Count: row.Rating1Count,
	}, nil
}

func (r *reviewRepo) ListStoreRatingStats(ctx context.Context, storeIDs []int64) ([]*domain.StoreRatingStat, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListStoreRatingStats(ctx, storeIDs)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.StoreRatingStat, len(rows))
	for i, row := range rows {
		out[i] = &domain.StoreRatingStat{StoreID: row.StoreID, TotalCount: row.TotalCount, AvgRating: row.AvgRating}
	}
	return out, nil
}

func (r *reviewRepo) ListByBuyerID(ctx context.Context, buyerID int64, limit, offset int32) ([]*domain.Review, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListBuyerReviews(ctx, gen.ListBuyerReviewsParams{
		BuyerID: buyerID,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}
	return toReviews(rows), nil
}

func (r *reviewRepo) CountByBuyerID(ctx context.Context, buyerID int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountBuyerReviews(ctx, buyerID)
}

func (r *reviewRepo) ListByProductID(ctx context.Context, productID int64, rating int32, withImages bool, limit, offset int32) ([]*domain.Review, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListProductReviews(ctx, gen.ListProductReviewsParams{
		ProductID: productID,
		Column2:   int16(rating),
		Column3:   withImages,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		return nil, err
	}
	return toReviews(rows), nil
}

func (r *reviewRepo) CountByProductID(ctx context.Context, productID int64, rating int32, withImages bool) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountProductReviews(ctx, gen.CountProductReviewsParams{
		ProductID: productID,
		Column2:   int16(rating),
		Column3:   withImages,
	})
}

func (r *reviewRepo) GetStatsByProductID(ctx context.Context, productID int64) (*domain.ProductStats, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetProductStats(ctx, productID)
	if err != nil {
		return nil, err
	}

	return &domain.ProductStats{
		TotalCount: row.TotalCount,
		AvgRating:  row.AvgRating,
	}, nil
}

// HideReview marks a review as hidden by the platform (B-135 admin moderation).
// Idempotent: skips already hidden / soft-deleted rows via WHERE clause.
func (r *reviewRepo) HideReview(ctx context.Context, reviewID int64, reason string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.HideReview(ctx, gen.HideReviewParams{
		ID:           reviewID,
		HiddenReason: nullableText(reason),
	})
}

// ReplyReview 賣家回覆評論（#1174）。WHERE 含擁有權 + 可見性（見 queries），
// 0 rows（pgx.ErrNoRows）→ 回 (nil, nil)，由 usecase 補查分流 404/403/409。
func (r *reviewRepo) ReplyReview(ctx context.Context, reviewID, storeID int64, content string) (*domain.Review, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.ReplyToReview(ctx, gen.ReplyToReviewParams{
		ID:           reviewID,
		ReplyContent: &content,
		StoreID:      storeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return toReview(row), nil
}

// toReview 轉換 sqlc model → domain.Review
func toReview(row gen.OrderItemReview) *domain.Review {
	rev := &domain.Review{
		ID:           row.ID,
		OrderNo:      row.OrderNo,
		OrderItemID:  row.OrderItemID,
		BuyerID:      row.BuyerID,
		StoreID:      row.StoreID,
		ProductID:    row.ProductID,
		Rating:       int32(row.Rating),
		ImageKeys:    row.ImageKeys,
		SkuName:      row.SkuName,
		ItemImageKey: row.ItemImageKey,
		Price:        row.Price,
	}
	rev.Content = stringValue(row.Content)
	if row.CreatedAt.Valid {
		rev.CreatedAt = row.CreatedAt.Time
	}
	rev.ReplyContent = stringValue(row.ReplyContent)
	if row.RepliedAt != nil {
		rev.RepliedAt = *row.RepliedAt
	}
	return rev
}

func toReviews(rows []gen.OrderItemReview) []*domain.Review {
	result := make([]*domain.Review, 0, len(rows))
	for _, row := range rows {
		result = append(result, toReview(row))
	}
	return result
}

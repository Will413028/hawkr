package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/review-service/internal/domain"
)

func TestGetProductReviews_ClampsAndOffsets(t *testing.T) {
	var gotLimit, gotOffset int32
	repo := &mockReviewRepo{
		ListByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool, limit, offset int32) ([]*domain.Review, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		CountByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool) (int64, error) {
			return 0, nil
		},
		GetStatsByProductIDFn: func(_ context.Context, _ int64) (*domain.ProductStats, error) {
			return &domain.ProductStats{}, nil
		},
	}

	out, err := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil).GetProductReviews(context.Background(), GetProductReviewsInput{
		ProductID: 1,
		Page:      3,
		PageSize:  999,
	})
	if err != nil {
		t.Fatalf("GetProductReviews() error = %v", err)
	}
	if gotLimit != domain.MaxPageSize || gotOffset != 2*domain.MaxPageSize {
		t.Errorf("repository pagination = (%d, %d), want (%d, %d)", gotLimit, gotOffset, domain.MaxPageSize, 2*domain.MaxPageSize)
	}
	if out.Page != 3 || out.PageSize != domain.MaxPageSize {
		t.Errorf("response pagination = (%d, %d), want (%d, %d)", out.Page, out.PageSize, 3, domain.MaxPageSize)
	}
}

func TestGetSellerReviews_PreservesPagination(t *testing.T) {
	var gotLimit, gotOffset int32
	repo := &mockReviewRepo{
		ListBySellerIDFn: func(_ context.Context, _ int64, _ int32, limit, offset int32) ([]*domain.Review, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		CountBySellerIDFn: func(_ context.Context, _ int64, _ int32) (int64, error) {
			return 0, nil
		},
	}

	out, err := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil).GetSellerReviews(context.Background(), GetSellerReviewsInput{
		SellerID: 1,
		Page:     2,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("GetSellerReviews() error = %v", err)
	}
	if gotLimit != 10 || gotOffset != 10 {
		t.Errorf("repository pagination = (%d, %d), want (10, 10)", gotLimit, gotOffset)
	}
	if out.Page != 2 || out.PageSize != 10 {
		t.Errorf("response pagination = (%d, %d), want (2, 10)", out.Page, out.PageSize)
	}
}

func TestListMyReviews_UsesDefaults(t *testing.T) {
	var gotLimit, gotOffset int32
	repo := &mockReviewRepo{
		ListByBuyerIDFn: func(_ context.Context, _ int64, limit, offset int32) ([]*domain.Review, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
		CountByBuyerIDFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
	}

	out, err := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil).ListMyReviews(context.Background(), ListMyReviewsInput{
		BuyerID: 1,
	})
	if err != nil {
		t.Fatalf("ListMyReviews() error = %v", err)
	}
	if gotLimit != domain.DefaultPageSize || gotOffset != 0 {
		t.Errorf("repository pagination = (%d, %d), want (%d, 0)", gotLimit, gotOffset, domain.DefaultPageSize)
	}
	if out.Page != 1 || out.PageSize != domain.DefaultPageSize {
		t.Errorf("response pagination = (%d, %d), want (1, %d)", out.Page, out.PageSize, domain.DefaultPageSize)
	}
}

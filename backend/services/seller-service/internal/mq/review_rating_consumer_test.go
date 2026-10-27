package mq

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

// ratingRecorderSellerRepo records UpdateRatingByStoreID calls; the other
// domain.SellerRepository methods are unused by the review.created handler.
type ratingRecorderSellerRepo struct {
	domain.SellerRepository
	calls []ratingCall
}

type ratingCall struct {
	storeID int64
	rating  float64
}

func (r *ratingRecorderSellerRepo) UpdateRatingByStoreID(_ context.Context, storeID int64, rating float64) error {
	r.calls = append(r.calls, ratingCall{storeID: storeID, rating: rating})
	return nil
}

func TestReviewRatingHandler_UpdatesByStoreID(t *testing.T) {
	repo := &ratingRecorderSellerRepo{}
	h := reviewRatingHandler(repo)

	body := []byte(`{"store_id":555,"buyer_id":100,"avg_rating":4.7,"total_count":12}`)
	if err := h(context.Background(), "review.created", body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.calls) != 1 {
		t.Fatalf("expected 1 UpdateRatingByStoreID call, got %d: %+v", len(repo.calls), repo.calls)
	}
	if repo.calls[0].storeID != 555 {
		t.Errorf("storeID = %d, want 555 (the event's store_id value)", repo.calls[0].storeID)
	}
	if repo.calls[0].rating != 4.7 {
		t.Errorf("rating = %v, want 4.7", repo.calls[0].rating)
	}
}

func TestReviewRatingHandler_MissingStoreID(t *testing.T) {
	repo := &ratingRecorderSellerRepo{}
	h := reviewRatingHandler(repo)

	body := []byte(`{"buyer_id":100,"avg_rating":4.7,"total_count":12}`)
	if err := h(context.Background(), "review.created", body); err == nil {
		t.Fatal("expected error for missing store_id, got nil")
	}
	if len(repo.calls) != 0 {
		t.Fatalf("expected no UpdateRatingByStoreID calls, got %d", len(repo.calls))
	}
}

func TestReviewRatingHandler_LegacySellerAccountIDKeyRejected(t *testing.T) {
	repo := &ratingRecorderSellerRepo{}
	h := reviewRatingHandler(repo)

	body := []byte(`{"seller_account_id":555,"buyer_id":100,"avg_rating":4.7,"total_count":12}`)
	if err := h(context.Background(), "review.created", body); err == nil {
		t.Fatal("expected error for legacy seller_account_id-only payload, got nil")
	}
	if len(repo.calls) != 0 {
		t.Fatalf("expected no UpdateRatingByStoreID calls, got %d: %+v", len(repo.calls), repo.calls)
	}
}

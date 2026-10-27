package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/domain"
)

// --- mock repo ---

type mockReviewRepo struct {
	InsertFn               func(ctx context.Context, r *domain.Review) (*domain.Review, error)
	GetByOrderItemIDFn     func(ctx context.Context, orderItemID int64) (*domain.Review, error)
	GetByIDFn              func(ctx context.Context, id int64) (*domain.Review, error)
	ListBySellerIDFn       func(ctx context.Context, storeID int64, rating, limit, offset int32) ([]*domain.Review, error)
	CountBySellerIDFn      func(ctx context.Context, storeID int64, rating int32) (int64, error)
	GetStatsBySellerIDFn   func(ctx context.Context, storeID int64) (*domain.SellerStats, error)
	ListStoreRatingStatsFn func(ctx context.Context, storeIDs []int64) ([]*domain.StoreRatingStat, error)
	ListByBuyerIDFn        func(ctx context.Context, buyerID int64, limit, offset int32) ([]*domain.Review, error)
	CountByBuyerIDFn       func(ctx context.Context, buyerID int64) (int64, error)
	ListByProductIDFn      func(ctx context.Context, productID int64, rating int32, withImages bool, limit, offset int32) ([]*domain.Review, error)
	CountByProductIDFn     func(ctx context.Context, productID int64, rating int32, withImages bool) (int64, error)
	GetStatsByProductIDFn  func(ctx context.Context, productID int64) (*domain.ProductStats, error)
	ReplyReviewFn          func(ctx context.Context, reviewID, storeID int64, content string) (*domain.Review, error)
}

func (m *mockReviewRepo) Insert(ctx context.Context, r *domain.Review) (*domain.Review, error) {
	return m.InsertFn(ctx, r)
}
func (m *mockReviewRepo) GetByOrderItemID(ctx context.Context, orderItemID int64) (*domain.Review, error) {
	return m.GetByOrderItemIDFn(ctx, orderItemID)
}
func (m *mockReviewRepo) GetByID(ctx context.Context, id int64) (*domain.Review, error) {
	return m.GetByIDFn(ctx, id)
}
func (m *mockReviewRepo) ListBySellerID(ctx context.Context, storeID int64, rating, limit, offset int32) ([]*domain.Review, error) {
	return m.ListBySellerIDFn(ctx, storeID, rating, limit, offset)
}
func (m *mockReviewRepo) CountBySellerID(ctx context.Context, storeID int64, rating int32) (int64, error) {
	return m.CountBySellerIDFn(ctx, storeID, rating)
}
func (m *mockReviewRepo) GetStatsBySellerID(ctx context.Context, storeID int64) (*domain.SellerStats, error) {
	return m.GetStatsBySellerIDFn(ctx, storeID)
}
func (m *mockReviewRepo) ListStoreRatingStats(ctx context.Context, storeIDs []int64) ([]*domain.StoreRatingStat, error) {
	return m.ListStoreRatingStatsFn(ctx, storeIDs)
}
func (m *mockReviewRepo) ListByBuyerID(ctx context.Context, buyerID int64, limit, offset int32) ([]*domain.Review, error) {
	return m.ListByBuyerIDFn(ctx, buyerID, limit, offset)
}
func (m *mockReviewRepo) CountByBuyerID(ctx context.Context, buyerID int64) (int64, error) {
	return m.CountByBuyerIDFn(ctx, buyerID)
}
func (m *mockReviewRepo) ListByProductID(ctx context.Context, productID int64, rating int32, withImages bool, limit, offset int32) ([]*domain.Review, error) {
	return m.ListByProductIDFn(ctx, productID, rating, withImages, limit, offset)
}
func (m *mockReviewRepo) CountByProductID(ctx context.Context, productID int64, rating int32, withImages bool) (int64, error) {
	return m.CountByProductIDFn(ctx, productID, rating, withImages)
}
func (m *mockReviewRepo) GetStatsByProductID(ctx context.Context, productID int64) (*domain.ProductStats, error) {
	return m.GetStatsByProductIDFn(ctx, productID)
}
func (m *mockReviewRepo) HideReview(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockReviewRepo) ReplyReview(ctx context.Context, reviewID, storeID int64, content string) (*domain.Review, error) {
	return m.ReplyReviewFn(ctx, reviewID, storeID, content)
}

// --- mock order client ---

type mockOrderClient struct {
	GetOrderForReviewFn func(ctx context.Context, orderNo string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error)
}

func (m *mockOrderClient) GetOrderForReview(ctx context.Context, orderNo string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
	return m.GetOrderForReviewFn(ctx, orderNo)
}

// --- mock media client ---

type mockMediaClient struct {
	FinalizeAndLinkFn func(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error)
}

func (m *mockMediaClient) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error) {
	if m.FinalizeAndLinkFn != nil {
		return m.FinalizeAndLinkFn(ctx, stagedToken, refType, refID)
	}
	return AssetInfo{AssetId: "asset-" + stagedToken, ObjectKey: "reviews/" + stagedToken + ".jpg"}, nil
}

// --- helpers ---

func fixedTSID() func() int64 {
	return func() int64 { return 12345 }
}

func newTestConfig() *config.Config {
	return &config.Config{}
}

func completedOrder(buyerID, sellerID int64, completedAt time.Time) *domain.OrderEligibility {
	return &domain.OrderEligibility{
		OrderNo:     "ORD-001",
		BuyerID:     buyerID,
		StoreID:     sellerID,
		Status:      domain.OrderStatusCompleted,
		CompletedAt: completedAt,
	}
}

// --- tests: SubmitItemReview ---

func TestSubmitItemReview_Success(t *testing.T) {
	buyerID := int64(100)
	sellerID := int64(200)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour) // 昨天完成

	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil // 尚未評論
		},
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			return r, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, sellerID, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID, SkuName: "蘋果", ImageURL: "http://img", Price: 100},
			}, nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, &mockMediaClient{}, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	got, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:      "ORD-001",
		OrderItemID:  itemID,
		BuyerID:      buyerID,
		Rating:       5,
		Content:      "很新鮮",
		StagedTokens: []string{"token1"},
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != 12345 {
		t.Errorf("expected ID 12345, got %d", got.ID)
	}
	if got.Rating != 5 {
		t.Errorf("expected rating 5, got %d", got.Rating)
	}
	if got.StoreID != sellerID {
		t.Errorf("expected sellerID %d, got %d", sellerID, got.StoreID)
	}
}

// --- mock seller client ---

type mockSellerClient struct {
	ResolvePrincipalFn func(ctx context.Context, storeID int64) int64
	ResolveStoreCodeFn func(ctx context.Context, storeID int64) (string, error)
}

func (m *mockSellerClient) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	if m.ResolvePrincipalFn == nil {
		return storeID
	}
	return m.ResolvePrincipalFn(ctx, storeID)
}

// Unset behaves like a seller row with no store_code: the caller degrades to
// the store_id tag rather than treating it as an error.
func (m *mockSellerClient) ResolveStoreCode(ctx context.Context, storeID int64) (string, error) {
	if m.ResolveStoreCodeFn == nil {
		return "", nil
	}
	return m.ResolveStoreCodeFn(ctx, storeID)
}

// --- mock event publisher ---

type mockEventPublisher struct {
	PublishReviewCreatedFn func(ctx context.Context, review *domain.Review, notifySellerAccountID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64, buyerNickname, buyerAvatar string) error
}

func (m *mockEventPublisher) PublishReviewCreated(ctx context.Context, review *domain.Review, notifySellerAccountID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64, buyerNickname, buyerAvatar string) error {
	return m.PublishReviewCreatedFn(ctx, review, notifySellerAccountID, avgRating, totalCount, productAvgRating, productReviewCount, buyerNickname, buyerAvatar)
}

func (m *mockEventPublisher) PublishReviewHidden(ctx context.Context, reviewID, storeID, productID int64, avgRating float64, totalCount int64, productAvgRating float64, productReviewCount int64) error {
	return nil
}

func TestSubmitItemReview_PublishesResolvedSellerAccountID(t *testing.T) {
	buyerID := int64(100)
	storeID := int64(200)
	resolvedAccountID := int64(9988776655)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			return r, nil
		},
		GetStatsBySellerIDFn: func(_ context.Context, _ int64) (*domain.SellerStats, error) {
			return &domain.SellerStats{TotalCount: 1, AvgRating: 5}, nil
		},
		GetStatsByProductIDFn: func(_ context.Context, _ int64) (*domain.ProductStats, error) {
			return &domain.ProductStats{TotalCount: 1, AvgRating: 5}, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, storeID, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID, SkuName: "蘋果", ImageURL: "http://img", Price: 100},
			}, nil
		},
	}
	sellerClient := &mockSellerClient{
		ResolvePrincipalFn: func(_ context.Context, gotStoreID int64) int64 {
			if gotStoreID != storeID {
				t.Errorf("ResolvePrincipal called with storeID %d, want %d", gotStoreID, storeID)
			}
			return resolvedAccountID
		},
	}

	var gotNotifySellerAccountID int64
	var publishCalled bool
	publisher := &mockEventPublisher{
		PublishReviewCreatedFn: func(_ context.Context, review *domain.Review, notifySellerAccountID int64, _ float64, _ int64, _ float64, _ int64, _, _ string) error {
			publishCalled = true
			gotNotifySellerAccountID = notifySellerAccountID
			if review.StoreID != storeID {
				t.Errorf("review.StoreID mutated to %d, want unchanged %d", review.StoreID, storeID)
			}
			return nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, &mockMediaClient{}, nil, sellerClient, publisher, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-001",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		Rating:      5,
		Content:     "很新鮮",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !publishCalled {
		t.Fatal("expected PublishReviewCreated to be called")
	}
	if gotNotifySellerAccountID != resolvedAccountID {
		t.Errorf("notifySellerAccountID = %d, want resolved %d (raw store_id %d must not leak through)",
			gotNotifySellerAccountID, resolvedAccountID, storeID)
	}
}

// TestSubmitItemReview_UnresolvedSeller_PublishesZeroNotStoreID is the
// discriminating half of #2789 stage B2a at the use-case layer: it pins BOTH
// halves of the decision, which are easy to get half-right.
//
//  1. notifySellerAccountID must be 0, not the raw store_id. Publishing the
//     store_id puts a store-space id in review.created's
//     notify_seller_account_id, which notification-service reads as the
//     recipient account_id — it addresses nobody, and does so with no log line
//     anywhere. 0 trips extractUserID's id <= 0 check, which skips the
//     notification and logs it.
//  2. The event must still be PUBLISHED. review.created has three other
//     consumers (order-service is_reviewed, product-service and seller-service
//     rating aggregates) that read store_id and the stats fields and never
//     touch this one. Suppressing the publish to avoid a bad recipient would
//     silently stop all three.
//
// Reverting the client to `return storeID` fails assertion 1; short-circuiting
// the publish on a zero principal fails assertion 2.
func TestSubmitItemReview_UnresolvedSeller_PublishesZeroNotStoreID(t *testing.T) {
	buyerID := int64(100)
	storeID := int64(200)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			return r, nil
		},
		GetStatsBySellerIDFn: func(_ context.Context, _ int64) (*domain.SellerStats, error) {
			return &domain.SellerStats{TotalCount: 1, AvgRating: 5}, nil
		},
		GetStatsByProductIDFn: func(_ context.Context, _ int64) (*domain.ProductStats, error) {
			return &domain.ProductStats{TotalCount: 1, AvgRating: 5}, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, storeID, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID, SkuName: "蘋果", ImageURL: "http://img", Price: 100},
			}, nil
		},
	}
	// Models the real client's unresolved answer (soft-deleted stint / unknown
	// store_id / seller-service down): 0, never the input.
	sellerClient := &mockSellerClient{
		ResolvePrincipalFn: func(_ context.Context, _ int64) int64 { return 0 },
	}

	var gotNotifySellerAccountID int64
	var publishCalled bool
	publisher := &mockEventPublisher{
		PublishReviewCreatedFn: func(_ context.Context, review *domain.Review, notifySellerAccountID int64, _ float64, _ int64, _ float64, _ int64, _, _ string) error {
			publishCalled = true
			gotNotifySellerAccountID = notifySellerAccountID
			if review.StoreID != storeID {
				t.Errorf("review.StoreID = %d, want unchanged %d (the other three consumers key on it)", review.StoreID, storeID)
			}
			return nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, &mockMediaClient{}, nil, sellerClient, publisher, newTestConfig(), fixedTSID(), nil)
	if _, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-002",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		Rating:      5,
		Content:     "很新鮮",
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !publishCalled {
		t.Fatal("review.created must still be published when the seller principal is unresolved — order/product/seller consumers do not read notify_seller_account_id")
	}
	if gotNotifySellerAccountID != 0 {
		t.Errorf("notifySellerAccountID = %d, want 0 (the raw store_id %d must not be published into an account-space recipient field)",
			gotNotifySellerAccountID, storeID)
	}
}

func TestSubmitItemReview_AlreadyReviewed(t *testing.T) {
	buyerID := int64(100)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return &domain.Review{ID: 1}, nil // 已評論
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, 200, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID},
			}, nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-001",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		Rating:      4,
	})

	if pkgerrors.GetCode(err) != pkgerrors.CodeConflict {
		t.Errorf("expected CodeConflict, got %v", err)
	}
}

func TestSubmitItemReview_OrderNotFound(t *testing.T) {
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return nil, nil, errors.New("not found")
		},
	}

	uc := NewReviewUseCase(&mockReviewRepo{}, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo: "ORD-NONE",
		BuyerID: 100,
		Rating:  5,
	})

	if pkgerrors.GetCode(err) != pkgerrors.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", err)
	}
}

func TestSubmitItemReview_NotYourOrder(t *testing.T) {
	completedAt := time.Now().Add(-24 * time.Hour)
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(999, 200, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: 1},
			}, nil
		},
	}

	uc := NewReviewUseCase(&mockReviewRepo{}, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-001",
		OrderItemID: 1,
		BuyerID:     100, // 不是 999
		Rating:      5,
	})

	if pkgerrors.GetCode(err) != pkgerrors.CodeForbidden {
		t.Errorf("expected CodeForbidden, got %v", err)
	}
}

func TestSubmitItemReview_ReviewExpired(t *testing.T) {
	buyerID := int64(100)
	itemID := int64(999)
	completedAt := time.Now().Add(-31 * 24 * time.Hour) // 31 天前

	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, 200, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID},
			}, nil
		},
	}

	uc := NewReviewUseCase(&mockReviewRepo{}, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-001",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		Rating:      5,
	})

	if pkgerrors.GetCode(err) != pkgerrors.CodePreconditionFailed {
		t.Errorf("expected CodePreconditionFailed, got %v", err)
	}
}

func TestSubmitItemReview_ItemNotInOrder(t *testing.T) {
	buyerID := int64(100)
	completedAt := time.Now().Add(-24 * time.Hour)

	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, 200, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: 111}, // 訂單只有 item 111
			}, nil
		},
	}

	uc := NewReviewUseCase(&mockReviewRepo{}, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-001",
		OrderItemID: 999, // 不在訂單中
		BuyerID:     buyerID,
		Rating:      5,
	})

	if pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected CodeInvalidInput, got %v", err)
	}
}

func TestSubmitItemReview_ImageURLsCapped(t *testing.T) {
	buyerID := int64(100)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	var inserted *domain.Review
	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			inserted = r
			return r, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, 200, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID},
			}, nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, &mockMediaClient{}, nil, nil, nil, newTestConfig(), fixedTSID(), nil)

	// 產生超過 MaxImageCount 個的 staged token
	tokens := make([]string, domain.MaxImageCount+3)
	for i := range tokens {
		tokens[i] = fmt.Sprintf("token%d", i)
	}
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:      "ORD-001",
		OrderItemID:  itemID,
		BuyerID:      buyerID,
		Rating:       3,
		StagedTokens: tokens,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inserted.ImageKeys) != domain.MaxImageCount {
		t.Errorf("expected %d image keys, got %d", domain.MaxImageCount, len(inserted.ImageKeys))
	}
	// 49 §6.2：DB 只存 objectKey，不得存完整 URL。mockMediaClient 回 "reviews/<token>.jpg"。
	for _, k := range inserted.ImageKeys {
		if strings.HasPrefix(k, "http") {
			t.Errorf("expected objectKey (no http prefix) stored, got %q", k)
		}
	}
}

// 49 §5.3：item image 由後端從訂單商品圖快照回填，並剝殼還原成 objectKey；前端送的
// item_image_url 一律忽略（SubmitItemReviewInput 已無此欄）。
func TestSubmitItemReview_ItemImageBackfilledAsKey(t *testing.T) {
	buyerID := int64(100)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	var inserted *domain.Review
	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) { return nil, nil },
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			inserted = r
			return r, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, 200, completedAt), []*domain.OrderItemInfo{
				// order-service 回傳組好的 CDN URL
				{OrderItemID: itemID, ImageURL: "https://test-media.example.com/products/abc.jpg"},
			}, nil
		},
	}

	cfg := &config.Config{MediaBaseURL: "https://test-media.example.com"}
	uc := NewReviewUseCase(repo, orderClient, &mockMediaClient{}, nil, nil, nil, cfg, fixedTSID(), nil)

	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-001",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		Rating:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inserted.ItemImageKey != "products/abc.jpg" {
		t.Errorf("expected item_image_key stripped to objectKey %q, got %q", "products/abc.jpg", inserted.ItemImageKey)
	}
}

func TestSubmitItemReview_OrderNotCompleted(t *testing.T) {
	buyerID := int64(100)
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return &domain.OrderEligibility{
				OrderNo:     "ORD-001",
				BuyerID:     buyerID,
				Status:      "40", // DELIVERED，非 COMPLETED("50")
				CompletedAt: time.Now(),
			}, []*domain.OrderItemInfo{{OrderItemID: 1}}, nil
		},
	}

	uc := NewReviewUseCase(&mockReviewRepo{}, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "ORD-001",
		OrderItemID: 1,
		BuyerID:     buyerID,
		Rating:      5,
	})

	if !errors.Is(err, ErrNotEligible) {
		t.Errorf("expected ErrNotEligible, got %v", err)
	}
}

// --- tests: GetProductReviews ---

func TestGetProductReviews_Success(t *testing.T) {
	productID := int64(42)
	reviews := []*domain.Review{
		{ID: 1, ProductID: productID, Rating: 5, Content: "很好"},
		{ID: 2, ProductID: productID, Rating: 4, Content: "不錯"},
	}
	repo := &mockReviewRepo{
		ListByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool, _, _ int32) ([]*domain.Review, error) {
			return reviews, nil
		},
		CountByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool) (int64, error) {
			return 2, nil
		},
		GetStatsByProductIDFn: func(_ context.Context, _ int64) (*domain.ProductStats, error) {
			return &domain.ProductStats{TotalCount: 2, AvgRating: 4.5}, nil
		},
	}

	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	out, err := uc.GetProductReviews(context.Background(), GetProductReviewsInput{
		ProductID: productID,
		Page:      1,
		PageSize:  20,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Reviews) != 2 {
		t.Errorf("expected 2 reviews, got %d", len(out.Reviews))
	}
	if out.Total != 2 {
		t.Errorf("expected total 2, got %d", out.Total)
	}
	if out.Stats.AvgRating != 4.5 {
		t.Errorf("expected avg 4.5, got %f", out.Stats.AvgRating)
	}
}

func TestGetProductReviews_Empty(t *testing.T) {
	repo := &mockReviewRepo{
		ListByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool, _, _ int32) ([]*domain.Review, error) {
			return nil, nil
		},
		CountByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool) (int64, error) {
			return 0, nil
		},
		GetStatsByProductIDFn: func(_ context.Context, _ int64) (*domain.ProductStats, error) {
			return &domain.ProductStats{TotalCount: 0, AvgRating: 0}, nil
		},
	}

	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	out, err := uc.GetProductReviews(context.Background(), GetProductReviewsInput{
		ProductID: 999,
		Page:      1,
		PageSize:  20,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Reviews) != 0 {
		t.Errorf("expected 0 reviews, got %d", len(out.Reviews))
	}
	if out.Total != 0 {
		t.Errorf("expected total 0, got %d", out.Total)
	}
	if out.Stats.TotalCount != 0 {
		t.Errorf("expected stats total 0, got %d", out.Stats.TotalCount)
	}
}

func TestGetProductReviews_RatingFilter(t *testing.T) {
	reviews := []*domain.Review{{ID: 1, Rating: 5}}
	repo := &mockReviewRepo{
		ListByProductIDFn: func(_ context.Context, _ int64, rating int32, _ bool, _, _ int32) ([]*domain.Review, error) {
			if rating != 5 {
				t.Errorf("expected rating=5, got %d", rating)
			}
			return reviews, nil
		},
		CountByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool) (int64, error) {
			return 1, nil
		},
		GetStatsByProductIDFn: func(_ context.Context, _ int64) (*domain.ProductStats, error) {
			return &domain.ProductStats{TotalCount: 1, AvgRating: 5}, nil
		},
	}

	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.GetProductReviews(context.Background(), GetProductReviewsInput{
		ProductID: 42,
		Rating:    5,
		Page:      1,
		PageSize:  20,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- tests: GetReview ---

func TestGetReview_Found(t *testing.T) {
	expected := &domain.Review{ID: 123, ProductID: 42, Rating: 4}
	repo := &mockReviewRepo{
		GetByIDFn: func(_ context.Context, id int64) (*domain.Review, error) {
			if id != 123 {
				t.Errorf("expected id=123, got %d", id)
			}
			return expected, nil
		},
	}

	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	got, err := uc.GetReview(context.Background(), 123)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.ID != 123 {
		t.Errorf("expected review id 123, got %v", got)
	}
}

func TestGetReview_NotFound(t *testing.T) {
	repo := &mockReviewRepo{
		GetByIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
	}

	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	got, err := uc.GetReview(context.Background(), 999)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil review, got %v", got)
	}
}

// --- tests: GetSellerStats ---

func TestGetSellerStats_NoReviews(t *testing.T) {
	repo := &mockReviewRepo{
		GetStatsBySellerIDFn: func(_ context.Context, _ int64) (*domain.SellerStats, error) {
			return &domain.SellerStats{TotalCount: 0, AvgRating: 0}, nil
		},
	}

	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	stats, err := uc.GetSellerStats(context.Background(), 200)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalCount != 0 {
		t.Errorf("expected 0 total count, got %d", stats.TotalCount)
	}
	if stats.AvgRating != 0 {
		t.Errorf("expected 0 avg rating, got %f", stats.AvgRating)
	}
}

func TestGetSellerStats_WithReviews(t *testing.T) {
	repo := &mockReviewRepo{
		GetStatsBySellerIDFn: func(_ context.Context, _ int64) (*domain.SellerStats, error) {
			return &domain.SellerStats{TotalCount: 10, AvgRating: 4.5}, nil
		},
	}

	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	stats, err := uc.GetSellerStats(context.Background(), 200)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalCount != 10 {
		t.Errorf("expected 10 total count, got %d", stats.TotalCount)
	}
	if stats.AvgRating != 4.5 {
		t.Errorf("expected 4.5 avg rating, got %f", stats.AvgRating)
	}
}

// --- regression tests: product_id 必須來自 order service（Issue #465）---

// TestSubmitItemReview_ProductIDFromOrderService 驗證 product_id 由 order service 提供，不採用客戶端值
func TestSubmitItemReview_ProductIDFromOrderService(t *testing.T) {
	const orderProductID = int64(825045450150940300)
	buyerID := int64(100)
	sellerID := int64(200)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	var insertedReview *domain.Review
	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			insertedReview = r
			return r, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, sellerID, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID, ProductID: orderProductID, SkuName: "711test", Price: 100},
			}, nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "O925068125",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		Rating:      5,
		// 客戶端未傳 product_id（零值）
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if insertedReview.ProductID != orderProductID {
		t.Errorf("product_id should be from order service: want %d, got %d", orderProductID, insertedReview.ProductID)
	}
}

// TestSubmitItemReview_ClientProductIDZeroIsIgnored 驗證客戶端傳 0 時，仍使用 order service 的值
func TestSubmitItemReview_ClientProductIDZeroIsIgnored(t *testing.T) {
	const orderProductID = int64(825045450150940300)
	buyerID := int64(100)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	var insertedReview *domain.Review
	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			insertedReview = r
			return r, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, 200, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID, ProductID: orderProductID},
			}, nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "O925068125",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		ProductID:   0, // 客戶端傳 0
		Rating:      5,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if insertedReview.ProductID != orderProductID {
		t.Errorf("client product_id=0 should be ignored: want %d, got %d", orderProductID, insertedReview.ProductID)
	}
}

// TestSubmitItemReview_ClientProductIDWrongValueIsIgnored 驗證客戶端傳錯誤值時，仍使用 order service 的值
func TestSubmitItemReview_ClientProductIDWrongValueIsIgnored(t *testing.T) {
	const orderProductID = int64(825045450150940300)
	const clientWrongProductID = int64(999)
	buyerID := int64(100)
	itemID := int64(999)
	completedAt := time.Now().Add(-24 * time.Hour)

	var insertedReview *domain.Review
	repo := &mockReviewRepo{
		GetByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
		InsertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			insertedReview = r
			return r, nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return completedOrder(buyerID, 200, completedAt), []*domain.OrderItemInfo{
				{OrderItemID: itemID, ProductID: orderProductID},
			}, nil
		},
	}

	uc := NewReviewUseCase(repo, orderClient, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.SubmitItemReview(context.Background(), SubmitItemReviewInput{
		OrderNo:     "O925068125",
		OrderItemID: itemID,
		BuyerID:     buyerID,
		ProductID:   clientWrongProductID, // 客戶端傳錯誤值
		Rating:      5,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if insertedReview.ProductID != orderProductID {
		t.Errorf("wrong client product_id should be ignored: want %d, got %d", orderProductID, insertedReview.ProductID)
	}
}

// ==================== ReplyReview（#1174 賣家回覆）====================

func TestReplyReview_Success(t *testing.T) {
	repo := &mockReviewRepo{
		ReplyReviewFn: func(_ context.Context, reviewID, sellerID int64, content string) (*domain.Review, error) {
			if reviewID != 123 || sellerID != 555 || content != "感謝您的支持" {
				t.Errorf("unexpected args: %d %d %q", reviewID, sellerID, content)
			}
			return &domain.Review{ID: 123, StoreID: 555, ReplyContent: content, RepliedAt: time.Now().UTC()}, nil
		},
	}
	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	got, err := uc.ReplyReview(context.Background(), 123, 555, "感謝您的支持")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.ReplyContent != "感謝您的支持" || got.RepliedAt.IsZero() {
		t.Errorf("expected reply set, got %+v", got)
	}
}

func TestReplyReview_EmptyContent(t *testing.T) {
	uc := NewReviewUseCase(&mockReviewRepo{}, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.ReplyReview(context.Background(), 123, 555, "   ")
	if pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
		t.Errorf("expected InvalidInput, got %v", err)
	}
}

func TestReplyReview_NotFound(t *testing.T) {
	repo := &mockReviewRepo{
		ReplyReviewFn: func(_ context.Context, _, _ int64, _ string) (*domain.Review, error) { return nil, nil },
		GetByIDFn:     func(_ context.Context, _ int64) (*domain.Review, error) { return nil, nil },
	}
	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.ReplyReview(context.Background(), 999, 555, "回覆內容")
	if !errors.Is(err, ErrReviewNotFound) {
		t.Errorf("expected ErrReviewNotFound, got %v", err)
	}
}

func TestReplyReview_NotYourReview(t *testing.T) {
	repo := &mockReviewRepo{
		ReplyReviewFn: func(_ context.Context, _, _ int64, _ string) (*domain.Review, error) { return nil, nil },
		GetByIDFn: func(_ context.Context, id int64) (*domain.Review, error) {
			return &domain.Review{ID: id, StoreID: 999}, nil // 別的賣家
		},
	}
	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.ReplyReview(context.Background(), 123, 555, "回覆內容")
	if !errors.Is(err, ErrNotYourReview) {
		t.Errorf("expected ErrNotYourReview, got %v", err)
	}
}

func TestReplyReview_NotReplyable(t *testing.T) {
	repo := &mockReviewRepo{
		// UPDATE 0 rows（被 hidden/deleted WHERE 擋掉）
		ReplyReviewFn: func(_ context.Context, _, _ int64, _ string) (*domain.Review, error) { return nil, nil },
		// 補查存在且屬本賣家 → 推斷為 hidden/deleted 不可回覆
		GetByIDFn: func(_ context.Context, id int64) (*domain.Review, error) {
			return &domain.Review{ID: id, StoreID: 555}, nil
		},
	}
	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	_, err := uc.ReplyReview(context.Background(), 123, 555, "回覆內容")
	if !errors.Is(err, ErrReviewNotReplyable) {
		t.Errorf("expected ErrReviewNotReplyable, got %v", err)
	}
}

// --- tests: ListStoreRatingStats ---

func TestListStoreRatingStats_DedupesAndPassesThrough(t *testing.T) {
	var got []int64
	repo := &mockReviewRepo{
		ListStoreRatingStatsFn: func(_ context.Context, ids []int64) ([]*domain.StoreRatingStat, error) {
			got = ids
			return []*domain.StoreRatingStat{{StoreID: 200, TotalCount: 10, AvgRating: 4.5}}, nil
		},
	}
	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)

	stats, err := uc.ListStoreRatingStats(context.Background(), []int64{200, 300, 200})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != 200 || got[1] != 300 {
		t.Errorf("repo got %v, want [200 300] (duplicates collapsed, order kept)", got)
	}
	if len(stats) != 1 || stats[0].TotalCount != 10 || stats[0].AvgRating != 4.5 {
		t.Errorf("stats = %+v", stats)
	}
}

func TestListStoreRatingStats_RejectsBadBatch(t *testing.T) {
	repo := &mockReviewRepo{
		ListStoreRatingStatsFn: func(_ context.Context, _ []int64) ([]*domain.StoreRatingStat, error) {
			t.Fatal("repo must not be called for an invalid batch")
			return nil, nil
		},
	}
	uc := NewReviewUseCase(repo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	tooMany := make([]int64, domain.MaxStoreRatingStatsBatch+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	for name, ids := range map[string][]int64{"empty": nil, "zero id": {1, 0}, "negative id": {-3}, "too many": tooMany} {
		if _, err := uc.ListStoreRatingStats(context.Background(), ids); pkgerrors.GetCode(err) != pkgerrors.CodeInvalidInput {
			t.Errorf("%s: err = %v, want INVALID_INPUT", name, err)
		}
	}
	// 剛好 100 個（含重複壓縮前的 100）是合法的。
	okRepo := &mockReviewRepo{ListStoreRatingStatsFn: func(_ context.Context, ids []int64) ([]*domain.StoreRatingStat, error) { return nil, nil }}
	uc = NewReviewUseCase(okRepo, &mockOrderClient{}, nil, nil, nil, nil, newTestConfig(), fixedTSID(), nil)
	if _, err := uc.ListStoreRatingStats(context.Background(), tooMany[:domain.MaxStoreRatingStatsBatch]); err != nil {
		t.Errorf("a full batch must be accepted: %v", err)
	}
}

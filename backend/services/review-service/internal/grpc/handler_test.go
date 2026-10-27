package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/Will413028/hawkr/go-pkg/testutil"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/domain"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// ==================== Mock Repo ====================

type testReviewRepo struct {
	insertFn               func(ctx context.Context, r *domain.Review) (*domain.Review, error)
	getByOrderItemIDFn     func(ctx context.Context, orderItemID int64) (*domain.Review, error)
	getByIDFn              func(ctx context.Context, id int64) (*domain.Review, error)
	listBySellerIDFn       func(ctx context.Context, storeID int64, rating, limit, offset int32) ([]*domain.Review, error)
	countBySellerIDFn      func(ctx context.Context, storeID int64, rating int32) (int64, error)
	getStatsBySellerIDFn   func(ctx context.Context, storeID int64) (*domain.SellerStats, error)
	listStoreRatingStatsFn func(ctx context.Context, storeIDs []int64) ([]*domain.StoreRatingStat, error)
	listByBuyerIDFn        func(ctx context.Context, buyerID int64, limit, offset int32) ([]*domain.Review, error)
	countByBuyerIDFn       func(ctx context.Context, buyerID int64) (int64, error)
	listByProductIDFn      func(ctx context.Context, productID int64, rating int32, withImages bool, limit, offset int32) ([]*domain.Review, error)
	countByProductIDFn     func(ctx context.Context, productID int64, rating int32, withImages bool) (int64, error)
	getStatsByProductIDFn  func(ctx context.Context, productID int64) (*domain.ProductStats, error)
	replyReviewFn          func(ctx context.Context, reviewID, storeID int64, content string) (*domain.Review, error)
}

func (r *testReviewRepo) Insert(ctx context.Context, rev *domain.Review) (*domain.Review, error) {
	if r.insertFn != nil {
		return r.insertFn(ctx, rev)
	}
	return rev, nil
}
func (r *testReviewRepo) GetByOrderItemID(ctx context.Context, id int64) (*domain.Review, error) {
	if r.getByOrderItemIDFn != nil {
		return r.getByOrderItemIDFn(ctx, id)
	}
	return nil, nil
}
func (r *testReviewRepo) GetByID(ctx context.Context, id int64) (*domain.Review, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, nil
}
func (r *testReviewRepo) ListBySellerID(ctx context.Context, storeID int64, rating, limit, offset int32) ([]*domain.Review, error) {
	if r.listBySellerIDFn != nil {
		return r.listBySellerIDFn(ctx, storeID, rating, limit, offset)
	}
	return nil, nil
}
func (r *testReviewRepo) CountBySellerID(ctx context.Context, storeID int64, rating int32) (int64, error) {
	if r.countBySellerIDFn != nil {
		return r.countBySellerIDFn(ctx, storeID, rating)
	}
	return 0, nil
}
func (r *testReviewRepo) GetStatsBySellerID(ctx context.Context, storeID int64) (*domain.SellerStats, error) {
	if r.getStatsBySellerIDFn != nil {
		return r.getStatsBySellerIDFn(ctx, storeID)
	}
	return &domain.SellerStats{}, nil
}
func (r *testReviewRepo) ListStoreRatingStats(ctx context.Context, storeIDs []int64) ([]*domain.StoreRatingStat, error) {
	if r.listStoreRatingStatsFn != nil {
		return r.listStoreRatingStatsFn(ctx, storeIDs)
	}
	return nil, nil
}
func (r *testReviewRepo) ListByBuyerID(ctx context.Context, buyerID int64, limit, offset int32) ([]*domain.Review, error) {
	if r.listByBuyerIDFn != nil {
		return r.listByBuyerIDFn(ctx, buyerID, limit, offset)
	}
	return nil, nil
}
func (r *testReviewRepo) CountByBuyerID(ctx context.Context, buyerID int64) (int64, error) {
	if r.countByBuyerIDFn != nil {
		return r.countByBuyerIDFn(ctx, buyerID)
	}
	return 0, nil
}
func (r *testReviewRepo) ListByProductID(ctx context.Context, productID int64, rating int32, withImages bool, limit, offset int32) ([]*domain.Review, error) {
	if r.listByProductIDFn != nil {
		return r.listByProductIDFn(ctx, productID, rating, withImages, limit, offset)
	}
	return nil, nil
}
func (r *testReviewRepo) CountByProductID(ctx context.Context, productID int64, rating int32, withImages bool) (int64, error) {
	if r.countByProductIDFn != nil {
		return r.countByProductIDFn(ctx, productID, rating, withImages)
	}
	return 0, nil
}
func (r *testReviewRepo) GetStatsByProductID(ctx context.Context, productID int64) (*domain.ProductStats, error) {
	if r.getStatsByProductIDFn != nil {
		return r.getStatsByProductIDFn(ctx, productID)
	}
	return &domain.ProductStats{}, nil
}
func (r *testReviewRepo) HideReview(_ context.Context, _ int64, _ string) error { return nil }
func (r *testReviewRepo) ReplyReview(ctx context.Context, reviewID, storeID int64, content string) (*domain.Review, error) {
	if r.replyReviewFn != nil {
		return r.replyReviewFn(ctx, reviewID, storeID, content)
	}
	return nil, nil
}

// ==================== Mock OrderClient ====================

type testOrderClient struct {
	getOrderForReviewFn func(ctx context.Context, orderNo string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error)
}

func (c *testOrderClient) GetOrderForReview(ctx context.Context, orderNo string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
	if c.getOrderForReviewFn != nil {
		return c.getOrderForReviewFn(ctx, orderNo)
	}
	return nil, nil, usecase.ErrOrderNotFound
}

// ==================== bufconn helper ====================

func newTestReviewClient(t *testing.T, repo domain.ReviewRepository, orderClient usecase.OrderClient) (pb.ReviewServiceClient, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer(
		gogrpc.UnaryInterceptor(interceptor.ErrorInterceptor()),
	)
	uc := usecase.NewReviewUseCase(repo, orderClient, nil, nil, nil, nil, &config.Config{}, func() int64 { return 42 }, nil)
	srv := NewReviewServer(uc, nil)
	pb.RegisterReviewServiceServer(grpcSrv, srv)
	go grpcSrv.Serve(lis) //nolint:errcheck

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	return pb.NewReviewServiceClient(conn), func() {
		_ = conn.Close()
		grpcSrv.Stop()
	}
}

// ==================== SubmitOrderReview 輸入驗證 ====================

func TestGRPC_SubmitOrderReview_InvalidRating(t *testing.T) {
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, &testOrderClient{})
	defer cleanup()

	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		OrderNo: "ORD-001", OrderItemId: "1", BuyerId: 100,
		Rating: 0,
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for invalid rating, got 0")
	}
}

func TestGRPC_SubmitOrderReview_ContentTooLong(t *testing.T) {
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, &testOrderClient{})
	defer cleanup()

	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		OrderNo: "ORD-001", OrderItemId: "1", BuyerId: 100,
		Rating:  3,
		Content: strings.Repeat("x", domain.MaxContentLength+1),
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for content too long, got 0")
	}
}

func TestGRPC_SubmitOrderReview_TooManyImages(t *testing.T) {
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, &testOrderClient{})
	defer cleanup()

	urls := make([]string, domain.MaxImageCount+1)
	for i := range urls {
		urls[i] = fmt.Sprintf("img%d", i)
	}
	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		OrderNo: "ORD-001", OrderItemId: "1", BuyerId: 100,
		Rating:       3,
		StagedTokens: urls,
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for too many images, got 0")
	}
}

// ==================== SubmitOrderReview 業務邏輯錯誤 ====================

func TestGRPC_SubmitOrderReview_AlreadyReviewed(t *testing.T) {
	repo := &testReviewRepo{
		getByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return &domain.Review{ID: 1}, nil
		},
	}
	orderClient := &testOrderClient{
		getOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return &domain.OrderEligibility{
				Status:      domain.OrderStatusCompleted,
				BuyerID:     100,
				CompletedAt: time.Now(),
			}, []*domain.OrderItemInfo{{OrderItemID: 999}}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, repo, orderClient)
	defer cleanup()

	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		Rating:      3,
		BuyerId:     100,
		OrderNo:     "ORD001",
		OrderItemId: "999",
		Price:       "100",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for already reviewed, got 0")
	}
}

// ==================== SubmitOrderReview 成功 ====================

func TestGRPC_SubmitOrderReview_Success(t *testing.T) {
	repo := &testReviewRepo{
		getByOrderItemIDFn: func(_ context.Context, _ int64) (*domain.Review, error) {
			return nil, nil
		},
		insertFn: func(_ context.Context, r *domain.Review) (*domain.Review, error) {
			return r, nil
		},
	}
	orderClient := &testOrderClient{
		getOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return &domain.OrderEligibility{
				Status:      domain.OrderStatusCompleted,
				BuyerID:     100,
				CompletedAt: time.Now(),
			}, []*domain.OrderItemInfo{{OrderItemID: 999}}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, repo, orderClient)
	defer cleanup()

	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		Rating:      3,
		BuyerId:     100,
		OrderNo:     "ORD001",
		OrderItemId: "999",
		Price:       "100",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ReviewId == "" {
		t.Error("expected non-empty ReviewId")
	}
}

// ==================== GetProductReviews ====================

func TestGRPC_GetProductReviews_Success(t *testing.T) {
	now := time.Now()
	repo := &testReviewRepo{
		listByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool, _, _ int32) ([]*domain.Review, error) {
			return []*domain.Review{{ID: 1, Rating: 4, CreatedAt: now}}, nil
		},
		countByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool) (int64, error) {
			return 1, nil
		},
		getStatsByProductIDFn: func(_ context.Context, _ int64) (*domain.ProductStats, error) {
			return &domain.ProductStats{TotalCount: 1, AvgRating: 4.0}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, repo, &testOrderClient{})
	defer cleanup()

	resp, err := client.GetProductReviews(context.Background(), &pb.GetProductReviewsRequest{
		ProductId: "123",
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Reviews) != 1 {
		t.Errorf("len(Reviews) = %d, want 1", len(resp.Reviews))
	}
	if resp.Stats.TotalCount != 1 {
		t.Errorf("Stats.TotalCount = %d, want 1", resp.Stats.TotalCount)
	}
}

// ==================== GetReview ====================

func TestGRPC_GetReview_NotFound(t *testing.T) {
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, &testOrderClient{})
	defer cleanup()

	resp, err := client.GetReview(context.Background(), &pb.GetReviewRequest{ReviewId: "9999"})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for not found review, got 0")
	}
}

// ==================== ErrorInterceptor 映射測試 ====================

func TestGRPC_SubmitOrderReview_PermissionDenied(t *testing.T) {
	orderClient := &testOrderClient{
		getOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return &domain.OrderEligibility{
				Status: domain.OrderStatusCompleted, BuyerID: 999, StoreID: 200,
				CompletedAt: time.Now().Add(-24 * time.Hour),
			}, []*domain.OrderItemInfo{{OrderItemID: 1}}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, orderClient)
	defer cleanup()

	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		OrderNo: "ORD-001", OrderItemId: "1", BuyerId: 100, // 不是 999
		ProductId: "42", Rating: 5, Content: "好", Price: "100",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for permission denied, got 0")
	}
}

func TestGRPC_SubmitOrderReview_ReviewExpired(t *testing.T) {
	buyerID := int64(100)
	orderClient := &testOrderClient{
		getOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return &domain.OrderEligibility{
				Status: domain.OrderStatusCompleted, BuyerID: buyerID, StoreID: 200,
				CompletedAt: time.Now().Add(-31 * 24 * time.Hour), // 過期
			}, []*domain.OrderItemInfo{{OrderItemID: 1}}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, orderClient)
	defer cleanup()

	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		OrderNo: "ORD-001", OrderItemId: "1", BuyerId: buyerID,
		ProductId: "42", Rating: 5, Content: "好", Price: "100",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for expired review, got 0")
	}
}

func TestGRPC_SubmitOrderReview_OrderNotCompleted(t *testing.T) {
	buyerID := int64(100)
	orderClient := &testOrderClient{
		getOrderForReviewFn: func(_ context.Context, _ string) (*domain.OrderEligibility, []*domain.OrderItemInfo, error) {
			return &domain.OrderEligibility{
				Status: "40", BuyerID: buyerID, // DELIVERED，非 COMPLETED("50")
				CompletedAt: time.Now(),
			}, []*domain.OrderItemInfo{{OrderItemID: 1}}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, orderClient)
	defer cleanup()

	resp, err := client.SubmitOrderReview(context.Background(), &pb.SubmitOrderReviewRequest{
		OrderNo: "ORD-001", OrderItemId: "1", BuyerId: buyerID,
		Rating: 5, Price: "0",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code == 0 {
		t.Fatal("expected non-zero code for order not completed, got 0")
	}
}

func TestGRPC_GetProductReviews_PlainErrorMapsToInternal(t *testing.T) {
	repo := &testReviewRepo{
		listByProductIDFn: func(_ context.Context, _ int64, _ int32, _ bool, _, _ int32) ([]*domain.Review, error) {
			return nil, errors.New("db connection failed")
		},
	}
	client, cleanup := newTestReviewClient(t, repo, &testOrderClient{})
	defer cleanup()

	_, err := client.GetProductReviews(context.Background(), &pb.GetProductReviewsRequest{
		ProductId: "42", Page: 1, PageSize: 20,
	})
	testutil.AssertGRPCCode(t, err, codes.Internal)
}

// ==================== GetSellerOrderReviewStats ====================

// regression: DB 以 float8 儲存 AVG(SMALLINT) 結果，105/22 = 4.7727... 為循環小數，
// gRPC 回傳的 AvgRating 字串必須格式化為 2 位小數。
func TestGRPC_GetSellerOrderReviewStats_RepeatDecimalRounded(t *testing.T) {
	repo := &testReviewRepo{
		getStatsBySellerIDFn: func(_ context.Context, _ int64) (*domain.SellerStats, error) {
			return &domain.SellerStats{TotalCount: 22, AvgRating: 4.7727272727272725}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, repo, &testOrderClient{})
	defer cleanup()

	resp, err := client.GetSellerOrderReviewStats(context.Background(), &pb.GetSellerOrderReviewStatsRequest{
		StoreId: "200",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
	if resp.AvgRating != "4.77" {
		t.Errorf("AvgRating = %q, want %q", resp.AvgRating, "4.77")
	}
}

// ==================== ListStoreRatingStats ====================

// 平均評分與單店版（GetSellerOrderReviewStats）一樣格式化成小數兩位；沒有評論的店不會出現。
func TestGRPC_ListStoreRatingStats_FormatsAndOmitsStoresWithoutReviews(t *testing.T) {
	var asked []int64
	repo := &testReviewRepo{
		listStoreRatingStatsFn: func(_ context.Context, ids []int64) ([]*domain.StoreRatingStat, error) {
			asked = ids
			return []*domain.StoreRatingStat{{StoreID: 200, TotalCount: 22, AvgRating: 4.7727272727272725}}, nil
		},
	}
	client, cleanup := newTestReviewClient(t, repo, &testOrderClient{})
	defer cleanup()

	resp, err := client.ListStoreRatingStats(context.Background(), &pb.ListStoreRatingStatsRequest{StoreIds: []string{"200", "300"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 || len(resp.Stats) != 1 {
		t.Fatalf("code=%d stats=%v", resp.Code, resp.Stats)
	}
	st := resp.Stats[0]
	if st.StoreId != "200" || st.TotalCount != "22" || st.AvgRating != "4.77" {
		t.Errorf("stat = %+v", st)
	}
	if len(asked) != 2 || asked[0] != 200 || asked[1] != 300 {
		t.Errorf("asked = %v", asked)
	}
}

func TestGRPC_ListStoreRatingStats_InvalidInput(t *testing.T) {
	client, cleanup := newTestReviewClient(t, &testReviewRepo{}, &testOrderClient{})
	defer cleanup()
	tooMany := make([]string, domain.MaxStoreRatingStatsBatch+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprint(i + 1)
	}
	for name, ids := range map[string][]string{
		"empty": nil, "not a number": {"200", "abc"}, "zero": {"0"}, "negative": {"-1"}, "blank": {""}, "too many": tooMany,
	} {
		resp, err := client.ListStoreRatingStats(context.Background(), &pb.ListStoreRatingStatsRequest{StoreIds: ids})
		if err != nil {
			t.Fatalf("%s: unexpected gRPC error: %v", name, err)
		}
		if resp.Code == 0 || len(resp.Stats) != 0 {
			t.Errorf("%s: code=%d stats=%v, want a non-zero code and no stats", name, resp.Code, resp.Stats)
		}
	}
}

func TestGRPC_ListStoreRatingStats_RepoErrorIsNotEmptyResult(t *testing.T) {
	repo := &testReviewRepo{
		listStoreRatingStatsFn: func(_ context.Context, _ []int64) ([]*domain.StoreRatingStat, error) {
			return nil, errors.New("db down")
		},
	}
	client, cleanup := newTestReviewClient(t, repo, &testOrderClient{})
	defer cleanup()
	resp, err := client.ListStoreRatingStats(context.Background(), &pb.ListStoreRatingStatsRequest{StoreIds: []string{"200"}})
	if err == nil && resp.Code == 0 {
		t.Fatalf("a repository failure must not look like 'no reviews': resp=%+v", resp)
	}
}

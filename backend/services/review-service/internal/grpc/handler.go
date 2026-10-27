package grpc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/blocklist"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/review"
	"github.com/Will413028/hawkr/review-service/internal/domain"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
	"github.com/redis/go-redis/v9"
)

// parseInt64 converts a §4 string id to int64 at the gRPC boundary; non-numeric
// (incl. empty) → 0, treated as not-found / invalid by the usecase.
func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func (s *ReviewServer) GetProductReviews(ctx context.Context, req *pb.GetProductReviewsRequest) (*pb.ProductReviewsResponse, error) {
	out, err := s.reviewUC.GetProductReviews(ctx, usecase.GetProductReviewsInput{
		ProductID:  parseInt64(req.ProductId),
		Rating:     req.Rating,
		WithImages: req.WithImages,
		Page:       req.Page,
		PageSize:   req.PageSize,
	})
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ProductReviewsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	// B-135: filter out reviews whose author is mutually blocked with viewer.
	// D11: aggregate Stats stays unfiltered (商品評分聚合不受個人封鎖影響).
	if req.ViewerId != 0 {
		filtered := out.Reviews[:0]
		for _, r := range out.Reviews {
			if blocklist.IsBlocked(ctx, s.rdb, req.ViewerId, r.BuyerID) {
				continue
			}
			filtered = append(filtered, r)
		}
		out.Reviews = filtered
	}

	records := make([]*pb.ReviewInfo, 0, len(out.Reviews))
	for _, r := range out.Reviews {
		records = append(records, toProtoReviewInfo(r))
	}

	pbStats := &pb.ReviewStats{}
	if out.Stats != nil {
		pbStats.TotalCount = out.Stats.TotalCount
		pbStats.AverageRating = out.Stats.AvgRating
	}

	return &pb.ProductReviewsResponse{
		Code:     0,
		Reviews:  records,
		Stats:    pbStats,
		Total:    out.Total,
		Page:     out.Page,
		PageSize: out.PageSize,
	}, nil
}

func (s *ReviewServer) GetReview(ctx context.Context, req *pb.GetReviewRequest) (*pb.ReviewResponse, error) {
	r, err := s.reviewUC.GetReview(ctx, parseInt64(req.ReviewId))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ReviewResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	if r == nil {
		return &pb.ReviewResponse{Code: int32(pkgerrors.CodeNotFound), Message: "review not found"}, nil
	}
	return &pb.ReviewResponse{
		Code:   0,
		Review: toProtoReviewInfo(r),
	}, nil
}

// ReplyReview 賣家回覆評論（#1174）。store_id 由 Gateway JWT 注入。
func (s *ReviewServer) ReplyReview(ctx context.Context, req *pb.ReplyReviewRequest) (*pb.ReviewResponse, error) {
	reviewID := parseInt64(req.ReviewId)
	if reviewID == 0 || req.StoreId == 0 {
		return &pb.ReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "review_id and store_id are required"}, nil
	}
	if req.Content == "" {
		return &pb.ReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "content is required"}, nil
	}
	r, err := s.reviewUC.ReplyReview(ctx, reviewID, req.StoreId, req.Content)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ReviewResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.ReviewResponse{
		Code:   0,
		Review: toProtoReviewInfo(r),
	}, nil
}

// ReviewServer 實作 ReviewServiceServer 中的訂單商品評論 RPCs
type ReviewServer struct {
	pb.UnimplementedReviewServiceServer
	reviewUC *usecase.ReviewUseCase
	rdb      *redis.Client
}

func NewReviewServer(reviewUC *usecase.ReviewUseCase, rdb *redis.Client) *ReviewServer {
	return &ReviewServer{reviewUC: reviewUC, rdb: rdb}
}

func (s *ReviewServer) SubmitOrderReview(ctx context.Context, req *pb.SubmitOrderReviewRequest) (*pb.SubmitOrderReviewResponse, error) {
	if req.OrderNo == "" || req.OrderItemId == "" || req.BuyerId == 0 {
		return &pb.SubmitOrderReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "order_no, order_item_id, buyer_id are required"}, nil
	}
	if req.Rating < domain.MinRating || req.Rating > domain.MaxRating {
		return &pb.SubmitOrderReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: fmt.Sprintf("rating must be between %d and %d", domain.MinRating, domain.MaxRating)}, nil
	}
	if len(req.Content) > domain.MaxContentLength {
		return &pb.SubmitOrderReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: fmt.Sprintf("content exceeds %d characters", domain.MaxContentLength)}, nil
	}
	if len(req.StagedTokens) > domain.MaxImageCount {
		return &pb.SubmitOrderReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: fmt.Sprintf("staged_tokens exceeds %d items", domain.MaxImageCount)}, nil
	}

	price, err := strconv.ParseInt(req.Price, 10, 64)
	if err != nil {
		return &pb.SubmitOrderReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid price"}, nil
	}

	rev, err := s.reviewUC.SubmitItemReview(ctx, usecase.SubmitItemReviewInput{
		OrderNo:      req.OrderNo,
		OrderItemID:  parseInt64(req.OrderItemId),
		BuyerID:      req.BuyerId,
		ProductID:    parseInt64(req.ProductId),
		Rating:       req.Rating,
		Content:      req.Content,
		StagedTokens: req.StagedTokens,
		SkuName:      req.SkuName,
		// req.ItemImageUrl 一律忽略：item image 後端從訂單商品圖快照回填（49 §5.3 凍結語意）
		Price: price,
	})
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.SubmitOrderReviewResponse{
				Code:    int32(code),
				Message: err.Error(),
			}, nil
		}
		return nil, err
	}

	return &pb.SubmitOrderReviewResponse{
		Code:     0,
		ReviewId: strconv.FormatInt(rev.ID, 10),
	}, nil
}

func (s *ReviewServer) ListMyOrderReviews(ctx context.Context, req *pb.ListMyOrderReviewsRequest) (*pb.ListMyOrderReviewsResponse, error) {
	out, err := s.reviewUC.ListMyReviews(ctx, usecase.ListMyReviewsInput{
		BuyerID:  req.BuyerId,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ListMyOrderReviewsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	records := make([]*pb.OrderReviewInfo, 0, len(out.Reviews))
	for _, r := range out.Reviews {
		records = append(records, toProtoReview(r))
	}

	return &pb.ListMyOrderReviewsResponse{
		Code:     0,
		Records:  records,
		Total:    strconv.FormatInt(out.Total, 10),
		Page:     strconv.FormatInt(int64(out.Page), 10),
		PageSize: strconv.FormatInt(int64(out.PageSize), 10),
	}, nil
}

func (s *ReviewServer) GetSellerOrderReviews(ctx context.Context, req *pb.GetSellerOrderReviewsRequest) (*pb.GetSellerOrderReviewsResponse, error) {
	out, err := s.reviewUC.GetSellerReviews(ctx, usecase.GetSellerReviewsInput{
		SellerID: parseInt64(req.StoreId),
		Rating:   req.Rating,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.GetSellerOrderReviewsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	records := make([]*pb.OrderReviewInfo, 0, len(out.Reviews))
	for _, r := range out.Reviews {
		records = append(records, toProtoReview(r))
	}

	return &pb.GetSellerOrderReviewsResponse{
		Code:     0,
		Records:  records,
		Total:    strconv.FormatInt(out.Total, 10),
		Page:     strconv.FormatInt(int64(out.Page), 10),
		PageSize: strconv.FormatInt(int64(out.PageSize), 10),
	}, nil
}

func (s *ReviewServer) GetSellerOrderReviewStats(ctx context.Context, req *pb.GetSellerOrderReviewStatsRequest) (*pb.GetSellerOrderReviewStatsResponse, error) {
	stats, err := s.reviewUC.GetSellerStats(ctx, parseInt64(req.StoreId))
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.GetSellerOrderReviewStatsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.GetSellerOrderReviewStatsResponse{
		Code:          0,
		TotalCount:    strconv.FormatInt(stats.TotalCount, 10),
		AvgRating:     fmt.Sprintf("%.2f", stats.AvgRating),
		Rating_5Count: strconv.FormatInt(stats.Rating5Count, 10),
		Rating_4Count: strconv.FormatInt(stats.Rating4Count, 10),
		Rating_3Count: strconv.FormatInt(stats.Rating3Count, 10),
		Rating_2Count: strconv.FormatInt(stats.Rating2Count, 10),
		Rating_1Count: strconv.FormatInt(stats.Rating1Count, 10),
	}, nil
}

// ListStoreRatingStats 是批次版的 GetSellerOrderReviewStats（內部呼叫）。store_ids 的任何一個不是正整數字串都是
// INVALID_INPUT（不靜默略過）；只回有評論的店。
func (s *ReviewServer) ListStoreRatingStats(ctx context.Context, req *pb.ListStoreRatingStatsRequest) (*pb.ListStoreRatingStatsResponse, error) {
	ids := make([]int64, len(req.GetStoreIds()))
	for i, raw := range req.GetStoreIds() {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return &pb.ListStoreRatingStatsResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "store_ids 必須是正整數字串"}, nil
		}
		ids[i] = id
	}
	stats, err := s.reviewUC.ListStoreRatingStats(ctx, ids)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ListStoreRatingStatsResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	out := make([]*pb.StoreRatingStat, len(stats))
	for i, st := range stats {
		out[i] = &pb.StoreRatingStat{
			StoreId:    strconv.FormatInt(st.StoreID, 10),
			TotalCount: strconv.FormatInt(st.TotalCount, 10),
			AvgRating:  fmt.Sprintf("%.2f", st.AvgRating),
		}
	}
	return &pb.ListStoreRatingStatsResponse{Code: 0, Stats: out}, nil
}

// HideReview — internal RPC called by social-service AdminResolveReport (B-135).
func (s *ReviewServer) HideReview(ctx context.Context, req *pb.HideReviewRequest) (*pb.HideReviewResponse, error) {
	if req.ReviewId == 0 {
		return &pb.HideReviewResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "review_id required"}, nil
	}
	if err := s.reviewUC.HideReview(ctx, req.ReviewId, req.HiddenReason); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.HideReviewResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.HideReviewResponse{Code: 0}, nil
}

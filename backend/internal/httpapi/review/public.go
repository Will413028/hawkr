package review

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1/reviewv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

// Public 實作 ReviewPublicService。
type Public struct {
	*presenter
	reads Reads
}

var _ reviewv1alpha1connect.ReviewPublicServiceHandler = (*Public)(nil)

// NewPublic 建立公開讀取的 handler。
func NewPublic(reads Reads, buyers Buyers, images rpc.Images, log *slog.Logger) (*Public, error) {
	p, err := newPresenter(buyers, images, log)
	if err != nil {
		return nil, err
	}
	return &Public{presenter: p, reads: reads}, nil
}

func (h *Public) ListProductReviews(ctx context.Context, req *connect.Request[pb.ListProductReviewsRequest]) (*connect.Response[pb.ListProductReviewsResponse], error) {
	msg := req.Msg
	productID, err := rpc.ParseID("product_id", msg.GetProductId())
	if err != nil {
		return nil, err
	}
	f, err := filterOf(msg.Rating, msg.GetWithImages())
	if err != nil {
		return nil, err
	}
	scope := filterScope(reviewv1alpha1connect.ReviewPublicServiceListProductReviewsProcedure, rpc.FormatID(productID), f)
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	viewer, err := h.viewer(ctx)
	if err != nil {
		return nil, err
	}
	page, err := h.reads.ListProductReviews(ctx, productID, viewer, f, after, msg.GetPageSize())
	if err != nil {
		return nil, h.failure(err)
	}
	return respond(ctx, h.presenter, page, scope, false, func(r []*pb.Review, token string) *pb.ListProductReviewsResponse {
		return &pb.ListProductReviewsResponse{Reviews: r, NextPageToken: token}
	})
}

func (h *Public) ListStoreReviews(ctx context.Context, req *connect.Request[pb.ListStoreReviewsRequest]) (*connect.Response[pb.ListStoreReviewsResponse], error) {
	msg := req.Msg
	storeID, err := rpc.ParseID("store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	f, err := filterOf(msg.Rating, msg.GetWithImages())
	if err != nil {
		return nil, err
	}
	scope := filterScope(reviewv1alpha1connect.ReviewPublicServiceListStoreReviewsProcedure, rpc.FormatID(storeID), f)
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	viewer, err := h.viewer(ctx)
	if err != nil {
		return nil, err
	}
	page, err := h.reads.ListStoreReviews(ctx, storeID, viewer, f, after, msg.GetPageSize())
	if err != nil {
		return nil, h.failure(err)
	}
	return respond(ctx, h.presenter, page, scope, false, func(r []*pb.Review, token string) *pb.ListStoreReviewsResponse {
		return &pb.ListStoreReviewsResponse{Reviews: r, NextPageToken: token}
	})
}

func (h *Public) GetStoreReviewStats(ctx context.Context, req *connect.Request[pb.GetStoreReviewStatsRequest]) (*connect.Response[pb.GetStoreReviewStatsResponse], error) {
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	s, err := h.reads.StoreStats(ctx, storeID)
	if err != nil {
		return nil, h.failure(err)
	}
	out := &pb.GetStoreReviewStatsResponse{Count: s.Count, Average: s.Average}
	for i, n := range s.Distribution {
		out.Distribution = append(out.Distribution, &pb.RatingCount{Rating: int32(i + 1), Count: n})
	}
	return connect.NewResponse(out), nil
}

func (h *Public) GetProductReviewStats(ctx context.Context, req *connect.Request[pb.GetProductReviewStatsRequest]) (*connect.Response[pb.GetProductReviewStatsResponse], error) {
	productID, err := rpc.ParseID("product_id", req.Msg.GetProductId())
	if err != nil {
		return nil, err
	}
	s, err := h.reads.ProductStats(ctx, productID)
	if err != nil {
		return nil, h.failure(err)
	}
	return connect.NewResponse(&pb.GetProductReviewStatsResponse{Count: s.Count, Average: s.Average}), nil
}

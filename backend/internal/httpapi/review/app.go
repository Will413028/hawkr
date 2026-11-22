package review

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1/reviewv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// App 實作 ReviewAppService。
type App struct {
	*presenter
	reads  Reads
	writes Writes
	stores Stores
}

var _ reviewv1alpha1connect.ReviewAppServiceHandler = (*App)(nil)

// NewApp 建立登入者的評價 handler。能力宣告表（declarations）與 ReviewAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(reads Reads, writes Writes, buyers Buyers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	return newApp(declarations, reads, writes, buyers, stores, images, log)
}

// newApp 的宣告表由參數給（測試用缺一列的表驗證啟動檢查）。
func newApp(table map[string]declaration, reads Reads, writes Writes, buyers Buyers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	p, err := newPresenter(buyers, images, log)
	if err != nil {
		return nil, err
	}
	if stores == nil {
		return nil, fmt.Errorf("httpapi/review：需要 Stores")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{presenter: p, reads: reads, writes: writes, stores: stores}, nil
}

// buyerID 回傳呼叫者的買家身分；只有顧客帳號能呼叫（沒有登入由 interceptor 擋，管理員回 permission_denied）。
func (h *App) buyerID(ctx context.Context) (int64, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return 0, err
	}
	id, err := h.buyers.CurrentBuyerID(ctx, p.ID)
	if err != nil {
		return 0, rpc.BuyerInvariant(p.ID, err)
	}
	return id, nil
}

// store 回傳呼叫者此刻的店（seller.CurrentStore）；沒有店回 not_found。賣家能力不在這裡檢查：由模組檢查，宣告見 declarations。
func (h *App) store(ctx context.Context) (seller.Store, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return seller.Store{}, err
	}
	return h.stores.CurrentStore(ctx, p.ID)
}

// replyStore 由評價所屬的店推導回覆的店家（讀取不套公開述詞）：評價不存在回 not_found；呼叫者不是那間店的店主（包含沒有店）時，
// 評價公開可讀回 permission_denied，不公開（隱藏、作者停用）回 not_found，不洩漏存在性；店主一律交給模組，不能回覆的由模組回 REVIEW_NOT_REPLYABLE。
// 賣家能力與店家一致的檢查仍在模組。
func (h *App) replyStore(ctx context.Context, reviewID uuid.UUID) (seller.Store, error) {
	p, err := rpc.RequireAccount(ctx)
	if err != nil {
		return seller.Store{}, err
	}
	ref, err := h.reads.ReviewStore(ctx, reviewID)
	if err != nil {
		return seller.Store{}, err
	}
	st, err := h.stores.Store(ctx, ref.StoreID)
	if err != nil {
		return seller.Store{}, err
	}
	if st.AccountID != p.ID {
		if !ref.Public {
			return seller.Store{}, apperr.NotFound("找不到這則評價")
		}
		return seller.Store{}, apperr.PermissionDenied("不是你的店的評價")
	}
	return st, nil
}

func (h *App) GetReview(ctx context.Context, req *connect.Request[pb.GetReviewRequest]) (*connect.Response[pb.GetReviewResponse], error) {
	id, err := rpc.ParseUUID("review_id", req.Msg.GetReviewId())
	if err != nil {
		return nil, err
	}
	viewer, err := h.buyerID(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.reads.GetReview(ctx, id, viewer)
	if err != nil {
		return nil, h.failure(err)
	}
	reviews, err := h.toReviews(ctx, []review.Review{r}, false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetReviewResponse{Review: reviews[0]}), nil
}

func (h *App) ReplyReview(ctx context.Context, req *connect.Request[pb.ReplyReviewRequest]) (*connect.Response[pb.ReplyReviewResponse], error) {
	id, err := rpc.ParseUUID("review_id", req.Msg.GetReviewId())
	if err != nil {
		return nil, err
	}
	st, err := h.replyStore(ctx, id)
	if err != nil {
		return nil, err
	}
	r, err := h.writes.Reply(ctx, review.ReplyInput{Store: st, ReviewID: id, Content: req.Msg.GetContent()})
	if err != nil {
		return nil, h.failure(err)
	}
	reviews, err := h.toReviews(ctx, []review.Review{r}, true)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ReplyReviewResponse{Review: reviews[0]}), nil
}

func (h *App) CreateReview(ctx context.Context, req *connect.Request[pb.CreateReviewRequest]) (*connect.Response[pb.CreateReviewResponse], error) {
	msg := req.Msg
	itemID, err := rpc.ParseID("order_item_id", msg.GetOrderItemId())
	if err != nil {
		return nil, err
	}
	buyerID, err := h.buyerID(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.writes.Submit(ctx, review.SubmitInput{
		BuyerID: buyerID, OrderNo: msg.GetOrderNo(), OrderItemID: itemID, Rating: int(msg.GetRating()),
		Content: msg.GetContent(), StagedTokens: msg.GetImageStagedTokens(),
	})
	if err != nil {
		return nil, h.failure(err)
	}
	reviews, err := h.toReviews(ctx, []review.Review{r}, true)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.CreateReviewResponse{Review: reviews[0]}), nil
}

func (h *App) ListMyReviews(ctx context.Context, req *connect.Request[pb.ListMyReviewsRequest]) (*connect.Response[pb.ListMyReviewsResponse], error) {
	msg := req.Msg
	var itemID int64
	if msg.OrderItemId != nil {
		var err error
		if itemID, err = rpc.ParseID("order_item_id", msg.GetOrderItemId()); err != nil {
			return nil, err
		}
	}
	buyerID, err := h.buyerID(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.FilterScope(reviewv1alpha1connect.ReviewAppServiceListMyReviewsProcedure, rpc.FormatID(buyerID), rpc.FormatID(itemID))
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.reads.ListMyReviews(ctx, buyerID, itemID, after, msg.GetPageSize())
	if err != nil {
		return nil, h.failure(err)
	}
	return respond(ctx, h.presenter, page, scope, true, func(r []*pb.Review, token string) *pb.ListMyReviewsResponse {
		return &pb.ListMyReviewsResponse{Reviews: r, NextPageToken: token}
	})
}

func (h *App) ListOwnStoreReviews(ctx context.Context, req *connect.Request[pb.ListOwnStoreReviewsRequest]) (*connect.Response[pb.ListOwnStoreReviewsResponse], error) {
	msg := req.Msg
	st, err := h.store(ctx)
	if err != nil {
		return nil, err
	}
	f, err := filterOf(msg.Rating, msg.GetWithImages())
	if err != nil {
		return nil, err
	}
	scope := filterScope(reviewv1alpha1connect.ReviewAppServiceListOwnStoreReviewsProcedure, rpc.FormatID(st.StoreID), f)
	after, err := decodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.reads.ListOwnStoreReviews(ctx, st.StoreID, f, after, msg.GetPageSize())
	if err != nil {
		return nil, h.failure(err)
	}
	return respond(ctx, h.presenter, page, scope, true, func(r []*pb.Review, token string) *pb.ListOwnStoreReviewsResponse {
		return &pb.ListOwnStoreReviewsResponse{Reviews: r, NextPageToken: token}
	})
}

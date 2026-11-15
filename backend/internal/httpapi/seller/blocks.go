package seller

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// blockModule 是店家封鎖的模組 API（*seller.Module，A7-1b）。
type blockModule interface {
	BlockBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
	UnblockBuyer(ctx context.Context, storeID, buyerID int64) (bool, error)
	ListBlockedBuyers(context.Context, model.BlockedBuyerListInput) (model.BlockedBuyerPage, error)
}

// BlockBuyer 以呼叫者店家的身分封鎖一位買家：參照由 social 以店主的讀取規則解析，而且要是這家店自己的內容；作者必須是買家，
// 買家要存在而且沒有關閉，再經跨身分規則擋下店主本人的買家身分。
func (h *App) BlockBuyer(ctx context.Context, req *connect.Request[pb.BlockBuyerRequest]) (*connect.Response[pb.BlockBuyerResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	ref := blocks.Ref{CommentID: req.Msg.GetCommentId(), ForumEntryID: req.Msg.GetForumEntryId()}
	author, err := blocks.ResolveAuthor(ctx, h.blocking.Authors, social.Caller{Store: &current}, ref)
	if err != nil {
		return nil, err
	}
	// 解析器在店主讀不到時退回匿名的公開讀取：別家店的內容不是這家店能封鎖的對象，與讀不到同一個答案。
	if author.StoreID != current.StoreID {
		return nil, apperr.NotFound("找不到這則內容")
	}
	if author.Author.Kind != social.ActorBuyer {
		return nil, apperr.InvalidInput("只能封鎖買家")
	}
	buyerID := author.Author.ID
	if err := blocks.LiveBuyer(ctx, h.followers, buyerID); err != nil {
		return nil, err
	}
	if err := h.blocking.Identities.RejectSamePerson(ctx, buyerID, current.StoreID); err != nil {
		return nil, err
	}
	changed, err := h.m.BlockBuyer(ctx, current.StoreID, buyerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.BlockBuyerResponse{Changed: changed}), nil
}

func (h *App) UnblockBuyer(ctx context.Context, req *connect.Request[pb.UnblockBuyerRequest]) (*connect.Response[pb.UnblockBuyerResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	buyerID, err := rpc.ParseID("buyer_id", req.Msg.GetBuyerId())
	if err != nil {
		return nil, err
	}
	changed, err := h.m.UnblockBuyer(ctx, current.StoreID, buyerID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnblockBuyerResponse{Changed: changed}), nil
}

func (h *App) ListBlockedBuyers(ctx context.Context, req *connect.Request[pb.ListBlockedBuyersRequest]) (*connect.Response[pb.ListBlockedBuyersResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(sellerv1alpha1connect.SellerAppServiceListBlockedBuyersProcedure, current.StoreID)
	after, err := rpc.ValidCursor[model.BlockedBuyerCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListBlockedBuyers(ctx, model.BlockedBuyerListInput{StoreID: current.StoreID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(page.Items))
	for i, item := range page.Items {
		ids[i] = item.BuyerID
	}
	views, err := blocks.BuyerViews(ctx, h.followers, ids)
	if err != nil {
		return nil, err
	}
	out := &pb.ListBlockedBuyersResponse{BlockedBuyers: make([]*pb.BlockedBuyer, len(page.Items))}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	for i, item := range page.Items {
		v := views[item.BuyerID]
		card := &pb.BuyerCard{Id: rpc.FormatID(item.BuyerID), Deleted: v.Deleted}
		if !v.Deleted {
			card.DisplayName = v.DisplayName
			if v.Avatar != nil {
				if err := rpc.ObjectKey(*v.Avatar); err != nil {
					return nil, err
				}
				card.Avatar = h.images.Image(*v.Avatar)
			}
		}
		out.BlockedBuyers[i] = &pb.BlockedBuyer{Buyer: card, CreateTime: timestamppb.New(item.BlockedAt)}
	}
	return connect.NewResponse(out), nil
}

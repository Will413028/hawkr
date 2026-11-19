package buyer

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// blockModule 是個人封鎖的模組 API（*buyer.Module，A7-1b）加上名單的買家顯示資料。
type blockModule interface {
	BlockBuyer(ctx context.Context, buyerID, targetBuyerID int64) (bool, error)
	UnblockBuyer(ctx context.Context, buyerID, targetBuyerID int64) (bool, error)
	BlockStore(ctx context.Context, buyerID, storeID int64) (bool, error)
	UnblockStore(ctx context.Context, buyerID, storeID int64) (bool, error)
	ListBlockedBuyers(context.Context, model.BlockedBuyerListInput) (model.BlockedBuyerPage, error)
	ListBlockedStores(context.Context, model.BlockedStoreListInput) (model.BlockedStorePage, error)
	BatchDisplay(ctx context.Context, ids []int64) ([]model.Display, error)
}

// BlockAuthor 封鎖一則內容的作者：參照由 social 以呼叫者的觀看規則解析（讀不到是 not_found），作者是買家走買家封鎖，是店家走店家封鎖。
func (h *App) BlockAuthor(ctx context.Context, req *connect.Request[pb.BlockAuthorRequest]) (*connect.Response[pb.BlockAuthorResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	ref := blocks.Ref{CommentID: req.Msg.GetCommentId(), ForumEntryID: req.Msg.GetForumEntryId()}
	author, err := blocks.ResolveAuthor(ctx, h.blocking.Authors, social.Caller{BuyerID: buyerID}, ref)
	if err != nil {
		return nil, err
	}
	var changed bool
	switch author.Author.Kind {
	case social.ActorBuyer:
		changed, err = h.m.BlockBuyer(ctx, buyerID, author.Author.ID)
	case social.ActorStore:
		changed, err = h.blockStore(ctx, buyerID, author.Author.ID)
	default:
		return nil, apperr.NotFound("找不到這則內容")
	}
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.BlockAuthorResponse{Changed: changed}), nil
}

func (h *App) BlockStore(ctx context.Context, req *connect.Request[pb.BlockStoreRequest]) (*connect.Response[pb.BlockStoreResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	changed, err := h.blockStore(ctx, buyerID, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.BlockStoreResponse{Changed: changed}), nil
}

// blockStore 是買家封鎖店的唯一入口：店家要存在而且曾經公開過（buyer 不檢查店是否存在；沒有卡片的店不洩漏存在與否，一律 not_found），
// 再經跨身分規則擋下封鎖自己的店。
func (h *App) blockStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	cards, err := h.storeCards(ctx, []int64{storeID})
	if err != nil {
		return false, err
	}
	if cards[storeID] == nil {
		return false, apperr.NotFound("找不到店家")
	}
	if err := h.blocking.Identities.RejectSamePerson(ctx, buyerID, storeID); err != nil {
		return false, err
	}
	return h.m.BlockStore(ctx, buyerID, storeID)
}

func (h *App) UnblockBuyer(ctx context.Context, req *connect.Request[pb.UnblockBuyerRequest]) (*connect.Response[pb.UnblockBuyerResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	target, err := rpc.ParseID("buyer_id", req.Msg.GetBuyerId())
	if err != nil {
		return nil, err
	}
	changed, err := h.m.UnblockBuyer(ctx, buyerID, target)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnblockBuyerResponse{Changed: changed}), nil
}

func (h *App) UnblockStore(ctx context.Context, req *connect.Request[pb.UnblockStoreRequest]) (*connect.Response[pb.UnblockStoreResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	storeID, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	changed, err := h.m.UnblockStore(ctx, buyerID, storeID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.UnblockStoreResponse{Changed: changed}), nil
}

func (h *App) ListBlockedBuyers(ctx context.Context, req *connect.Request[pb.ListBlockedBuyersRequest]) (*connect.Response[pb.ListBlockedBuyersResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListBlockedBuyersProcedure, buyerID)
	after, err := rpc.ValidCursor[model.BlockedBuyerCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListBlockedBuyers(ctx, model.BlockedBuyerListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(page.Items))
	for i, item := range page.Items {
		ids[i] = item.BuyerID
	}
	views, err := blocks.BuyerViews(ctx, h.m, ids)
	if err != nil {
		return nil, err
	}
	out := &pb.ListBlockedBuyersResponse{BlockedBuyers: make([]*pb.BlockedBuyer, len(page.Items))}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	for i, item := range page.Items {
		v := views[item.BuyerID]
		row := &pb.BlockedBuyer{BuyerId: rpc.FormatID(item.BuyerID), Deleted: v.Deleted, DisplayName: v.DisplayName, CreateTime: timestamppb.New(item.Time)}
		if v.Avatar != nil {
			if err := rpc.ObjectKey(*v.Avatar); err != nil {
				return nil, err
			}
			row.Avatar = h.images.Image(*v.Avatar)
		}
		out.BlockedBuyers[i] = row
	}
	return connect.NewResponse(out), nil
}

func (h *App) ListBlockedStores(ctx context.Context, req *connect.Request[pb.ListBlockedStoresRequest]) (*connect.Response[pb.ListBlockedStoresResponse], error) {
	buyerID, err := h.caller(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListBlockedStoresProcedure, buyerID)
	after, err := rpc.ValidCursor[model.BlockedStoreCursor](scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.m.ListBlockedStores(ctx, model.BlockedStoreListInput{BuyerID: buyerID, PageSize: req.Msg.GetPageSize(), After: after})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(page.Items))
	for i, item := range page.Items {
		ids[i] = item.StoreID
	}
	stores, err := h.storeCards(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := &pb.ListBlockedStoresResponse{BlockedStores: make([]*pb.BlockedStore, len(page.Items))}
	if page.Next != nil {
		out.NextPageToken = rpc.TokenFor(scope, page.Next)
	}
	for i, item := range page.Items {
		out.BlockedStores[i] = &pb.BlockedStore{StoreId: rpc.FormatID(item.StoreID), Store: stores[item.StoreID], CreateTime: timestamppb.New(item.Time)}
	}
	return connect.NewResponse(out), nil
}

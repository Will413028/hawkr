package buyer

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	httpcatalog "github.com/Will413028/hawkr/backend/internal/httpapi/catalog"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1/buyerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// FollowedProducts 是「追蹤中」動態的來源（B4 的 D24 Q2）：呼叫者追蹤的店家（buyer 擁有追蹤關係），與經 catalog 的 Go API 讀這些店家的公開商品。
// 追蹤的店家清單與商品過濾都在這裡之外：這裡不自己判斷店家可不可曝光，那是 catalog 依請求的 clock 做的。
type FollowedProducts struct {
	Follows interface {
		FollowedStoreIDs(ctx context.Context, buyerID int64) ([]int64, error)
	}
	Catalog interface {
		ListPublicProductsOfStores(ctx context.Context, at time.Time, storeIDs []int64, after *catalog.ListCursor, size int32) (catalog.ProductPage, error)
	}
	Cards *httpcatalog.Presenter
	// Now 是請求的 clock。
	Now func() time.Time
}

// Complete 回報是否全部設好；NewHandler 組裝時一定設好。
func (f FollowedProducts) Complete() bool {
	return f.Follows != nil && f.Catalog != nil && f.Cards != nil && f.Now != nil
}

// ErrFollowedProductsRequired 是 ListFollowedStoreProducts 在沒有組裝來源時的錯誤：組裝錯誤，不是使用者的錯。
var ErrFollowedProductsRequired = fmt.Errorf("httpapi/buyer：ListFollowedStoreProducts 需要 Ports.Followed")

// ListFollowedStoreProducts 列出呼叫者追蹤的店家的公開商品，最新上架在前。沒有追蹤任何店家回空清單。
func (a *App) ListFollowedStoreProducts(ctx context.Context, req *connect.Request[pb.ListFollowedStoreProductsRequest]) (*connect.Response[pb.ListFollowedStoreProductsResponse], error) {
	f := a.ports.Followed
	if !f.Complete() {
		return nil, ErrFollowedProductsRequired
	}
	buyerID, err := a.caller(ctx)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.GetPageSize() < 0 {
		return nil, rpc.WithFieldPaths(apperr.InvalidInputOn([]string{"page_size"}, "page_size 不得為負數"), rpc.FieldPaths{"page_size": "page_size"})
	}
	scope := rpc.PageScope(buyerv1alpha1connect.BuyerAppServiceListFollowedStoreProductsProcedure, buyerID)
	after, err := httpcatalog.DecodeCursor(scope, msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	stores, err := f.Follows.FollowedStoreIDs(ctx, buyerID)
	if err != nil {
		return nil, err
	}
	page, err := f.Catalog.ListPublicProductsOfStores(ctx, f.Now(), stores, after, msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	products, err := f.Cards.Summaries(ctx, page.Items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListFollowedStoreProductsResponse{Products: products, NextPageToken: httpcatalog.NextToken(scope, page.Next)}), nil
}

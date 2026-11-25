package buyer

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
)

// ListFollowedStoreProducts 的 handler 隨 catalog 的 Connect 接線（B4-3b）實作；契約已在 api/ 定義，在那之前回 unimplemented。
func (a *App) ListFollowedStoreProducts(context.Context, *connect.Request[pb.ListFollowedStoreProductsRequest]) (*connect.Response[pb.ListFollowedStoreProductsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("buyer: ListFollowedStoreProducts is not wired yet"))
}

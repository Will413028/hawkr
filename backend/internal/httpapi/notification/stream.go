package notification

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
)

// WatchNotifications 還沒有串流實作，回 unimplemented。
func (h *App) WatchNotifications(context.Context, *connect.Request[pb.WatchNotificationsRequest], *connect.ServerStream[pb.WatchNotificationsResponse]) error {
	return rpc.Error(connect.CodeUnimplemented, "%s", "串流尚未提供")
}

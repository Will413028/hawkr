package notification

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
)

// streamLifetime 是一條串流的最長壽命（估計值，不是量測值）：session 被撤銷或店家換了，最慢這麼久後連線結束、client 重連時重新驗證。
const streamLifetime = 15 * time.Minute

// WatchNotifications 訂閱呼叫者在指定受眾的訊號：連線建立先送一則 inbox 訊號，之後每有變化再送；STORE 受眾另外承載賣家工作區事件。
// 受眾與店家的檢查同 unary 方法（recipient）：只有顧客帳號能呼叫，STORE 受眾沒有現任店家回 not_found、沒有 CapNotification 被拒；超過每帳號上限回 resource_exhausted。
// 協定沒有心跳訊號，也沒有續傳：壽命到了正常結束，client 重連後先重讀列表。
func (h *App) WatchNotifications(ctx context.Context, req *connect.Request[pb.WatchNotificationsRequest], st *connect.ServerStream[pb.WatchNotificationsResponse]) error {
	r, err := h.recipient(ctx, req.Msg.GetAudience())
	if err != nil {
		return err
	}
	params := notification.WatchParams{AccountID: r.AccountID, Audience: r.Audience, StoreID: r.StoreID}
	sub, err := h.notifications.Watch(params)
	if err != nil {
		return err
	}
	defer sub.Close()
	ctx, cancel := context.WithTimeout(ctx, h.lifetime())
	defer cancel()
	for {
		ev, err := sub.Next(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		out, err := toResponse(ev)
		if err != nil {
			return err
		}
		if err := st.Send(out); err != nil {
			return err
		}
	}
}

func toResponse(ev notification.StreamEvent) (*pb.WatchNotificationsResponse, error) {
	if ev.Inbox != nil {
		in := &pb.InboxSignal{UnreadCount: ev.Inbox.UnreadCount}
		if ev.Inbox.LatestItemID != nil {
			id := ev.Inbox.LatestItemID.String()
			in.LatestInboxItemId = &id
		}
		for _, h := range ev.Inbox.Hints {
			hint, err := hintToProto(h)
			if err != nil {
				return nil, err
			}
			in.StateRefreshHints = append(in.StateRefreshHints, hint)
		}
		return &pb.WatchNotificationsResponse{Event: &pb.WatchNotificationsResponse_Inbox{Inbox: in}}, nil
	}
	var change pb.SellerWorkspaceChange
	switch ev.Workspace {
	case notification.SellerOrderChanged:
		change = pb.SellerWorkspaceChange_SELLER_WORKSPACE_CHANGE_ORDER
	case notification.SellerAfterSaleChanged:
		change = pb.SellerWorkspaceChange_SELLER_WORKSPACE_CHANGE_AFTER_SALE
	default:
		return nil, fmt.Errorf("httpapi/notification：不認得的賣家工作區變更 %d", int(ev.Workspace))
	}
	return &pb.WatchNotificationsResponse{Event: &pb.WatchNotificationsResponse_SellerWorkspace{SellerWorkspace: &pb.SellerWorkspaceSignal{Change: change}}}, nil
}

func hintToProto(h notification.StateHint) (pb.StateRefreshHint, error) {
	switch h {
	case notification.StateHintOrder:
		return pb.StateRefreshHint_STATE_REFRESH_HINT_ORDER, nil
	case notification.StateHintAfterSale:
		return pb.StateRefreshHint_STATE_REFRESH_HINT_AFTER_SALE, nil
	case notification.StateHintSellerStatus:
		return pb.StateRefreshHint_STATE_REFRESH_HINT_SELLER_STATUS, nil
	case notification.StateHintNone:
	}
	return 0, fmt.Errorf("httpapi/notification：不認得的狀態重讀提示 %q", h)
}

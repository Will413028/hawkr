package order

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"google.golang.org/grpc"
)

type Client struct {
	client orderpb.OrderServiceClient
}

func NewClient(cfg *config.Config) (*Client, error) {
	conn, err := grpcclient.Dial(cfg.OrderService.Addr,
		// 內部身分 HMAC 簽章（#1366），比照 coupon client 的既有寫法。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("crm-service", false)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to order-service at %s: %w", cfg.OrderService.Addr, err)
	}
	return &Client{client: orderpb.NewOrderServiceClient(conn)}, nil
}

// GetOrderByNo resolves store_id/buyer identity for a single order_no.
//
// 🔴 用 GetOrderForAfterSales 而不是 SearchOrders（#3367）：後者的 order_no 是
// **fuzzy/LIKE**（proto 欄位註解自己寫著「模糊搜尋」），而我方只取第一頁 10 筆
// ⇒ 目標不在前 10 筆時會回「找不到」，而那與「訂單真的不存在」在呼叫端**完全無法
// 分辨**。dev 上這條路徑累積了 306 則死信（`AFTERSALES_REFUND_COMPLETED` 重試耗盡）。
//
// GetOrderForAfterSales 的 server 端走 `orderRepository.findByOrderNo` 精確查找，
// 授權面 OrderGrpcService 的註解明載「內部呼叫（x-internal-service），無權限檢查」，
// 且既有 caller 除 aftersales 外還有 coupon-service ⇒ 它不是售後專用的 RPC。
//
// ⚠️ **尚未做的那一半**（#3367）：訂單不存在時 server 回 NOT_FOUND(404)，那是
// **永久性失敗**，重試多少次都一樣。目前仍與暫時性失敗走同一條路（回 error ⇒ 退避
// 重試 ⇒ 耗盡進 DLQ），因為要分辨它得先決定 404 這個通用碼在 Go 側放哪
// （`pkg/errors/bizcode` 目前只有各 domain 專屬的 NotFound，沒有通用的），
// 以及「退款完成事件到達但訂單不存在」該如何處置——後者需要業務判斷。
const notFoundCode = 404

func (c *Client) GetOrderByNo(ctx context.Context, orderNo string) (*usecase.OrderInfo, error) {
	resp, err := c.client.GetOrderForAfterSales(ctx, &orderpb.GetOrderForAfterSalesRequest{
		OrderNo: orderNo,
	})
	if err != nil {
		return nil, fmt.Errorf("get order by order_no=%s: %w", orderNo, err)
	}
	if resp.GetCode() == notFoundCode {
		return nil, fmt.Errorf("get order by order_no=%s: server error code=%d msg=%s: %w",
			orderNo, resp.GetCode(), resp.GetMessage(), usecase.ErrOrderNotFound)
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("get order by order_no=%s: server error code=%d msg=%s", orderNo, resp.GetCode(), resp.GetMessage())
	}
	return orderInfoFromView(resp.GetData(), orderNo)
}

// orderInfoFromView 把 proto view 折成 usecase 型別。
//
// 抽成純函式是為了讓欄位解析有測試覆蓋：GetOrderByNo 本體是一次 gRPC 呼叫，
// 沒有 mock 就測不到，而解析失敗（store_id/buyer_id 不是數字、data 為空）
// 正是會在執行期炸掉的那幾格。
func orderInfoFromView(view *orderpb.OrderAfterSalesView, orderNo string) (*usecase.OrderInfo, error) {
	if view == nil {
		return nil, fmt.Errorf("order_no %q: server returned code=0 with empty data", orderNo)
	}
	storeID, err := strconv.ParseInt(view.GetStoreId(), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse store_id %q for order_no=%s: %w", view.GetStoreId(), orderNo, err)
	}
	buyerID, err := strconv.ParseInt(view.GetBuyerId(), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse buyer_id %q for order_no=%s: %w", view.GetBuyerId(), orderNo, err)
	}
	return &usecase.OrderInfo{OrderNo: view.GetOrderNo(), StoreID: storeID, BuyerID: buyerID}, nil
}

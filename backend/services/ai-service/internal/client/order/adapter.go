package order

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/orderprovisioning"
	"github.com/Will413028/hawkr/ai-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Client 是 order-service 的 gRPC 轉接層。
//
// ⚠️ 它同時滿足兩個**不同消費端**的窄介面，所以型別本身必須是導出的（di.go 分別把它
// 綁成 usecase.OrderClient 與 orderprovisioning.RfqOrderCreator）：前者是 agent tool
// 的讀取面，後者是 outbox worker 的建單面。⛔ 不要把 CreateRfqOrder 加進
// usecase.OrderClient——那個介面是 agent tool 的契約，worker 不是 agent。
type Client struct {
	client orderpb.OrderServiceClient
}

// parseTimeToMs 將 ISO 8601 時間字串轉為 Unix 毫秒。
func parseTimeToMs(s string) int64 {
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UnixMilli()
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ms
	}
	return 0
}

// NewOrderClient creates a gRPC client to order-service.
func NewOrderClient(cfg *config.Config) (*Client, error) {
	target := fmt.Sprintf("%s:%s", cfg.OrderServiceHost, cfg.OrderServicePort)
	conn, err := grpcclient.Dial(target,
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
		// 內部身分 HMAC 簽章（#1366）；accountId 由各 method 設在 outgoing x-account-id，
		// 故此處 forwardAccountHeaders=false（不從 incoming 轉發），interceptor 讀 outgoing 綁進簽章。
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("ai-service", false)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to order-service at %s: %w", target, err)
	}
	return &Client{client: orderpb.NewOrderServiceClient(conn)}, nil
}

// GetSellerOrders queries a seller's orders. storeID is already the caller's
// CURRENT store_id (resolved by gateway before this call — account_id 命名債正名),
// not a raw login account_id needing resolution here.
func (c *Client) GetSellerOrders(ctx context.Context, storeID identity.StoreID, statusFilter string, page, pageSize int) ([]*usecase.SellerOrder, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-account-id", strconv.FormatInt(int64(storeID), 10)))

	resp, err := c.client.GetSellerOrders(ctx, &orderpb.GetSellerOrdersRequest{
		StoreId:      int64(storeID),
		StatusFilter: statusFilter,
		Page: &orderpb.PageRequest{
			Page: strconv.Itoa(page),
			Size: strconv.Itoa(pageSize),
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("get seller orders failed: [%d] %s", resp.GetCode(), resp.GetMessage())
	}

	orders := make([]*usecase.SellerOrder, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		amt, _ := strconv.ParseInt(o.GetTotalActualAmount(), 10, 64)
		createMs := parseTimeToMs(o.GetCreateTime())

		items := make([]usecase.SellerOrderItem, 0, len(o.GetItems()))
		for _, item := range o.GetItems() {
			qty, _ := strconv.Atoi(item.GetQuantity())
			items = append(items, usecase.SellerOrderItem{
				ProductName: item.GetProductName(),
				SkuName:     item.GetSkuName(),
				Quantity:    qty,
				SkuID:       item.GetSkuId(),
			})
		}

		orders = append(orders, &usecase.SellerOrder{
			OrderNo:      o.GetOrderNo(),
			Status:       o.GetStatus(),
			Amount:       amt,
			CreateTimeMs: createMs,
			BuyerName:    o.GetRecipientName(),
			Items:        items,
		})
	}
	return orders, nil
}

// GetMyOrders queries a buyer's orders. buyerID is already the caller's CURRENT
// buyer_id (resolved by gateway before this call — account_id 命名債正名), not a
// raw login account_id needing resolution here.
func (c *Client) GetMyOrders(ctx context.Context, buyerID identity.BuyerID, statusFilter string, page, pageSize int) ([]*usecase.BuyerOrder, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-account-id", strconv.FormatInt(int64(buyerID), 10)))

	resp, err := c.client.GetMyOrders(ctx, &orderpb.GetMyOrdersRequest{
		BuyerId:      int64(buyerID),
		StatusFilter: statusFilter,
		Page: &orderpb.PageRequest{
			Page: strconv.Itoa(page),
			Size: strconv.Itoa(pageSize),
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("get my orders failed: [%d] %s", resp.GetCode(), resp.GetMessage())
	}

	orders := make([]*usecase.BuyerOrder, 0, len(resp.GetOrders()))
	for _, o := range resp.GetOrders() {
		amt, _ := strconv.ParseInt(o.GetTotalActualAmount(), 10, 64)
		createMs := parseTimeToMs(o.GetCreateTime())
		itemCount, _ := strconv.Atoi(o.GetItemCount())
		orders = append(orders, &usecase.BuyerOrder{
			OrderNo:      o.GetOrderNo(),
			Status:       o.GetStatus(),
			Amount:       amt,
			CreateTimeMs: createMs,
			SellerName:   o.GetSellerName(),
			ItemCount:    itemCount,
		})
	}
	return orders, nil
}

// GetBuyerOrderDetail queries a single order for the buyer. buyerID is already
// the caller's CURRENT buyer_id (resolved by gateway before this call), not a
// raw login account_id needing resolution here.
func (c *Client) GetBuyerOrderDetail(ctx context.Context, buyerID identity.BuyerID, orderNo string) (*usecase.BuyerOrderDetail, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-account-id", strconv.FormatInt(int64(buyerID), 10)))

	resp, err := c.client.GetOrder(ctx, &orderpb.GetOrderRequest{
		BuyerId: int64(buyerID),
		OrderNo: orderNo,
	})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("order %s: [%d] %s", orderNo, resp.GetCode(), resp.GetMessage())
	}

	d := resp.GetData()
	if d == nil {
		return nil, fmt.Errorf("order %s not found", orderNo)
	}

	amt, _ := strconv.ParseInt(d.GetTotalActualAmount(), 10, 64)
	shippingFee, _ := strconv.ParseInt(d.GetShippingFee(), 10, 64)
	createMs := parseTimeToMs(d.GetCreateTime())
	return &usecase.BuyerOrderDetail{
		OrderNo:        d.GetOrderNo(),
		Status:         d.GetStatus(),
		Amount:         amt,
		CreateTimeMs:   createMs,
		SellerName:     d.GetSellerName(),
		ShippingMethod: d.GetShippingMethod(),
		ShippingStatus: d.GetShippingStatus(),
		ShippingFee:    shippingFee,
		RecipientName:  d.GetRecipientName(),
		PaymentMethod:  d.GetPaymentMethod(),
	}, nil
}

// GetSellerOrderDetail queries a single order for the seller. storeID is
// already the caller's CURRENT store_id (resolved by gateway before this
// call), not a raw login account_id needing resolution here.
func (c *Client) GetSellerOrderDetail(ctx context.Context, storeID identity.StoreID, orderNo string) (*usecase.OrderDetailSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-account-id", strconv.FormatInt(int64(storeID), 10)))

	resp, err := c.client.GetSellerOrderDetail(ctx, &orderpb.GetSellerOrderDetailRequest{
		StoreId: int64(storeID),
		OrderNo: orderNo,
	})
	if err != nil {
		return nil, err
	}
	if resp.GetCode() != 0 {
		return nil, fmt.Errorf("order %s: [%d] %s", orderNo, resp.GetCode(), resp.GetMessage())
	}

	d := resp.GetData()
	if d == nil {
		return nil, fmt.Errorf("order %s not found", orderNo)
	}

	amt, _ := strconv.ParseInt(d.GetTotalActualAmount(), 10, 64)
	return &usecase.OrderDetailSummary{
		OrderNo:        d.GetOrderNo(),
		Status:         d.GetStatus(),
		Amount:         amt,
		BuyerID:        d.GetBuyerId(),
		ShipmentNo:     d.GetShipmentNo(),
		ShippingStatus: d.GetShippingStatus(),
	}, nil
}

// CreateRfqOrder 投遞 RFQ 成交建單命令（doc 64 §6.4）。
//
// ⛔ **不設 SkuId**：RFQ 明細是 non-catalog line（§6.4.9），缺值在 wire 上就是 proto3
// string 的 ""，order-service 端映成 NULL。§6.4.15 已明文「不改成 proto3 optional、
// 不動任何 .proto」——本 repo 對字串型 optional 的既有慣例就是空字串，同一支呼叫裡的
// delivery_temp 就是這樣寫的。
//
// ⚠️ 也不設 DeliveryTemp：acceptance 與 canonical payload 都沒有溫層這個欄位（實查
// procurement schema 零命中），沒有來源就不要編一個——它是選填欄位。
//
// 回應的 code 原樣帶回給呼叫端分類（終態拒絕 vs 可重試），這裡只負責轉接：⛔ 不在這
// 一層把非 0 變成 error，那會把「訂單已建、只是冪等命中」與「被拒絕」壓成同一種東西。
func (c *Client) CreateRfqOrder(ctx context.Context, cmd orderprovisioning.RfqOrderCommand) (orderprovisioning.RfqOrderResult, error) {
	ctx, cancel := context.WithTimeout(ctx, grpcclient.Timeout5s)
	defer cancel()

	resp, err := c.client.CreateRfqOrder(ctx, &orderpb.CreateRfqOrderRequest{
		AcceptanceId: strconv.FormatInt(cmd.AcceptanceID, 10),
		BuyerId:      int64(cmd.BuyerID),
		StoreId:      strconv.FormatInt(cmd.StoreID, 10),
		TotalAmount:  strconv.FormatInt(cmd.TotalAmount, 10),
		ShippingFee:  strconv.FormatInt(cmd.ShippingFee, 10),
		SkuName:      cmd.SkuName,
		UnitPrice:    strconv.FormatInt(cmd.UnitPrice, 10),
		Lane:         cmd.Lane,
	})
	if err != nil {
		return orderprovisioning.RfqOrderResult{}, err
	}

	out := orderprovisioning.RfqOrderResult{
		Code:     resp.GetCode(),
		Message:  resp.GetMessage(),
		OrderNo:  resp.GetData().GetOrderNo(),
		Replayed: resp.GetData().GetReplayed(),
	}
	if deadline := resp.GetData().GetPaymentDeadline(); deadline != "" {
		parsed, parseErr := time.Parse(time.RFC3339, deadline)
		if parseErr != nil {
			// 付款期限解不開就不要猜一個：它會被寫進 acceptance 給買家看。
			return orderprovisioning.RfqOrderResult{}, fmt.Errorf("parse payment deadline %q: %w", deadline, parseErr)
		}
		out.PaymentDeadline = parsed
	}
	return out, nil
}

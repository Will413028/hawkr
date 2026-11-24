package orderservice

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"

	orderpb "github.com/Will413028/hawkr/backend/internal/legacy/gen/order"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/order/orderconnect"
)

type fakeBuyerOrders struct {
	orderconnect.OrderServiceClient
	list   *orderpb.GetMyOrdersResponse
	detail *orderpb.GetOrderResponse
	rfq    *orderpb.CreateRfqOrderResponse
	err    error
	seen   *seenBuyerCalls
}

type seenBuyerCalls struct {
	list   *orderpb.GetMyOrdersRequest
	detail *orderpb.GetOrderRequest
	rfq    *orderpb.CreateRfqOrderRequest
	header http.Header
}

func (f fakeBuyerOrders) GetMyOrders(_ context.Context, req *connect.Request[orderpb.GetMyOrdersRequest]) (*connect.Response[orderpb.GetMyOrdersResponse], error) {
	f.seen.list, f.seen.header = req.Msg, req.Header()
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.list), nil
}

func (f fakeBuyerOrders) GetOrder(_ context.Context, req *connect.Request[orderpb.GetOrderRequest]) (*connect.Response[orderpb.GetOrderResponse], error) {
	f.seen.detail, f.seen.header = req.Msg, req.Header()
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.detail), nil
}

func (f fakeBuyerOrders) CreateRfqOrder(_ context.Context, req *connect.Request[orderpb.CreateRfqOrderRequest]) (*connect.Response[orderpb.CreateRfqOrderResponse], error) {
	f.seen.rfq = req.Msg
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.rfq), nil
}

func TestMyOrders(t *testing.T) {
	seen := &seenBuyerCalls{}
	f := fakeBuyerOrders{seen: seen, list: &orderpb.GetMyOrdersResponse{Orders: []*orderpb.OrderSummaryView{
		{OrderNo: "O1", CreateTime: "2023-11-14T22:13:20.123Z", Status: "10", TotalActualAmount: "1250", ItemCount: "3", SellerName: "店"},
	}}}
	got, err := (&Client{rpc: f}).MyOrders(context.Background(), 7, "10,20", 2, 5)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	o := got[0]
	if o.OrderNo != "O1" || o.Status != "10" || o.Amount != 1250 || o.ItemCount != 3 || o.SellerName != "店" || o.CreateTime.UnixMilli() != 1700000000123 {
		t.Fatalf("order = %+v", o)
	}
	r := seen.list
	if r.GetBuyerId() != 7 || r.GetStatusFilter() != "10,20" || r.GetPage().GetPage() != "2" || r.GetPage().GetSize() != "5" || seen.header.Get("x-account-id") != "7" {
		t.Fatalf("request = %v header=%v", r, seen.header)
	}
}

func TestOrderTimeAcceptsIsoAndMillisecondFallback(t *testing.T) {
	for raw, want := range map[string]int64{
		"2023-11-14T22:13:20Z":        1700000000000,
		"2023-11-14T22:13:20.123456Z": 1700000000123,
		"2023-11-15T06:13:20+08:00":   1700000000000,
		"1700000000000":               1700000000000,
	} {
		got, err := orderTime(raw)
		if err != nil || got.UnixMilli() != want {
			t.Errorf("orderTime(%q) = %v, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"", "yesterday", "-1", "2023-11-14"} {
		if _, err := orderTime(raw); err == nil {
			t.Errorf("orderTime(%q) accepted", raw)
		}
	}
}

func TestMyOrdersRejectsBadData(t *testing.T) {
	good := func() *orderpb.OrderSummaryView {
		return &orderpb.OrderSummaryView{OrderNo: "O1", CreateTime: "1", TotalActualAmount: "1", ItemCount: "1"}
	}
	for name, mutate := range map[string]func(*orderpb.OrderSummaryView){
		"no order no": func(o *orderpb.OrderSummaryView) { o.OrderNo = "" },
		"bad amount":  func(o *orderpb.OrderSummaryView) { o.TotalActualAmount = "x" },
		"bad time":    func(o *orderpb.OrderSummaryView) { o.CreateTime = "" },
		"bad count":   func(o *orderpb.OrderSummaryView) { o.ItemCount = "-1" },
	} {
		o := good()
		mutate(o)
		f := fakeBuyerOrders{seen: &seenBuyerCalls{}, list: &orderpb.GetMyOrdersResponse{Orders: []*orderpb.OrderSummaryView{o}}}
		if _, err := (&Client{rpc: f}).MyOrders(context.Background(), 7, "", 1, 5); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	for name, f := range map[string]fakeBuyerOrders{
		"code":      {list: &orderpb.GetMyOrdersResponse{Code: 500}},
		"transport": {err: errors.New("offline")},
	} {
		f.seen = &seenBuyerCalls{}
		if _, err := (&Client{rpc: f}).MyOrders(context.Background(), 7, "", 1, 5); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := (&Client{rpc: fakeBuyerOrders{seen: &seenBuyerCalls{}}}).MyOrders(context.Background(), 0, "", 1, 5); err == nil {
		t.Error("zero buyer accepted")
	}
}

func TestBuyerOrderDetail(t *testing.T) {
	seen := &seenBuyerCalls{}
	f := fakeBuyerOrders{seen: seen, detail: &orderpb.GetOrderResponse{Data: &orderpb.OrderDetailView{
		OrderNo: "O1", Status: "20", TotalActualAmount: "900", ShippingFee: "100", CreateTime: "2023-11-14T22:13:20Z",
		SellerName: "店", ShippingMethod: "3001", RecipientName: "收", PaymentMethod: "card",
	}}}
	d, err := (&Client{rpc: f}).BuyerOrderDetail(context.Background(), 7, "O1")
	if err != nil {
		t.Fatal(err)
	}
	want := BuyerOrderDetail{OrderNo: "O1", Status: "20", SellerName: "店", ShippingMethod: "3001", RecipientName: "收", PaymentMethod: "card", Amount: 900, CreateTime: d.CreateTime, ShippingFee: 100}
	if d != want || d.CreateTime.UnixMilli() != 1700000000000 {
		t.Fatalf("detail = %+v", d)
	}
	if seen.detail.GetBuyerId() != 7 || seen.detail.GetOrderNo() != "O1" || seen.header.Get("x-account-id") != "7" {
		t.Fatalf("request = %v header=%v", seen.detail, seen.header)
	}
	f.detail = &orderpb.GetOrderResponse{}
	if _, err := (&Client{rpc: f}).BuyerOrderDetail(context.Background(), 7, "O1"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("missing data err = %v", err)
	}
	f.detail = &orderpb.GetOrderResponse{Code: 404}
	if _, err := (&Client{rpc: f}).BuyerOrderDetail(context.Background(), 7, "O1"); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("NOT_FOUND code err = %v", err)
	}
	f.detail = &orderpb.GetOrderResponse{Code: 500}
	if _, err := (&Client{rpc: f}).BuyerOrderDetail(context.Background(), 7, "O1"); err == nil || errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("other code err = %v", err)
	}
	f.detail = &orderpb.GetOrderResponse{Data: &orderpb.OrderDetailView{OrderNo: "O1", TotalActualAmount: "x"}}
	if _, err := (&Client{rpc: f}).BuyerOrderDetail(context.Background(), 7, "O1"); err == nil {
		t.Fatal("bad amount accepted")
	}
}

func rfqCommand() RfqOrderCommand {
	return RfqOrderCommand{AcceptanceID: 99, BuyerID: 7, StoreID: 5, TotalAmount: 1100, ShippingFee: 100, UnitPrice: 1000, SkuName: "高麗菜 12.5 公斤", Lane: "PLATFORM"}
}

func TestCreateRfqOrder(t *testing.T) {
	seen := &seenBuyerCalls{}
	f := fakeBuyerOrders{seen: seen, rfq: &orderpb.CreateRfqOrderResponse{Data: &orderpb.CreateRfqOrderData{OrderNo: "O9", PaymentDeadline: "2026-11-20T10:00:00Z", Replayed: true}}}
	got, err := (&Client{rpc: f}).CreateRfqOrder(context.Background(), rfqCommand())
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderNo != "O9" || !got.Replayed || got.PaymentDeadline.IsZero() || got.Code != 0 {
		t.Fatalf("result = %+v", got)
	}
	r := seen.rfq
	if r.GetAcceptanceId() != "99" || r.GetBuyerId() != 7 || r.GetStoreId() != "5" || r.GetTotalAmount() != "1100" || r.GetShippingFee() != "100" ||
		r.GetUnitPrice() != "1000" || r.GetSkuName() != "高麗菜 12.5 公斤" || r.GetLane() != "PLATFORM" || r.GetSkuId() != "" || r.GetDeliveryTemp() != "" {
		t.Fatalf("request = %v", r)
	}
}

func TestCreateRfqOrderKeepsRejectionCodeAndRejectsBadInput(t *testing.T) {
	f := fakeBuyerOrders{seen: &seenBuyerCalls{}, rfq: &orderpb.CreateRfqOrderResponse{Code: 4001, Message: "rejected"}}
	got, err := (&Client{rpc: f}).CreateRfqOrder(context.Background(), rfqCommand())
	if err != nil || got.Code != 4001 || got.Message != "rejected" || got.OrderNo != "" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	f.rfq = &orderpb.CreateRfqOrderResponse{Data: &orderpb.CreateRfqOrderData{OrderNo: "O9", PaymentDeadline: "tomorrow"}}
	if _, err := (&Client{rpc: f}).CreateRfqOrder(context.Background(), rfqCommand()); err == nil {
		t.Error("bad deadline accepted")
	}
	f.rfq = &orderpb.CreateRfqOrderResponse{}
	if _, err := (&Client{rpc: f}).CreateRfqOrder(context.Background(), rfqCommand()); err == nil {
		t.Error("success without order number accepted")
	}
	f.err = errors.New("offline")
	if _, err := (&Client{rpc: f}).CreateRfqOrder(context.Background(), rfqCommand()); err == nil {
		t.Error("transport error swallowed")
	}
	for name, mutate := range map[string]func(*RfqOrderCommand){
		"acceptance": func(c *RfqOrderCommand) { c.AcceptanceID = 0 },
		"buyer":      func(c *RfqOrderCommand) { c.BuyerID = 0 },
		"store":      func(c *RfqOrderCommand) { c.StoreID = 0 },
		"sku name":   func(c *RfqOrderCommand) { c.SkuName = " " },
		"lane":       func(c *RfqOrderCommand) { c.Lane = "" },
	} {
		cmd := rfqCommand()
		mutate(&cmd)
		if _, err := (&Client{rpc: fakeBuyerOrders{seen: &seenBuyerCalls{}}}).CreateRfqOrder(context.Background(), cmd); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

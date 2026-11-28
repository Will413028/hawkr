package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const (
	checkoutCodeUnavailable   int32 = 4001
	checkoutCodePriceChanged  int32 = 4091
	checkoutCodeAlreadyOrder  int32 = 4092
	checkoutPendingOrderNoMsg       = "您已有未付款訂單：O20260831001，請先完成付款或取消後再下單"
)

type checkoutPreserveServer struct {
	orderpb.UnimplementedOrderServiceServer
	preview *orderpb.CheckoutPreviewResponse
	confirm *orderpb.CheckoutConfirmResponse
}

func (s *checkoutPreserveServer) CheckoutPreview(context.Context, *orderpb.CheckoutPreviewRequest) (*orderpb.CheckoutPreviewResponse, error) {
	return s.preview, nil
}

func (s *checkoutPreserveServer) CheckoutConfirm(context.Context, *orderpb.CheckoutConfirmRequest) (*orderpb.CheckoutConfirmResponse, error) {
	return s.confirm, nil
}

func newCheckoutPreserveRouter(t *testing.T, srvImpl *checkoutPreserveServer) http.Handler {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := gogrpc.NewServer()
	orderpb.RegisterOrderServiceServer(srv, srvImpl)
	go srv.Serve(lis) //nolint:errcheck

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithChainUnaryInterceptor(grpcclient.EnvelopeGuardInterceptor()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })

	h := &Holder{
		Order:   grpcclient.NewOrderClientForTest(orderpb.NewOrderServiceClient(conn)),
		BuyerID: buyerGrandfatherResolverForTest(),
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	app := r.Group("/api/v1/app/checkout")
	app.Use(fakeJWTAuth())
	{
		app.POST("/preview", h.CheckoutPreview)
		app.POST("/confirm", h.CheckoutConfirm)
	}
	return r
}

func postCheckout(t *testing.T, r http.Handler, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func previewBody() map[string]any {
	return map[string]any{
		"shippingMethod": "TAQBIN",
		"deliveryTemp":   "CHILLED",
		"items":          []map[string]string{{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"}},
	}
}

func confirmBody() map[string]any {
	b := previewBody()
	b["checkoutId"] = "CK-0001"
	b["recipientName"] = "王小明"
	b["recipientPhone"] = "0912345678"
	return b
}

func unavailableItemsFixture() []*orderpb.UnavailableItem {
	return []*orderpb.UnavailableItem{{
		SkuId:        "SKU001",
		SkuName:      "有機高麗菜",
		Reason:       "INSUFFICIENT_STOCK",
		RequestedQty: "2",
		AvailableQty: "1",
	}}
}

func assertCheckoutEnvelope(t *testing.T, w *httptest.ResponseRecorder, wantHTTP int, wantCode int32) map[string]any {
	t.Helper()
	if w.Code != wantHTTP {
		t.Fatalf("HTTP status = %d, want %d；body=%s", w.Code, wantHTTP, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v；raw=%s", err, w.Body.String())
	}
	got, _ := body["code"].(float64)
	if int32(got) != wantCode {
		t.Fatalf("envelope code = %v, want %d；body=%s", body["code"], wantCode, w.Body.String())
	}
	return body
}

func dataObj(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	d, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 不是物件或不存在 —— preserveData 的 data 在邊界上蒸發了；body=%v", body)
	}
	return d
}

func TestCheckoutPreview_4001_UnavailableItemsSurviveTheGrpcBoundary(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		preview: &orderpb.CheckoutPreviewResponse{
			Code:    checkoutCodeUnavailable,
			Message: "商品不可購買",
			Data:    &orderpb.CheckoutPreviewData{UnavailableItems: unavailableItemsFixture()},
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/preview", previewBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, checkoutCodeUnavailable)

	if body["message"] != "商品不可購買" {
		t.Errorf("message = %v, want 商品不可購買", body["message"])
	}
	items, ok := dataObj(t, body)["unavailableItems"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("data.unavailableItems 應有 1 筆，得到 %v；body=%s", dataObj(t, body)["unavailableItems"], w.Body.String())
	}
	it, _ := items[0].(map[string]any)
	for field, want := range map[string]string{
		"skuId": "SKU001", "skuName": "有機高麗菜", "reason": "INSUFFICIENT_STOCK",
		"requestedQty": "2", "availableQty": "1",
	} {
		if it[field] != want {
			t.Errorf("unavailableItems[0].%s = %v, want %q（前端靠這幾欄畫對話框）", field, it[field], want)
		}
	}
}

func TestCheckoutConfirm_4001_UnavailableItemsSurviveTheGrpcBoundary(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		confirm: &orderpb.CheckoutConfirmResponse{
			Code:    checkoutCodeUnavailable,
			Message: "商品不可購買",
			Data:    &orderpb.CheckoutConfirmData{UnavailableItems: unavailableItemsFixture()},
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/confirm", confirmBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, checkoutCodeUnavailable)

	items, ok := dataObj(t, body)["unavailableItems"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("data.unavailableItems 應有 1 筆；body=%s", w.Body.String())
	}
}

func TestCheckoutConfirm_4091_PriceChangedDetailSurvives(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		confirm: &orderpb.CheckoutConfirmResponse{
			Code:    checkoutCodePriceChanged,
			Message: "訂單金額已變動",
			PriceChanged: &orderpb.CheckoutPriceChangedDetail{
				OldTotalAmount: "480",
				NewTotalAmount: "520",
				ChangeReasons:  "PRODUCT_PRICE,SHIPPING_FEE",
				NewSubtotal:    "370",
				NewShippingFee: "150",
				NewDiscount:    "0",
			},
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/confirm", confirmBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, checkoutCodePriceChanged)

	pc, ok := dataObj(t, body)["priceChanged"].(map[string]any)
	if !ok {
		t.Fatalf("data.priceChanged 不見了 —— 前端要靠它顯示新舊金額；body=%s", w.Body.String())
	}
	for field, want := range map[string]string{
		"oldTotalAmount": "480", "newTotalAmount": "520",
		"changeReasons": "PRODUCT_PRICE,SHIPPING_FEE",
		"newSubtotal":   "370", "newShippingFee": "150", "newDiscount": "0",
	} {
		if pc[field] != want {
			t.Errorf("priceChanged.%s = %v, want %q", field, pc[field], want)
		}
	}
}

func TestCheckoutConfirm_4092_MessageCarriesOrderNoAndStaysHTTP200(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		confirm: &orderpb.CheckoutConfirmResponse{
			Code:    checkoutCodeAlreadyOrder,
			Message: checkoutPendingOrderNoMsg,
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/confirm", confirmBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, checkoutCodeAlreadyOrder)

	if body["message"] != checkoutPendingOrderNoMsg {
		t.Errorf("message = %v, want %q（orderNo 在 message 裡，前端解析它導向付款頁）",
			body["message"], checkoutPendingOrderNoMsg)
	}
}

func TestCheckoutPreview_4001_WithoutItemsFallsToGrpcResultShape(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		preview: &orderpb.CheckoutPreviewResponse{
			Code:    checkoutCodeUnavailable,
			Message: "商品不可購買",
			Data:    &orderpb.CheckoutPreviewData{},
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/preview", previewBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, checkoutCodeUnavailable)

	if _, exists := body["data"]; exists {
		t.Errorf("GrpcResult 的非零分支不該帶 data（既有行為），得到 %v", body["data"])
	}
}

func TestCheckoutPreview_SuccessUnchangedByCarrier(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		preview: &orderpb.CheckoutPreviewResponse{
			Code:    0,
			Message: "success",
			Data: &orderpb.CheckoutPreviewData{
				CheckoutId:        "CK-0001",
				TotalActualAmount: "480",
			},
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/preview", previewBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, 0)

	if body["message"] != "success" {
		t.Errorf("message = %v, want success", body["message"])
	}
	d := dataObj(t, body)
	if d["checkoutId"] != "CK-0001" || d["totalActualAmount"] != "480" {
		t.Errorf("成功回應的 data 被動到了：%v", d)
	}
}

func TestCheckoutConfirm_SuccessUnchangedByCarrier(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		confirm: &orderpb.CheckoutConfirmResponse{
			Code:    0,
			Message: "success",
			Data: &orderpb.CheckoutConfirmData{
				CheckoutId:     "CK-0001",
				OrderNos:       []string{"O20260831001"},
				TotalPayAmount: "480",
			},
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/confirm", confirmBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, 0)

	d := dataObj(t, body)
	nos, ok := d["orderNos"].([]any)
	if !ok || len(nos) != 1 || nos[0] != "O20260831001" {
		t.Errorf("成功回應的 orderNos 被動到了：%v", d["orderNos"])
	}
}

func bizcarrierCounterSum(t *testing.T, name string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	var sum float64
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			sum += metricValue(m)
		}
	}
	return sum
}

func metricValue(m *dto.Metric) float64 {
	if c := m.GetCounter(); c != nil {
		return c.GetValue()
	}
	return 0
}

// TestCheckoutHotPathNeverRelIesOnCarrierRewrite 斷言：結帳的三個 preserveData 碼
// **是 handler 自己送出去的**，不是被 carrier 改寫出來的。
//
// 為什麼這條不是多餘的：body 斷言分不出「handler 寫對了」與「handler 寫錯但被
// carrier 改正了」—— 兩者逐位元相同（writeBizEnvelope 是共用寫出點，那是刻意的）。
// 唯一分得出來的是計數器。swallowed 一旦在結帳路徑上跳動，代表 checkout.go 的顯式
// 分支失效、熱路徑改成依賴安全網 —— 那是必須有人知道的變化，即使前端看不出來。
func TestCheckoutHotPathNeverReliesOnCarrierRewrite(t *testing.T) {
	beforeSwallowed := bizcarrierCounterSum(t, "gateway_bizcode_swallowed_total")
	beforeEscaped := bizcarrierCounterSum(t, "gateway_bizcode_escaped_total")

	cases := []struct {
		name string
		path string
		body map[string]any
		srv  *checkoutPreserveServer
	}{
		{"preview-4001", "/api/v1/app/checkout/preview", previewBody(), &checkoutPreserveServer{
			preview: &orderpb.CheckoutPreviewResponse{Code: checkoutCodeUnavailable, Message: "商品不可購買",
				Data: &orderpb.CheckoutPreviewData{UnavailableItems: unavailableItemsFixture()}},
		}},
		{"confirm-4001", "/api/v1/app/checkout/confirm", confirmBody(), &checkoutPreserveServer{
			confirm: &orderpb.CheckoutConfirmResponse{Code: checkoutCodeUnavailable, Message: "商品不可購買",
				Data: &orderpb.CheckoutConfirmData{UnavailableItems: unavailableItemsFixture()}},
		}},
		{"confirm-4091", "/api/v1/app/checkout/confirm", confirmBody(), &checkoutPreserveServer{
			confirm: &orderpb.CheckoutConfirmResponse{Code: checkoutCodePriceChanged, Message: "訂單金額已變動",
				PriceChanged: &orderpb.CheckoutPriceChangedDetail{OldTotalAmount: "480", NewTotalAmount: "520"}},
		}},
		{"confirm-4092", "/api/v1/app/checkout/confirm", confirmBody(), &checkoutPreserveServer{
			confirm: &orderpb.CheckoutConfirmResponse{Code: checkoutCodeAlreadyOrder, Message: checkoutPendingOrderNoMsg},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newCheckoutPreserveRouter(t, tc.srv)
			w := postCheckout(t, r, tc.path, tc.body)
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200；body=%s", w.Code, w.Body.String())
			}
		})
	}

	if got := bizcarrierCounterSum(t, "gateway_bizcode_swallowed_total"); got != beforeSwallowed {
		t.Errorf("gateway_bizcode_swallowed_total 從 %v 變成 %v —— 結帳熱路徑開始依賴 carrier 的自動改正，"+
			"代表 checkout.go 的顯式業務碼分支失效了（body 看不出來，只有這個計數器會動）",
			beforeSwallowed, got)
	}
	if got := bizcarrierCounterSum(t, "gateway_bizcode_escaped_total"); got != beforeEscaped {
		t.Errorf("gateway_bizcode_escaped_total 從 %v 變成 %v —— 結帳回應繞過了 internal/response",
			beforeEscaped, got)
	}
}

// ---------------------------------------------------------------------------
// checkout.go 唯一真正到得了 `Biz(c, 0, ...)` 的分支
// ---------------------------------------------------------------------------
//
// checkout.go 的三個 response.Biz 呼叫點傳的是 resp.Code（不是字面 0），所以它們
// **不是**第十種繞道的實例——但 CheckoutConfirm 的 priceChanged 分支
// （`if resp.PriceChanged != nil && resp.PriceChanged.NewTotalAmount != ""`）
// **沒有檢查 resp.Code**，所以 order-service 若回「code=0 ＋ PriceChanged 已填」，
// 這裡就會真的呼叫 Biz(c, 0, ...) —— 熱路徑上唯一到得了那個形狀的地方。
//
// 本測試釘住它在 carrier idle 時的輸出逐欄不變。carrier armed 時它**應該**被改寫
// （那正是第十種繞道要擋的東西），而今天不可能 armed：同一支 method 回 code=0 會
// Disarm，而這個 handler 在 arm 之後不打第二支 RPC。
func TestCheckoutConfirm_ZeroCodeWithPriceChanged_ShapeUnchanged(t *testing.T) {
	r := newCheckoutPreserveRouter(t, &checkoutPreserveServer{
		confirm: &orderpb.CheckoutConfirmResponse{
			Code:         0,
			Message:      "success",
			PriceChanged: &orderpb.CheckoutPriceChangedDetail{NewTotalAmount: "520", OldTotalAmount: "480"},
		},
	})

	w := postCheckout(t, r, "/api/v1/app/checkout/confirm", confirmBody())
	body := assertCheckoutEnvelope(t, w, http.StatusOK, 0)

	pc, ok := dataObj(t, body)["priceChanged"].(map[string]any)
	if !ok {
		t.Fatalf("data.priceChanged 不見了；body=%s", w.Body.String())
	}
	if pc["newTotalAmount"] != "520" || pc["oldTotalAmount"] != "480" {
		t.Errorf("priceChanged 內容被動到了：%v", pc)
	}
}

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/gin-gonic/gin"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// ---------------------------------------------------------------------------
// 這個檔案問一個問題：**結帳被打壞了沒有？**
// ---------------------------------------------------------------------------
//
// 既有的 checkout_preservedata_e2e_test.go 斷言的是「形狀對不對」——它自己寫下
// 期望值，然後檢查回應符合期望。那證明得了「現在的行為符合我認為正確的樣子」，
// 證明不了「現在的行為與改動之前逐位元相同」。兩者的差別正是回歸測試的全部價值：
// 期望值是我寫的，我如果對舊行為的理解本身就錯了，斷言會跟著錯。
//
// 所以本檔不寫期望值，改成**兩套 stack 跑同一份 proto fake，比對 body bytes**：
//
//	legacy  ＝ 改動前的 gateway：interceptor 對 preserveData 碼無條件 return nil
//	          ＋ 完全沒有 bizcarrier.Middleware（⇒ response 層的 guard 全程 no-op）
//	current ＝ 現在的 gateway：production 的 EnvelopeGuardInterceptor ＋ bizcarrier.Middleware
//
// legacy 那半邊的 interceptor 是從 `git diff gateway/internal/grpc/
// envelope_guard_interceptor.go` 的**被刪除行**逐行抄回來的（見 legacyEnvelopeGuard
// 的註解），不是我對舊行為的印象。
//
// 第三套 stack（noCarrier ＝ production interceptor 但沒掛 middleware）是
// **Arm 失敗的劇本**：它不是假想的，它就是「有人把 r.Use(bizcarrier.Middleware())
// 移到 SetupRoutes 之後」「有人在另一個 engine 上掛了結帳路由」「有人把 checkout
// 的 RPC 改成吃 detached context」這三件事共同的可觀測結果。本檔把那個結果的
// **實際 body** 記下來，讓「降級有多痛」不再是形容詞。

// ---------------------------------------------------------------------------
// legacy replica
// ---------------------------------------------------------------------------

// legacyPreserveData 是改動前 envelopeCodePreserveData 名單裡與結帳有關的三個碼。
// 本檔只餵這三個碼與 0，所以複製這三筆就足以忠實重現舊行為；其餘碼走下面的
// fallback（與舊碼同一支 `{FailedPrecondition, preserveBizCode:true}`）。
var legacyPreserveData = map[int32]bool{4001: true, 4091: true, 4092: true}

// legacyEnvelopeGuard 是改動**前**的 EnvelopeGuardInterceptor。
//
// 逐行對照 `git diff` 的被刪除行：
//
//	if !ok || resp.GetCode() == 0 { return nil }        // 合成一條，且沒有 Disarm
//	code := resp.GetCode()
//	msg  := ...
//	if envelopeCodePreserveData[code] { return nil }    // 🔴 無條件，沒有 Arm
//	mapping, found := envelopeCodeToGRPC[code]
//	if !found { mapping = {FailedPrecondition, preserveBizCode:true} }
//	...
func legacyEnvelopeGuard() gogrpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *gogrpc.ClientConn, invoker gogrpc.UnaryInvoker, opts ...gogrpc.CallOption) error {
		if err := invoker(ctx, method, req, reply, cc, opts...); err != nil {
			return err
		}
		resp, ok := reply.(interface{ GetCode() int32 })
		if !ok || resp.GetCode() == 0 {
			return nil
		}
		code := resp.GetCode()
		msg := fmt.Sprintf("service error (code=%d)", code)
		if m, ok := reply.(interface{ GetMessage() string }); ok && m.GetMessage() != "" {
			msg = m.GetMessage()
		}
		if legacyPreserveData[code] {
			return nil
		}
		return status.Error(codes.FailedPrecondition, fmt.Sprintf("[%d] %s", code, msg))
	}
}

// ---------------------------------------------------------------------------
// 可腳本化的 order-service 替身（四支結帳 RPC 全含）
// ---------------------------------------------------------------------------

type checkoutScript struct {
	preview     *orderpb.CheckoutPreviewResponse
	confirm     *orderpb.CheckoutConfirmResponse
	validate    *orderpb.CheckoutValidateResponse
	shippingFee *orderpb.CheckoutShippingFeeResponse
	// byCheckoutID 讓同一台 server 依請求內容回不同腳本，供併發污染測試使用。
	byCheckoutID map[string]*orderpb.CheckoutConfirmResponse
}

type checkoutGoldenServer struct {
	orderpb.UnimplementedOrderServiceServer
	s *checkoutScript
}

func (g *checkoutGoldenServer) CheckoutPreview(context.Context, *orderpb.CheckoutPreviewRequest) (*orderpb.CheckoutPreviewResponse, error) {
	return g.s.preview, nil
}

func (g *checkoutGoldenServer) CheckoutConfirm(_ context.Context, req *orderpb.CheckoutConfirmRequest) (*orderpb.CheckoutConfirmResponse, error) {
	if g.s.byCheckoutID != nil {
		if r, ok := g.s.byCheckoutID[req.GetCheckoutId()]; ok {
			return r, nil
		}
	}
	return g.s.confirm, nil
}

func (g *checkoutGoldenServer) CheckoutValidate(context.Context, *orderpb.CheckoutValidateRequest) (*orderpb.CheckoutValidateResponse, error) {
	return g.s.validate, nil
}

func (g *checkoutGoldenServer) CheckoutShippingFee(context.Context, *orderpb.CheckoutShippingFeeRequest) (*orderpb.CheckoutShippingFeeResponse, error) {
	return g.s.shippingFee, nil
}

type stackMode int

const (
	stackLegacy    stackMode = iota // 改動前
	stackCurrent                    // 改動後（production 接線）
	stackNoCarrier                  // 改動後但 carrier 缺席 ⇒ Arm 回 false
)

func newCheckoutGoldenRouter(t *testing.T, s *checkoutScript, mode stackMode) http.Handler {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := gogrpc.NewServer()
	orderpb.RegisterOrderServiceServer(srv, &checkoutGoldenServer{s: s})
	go srv.Serve(lis) //nolint:errcheck

	itc := grpcclient.EnvelopeGuardInterceptor()
	if mode == stackLegacy {
		itc = legacyEnvelopeGuard()
	}
	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithChainUnaryInterceptor(itc),
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
	if mode == stackCurrent {
		// production 掛在 newEngine 的全域鏈上（infrastructure/di.go）；
		// legacy 沒有它，noCarrier 刻意不掛。
		r.Use(bizcarrier.Middleware())
	}
	app := r.Group("/api/v1/app/checkout")
	app.Use(fakeJWTAuth())
	{
		app.POST("/validate", h.CheckoutValidate)
		app.POST("/shipping_fee", h.CheckoutShippingFee)
		app.POST("/preview", h.CheckoutPreview)
		app.POST("/confirm", h.CheckoutConfirm)
	}
	return r
}

func goldenPOST(t *testing.T, r http.Handler, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	// 🔴 fakeJWTAuth 靠這個 header 認人；漏掉的話每一條請求都在 handler 之前就被
	// 401 擋掉，而「401 vs 401」逐位元相同 —— 本檔第一版就是這樣 12/12 全綠地
	// 什麼都沒測到（量測要有鑑別力）。assertReachedHandler 是為此加的護欄。
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// assertReachedHandler 擋住「兩邊都被 middleware 攔掉，於是逐位元相同」這種假綠。
func assertReachedHandler(t *testing.T, w *httptest.ResponseRecorder, who string) {
	t.Helper()
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusNotFound {
		t.Fatalf("%s 的請求沒進到 handler（HTTP %d，body=%s）——這一輪比對零鑑別力",
			who, w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// fixtures（形狀全部來自 api/proto/order/order.proto，不是手寫的「我以為的」回應）
// ---------------------------------------------------------------------------

func goldenUnavailable() []*orderpb.UnavailableItem {
	// 四個 reason 全覆蓋：proto 的註解列的就是這四個
	// （DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK / LOOKUP_FAILED）。
	return []*orderpb.UnavailableItem{
		{SkuId: "510", SkuName: "有機高麗菜", Reason: "INSUFFICIENT_STOCK", RequestedQty: "5", AvailableQty: "2"},
		{SkuId: "511", SkuName: "溫室小黃瓜", Reason: "OUT_OF_STOCK", RequestedQty: "1", AvailableQty: "0"},
		{SkuId: "512", SkuName: "下架的芭樂", Reason: "DELISTED", RequestedQty: "3", AvailableQty: ""},
		{SkuId: "513", SkuName: "", Reason: "LOOKUP_FAILED", RequestedQty: "2", AvailableQty: ""},
	}
}

func goldenPriceChanged() *orderpb.CheckoutPriceChangedDetail {
	return &orderpb.CheckoutPriceChangedDetail{
		OldTotalAmount: "1055",
		NewTotalAmount: "1180",
		ChangeReasons:  "PRODUCT_PRICE,SHIPPING_FEE",
		NewSubtotal:    "1080",
	}
}

func goldenPreviewOK() *orderpb.CheckoutPreviewResponse {
	return &orderpb.CheckoutPreviewResponse{Code: 0, Message: "success", Data: &orderpb.CheckoutPreviewData{
		CheckoutId:          "58275c66-0000-4000-8000-000000000001",
		TotalOriginalAmount: "1000",
		TotalShippingFee:    "55",
		TotalDiscount:       "0",
		TotalActualAmount:   "1055",
		PaymentMerId:        "HAWK2685914682",
		AvailablePaymentMethods: []*orderpb.AvailablePaymentMethod{
			{PaymentType: "2004", Eligible: true},
			{PaymentType: "2001", Eligible: false, IneligibleReason: "2102"},
		},
	}}
}

func goldenConfirmOK() *orderpb.CheckoutConfirmResponse {
	return &orderpb.CheckoutConfirmResponse{Code: 0, Message: "success", Data: &orderpb.CheckoutConfirmData{
		CheckoutId:     "58275c66-0000-4000-8000-000000000001",
		OrderNos:       []string{"O664700149"},
		TotalPayAmount: "1055",
	}}
}

func goldenPreviewReq() map[string]any {
	return map[string]any{
		"shippingMethod": "3002", "deliveryTemp": "3201", "receiverZipCode": "100",
		"items": []map[string]any{{"skuId": "510", "quantity": "5", "productId": "875563677398107445"}},
	}
}

func goldenConfirmReq(checkoutID string) map[string]any {
	return map[string]any{
		"checkoutId": checkoutID, "shippingMethod": "3002", "deliveryTemp": "3201",
		"receiverZipCode": "100", "recipientName": "測試買家", "recipientPhone": "0912345678",
		"shippingAddress": "台北市中正區測試路1號",
		"items":           []map[string]any{{"skuId": "510", "quantity": "5", "productId": "875563677398107445"}},
	}
}

// ---------------------------------------------------------------------------
// 主測試：legacy vs current 逐位元
// ---------------------------------------------------------------------------

type goldenCase struct {
	name   string
	path   string
	body   map[string]any
	script func() *checkoutScript
}

func checkoutGoldenCases() []goldenCase {
	base := func() *checkoutScript {
		return &checkoutScript{
			preview:     goldenPreviewOK(),
			confirm:     goldenConfirmOK(),
			validate:    &orderpb.CheckoutValidateResponse{Code: 0, Message: "success", Data: &orderpb.CheckoutValidateData{Valid: true}},
			shippingFee: &orderpb.CheckoutShippingFeeResponse{Code: 0, Message: "success", Data: &orderpb.CheckoutShippingFeeData{}},
		}
	}
	return []goldenCase{
		{
			name: "preview/4001_帶unavailableItems（走 response.Biz）", path: "/app/checkout/preview", body: goldenPreviewReq(),
			script: func() *checkoutScript {
				s := base()
				s.preview = &orderpb.CheckoutPreviewResponse{Code: 4001, Message: "商品不可購買",
					Data: &orderpb.CheckoutPreviewData{UnavailableItems: goldenUnavailable()}}
				return s
			},
		},
		{
			name: "preview/4001_Data為nil（落到 GrpcResult）", path: "/app/checkout/preview", body: goldenPreviewReq(),
			script: func() *checkoutScript {
				s := base()
				s.preview = &orderpb.CheckoutPreviewResponse{Code: 4001, Message: "商品不可購買"}
				return s
			},
		},
		{
			name: "preview/4001_Data非nil但清單為空", path: "/app/checkout/preview", body: goldenPreviewReq(),
			script: func() *checkoutScript {
				s := base()
				s.preview = &orderpb.CheckoutPreviewResponse{Code: 4001, Message: "商品不可購買",
					Data: &orderpb.CheckoutPreviewData{CheckoutId: "x"}}
				return s
			},
		},
		{
			name: "preview/成功", path: "/app/checkout/preview", body: goldenPreviewReq(), script: base,
		},
		{
			name: "confirm/4001_帶unavailableItems", path: "/app/checkout/confirm", body: goldenConfirmReq("c1"),
			script: func() *checkoutScript {
				s := base()
				s.confirm = &orderpb.CheckoutConfirmResponse{Code: 4001, Message: "商品不可購買",
					Data: &orderpb.CheckoutConfirmData{UnavailableItems: goldenUnavailable()}}
				return s
			},
		},
		{
			name: "confirm/4091_帶priceChanged", path: "/app/checkout/confirm", body: goldenConfirmReq("c1"),
			script: func() *checkoutScript {
				s := base()
				s.confirm = &orderpb.CheckoutConfirmResponse{Code: 4091, Message: "商品價格已變動，請重新確認訂單",
					PriceChanged: goldenPriceChanged()}
				return s
			},
		},
		{
			name: "confirm/4092_orderNo在message（Data為nil）", path: "/app/checkout/confirm", body: goldenConfirmReq("c1"),
			script: func() *checkoutScript {
				s := base()
				s.confirm = &orderpb.CheckoutConfirmResponse{Code: 4092,
					Message: "您已有未付款訂單：O664700149，請先完成付款或取消後再下單"}
				return s
			},
		},
		{
			// 🔴 checkout.go:641 的判斷式沒有 `resp.Code != 0`，所以這是熱路徑上
			// 唯一真的到得了 `response.Biz(c, 0, ...)` 的地方（第十種繞道的形狀）。
			name: "confirm/code0但PriceChanged已填（Biz(c,0,...)）", path: "/app/checkout/confirm", body: goldenConfirmReq("c1"),
			script: func() *checkoutScript {
				s := base()
				s.confirm = &orderpb.CheckoutConfirmResponse{Code: 0, Message: "success",
					Data:         &orderpb.CheckoutConfirmData{OrderNos: []string{"O664700149"}, TotalPayAmount: "1055"},
					PriceChanged: goldenPriceChanged()}
				return s
			},
		},
		{
			name: "confirm/成功", path: "/app/checkout/confirm", body: goldenConfirmReq("c1"), script: base,
		},
		{
			name: "validate/4001（GrpcResult 會丟 data，legacy 也是）", path: "/app/checkout/validate",
			body: map[string]any{"items": []map[string]any{{"skuId": "510", "quantity": "5", "productId": "8"}}},
			script: func() *checkoutScript {
				s := base()
				s.validate = &orderpb.CheckoutValidateResponse{Code: 4001, Message: "商品不可購買",
					Data: &orderpb.CheckoutValidateData{UnavailableItems: goldenUnavailable()}}
				return s
			},
		},
		{
			name: "validate/成功帶unavailableItems", path: "/app/checkout/validate",
			body: map[string]any{"items": []map[string]any{{"skuId": "510", "quantity": "5", "productId": "8"}}},
			script: func() *checkoutScript {
				s := base()
				s.validate = &orderpb.CheckoutValidateResponse{Code: 0, Message: "success",
					Data: &orderpb.CheckoutValidateData{Valid: false, UnavailableItems: goldenUnavailable()}}
				return s
			},
		},
		{
			name: "shipping_fee/4091", path: "/app/checkout/shipping_fee",
			body: map[string]any{"shippingMethod": "3002", "deliveryTemp": "3201", "receiverZipCode": "100",
				"items": []map[string]any{{"skuId": "510", "quantity": "5", "productId": "8"}}},
			script: func() *checkoutScript {
				s := base()
				s.shippingFee = &orderpb.CheckoutShippingFeeResponse{Code: 4091, Message: "運費已變動"}
				return s
			},
		},
	}
}

// TestCheckout_LegacyVsCurrent_ByteIdentical 是本檔的主張本體：
// 對每一個腳本，改動前後的 HTTP status／Content-Type／body bytes 必須完全相同。
func TestCheckout_LegacyVsCurrent_ByteIdentical(t *testing.T) {
	for _, tc := range checkoutGoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			legacy := goldenPOST(t, newCheckoutGoldenRouter(t, tc.script(), stackLegacy), tc.path, tc.body)
			current := goldenPOST(t, newCheckoutGoldenRouter(t, tc.script(), stackCurrent), tc.path, tc.body)
			assertReachedHandler(t, legacy, "legacy")
			assertReachedHandler(t, current, "current")

			if legacy.Code != current.Code {
				t.Errorf("HTTP status 變了：legacy=%d current=%d\nlegacy body=%s\ncurrent body=%s",
					legacy.Code, current.Code, legacy.Body.String(), current.Body.String())
			}
			if got, want := current.Header().Get("Content-Type"), legacy.Header().Get("Content-Type"); got != want {
				t.Errorf("Content-Type 變了：legacy=%q current=%q", want, got)
			}
			if !bytes.Equal(legacy.Body.Bytes(), current.Body.Bytes()) {
				t.Errorf("body bytes 變了\nlegacy : %s\ncurrent: %s", legacy.Body.String(), current.Body.String())
			}
		})
	}
}

// TestCheckout_PreserveDataCodesMustStayHTTP200 把前端的硬需求寫成機械斷言。
//
// 買家 App 的 Dio 實例（hawkr_buyer/lib/core/networking/dio_provider.dart 的
// `Dio(BaseOptions(...))`）**沒有設 validateStatus** ⇒ 沿用 Dio 預設「只接受 2xx」。
// 結帳的三個業務碼一旦變成 4xx，Dio 會丟 DioException，
// checkout_payment_sheet.dart 的 `catch (e)` 直接顯示「訂單建立失敗，請稍後再試」，
// 而 `_handleConfirmError` 裡 4001／4091／4092 的三個分支永遠到不了。
// ⇒ 「HTTP 200」對這三個碼不是風格偏好，是前端能不能顯示正確對話框的前提。
func TestCheckout_PreserveDataCodesMustStayHTTP200(t *testing.T) {
	cases := []struct {
		name   string
		script *checkoutScript
		path   string
		body   map[string]any
		code   float64
	}{
		{"4001", &checkoutScript{preview: &orderpb.CheckoutPreviewResponse{Code: 4001, Message: "商品不可購買",
			Data: &orderpb.CheckoutPreviewData{UnavailableItems: goldenUnavailable()}}},
			"/app/checkout/preview", goldenPreviewReq(), 4001},
		{"4091", &checkoutScript{confirm: &orderpb.CheckoutConfirmResponse{Code: 4091, Message: "價格變動",
			PriceChanged: goldenPriceChanged()}},
			"/app/checkout/confirm", goldenConfirmReq("c1"), 4091},
		{"4092", &checkoutScript{confirm: &orderpb.CheckoutConfirmResponse{Code: 4092,
			Message: "您已有未付款訂單：O664700149"}},
			"/app/checkout/confirm", goldenConfirmReq("c1"), 4092},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := goldenPOST(t, newCheckoutGoldenRouter(t, tc.script, stackCurrent), tc.path, tc.body)
			assertReachedHandler(t, w, "current")
			if w.Code != http.StatusOK {
				t.Fatalf("HTTP %d ≠ 200：買家 App 的 Dio 只收 2xx，非 2xx 會讓 %v 的分支永遠到不了",
					w.Code, tc.code)
			}
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("body 不是 JSON: %v", err)
			}
			if body["code"] != tc.code {
				t.Fatalf("envelope code = %v，want %v；body=%s", body["code"], tc.code, w.Body.String())
			}
		})
	}
}

// TestCheckout_CarrierAbsent_IsACatastrophicDowngrade 把 Arm 失敗的劇本跑出來。
//
// 這不是「假設有一天」——它是三件事共同的可觀測結果：
// ① r.Use(bizcarrier.Middleware()) 被移到 SetupRoutes 之後（gin 在註冊路由當下才
//
//	複製 handler chain，移到後面等於對已註冊的路由完全不生效）
//
// ② 結帳路由被掛到另一個 engine 上
// ③ 有人把 h.Order.CheckoutX(c.Request.Context(), …) 改成吃 detached context
//
// 本測試只斷言「它與 legacy 不同」＋把實際 body 印出來，不斷言某個特定字串——
// 重點是讓降級的代價有具體數字，而不是形容詞。
func TestCheckout_CarrierAbsent_IsACatastrophicDowngrade(t *testing.T) {
	mk := func() *checkoutScript {
		return &checkoutScript{preview: &orderpb.CheckoutPreviewResponse{Code: 4001, Message: "商品不可購買",
			Data: &orderpb.CheckoutPreviewData{UnavailableItems: goldenUnavailable()}}}
	}
	legacy := goldenPOST(t, newCheckoutGoldenRouter(t, mk(), stackLegacy), "/app/checkout/preview", goldenPreviewReq())
	broken := goldenPOST(t, newCheckoutGoldenRouter(t, mk(), stackNoCarrier), "/app/checkout/preview", goldenPreviewReq())
	assertReachedHandler(t, legacy, "legacy")

	t.Logf("legacy  : HTTP %d %s", legacy.Code, legacy.Body.String())
	t.Logf("noCarrier: HTTP %d %s", broken.Code, broken.Body.String())

	if legacy.Code == broken.Code && bytes.Equal(legacy.Body.Bytes(), broken.Body.Bytes()) {
		t.Fatal("carrier 缺席時輸出竟與 legacy 相同 —— 那表示 Arm 的 fail-closed 分支沒有被走到，" +
			"本測試對它零鑑別力，必須先修測試")
	}
	if broken.Code == http.StatusOK {
		t.Errorf("預期降級成非 200（status.Error 路徑），實際 HTTP 200：%s", broken.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(broken.Body.Bytes(), &body)
	if d, ok := body["data"]; ok && d != nil {
		t.Errorf("預期 data 在 gRPC 邊界蒸發，實際還在：%v", d)
	}
}

// TestCheckout_ConcurrentRequestsDoNotCrossContaminate：carrier 的容器是「單一 HTTP
// 請求」這件事，用併發實測而不是讀 code 推論。
//
// 一條 4001、一條成功，同時打同一個 router。若 holder 不是 per-request（例如哪天被
// 改成 per-connection 或 package 級變數），成功那條會被改寫成 4001 —— 那正是
// 「買家什麼都沒買錯，卻被告知商品不可購買」。
func TestCheckout_ConcurrentRequestsDoNotCrossContaminate(t *testing.T) {
	s := &checkoutScript{
		byCheckoutID: map[string]*orderpb.CheckoutConfirmResponse{
			"bad": {Code: 4001, Message: "商品不可購買",
				Data: &orderpb.CheckoutConfirmData{UnavailableItems: goldenUnavailable()}},
			"good": goldenConfirmOK(),
		},
	}
	r := newCheckoutGoldenRouter(t, s, stackCurrent)

	const n = 40
	var wg sync.WaitGroup
	codes := make([]float64, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "good"
			if i%2 == 0 {
				id = "bad"
			}
			w := goldenPOST(t, r, "/app/checkout/confirm", goldenConfirmReq(id))
			var body map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if v, ok := body["code"].(float64); ok {
				codes[i] = v
			} else {
				codes[i] = -1
			}
		}(i)
	}
	wg.Wait()
	for i, got := range codes {
		want := float64(0)
		if i%2 == 0 {
			want = 4001
		}
		if got != want {
			t.Fatalf("第 %d 條請求 code=%v want=%v —— carrier 跨請求污染", i, got, want)
		}
	}
}

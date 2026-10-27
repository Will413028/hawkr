package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	orderpb "github.com/Will413028/hawkr/go-pkg/proto/order"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// 業務碼統一透傳 E2E（真 EnvelopeGuardInterceptor，非 mock client；issue #1479）。
//
// 背景：EnvelopeGuardInterceptor 會把「未登錄於 envelopeCodeToGRPC 的業務碼」
// （gRPC OK + resp.Code != 0）轉為 FailedPrecondition + "[NNNN] " message 前綴；
// GrpcErr 對 FailedPrecondition 統一將前綴還原為 envelope Code（不再侷限
// 9240/9241 停權家族白名單——issue #1479 前的舊行為）。本檔沿真實鏈路
// （fake order server → bufconn → 真 interceptor → OrderClient → CheckoutValidate
// handler → response.GrpcErr）斷言：任何帶 "[NNNN] " 前綴的業務碼（停權家族
// 9240/9241、一般業務碼 4015 皆同）→ HTTP 409 + envelope code=業務碼 +
// message 乾淨（無前綴）。
type suspensionFakeOrderServer struct {
	orderpb.UnimplementedOrderServiceServer
	code    int32
	message string
}

func (s *suspensionFakeOrderServer) CheckoutValidate(_ context.Context, _ *orderpb.CheckoutValidateRequest) (*orderpb.CheckoutValidateResponse, error) {
	// 鏡射 Java OrderGrpcService.checkoutValidate 的 OrderBusinessException 分支：
	// gRPC OK + envelope Code=業務碼 + message 為 CodeMeta 乾淨中文（無 [NNNN] 前綴）。
	return &orderpb.CheckoutValidateResponse{Code: s.code, Message: s.message}, nil
}

// newCheckoutRouterWithRealInterceptor 起 bufconn fake order server，並以
// 「掛真 EnvelopeGuardInterceptor 的連線」建 OrderClient——這正是 production
// DefaultDialOpts 中 envelope guard 的位置（其餘 auth/breaker interceptor 與本
// 測試主張無關，不掛）。
func newCheckoutRouterWithRealInterceptor(t *testing.T, srv orderpb.OrderServiceServer) http.Handler {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer()
	orderpb.RegisterOrderServiceServer(grpcSrv, srv)
	go grpcSrv.Serve(lis) //nolint:errcheck

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
	t.Cleanup(func() {
		_ = conn.Close()
		grpcSrv.Stop()
	})

	h := &Holder{Order: grpcclient.NewOrderClientForTest(orderpb.NewOrderServiceClient(conn)), BuyerID: buyerGrandfatherResolverForTest()}
	return setupCheckoutRouter(h)
}

func postCheckoutValidate(t *testing.T, router http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]string{
			{"skuId": "SKU001", "quantity": "2", "productId": "PROD001"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/app/checkout/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "100")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestCheckoutValidate_Suspended9241_PassthroughEnvelopeCode(t *testing.T) {
	router := newCheckoutRouterWithRealInterceptor(t, &suspensionFakeOrderServer{
		code: 9241, message: "賣家停權中，商品暫停販售",
	})
	w := postCheckoutValidate(t, router)

	assertHTTPStatus(t, w, http.StatusConflict) // HTTP 仍 409 塌縮
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 9241) // envelope code = 業務碼，前端讀 code 分流
	if resp["message"] != "賣家停權中，商品暫停販售" {
		t.Errorf("message = %q, want clean text without [9241] prefix", resp["message"])
	}
	if resp["messageZh"] != "賣家停權中，商品暫停販售" {
		t.Errorf("messageZh = %q, want clean text", resp["messageZh"])
	}
}

func TestCheckoutValidate_Suspended9240_PassthroughEnvelopeCode(t *testing.T) {
	// 9240 同家族同機制（Java 端賣家面操作被停權擋下時走同一條 interceptor 路徑）。
	router := newCheckoutRouterWithRealInterceptor(t, &suspensionFakeOrderServer{
		code: 9240, message: "賣家帳號停權中，無法執行此操作",
	})
	w := postCheckoutValidate(t, router)

	assertHTTPStatus(t, w, http.StatusConflict)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 9240)
	if resp["message"] != "賣家帳號停權中，無法執行此操作" {
		t.Errorf("message = %q, want clean text without [9240] prefix", resp["message"])
	}
}

func TestCheckoutValidate_UnregisteredBizCode_PassthroughEnvelopeCode(t *testing.T) {
	// issue #1479：未登錄於 envelopeCodeToGRPC 的一般業務碼（4015
	// SELF_PURCHASE_NOT_ALLOWED）不再是例外——與 9240/9241 停權家族同待遇，
	// 一律還原 envelope code、message 去 "[NNNN] " 前綴。HTTP status 仍固定 409。
	router := newCheckoutRouterWithRealInterceptor(t, &suspensionFakeOrderServer{
		code: 4015, message: "不可購買自己上架的商品",
	})
	w := postCheckoutValidate(t, router)

	assertHTTPStatus(t, w, http.StatusConflict)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 4015)
	if resp["message"] != "不可購買自己上架的商品" {
		t.Errorf("message = %q, want clean text without [4015] prefix", resp["message"])
	}
	if resp["messageZh"] != "不可購買自己上架的商品" {
		t.Errorf("messageZh = %q, want clean text", resp["messageZh"])
	}
}

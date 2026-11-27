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

type suspensionFakeOrderServer struct {
	orderpb.UnimplementedOrderServiceServer
	code    int32
	message string
}

func (s *suspensionFakeOrderServer) CheckoutValidate(_ context.Context, _ *orderpb.CheckoutValidateRequest) (*orderpb.CheckoutValidateResponse, error) {
	return &orderpb.CheckoutValidateResponse{Code: s.code, Message: s.message}, nil
}

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

	assertHTTPStatus(t, w, http.StatusConflict)
	resp := parseJSON(t, w.Body)
	assertCode(t, resp, 9241)
	if resp["message"] != "賣家停權中，商品暫停販售" {
		t.Errorf("message = %q, want clean text without [9241] prefix", resp["message"])
	}
	if resp["messageZh"] != "賣家停權中，商品暫停販售" {
		t.Errorf("messageZh = %q, want clean text", resp["messageZh"])
	}
}

func TestCheckoutValidate_Suspended9240_PassthroughEnvelopeCode(t *testing.T) {
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

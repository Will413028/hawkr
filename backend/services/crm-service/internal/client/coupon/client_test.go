package coupon

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// #1366：coupon client 的 x-internal-service 注入從本地 internalServiceInterceptor 遷移到
// 共用 interceptor.InternalAuthClientInterceptor（HMAC 簽章基建）。此測試沿用原意——確認
// 對 coupon-service 的呼叫仍帶 x-internal-service（否則 ListBuyerCouponsForSeller 會被當外部呼叫拒絕，
// GetCustomer 靜默吞錯、CouponCount 歸零；Plan C Phase 1 Task 6 原教訓）。
func TestInternalServiceInterceptor_InjectsMetadata(t *testing.T) {
	var capturedCtx context.Context
	fakeInvoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		capturedCtx = ctx
		return nil
	}

	// 不轉發使用者身分（forwardAccountHeaders=false），保留原 internalServiceInterceptor 語義。
	ci := interceptor.InternalAuthClientInterceptor("crm-service", false)
	err := ci(context.Background(), "/hawkr.coupon.CouponService/ListBuyerCouponsForSeller", nil, nil, nil, fakeInvoker)
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	md, ok := metadata.FromOutgoingContext(capturedCtx)
	if !ok {
		t.Fatal("expected outgoing metadata to be set")
	}
	got := md.Get("x-internal-service")
	if len(got) != 1 || got[0] != "crm-service" {
		t.Errorf("x-internal-service metadata = %v, want [crm-service]", got)
	}
}

// TestNewClient_ConnectsToConfiguredTarget is the behavior baseline for the
// "grpc.NewClient boilerplate → grpcclient.Dial" refactor in this file — see
// services/seller-service/internal/client/media/client_test.go (Task 3) for
// the full rationale.
func TestNewClient_ConnectsToConfiguredTarget(t *testing.T) {
	cfg := &config.Config{
		CouponService: config.ExternalServiceConfig{Addr: "coupon-service.internal:9090"},
	}

	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c == nil {
		t.Fatal("NewClient() returned nil client")
	}
}

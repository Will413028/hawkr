package grpc

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type fakeClientStream struct{ grpc.ClientStream }

func TestAuthMetadataStreamInterceptor_ForwardsContext(t *testing.T) {
	// All seven forwarded fields, so a field silently dropped from the
	// interceptor cannot pass this test (buyer/store id in particular come
	// from a different code path than the auth-middleware ones).
	ctx := WithAuthContext(context.Background(), 12345, "seller")
	ctx = WithTraceContext(ctx, "trace-abc", "req-def")
	ctx = WithSellerSuspension(ctx, "4602")
	ctx = WithBuyerID(ctx, identity.BuyerID(999))
	ctx = WithStoreID(ctx, identity.StoreID(777))

	var gotMD metadata.MD
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		gotMD, _ = metadata.FromOutgoingContext(ctx)
		return &fakeClientStream{}, nil
	}

	intercept := AuthMetadataStreamInterceptor()
	cc := dialTarget(t, "dns:///svc-a:50051")
	if _, err := intercept(ctx, &grpc.StreamDesc{}, cc, "/m", streamer); err != nil {
		t.Fatalf("intercept: %v", err)
	}
	assertMDValue(t, gotMD, "x-account-id", "12345")
	assertMDValue(t, gotMD, "x-account-type", "seller")
	assertMDValue(t, gotMD, "x-trace-id", "trace-abc")
	assertMDValue(t, gotMD, "x-request-id", "req-def")
	assertMDValue(t, gotMD, "x-seller-suspended", "4602")
	assertMDValue(t, gotMD, "x-buyer-id", "999")
	assertMDValue(t, gotMD, "x-store-id", "777")
}

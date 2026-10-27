package grpc

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestAuthMetadataInterceptor_ForwardsBuyerAndStoreID(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		wantBuyer string
		wantStore string
	}{
		{
			name:      "buyer id set",
			ctx:       WithBuyerID(context.Background(), identity.BuyerID(2685914682)),
			wantBuyer: "2685914682",
		},
		{
			name:      "store id set",
			ctx:       WithStoreID(context.Background(), identity.StoreID(2625627849)),
			wantStore: "2625627849",
		},
		{
			name: "neither set",
			ctx:  context.Background(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMD metadata.MD
			invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				gotMD, _ = metadata.FromOutgoingContext(ctx)
				return nil
			}
			intercept := AuthMetadataInterceptor()
			cc := dialTarget(t, "dns:///svc-a:50051")
			if err := intercept(tt.ctx, "/m", nil, nil, cc, invoker); err != nil {
				t.Fatalf("intercept: %v", err)
			}
			assertMDValue(t, gotMD, "x-buyer-id", tt.wantBuyer)
			assertMDValue(t, gotMD, "x-store-id", tt.wantStore)
		})
	}
}

func assertMDValue(t *testing.T, md metadata.MD, key, want string) {
	t.Helper()
	got := md.Get(key)
	if want == "" {
		if len(got) != 0 {
			t.Fatalf("%s: expected absent, got %v", key, got)
		}
		return
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s: want %q got %v", key, want, got)
	}
}

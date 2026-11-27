package grpc

import (
	"context"
	"strconv"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestCartClient_CtxWithAccount_SurvivesAuthMetadataInterceptor(t *testing.T) {
	const rawAccountID = int64(2619321233)
	const resolvedBuyerID = int64(7484139444629864823)

	baseCtx := WithAuthContext(context.Background(), rawAccountID, "3")

	rpcCtx, cancel := ctxWithAccount(baseCtx, time.Second, resolvedBuyerID)
	defer cancel()

	intercept := AuthMetadataInterceptor()
	var captured context.Context
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		captured = ctx
		return nil
	}
	if err := intercept(rpcCtx, "/cart.CartService/AddItem", nil, nil, nil, invoker); err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	md, ok := metadata.FromOutgoingContext(captured)
	if !ok {
		t.Fatal("no outgoing metadata found")
	}
	values := md.Get("x-account-id")
	if len(values) != 1 {
		t.Fatalf("x-account-id should carry exactly one value, got %v (duplicate values let grpc-java's last-value-wins Metadata.get() silently discard the resolved buyer_id)", values)
	}
	if values[0] != strconv.FormatInt(resolvedBuyerID, 10) {
		t.Fatalf("x-account-id = %v, want resolved buyer_id %d", values[0], resolvedBuyerID)
	}
}

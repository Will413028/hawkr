package buyer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
)

type fakeBuyerServiceClient struct {
	pb.BuyerServiceClient
	resp *pb.BuyerEntitlement
	err  error
}

func (f *fakeBuyerServiceClient) GetBuyerEntitlement(_ context.Context, _ *pb.GetBuyerEntitlementRequest, _ ...grpc.CallOption) (*pb.BuyerEntitlement, error) {
	return f.resp, f.err
}

func TestGetBuyerEntitlement_ParsesResponse(t *testing.T) {
	fake := &fakeBuyerServiceClient{resp: &pb.BuyerEntitlement{
		PlanCode: "premium", MonthlyAnswerLimit: "1000",
		PeriodStart: "2026-01-01T00:00:00Z", PeriodEnd: "2026-01-31T00:00:00Z", Source: "admin",
	}}
	c := &client{client: fake}

	got, err := c.GetBuyerEntitlement(context.Background(), identity.BuyerID(42))
	require.NoError(t, err)
	require.Equal(t, "premium", got.PlanCode)
	require.Equal(t, 1000, got.MonthlyAnswerLimit)
	require.Equal(t, "admin", got.Source)
}

func TestGetBuyerEntitlement_InvalidLimit_ReturnsError(t *testing.T) {
	fake := &fakeBuyerServiceClient{resp: &pb.BuyerEntitlement{MonthlyAnswerLimit: "not-a-number", PeriodStart: "2026-01-01T00:00:00Z", PeriodEnd: "2026-01-31T00:00:00Z"}}
	c := &client{client: fake}

	_, err := c.GetBuyerEntitlement(context.Background(), identity.BuyerID(42))
	require.Error(t, err)
}

// TestGetBuyerEntitlement_NonZeroCode_ReturnsError guards against
// buyer-service's "200-shaped business error" contract being silently
// swallowed: GetBuyerEntitlement reports invalid buyer_id / business errors
// via a non-zero Code with nil error (see handler.go), never via the gRPC
// error channel. Mirrors internal/client/order/adapter_test.go's equivalent
// coverage for order-service's identical response contract.
func TestGetBuyerEntitlement_NonZeroCode_ReturnsError(t *testing.T) {
	fake := &fakeBuyerServiceClient{resp: &pb.BuyerEntitlement{Code: 4000, Message: "invalid buyer_id"}}
	c := &client{client: fake}

	got, err := c.GetBuyerEntitlement(context.Background(), identity.BuyerID(42))
	require.Error(t, err)
	require.Equal(t, Entitlement{}, got)
	require.Contains(t, err.Error(), "4000")
	require.Contains(t, err.Error(), "invalid buyer_id")
}

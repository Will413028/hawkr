package buyerid_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/identity"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeBuyerServer struct {
	buyerpb.UnimplementedBuyerServiceServer
	code    int32
	message string
}

func (s *fakeBuyerServer) GetBuyerProfile(_ context.Context, _ *buyerpb.GetBuyerProfileRequest) (*buyerpb.BuyerProfileResponse, error) {
	return &buyerpb.BuyerProfileResponse{Code: s.code, Message: s.message}, nil
}

func newResolverWithRealInterceptor(t *testing.T, srv buyerpb.BuyerServiceServer) *buyerid.Resolver {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer()
	buyerpb.RegisterBuyerServiceServer(grpcSrv, srv)
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

	client := grpcclient.NewBuyerClientForTest(buyerpb.NewBuyerServiceClient(conn))
	return buyerid.New(client)
}

func TestCurrentBuyerID_NoRowThroughRealInterceptor(t *testing.T) {
	r := newResolverWithRealInterceptor(t, &fakeBuyerServer{code: 404, message: "buyer not found"})

	if _, err := r.CurrentBuyerID(context.Background(), identity.AccountID(42)); !errors.Is(err, buyerid.ErrNoCurrentBuyer) {
		t.Fatalf("err = %v, want ErrNoCurrentBuyer", err)
	}
}

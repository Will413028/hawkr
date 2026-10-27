package storeid_test

import (
	"context"
	"errors"
	"net"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"github.com/Will413028/hawkr/go-pkg/identity"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeSellerServer struct {
	sellerpb.UnimplementedSellerServiceServer
}

func (s *fakeSellerServer) GetCurrentStoreID(_ context.Context, _ *sellerpb.GetCurrentStoreIDRequest) (*sellerpb.GetCurrentStoreIDResponse, error) {
	return nil, status.Error(codes.NotFound, "no current store for account")
}

func newResolverWithRealInterceptor(t *testing.T, srv sellerpb.SellerServiceServer) *storeid.Resolver {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer()
	sellerpb.RegisterSellerServiceServer(grpcSrv, srv)
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

	client := grpcclient.NewSellerClientForTest(sellerpb.NewSellerServiceClient(conn))
	return storeid.New(client)
}

func TestCurrentStoreID_NoCurrentStoreThroughRealWire(t *testing.T) {
	r := newResolverWithRealInterceptor(t, &fakeSellerServer{})

	if _, err := r.CurrentStoreID(context.Background(), identity.AccountID(42)); !errors.Is(err, storeid.ErrNoCurrentStore) {
		t.Fatalf("err = %v, want ErrNoCurrentStore", err)
	}
}

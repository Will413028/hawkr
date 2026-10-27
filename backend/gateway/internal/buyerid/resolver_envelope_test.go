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

// 真正過 EnvelopeGuardInterceptor 的 regression（複製 storeid 的
// resolver_envelope_test.go 套路：resolver_test.go 用 GetterFunc 直接 mock
// BuyerGetter 回傳 err=nil+resp=nil，這個組合在正式環境不會發生——真實鏈路
// GetBuyerProfile 對「無現任 buyer stint」一律回 envelope code 404，先被
// EnvelopeGuardInterceptor 轉成 err!=nil + status.Code()==NotFound，繞過
// interceptor 的 mock 給了假信心。本檔起 bufconn fake buyer server，接上
// production 同款的 EnvelopeGuardInterceptor + *grpcclient.BuyerClient，驗證
// resolver 在真實鏈路下仍正確判斷 ErrNoCurrentBuyer）。
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

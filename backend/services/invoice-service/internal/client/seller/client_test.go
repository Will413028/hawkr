package seller

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeSeller 是一個真的 gRPC server（跑在 bufconn 上），不是 stub interface 的假物件：
// 只有真的過一趟 gRPC，「deadline 有沒有被設上」才驗得到——那個值是用 wire 上的
// grpc-timeout header 傳的，換成 interface 假物件就完全測不到。
type fakeSeller struct {
	pb.UnimplementedSellerServiceServer

	accountID int64
	err       error

	gotStoreID  int64
	gotDeadline time.Duration
	hadDeadline bool
}

func (f *fakeSeller) ResolveSellerAccountID(ctx context.Context, req *pb.ResolveSellerAccountIDRequest) (*pb.ResolveSellerAccountIDResponse, error) {
	f.gotStoreID = req.GetStoreId()
	if dl, ok := ctx.Deadline(); ok {
		f.hadDeadline = true
		f.gotDeadline = time.Until(dl)
	}
	if f.err != nil {
		return nil, f.err
	}
	return &pb.ResolveSellerAccountIDResponse{AccountId: f.accountID}, nil
}

func newTestClient(t *testing.T, srv *fakeSeller, deadline time.Duration) *Client {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	pb.RegisterSellerServiceServer(gs, srv)
	go func() { _ = gs.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		gs.Stop()
		_ = lis.Close()
	})
	return NewClientWithStub(pb.NewSellerServiceClient(conn), deadline)
}

// TSID 量級的 account_id：用小數字測不出 int64 有沒有在某一層被截斷。
const tsidAccountID int64 = 2685914682123456
const tsidStoreID int64 = 2685914682

func TestResolveSellerAccountID_Success(t *testing.T) {
	srv := &fakeSeller{accountID: tsidAccountID}
	c := newTestClient(t, srv, 2*time.Second)

	got, err := c.ResolveSellerAccountID(context.Background(), tsidStoreID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tsidAccountID {
		t.Fatalf("account_id = %d, want %d", got, tsidAccountID)
	}
	// 身分不可混用：送出去的必須是 store_id 本身，不是 account_id。
	if srv.gotStoreID != tsidStoreID {
		t.Fatalf("server saw store_id = %d, want %d", srv.gotStoreID, tsidStoreID)
	}
}

// deadline 必須真的到達對端。這支是「拿掉 deadline」變異的偵測點。
func TestResolveSellerAccountID_DeadlineReachesServer(t *testing.T) {
	srv := &fakeSeller{accountID: 7}
	c := newTestClient(t, srv, 2*time.Second)

	if _, err := c.ResolveSellerAccountID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !srv.hadDeadline {
		t.Fatal("server context 沒有 deadline —— per-call deadline 沒有被設上")
	}
	// 允許傳輸損耗，但必須落在「這個 client 設的那個值」附近，
	// 而不是別人（呼叫端 ctx）碰巧帶進來的某個值。
	if srv.gotDeadline <= 1*time.Second || srv.gotDeadline > 2*time.Second {
		t.Fatalf("server 收到的剩餘 deadline = %v，want (1s, 2s]", srv.gotDeadline)
	}
}

// 呼叫端 ctx 已經帶了一個更短的 deadline 時不得被放大。
func TestResolveSellerAccountID_DoesNotExtendCallerDeadline(t *testing.T) {
	srv := &fakeSeller{accountID: 7}
	c := newTestClient(t, srv, 30*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := c.ResolveSellerAccountID(ctx, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.gotDeadline > 1*time.Second {
		t.Fatalf("剩餘 deadline = %v，呼叫端的 500ms 被放大了", srv.gotDeadline)
	}
}

// 🔴 下游回 error 時必須回 error，不得回 (0, nil)。
func TestResolveSellerAccountID_DownstreamErrorIsPropagated(t *testing.T) {
	srv := &fakeSeller{err: status.Error(codes.Internal, "boom")}
	c := newTestClient(t, srv, 2*time.Second)

	got, err := c.ResolveSellerAccountID(context.Background(), 1)
	if err == nil {
		t.Fatal("下游 Internal 錯誤被吞成 nil error —— 呼叫端會把 0 當成解析成功")
	}
	if got != 0 {
		t.Fatalf("錯誤路徑回了 %d，want 0", got)
	}
	if errors.Is(err, ErrSellerNotFound) {
		t.Fatal("Internal 被誤判成 NotFound")
	}
}

func TestResolveSellerAccountID_NotFoundIsTyped(t *testing.T) {
	srv := &fakeSeller{err: status.Error(codes.NotFound, "seller not found for store_id")}
	c := newTestClient(t, srv, 2*time.Second)

	_, err := c.ResolveSellerAccountID(context.Background(), 1)
	if !errors.Is(err, ErrSellerNotFound) {
		t.Fatalf("err = %v，want wraps ErrSellerNotFound", err)
	}
}

// 🔴 下游回 0 是「解析不出來」不是「account_id 就是 0」：必須 error。
func TestResolveSellerAccountID_ZeroAccountIDIsError(t *testing.T) {
	for _, v := range []int64{0, -1} {
		srv := &fakeSeller{accountID: v}
		c := newTestClient(t, srv, 2*time.Second)
		got, err := c.ResolveSellerAccountID(context.Background(), 1)
		if err == nil {
			t.Fatalf("account_id=%d 被當成合法解析結果", v)
		}
		if got != 0 {
			t.Fatalf("錯誤路徑回了 %d，want 0", got)
		}
	}
}

func TestResolveSellerAccountID_RejectsInvalidStoreID(t *testing.T) {
	srv := &fakeSeller{accountID: tsidAccountID}
	c := newTestClient(t, srv, 2*time.Second)

	if _, err := c.ResolveSellerAccountID(context.Background(), 0); err == nil {
		t.Fatal("store_id=0 應該連打都不打就 error")
	}
	if srv.gotStoreID != 0 || srv.hadDeadline {
		t.Fatal("store_id 無效時不應該真的發出 RPC")
	}
}

func TestNewClient_EmptyAddrFailsLoud(t *testing.T) {
	if _, err := NewClient(nil, ""); err == nil {
		t.Fatal("空位址應該 fail-loud，而不是撥一條永遠連不上的連線")
	}
}

func TestNewClientWithStub_ZeroDeadlineFallsBackToDefault(t *testing.T) {
	c := NewClientWithStub(nil, 0)
	if c.deadline != DefaultDeadline {
		t.Fatalf("deadline = %v，want %v（0 不得代表「不設 deadline」）", c.deadline, DefaultDeadline)
	}
}

// sellerAccountResolver 是 internal/worker.SellerAccountResolver 的逐字複本。
//
// 在這裡複製一份而不是 import worker：client 是 Layer 3、worker 是 Layer 4.5，
// 反向 import 是層級倒置。複本的維護成本由這支斷言本身承擔——worker 那邊改了簽章，
// 接線的那一行會編譯錯，這裡則會在下一次有人看時對不上，兩者不會同時無聲。
type sellerAccountResolver interface {
	ResolveSellerAccountID(ctx context.Context, storeID int64) (int64, error)
}

// 編譯期斷言：*Client 必須能直接當成 worker 的 SellerAccountResolver 接上去。
var _ sellerAccountResolver = (*Client)(nil)

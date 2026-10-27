package shipping

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/shipping"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeShipping 是一個真的 gRPC server（跑在 bufconn 上），不是 stub interface 的假物件：
// 只有真的過一趟 gRPC，「deadline 有沒有被設上」才驗得到——那個值是用 wire 上的
// grpc-timeout header 傳的，換成 interface 假物件就完全測不到。
type fakeShipping struct {
	pb.UnimplementedShippingServiceServer

	resp *pb.GetShipmentsByOrderNosResponse
	err  error

	gotOrderNos []string
	hadDeadline bool
	gotDeadline time.Duration
}

func (f *fakeShipping) GetShipmentsByOrderNos(
	ctx context.Context, req *pb.GetShipmentsByOrderNosRequest,
) (*pb.GetShipmentsByOrderNosResponse, error) {
	f.gotOrderNos = req.GetOrderNos()
	if dl, ok := ctx.Deadline(); ok {
		f.hadDeadline = true
		f.gotDeadline = time.Until(dl)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newTestClient(t *testing.T, srv *fakeShipping, deadline time.Duration) *Client {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	pb.RegisterShippingServiceServer(gs, srv)
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
	t.Cleanup(func() { _ = conn.Close(); gs.Stop() })
	return NewClientWithStub(pb.NewShippingServiceClient(conn), deadline)
}

// 欄位名與型別抄自 api/proto/shipping/shipping.proto 的 ShipmentDetailView
// （§0.3：fixture 必須來自契約定義，不得手寫「我覺得對方會發成這樣」）。
func TestBatchGetShipmentStatus_MapsOrderNoToStatus(t *testing.T) {
	srv := &fakeShipping{resp: &pb.GetShipmentsByOrderNosResponse{
		Code: 0,
		Data: []*pb.ShipmentDetailView{
			{OrderNo: "O1", Status: "210"}, // TCAT_CALLED
			{OrderNo: "O2", Status: "11"},  // PICKED_UP
		},
	}}
	c := newTestClient(t, srv, 500*time.Millisecond)

	// O3 刻意沒有物流單：它必須**不在** map 內，呼叫端才回得出誠實的空字串。
	got, err := c.BatchGetShipmentStatus(context.Background(), []string{"O1", "O2", "O3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["O1"] != "210" || got["O2"] != "11" {
		t.Fatalf("status map wrong: %#v", got)
	}
	if _, ok := got["O3"]; ok {
		t.Fatalf("O3 有物流單？map=%#v —— 查不到的訂單不得進 map", got)
	}
	if !srv.hadDeadline {
		t.Fatal("per-call deadline 沒被設上：shipping-service 卡住時整個 S06 會跟著掛住")
	}
	if srv.gotDeadline > 500*time.Millisecond {
		t.Fatalf("deadline 比指定值長：%v", srv.gotDeadline)
	}
}

// 🔴 這一格是本 client 存在的理由：錯誤**不得**在這一層被吃成空 map。
// 吃掉了，上層就再也分不出「整批查不到」與「這幾筆沒出貨」——那正是 #3019 的原始症狀。
func TestBatchGetShipmentStatus_FailsLoud(t *testing.T) {
	cases := []struct {
		name string
		srv  *fakeShipping
	}{
		{"gRPC error", &fakeShipping{err: status.Error(codes.Unavailable, "down")}},
		{"envelope code != 0", &fakeShipping{resp: &pb.GetShipmentsByOrderNosResponse{Code: 5001, Message: "boom"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.srv, 500*time.Millisecond)
			got, err := c.BatchGetShipmentStatus(context.Background(), []string{"O1"})
			if err == nil {
				t.Fatalf("錯誤被吃掉了，回了 %#v —— 降級是呼叫端的決定，不是這一層的", got)
			}
		})
	}
}

func TestBatchGetShipmentStatus_EmptyInputSkipsCall(t *testing.T) {
	srv := &fakeShipping{err: errors.New("不該被呼叫")}
	c := newTestClient(t, srv, 500*time.Millisecond)
	got, err := c.BatchGetShipmentStatus(context.Background(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("空輸入應直接回空 map：got=%#v err=%v", got, err)
	}
	if srv.gotOrderNos != nil {
		t.Fatal("空輸入不該發出 RPC")
	}
}

// 超過上限一律報錯而不是靜默截斷：截斷會讓被丟掉的那些訂單回空字串，
// 而空字串在這一欄會被讀成「還沒出貨」。
func TestBatchGetShipmentStatus_RejectsOversizedBatch(t *testing.T) {
	srv := &fakeShipping{resp: &pb.GetShipmentsByOrderNosResponse{Code: 0}}
	c := newTestClient(t, srv, 500*time.Millisecond)

	tooMany := make([]string, MaxBatch+1)
	for i := range tooMany {
		tooMany[i] = "O"
	}
	if _, err := c.BatchGetShipmentStatus(context.Background(), tooMany); err == nil {
		t.Fatal("超過 MaxBatch 應報錯，不得靜默截斷")
	}
	if srv.gotOrderNos != nil {
		t.Fatal("超量時不該發出 RPC")
	}
}

// 一張訂單多筆物流單（退貨便會再開一張）時取第一筆，與 GetShipmentByOrderNo 的單筆語意一致。
func TestBatchGetShipmentStatus_DuplicateOrderNoKeepsFirst(t *testing.T) {
	srv := &fakeShipping{resp: &pb.GetShipmentsByOrderNosResponse{
		Code: 0,
		Data: []*pb.ShipmentDetailView{
			{OrderNo: "O1", Status: "31"},
			{OrderNo: "O1", Status: "51"},
			{OrderNo: "", Status: "99"}, // 沒有 order_no 的列必須被濾掉，不得建出 "" 這個 key
		},
	}}
	c := newTestClient(t, srv, 500*time.Millisecond)

	got, err := c.BatchGetShipmentStatus(context.Background(), []string{"O1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["O1"] != "31" {
		t.Fatalf("重複 order_no 應取第一筆，got=%q", got["O1"])
	}
	if _, ok := got[""]; ok {
		t.Fatal(`空 order_no 不得成為 map key`)
	}
}

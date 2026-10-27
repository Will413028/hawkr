package buyer

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// 真的 gRPC server（bufconn）而不是 interface 假物件：只有過一趟 gRPC，
// 「deadline 有沒有被設上」才驗得到（那個值走 wire 上的 grpc-timeout header）。
type fakeBuyer struct {
	pb.UnimplementedBuyerServiceServer

	resp        *pb.BatchGetBuyerDisplayByBuyerIDsResponse
	err         error
	gotIDs      []int64
	hadDeadline bool
}

func (f *fakeBuyer) BatchGetBuyerDisplayByBuyerIDs(
	ctx context.Context, req *pb.BatchGetBuyerDisplayByBuyerIDsRequest,
) (*pb.BatchGetBuyerDisplayByBuyerIDsResponse, error) {
	f.gotIDs = req.GetBuyerIds()
	_, f.hadDeadline = ctx.Deadline()
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newTestClient(t *testing.T, srv *fakeBuyer) *Client {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	pb.RegisterBuyerServiceServer(gs, srv)
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
	return NewClientWithStub(pb.NewBuyerServiceClient(conn), 500*time.Millisecond)
}

// 欄位名與型別抄自 api/proto/buyer/buyer.proto 的 BuyerDisplay（§0.3）。
func TestBatchGetBuyerDisplay(t *testing.T) {
	srv := &fakeBuyer{resp: &pb.BatchGetBuyerDisplayByBuyerIDsResponse{
		Code: 0,
		Buyers: map[int64]*pb.BuyerDisplay{
			11: {DisplayName: "陳小明", Deleted: false},
			12: {DisplayName: "已註銷的買家", Deleted: true},
			// 🔴 送 nil，但**收到的是空 message 不是 nil**（protobuf map 的 wire 行為）。
			// 這一格釘住那件事：判準必須是「有沒有名字」而不是「指標是不是 nil」，
			// 後者在 in-process 假物件上看起來有效、一上 wire 就永遠不成立。
			13: nil,
		},
	}}
	c := newTestClient(t, srv)

	got, err := c.BatchGetBuyerDisplay(context.Background(), []int64{11, 12, 13, 14})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[11].DisplayName != "陳小明" || got[11].Deleted {
		t.Fatalf("11 錯：%#v", got[11])
	}
	// 🔴 Deleted 必須帶出來：呼叫端靠它決定要不要遮罩（墓碑不是個資，遮了讀不懂）。
	// 少了這一欄，唯一的替代做法是拿 DisplayName 去跟墓碑字串比對，
	// 而 proto 註解明令禁止那件事。
	if !got[12].Deleted {
		t.Fatalf("12 的 Deleted 沒帶出來：%#v", got[12])
	}
	if _, ok := got[13]; ok {
		t.Fatalf("沒有名字的項目不得進 map（wire 上它是空 message，不是 nil）：%#v", got[13])
	}
	if _, ok := got[14]; ok {
		t.Fatal("查無的 id 不得進 map")
	}
	if !srv.hadDeadline {
		t.Fatal("per-call deadline 沒被設上")
	}
}

func TestBatchGetBuyerDisplay_FailsLoud(t *testing.T) {
	for name, srv := range map[string]*fakeBuyer{
		"gRPC error":       {err: status.Error(codes.Unavailable, "down")},
		"envelope code!=0": {resp: &pb.BatchGetBuyerDisplayByBuyerIDsResponse{Code: 5001, Message: "boom"}},
	} {
		t.Run(name, func(t *testing.T) {
			c := newTestClient(t, srv)
			if got, err := c.BatchGetBuyerDisplay(context.Background(), []int64{11}); err == nil {
				t.Fatalf("錯誤被吃掉了，回了 %#v —— 降級是呼叫端的決定", got)
			}
		})
	}
}

func TestBatchGetBuyerDisplay_GuardsInput(t *testing.T) {
	srv := &fakeBuyer{resp: &pb.BatchGetBuyerDisplayByBuyerIDsResponse{Code: 0}}
	c := newTestClient(t, srv)

	if got, err := c.BatchGetBuyerDisplay(context.Background(), nil); err != nil || len(got) != 0 {
		t.Fatalf("空輸入應直接回空 map：%#v %v", got, err)
	}
	if srv.gotIDs != nil {
		t.Fatal("空輸入不該發出 RPC")
	}
	tooMany := make([]int64, MaxBatch+1)
	if _, err := c.BatchGetBuyerDisplay(context.Background(), tooMany); err == nil {
		t.Fatal("超過 MaxBatch 應報錯，不得靜默截斷")
	}
}

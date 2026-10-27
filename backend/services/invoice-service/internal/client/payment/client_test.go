package payment

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakePayment 是一個真的 gRPC server（跑在 bufconn 上），不是 stub interface 的假物件：
// 只有真的過一趟 gRPC，「deadline 有沒有被設上」才驗得到——那個值是用 wire 上的
// grpc-timeout header 傳的，換成 interface 假物件就完全測不到。
type fakePayment struct {
	pb.UnimplementedPaymentServiceServer

	resp *pb.GetPaymentFactsInternalResponse
	err  error

	gotOrderNo  string
	gotDeadline time.Duration
	hadDeadline bool
}

func (f *fakePayment) GetPaymentFactsInternal(ctx context.Context, req *pb.GetPaymentFactsInternalRequest) (*pb.GetPaymentFactsInternalResponse, error) {
	f.gotOrderNo = req.GetOrderNo()
	if dl, ok := ctx.Deadline(); ok {
		f.hadDeadline = true
		f.gotDeadline = time.Until(dl)
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func newTestClient(t *testing.T, srv *fakePayment, deadline time.Duration) *Client {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	pb.RegisterPaymentServiceServer(gs, srv)
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
	return NewClientWithStub(pb.NewPaymentServiceClient(conn), deadline)
}

func okResp(data *pb.PaymentFactsData) *pb.GetPaymentFactsInternalResponse {
	return &pb.GetPaymentFactsInternalResponse{Code: 0, Message: "success", Data: data}
}

const testOrderNo = "O202608300001"

func TestGetPaymentFacts_FieldMapping(t *testing.T) {
	providerPaidAt := "2026-08-30T01:02:03Z"
	srv := &fakePayment{resp: okResp(&pb.PaymentFactsData{
		OrderNo:        testOrderNo,
		TradeNo:        testOrderNo + "001",
		ProviderPaidAt: &providerPaidAt,
		PayTime:        "2026-08-30T01:05:00Z",
		TradeAmt:       "980",
		PaymentType:    "2201",
		IsDuplicate:    false,
	})}
	c := newTestClient(t, srv, 2*time.Second)

	got, err := c.GetPaymentFacts(context.Background(), testOrderNo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.gotOrderNo != testOrderNo {
		t.Fatalf("order_no 沒有送到對端：%q", srv.gotOrderNo)
	}
	if got.TradeNo != testOrderNo+"001" {
		t.Fatalf("trade_no = %q", got.TradeNo)
	}
	if got.ProviderPaidAt == nil || !got.ProviderPaidAt.Equal(time.Date(2026, 8, 30, 1, 2, 3, 0, time.UTC)) {
		t.Fatalf("provider_paid_at = %v", got.ProviderPaidAt)
	}
	if !got.PayTime.Equal(time.Date(2026, 8, 30, 1, 5, 0, 0, time.UTC)) {
		t.Fatalf("pay_time = %v", got.PayTime)
	}
	if got.IsDuplicate {
		t.Fatalf("is_duplicate 應為 false")
	}
	// PaidAt() 的 COALESCE：provider_paid_at 非 nil 時取它，不是 pay_time。
	if !got.PaidAt().Equal(time.Date(2026, 8, 30, 1, 2, 3, 0, time.UTC)) {
		t.Fatalf("PaidAt() = %v，want provider_paid_at", got.PaidAt())
	}
}

// TestGetPaymentFacts_DeadlineOnWire 驗 deadline 真的走了 grpc-timeout header。
// 用對端讀回來的剩餘時間，而不是看 client 有沒有呼叫 WithTimeout——後者是白盒推理。
func TestGetPaymentFacts_DeadlineOnWire(t *testing.T) {
	srv := &fakePayment{resp: okResp(&pb.PaymentFactsData{
		TradeNo: "T1", PayTime: "2026-08-30T01:05:00Z",
	})}
	c := newTestClient(t, srv, 1500*time.Millisecond)

	if _, err := c.GetPaymentFacts(context.Background(), testOrderNo); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !srv.hadDeadline {
		t.Fatalf("對端沒有收到 deadline：grpc-timeout header 沒有被設上")
	}
	if srv.gotDeadline <= 0 || srv.gotDeadline > 1500*time.Millisecond {
		t.Fatalf("對端看到的剩餘時間 = %v，want (0, 1.5s]", srv.gotDeadline)
	}
}

// TestGetPaymentFacts_ProviderPaidAtTriState 是本檔最重要的一支：
// 「欄位未設」與「欄位設成空字串」必須落到不同的行為。
// 用 GetProviderPaidAt() 直接取會把兩者塌成 ""，這支就是那個變異的偵測器。
func TestGetPaymentFacts_ProviderPaidAtTriState(t *testing.T) {
	payTime := time.Date(2026, 8, 30, 1, 5, 0, 0, time.UTC)

	t.Run("欄位未設＝ATM 常態，PaidAt 退回 pay_time", func(t *testing.T) {
		srv := &fakePayment{resp: okResp(&pb.PaymentFactsData{
			TradeNo: "T1", PayTime: "2026-08-30T01:05:00Z", ProviderPaidAt: nil,
		})}
		c := newTestClient(t, srv, 2*time.Second)

		got, err := c.GetPaymentFacts(context.Background(), testOrderNo)
		if err != nil {
			t.Fatalf("欄位未設是合法值，不該是錯誤：%v", err)
		}
		if got.ProviderPaidAt != nil {
			t.Fatalf("ProviderPaidAt = %v，want nil", got.ProviderPaidAt)
		}
		if !got.PaidAt().Equal(payTime) {
			t.Fatalf("PaidAt() = %v，want %v", got.PaidAt(), payTime)
		}
	})

	t.Run("欄位設成空字串＝上游壞了，必須 fail-loud", func(t *testing.T) {
		empty := ""
		srv := &fakePayment{resp: okResp(&pb.PaymentFactsData{
			TradeNo: "T1", PayTime: "2026-08-30T01:05:00Z", ProviderPaidAt: &empty,
		})}
		c := newTestClient(t, srv, 2*time.Second)

		if _, err := c.GetPaymentFacts(context.Background(), testOrderNo); err == nil {
			t.Fatalf("欄位已設但內容為空必須 fail-loud，實際回 nil error" +
				"（若改用 GetProviderPaidAt() 直接取，這一格會與『欄位未設』塌成同一件事）")
		}
	})
}

// TestGetPaymentFacts_DownstreamErrorsNeverReturnZeroValue 逐條驗 fail-loud。
// 任何一條退化成「零值 ＋ nil error」都會開出一張日期或金額基準錯誤的發票。
func TestGetPaymentFacts_DownstreamErrorsNeverReturnZeroValue(t *testing.T) {
	cases := []struct {
		name    string
		srv     *fakePayment
		wantIs  error
		wantErr bool
	}{
		{
			name:    "gRPC NotFound",
			srv:     &fakePayment{err: status.Error(codes.NotFound, "no such order")},
			wantIs:  ErrPaymentFactsNotFound,
			wantErr: true,
		},
		{
			name:    "gRPC Unavailable（下游掛了）",
			srv:     &fakePayment{err: status.Error(codes.Unavailable, "connection refused")},
			wantErr: true,
		},
		{
			// 🔴 payment 端**實際**回的形狀：gRPC status = OK，錯誤在 envelope 的 code。
			// 8015 = TRANSACTION_NOT_FOUND_FOR_ORDER（GrpcResponseCode.java）。
			// 只看 gRPC error 的 client 會拿到一個全零的 PaymentFactsData，
			// PaidAt() 靜默退回本地時鐘——那正是這支 RPC 存在的理由要防的事。
			name:    "envelope code=8015 ＋ 無 data（gRPC status 是 OK）",
			srv:     &fakePayment{resp: &pb.GetPaymentFactsInternalResponse{Code: 8015, Message: "查無該訂單的交易"}},
			wantIs:  ErrPaymentFactsNotFound,
			wantErr: true,
		},
		{
			// 🔴 code != 0 **且** data 非空：這一格才驗得到「client 真的讀了 code」。
			// 只給「code != 0 ＋ 無 data」的話，拿掉 code 檢查之後 data==nil 那一條
			// 仍會回同一個 ErrPaymentFactsNotFound ⇒ 測試照樣綠，鑑別力是假的
			// （2026-08-30 實測過這個假綠）。
			name: "envelope code=8015 ＋ data 非空（唯一驗得到 code 檢查的一格）",
			srv: &fakePayment{resp: &pb.GetPaymentFactsInternalResponse{
				Code: 8015, Message: "查無該訂單的交易",
				Data: &pb.PaymentFactsData{TradeNo: "T9", PayTime: "2026-08-30T01:05:00Z"},
			}},
			wantIs:  ErrPaymentFactsNotFound,
			wantErr: true,
		},
		{
			name:    "envelope code != 0（其他業務碼）",
			srv:     &fakePayment{resp: &pb.GetPaymentFactsInternalResponse{Code: 4004, Message: "查無交易"}},
			wantIs:  ErrPaymentFactsNotFound,
			wantErr: true,
		},
		{
			name:    "code=0 但 data 為 nil",
			srv:     &fakePayment{resp: okResp(nil)},
			wantIs:  ErrPaymentFactsNotFound,
			wantErr: true,
		},
		{
			name: "is_duplicate=true（payment 端選錯 canonical 交易）",
			srv: &fakePayment{resp: okResp(&pb.PaymentFactsData{
				TradeNo: "T2", PayTime: "2026-08-30T01:05:00Z", IsDuplicate: true,
			})},
			wantErr: true,
		},
		{
			name:    "pay_time 解析不了",
			srv:     &fakePayment{resp: okResp(&pb.PaymentFactsData{TradeNo: "T1", PayTime: "not-a-time"})},
			wantErr: true,
		},
		{
			name:    "pay_time 為空",
			srv:     &fakePayment{resp: okResp(&pb.PaymentFactsData{TradeNo: "T1", PayTime: ""})},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.srv, 2*time.Second)
			got, err := c.GetPaymentFacts(context.Background(), testOrderNo)
			if tc.wantErr && err == nil {
				t.Fatalf("必須回 error，實際回零值 ＋ nil：%+v", got)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("errors.Is(err, %v) = false；err = %v", tc.wantIs, err)
			}
			// 陰性斷言：回 error 時不得同時交出一個看起來可用的值。
			if err != nil && (got.TradeNo != "" || !got.PayTime.IsZero()) {
				t.Fatalf("回 error 時仍交出了部分值：%+v", got)
			}
		})
	}
}

func TestNewClient_RejectsEmptyAddr(t *testing.T) {
	if _, err := NewClient(nil, ""); err == nil {
		t.Fatalf("空位址必須 fail-loud（否則 grpc.NewClient 會撥一個永遠連不上的目標）")
	}
}

func TestNewClientWithStub_DeadlineFallback(t *testing.T) {
	c := NewClientWithStub(nil, 0)
	if c.deadline != DefaultDeadline {
		t.Fatalf("deadline <= 0 必須退回 DefaultDeadline，實際 %v", c.deadline)
	}
}

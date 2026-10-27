package order

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pb "github.com/Will413028/hawkr/go-pkg/proto/order"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeOrder 是一個真的 gRPC server（bufconn），不是 stub interface 的假物件：
// deadline 是用 wire 上的 grpc-timeout header 傳的，換成假物件就完全測不到。
type fakeOrder struct {
	pb.UnimplementedOrderServiceServer

	resp *pb.GetOrderInvoiceSnapshotInternalResponse
	err  error

	gotOrderNo  string
	gotDeadline time.Duration
	hadDeadline bool

	// Revise 那一支的錄音與腳本
	reviseResp  *pb.ReviseOrderInvoiceSnapshotInternalResponse
	reviseErr   error
	gotRevise   *pb.ReviseOrderInvoiceSnapshotInternalRequest
	reviseCalls int
}

func (f *fakeOrder) ReviseOrderInvoiceSnapshotInternal(
	ctx context.Context, req *pb.ReviseOrderInvoiceSnapshotInternalRequest,
) (*pb.ReviseOrderInvoiceSnapshotInternalResponse, error) {
	f.reviseCalls++
	f.gotRevise = req
	if f.reviseErr != nil {
		return nil, f.reviseErr
	}
	return f.reviseResp, nil
}

func (f *fakeOrder) GetOrderInvoiceSnapshotInternal(ctx context.Context, req *pb.GetOrderInvoiceSnapshotInternalRequest) (*pb.GetOrderInvoiceSnapshotInternalResponse, error) {
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

func newTestClient(t *testing.T, srv *fakeOrder, prefs BuyerPreferenceReader, deadline time.Duration) *Client {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	pb.RegisterOrderServiceServer(gs, srv)
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
	return NewClientWithStub(pb.NewOrderServiceClient(conn), prefs, deadline)
}

const testOrderNo = "O202608300001"

func fullSnapshot() *pb.OrderInvoiceSnapshotData {
	return &pb.OrderInvoiceSnapshotData{
		OrderNo:              testOrderNo,
		SnapshotVersion:      "1",
		InvoiceKind:          "B2C",
		CarrierSelectionMode: codemeta.InvoiceCarrierSelectionModeNone.Value,
		NotifyEmail:          "buyer@example.com",
		TotalOriginalAmount:  "1000",
		ShippingFee:          "80",
		DiscountAmount:       "100",
		TotalActualAmount:    "980",
		DiscountFunding:      "SELLER",
		IsConsolidatedOrder:  false,
		Items: []*pb.OrderInvoiceSnapshotItem{
			{OrderItemId: "2685914682000001", SkuId: "2685914682000011", ProductName: "有機高麗菜",
				SkuName: "大顆", Quantity: "2", UnitPrice: "300"},
			{OrderItemId: "2685914682000002", SkuId: "2685914682000012", ProductName: "有機番茄",
				SkuName: "", Quantity: "1", UnitPrice: "400"},
		},
	}
}

func okResp(d *pb.OrderInvoiceSnapshotData) *pb.GetOrderInvoiceSnapshotInternalResponse {
	return &pb.GetOrderInvoiceSnapshotInternalResponse{Code: 0, Message: "success", Data: d}
}

func TestGetOrderInvoiceSnapshot_FieldMapping(t *testing.T) {
	srv := &fakeOrder{resp: okResp(fullSnapshot())}
	c := newTestClient(t, srv, newPrefs(), 2*time.Second)

	got, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.gotOrderNo != testOrderNo {
		t.Fatalf("order_no 沒有送到對端：%q", srv.gotOrderNo)
	}
	if got.SnapshotVersion != 1 {
		t.Fatalf("snapshot_version = %d", got.SnapshotVersion)
	}
	s := got.Snapshot
	if s.TotalOriginalAmount != 1000 || s.ShippingFee != 80 || s.DiscountAmount != 100 || s.TotalActualAmount != 980 {
		t.Fatalf("金額對映錯：%+v", s)
	}
	if s.DiscountFunding != domain.DiscountFundingSeller {
		t.Fatalf("discount_funding = %v", s.DiscountFunding)
	}
	if got.HasPlatformFundedDiscount {
		t.Fatalf("SELLER 出資不該觸發 fail-closed")
	}
	if got.NotifyEmail != "buyer@example.com" {
		t.Fatalf("notify_email = %q", got.NotifyEmail)
	}
	if len(s.Items) != 2 {
		t.Fatalf("items 數 = %d", len(s.Items))
	}
	// 順序即 line_seq（§5A.3）：本層原樣保留 proto 的順序，不重排。
	if s.Items[0].OrderItemID != 2685914682000001 || s.Items[0].SkuID != 2685914682000011 ||
		s.Items[0].ProductName != "有機高麗菜" || s.Items[0].SkuName != "大顆" ||
		s.Items[0].Quantity != 2 || s.Items[0].UnitPrice != 300 {
		t.Fatalf("items[0] 對映錯：%+v", s.Items[0])
	}
	if s.Items[1].OrderItemID != 2685914682000002 || s.Items[1].SkuName != "" || s.Items[1].UnitPrice != 400 {
		t.Fatalf("items[1] 對映錯：%+v", s.Items[1])
	}
}

// TestGetOrderInvoiceSnapshot_TSIDPrecision：TSID 量級的 id 過一趟 string↔int64
// 不能掉精度。用小數字測不出來。
func TestGetOrderInvoiceSnapshot_TSIDPrecision(t *testing.T) {
	d := fullSnapshot()
	d.Items[0].OrderItemId = "9007199254740993" // 2^53 + 1：float64 表達不了
	srv := &fakeOrder{resp: okResp(d)}
	c := newTestClient(t, srv, newPrefs(), 2*time.Second)

	got, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Snapshot.Items[0].OrderItemID != 9007199254740993 {
		t.Fatalf("TSID 掉精度：%d", got.Snapshot.Items[0].OrderItemID)
	}
}

func TestGetOrderInvoiceSnapshot_DeadlineOnWire(t *testing.T) {
	srv := &fakeOrder{resp: okResp(fullSnapshot())}
	c := newTestClient(t, srv, newPrefs(), 1500*time.Millisecond)

	if _, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !srv.hadDeadline {
		t.Fatalf("對端沒有收到 deadline：grpc-timeout header 沒有被設上")
	}
	if srv.gotDeadline <= 0 || srv.gotDeadline > 1500*time.Millisecond {
		t.Fatalf("對端看到的剩餘時間 = %v，want (0, 1.5s]", srv.gotDeadline)
	}
}

// TestGetOrderInvoiceSnapshot_FailClosedDiscountFunding：`discount_funding` 為 ""
// 是 order-service 判不出來（U-7），**必須** fail-closed，不得當成 SELLER。
func TestGetOrderInvoiceSnapshot_FailClosedDiscountFunding(t *testing.T) {
	cases := []struct {
		funding  string
		discount string
		want     bool
	}{
		{"SELLER", "100", false},
		{"PLATFORM", "100", true},
		{"", "100", true},          // 判不出來 ⇒ fail-closed
		{"", "0", false},           // 沒有折扣 ⇒ 資金來源無意義
		{"WHO_KNOWS", "100", true}, // 值域外 ⇒ 同樣 fail-closed
	}
	for _, tc := range cases {
		d := fullSnapshot()
		d.DiscountFunding = tc.funding
		d.DiscountAmount = tc.discount
		srv := &fakeOrder{resp: okResp(d)}
		c := newTestClient(t, srv, newPrefs(), 2*time.Second)

		got, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID)
		if err != nil {
			t.Fatalf("funding=%q discount=%q: %v", tc.funding, tc.discount, err)
		}
		if got.HasPlatformFundedDiscount != tc.want {
			t.Fatalf("funding=%q discount=%q: HasPlatformFundedDiscount = %v，want %v",
				tc.funding, tc.discount, got.HasPlatformFundedDiscount, tc.want)
		}
	}
}

func TestGetOrderInvoiceSnapshot_ConsolidatedOrderPassthrough(t *testing.T) {
	d := fullSnapshot()
	d.IsConsolidatedOrder = true
	srv := &fakeOrder{resp: okResp(d)}
	c := newTestClient(t, srv, newPrefs(), 2*time.Second)

	got, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsConsolidatedOrder {
		t.Fatalf("is_consolidated_order 沒有透傳")
	}
}

// TestGetOrderInvoiceSnapshot_CarrierResolutionIsWired 驗「解析步驟真的被插進來了」。
//
// 這一支與 carrier_test.go 的差別：那邊驗解析函式本身，這邊驗 client 有沒有呼叫它、
// 有沒有把訂單的 buyer_id 傳下去、有沒有把結果覆蓋回 OrderFacts。
func TestGetOrderInvoiceSnapshot_CarrierResolutionIsWired(t *testing.T) {
	d := fullSnapshot()
	d.CarrierSelectionMode = codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value
	d.BuyerInvoicePreferenceId = "2685914682123456"

	prefs := newPrefs(mobilePref(testPrefID, testBuyerID))
	prefs.plaintext[testPrefID] = "/T7R3U+P"

	srv := &fakeOrder{resp: okResp(d)}
	c := newTestClient(t, srv, prefs, 2*time.Second)

	got, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prefs.getByIDCalls != 1 {
		t.Fatalf("載具解析沒有被呼叫（getByID calls = %d）", prefs.getByIDCalls)
	}
	if prefs.gotBuyerID != testBuyerID {
		t.Fatalf("歸屬述詞收到的 buyer_id = %d，want %d", prefs.gotBuyerID, testBuyerID)
	}
	if got.CarrierType.Code != codemeta.InvoiceCarrierTypeMobileBarcode.Code || got.CarrierID1 != "/T7R3U+P" {
		t.Fatalf("解析結果沒有回填 OrderFacts：%+v", got)
	}
	if got.PreferenceOwnershipMismatch {
		t.Fatalf("歸屬正確時不該標 mismatch")
	}
}

func TestGetOrderInvoiceSnapshot_OwnershipMismatchSurfaces(t *testing.T) {
	d := fullSnapshot()
	d.CarrierSelectionMode = codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value
	d.BuyerInvoicePreferenceId = "2685914682123456"

	// 這一列屬於別的買家。
	prefs := newPrefs(mobilePref(testPrefID, otherBuyerID))
	prefs.plaintext[testPrefID] = "/T7R3U+P"

	srv := &fakeOrder{resp: okResp(d)}
	c := newTestClient(t, srv, prefs, 2*time.Second)

	got, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID)
	if err != nil {
		t.Fatalf("歸屬錯誤不是傳輸錯誤：%v", err)
	}
	if !got.PreferenceOwnershipMismatch {
		t.Fatalf("OrderFacts.PreferenceOwnershipMismatch 必須為 true（否則 usecase 開不出 AWAITING_MANUAL）")
	}
	if got.CarrierType.Code != "" || got.CarrierID1 != "" {
		t.Fatalf("歸屬錯誤時不得交出載具：%+v", got)
	}
	// 陰性：品項與金額照樣要在（AWAITING_MANUAL 的文件仍要落明細供 ADM 判定）。
	if len(got.Snapshot.Items) != 2 || got.Snapshot.TotalActualAmount != 980 {
		t.Fatalf("歸屬錯誤不該連品項一起吞掉：%+v", got.Snapshot)
	}
}

func TestGetOrderInvoiceSnapshot_DownstreamErrorsNeverReturnZeroValue(t *testing.T) {
	badVersion := fullSnapshot()
	badVersion.SnapshotVersion = "0"
	emptyAmount := fullSnapshot()
	emptyAmount.TotalActualAmount = ""
	badItemQty := fullSnapshot()
	badItemQty.Items[0].Quantity = "0"
	badPrefID := fullSnapshot()
	badPrefID.CarrierSelectionMode = codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value
	badPrefID.BuyerInvoicePreferenceId = "abc"

	cases := []struct {
		name   string
		srv    *fakeOrder
		wantIs error
	}{
		{"gRPC NotFound", &fakeOrder{err: status.Error(codes.NotFound, "no such order")}, ErrOrderSnapshotNotFound},
		{"gRPC Unavailable", &fakeOrder{err: status.Error(codes.Unavailable, "down")}, nil},
		{"envelope code != 0", &fakeOrder{resp: &pb.GetOrderInvoiceSnapshotInternalResponse{Code: 4004, Message: "查無快照"}}, ErrOrderSnapshotNotFound},
		// 🔴 code != 0 **且** data 非空：只有這一格驗得到「client 真的讀了 code」。
		// 只給「code != 0 ＋ 無 data」時，拿掉 code 檢查後 data==nil 那條會回同一個
		// 錯誤 ⇒ 假綠（PAY client 那側實測過）。
		{"envelope code != 0 ＋ data 非空", &fakeOrder{resp: &pb.GetOrderInvoiceSnapshotInternalResponse{
			Code: 4004, Message: "查無快照", Data: fullSnapshot()}}, ErrOrderSnapshotNotFound},
		{"code=0 但 data 為 nil", &fakeOrder{resp: okResp(nil)}, ErrOrderSnapshotNotFound},
		{"snapshot_version < 1", &fakeOrder{resp: okResp(badVersion)}, nil},
		{"金額欄位為空字串", &fakeOrder{resp: okResp(emptyAmount)}, nil},
		{"品項數量非正", &fakeOrder{resp: okResp(badItemQty)}, nil},
		{"preference_id 不是數字", &fakeOrder{resp: okResp(badPrefID)}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.srv, newPrefs(), 2*time.Second)
			got, err := c.GetOrderInvoiceSnapshot(context.Background(), testOrderNo, testBuyerID)
			if err == nil {
				t.Fatalf("必須回 error，實際回零值 ＋ nil：%+v", got)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Fatalf("errors.Is(err, %v) = false；err = %v", tc.wantIs, err)
			}
			// 陰性：回 error 時不得同時交出一個看起來可用的快照。
			if got.SnapshotVersion != 0 || len(got.Snapshot.Items) != 0 || got.Snapshot.TotalActualAmount != 0 {
				t.Fatalf("回 error 時仍交出了部分值：%+v", got)
			}
		})
	}
}

func TestNewClient_Guards(t *testing.T) {
	if _, err := NewClient(nil, "", newPrefs()); err == nil {
		t.Fatalf("空位址必須 fail-loud")
	}
	if _, err := NewClient(nil, "order-service:9915", nil); err == nil {
		t.Fatalf("沒有 BuyerPreferenceReader 就解析不了載具，必須 fail-loud")
	}
}

func TestNewClientWithStub_DeadlineFallback(t *testing.T) {
	c := NewClientWithStub(nil, newPrefs(), 0)
	if c.deadline != DefaultDeadline {
		t.Fatalf("deadline <= 0 必須退回 DefaultDeadline，實際 %v", c.deadline)
	}
}

package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 本檔驗 B07 寫入面的**型別邊界與 Tier A 出口形狀**；業務規則本身在
// internal/usecase/complaint_test.go。
//
// 🔴 核心斷言是「越權與查無的回應逐位元相同」，而且用 proto.Marshal 比對不是比 code：
// 比 code 只證明兩者同碼，證明不了 message、data 或任何未來新增的欄位沒有洩漏
// 「這件案子存在，只是不是你的」。

const (
	xBuyerA int64 = 2623900520
	xBuyerB int64 = 2688637900
	xOrder        = "O20260830000009"
	xDocID  int64 = 7101
)

var xNow = time.Date(2026, 3, 2, 6, 0, 0, 0, time.UTC)

type xTx struct{}

func (xTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type xDocs struct {
	domain.DocumentRepository
	owner int64
}

func (f xDocs) ListOrderDocumentsForBuyer(_ context.Context, sourceRef string, buyerID int64) ([]domain.Document, error) {
	if sourceRef != xOrder || buyerID != f.owner {
		return nil, nil
	}
	return []domain.Document{{ID: xDocID, IssuerID: 5101, CreatedAt: xNow.Add(-48 * time.Hour)}}, nil
}

type xBiz struct{ synced *time.Time }

func (xBiz) DeleteAll(context.Context) (int64, error)           { return 0, nil }
func (xBiz) Insert(context.Context, time.Time, time.Time) error { return nil }
func (xBiz) ListHolidays(context.Context, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}
func (f xBiz) SyncedAt(context.Context) (*time.Time, error) { return f.synced, nil }

type xOutbox struct{ domain.OutboxRepository }

func (xOutbox) Insert(context.Context, domain.NewOutboxEvent) (domain.OutboxEvent, error) {
	return domain.OutboxEvent{}, nil
}

// xComplaints 照 SQL 述詞回應：id ＋ complainant_buyer_id 缺一即「查不到」。
type xComplaints struct {
	domain.ComplaintRepository
	rows map[int64]domain.Complaint
}

func (r xComplaints) GetForBuyer(_ context.Context, id, buyerID int64) (domain.Complaint, error) {
	row, ok := r.rows[id]
	if !ok || row.ComplainantBuyerID != buyerID {
		return domain.Complaint{}, pkgerrors.NewBusiness(bizcode.InvoiceComplaintNotFound,
			bizcode.Message(bizcode.InvoiceComplaintNotFound))
	}
	return row, nil
}

func (r xComplaints) ListForBuyerOrder(_ context.Context, in domain.BuyerComplaintQuery) ([]domain.Complaint, error) {
	out := []domain.Complaint{}
	for _, row := range r.rows {
		if row.ComplainantBuyerID == in.BuyerID && row.OrderNo != nil && *row.OrderNo == in.OrderNo {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r xComplaints) ListEvents(context.Context, int64) ([]domain.ComplaintEvent, error) {
	return nil, nil
}

func (r xComplaints) CountWithdrawnByDocument(context.Context, int64) (int64, error) { return 0, nil }

func (r xComplaints) Create(_ context.Context, in domain.NewComplaint) (domain.Complaint, bool, error) {
	order := in.OrderNo
	row := domain.Complaint{
		ID: in.ID, DocumentID: in.DocumentID, IssuerID: in.IssuerID, OrderNo: &order,
		ComplainantBuyerID: in.ComplainantBuyerID, ReasonCode: in.ReasonCode,
		Status: codemeta.InvoiceComplaintStatusOpen.Value, SlaDueAt: in.SlaDueAt,
		CreatedAt: in.Now, UpdatedAt: in.Now,
	}
	r.rows[row.ID] = row
	return row, true, nil
}

func (r xComplaints) EscalateByBuyer(_ context.Context, id, buyerID, version int64, now time.Time) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.ComplainantBuyerID != buyerID || row.Version != version ||
		row.Status != codemeta.InvoiceComplaintStatusSellerResponded.Value {
		return false, nil
	}
	row.Status = codemeta.InvoiceComplaintStatusEscalatedToAdm.Value
	row.Version++
	row.UpdatedAt = now
	r.rows[id] = row
	return true, nil
}

func (r xComplaints) Withdraw(_ context.Context, id, buyerID, version int64, now time.Time) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.ComplainantBuyerID != buyerID || row.Version != version {
		return false, nil
	}
	row.Status = codemeta.InvoiceComplaintStatusWithdrawn.Value
	row.Version++
	r.rows[id] = row
	return true, nil
}

func (r xComplaints) Reopen(_ context.Context, id, buyerID, version int64, _, _ time.Time) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.ComplainantBuyerID != buyerID || row.Version != version {
		return false, nil
	}
	row.Status = codemeta.InvoiceComplaintStatusOpen.Value
	row.Version++
	r.rows[id] = row
	return true, nil
}

func (r xComplaints) AppendEvent(_ context.Context, in domain.NewComplaintEvent) (domain.ComplaintEvent, error) {
	return domain.ComplaintEvent{ComplaintID: in.ComplaintID, EventType: in.EventType, ActorType: in.ActorType}, nil
}

func newComplaintServer(rows ...domain.Complaint) *InvoiceServer {
	repo := xComplaints{rows: map[int64]domain.Complaint{}}
	for _, r := range rows {
		repo.rows[r.ID] = r
	}
	synced := xNow
	var seq int64 = 9100000000
	uc := usecase.NewComplaint(xTx{}, repo, xDocs{owner: xBuyerA}, xIssuers{}, xOutbox{},
		xBiz{synced: &synced}, nil, func() int64 { seq++; return seq }, nil).
		WithClock(func() time.Time { return xNow })
	return NewInvoiceServer(nil, nil, nil, uc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

// xIssuers 回一列 issuer_ref_id ≠ id 的 issuer：出向事件的 store_id 必須是前者。
type xIssuers struct{ domain.IssuerRepository }

func (xIssuers) GetByID(_ context.Context, id int64) (domain.Issuer, error) {
	return domain.Issuer{ID: id, IssuerType: "SELLER", IssuerRefID: 2685914682}, nil
}

func xRow(id, buyerID int64, status string, version int64) domain.Complaint {
	order := xOrder
	return domain.Complaint{
		ID: id, DocumentID: xDocID, IssuerID: 5101, OrderNo: &order,
		ComplainantBuyerID: buyerID, ReasonCode: "NOT_RECEIVED", Status: status,
		SlaDueAt: xNow.Add(72 * time.Hour), Version: version,
		CreatedAt: xNow.Add(-time.Hour), UpdatedAt: xNow.Add(-time.Hour),
	}
}

// ---------------------------------------------------------------------------
// Tier A：越權與查無逐位元相同
// ---------------------------------------------------------------------------

// 🔴 這是本檔最重要的一支。三支動作端點都必須通過——比 code 相同是不夠的。
func TestComplaintActionsForeignAndMissingAreByteIdentical(t *testing.T) {
	const foreignID, missingID int64 = 6001, 6002

	cases := []struct {
		name string
		call func(*InvoiceServer, string) (*pb.InvoiceComplaintResponse, error)
	}{
		{"63 rejections", func(s *InvoiceServer, id string) (*pb.InvoiceComplaintResponse, error) {
			return s.RejectInvoiceComplaintResponse(context.Background(), &pb.RejectInvoiceComplaintResponseRequest{
				BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: id, ExpectedVersion: "0", Reason: "還是沒收到",
			})
		}},
		{"64 withdrawals", func(s *InvoiceServer, id string) (*pb.InvoiceComplaintResponse, error) {
			return s.WithdrawInvoiceComplaint(context.Background(), &pb.WithdrawInvoiceComplaintRequest{
				BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: id, ExpectedVersion: "0",
			})
		}},
		{"65 reopenings", func(s *InvoiceServer, id string) (*pb.InvoiceComplaintResponse, error) {
			return s.ReopenInvoiceComplaint(context.Background(), &pb.ReopenInvoiceComplaintRequest{
				BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: id, ExpectedVersion: "0",
			})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 6001 真的存在，但屬於買家 B。
			s := newComplaintServer(xRow(foreignID, xBuyerB, codemeta.InvoiceComplaintStatusSellerResponded.Value, 0))

			foreign, err := tc.call(s, "6001")
			if err != nil {
				t.Fatalf("越權應回 envelope 不是 gRPC error：%v", err)
			}
			missing, err := tc.call(s, "6002")
			if err != nil {
				t.Fatalf("查無應回 envelope 不是 gRPC error：%v", err)
			}
			if foreign.GetCode() != bizcode.InvoiceComplaintNotFound {
				t.Fatalf("code = %d, want 9309", foreign.GetCode())
			}

			a, err := proto.Marshal(foreign)
			if err != nil {
				t.Fatal(err)
			}
			b, err := proto.Marshal(missing)
			if err != nil {
				t.Fatal(err)
			}
			if string(a) != string(b) {
				t.Fatalf("越權與查無的回應不同：\n越權 %+v\n查無 %+v", foreign, missing)
			}
			_ = missingID
		})
	}
}

// 建案的 Tier A：別人的訂單回 9301，與「這張訂單沒有發票文件」逐位元相同。
func TestCreateComplaintForeignOrderIsByteIdenticalToMissingOrder(t *testing.T) {
	s := newComplaintServer()

	foreign, err := s.CreateInvoiceComplaint(context.Background(), &pb.CreateInvoiceComplaintRequest{
		BuyerId: xBuyerB, OrderNo: xOrder, ReasonCode: "NOT_RECEIVED",
	})
	if err != nil {
		t.Fatalf("越權應回 envelope：%v", err)
	}
	missing, err := s.CreateInvoiceComplaint(context.Background(), &pb.CreateInvoiceComplaintRequest{
		BuyerId: xBuyerA, OrderNo: "O_DOES_NOT_EXIST", ReasonCode: "NOT_RECEIVED",
	})
	if err != nil {
		t.Fatalf("查無應回 envelope：%v", err)
	}
	if foreign.GetCode() != bizcode.InvoiceDocumentNotFound {
		t.Fatalf("code = %d, want 9301", foreign.GetCode())
	}
	a, _ := proto.Marshal(foreign)
	b, _ := proto.Marshal(missing)
	if string(a) != string(b) {
		t.Fatalf("越權與查無的回應不同：\n%+v\n%+v", foreign, missing)
	}
}

// ---------------------------------------------------------------------------
// 陽性路徑與回應形狀
// ---------------------------------------------------------------------------

func TestCreateComplaintSuccessShape(t *testing.T) {
	s := newComplaintServer()

	resp, err := s.CreateInvoiceComplaint(context.Background(), &pb.CreateInvoiceComplaintRequest{
		BuyerId: xBuyerA, OrderNo: xOrder, ReasonCode: "NOT_RECEIVED", Description: "沒收到",
	})
	if err != nil {
		t.Fatalf("CreateInvoiceComplaint: %v", err)
	}
	if resp.GetCode() != 0 || resp.GetMessage() != msgSuccess {
		t.Fatalf("envelope = %d/%q", resp.GetCode(), resp.GetMessage())
	}
	d := resp.GetData()
	if d.GetComplaintId() == "" || d.GetStatus() != codemeta.InvoiceComplaintStatusOpen.Value {
		t.Fatalf("data = %+v", d)
	}
	// dueAt 是 ISO 8601 UTC；remaining 是可為負的數字字串（B07 的 SlaCountdown 讀它）。
	if d.GetDueAt() == "" {
		t.Fatal("due_at 不得為空：sla_due_at 是 NOT NULL")
	}
	if d.GetRemainingBusinessDays() == "" {
		t.Fatal("行事曆已同步時 remaining_business_days 必須算得出來")
	}
	// 買家可見的欄位裡不得出現 provider 細節；本 message 型別上就沒有那些欄位（reserved）。
	if d.GetOrderNo() != xOrder {
		t.Fatalf("order_no = %q", d.GetOrderNo())
	}
}

// 9310：HTTP 200 ＋ 非零 code ＋ **data 必須在**（前端要用 data.complaintId 導頁）。
func TestCreateComplaintDuplicateCarriesExistingCaseInData(t *testing.T) {
	existing := xRow(6100, xBuyerA, codemeta.InvoiceComplaintStatusOpen.Value, 0)
	s := newComplaintServer(existing)

	resp, err := s.CreateInvoiceComplaint(context.Background(), &pb.CreateInvoiceComplaintRequest{
		BuyerId: xBuyerA, OrderNo: xOrder, ReasonCode: "NOT_RECEIVED",
	})
	if err != nil {
		t.Fatalf("重複申訴不得回 gRPC error：%v", err)
	}
	if resp.GetCode() != bizcode.InvoiceComplaintDuplicate {
		t.Fatalf("code = %d, want 9310", resp.GetCode())
	}
	if resp.GetData().GetComplaintId() != "6100" {
		t.Fatalf("9310 必須帶既有案件 id，實際 data = %+v", resp.GetData())
	}
}

func TestComplaintActionsReturnTransitionedRecord(t *testing.T) {
	s := newComplaintServer(xRow(6200, xBuyerA, codemeta.InvoiceComplaintStatusSellerResponded.Value, 4))

	resp, err := s.RejectInvoiceComplaintResponse(context.Background(), &pb.RejectInvoiceComplaintResponseRequest{
		BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "6200", ExpectedVersion: "4", Reason: "還是沒收到",
	})
	if err != nil {
		t.Fatalf("RejectInvoiceComplaintResponse: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d", resp.GetCode())
	}
	// 回的必須是**轉移後**那一版：前端據此直接覆蓋 cache，不再打一次端點 15。
	if resp.GetData().GetStatus() != codemeta.InvoiceComplaintStatusEscalatedToAdm.Value {
		t.Fatalf("status = %s", resp.GetData().GetStatus())
	}
	if resp.GetData().GetVersion() != "5" {
		t.Fatalf("version = %s, want 5", resp.GetData().GetVersion())
	}
}

// ---------------------------------------------------------------------------
// 型別邊界
// ---------------------------------------------------------------------------

func TestComplaintActionsRejectMalformedArguments(t *testing.T) {
	s := newComplaintServer(xRow(6300, xBuyerA, codemeta.InvoiceComplaintStatusSellerResponded.Value, 0))

	cases := []struct {
		name string
		req  *pb.RejectInvoiceComplaintResponseRequest
	}{
		{"缺 buyer_id", &pb.RejectInvoiceComplaintResponseRequest{OrderNo: xOrder, ComplaintId: "6300", ExpectedVersion: "0", Reason: "x"}},
		{"缺 order_no", &pb.RejectInvoiceComplaintResponseRequest{BuyerId: xBuyerA, ComplaintId: "6300", ExpectedVersion: "0", Reason: "x"}},
		{"complaint_id 非數字", &pb.RejectInvoiceComplaintResponseRequest{BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "abc", ExpectedVersion: "0", Reason: "x"}},
		{"expected_version 缺席", &pb.RejectInvoiceComplaintResponseRequest{BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "6300", Reason: "x"}},
		{"reason 缺席", &pb.RejectInvoiceComplaintResponseRequest{BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "6300", ExpectedVersion: "0"}},
		{"reason 超長", &pb.RejectInvoiceComplaintResponseRequest{BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "6300", ExpectedVersion: "0", Reason: xLong(501)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.RejectInvoiceComplaintResponse(context.Background(), tc.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("err = %v, want InvalidArgument（契約違規不借業務碼）", err)
			}
		})
	}
}

// expectedVersion = "0" 是合法值（chk_invcomp_version_nonneg 從 0 起算），
// 不得與「缺席」共用一條路。
func TestComplaintActionAcceptsZeroVersion(t *testing.T) {
	s := newComplaintServer(xRow(6400, xBuyerA, codemeta.InvoiceComplaintStatusOpen.Value, 0))

	resp, err := s.WithdrawInvoiceComplaint(context.Background(), &pb.WithdrawInvoiceComplaintRequest{
		BuyerId: xBuyerA, OrderNo: xOrder, ComplaintId: "6400", ExpectedVersion: "0",
	})
	if err != nil {
		t.Fatalf("version=0 應該通過：%v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("code = %d", resp.GetCode())
	}
}

func xLong(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = '字'
	}
	return string(b)
}

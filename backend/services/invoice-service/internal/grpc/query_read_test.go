package grpc

import (
	"context"
	"errors"
	sellerclient "github.com/Will413028/hawkr/invoice-service/internal/client/seller"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔驗 §6.1 剩餘唯讀 RPC 的 handler 層。
//
// 每個 fake 都**照 SQL 述詞的形狀**回答（擁有者是查詢條件的一部分）——與 query_test.go
// 同一條理由：真正守住 Tier A 的是 db/queries 的 WHERE，本層驗的是「repository 依述詞
// 交出什麼，handler 就吐出什麼形狀的 envelope」。SQL 那一半由
// internal/usecase/query_read_integration_test.go 用真的 PostgreSQL 驗。

// ---------------------------------------------------------------------------
// 三個與 query_test.go 共用的 fake（該檔的 fixture 也用它們的零值）
// ---------------------------------------------------------------------------

type qAdjustments struct {
	domain.AdjustmentRepository
	byDoc map[int64][]domain.Adjustment
}

func (q qAdjustments) ListByDocument(_ context.Context, docID int64) ([]domain.Adjustment, error) {
	return q.byDoc[docID], nil
}

type qCorrections struct {
	domain.CorrectionRequestRepository
	rows []domain.CorrectionRequest
	// 兩張表模擬 SQL 的第二、三段述詞（該 document 的買受人／所屬訂單）。
	docBuyer map[int64]int64
	docOrder map[int64]string
}

// GetLatestByOrder 鏡射 SQL：join 到 document 取整張**訂單**上最新那一筆（不分狀態）。
//
// 🔴 **範圍必須是訂單不是單一 document**：更正請求掛在被作廢的那張 document 上，
// 作廢重開完成後訂單的最新 document 是重開的新那張、身上沒有 correction 列
// ——替身若只比對單一 documentID，就複製不出那個缺陷、也證明不了修好了。
//
// 🔴 **不得寫成「回第一筆」**：那會讓「一筆 REJECTED ＋ 一筆新的 REQUESTED」
// 這個合法組合在測試裡拿到舊的那筆，而端點 8 要給的是最新一筆。
func (q qCorrections) GetLatestByOrder(_ context.Context, orderNo string, buyerID int64) (domain.CorrectionRequest, error) {
	var out domain.CorrectionRequest
	for _, r := range q.rows {
		if q.docOrder[r.DocumentID] != orderNo || r.RequestedByBuyerID != buyerID {
			continue
		}
		if r.ID > out.ID {
			out = r
		}
	}
	if out.ID == 0 {
		return domain.CorrectionRequest{}, notFoundBiz(bizcode.InvoiceCorrectionNotAllowed)
	}
	return out, nil
}

func (q qCorrections) GetForBuyer(_ context.Context, id, buyerID int64, sourceRef string) (domain.CorrectionRequest, error) {
	for _, r := range q.rows {
		if r.ID == id && r.RequestedByBuyerID == buyerID &&
			q.docBuyer[r.DocumentID] == buyerID && q.docOrder[r.DocumentID] == sourceRef {
			return r, nil
		}
	}
	return domain.CorrectionRequest{}, notFoundBiz(bizcode.InvoiceDocumentNotFound)
}

type qOverview struct {
	seller domain.SellerOverviewCounts
	admin  domain.AdminOverviewCounts
}

func (q qOverview) SellerCounts(context.Context, domain.SellerOverviewCountsQuery) (domain.SellerOverviewCounts, error) {
	return q.seller, nil
}

func (q qOverview) AdminCounts(context.Context, domain.AdminOverviewCountsQuery) (domain.AdminOverviewCounts, error) {
	return q.admin, nil
}

// ---------------------------------------------------------------------------
// 讀取面的 fake
// ---------------------------------------------------------------------------

const (
	rIssuerA = int64(9101)
	rIssuerB = int64(9102)
)

type rIssuers struct {
	domain.IssuerRepository
	byRef map[int64]domain.Issuer
	byID  map[int64]domain.Issuer
}

func (r rIssuers) GetByRef(_ context.Context, issuerType string, refID int64) (domain.Issuer, error) {
	if issuerType == codemeta.InvoiceIssuerTypeSeller.Value {
		if v, ok := r.byRef[refID]; ok {
			return v, nil
		}
	}
	return domain.Issuer{}, notFoundBiz(bizcode.InvoiceProviderAccountNotFound)
}

func (r rIssuers) GetByID(_ context.Context, id int64) (domain.Issuer, error) {
	if v, ok := r.byID[id]; ok {
		return v, nil
	}
	return domain.Issuer{}, notFoundBiz(bizcode.InvoiceProviderAccountNotFound)
}

// rDocs 照 SQL 的形狀回答：擁有者是述詞不是「查回來再比對」。
//
// countBias 刻意讓 Count 與 List 回不同的母體大小，用來釘住「total 來自 count query」——
// 用 len(records) 充數的實作在這裡當場紅。
type rDocs struct {
	domain.DocumentRepository
	all       []domain.Document
	secrets   map[int64]domain.RecipientSecrets
	lines     map[int64][]domain.DocumentLine
	latestOp  map[int64]int64
	countBias int64
}

func (r rDocs) byBuyer(buyerID int64) []domain.Document {
	out := []domain.Document{}
	for _, d := range r.all {
		if d.RecipientType == codemeta.InvoiceRecipientTypeBuyer.Value &&
			d.RecipientRefID != nil && *d.RecipientRefID == buyerID {
			out = append(out, d)
		}
	}
	return out
}

func (r rDocs) byIssuer(issuerID int64) []domain.Document {
	out := []domain.Document{}
	for _, d := range r.all {
		if d.IssuerID == issuerID {
			out = append(out, d)
		}
	}
	return out
}

func slicePage[T any](rows []T, p domain.Page) []T {
	if p.Offset >= int32(len(rows)) {
		return []T{}
	}
	end := p.Offset + p.Limit
	if p.Limit <= 0 || end > int32(len(rows)) {
		end = int32(len(rows))
	}
	return rows[p.Offset:end]
}

func (r rDocs) ListForBuyer(_ context.Context, in domain.BuyerDocumentQuery) ([]domain.Document, error) {
	return slicePage(r.byBuyer(in.BuyerID), in.Page), nil
}

func (r rDocs) CountForBuyer(_ context.Context, in domain.BuyerDocumentQuery) (int64, error) {
	return int64(len(r.byBuyer(in.BuyerID))) + r.countBias, nil
}

func (r rDocs) GetForBuyerByID(_ context.Context, id, buyerID int64) (domain.Document, error) {
	for _, d := range r.byBuyer(buyerID) {
		if d.ID == id {
			return d, nil
		}
	}
	return domain.Document{}, notFoundBiz(bizcode.InvoiceDocumentNotFound)
}

func (r rDocs) ListOrderDocumentsForBuyer(_ context.Context, sourceRef string, buyerID int64) ([]domain.Document, error) {
	out := []domain.Document{}
	for _, d := range r.byBuyer(buyerID) {
		if d.SourceType == sourceTypeOrder && str(d.SourceRef) == sourceRef {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r rDocs) sellerItem(d domain.Document) domain.SellerDocumentItem {
	return domain.SellerDocumentItem{Document: d, LatestOperationID: r.latestOp[d.ID]}
}

func (r rDocs) GetForSeller(_ context.Context, id, storeID int64) (domain.SellerDocumentItem, error) {
	for _, d := range r.byIssuer(issuerOfStore(storeID)) {
		if d.ID == id {
			return r.sellerItem(d), nil
		}
	}
	return domain.SellerDocumentItem{}, notFoundBiz(bizcode.InvoiceDocumentNotFound)
}

func (r rDocs) ListForSeller(_ context.Context, in domain.SellerDocumentQuery) ([]domain.SellerDocumentItem, error) {
	rows := r.byIssuer(issuerOfStore(in.StoreID))
	if in.AbnormalOnly {
		filtered := []domain.Document{}
		for _, d := range rows {
			for _, st := range in.AbnormalStatuses {
				if d.Status == st {
					filtered = append(filtered, d)
					break
				}
			}
		}
		rows = filtered
	}
	page := slicePage(rows, in.Page)
	out := make([]domain.SellerDocumentItem, 0, len(page))
	for _, d := range page {
		out = append(out, r.sellerItem(d))
	}
	return out, nil
}

func (r rDocs) CountForSeller(_ context.Context, in domain.SellerDocumentQuery) (int64, error) {
	return int64(len(r.byIssuer(issuerOfStore(in.StoreID)))), nil
}

func (r rDocs) ListRecentForIssuer(_ context.Context, issuerID int64, limit int32) ([]domain.Document, error) {
	rows := r.byIssuer(issuerID)
	if int32(len(rows)) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (r rDocs) ListLines(_ context.Context, docID int64) ([]domain.DocumentLine, error) {
	return r.lines[docID], nil
}

func (r rDocs) WithDecryptedRecipient(_ context.Context, id int64, fn func(domain.RecipientSecrets) error) error {
	return fn(r.secrets[id])
}

// issuerOfStore 是本檔 fixture 的 store_id → issuer_id 對應（模擬 SQL 的 join）。
func issuerOfStore(storeID int64) int64 {
	switch storeID {
	case storeA:
		return rIssuerA
	case storeB:
		return rIssuerB
	default:
		return 0
	}
}

type rOps struct {
	domain.OperationRepository
	views map[int64]domain.OperationView // jobID -> view
	// storeOf 模擬三表 join 的擁有者述詞。
	storeOf map[int64]int64
	byDoc   map[int64][]domain.Operation
}

func (r rOps) GetViewForSeller(_ context.Context, id, storeID int64) (domain.OperationView, error) {
	if v, ok := r.views[id]; ok && r.storeOf[id] == storeID {
		return v, nil
	}
	return domain.OperationView{}, notFoundBiz(bizcode.InvoiceJobNotFound)
}

func (r rOps) ListByDocument(_ context.Context, docID int64) ([]domain.Operation, error) {
	return r.byDoc[docID], nil
}

func (r rOps) ListOrderExternalIssuances(_ context.Context, storeID int64, _ string, allowed []string, _ domain.Page) ([]domain.OperationView, error) {
	out := []domain.OperationView{}
	for id, v := range r.views {
		if r.storeOf[id] != storeID {
			continue
		}
		for _, st := range allowed {
			if v.Operation.Status == st {
				out = append(out, v)
				break
			}
		}
	}
	return out, nil
}

func (r rOps) CountOrderExternalIssuances(ctx context.Context, storeID int64, orderNo string, allowed []string) (int64, error) {
	rows, err := r.ListOrderExternalIssuances(ctx, storeID, orderNo, allowed, domain.Page{})
	return int64(len(rows)), err
}

func (r rOps) ListPriorityViewsForAdmin(context.Context, domain.PriorityOperationQuery) ([]domain.OperationView, error) {
	return nil, nil
}

type rComplaints struct {
	domain.ComplaintRepository
	rows    []domain.Complaint
	storeOf map[int64]int64 // complaintID -> store
}

func (r rComplaints) ListForBuyerOrder(_ context.Context, in domain.BuyerComplaintQuery) ([]domain.Complaint, error) {
	out := []domain.Complaint{}
	for _, c := range r.rows {
		if c.ComplainantBuyerID == in.BuyerID && str(c.OrderNo) == in.OrderNo {
			out = append(out, c)
		}
	}
	return slicePage(out, in.Page), nil
}

func (r rComplaints) CountForBuyerOrder(ctx context.Context, in domain.BuyerComplaintQuery) (int64, error) {
	in.Page = domain.Page{}
	rows, err := r.ListForBuyerOrder(ctx, in)
	return int64(len(rows)), err
}

func (r rComplaints) GetForSeller(_ context.Context, id, storeID int64) (domain.Complaint, error) {
	for _, c := range r.rows {
		if c.ID == id && r.storeOf[id] == storeID {
			return c, nil
		}
	}
	return domain.Complaint{}, notFoundBiz(bizcode.InvoiceComplaintNotFound)
}

func (r rComplaints) ListEvents(context.Context, int64) ([]domain.ComplaintEvent, error) {
	return nil, nil
}

type rAudit struct {
	domain.AuditRepository
	correlationID string
	appended      []domain.NewAdminAuditLogEntry
}

// LatestProviderCorrelationID：替身回固定值（#3031）。
// 嵌入 domain.AuditRepository 只讓型別成立，實際呼叫會 nil-deref ⇒ 必須明寫。
func (r *rAudit) LatestProviderCorrelationID(_ context.Context, _ int64) (string, error) {
	return r.correlationID, nil
}

func (r *rAudit) AppendAdminAuditLog(_ context.Context, in domain.NewAdminAuditLogEntry) (int64, error) {
	r.appended = append(r.appended, in)
	return int64(len(r.appended)), nil
}

func (r *rAudit) ListStatusHistory(context.Context, string, int64) ([]domain.StatusHistoryEntry, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// harness
// ---------------------------------------------------------------------------

const (
	rDocOldID = int64(7001) // 作廢重開鏈的舊件
	rDocNewID = int64(7002) // 作廢重開鏈的新件
	rOrderNo  = "O000000701"
)

type rFixture struct {
	server *InvoiceServer
	docs   *rDocs
	audit  *rAudit
}

func newReadServer(t *testing.T, mut func(*rFixture)) *rFixture {
	t.Helper()
	return newReadServerWithCorrections(t, qCorrections{}, mut)
}

// newReadServerWithCorrections 讓測試注入更正請求。它與 newReadServer 分開，
// 是因為絕大多數讀取測試不該有任何更正列（有的話 latest_correction_id 會意外非空）。
func newReadServerWithCorrections(t *testing.T, corrs qCorrections, mut func(*rFixture)) *rFixture {
	t.Helper()
	buyer := buyerA
	docs := &rDocs{
		all: []domain.Document{
			// 舊件（已被取代）與新件（現行）：ListOrderDocumentsForBuyer 的排序是
			// snapshot_version DESC → 新件在前。
			{
				ID: rDocNewID, IssuerID: rIssuerA, IssuerNameSnapshot: "測試賣場",
				SourceType: sourceTypeOrder, SourceRef: strp(rOrderNo),
				RecipientType: codemeta.InvoiceRecipientTypeBuyer.Value, RecipientRefID: &buyer,
				RecipientTaxID: domain.B2CBuyerIdentifier,
				DocumentKind:   "B2C", FulfillmentMode: codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
				Status: codemeta.InvoiceDocumentStatusIssued.Value, SnapshotVersion: 2,
				CarrierType: strp("3J0002"), CarrierId1Ciphertext: strp("ENCRYPTED"),
				RecipientEmailCiphertext: strp("ENCRYPTED"),
				TotalAmount:              100, CreatedAt: time.Unix(2000, 0).UTC(),
			},
			{
				ID: rDocOldID, IssuerID: rIssuerA, IssuerNameSnapshot: "測試賣場",
				SourceType: sourceTypeOrder, SourceRef: strp(rOrderNo),
				RecipientType: codemeta.InvoiceRecipientTypeBuyer.Value, RecipientRefID: &buyer,
				RecipientTaxID: domain.B2CBuyerIdentifier,
				DocumentKind:   "B2C", FulfillmentMode: codemeta.InvoiceFulfillmentModeAmegoAuto.Value,
				Status: codemeta.InvoiceDocumentStatusSuperseded.Value, SnapshotVersion: 1,
				TotalAmount: 100, CreatedAt: time.Unix(1000, 0).UTC(),
			},
		},
		secrets: map[int64]domain.RecipientSecrets{
			rDocNewID: {CarrierID1: "/T7R3U+P", Email: "acmestore@example.com"},
		},
		latestOp: map[int64]int64{rDocNewID: 8001},
	}

	ops := rOps{
		views: map[int64]domain.OperationView{
			8001: {
				Operation: domain.Operation{
					ID: 8001, DocumentID: rDocNewID, OperationType: "ISSUE",
					Status:      codemeta.InvoiceOperationStatusManualProcessAllowed.Value,
					MaxAttempts: 13, ProviderRequestNo: "R-1",
				},
				IssuerRefID: storeA, DocumentSourceRef: rOrderNo,
			},
			8002: {
				Operation: domain.Operation{
					ID: 8002, DocumentID: rDocNewID, OperationType: "ISSUE",
					Status: codemeta.InvoiceOperationStatusFailedPermanent.Value, MaxAttempts: 13,
				},
				IssuerRefID: storeA, DocumentSourceRef: rOrderNo,
			},
		},
		storeOf: map[int64]int64{8001: storeA, 8002: storeA},
	}

	audit := &rAudit{}
	fx := &rFixture{docs: docs, audit: audit}
	q := usecase.NewQuery(
		rIssuers{
			byRef: map[int64]domain.Issuer{storeA: {ID: rIssuerA, IssuerType: "SELLER", IssuerRefID: storeA}},
			byID:  map[int64]domain.Issuer{rIssuerA: {ID: rIssuerA, IssuerType: "SELLER", IssuerRefID: storeA}},
		},
		qProfiles{byIssuer: map[int64]domain.SellerTaxProfile{}},
		qAccounts{byIssuer: map[int64]domain.ProviderAccount{}},
		qPrefs{},
		docs,
		ops,
		rComplaints{
			rows: []domain.Complaint{{
				ID: 9001, ComplainantBuyerID: buyerA, IssuerID: rIssuerA, OrderNo: strp(rOrderNo),
				Status:   codemeta.InvoiceComplaintStatusOpen.Value,
				SlaDueAt: time.Unix(5000, 0).UTC(), CreatedAt: time.Unix(1000, 0).UTC(),
			}},
			storeOf: map[int64]int64{9001: storeA},
		},
		audit,
		qAdjustments{},
		corrs,
		qOverview{},
		qBizDays{synced: syncedNow(time.Now().UTC())},
		qCutovers{},
		qUnlocks{},
		qCancelled{},
		usecase.IssuanceConfig{MaxAttempts: 13, BaseBackoff: 30 * time.Second, MaxBackoff: 30 * time.Minute},
		nil,
	)
	fx.server = NewInvoiceServer(nil, q, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if mut != nil {
		mut(fx)
	}
	return fx
}

// ---------------------------------------------------------------------------
// 端點 8：documents 是 repeated
// ---------------------------------------------------------------------------

// TestGetOrderInvoice_ReturnsEveryDocumentInTheReissueChain 釘住
// ListOrderDocumentsForBuyer 被寫成 :many 的那個理由。
//
// 判準不是「有回東西」，是**兩份都在**：只取第一筆的實作會讓買家看到現行發票而
// 完全看不到那張被作廢的舊發票（或反過來，取到舊的），而畫面完全正常。
func TestGetOrderInvoice_ReturnsEveryDocumentInTheReissueChain(t *testing.T) {
	fx := newReadServer(t, nil)
	resp, err := fx.server.GetOrderInvoice(context.Background(),
		&pb.GetOrderInvoiceRequest{BuyerId: buyerA, OrderNo: rOrderNo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	docs := resp.GetData().GetDocuments()
	if len(docs) != 2 {
		t.Fatalf("🔴 作廢重開鏈上有 2 份文件，回應只有 %d 份 —— 只取第一筆的實作會讓買家看不到另一張", len(docs))
	}
	got := map[string]bool{}
	for _, d := range docs {
		got[d.GetDocumentId()] = true
	}
	if !got["7001"] || !got["7002"] {
		t.Fatalf("兩份文件的 id 應同時出現，實得 %v", got)
	}

	// state 取**最新一份主文件**（排序第一列），不是任意一份。
	if resp.GetData().GetState() != "ISSUED" {
		t.Fatalf("state 應取最新一份主文件（ISSUED），實得 %q", resp.GetData().GetState())
	}
	// snapshot_version 同樣取最新那一份：端點 11 的 expected_snapshot_version 唯一由此取。
	if resp.GetData().GetSnapshotVersion() != "2" {
		t.Fatalf("snapshot_version 應取最新一份（2），實得 %q", resp.GetData().GetSnapshotVersion())
	}
}

// TestGetOrderInvoice_LatestCorrectionSurvivesReissue 釘住 latest_correction_id 的**範圍**。
//
// 🔴 更正請求掛在**被作廢的那張** document（rDocOldID）上，而作廢重開完成之後
// 訂單的最新 document 是**重開的新那張**（rDocNewID）、身上沒有任何 correction 列。
// 以「最新 document」為範圍去查 ⇒ 每一次**成功**的作廢重開都會讓這一欄立刻變回
// 空字串，買家一重新載入就看不到「你的更正已完成」——而端點 12 是**只能**靠這一欄
// 才到得了的（官網的 fetchInvoiceCorrection 沒有第二個 id 來源）。
//
// dev 實測 O323007025（2026-09-05）：correction 7501682805546418176 是 COMPLETED、
// reissued_document_id 指向現行文件，端點 8 仍回空字串。
//
// 變異驗證：把 GetLatestByOrder 的呼叫換回以 latest.ID 查 document ⇒ 本測轉紅。
func TestGetOrderInvoice_LatestCorrectionSurvivesReissue(t *testing.T) {
	corr := domain.CorrectionRequest{
		ID: 8801, DocumentID: rDocOldID, RequestedByBuyerID: buyerA,
		Status: "COMPLETED", HandlingMode: strp("VOID_AND_REISSUE"),
	}
	fx := newReadServerWithCorrections(t, qCorrections{
		rows:     []domain.CorrectionRequest{corr},
		docOrder: map[int64]string{rDocOldID: rOrderNo, rDocNewID: rOrderNo},
		docBuyer: map[int64]int64{rDocOldID: buyerA, rDocNewID: buyerA},
	}, nil)

	resp, err := fx.server.GetOrderInvoice(context.Background(),
		&pb.GetOrderInvoiceRequest{BuyerId: buyerA, OrderNo: rOrderNo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.GetData().GetLatestCorrectionId(); got != "8801" {
		t.Fatalf("🔴 latest_correction_id = %q，want \"8801\" —— "+
			"更正掛在被作廢的舊件上，以最新 document 查會讓成功的作廢重開把它打回空字串", got)
	}

	// 陰性：別的買家看不到這一筆（述詞含 requested_by_buyer_id）。
	other, err := fx.server.GetOrderInvoice(context.Background(),
		&pb.GetOrderInvoiceRequest{BuyerId: buyerB, OrderNo: rOrderNo})
	if err == nil && other.GetData().GetLatestCorrectionId() != "" {
		t.Fatalf("別的買家拿到了 latest_correction_id = %q", other.GetData().GetLatestCorrectionId())
	}
}

// TestGetOrderInvoice_MasksNotifiedEmail 釘住 MaskEmail 的生產路徑。
//
// 欄名 notified_email_masked 就是它的型別契約（§8.1.3(d) 裁決 3 禁止「型別上說謊」
// 的欄名）⇒ 值必須是遮罩態。明碼出現在這裡等於出現在 devtools、Gateway access log
// 的 resp_body 與前端 Sentry breadcrumb。
func TestGetOrderInvoice_MasksNotifiedEmail(t *testing.T) {
	fx := newReadServer(t, nil)
	resp, err := fx.server.GetOrderInvoice(context.Background(),
		&pb.GetOrderInvoiceRequest{BuyerId: buyerA, OrderNo: rOrderNo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := resp.GetData().GetNotifiedEmailMasked(); got != "acme***@example.com" {
		t.Fatalf("notified_email_masked 應為遮罩值，實得 %q", got)
	}
	assertNoSubstring(t, resp, "buyer email 明碼不得出現在回應裡", "acmestore@example.com")
}

// TestGetOrderInvoice_TierA_ForeignBuyerSeesAwaitingPayment 是端點 8 的 Tier A 陰性。
//
// 別人的訂單得到的**不是**錯誤，而是與「這張訂單還沒付款」完全相同的回應——
// 若 buyer_id 沒有進 SQL 的 WHERE，這裡會拿到兩份文件而當場紅。
func TestGetOrderInvoice_TierA_ForeignBuyerSeesAwaitingPayment(t *testing.T) {
	fx := newReadServer(t, nil)
	ctx := context.Background()

	// 兩次都用**同一個 order_no**：order_no 是請求回聲，換一個訂單號來比對只會
	// 比到那個回聲不同，測試會為了一個與越權無關的欄位而紅（第一版就是這樣）。
	// 要比的是「這張訂單不是我的」與「這位使用者什麼都沒有」。
	foreign, err := fx.server.GetOrderInvoice(ctx, &pb.GetOrderInvoiceRequest{BuyerId: buyerB, OrderNo: rOrderNo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	unknown, err := fx.server.GetOrderInvoice(ctx, &pb.GetOrderInvoiceRequest{BuyerId: 999999, OrderNo: rOrderNo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, foreign, unknown, "別人的訂單與沒有任何資料的買家必須完全相同（§8.1.4(c)）")
	if len(foreign.GetData().GetDocuments()) != 0 {
		t.Fatal("🔴 越權讀到了文件 —— recipient_ref_id 沒有進 SQL 的 WHERE")
	}
}

// ---------------------------------------------------------------------------
// total 來自 count query
// ---------------------------------------------------------------------------

// TestListBuyerInvoiceDocuments_TotalComesFromCountNotPageLength 釘住 total 的來源。
//
// fake 的 Count 刻意比 List 的母體多 3（模擬「第 2 頁只有 3 列但總數是 23」）：
// 用 len(records) 充數的實作在這裡當場紅。用 len 的症狀是分頁在第二頁之後說謊
// ——前端據 total 算出的頁數每翻一頁就變。
func TestListBuyerInvoiceDocuments_TotalComesFromCountNotPageLength(t *testing.T) {
	fx := newReadServer(t, func(f *rFixture) { f.docs.countBias = 3 })
	resp, err := fx.server.ListBuyerInvoiceDocuments(context.Background(),
		&pb.ListBuyerInvoiceDocumentsRequest{BuyerId: buyerA, Page: "1", PageSize: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := len(resp.GetData().GetRecords()); n != 1 {
		t.Fatalf("page_size=1 應只回 1 列，實得 %d", n)
	}
	// 母體 2 列 ＋ bias 3 ＝ 5。用 len(records) 的實作會回 "1"。
	if got := resp.GetData().GetTotal(); got != "5" {
		t.Fatalf("🔴 total 必須來自 count query（預期 \"5\"），實得 %q", got)
	}
	if resp.GetData().GetPage() != "1" || resp.GetData().GetPageSize() != "1" {
		t.Fatalf("分頁欄位應回填正規化後的值，實得 page=%q pageSize=%q",
			resp.GetData().GetPage(), resp.GetData().GetPageSize())
	}
}

// TestListBuyerInvoiceDocuments_PagesCoverTheWholeSetExactlyOnce 是跨頁的鑑別力測試。
//
// 逐頁抓完之後，各頁的列數相加必須等於 total，且沒有任何一列出現兩次。
// OFFSET 分頁在排序鍵不穩定時會漏列或重複，而那兩種失敗都不會報錯。
func TestListBuyerInvoiceDocuments_PagesCoverTheWholeSetExactlyOnce(t *testing.T) {
	fx := newReadServer(t, nil)
	ctx := context.Background()

	seen := map[string]int{}
	summed := 0
	total := ""
	for page := 1; page <= 3; page++ {
		resp, err := fx.server.ListBuyerInvoiceDocuments(ctx, &pb.ListBuyerInvoiceDocumentsRequest{
			BuyerId: buyerA, Page: fmtInt32(int32(page)), PageSize: "1",
		})
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if total == "" {
			total = resp.GetData().GetTotal()
		}
		if got := resp.GetData().GetTotal(); got != total {
			t.Fatalf("🔴 total 在翻頁之間改變了（%q → %q）—— records 與 total 分屬兩個母體", total, got)
		}
		for _, rec := range resp.GetData().GetRecords() {
			seen[rec.GetSummary().GetDocumentId()]++
			summed++
		}
	}
	if total != "2" {
		t.Fatalf("母體是 2 列，total 應為 \"2\"，實得 %q", total)
	}
	if summed != 2 {
		t.Fatalf("🔴 各頁列數相加 %d ≠ total 2 —— 分頁漏列或重複", summed)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("🔴 文件 %s 在跨頁時出現 %d 次", id, n)
		}
	}
}

// ---------------------------------------------------------------------------
// 端點 31：賣家看買家 PII 要遮罩
// ---------------------------------------------------------------------------

// TestGetSellerInvoiceDocument_MasksBuyerCarrier 是 §8.1.3(b)「載具識別碼：
// 賣家視角遮罩」的 handler 層斷言。
//
// 判準有兩層：① 回應裡是遮罩值 ② **整個序列化後的回應找不到明碼**。
// 只驗①擋不住「有人多加一個欄位把明碼帶出去」。
// DB 內是密文、三面對照的完整版在 query_read_integration_test.go。
func TestGetSellerInvoiceDocument_MasksBuyerCarrier(t *testing.T) {
	fx := newReadServer(t, nil)
	resp, err := fx.server.GetSellerInvoiceDocument(context.Background(),
		&pb.GetSellerInvoiceDocumentRequest{StoreId: storeA, DocumentId: "7002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resp.GetData().GetDetail().GetSummary().GetCarrierSummary()
	if got != "/T••••+P" {
		t.Fatalf("🔴 賣家視角的載具必須遮罩（§8.1.3(b)），實得 %q", got)
	}
	assertNoSubstring(t, resp, "買家載具明碼不得出現在賣家的回應裡", "/T7R3U+P", "acmestore@example.com")

	// funded_by 是 fail-closed 的 UNDETERMINED，不是常數 "SELLER"：
	// 寫成 SELLER 等於在契約層宣告 ADMIN 券不可能出現，真的出現時會靜默以
	// 折扣後金額開票 —— 那是稅務錯誤不是顯示錯誤。
	if resp.GetData().GetFundedBy() != fundedByUndetermined {
		t.Fatalf("funded_by 應為 UNDETERMINED（E4 fail-closed），實得 %q", resp.GetData().GetFundedBy())
	}
	// latest_job_id 是 S07「查看異常」的唯一 href 來源，少了它「看得到異常，點不進去」。
	if resp.GetData().GetLatestJobId() != "8001" {
		t.Fatalf("latest_job_id 應為 8001，實得 %q", resp.GetData().GetLatestJobId())
	}
}

// TestGetSellerInvoiceDocument_TierA_ForeignStoreIsIndistinguishable 是賣家側的 Tier A 陰性。
func TestGetSellerInvoiceDocument_TierA_ForeignStoreIsIndistinguishable(t *testing.T) {
	fx := newReadServer(t, nil)
	ctx := context.Background()

	foreign, err := fx.server.GetSellerInvoiceDocument(ctx,
		&pb.GetSellerInvoiceDocumentRequest{StoreId: storeB, DocumentId: "7002"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	missing, err := fx.server.GetSellerInvoiceDocument(ctx,
		&pb.GetSellerInvoiceDocumentRequest{StoreId: storeB, DocumentId: "9999999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, foreign, missing, "別的 store_id 與不存在的 document_id 必須完全相同")
	if foreign.GetCode() != bizcode.InvoiceDocumentNotFound {
		t.Fatalf("預期業務碼 %d，實得 %d", bizcode.InvoiceDocumentNotFound, foreign.GetCode())
	}
	if foreign.GetData() != nil {
		t.Fatal("非零業務碼不得帶 data")
	}
}

// TestGetSellerInvoiceJob_TierA_ForeignStoreIsIndistinguishable 是 job 面的 Tier A 陰性。
func TestGetSellerInvoiceJob_TierA_ForeignStoreIsIndistinguishable(t *testing.T) {
	fx := newReadServer(t, nil)
	ctx := context.Background()

	mine, err := fx.server.GetSellerInvoiceJob(ctx, &pb.GetSellerInvoiceJobRequest{StoreId: storeA, JobId: "8001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mine.GetData().GetOrderNo() != rOrderNo {
		t.Fatalf("order_no 必須由 document join 帶出（invoice_operations 沒有這個欄），實得 %q",
			mine.GetData().GetOrderNo())
	}
	if mine.GetData().GetStoreId() != "2685914682" {
		t.Fatalf("store_id 必須由 issuer join 帶出，實得 %q", mine.GetData().GetStoreId())
	}

	foreign, err := fx.server.GetSellerInvoiceJob(ctx, &pb.GetSellerInvoiceJobRequest{StoreId: storeB, JobId: "8001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	missing, err := fx.server.GetSellerInvoiceJob(ctx, &pb.GetSellerInvoiceJobRequest{StoreId: storeB, JobId: "9999999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, foreign, missing, "別的 store_id 與不存在的 job_id 必須完全相同")
	if foreign.GetCode() != bizcode.InvoiceJobNotFound {
		t.Fatalf("預期業務碼 %d，實得 %d", bizcode.InvoiceJobNotFound, foreign.GetCode())
	}
}

// ---------------------------------------------------------------------------
// 62R：述詞寫死是安全邊界不是效能優化
// ---------------------------------------------------------------------------

// TestListOrderExternalIssuances_DoesNotLeakOtherJobs 釘住 §6.1.10 對 62R 的測試要求
// （逐字：「要斷言同一張訂單有一筆 FAILED_PERMANENT 的 operation 時，它不出現在回應裡」）。
//
// 只驗 happy path 抓不到述詞被放寬，而放寬之後 62R 就退化成 S08 的等價物 ——
// 那是停權賣家 BLOCK 的畫面。
func TestListOrderExternalIssuances_DoesNotLeakOtherJobs(t *testing.T) {
	fx := newReadServer(t, nil)
	resp, err := fx.server.ListOrderExternalIssuances(context.Background(),
		&pb.ListOrderExternalIssuancesRequest{StoreId: storeA, OrderNo: rOrderNo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	records := resp.GetData().GetRecords()
	if len(records) != 1 || records[0].GetJobId() != "8001" {
		t.Fatalf("🔴 述詞被放寬了：只有 MANUAL_PROCESS_ALLOWED／RESOLVED_EXTERNALLY 可以出現，實得 %v", records)
	}
	for _, r := range records {
		if r.GetStatus() == codemeta.InvoiceOperationStatusFailedPermanent.Value {
			t.Fatal("🔴 FAILED_PERMANENT 的 job 洩漏進 62R —— 這條路由的述詞是安全邊界")
		}
	}
	// can_report_external 由後端判定（真值 ＝ MANUAL_PROCESS_ALLOWED），前端不得從 status 推導。
	if !records[0].GetCanReportExternal() {
		t.Fatal("MANUAL_PROCESS_ALLOWED 的 job 必須 can_report_external=true")
	}
}

// TestListOrderExternalIssuances_ForeignStoreGetsEmptyNot404 釘住「清單端點上的
// Tier A 正確形狀是空 records，不是 9340」。
func TestListOrderExternalIssuances_ForeignStoreGetsEmptyNot404(t *testing.T) {
	fx := newReadServer(t, nil)
	resp, err := fx.server.ListOrderExternalIssuances(context.Background(),
		&pb.ListOrderExternalIssuancesRequest{StoreId: storeB, OrderNo: rOrderNo})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetCode() != 0 {
		t.Fatalf("62R 沒有任何業務碼（查無／越權一律 code 0 ＋ 空 records），實得 %d", resp.GetCode())
	}
	if len(resp.GetData().GetRecords()) != 0 {
		t.Fatal("🔴 別的 store_id 讀到了案件")
	}
}

// ---------------------------------------------------------------------------
// A01：取不到的 KPI 不得回 0
// ---------------------------------------------------------------------------

// TestGetInvoiceOverviewForAdmin_SuspendedSellersIsUnavailableNotZero 釘住
// §6.1.11a 對 suspended_sellers 的逐字要求。
//
// 停權狀態的權威是 sellers.status（seller-service），發票模組只提建議、不執行、
// 也不持有那個狀態 ⇒ 回 "0" 看起來就像「今天沒有人被停權」，而那是一句假話。
// 同時：那一格**仍然要出現在清單裡**，缺一格會讓前端的 8 格版面塌掉。
func TestGetInvoiceOverviewForAdmin_SuspendedSellersIsUnavailableNotZero(t *testing.T) {
	fx := newReadServer(t, nil)
	resp, err := fx.server.GetInvoiceOverviewForAdmin(context.Background(),
		&pb.GetInvoiceOverviewForAdminRequest{ActorAccountId: 4242})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	kpis := resp.GetData().GetKpis()
	if len(kpis) != 8 {
		t.Fatalf("8 個 KPI 是封閉列舉，實得 %d 個", len(kpis))
	}
	var suspended *pb.InvoiceKpiItem
	for _, k := range kpis {
		if k.GetKey() == "suspended_sellers" {
			suspended = k
		}
		// 每個 KPI 都必須帶自己的口徑：沒有口徑的儀表板數字會讓兩個人看同一個
		// 畫面得到不同結論（T-I12）。
		if k.GetDefinition() == "" {
			t.Fatalf("KPI %q 缺 definition", k.GetKey())
		}
	}
	if suspended == nil {
		t.Fatal("suspended_sellers 那一格必須存在（缺一格會讓前端的 8 格版面塌掉）")
	}
	if suspended.GetValue() != "" {
		t.Fatalf("🔴 取不到的 KPI 不得回數字（\"0\" 看起來就像今天沒有人被停權），實得 %q", suspended.GetValue())
	}
	if !strings.Contains(suspended.GetDefinition(), "seller-service") {
		t.Fatalf("definition 必須說出權威在哪，實得 %q", suspended.GetDefinition())
	}
}

// ---------------------------------------------------------------------------
// 型別邊界：400 而不是靜默
// ---------------------------------------------------------------------------

// TestListRPCs_RejectContractViolations 把四類契約違規一次釘住。
//
// 四者都必須是 InvalidArgument 而不是空結果或業務碼：
//   - 身分欄缺席 ＝ Gateway 沒注入（不是「這個人沒有資料」）
//   - page_size 超上限 ＝ 前端會以為自己拿到全部（58A T25：不夾住）
//   - 日期格式鬆散 ＝ AMEGO 實測會回全部歷史，驗證要在最上游一次做對
//   - 視圖名不認得 ＝ 靜默當成「不篩」會把全部案件交出去
func TestListRPCs_RejectContractViolations(t *testing.T) {
	fx := newReadServer(t, nil)
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"buyer_id 缺席", func() error {
			_, err := fx.server.ListBuyerInvoiceDocuments(ctx, &pb.ListBuyerInvoiceDocumentsRequest{})
			return err
		}},
		{"store_id 缺席", func() error {
			_, err := fx.server.GetSellerInvoiceOverview(ctx, &pb.GetSellerInvoiceOverviewRequest{})
			return err
		}},
		{"actor_account_id 缺席", func() error {
			_, err := fx.server.ListInvoiceJobsForAdmin(ctx, &pb.ListInvoiceJobsForAdminRequest{})
			return err
		}},
		{"page_size 超上限", func() error {
			_, err := fx.server.ListBuyerInvoiceDocuments(ctx, &pb.ListBuyerInvoiceDocumentsRequest{
				BuyerId: buyerA, PageSize: "500",
			})
			return err
		}},
		{"date_from 格式鬆散", func() error {
			_, err := fx.server.ListBuyerInvoiceDocuments(ctx, &pb.ListBuyerInvoiceDocumentsRequest{
				BuyerId: buyerA, DateFrom: "2026-8-1",
			})
			return err
		}},
		{"賣家申訴視圖名不認得", func() error {
			_, err := fx.server.ListSellerInvoiceComplaints(ctx, &pb.ListSellerInvoiceComplaintsRequest{
				StoreId: storeA, Status: "AWAITING_ADM",
			})
			return err
		}},
		{"ADM job 分頁名不認得", func() error {
			_, err := fx.server.ListInvoiceJobsForAdmin(ctx, &pb.ListInvoiceJobsForAdminRequest{
				ActorAccountId: 4242, Tab: "IN_PROGRESS",
			})
			return err
		}},
		{"upload_status 投影值不認得", func() error {
			_, err := fx.server.ListInvoiceJobsForAdmin(ctx, &pb.ListInvoiceJobsForAdminRequest{
				ActorAccountId: 4242, UploadStatus: "SENT",
			})
			return err
		}},
		{"稅務審核 filter 不認得", func() error {
			_, err := fx.server.ListTaxReviewsForAdmin(ctx, &pb.ListTaxReviewsForAdminRequest{
				ActorAccountId: 4242, Filter: "ALL",
			})
			return err
		}},
		{"order_no 缺席（62R 的述詞之一）", func() error {
			_, err := fx.server.ListOrderExternalIssuances(ctx, &pb.ListOrderExternalIssuancesRequest{StoreId: storeA})
			return err
		}},
	}
	for _, c := range cases {
		if got := status.Code(c.call()); got != codes.InvalidArgument {
			t.Fatalf("%s：應回 InvalidArgument，實得 %v", c.name, got)
		}
	}
}

// TestGetSellerInvoiceComplaint_TierA 是賣家申訴詳情的 Tier A 陰性。
func TestGetSellerInvoiceComplaint_TierA(t *testing.T) {
	fx := newReadServer(t, nil)
	ctx := context.Background()

	foreign, err := fx.server.GetSellerInvoiceComplaint(ctx,
		&pb.GetSellerInvoiceComplaintRequest{StoreId: storeB, ComplaintId: "9001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	missing, err := fx.server.GetSellerInvoiceComplaint(ctx,
		&pb.GetSellerInvoiceComplaintRequest{StoreId: storeB, ComplaintId: "9999999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, foreign, missing, "別的 store_id 與不存在的 complaint_id 必須完全相同")
	if foreign.GetCode() != bizcode.InvoiceComplaintNotFound {
		t.Fatalf("預期業務碼 %d，實得 %d", bizcode.InvoiceComplaintNotFound, foreign.GetCode())
	}
}

// TestGetInvoiceCorrection_TierA_ThreePredicates 釘住 §8.1.4(b) 對端點 12 的
// 「三段擁有者述詞缺一不可」。
//
// 第三段（該 document 屬於 request 帶進來的那張訂單）最容易被省略，
// 而省略之後「訂單是我的但發票的買受人不是我」（ADM 事後改過買受人）就變成可讀。
func TestGetInvoiceCorrection_TierA_ThreePredicates(t *testing.T) {
	// 本檔的 fixture 沒有 correction 列 ⇒ 三種入參都應該落在**同一個** not-found，
	// 而那正是三段述詞都在 SQL 裡時的必然結果。
	fx := newReadServer(t, nil)
	ctx := context.Background()
	mine, err := fx.server.GetInvoiceCorrection(ctx, &pb.GetInvoiceCorrectionRequest{
		BuyerId: buyerA, OrderNo: rOrderNo, CorrectionId: "6001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foreignBuyer, err := fx.server.GetInvoiceCorrection(ctx, &pb.GetInvoiceCorrectionRequest{
		BuyerId: buyerB, OrderNo: rOrderNo, CorrectionId: "6001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wrongOrder, err := fx.server.GetInvoiceCorrection(ctx, &pb.GetInvoiceCorrectionRequest{
		BuyerId: buyerA, OrderNo: "O999999999", CorrectionId: "6001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertProtoBytesEqual(t, mine, foreignBuyer, "不同買家必須得到相同的 not-found")
	assertProtoBytesEqual(t, mine, wrongOrder, "order_no 不匹配必須走同一個 not-found 出口（不得另開一個碼）")
	if mine.GetData() != nil {
		t.Fatal("非零業務碼不得帶 data")
	}
}

// TestGetSellerInvoiceCapabilitySimple_RequiresInternalCallAndSharesTheOverviewProjection
// 釘住這支內部 RPC 的兩件事。
//
// ① 它必須過內部呼叫閘：回的是賣家的稅務審核狀態與接單上限，不是可以讓任何
//
//	同 namespace 的 pod 拿走的東西（§5.2 的 x-internal-service 只是入口，
//	真正的驗證在 interceptor 的簽章）。
//
// ② can_issue_automatically 的唯一定義是 invoice_mode == AMEGO_AUTO ——
//
//	在這裡另湊一組 if 會漏掉「連線被 SUSPENDED／REVOKED」那一格，而那一格
//	每一筆都開不出來（決策樹是雙條件，不能只看賣家資格）。
func TestGetSellerInvoiceCapabilitySimple_RequiresInternalCallAndSharesTheOverviewProjection(t *testing.T) {
	fx := newReadServer(t, nil)

	// 沒有內部呼叫標記 ⇒ 拒絕。
	if _, err := fx.server.GetSellerInvoiceCapabilitySimple(context.Background(),
		&pb.GetSellerInvoiceCapabilitySimpleRequest{StoreId: "2685914682"}); err == nil {
		t.Fatal("🔴 非內部呼叫不得取得賣家稅務狀態")
	}

	resp, err := fx.server.GetSellerInvoiceCapabilitySimple(internalCtx(),
		&pb.GetSellerInvoiceCapabilitySimpleRequest{StoreId: "2685914682"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 與 S01 總覽讀同一份投影 —— 兩份實作漂移的症狀是「seller-service 說可以、
	// 發票模組說不行」，而兩邊都不會報錯。
	overview, err := fx.server.GetSellerInvoiceOverview(internalCtx(),
		&pb.GetSellerInvoiceOverviewRequest{StoreId: storeA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetData().GetInvoiceMode() != overview.GetData().GetInvoiceMode() {
		t.Fatalf("🔴 兩支對同一位賣家給出不同的 invoice_mode：%q vs %q",
			resp.GetData().GetInvoiceMode(), overview.GetData().GetInvoiceMode())
	}
	if resp.GetData().GetTaxReviewStatus() != overview.GetData().GetTaxReviewStatus() {
		t.Fatal("🔴 兩支對同一位賣家給出不同的 tax_review_status")
	}

	wantAuto := resp.GetData().GetInvoiceMode() == codemeta.InvoiceFulfillmentModeAmegoAuto.Value
	if resp.GetData().GetCanIssueAutomatically() != wantAuto {
		t.Fatalf("can_issue_automatically 的唯一定義是 invoice_mode==AMEGO_AUTO，實得 %v（mode=%q）",
			resp.GetData().GetCanIssueAutomatically(), resp.GetData().GetInvoiceMode())
	}
	// 本 fixture 的賣家沒有稅務資格也沒有 provider 帳號 ⇒ 決策樹落在
	// LEGACY_MANUAL，能力必須是 false。回 true 代表雙條件被簡化成單條件。
	if resp.GetData().GetCanIssueAutomatically() {
		t.Fatalf("🔴 沒有稅務資格與 provider 帳號的賣家不得判為可自動開票（mode=%q）",
			resp.GetData().GetInvoiceMode())
	}
}

// ---------------------------------------------------------------------------
// #3017：ADM 工作台的識別對象欄
// ---------------------------------------------------------------------------

// rSellerNames 是店名解析的替身。
//
// 🔴 它**記下自己收到什麼**，而不是無條件回一個名字：本票的成因正是
// 「gateway 的契約測試自己手寫了 StoreName: "測試商店"，於是驗的是會不會透傳、
// 不是會不會有值」。無條件回值的替身會重蹈同一個覆轍——把 store_id 傳錯（或根本
// 沒傳）照樣全綠。
type rSellerNames struct {
	got  []int64
	byID map[int64]string
	err  error
}

func (f *rSellerNames) BatchGetSellerMeta(_ context.Context, storeIDs []int64) (map[int64]string, error) {
	f.got = append(f.got, storeIDs...)
	if f.err != nil {
		return nil, f.err
	}
	out := map[int64]string{}
	for _, id := range storeIDs {
		if n, ok := f.byID[id]; ok {
			out[id] = n
		}
	}
	return out, nil
}

func TestResolveStoreNames(t *testing.T) {
	ctx := context.Background()

	t.Run("批次去重後才送出（同一賣家在一頁出現多列是常態）", func(t *testing.T) {
		names := &rSellerNames{byID: map[int64]string{7: "甲店", 9: "乙店"}}
		s := &InvoiceServer{sellerNames: names}
		got := s.resolveStoreNames(ctx, []int64{7, 9, 7, 0, 9})
		if len(names.got) != 2 {
			t.Fatalf("送出 %v，應去重成兩個（0 也要濾掉）", names.got)
		}
		if got[7] != "甲店" || got[9] != "乙店" {
			t.Fatalf("解析結果 = %v", got)
		}
	})

	t.Run("解析失敗降級成空 map，不往上拋", func(t *testing.T) {
		s := &InvoiceServer{sellerNames: &rSellerNames{err: errors.New("unavailable")}}
		if got := s.resolveStoreNames(ctx, []int64{7}); len(got) != 0 {
			t.Fatalf("失敗時仍回了 %v —— 那些名字是哪來的？", got)
		}
	})

	t.Run("沒有 resolver 時不 panic（它是可選依賴）", func(t *testing.T) {
		s := &InvoiceServer{}
		if got := s.resolveStoreNames(ctx, []int64{7}); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})
}

// TestAdminListsPopulateStoreName 是「生產者有沒有填」的機械保證（#3017）。
//
// 🔴 **為什麼需要一支掃原始碼的測試**：本票的成因不是邏輯錯，是
// 「proto 有欄位、gateway 照實透傳、只有生產者那一端從來沒填」，而
// gateway 的契約測試 fixture 自己手寫了 `StoreName: "測試商店"`
// ⇒ 生產者永遠回空、測試永遠有值，**兩邊都綠**。
// 行為層的測試需要一整組 ADM 讀取 repository 的替身（本檔的 rFixture 沒有 wire
// 稅籍與 provider 帳戶那兩組），成本遠高於它的鑑別力。
//
// ⚠️ **這道 gate 擋得住什麼、擋不住什麼**（§3.10 的誠實標註）：
//   - 擋得住：有人把賦值刪掉、或新增第三個 ADM 列表卻忘了填
//   - 擋不住：賦值存在但取錯 map、或 resolveStoreNames 根本沒被呼叫到
//     （後者由上面的 TestResolveStoreNames 與實際 dev 驗證涵蓋）
func TestAdminListsPopulateStoreName(t *testing.T) {
	src, err := os.ReadFile("query_read.go")
	if err != nil {
		t.Fatalf("讀不到 query_read.go：%v", err)
	}
	body := string(src)

	// 對照組：先證明這個檔案裡確實找得到我們預期存在的東西。
	// 少了它，「零命中」既可能是「賦值被刪了」也可能是「檔名改了、我在讀空氣」。
	if !strings.Contains(body, "func (s *InvoiceServer) ListTaxReviewsForAdmin(") {
		t.Fatal("對照組不成立：query_read.go 裡找不到 ListTaxReviewsForAdmin —— 本測試在讀錯的檔案")
	}

	if n := strings.Count(body, "resolveStoreNames(ctx,"); n < 2 {
		t.Errorf("resolveStoreNames 只被呼叫 %d 次，A02（稅籍佇列）與 A04（串接監控）"+
			"兩個工作台都要有 —— 少一個那一頁就只剩 18 位數字 store_id（#3017）", n)
	}
	if !strings.Contains(body, "row.StoreName = names[") {
		t.Error("A02 沒有回填 store_name")
	}
	if !strings.Contains(body, "StoreName: names[") {
		t.Error("A04 沒有回填 store_name")
	}
}

// 編譯期斷言：真正的 seller client 滿足本層宣告的介面。
// 少了它，介面與實作漂移只會在 fx 圖組裝時才發現，而那是開機。
var _ SellerNameResolver = (*sellerclient.Client)(nil)

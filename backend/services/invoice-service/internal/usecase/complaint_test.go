package usecase

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// 本檔驗 B07 寫入面的**業務規則**（§6.1.6 端點 13／63／64／65 ＝ §7.3.5 K01／K07／K11／K13）。
// 型別邊界與 Tier A 的回應形狀在 internal/grpc/complaint_test.go。
//
// 🔴 三個 fake 的述詞**逐字照 SQL 寫**（allowed-from、complainant_buyer_id、version、
// reopen_count、reopen_allowed_after）。fake 寫寬鬆一點，這裡的所有守衛測試就只是在
// 驗 fake 自己，而正式路徑的 rows=0 一個都不會被抓到。

const (
	cBuyerA int64 = 2623900520
	cBuyerB int64 = 2688637900
	cDocID  int64 = 7001
	// cIssuer 是 invoice_issuers 的 PK（TSID），cStoreID 是同一列的 issuer_ref_id
	// （＝ sellers.store_id）。**兩者刻意不同**：相同的話，「payload 的 store_id 填了
	// 哪一個」這件事在測試裡不可觀測。
	cIssuer  int64 = 5001
	cStoreID int64 = 2685914682
	cOrder         = "O20260830000001"
)

// cNow 固定成週一，讓 3 工作天落在同一週（週二起算 → 週五）。
var cNow = time.Date(2026, 3, 2, 6, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type cDocs struct {
	domain.DocumentRepository
	docs map[int64][]domain.Document // buyerID → docs（新到舊）
}

func (f cDocs) ListOrderDocumentsForBuyer(_ context.Context, sourceRef string, buyerID int64) ([]domain.Document, error) {
	if sourceRef != cOrder {
		return nil, nil
	}
	return f.docs[buyerID], nil
}

type cBizDays struct {
	synced   *time.Time
	holidays []time.Time
}

func (cBizDays) DeleteAll(context.Context) (int64, error)           { return 0, nil }
func (cBizDays) Insert(context.Context, time.Time, time.Time) error { return nil }
func (f cBizDays) ListHolidays(context.Context, time.Time, time.Time) ([]time.Time, error) {
	return f.holidays, nil
}
func (f cBizDays) SyncedAt(context.Context) (*time.Time, error) { return f.synced, nil }

type cComplaints struct {
	domain.ComplaintRepository

	rows      map[int64]domain.Complaint
	events    map[int64][]domain.ComplaintEvent
	withdrawn int64

	created  []domain.NewComplaint
	appended []domain.NewComplaintEvent
	nextID   int64

	// K04（#3055 的橫向稽查）：overdueRows 是 MarkOverdue 這一輪要回的列，
	// overdueCalls 記它被呼叫幾次、markErr 讓它失敗。
	overdueRows  []domain.Complaint
	overdueCalls int
	overdueErr   error

	// K05（2026-09-08 的零呼叫端稽核）：escalateBefore 記下呼叫端算出來的窗口上界，
	// 那是本 fake 唯一有鑑別力的地方——只記 calls 的替身對「窗口算錯」完全無感，
	// 而算錯的後果是「剛進 OVERDUE 的申訴立刻被升級」或「永遠升級不了」。
	escalateRows   []domain.Complaint
	escalateCalls  int
	escalateBefore time.Time
	escalateErr    error

	// K06（同一輪稽核，修正 adapter 自呼叫的假陰性後才看見）：
	// reviewReopenAfter／reviewResolution 是本 fake 唯一有鑑別力的兩格——
	// 冷卻窗口算錯、或結案理由填成 VIOLATION（會計進賣家違規窗口）都只有它們抓得到。
	reviewRows        []domain.Complaint
	reviewCalls       int
	reviewReopenAfter time.Time
	reviewResolution  string
	reviewErr         error
}

func (r *cComplaints) ResolveExpiredBuyerReviews(_ context.Context, _, reopenAllowedAfter time.Time, resolutionCode string, _ int32) ([]domain.Complaint, error) {
	r.reviewCalls++
	r.reviewReopenAfter = reopenAllowedAfter
	r.reviewResolution = resolutionCode
	if r.reviewErr != nil {
		return nil, r.reviewErr
	}
	return r.reviewRows, nil
}

// EscalateOverdue 是 K05 的候選查詢＋升級。真實述詞（status='OVERDUE' AND
// overdue_at <= escalate_before）由 repository 的 integration test 守；本 fake 只把
// 呼叫端算出來的 escalate_before 留下來讓測試斷言。
func (r *cComplaints) EscalateOverdue(_ context.Context, _, escalateBefore time.Time, _ int32) ([]domain.Complaint, error) {
	r.escalateCalls++
	r.escalateBefore = escalateBefore
	if r.escalateErr != nil {
		return nil, r.escalateErr
	}
	return r.escalateRows, nil
}

// MarkOverdue 是 K04 的候選查詢＋標記。fake 只回預先擺好的列，
// 真實述詞（status='OPEN' AND sla_due_at <= now）由 repository 的 integration test 守。
func (r *cComplaints) MarkOverdue(_ context.Context, _ time.Time, _ int32) ([]domain.Complaint, error) {
	r.overdueCalls++
	if r.overdueErr != nil {
		return nil, r.overdueErr
	}
	return r.overdueRows, nil
}

func newComplaintRepo(rows ...domain.Complaint) *cComplaints {
	r := &cComplaints{rows: map[int64]domain.Complaint{}, events: map[int64][]domain.ComplaintEvent{}, nextID: 1}
	for _, row := range rows {
		r.rows[row.ID] = row
	}
	return r
}

// GetForBuyer 逐字照 GetInvoiceComplaintForBuyer：id ＋ complainant_buyer_id 兩條述詞。
func (r *cComplaints) GetForBuyer(_ context.Context, id, buyerID int64) (domain.Complaint, error) {
	row, ok := r.rows[id]
	if !ok || row.ComplainantBuyerID != buyerID {
		return domain.Complaint{}, complaintNotFound()
	}
	return row, nil
}

func (r *cComplaints) ListForBuyerOrder(_ context.Context, in domain.BuyerComplaintQuery) ([]domain.Complaint, error) {
	out := []domain.Complaint{}
	for _, row := range r.rows {
		if row.ComplainantBuyerID == in.BuyerID && derefString(row.OrderNo) == in.OrderNo {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *cComplaints) ListEvents(_ context.Context, id int64) ([]domain.ComplaintEvent, error) {
	return r.events[id], nil
}

func (r *cComplaints) CountWithdrawnByDocument(context.Context, int64) (int64, error) {
	return r.withdrawn, nil
}

// Create 照 uq_invcomp_open_per_document：同一 document 已有未結案案件即 inserted=false。
func (r *cComplaints) Create(_ context.Context, in domain.NewComplaint) (domain.Complaint, bool, error) {
	r.created = append(r.created, in)
	for _, row := range r.rows {
		if row.DocumentID == in.DocumentID && containsString(openComplaintStatuses, row.Status) {
			return domain.Complaint{}, false, nil
		}
	}
	row := domain.Complaint{
		ID: in.ID, DocumentID: in.DocumentID, IssuerID: in.IssuerID,
		OrderNo: &in.OrderNo, ComplainantBuyerID: in.ComplainantBuyerID,
		ReasonCode: in.ReasonCode, Status: codemeta.InvoiceComplaintStatusOpen.Value,
		SlaDueAt: in.SlaDueAt, CreatedAt: in.Now, UpdatedAt: in.Now,
	}
	if in.Description != "" {
		d := in.Description
		row.Description = &d
	}
	r.rows[row.ID] = row
	return row, true, nil
}

// EscalateByBuyer 逐字照 K07 的 WHERE：狀態 ＋ 買家 ＋ 版本。
func (r *cComplaints) EscalateByBuyer(_ context.Context, id, buyerID, expectedVersion int64, now time.Time) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.Status != codemeta.InvoiceComplaintStatusSellerResponded.Value ||
		row.ComplainantBuyerID != buyerID || row.Version != expectedVersion {
		return false, nil
	}
	row.Status = codemeta.InvoiceComplaintStatusEscalatedToAdm.Value
	row.Version++
	row.UpdatedAt = now
	r.rows[id] = row
	return true, nil
}

// Withdraw 逐字照 K11 的 WHERE：三個 allowed-from ＋ 買家 ＋ 版本。
func (r *cComplaints) Withdraw(_ context.Context, id, buyerID, expectedVersion int64, now time.Time) (bool, error) {
	row, ok := r.rows[id]
	allowed := []string{
		codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusOverdue.Value,
		codemeta.InvoiceComplaintStatusSellerResponded.Value,
	}
	if !ok || !containsString(allowed, row.Status) ||
		row.ComplainantBuyerID != buyerID || row.Version != expectedVersion {
		return false, nil
	}
	row.Status = codemeta.InvoiceComplaintStatusWithdrawn.Value
	row.Version++
	row.UpdatedAt = now
	r.rows[id] = row
	return true, nil
}

// Reopen 逐字照 K13 的 WHERE：RESOLVED ＋ 買家 ＋ 版本 ＋ reopen_count < 2 ＋ 冷卻已過。
func (r *cComplaints) Reopen(_ context.Context, id, buyerID, expectedVersion int64, slaDueAt, now time.Time) (bool, error) {
	row, ok := r.rows[id]
	if !ok || row.Status != codemeta.InvoiceComplaintStatusResolved.Value ||
		row.ComplainantBuyerID != buyerID || row.Version != expectedVersion ||
		row.ReopenCount >= 2 ||
		row.ReopenAllowedAfter == nil || row.ReopenAllowedAfter.After(now) {
		return false, nil
	}
	row.Status = codemeta.InvoiceComplaintStatusOpen.Value
	row.ResolvedAt = nil
	row.ReopenAllowedAfter = nil
	row.ReopenCount++
	row.SlaDueAt = slaDueAt
	row.Version++
	row.UpdatedAt = now
	r.rows[id] = row
	return true, nil
}

func (r *cComplaints) AppendEvent(_ context.Context, in domain.NewComplaintEvent) (domain.ComplaintEvent, error) {
	r.appended = append(r.appended, in)
	e := domain.ComplaintEvent{
		ID: r.nextID, ComplaintID: in.ComplaintID, EventType: in.EventType,
		ActorType: in.ActorType, OccurredAt: in.Now,
	}
	r.nextID++
	r.events[in.ComplaintID] = append(r.events[in.ComplaintID], e)
	return e, nil
}

// ---------------------------------------------------------------------------
// fixture
// ---------------------------------------------------------------------------

type cFixture struct {
	uc      *Complaint
	repo    *cComplaints
	outbox  *fakeOutbox
	docs    cDocs
	created time.Time
	audit   *fakeAudit
}

// newComplaintUC 造一個「文件已建立 48 小時、行事曆已同步且無國定假日」的環境。
func newComplaintUC(t *testing.T, rows ...domain.Complaint) *cFixture {
	t.Helper()
	docCreated := cNow.Add(-48 * time.Hour)
	docs := cDocs{docs: map[int64][]domain.Document{
		cBuyerA: {{ID: cDocID, IssuerID: cIssuer, CreatedAt: docCreated}},
	}}
	repo := newComplaintRepo(rows...)
	ob := &fakeOutbox{}
	synced := cNow
	var seq int64 = 9000000000
	// 🔴 issuer.ID（PK）與 issuer.IssuerRefID（＝store_id）刻意給**不同**的值：
	// 兩者相同的 fixture 對「填錯了哪一個」零鑑別力。
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{
		ID: cIssuer, IssuerType: "SELLER", IssuerRefID: cStoreID,
	}}
	audit := &fakeAudit{}
	uc := NewComplaint(directTx{}, repo, docs, issuers, ob, cBizDays{synced: &synced}, audit,
		func() int64 { seq++; return seq }, nil).
		WithClock(func() time.Time { return cNow })
	return &cFixture{uc: uc, repo: repo, outbox: ob, docs: docs, created: docCreated, audit: audit}
}

func complaintRow(id, buyerID int64, status string, version int64) domain.Complaint {
	order := cOrder
	return domain.Complaint{
		ID: id, DocumentID: cDocID, IssuerID: cIssuer, OrderNo: &order,
		ComplainantBuyerID: buyerID, ReasonCode: "NOT_RECEIVED",
		Status: status, SlaDueAt: cNow.Add(72 * time.Hour), Version: version,
		CreatedAt: cNow.Add(-time.Hour), UpdatedAt: cNow.Add(-time.Hour),
	}
}

func bizCodeOf(t *testing.T, err error) int32 {
	t.Helper()
	if err == nil {
		t.Fatal("預期業務錯誤，實際成功")
	}
	code, _, ok := errAsBiz(err)
	if !ok {
		t.Fatalf("預期 DomainError，實際 %v", err)
	}
	return code
}

// ---------------------------------------------------------------------------
// K01 建案（端點 13）
// ---------------------------------------------------------------------------

func TestFileComplaintCreatesCaseWithSlaEventAndOutbox(t *testing.T) {
	fx := newComplaintUC(t)

	res, err := fx.uc.File(context.Background(), FileComplaintInput{
		BuyerID: cBuyerA, OrderNo: cOrder, ReasonCode: "NOT_RECEIVED", Description: "沒收到",
	})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if res.Duplicate {
		t.Fatal("不該是重複申訴")
	}
	if got := res.View.Complaint.Status; got != codemeta.InvoiceComplaintStatusOpen.Value {
		t.Fatalf("status = %s, want OPEN", got)
	}
	// 起算週二 00:00（台北）→ 推 3 個工作天 → 週五 00:00 台北 ＝ 03-05T16:00Z。
	wantDue := time.Date(2026, 3, 5, 16, 0, 0, 0, time.UTC)
	if !res.View.Complaint.SlaDueAt.Equal(wantDue) {
		t.Fatalf("sla_due_at = %s, want %s", res.View.Complaint.SlaDueAt, wantDue)
	}
	// 時間軸：CREATED ＋ actor BUYER ＋ action 必須留空（買家的動作不進 action 軸）。
	if len(fx.repo.appended) != 1 {
		t.Fatalf("appended = %d 列，want 1", len(fx.repo.appended))
	}
	ev := fx.repo.appended[0]
	if ev.EventType != codemeta.InvoiceComplaintEventTypeCreated.Value || ev.ActorType != "BUYER" || ev.Action != "" {
		t.Fatalf("event = %+v", ev)
	}
	// outbox：通知賣家，與建案同交易。
	if len(fx.outbox.inserted) != 1 {
		t.Fatalf("outbox = %d 列，want 1", len(fx.outbox.inserted))
	}
	if got := fx.outbox.inserted[0].RoutingKey; got != pkgmq.RoutingKeyInvoiceComplaintCreated {
		t.Fatalf("routing key = %s", got)
	}
	if got := fx.outbox.inserted[0].AggregateType; got != "COMPLAINT" {
		t.Fatalf("aggregate_type = %s, want COMPLAINT", got)
	}
}

// 9311：開票觸發時點 + 24h 之前不開放。不設閘 ⇒ 申訴變催單管道 ⇒ 誤判違規 ⇒ 停權。
func TestFileComplaintBeforeGateReturns9311(t *testing.T) {
	fx := newComplaintUC(t)
	fx.docs.docs[cBuyerA] = []domain.Document{{ID: cDocID, IssuerID: cIssuer, CreatedAt: cNow.Add(-time.Hour)}}

	_, err := fx.uc.File(context.Background(), FileComplaintInput{BuyerID: cBuyerA, OrderNo: cOrder})
	if got := bizCodeOf(t, err); got != bizcode.InvoiceComplaintNotOpenYet {
		t.Fatalf("code = %d, want 9311", got)
	}
	if len(fx.repo.created) != 0 {
		t.Fatal("閘門擋下時不得寫入任何案件")
	}
}

// 9310：已有進行中案件 ⇒ 不是錯誤，回既有那一件（HTTP 200 ＋ data.complaintId）。
func TestFileComplaintDuplicateReturnsExistingCase(t *testing.T) {
	existing := complaintRow(4242, cBuyerA, codemeta.InvoiceComplaintStatusOpen.Value, 0)
	fx := newComplaintUC(t, existing)

	res, err := fx.uc.File(context.Background(), FileComplaintInput{BuyerID: cBuyerA, OrderNo: cOrder})
	if err != nil {
		t.Fatalf("重複申訴不該是錯誤：%v", err)
	}
	if !res.Duplicate || res.View.Complaint.ID != existing.ID {
		t.Fatalf("res = %+v，want Duplicate=true 且指向 4242", res)
	}
	if len(fx.outbox.inserted) != 0 {
		t.Fatal("重複申訴不得再發一次 invoice.complaint.created")
	}
}

// 9336：撤回不是免費的重來鍵——同一 document 已 WITHDRAWN 兩件即擋下。
func TestFileComplaintWithdrawQuotaExhaustedReturns9336(t *testing.T) {
	fx := newComplaintUC(t)
	fx.repo.withdrawn = 2

	_, err := fx.uc.File(context.Background(), FileComplaintInput{BuyerID: cBuyerA, OrderNo: cOrder})
	if got := bizCodeOf(t, err); got != bizcode.InvoiceComplaintActionNotAllowed {
		t.Fatalf("code = %d, want 9336", got)
	}
}

// Tier A：別人的訂單得到空清單 ⇒ 與「這張訂單沒有發票文件」共用 9301。
func TestFileComplaintForeignOrderReturns9301(t *testing.T) {
	fx := newComplaintUC(t)

	_, err := fx.uc.File(context.Background(), FileComplaintInput{BuyerID: cBuyerB, OrderNo: cOrder})
	if got := bizCodeOf(t, err); got != bizcode.InvoiceDocumentNotFound {
		t.Fatalf("code = %d, want 9301", got)
	}
}

func TestFileComplaintRejectsUnknownReasonCodeAndLongDescription(t *testing.T) {
	fx := newComplaintUC(t)

	if _, err := fx.uc.File(context.Background(), FileComplaintInput{
		BuyerID: cBuyerA, OrderNo: cOrder, ReasonCode: "SOMETHING_ELSE",
	}); !errors.Is(err, ErrComplaintReasonCodeInvalid) {
		t.Fatalf("err = %v, want ErrComplaintReasonCodeInvalid", err)
	}
	if _, err := fx.uc.File(context.Background(), FileComplaintInput{
		BuyerID: cBuyerA, OrderNo: cOrder, Description: longText(501),
	}); !errors.Is(err, ErrComplaintDescriptionTooLong) {
		t.Fatalf("err = %v, want ErrComplaintDescriptionTooLong", err)
	}
	if len(fx.repo.created) != 0 {
		t.Fatal("格式錯的請求不得進到寫入")
	}
}

// 行事曆從未同步 ⇒ 落硬上限（偏晚），**不得**退化成 3 自然日。
func TestFileComplaintWithStaleCalendarUsesHardCap(t *testing.T) {
	fx := newComplaintUC(t)
	fx.uc.businessDay = cBizDays{synced: nil}

	res, err := fx.uc.File(context.Background(), FileComplaintInput{BuyerID: cBuyerA, OrderNo: cOrder})
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	wantCap := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC).Add(-8 * time.Hour) // 03-16 00:00 台北
	if !res.View.Complaint.SlaDueAt.Equal(wantCap) {
		t.Fatalf("sla_due_at = %s, want 硬上限 %s", res.View.Complaint.SlaDueAt, wantCap)
	}
	// 倒數算不出來就明說算不出來——回 0 會被前端讀成「今天到期」。
	if res.View.RemainingBusinessDays != nil {
		t.Fatalf("remaining = %d，行事曆未涵蓋時應保持未計算", *res.View.RemainingBusinessDays)
	}
}

// ---------------------------------------------------------------------------
// K07／K11／K13 三支動作端點（63／64／65）
// ---------------------------------------------------------------------------

func TestRejectEscalatesToAdm(t *testing.T) {
	row := complaintRow(5001, cBuyerA, codemeta.InvoiceComplaintStatusSellerResponded.Value, 3)
	fx := newComplaintUC(t, row)

	view, err := fx.uc.Reject(context.Background(), ComplaintActionInput{
		BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID, ExpectedVersion: 3, Reason: "還是沒收到",
	})
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if view.Complaint.Status != codemeta.InvoiceComplaintStatusEscalatedToAdm.Value {
		t.Fatalf("status = %s", view.Complaint.Status)
	}
	if view.Complaint.Version != 4 {
		t.Fatalf("version = %d, want 4（回的必須是轉移後那一版）", view.Complaint.Version)
	}
	ev := fx.repo.appended[len(fx.repo.appended)-1]
	if ev.EventType != codemeta.InvoiceComplaintEventTypeEscalated.Value || ev.Action != "" {
		t.Fatalf("event = %+v", ev)
	}
}

func TestWithdrawFromEachAllowedStatus(t *testing.T) {
	for _, from := range []string{
		codemeta.InvoiceComplaintStatusOpen.Value,
		codemeta.InvoiceComplaintStatusOverdue.Value,
		codemeta.InvoiceComplaintStatusSellerResponded.Value,
	} {
		t.Run(from, func(t *testing.T) {
			row := complaintRow(5002, cBuyerA, from, 1)
			fx := newComplaintUC(t, row)
			view, err := fx.uc.Withdraw(context.Background(), ComplaintActionInput{
				BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID, ExpectedVersion: 1,
			})
			if err != nil {
				t.Fatalf("Withdraw: %v", err)
			}
			if view.Complaint.Status != codemeta.InvoiceComplaintStatusWithdrawn.Value {
				t.Fatalf("status = %s", view.Complaint.Status)
			}
			// 撤回不寫 resolved_at（chk_invcomp_resolved_pair 只綁 RESOLVED／VIOLATION_CONFIRMED）。
			if view.Complaint.ResolvedAt != nil {
				t.Fatal("撤回不得寫 resolved_at")
			}
		})
	}
}

func TestReopenResetsSlaAndBumpsCount(t *testing.T) {
	row := complaintRow(5003, cBuyerA, codemeta.InvoiceComplaintStatusResolved.Value, 7)
	resolved := cNow.Add(-48 * time.Hour)
	allowed := cNow.Add(-24 * time.Hour)
	row.ResolvedAt = &resolved
	row.ReopenAllowedAfter = &allowed
	fx := newComplaintUC(t, row)

	view, err := fx.uc.Reopen(context.Background(), ComplaintActionInput{
		BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID, ExpectedVersion: 7,
	})
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if view.Complaint.Status != codemeta.InvoiceComplaintStatusOpen.Value {
		t.Fatalf("status = %s, want OPEN", view.Complaint.Status)
	}
	if view.Complaint.ReopenCount != 1 {
		t.Fatalf("reopen_count = %d, want 1", view.Complaint.ReopenCount)
	}
	if view.Complaint.ResolvedAt != nil {
		t.Fatal("離開 RESOLVED 必須清空 resolved_at（chk_invcomp_resolved_pair 雙向等式）")
	}
	// SLA 必須重置：不重置的話這一列一進 OPEN 就落回逾期掃描的候選集合。
	wantDue := time.Date(2026, 3, 5, 16, 0, 0, 0, time.UTC)
	if !view.Complaint.SlaDueAt.Equal(wantDue) {
		t.Fatalf("sla_due_at = %s, want 重置為 %s", view.Complaint.SlaDueAt, wantDue)
	}
}

// 🔴 狀態機陰性：不合法的來源狀態一律 9336，且資料未被改動。
func TestActionsFromDisallowedStatusReturn9336AndDoNotMutate(t *testing.T) {
	cases := []struct {
		name string
		from string
		run  func(*Complaint, ComplaintActionInput) error
	}{
		{"63 只能從 SELLER_RESPONDED", codemeta.InvoiceComplaintStatusOpen.Value,
			func(u *Complaint, in ComplaintActionInput) error {
				in.Reason = "x"
				_, err := u.Reject(context.Background(), in)
				return err
			}},
		{"64 不能撤回已結案", codemeta.InvoiceComplaintStatusResolved.Value,
			func(u *Complaint, in ComplaintActionInput) error {
				_, err := u.Withdraw(context.Background(), in)
				return err
			}},
		{"65 只能重開 RESOLVED", codemeta.InvoiceComplaintStatusOpen.Value,
			func(u *Complaint, in ComplaintActionInput) error {
				_, err := u.Reopen(context.Background(), in)
				return err
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := complaintRow(5004, cBuyerA, tc.from, 2)
			fx := newComplaintUC(t, row)
			err := tc.run(fx.uc, ComplaintActionInput{
				BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID, ExpectedVersion: 2,
			})
			if got := bizCodeOf(t, err); got != bizcode.InvoiceComplaintActionNotAllowed {
				t.Fatalf("code = %d, want 9336", got)
			}
			after := fx.repo.rows[row.ID]
			if after.Status != tc.from || after.Version != 2 {
				t.Fatalf("守衛不成立卻改到了資料：%s v%d", after.Status, after.Version)
			}
			if len(fx.repo.appended) != 0 {
				t.Fatal("守衛不成立不得留下時間軸事件")
			}
		})
	}
}

// 重開的另外兩條守衛：次數用完、冷卻未到，兩者都回 9336。
func TestReopenGuardsOnCountAndCooldown(t *testing.T) {
	past := cNow.Add(-time.Hour)
	future := cNow.Add(time.Hour)
	resolved := cNow.Add(-48 * time.Hour)

	cases := []struct {
		name       string
		count      int16
		allowedAft *time.Time
	}{
		{"已達 2 次上限", 2, &past},
		{"冷卻未到", 0, &future},
		{"冷卻欄為空", 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := complaintRow(5005, cBuyerA, codemeta.InvoiceComplaintStatusResolved.Value, 1)
			row.ReopenCount = tc.count
			row.ReopenAllowedAfter = tc.allowedAft
			row.ResolvedAt = &resolved
			fx := newComplaintUC(t, row)

			_, err := fx.uc.Reopen(context.Background(), ComplaintActionInput{
				BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID, ExpectedVersion: 1,
			})
			if got := bizCodeOf(t, err); got != bizcode.InvoiceComplaintActionNotAllowed {
				t.Fatalf("code = %d, want 9336", got)
			}
			if fx.repo.rows[row.ID].ReopenCount != tc.count {
				t.Fatal("守衛不成立卻改到了 reopen_count")
			}
		})
	}
}

// 版本不符回 9322，且不得改到資料。
func TestActionsWithStaleVersionReturn9322(t *testing.T) {
	row := complaintRow(5006, cBuyerA, codemeta.InvoiceComplaintStatusSellerResponded.Value, 9)
	fx := newComplaintUC(t, row)

	_, err := fx.uc.Reject(context.Background(), ComplaintActionInput{
		BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID, ExpectedVersion: 8, Reason: "x",
	})
	if got := bizCodeOf(t, err); got != bizcode.SellerTaxProfileVersionConflict {
		t.Fatalf("code = %d, want 9322", got)
	}
	if fx.repo.rows[row.ID].Status != codemeta.InvoiceComplaintStatusSellerResponded.Value {
		t.Fatal("版本不符卻改到了狀態")
	}
}

// 🔴 Tier A：拿 A 的身分操作 B 的案件，回的必須與「不存在」同一個碼。
func TestActionsAreOwnerScoped(t *testing.T) {
	row := complaintRow(5007, cBuyerB, codemeta.InvoiceComplaintStatusSellerResponded.Value, 0)
	fx := newComplaintUC(t, row)

	in := ComplaintActionInput{BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID, Reason: "x"}
	_, foreign := fx.uc.Reject(context.Background(), in)
	missing := in
	missing.ComplaintID = 999999
	_, absent := fx.uc.Reject(context.Background(), missing)

	if bizCodeOf(t, foreign) != bizcode.InvoiceComplaintNotFound {
		t.Fatalf("越權 code = %d, want 9309", bizCodeOf(t, foreign))
	}
	if foreign.Error() != absent.Error() {
		t.Fatalf("越權與查無的訊息不同：%q vs %q", foreign, absent)
	}
	if fx.repo.rows[row.ID].Status != codemeta.InvoiceComplaintStatusSellerResponded.Value {
		t.Fatal("越權卻改到了別人的案件")
	}
}

// order_no 不匹配走**同一個** 9309 出口，不另開碼。
func TestActionsWithMismatchedOrderReturnSameNotFound(t *testing.T) {
	row := complaintRow(5008, cBuyerA, codemeta.InvoiceComplaintStatusSellerResponded.Value, 0)
	fx := newComplaintUC(t, row)

	_, err := fx.uc.Reject(context.Background(), ComplaintActionInput{
		BuyerID: cBuyerA, OrderNo: "O_OTHER", ComplaintID: row.ID, Reason: "x",
	})
	if got := bizCodeOf(t, err); got != bizcode.InvoiceComplaintNotFound {
		t.Fatalf("code = %d, want 9309", got)
	}
}

func TestRejectRequiresReasonAndWithdrawDoesNot(t *testing.T) {
	row := complaintRow(5009, cBuyerA, codemeta.InvoiceComplaintStatusSellerResponded.Value, 0)
	fx := newComplaintUC(t, row)

	if _, err := fx.uc.Reject(context.Background(), ComplaintActionInput{
		BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID,
	}); !errors.Is(err, ErrComplaintReasonRequired) {
		t.Fatalf("63 缺理由應 400，實際 %v", err)
	}
	if _, err := fx.uc.Withdraw(context.Background(), ComplaintActionInput{
		BuyerID: cBuyerA, OrderNo: cOrder, ComplaintID: row.ID,
	}); err != nil {
		t.Fatalf("64 沒有 reason 欄，不得因為缺理由而失敗：%v", err)
	}
}

// ---------------------------------------------------------------------------
// 交易邊界
// ---------------------------------------------------------------------------

// failingTx 讓 callback 跑完之後整筆失敗，模擬 rollback。
type failingTx struct{ err error }

func (f failingTx) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return f.err
}

// 建案的三個寫入（案件、時間軸、outbox）必須同進同退：交易失敗時呼叫端拿到錯誤，
// 不得回一個「看起來成功」的結果。
func TestFileComplaintPropagatesTransactionFailure(t *testing.T) {
	fx := newComplaintUC(t)
	boom := errors.New("commit failed")
	fx.uc.tx = failingTx{err: boom}

	res, err := fx.uc.File(context.Background(), FileComplaintInput{BuyerID: cBuyerA, OrderNo: cOrder})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if res.View.Complaint.ID != 0 {
		t.Fatal("交易失敗卻回了一份結果")
	}
}

func longText(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = '字'
	}
	return string(b)
}

// errAsBiz 把 DomainError 的業務碼拆出來（bizOf 住在 internal/grpc，這裡不能用）。
func errAsBiz(err error) (int32, string, bool) {
	var de *pkgerrors.DomainError
	if !errors.As(err, &de) {
		return 0, "", false
	}
	return int32(de.Code), de.DomainMessage(), true
}

// ---------------------------------------------------------------------------
// K05：申訴逾期掃描（P10，#3055 的橫向稽查）
// ---------------------------------------------------------------------------

// 逾期的申訴必須標成 OVERDUE **並**通知賣家。
//
// 🔴 這支之前不存在，而 ComplaintRepository.MarkOverdue 早就實作好了、零呼叫端 ⇒
// 逾期的申訴永遠停在 OPEN、賣家不會被催、ADM 的 OVERDUE 分頁恆空，
// 而 3 個工作天是本模組對買家的承諾。
// K05：OVERDUE 滿 24 小時 → ESCALATED_TO_ADM。
//
// 🔴 這支之前不存在，而 ComplaintRepository.EscalateOverdue 從介面到 sqlc 生成物
// 一應俱全、零呼叫端 ⇒ OVERDUE 的三條出邊裡唯一不需要外人動作的那條從來不會發生，
// 申訴永遠停在 OVERDUE、平台永遠不會接手。dev 上 7500699963840528384 卡了 5 天。
func TestEscalateOverdueComplaints_TransitionsAndLeavesTrail(t *testing.T) {
	f := newComplaintUC(t)
	f.repo.escalateRows = []domain.Complaint{
		complaintRow(7101, cBuyerA, "ESCALATED_TO_ADM", 4),
		complaintRow(7102, cBuyerA, "ESCALATED_TO_ADM", 5),
	}

	n, err := f.uc.EscalateOverdueComplaints(context.Background(), cNow)
	if err != nil {
		t.Fatalf("EscalateOverdueComplaints: %v", err)
	}
	if n != 2 {
		t.Fatalf("回報升級 %d 筆，want 2", n)
	}
	if f.repo.escalateCalls != 1 {
		t.Fatalf("EscalateOverdue 被呼叫 %d 次，want 1", f.repo.escalateCalls)
	}

	// 🔴 窗口上界硬寫 24h，**不引用 escalateOverdueWindow**：期望值取自被測物的常數
	// 等於「它改成多少都會過」，那是自己跟自己對。24 小時的來源是 SQL 註解與 CodeMeta
	// 對 OVERDUE 的描述（「24 小時後自動升級 ADM」），三處必須同值。
	wantBefore := cNow.Add(-24 * time.Hour)
	if !f.repo.escalateBefore.Equal(wantBefore) {
		t.Fatalf("escalate_before = %s，want %s（OVERDUE 之後再等 24 小時）",
			f.repo.escalateBefore, wantBefore)
	}

	// I7：每一筆升級都要留痕，而且要落在 COMPLAINT 軸上。
	if len(f.audit.entries) != 2 {
		t.Fatalf("應留 2 列狀態歷程，實得 %d", len(f.audit.entries))
	}
	for _, e := range f.audit.entries {
		if e.SubjectType != subjectComplaint {
			t.Fatalf("留痕軸 = %q，want %q", e.SubjectType, subjectComplaint)
		}
		if e.FromStatus == nil || *e.FromStatus != codemeta.InvoiceComplaintStatusOverdue.Value {
			t.Fatalf("from = %v，want OVERDUE（不得寫成已更新後的值，那會變成自轉移）", e.FromStatus)
		}
		if e.ToStatus != codemeta.InvoiceComplaintStatusEscalatedToAdm.Value {
			t.Fatalf("to = %q，want ESCALATED_TO_ADM", e.ToStatus)
		}
		if e.ReasonCode == nil || *e.ReasonCode != codemeta.InvoiceComplaintEventTypeEscalated.Value {
			t.Fatalf("reason_code = %v，want ESCALATED（CodeMeta 對該值的描述逐字指名 K05）", e.ReasonCode)
		}
	}
}

// K06：賣家已回覆、買家異議期屆滿 → RESOLVED（逾期視為接受）。
//
// 🔴 這支之前不存在，而 SQL 註解逐字寫著「這是 K06 唯一的觸發者」——那個唯一的
// 觸發者零呼叫端 ⇒ 案件永遠停在 SELLER_RESPONDED。dev 上 7500462677014085632
// 的 buyer_review_due_at 是 2026-09-04，卡了 4 天。
func TestResolveExpiredBuyerReviews_ResolvesAndLeavesTrail(t *testing.T) {
	f := newComplaintUC(t)
	f.repo.reviewRows = []domain.Complaint{
		complaintRow(7201, cBuyerA, "RESOLVED", 6),
	}

	n, err := f.uc.ResolveExpiredBuyerReviews(context.Background(), cNow)
	if err != nil || n != 1 {
		t.Fatalf("ResolveExpiredBuyerReviews = (%d, %v)，want (1, nil)", n, err)
	}
	if f.repo.reviewCalls != 1 {
		t.Fatalf("應呼叫 1 次，實得 %d", f.repo.reviewCalls)
	}

	// 🔴 冷卻窗口硬寫 24h，**不引用 reopenCooldownWindow**（期望值取自被測物等於自己跟自己對）。
	// 24 小時的來源是 ResolveExpiredBuyerReviews 的 SQL 註解「reopen_allowed_after = now + 24h」。
	wantReopen := cNow.Add(24 * time.Hour)
	if !f.repo.reviewReopenAfter.Equal(wantReopen) {
		t.Fatalf("reopen_allowed_after = %s，want %s", f.repo.reviewReopenAfter, wantReopen)
	}

	// 🔴 結案理由必須是 NO_VIOLATION：填成 VIOLATION 會讓賣家平白進 90 天違規窗口
	// （CountConfirmedViolationOrders 只計 VIOLATION_CONFIRMED，但 resolution_code
	// 是 ADM 判讀的依據）。買家只是沒表態，不是賣家違規。
	if f.repo.reviewResolution != "NO_VIOLATION" {
		t.Fatalf("resolution_code = %q，want NO_VIOLATION", f.repo.reviewResolution)
	}

	if len(f.audit.entries) != 1 {
		t.Fatalf("應留 1 列狀態歷程，實得 %d", len(f.audit.entries))
	}
	e := f.audit.entries[0]
	if e.SubjectType != subjectComplaint {
		t.Fatalf("留痕軸 = %q，want %q", e.SubjectType, subjectComplaint)
	}
	if e.FromStatus == nil || *e.FromStatus != codemeta.InvoiceComplaintStatusSellerResponded.Value {
		t.Fatalf("from = %v，want SELLER_RESPONDED", e.FromStatus)
	}
	if e.ToStatus != codemeta.InvoiceComplaintStatusResolved.Value {
		t.Fatalf("to = %q，want RESOLVED", e.ToStatus)
	}
}

// 空集合不是錯誤，也不得留下任何痕跡。
func TestEscalateOverdueComplaints_EmptyIsNotAnError(t *testing.T) {
	f := newComplaintUC(t)
	n, err := f.uc.EscalateOverdueComplaints(context.Background(), cNow)
	if err != nil || n != 0 {
		t.Fatalf("空集合應回 (0, nil)，實得 (%d, %v)", n, err)
	}
	if len(f.audit.entries) != 0 {
		t.Fatalf("沒有列被升級時不得留痕，實得 %d", len(f.audit.entries))
	}
}

func TestSweepOverdueComplaints_EmitsOnePerRow(t *testing.T) {
	f := newComplaintUC(t)
	f.repo.overdueRows = []domain.Complaint{
		complaintRow(7001, cBuyerA, "OVERDUE", 2),
		complaintRow(7002, cBuyerA, "OVERDUE", 3),
	}

	n, err := f.uc.SweepOverdueComplaints(context.Background(), cNow)
	if err != nil {
		t.Fatalf("SweepOverdueComplaints: %v", err)
	}
	if n != 2 {
		t.Fatalf("回報標到 %d 筆，want 2", n)
	}
	if f.repo.overdueCalls != 1 {
		t.Fatalf("MarkOverdue 被呼叫 %d 次，want 1", f.repo.overdueCalls)
	}
	if len(f.outbox.inserted) != 2 {
		t.Fatalf("outbox 應有兩列（一筆一則），實得 %d", len(f.outbox.inserted))
	}
	for _, ev := range f.outbox.inserted {
		if ev.RoutingKey != pkgmq.RoutingKeyInvoiceComplaintOverdue ||
			ev.EventType != string(pkgmq.EventTypeInvoiceComplaintOverdue) {
			t.Fatalf("routing key／event_type = %q／%q", ev.RoutingKey, ev.EventType)
		}
		// 🔴 store_id 必須是 issuer_ref_id：模板的收件人是賣家，dispatcher 拿這個值
		// 去解析帳號。填成 issuer PK 的症狀是通知靜默不送，全程零錯誤零 DLQ。
		if !containsBytes(ev.Payload, `"store_id":"`+strconv.FormatInt(cStoreID, 10)+`"`) {
			t.Errorf("store_id 不是 issuer_ref_id：%s", ev.Payload)
		}
		if containsBytes(ev.Payload, `"store_id":"`+strconv.FormatInt(cIssuer, 10)+`"`) {
			t.Error("store_id 取到 issuer PK 了")
		}
	}
	// 冪等鍵的 phase 必須是 overdue：與同一件案子的 created／responded 共用一個鍵的話，
	// notification 端的唯一鍵會把三則去重成一則。
	if !containsBytes(f.outbox.inserted[0].Payload, `"notification_ref_id":"INVCPL:7001:overdue"`) {
		t.Errorf("冪等鍵的 phase 不是 overdue：%s", f.outbox.inserted[0].Payload)
	}
}

// 沒有逾期的案子 ⇒ 一列 outbox 都不該有。
// 沒有這一支，一個「每輪都發」的實作也會讓上面那支通過。
func TestSweepOverdueComplaints_NothingDueEmitsNothing(t *testing.T) {
	f := newComplaintUC(t)

	n, err := f.uc.SweepOverdueComplaints(context.Background(), cNow)
	if err != nil {
		t.Fatalf("SweepOverdueComplaints: %v", err)
	}
	if n != 0 || len(f.outbox.inserted) != 0 {
		t.Fatalf("沒有逾期案件卻標了 %d 筆、發了 %d 則", n, len(f.outbox.inserted))
	}
}

// 標記與事件在同一個交易內：事件寫入失敗 ⇒ 整批 rollback。
//
// 🔴 分開做的話，標記成功而事件失敗時該筆已經離開候選集合（status 不再是 OPEN）
// ⇒ 通知永遠不會補發，而且完全無聲。
func TestSweepOverdueComplaints_OutboxFailureRollsBack(t *testing.T) {
	f := newComplaintUC(t)
	f.repo.overdueRows = []domain.Complaint{complaintRow(7003, cBuyerA, "OVERDUE", 2)}
	f.outbox.err = errors.New("outbox down")

	if _, err := f.uc.SweepOverdueComplaints(context.Background(), cNow); err == nil {
		t.Fatal("事件寫不進去卻回成功 —— 那筆已經離開候選集合，通知永遠不會補發")
	}
}

// I7：申訴的每一次 status 轉移都要有一列 invoice_status_history（#3109）。
//
// 🔴 在 2026-09-04 之前申訴軸**只寫 invoice_complaint_events**（買家可見時間軸），
// 完全不寫稽核表。dev 實證：`subject_type='COMPLAINT'` 的歷程全表只有 1 列，
// 而那一列是 K09 違規計數專用的 —— K01–K13 的狀態轉移一列都沒有，
// 於是 A10 稽核與 AuditTrailDrawer 對申訴軸整段空白。
func TestComplaintTransition_WritesStatusHistory(t *testing.T) {
	row := complaintRow(4242, cBuyerA, codemeta.InvoiceComplaintStatusOpen.Value, 0)
	fx := newComplaintUC(t, row)

	if _, err := fx.uc.Withdraw(context.Background(), ComplaintActionInput{
		ComplaintID: 4242, BuyerID: cBuyerA, ExpectedVersion: row.Version,
	}); err != nil {
		t.Fatalf("Withdraw 失敗：%v", err)
	}
	if len(fx.audit.entries) != 1 {
		t.Fatalf("I7：狀態轉移要留痕，實得 %d 列", len(fx.audit.entries))
	}
	e := fx.audit.entries[0]
	if e.SubjectType != subjectComplaint {
		t.Fatalf("subject_type 應為 COMPLAINT，實得 %q", e.SubjectType)
	}
	if derefString(e.FromStatus) != codemeta.InvoiceComplaintStatusOpen.Value {
		t.Fatalf("from 應是轉移**前**的狀態，實得 %v", e.FromStatus)
	}
	// to 取自重讀後的實際值，不是 Go 端依 eventType 推導的期望值。
	if e.ToStatus == "" || e.ToStatus == derefString(e.FromStatus) {
		t.Fatalf("to 必須是實際落地的新狀態，實得 %q", e.ToStatus)
	}
	// 買家可見時間軸仍要寫——兩張表用途不同，不是二選一。
	if len(fx.repo.events[4242]) == 0 {
		t.Fatal("invoice_complaint_events 也要寫（買家時間軸）")
	}
}

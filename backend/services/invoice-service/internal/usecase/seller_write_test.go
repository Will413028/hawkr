package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

var sellerNow = time.Date(2026, 8, 30, 7, 0, 0, 0, time.UTC)

const sellerStoreID int64 = 2685914682

type sellerComplaintFixture struct {
	uc         *SellerComplaint
	complaints *fakeComplaintRepo
	outbox     *fakeOutbox
	audit      *fakeAudit
}

func newSellerComplaintFixture(status string, version int64) *sellerComplaintFixture {
	orderNo := "O2026083000001"
	complaints := &fakeComplaintRepo{stored: domain.Complaint{
		ID: 6001, DocumentID: 8001, IssuerID: 900, OrderNo: &orderNo,
		ComplainantBuyerID: 2623900520, Status: status, Version: version,
		SlaDueAt: sellerNow.Add(48 * time.Hour), CreatedAt: sellerNow.Add(-24 * time.Hour),
	}}
	outbox := &fakeOutbox{}
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{
		ID: 900, IssuerType: "SELLER", IssuerRefID: sellerStoreID,
	}}
	audit := &fakeAudit{}
	uc := NewSellerComplaint(directTx{}, complaints, issuers, outbox, &fakeBizDays{}, audit, zap.NewNop()).
		WithClock(func() time.Time { return sellerNow })
	return &sellerComplaintFixture{uc: uc, complaints: complaints, outbox: outbox, audit: audit}
}

func TestSellerComplaint_Respond_Resent(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)

	view, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 2685914682, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionResent.Value,
		Note:   "已重新寄送至買家信箱", ExpectedVersion: 2, IdempotencyKey: "idem-36",
	})
	if err != nil {
		t.Fatalf("Respond 失敗：%v", err)
	}
	if view.Complaint.Status != "SELLER_RESPONDED" {
		t.Fatalf("狀態應為 SELLER_RESPONDED，實得 %q", view.Complaint.Status)
	}
	if len(fx.complaints.responded) != 1 {
		t.Fatalf("應走 K02 一次，實得 %d", len(fx.complaints.responded))
	}
	want := sellerNow.Add(72 * time.Hour)
	if !fx.complaints.responded[0].DueAt.Equal(want) {
		t.Fatalf("buyer_review_due_at 應為 %v，實得 %v", want, fx.complaints.responded[0].DueAt)
	}
	if fx.complaints.responded[0].StoreID != 2685914682 {
		t.Fatalf("CAS 必須帶 store_id，實得 %d", fx.complaints.responded[0].StoreID)
	}
	if len(fx.complaints.events) != 1 {
		t.Fatalf("應追加一列時間軸，實得 %d", len(fx.complaints.events))
	}
	ev := fx.complaints.events[0]
	if ev.EventType != codemeta.InvoiceComplaintEventTypeSellerReply.Value ||
		ev.Action != codemeta.InvoiceComplaintActionResent.Value ||
		ev.ActorType != complaintActorSeller {
		t.Fatalf("時間軸內容不符：%+v", ev)
	}
}

// TestSellerComplaint_Respond_NeedPlatformHelp 驗 K03：它是合法選項，而且走另一條邊。
//
// 把它做成「不算回覆」的後果是賣家請求平台協助之後仍被 SLA 計為逾期。
func TestSellerComplaint_Respond_NeedPlatformHelp(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)

	view, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 2685914682, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionNeedPlatformHelp.Value,
		Note:   "買家堅稱未收到，需要平台協助查證", ExpectedVersion: 2,
	})
	if err != nil {
		t.Fatalf("Respond 失敗：%v", err)
	}
	if view.Complaint.Status != "ESCALATED_TO_ADM" {
		t.Fatalf("狀態應為 ESCALATED_TO_ADM，實得 %q", view.Complaint.Status)
	}
	if len(fx.complaints.escalated) != 1 || len(fx.complaints.responded) != 0 {
		t.Fatalf("NEED_PLATFORM_HELP 必須走 K03 而不是 K02：escalated=%d responded=%d",
			len(fx.complaints.escalated), len(fx.complaints.responded))
	}
	if fx.complaints.events[0].EventType != codemeta.InvoiceComplaintEventTypeEscalated.Value {
		t.Fatalf("event_type 應為 ESCALATED，實得 %q", fx.complaints.events[0].EventType)
	}
}

// TestSellerComplaint_Respond_NeedPlatformHelpRequiresNote 驗 K03 的必填說明。
func TestSellerComplaint_Respond_NeedPlatformHelpRequiresNote(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)

	_, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 2685914682, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionNeedPlatformHelp.Value, ExpectedVersion: 2,
	})
	if !errors.Is(err, ErrComplaintReasonRequired) {
		t.Fatalf("NEED_PLATFORM_HELP 缺說明應回 400，實得 %v", err)
	}
	if len(fx.complaints.escalated) != 0 {
		t.Fatalf("驗證失敗時不得下 CAS")
	}
}

// TestSellerComplaint_Respond_StateNotAllowed 驗 rows=0 ⇒ 9336，且不得留下半套。
func TestSellerComplaint_Respond_StateNotAllowed(t *testing.T) {
	fx := newSellerComplaintFixture("RESOLVED", 2)
	fx.complaints.casFails = true

	_, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 2685914682, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionReissued.Value, ExpectedVersion: 2,
	})
	assertBiz(t, err, bizcode.InvoiceComplaintActionNotAllowed)
	if len(fx.complaints.events) != 0 || len(fx.outbox.inserted) != 0 {
		t.Fatalf("CAS 不成立時不得寫時間軸或 outbox：events=%d outbox=%d",
			len(fx.complaints.events), len(fx.outbox.inserted))
	}
}

// TestSellerComplaint_Respond_UnknownAction 驗值域外的 action ⇒ 400 而不是 9336。
//
// 9336 的語意是「這個動作在目前狀態下不允許」，而值域外的字串連「是哪個動作」都答不出來。
func TestSellerComplaint_Respond_UnknownAction(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)

	for _, action := range []string{
		"", "SELLER_REPLY",
		codemeta.InvoiceComplaintActionDecidedResolved.Value,
		codemeta.InvoiceComplaintActionReminded.Value,
	} {
		_, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
			StoreID: 2685914682, ComplaintID: 6001, Action: action, ExpectedVersion: 2,
		})
		if !errors.Is(err, ErrComplaintActionInvalid) {
			t.Fatalf("action=%q 應回 400，實得 %v", action, err)
		}
	}
	if len(fx.complaints.responded) != 0 || len(fx.complaints.escalated) != 0 {
		t.Fatalf("值域外的 action 不得下任何 CAS")
	}
}

// TestSellerComplaint_Respond_TierA 驗越權與查無共用 9309。
func TestSellerComplaint_Respond_TierA(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)
	fx.complaints.sellerErr = notFoundErr(bizcode.InvoiceComplaintNotFound)

	_, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 999, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionResent.Value, ExpectedVersion: 2,
	})
	assertBiz(t, err, bizcode.InvoiceComplaintNotFound)
	if len(fx.complaints.responded) != 0 {
		t.Fatalf("越權時不得下 CAS")
	}
}

func TestSellerComplaint_Respond_VersionConflict(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)

	_, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 2685914682, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionResent.Value, ExpectedVersion: 1,
	})
	assertBiz(t, err, bizcode.SellerTaxProfileVersionConflict)
}

// TestSellerComplaint_Respond_EventPayloadHasNoFreeText 驗 payload 邊界。
//
// §6.3.3.3 逐字：P8／P9／P10 不放買家或賣家填寫的自由文字（可能含個資與情緒性內容）。
func TestSellerComplaint_Respond_EventPayloadHasNoFreeText(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)
	const secret = "客訴內容：買家住在台北市信義區XX路"

	if _, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 2685914682, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionResent.Value, Note: secret, ExpectedVersion: 2,
	}); err != nil {
		t.Fatalf("Respond 失敗：%v", err)
	}
	if len(fx.outbox.inserted) != 1 {
		t.Fatalf("應發一則 P9，實得 %d", len(fx.outbox.inserted))
	}
	ev := fx.outbox.inserted[0]
	if ev.RoutingKey != pkgmq.RoutingKeyInvoiceComplaintResponded {
		t.Fatalf("routing key 應為 invoice.complaint.responded，實得 %q", ev.RoutingKey)
	}
	if strings.Contains(string(ev.Payload), secret) {
		t.Fatalf("payload 不得含賣家填寫的自由文字：%s", ev.Payload)
	}
	var decoded struct {
		NotificationRefID string `json:"notification_ref_id"`
	}
	if err := json.Unmarshal(ev.Payload, &decoded); err != nil {
		t.Fatalf("payload 不是合法 JSON：%v", err)
	}
	if !strings.HasSuffix(decoded.NotificationRefID, ":responded") {
		t.Fatalf("notification_ref_id 應帶 :responded，實得 %q", decoded.NotificationRefID)
	}
}

type sellerJobFixture struct {
	uc     *SellerJob
	ops    *fakeOpRepo
	docs   *fakeDocRepo
	audit  *fakeAudit
	outbox *fakeOutbox
}

func newSellerJobFixture(status string, version int64) *sellerJobFixture {
	op := domain.Operation{ID: 7001, DocumentID: 8001, Status: status, Version: version}
	doc := docAt("ISSUE_FAILED", "")
	ops := &fakeOpRepo{stored: op}
	docs := &fakeDocRepo{stored: doc}
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{ID: 900, IssuerType: "SELLER", IssuerRefID: 2685914682}}
	audit := &fakeAudit{}
	outbox := &fakeOutbox{}
	jobs := NewAdminJob(directTx{}, ops, docs, issuers, audit, outbox, &fakeUnlockRepo{}, nil, nil, nil, IssuanceConfig{MaxAttempts: 13}, testIDs(), zap.NewNop()).
		WithClock(func() time.Time { return sellerNow })
	return &sellerJobFixture{uc: NewSellerJob(jobs, ops, docs), ops: ops, docs: docs,
		audit: audit, outbox: outbox}
}

// TestSellerJob_ReportExternalIssuance_ActorIsSellerOnBothAxes 守「兩條軸記同一個 actor」。
//
// 🔴 這支測試的存在理由是一次 dev 實機驗證（2026-09-07，op `7501682158306590721`，
// 以賣家身分打端點 62）：`invoice_status_history` 的 DOCUMENT 軸記 `SELLER`、
// 而 OPERATION 軸記 **`ADMIN`**。成因是 `resolveExternally` 相鄰兩行分別呼叫
// `appendHistory`（寫死 `jobActorAdmin`）與 `appendHistoryAs`（吃 `in.ActorType`）——
// 而 `appendHistoryAs` 的註解自己就寫著「端點 62 走 SELLER」，**知道要改的人只改了一半**。
//
// 既有的 TestSellerJob_ReportExternalIssuance 驗了狀態轉換卻沒驗 actor，所以全綠。
// 發票留痕保存 5 年供稅務稽核，把賣家自行開立回報記成 ADM 做的是實質稽核缺陷。
func TestSellerJob_ReportExternalIssuance_ActorIsSellerOnBothAxes(t *testing.T) {
	fx := newSellerJobFixture("MANUAL_PROCESS_ALLOWED", 4)

	if _, err := fx.uc.ReportExternalIssuance(context.Background(), ReportExternalIssuanceInput{
		StoreID: 2685914682, JobID: 7001, InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
		ExpectedVersion: 4, IdempotencyKey: "idem-62-actor",
	}); err != nil {
		t.Fatalf("ReportExternalIssuance 失敗：%v", err)
	}

	seen := map[string]string{}
	for _, e := range fx.audit.entries {
		if e.ToStatus != "RESOLVED_EXTERNALLY" {
			continue
		}
		seen[e.SubjectType] = e.ActorType
	}
	for _, axis := range []string{subjectOperation, subjectDocument} {
		got, ok := seen[axis]
		if !ok {
			t.Fatalf("🔴 %s 軸沒有留下 RESOLVED_EXTERNALLY 的狀態歷程", axis)
		}
		if got != providerActorSeller {
			t.Errorf("🔴 %s 軸的 actor_type ＝ %q，應為 %q —— "+
				"賣家打端點 62 卻被記成別人做的，稅務稽核時對不上人",
				axis, got, providerActorSeller)
		}
	}
}

func TestSellerJob_ReportExternalIssuance(t *testing.T) {
	fx := newSellerJobFixture("MANUAL_PROCESS_ALLOWED", 4)

	view, err := fx.uc.ReportExternalIssuance(context.Background(), ReportExternalIssuanceInput{
		StoreID: 2685914682, JobID: 7001, InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
		EvidenceAssetKeys: []string{"invoice/2026/08/abc.png"}, Note: "已於光貿後台自行開立",
		ExpectedVersion: 4, IdempotencyKey: "idem-62",
	})
	if err != nil {
		t.Fatalf("ReportExternalIssuance 失敗：%v", err)
	}
	if view.Operation.Status != "RESOLVED_EXTERNALLY" {
		t.Fatalf("狀態應為 RESOLVED_EXTERNALLY，實得 %q", view.Operation.Status)
	}
	if len(fx.ops.resolvedExternally) != 1 || len(fx.docs.resolvedExternally) != 1 {
		t.Fatalf("operation／document 應各轉一次，實得 %d／%d",
			len(fx.ops.resolvedExternally), len(fx.docs.resolvedExternally))
	}
	if fx.docs.resolvedExternally[0].ExternalInvoiceNo != "AB12345678" {
		t.Fatalf("號碼應寫進 external_invoice_no")
	}
	if len(fx.outbox.inserted) != 1 {
		t.Fatalf("應發一則 P12，實得 %d", len(fx.outbox.inserted))
	}
	if !strings.Contains(string(fx.outbox.inserted[0].Payload), `"external_invoice_no"`) {
		t.Fatalf("P12 的 wire 欄名必須是 external_invoice_no：%s", fx.outbox.inserted[0].Payload)
	}
	if len(fx.audit.admin) != 0 {
		t.Fatalf("賣家路徑不得寫 invoice_admin_audit_logs，實得 %d 列", len(fx.audit.admin))
	}
	var sawSeller bool
	for _, e := range fx.audit.entries {
		if e.ActorType == "SELLER" {
			sawSeller = true
		}
	}
	if !sawSeller {
		t.Fatalf("狀態歷程的 actor_type 應含 SELLER，實得 %+v", fx.audit.entries)
	}
	var sawEvidence bool
	for _, e := range fx.audit.entries {
		if e.ReasonDetail != nil && strings.Contains(*e.ReasonDetail, "invoice/2026/08/abc.png") {
			sawEvidence = true
		}
	}
	if !sawEvidence {
		t.Fatalf("evidence key 未落在任何持久化欄位")
	}
}

// TestSellerJob_ReportExternalIssuance_NotAllowed 驗 9349：唯一來源狀態是 MANUAL_PROCESS_ALLOWED。
//
// 不在那一格就回碼，**不是**靜默成功：賣家按了沒反應而 operation 永遠停在非終態，
// 買家端永遠顯示「處理中」，法定期限照樣倒數。
func TestSellerJob_ReportExternalIssuance_NotAllowed(t *testing.T) {
	fx := newSellerJobFixture("FAILED_PERMANENT", 4)

	_, err := fx.uc.ReportExternalIssuance(context.Background(), ReportExternalIssuanceInput{
		StoreID: 2685914682, JobID: 7001, InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
		ExpectedVersion: 4,
	})
	assertBiz(t, err, bizcode.InvoiceExternalIssuanceNotAllowed)
	if len(fx.ops.resolvedExternally) != 0 || len(fx.outbox.inserted) != 0 {
		t.Fatalf("狀態不允許時不得有任何寫入")
	}
}

// TestSellerJob_ReportExternalIssuance_TierA 驗越權與查無共用 9340。
func TestSellerJob_ReportExternalIssuance_TierA(t *testing.T) {
	fx := newSellerJobFixture("MANUAL_PROCESS_ALLOWED", 4)
	fx.ops.sellerViewErr = notFoundErr(bizcode.InvoiceJobNotFound)

	_, err := fx.uc.ReportExternalIssuance(context.Background(), ReportExternalIssuanceInput{
		StoreID: 999, JobID: 7001, InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
		ExpectedVersion: 4,
	})
	assertBiz(t, err, bizcode.InvoiceJobNotFound)
	if len(fx.ops.resolvedExternally) != 0 {
		t.Fatalf("越權時不得下 CAS")
	}
}

func TestSellerJob_ReportExternalIssuance_VersionConflict(t *testing.T) {
	fx := newSellerJobFixture("MANUAL_PROCESS_ALLOWED", 4)

	_, err := fx.uc.ReportExternalIssuance(context.Background(), ReportExternalIssuanceInput{
		StoreID: 2685914682, JobID: 7001, InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
		ExpectedVersion: 3,
	})
	assertBiz(t, err, bizcode.InvoiceJobVersionConflict)
	if len(fx.ops.resolvedExternally) != 0 {
		t.Fatalf("版本不符時不得下 CAS")
	}
}

// TestSellerJob_ReportExternalIssuance_BadInput 驗四種契約違規全部回 400（不是 9349）。
func TestSellerJob_ReportExternalIssuance_BadInput(t *testing.T) {
	cases := []struct {
		name string
		in   ReportExternalIssuanceInput
		want error
	}{
		{"號碼格式", ReportExternalIssuanceInput{InvoiceNo: "AB1234567", InvoiceDate: "2026-08-25"}, ErrExternalInvoiceNoInvalid},
		{"日期格式", ReportExternalIssuanceInput{InvoiceNo: "AB12345678", InvoiceDate: "2026/08/25"}, ErrExternalInvoiceDateInvalid},
		{"佐證超量", ReportExternalIssuanceInput{InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
			EvidenceAssetKeys: []string{"a", "b", "c", "d", "e", "f"}}, ErrExternalEvidenceTooMany},
		{"佐證是 URL", ReportExternalIssuanceInput{InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
			EvidenceAssetKeys: []string{"https://evil.example/x.png"}}, ErrExternalEvidenceInvalid},
		{"佐證有跳脫", ReportExternalIssuanceInput{InvoiceNo: "AB12345678", InvoiceDate: "2026-08-25",
			EvidenceAssetKeys: []string{"../../etc/passwd"}}, ErrExternalEvidenceInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newSellerJobFixture("MANUAL_PROCESS_ALLOWED", 4)
			in := tc.in
			in.StoreID, in.JobID, in.ExpectedVersion = 2685914682, 7001, 4
			if _, err := fx.uc.ReportExternalIssuance(context.Background(), in); !errors.Is(err, tc.want) {
				t.Fatalf("應回 %v，實得 %v", tc.want, err)
			}
			if len(fx.ops.sellerViews) != 0 {
				t.Fatalf("格式驗證失敗時不該開交易讀取")
			}
		})
	}
}

// TestSellerJob_ReportExternalIssuance_DateBeforePayment 驗發票日不得早於付款日。
func TestSellerJob_ReportExternalIssuance_DateBeforePayment(t *testing.T) {
	fx := newSellerJobFixture("MANUAL_PROCESS_ALLOWED", 4)

	_, err := fx.uc.ReportExternalIssuance(context.Background(), ReportExternalIssuanceInput{
		StoreID: 2685914682, JobID: 7001, InvoiceNo: "AB12345678", InvoiceDate: "2026-08-19",
		ExpectedVersion: 4,
	})
	if !errors.Is(err, ErrExternalInvoiceDateTooEarly) {
		t.Fatalf("應回 400（發票日早於付款日），實得 %v", err)
	}
	if len(fx.ops.resolvedExternally) != 0 {
		t.Fatalf("驗證失敗時不得下 CAS")
	}
}

// I7：賣家回覆也要留痕（#3109）。complaint_events 是時間軸不是稽核表。
func TestSellerComplaint_Respond_WritesStatusHistory(t *testing.T) {
	fx := newSellerComplaintFixture("OPEN", 2)

	if _, err := fx.uc.Respond(context.Background(), RespondComplaintInput{
		StoreID: 2685914682, ComplaintID: 6001,
		Action: codemeta.InvoiceComplaintActionResent.Value,
		Note:   "已重新寄送", ExpectedVersion: 2, IdempotencyKey: "idem-i7",
	}); err != nil {
		t.Fatalf("Respond 失敗：%v", err)
	}
	if len(fx.audit.entries) != 1 {
		t.Fatalf("I7：狀態轉移要留痕，實得 %d 列", len(fx.audit.entries))
	}
	e := fx.audit.entries[0]
	if e.SubjectType != subjectComplaint ||
		derefString(e.FromStatus) != "OPEN" || e.ToStatus != "SELLER_RESPONDED" {
		t.Fatalf("history 內容錯：%s %v -> %s", e.SubjectType, e.FromStatus, e.ToStatus)
	}
}

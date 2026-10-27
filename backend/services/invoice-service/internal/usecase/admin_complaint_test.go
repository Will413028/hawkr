package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

type complaintAdminFixture struct {
	uc         *AdminComplaint
	complaints *fakeComplaintRepo
	audit      *fakeAudit
	issuers    *fakeIssuerRepo
}

func newComplaintAdminFixture(status string, version int64, violations int64) *complaintAdminFixture {
	order := "O2026082000001"
	complaints := &fakeComplaintRepo{
		stored: domain.Complaint{
			ID: 6001, DocumentID: 8001, IssuerID: 900, OrderNo: &order,
			ComplainantBuyerID: 2623900520, ReasonCode: "NOT_RECEIVED",
			Status: status, Version: version, SlaDueAt: adminNow.Add(48 * time.Hour),
		},
		violationCount: violations,
	}
	issuers := &fakeIssuerRepo{issuer: domain.Issuer{ID: 900, IssuerType: "SELLER", IssuerRefID: 2685914682}}
	audit := &fakeAudit{}
	uc := NewAdminComplaint(directTx{}, complaints, issuers, audit, &fakeBizDays{}, zap.NewNop()).
		WithClock(func() time.Time { return adminNow })
	return &complaintAdminFixture{uc: uc, complaints: complaints, audit: audit, issuers: issuers}
}

func adminComplaintInput(action string) ComplaintActionInputAdmin {
	return ComplaintActionInputAdmin{
		ActorAccountID: 42, ComplaintID: 6001, Action: action,
		Reason: "已確認賣家補開", ExpectedVersion: 4, IdempotencyKey: "idem-a",
	}
}

func TestAdminComplaint_DecideResolved(t *testing.T) {
	fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 0)
	got, err := fx.uc.Act(context.Background(), adminComplaintInput(complaintActionDecideResolved))
	if err != nil {
		t.Fatalf("Act 失敗：%v", err)
	}
	if got.Status != codemeta.InvoiceComplaintStatusResolved.Value {
		t.Fatalf("應轉 RESOLVED，實得 %q", got.Status)
	}
	if len(fx.complaints.decided) != 1 {
		t.Fatalf("應呼叫 DecideEscalated 一次，實得 %d", len(fx.complaints.decided))
	}
	d := fx.complaints.decided[0]
	// 🔴 reopen_allowed_after 不寫，買家永遠重開不了——而 K13 那條邊正是
	// 「RESOLVED 不是終態」的全部意義。
	if d.ReopenAllowedAfter == nil || !d.ReopenAllowedAfter.Equal(adminNow.Add(24*time.Hour)) {
		t.Fatalf("K08 必須寫 reopen_allowed_after = now + 24h，實得 %v", d.ReopenAllowedAfter)
	}
	if d.ExpectedVersion != 4 {
		t.Fatalf("CAS 必須帶 expectedVersion=4，實得 %d", d.ExpectedVersion)
	}
	// 時間軸：action = DECIDED_RESOLVED，actor_type = ADMIN。
	if len(fx.complaints.events) != 1 ||
		fx.complaints.events[0].Action != codemeta.InvoiceComplaintActionDecidedResolved.Value ||
		fx.complaints.events[0].ActorType != complaintActorAdmin {
		t.Fatalf("時間軸事件不正確：%+v", fx.complaints.events)
	}
	// DECIDE_RESOLVED 不計入違規、不給建議。
	if got.Recommendation != nil {
		t.Fatal("判定已解決不得回停權建議")
	}
}

func TestAdminComplaint_DecideViolation(t *testing.T) {
	fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 3)
	got, err := fx.uc.Act(context.Background(), adminComplaintInput(complaintActionDecideViolation))
	if err != nil {
		t.Fatalf("Act 失敗：%v", err)
	}
	if got.Status != codemeta.InvoiceComplaintStatusViolationConfirmed.Value {
		t.Fatalf("應轉 VIOLATION_CONFIRMED，實得 %q", got.Status)
	}
	// VIOLATION_CONFIRMED 是終態、沒有重開邊 ⇒ reopen_allowed_after 必須是 nil。
	if fx.complaints.decided[0].ReopenAllowedAfter != nil {
		t.Fatal("K09 不得寫 reopen_allowed_after")
	}
	r := got.Recommendation
	if r == nil {
		t.Fatal("判定違規必須回停權建議")
	}
	// 🔴 storeId 走 invoice_issuers.issuer_ref_id，不是 issuer_id：治理端點的 sellerId
	// 是 store surrogate，送 issuer 的 PK 過去會查不到或查到別人。
	if r.StoreID != 2685914682 {
		t.Fatalf("storeId 應為 issuer_ref_id，實得 %d", r.StoreID)
	}
	if !r.ThresholdReached || !r.ShouldSuspend || r.ViolationCountIn90Days != 3 {
		t.Fatalf("90 天 3 次應達門檻，實得 %+v", r)
	}
	if r.SuggestedSuspendDays != 14 {
		t.Fatalf("建議停權天數應為 14，實得 %d", r.SuggestedSuspendDays)
	}
	// 🔴 complaintId 必須寫進建議理由：seller_violations 的兩個 source FK 都不是申訴案件，
	// 從違規紀錄回查「這是哪一件申訴造成的」的唯一線索就是這段文字。
	if !strings.Contains(r.SuggestedReason, "6001") {
		t.Fatalf("建議理由必須帶 complaintId，實得 %q", r.SuggestedReason)
	}
	// 滑動窗即時算 ⇒ 當下的計數與門檻必須留痕，否則事後重建不出「當時為什麼建議停權」。
	var found bool
	for _, e := range fx.audit.entries {
		if e.ReasonDetail != nil && strings.Contains(*e.ReasonDetail, "90d violations=3") {
			found = true
		}
	}
	if !found {
		t.Fatal("狀態歷程必須帶當下的窗口計數與門檻")
	}
}

// 🔴 發票模組只提建議、不執行處分（I-57）：不蓋 suspended_at、不發停權事件、不排恢復 job。
func TestAdminComplaint_RecommendSuspension_OnlyAdvises(t *testing.T) {
	fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 1)
	in := adminComplaintInput(complaintActionRecommendSuspension)
	in.Confirmed = true

	got, err := fx.uc.Act(context.Background(), in)
	if err != nil {
		t.Fatalf("Act 失敗：%v", err)
	}
	if got.Status != "ESCALATED_TO_ADM" {
		t.Fatalf("建議停權不得改變案件狀態，實得 %q", got.Status)
	}
	if len(fx.complaints.decided)+len(fx.complaints.returned) != 0 {
		t.Fatal("建議停權不得做任何狀態 CAS")
	}
	// RECOMMEND_SUSPENSION 刻意不寫時間軸（InvoiceComplaintAction 因此是 6 值不是 8 值）。
	if len(fx.complaints.events) != 0 {
		t.Fatalf("RECOMMEND_SUSPENSION 不得寫時間軸，實得 %d 列", len(fx.complaints.events))
	}
	if len(fx.audit.admin) != 1 {
		t.Fatalf("必須寫一列稽核，實得 %d", len(fx.audit.admin))
	}
	if got.Recommendation == nil || !got.Recommendation.ShouldSuspend {
		t.Fatal("必須回建議物件")
	}
}

func TestAdminComplaint_RecommendSuspension_NeedsConfirm(t *testing.T) {
	fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 5)
	in := adminComplaintInput(complaintActionRecommendSuspension)
	in.Confirmed = false
	if _, err := fx.uc.Act(context.Background(), in); !errors.Is(err, ErrComplaintConfirmRequired) {
		t.Fatalf("期望 ErrComplaintConfirmRequired，實得 %v", err)
	}
	if len(fx.audit.admin) != 0 {
		t.Fatal("缺二次確認不得留下稽核")
	}
}

func TestAdminComplaint_RequestRework(t *testing.T) {
	fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 0)
	got, err := fx.uc.Act(context.Background(), adminComplaintInput(complaintActionRequestRework))
	if err != nil {
		t.Fatalf("Act 失敗：%v", err)
	}
	if got.Status != codemeta.InvoiceComplaintStatusOpen.Value {
		t.Fatalf("K10 應轉回 OPEN，實得 %q", got.Status)
	}
	if len(fx.complaints.returned) != 1 || fx.complaints.returned[0].Version != 4 {
		t.Fatalf("應呼叫 ReturnToSeller 並帶 expectedVersion，實得 %+v", fx.complaints.returned)
	}
	// REQUEST_REWORK 刻意不寫時間軸（副作用只有重置 SLA）。
	if len(fx.complaints.events) != 0 {
		t.Fatalf("REQUEST_REWORK 不得寫時間軸，實得 %d 列", len(fx.complaints.events))
	}
}

func TestAdminComplaint_RemindSeller_NoStateChange(t *testing.T) {
	fx := newComplaintAdminFixture("OPEN", 4, 0)
	got, err := fx.uc.Act(context.Background(), adminComplaintInput(complaintActionRemindSeller))
	if err != nil {
		t.Fatalf("Act 失敗：%v", err)
	}
	if got.Status != "OPEN" {
		t.Fatalf("提醒不得改變狀態，實得 %q", got.Status)
	}
	if len(fx.complaints.decided)+len(fx.complaints.returned) != 0 {
		t.Fatal("提醒不得做任何狀態 CAS")
	}
	// 不改狀態 ⇒ 不寫狀態歷程（寫了會讓 A08 的歷程出現一列 from == to 的雜訊）。
	if len(fx.audit.entries) != 0 {
		t.Fatalf("不改狀態不得寫狀態歷程，實得 %d 列", len(fx.audit.entries))
	}
	if len(fx.complaints.events) != 1 ||
		fx.complaints.events[0].Action != codemeta.InvoiceComplaintActionReminded.Value {
		t.Fatalf("提醒必須寫一列 REMINDED，實得 %+v", fx.complaints.events)
	}
}

func TestAdminComplaint_Guards(t *testing.T) {
	t.Run("未知 action", func(t *testing.T) {
		fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 0)
		in := adminComplaintInput("SOMETHING")
		if _, err := fx.uc.Act(context.Background(), in); !errors.Is(err, ErrComplaintActionUnknown) {
			t.Fatalf("期望 ErrComplaintActionUnknown，實得 %v", err)
		}
	})
	t.Run("缺理由", func(t *testing.T) {
		fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 0)
		in := adminComplaintInput(complaintActionDecideResolved)
		in.Reason = "  "
		if _, err := fx.uc.Act(context.Background(), in); !errors.Is(err, ErrComplaintAdminReasonRequired) {
			t.Fatalf("期望 ErrComplaintAdminReasonRequired，實得 %v", err)
		}
	})
	t.Run("版本不符回 9322", func(t *testing.T) {
		fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 5, 0)
		if _, err := fx.uc.Act(context.Background(), adminComplaintInput(complaintActionDecideResolved)); bizCodeOf(t, err) != bizcode.SellerTaxProfileVersionConflict {
			t.Fatalf("期望 9322，實得 %d", bizCodeOf(t, err))
		}
		if len(fx.complaints.decided) != 0 {
			t.Fatal("版本不符不得進 CAS")
		}
	})
	t.Run("CAS rows=0 回 9336", func(t *testing.T) {
		fx := newComplaintAdminFixture("ESCALATED_TO_ADM", 4, 0)
		fx.complaints.casFails = true
		err := func() error {
			_, e := fx.uc.Act(context.Background(), adminComplaintInput(complaintActionDecideResolved))
			return e
		}()
		if bizCodeOf(t, err) != bizcode.InvoiceComplaintActionNotAllowed {
			t.Fatalf("rows=0 必須回 9336，實得 %d", bizCodeOf(t, err))
		}
		// 🔴 CAS 失敗後不得寫時間軸／稽核：那正是「守衛不成立卻寫了一半」的形狀。
		if len(fx.complaints.events) != 0 || len(fx.audit.admin) != 0 {
			t.Fatal("CAS 失敗後不得留下任何寫入")
		}
	})
}

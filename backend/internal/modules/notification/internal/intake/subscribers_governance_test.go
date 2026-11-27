package intake

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

var pureHelpers = []string{
	"measureCreatedIntent", "requestDecidedIntent", "measureRevokedNotified", "measureEndedNotified", "moderationReportReceivedIntent", "socialAuthorBuyer", "messageAuthorBuyer", "string",
}

// 治理訂閱者同樣在來源的 transaction 內執行：只准呼叫 Accept（每個函式語法上剛好一處）與純函式，不讀別的模組、不碰資料庫。
func TestGovernanceSubscribersOnlyAccept(t *testing.T) {
	fns, imports := funcs(t, "subscribers_governance.go")
	if len(fns) < 27 {
		t.Fatalf("subscribers_governance.go 只有 %d 個訂閱者，預期 27 個", len(fns))
	}
	for _, fn := range fns {
		accepts := 0
		for _, c := range calls(fn, imports) {
			switch {
			case c == ".Accept":
				accepts++
			case !slices.Contains(pureHelpers, c):
				t.Errorf("%s 呼叫了 %s；治理訂閱者只准呼叫 Accept 與純函式 %v", fn.Name.Name, c, pureHelpers)
			}
		}
		if accepts != 1 {
			t.Errorf("%s 有 %d 處 Accept，預期剛好 1 處", fn.Name.Name, accepts)
		}
	}
}

func TestGovernanceIntentHelpersArePure(t *testing.T) {
	allowed := []string{"len", "string", "fmt.Errorf"}
	fns, imports := funcs(t, "governance_intents.go")
	for _, fn := range fns {
		for _, c := range calls(fn, imports) {
			if !slices.Contains(allowed, c) {
				t.Errorf("%s 呼叫了 %s，純函式只准 %v", fn.Name.Name, c, allowed)
			}
		}
	}
}

// 註冊表的字串值與擁有模組的列舉值一致（註冊表不 import 來源模組，這裡是兩邊唯一的對照）。
func TestRegistryValuesMatchTheOwnersEnums(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{registry.MeasureKindWarning, string(seller.MeasureWarning)},
		{registry.MeasureKindTermination, string(seller.MeasureTermination)},
		{registry.MeasureKindSuspension, string(seller.MeasureSuspension)},
		{registry.MeasureKindTakedown, string(seller.MeasureContentTakedown)},
		{registry.RequestKindAppeal, string(seller.RequestKindAppeal)},
		{registry.RequestKindRemediation, string(seller.RequestKindRemediation)},
		{registry.RequestOutcomeAccepted, string(seller.RequestAccepted)},
		{registry.RequestOutcomeRejected, string(seller.RequestRejected)},
		{registry.InquiryKindRecords, string(seller.InquiryKindRecords)},
		{registry.InquiryKindExplanation, string(seller.InquiryKindExplanation)},
		{registry.DeadlineTierSevenDays, string(payment.DeadlineSevenDays)},
		{registry.DeadlineTierOneDay, string(payment.DeadlineOneDay)},
		{registry.ReportOutcomeActionTaken, string(seller.OutcomeActionTaken)},
		{registry.ReportOutcomeNoViolation, string(seller.OutcomeNoViolation)},
		{registry.ReportOutcomeInsufficient, string(seller.OutcomeInsufficient)},
		{registry.ReportOutcomeDuplicate, string(seller.OutcomeDuplicate)},
		{registry.ReportOutcomeAbusive, string(seller.OutcomeAbusive)},
		{registry.ModerationOutcomeViolation, string(moderation.OutcomeViolation)},
		{registry.ModerationOutcomeNoViolation, string(moderation.OutcomeNoViolation)},
	} {
		if c.got != c.want {
			t.Errorf("註冊表的值 %q 與擁有模組的值 %q 不同", c.got, c.want)
		}
	}
}

func render(t *testing.T, typ string, params any, l uitext.Locale) error {
	t.Helper()
	e, ok := registry.Lookup(typ)
	if !ok {
		t.Fatalf("沒有型別 %s", typ)
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Render(l, raw)
	return err
}

// 擁有模組的每個列舉值都有文案：措施種類、請求種類、詢問種類、檢舉結果、代行動作；文案缺任何一個就紅，不在執行期才發現。
func TestEveryOwnerEnumValueRenders(t *testing.T) {
	at := time.Date(2026, 11, 23, 4, 30, 0, 0, time.UTC)
	id := uuid.MustParse("0190a000-0000-7000-8000-000000000002")
	for _, l := range []uitext.Locale{uitext.ZhTW, uitext.EN} {
		for _, k := range []seller.MeasureKind{seller.MeasureWarning, seller.MeasureTermination} {
			if err := render(t, "store_measure_issued", registry.StoreMeasureIssuedParams{StoreID: 1, MeasureID: id, Kind: string(k), OccurredAt: at}, l); err != nil {
				t.Errorf("措施 %s/%s：%v", k, l, err)
			}
		}
		for _, k := range seller.RequestKindValues() {
			for _, o := range []seller.RequestState{seller.RequestAccepted, seller.RequestRejected} {
				p := registry.StoreMeasureRequestDecidedParams{StoreID: 1, MeasureID: id, RequestID: id, RequestKind: string(k), Outcome: string(o), OccurredAt: at}
				if err := render(t, "store_measure_request_decided", p, l); err != nil {
					t.Errorf("請求 %s/%s/%s：%v", k, o, l, err)
				}
			}
		}
		for _, k := range seller.InquiryKindValues() {
			if err := render(t, "store_case_inquiry", registry.StoreCaseInquiryParams{StoreID: 1, InquiryID: id, Kind: string(k), DueAt: at, OccurredAt: at}, l); err != nil {
				t.Errorf("詢問 %s/%s：%v", k, l, err)
			}
		}
		for _, o := range seller.OutcomeValues() {
			if err := render(t, "report_resolved", registry.ReportResolvedParams{ReportSource: registry.ReportSourceSeller, ReportID: id, BuyerID: 1, Outcome: string(o), OccurredAt: at}, l); err != nil {
				t.Errorf("檢舉結果 %s/%s：%v", o, l, err)
			}
		}
		for _, a := range delegation.AuditActions() {
			p := registry.DelegationOperationCompletedParams{AuditLogID: id, TargetStoreID: 1, TargetSellerAccountID: 2, Capability: "products.write", Action: string(a), OccurredAt: at}
			if err := render(t, "delegation_operation_completed", p, l); err != nil {
				t.Errorf("代行動作 %s/%s：%v", a, l, err)
			}
		}
	}
}

func TestMeasureCreatedIntentPicksTheTypeByKind(t *testing.T) {
	id := uuid.MustParse("0190a000-0000-7000-8000-000000000002")
	e := seller.MeasureCreated{MeasureID: id, StoreID: 11, Kind: seller.MeasureWarning}
	if name, _, ok := measureCreatedIntent(e); !ok || name != "store_measure_issued" {
		t.Errorf("警告：%q %v", name, ok)
	}
	e.Kind = seller.MeasureTermination
	if name, _, ok := measureCreatedIntent(e); !ok || name != "store_measure_issued" {
		t.Errorf("終止：%q %v", name, ok)
	}
	e.Kind = seller.MeasureSuspension
	if _, _, ok := measureCreatedIntent(e); ok {
		t.Error("停權措施由 StoreSuspended 通知，不另發")
	}
	e.Kind = seller.MeasureContentTakedown
	e.Contents = make([]seller.ContentRef, 3)
	name, params, ok := measureCreatedIntent(e)
	if p, isTakedown := params.(registry.StoreContentTakenDownParams); !ok || name != "store_content_taken_down" || !isTakedown || p.ItemCount != 3 {
		t.Errorf("內容下架：%q %+v %v", name, params, ok)
	}
}

// 接受與駁回的裁定都是 store_measure_request_decided（冪等鍵是請求 ID，參數帶裁定，D3）；其他裁定是錯誤。
func TestRequestDecidedIntentRegistersEveryDecisionAsRequestDecided(t *testing.T) {
	id := uuid.MustParse("0190a000-0000-7000-8000-000000000002")
	req := uuid.MustParse("0190a000-0000-7000-8000-000000000003")
	for _, kind := range seller.RequestKindValues() {
		for _, outcome := range []seller.RequestState{seller.RequestAccepted, seller.RequestRejected} {
			name, params, err := requestDecidedIntent(seller.MeasureRequestDecided{MeasureID: id, RequestID: req, Kind: kind, Outcome: outcome, StoreID: 11})
			p, ok := params.(registry.StoreMeasureRequestDecidedParams)
			if err != nil || name != "store_measure_request_decided" || !ok || p.Outcome != string(outcome) || p.RequestKind != string(kind) {
				t.Errorf("%s/%s：%q %+v %v", kind, outcome, name, params, err)
				continue
			}
			e, _ := registry.Lookup(name)
			raw, _ := json.Marshal(params)
			if key, err := e.IdempotencyKey(raw); err != nil || key != name+":"+req.String() {
				t.Errorf("%s/%s 的冪等鍵 %q（%v）", kind, outcome, key, err)
			}
		}
	}
	if _, _, err := requestDecidedIntent(seller.MeasureRequestDecided{MeasureID: id, Kind: seller.RequestKindAppeal, Outcome: seller.RequestClosed}); err == nil {
		t.Error("CLOSED 不是裁定結果，要回錯誤而不是靜默略過")
	}
}

// 接受申覆的撤銷、接受解除請求的結束由裁定通知，不另發；其他撤銷原因與管理員直接結束照常通知。
func TestMeasureRevokedAndEndedSkipTheAcceptedRequestDisposition(t *testing.T) {
	for _, r := range seller.RevokeReasonValues() {
		if got, want := measureRevokedNotified(seller.MeasureRevoked{Reason: r}), r != seller.RevokeAppealAccepted; got != want {
			t.Errorf("撤銷原因 %s：notified=%v", r, got)
		}
	}
	if measureEndedNotified(seller.MeasureEnded{RequestID: uuid.MustParse("0190a000-0000-7000-8000-000000000003")}) {
		t.Error("接受解除請求的結束不另發")
	}
	if !measureEndedNotified(seller.MeasureEnded{}) {
		t.Error("管理員直接結束要通知")
	}
}

func TestModerationReportReceivedIntentSplitsBuyerAndStoreReporters(t *testing.T) {
	id := uuid.MustParse("0190a000-0000-7000-8000-000000000004")
	if name, _, err := moderationReportReceivedIntent(moderation.ReportReceived{ReportID: id, ReporterBuyerID: 7}); err != nil || name != "report_received" {
		t.Errorf("買家：%q %v", name, err)
	}
	if name, _, err := moderationReportReceivedIntent(moderation.ReportReceived{ReportID: id, ReporterStoreID: 7}); err != nil || name != "store_report_received" {
		t.Errorf("店家：%q %v", name, err)
	}
	for _, e := range []moderation.ReportReceived{{ReportID: id}, {ReportID: id, ReporterBuyerID: 7, ReporterStoreID: 7}} {
		if _, _, err := moderationReportReceivedIntent(e); err == nil {
			t.Errorf("檢舉人不是恰好一個要回錯誤：%+v", e)
		}
	}
}

// 作者通知只給買家作者：店家作者（社群留言、聊天訊息）不通知，店家內容走 seller 的措施。
func TestAuthorNoticesAreOnlyForBuyerAuthors(t *testing.T) {
	if id, ok := socialAuthorBuyer(social.Actor{Kind: social.ActorBuyer, ID: 7}); !ok || id != 7 {
		t.Errorf("買家作者：%d %v", id, ok)
	}
	if _, ok := socialAuthorBuyer(social.Actor{Kind: social.ActorStore, ID: 7}); ok {
		t.Error("店家作者不通知")
	}
	if id, ok := messageAuthorBuyer(im.PartyBuyer, 7); !ok || id != 7 {
		t.Errorf("買家寄的訊息：%d %v", id, ok)
	}
	if _, ok := messageAuthorBuyer(im.PartyStore, 7); ok {
		t.Error("店家寄的訊息不通知店家作者")
	}
}

//go:build integration

package seller_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 審閱政策（step10e-3d-design.md E10、§3.1）：每種待辦一支 seller.review_<種類>(at)，成員、開始與期限只寫在 SQL。
// 這個檔的測試：逐種的表格（成員集合、開始、期限、邊界 ±1µs）、成員 ⇔ Go 判定式的資料集、依 ID 批次讀的計畫。

// reviewKinds 是函式的種類與 SQL 函式名（設計 §1.3 的 11 種加 step10e-5a 的 PAYUNI_BIND_RETRY）。
var reviewKinds = []struct{ kind, fn string }{
	{"REPORT_TRIAGE", "review_report_triage"},
	{"CASE_DECISION", "review_case_decision"},
	{"CASE_INQUIRY_OVERDUE", "review_case_inquiry_overdue"},
	{"CASE_FDA_REFERRAL", "review_case_fda_referral"},
	{"AUTHORITY_ORDER", "review_authority_order"},
	{"AUTHORITY_TARGET_NOTICE", "review_authority_target_notice"},
	{"MEASURE_APPEAL", "review_measure_appeal"},
	{"MEASURE_LIFT_REQUEST", "review_measure_lift_request"},
	{"MEASURE_REVIEW", "review_measure_review"},
	{"FOOD_REGISTRATION", "review_food_registration"},
	{"KYC_APPLICATION", "review_kyc_application"},
	{"PAYUNI_BIND_RETRY", "review_payuni_bind_retry"},
}

// reviewRow 是函式的一列（seller.review_item 的欄位）。
type reviewRow struct {
	Kind, ItemID                           string
	ReportID, CaseID, InquiryID, OrderID   *uuid.UUID
	Position                               *int16
	AccountID                              *int64
	MeasureID, RequestID, FoodRegID, KYCID *uuid.UUID
	StoreID                                *int64
	StoreName                              *string
	Start, Due                             *time.Time
}

// reviewOf 以 at 呼叫一支審閱政策函式，依 item_id 排序。
func (f *fixture) reviewOf(t *testing.T, fn string, at time.Time) []reviewRow {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT kind, item_id, report_id, case_id, inquiry_id, authority_order_id, target_position, account_id,
		measure_id, request_id, food_registration_id, kyc_application_id, store_id, store_name, start_at, due_at
		FROM seller.`+fn+`($1::timestamptz) ORDER BY item_id COLLATE "C"`, at)
	must(t, err)
	defer rows.Close()
	var out []reviewRow
	for rows.Next() {
		var r reviewRow
		must(t, rows.Scan(&r.Kind, &r.ItemID, &r.ReportID, &r.CaseID, &r.InquiryID, &r.OrderID, &r.Position, &r.AccountID,
			&r.MeasureID, &r.RequestID, &r.FoodRegID, &r.KYCID, &r.StoreID, &r.StoreName, &r.Start, &r.Due))
		out = append(out, r)
	}
	must(t, rows.Err())
	return out
}

// span 是一件待辦預期的開始與期限；store 是預期的店家（0 是沒有單一店家）。
type span struct {
	start, due time.Time
	store      int64
}

// reviewSet 是函式在 at 的結果：item_id → 一列。
func (f *fixture) reviewSet(t *testing.T, fn string, at time.Time) map[string]reviewRow {
	t.Helper()
	out := map[string]reviewRow{}
	for _, r := range f.reviewOf(t, fn, at) {
		if _, dup := out[r.ItemID]; dup {
			t.Fatalf("%s：item_id %s 重複", fn, r.ItemID)
		}
		out[r.ItemID] = r
	}
	return out
}

func (f *fixture) storeNameOf(t *testing.T, storeID int64) string {
	t.Helper()
	var name *string
	ctx := context.Background()
	must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT store_name FROM seller.stores WHERE store_id = $1", storeID).Scan(&name))
	if name == nil {
		return ""
	}
	return *name
}

// wantReview 比較函式在 at 的結果與預期：成員集合相等，每一列的種類、店家、開始與期限等於預期（時間精確到 µs，不用容差）。
func (f *fixture) wantReview(t *testing.T, kind, fn string, at time.Time, want map[string]span) {
	t.Helper()
	got := f.reviewSet(t, fn, at)
	for id, w := range want {
		r, ok := got[id]
		if !ok {
			t.Errorf("%s：%s 應該在 %v 的結果裡", fn, id, at)
			continue
		}
		if r.Kind != kind {
			t.Errorf("%s：%s 的種類 %q，預期 %q", fn, id, r.Kind, kind)
		}
		if r.Start == nil || !r.Start.Equal(w.start) {
			t.Errorf("%s：%s 的開始 %v，預期 %v", fn, id, fmtTime(r.Start), w.start)
		}
		if r.Due == nil || !r.Due.Equal(w.due) {
			t.Errorf("%s：%s 的期限 %v，預期 %v", fn, id, fmtTime(r.Due), w.due)
		}
		switch {
		case w.store == 0 && (r.StoreID != nil || r.StoreName != nil):
			t.Errorf("%s：%s 沒有單一店家，卻有 store_id=%v store_name=%v", fn, id, r.StoreID, r.StoreName)
		case w.store != 0 && (r.StoreID == nil || *r.StoreID != w.store):
			t.Errorf("%s：%s 的店家 %v，預期 %d", fn, id, r.StoreID, w.store)
		case w.store != 0 && (r.StoreName == nil || *r.StoreName != f.storeNameOf(t, w.store)):
			t.Errorf("%s：%s 的店名 %v，預期 %q", fn, id, r.StoreName, f.storeNameOf(t, w.store))
		}
	}
	for id := range got {
		if _, ok := want[id]; !ok {
			t.Errorf("%s：%s 不該在 %v 的結果裡（差一點不是的反例進來了）", fn, id, at)
		}
	}
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return "NULL"
	}
	return t.Format(time.RFC3339Nano)
}

// taiwanID 產生格式與檢查碼都正確的身分證號（A + 性別碼 1 + 7 位序號 + 檢查碼；不是真實的人）。
func taiwanID(serial int) string {
	body := fmt.Sprintf("1%07d", serial)
	sum := 1 // 'A' = 10：十位數 1 權重 1、個位數 0 權重 9
	for i, c := range body {
		sum += int(c-'0') * (8 - i)
	}
	return "A" + body + fmt.Sprint((10-sum%10)%10)
}

// submitKYCFor 為帳號建立並送出一份個人賣家的申請（身分證號與銀行帳號各用序號換一個），回傳送出的申請。
func (f *fixture) submitKYCFor(t *testing.T, accountID int64, serial int) KYCApplication {
	t.Helper()
	d := f.savedDraft(t, kycDraft(accountID, taiwanID(serial), fmt.Sprintf("0123456789%02d", serial%100)))
	a, err := f.module.SubmitKYC(context.Background(), SubmitKYCInput{AcceptedAgreements: CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: d.Version})
	must(t, err)
	return a
}

// tick 讓時鐘前進 d（模組用同一個時鐘，之後建立的列從這一刻起算）。
func (f *fixture) tick(d time.Duration) time.Time {
	f.now = f.now.Add(d)
	return f.now
}

// reviewWorld 是每一種待辦的成員，加上每一種「差一點不是」的反例。want 是世界建完的那一刻（at）函式預期的結果：種類 → item_id → 開始、期限與店家；
// 預期的時間都是建立當下的時鐘加上設計 §1.3 的時長（字面值寫在這裡，不從 SQL 或 Go 常數讀，所以任一分支的時長改掉測試會失敗）。
type reviewWorld struct {
	f    *fixture
	at   time.Time
	want map[string]map[string]span

	// 邊界與判定式比對用的對象。
	openInquiry     uuid.UUID // 還沒結束、due_at 在 at 之後的詢問
	openInquiryDue  time.Time
	openInquiryCase uuid.UUID
	withheld        struct {
		order    uuid.UUID
		position int
		recheck  time.Time
		store    int64
	}
	risks              []riskMember // 全部的 RISK 措施（複審用）
	reviewSoon         riskMember   // 複審已進入的 RISK 措施（review_by 在建立後 20 小時，at 時還沒到）
	reviewLater        riskMember   // 複審還沒進入的 RISK 措施（review_by 在建立後 60 小時，at 時進入時間還沒到）
	appealID, liftID   uuid.UUID
	appealMeasure      uuid.UUID
	liftMeasure        uuid.UUID
	mootMeasure        uuid.UUID // 解除請求還是 PENDING、措施已結束
	foodStartsAtKYC    uuid.UUID // 食品業者登錄先送出、KYC 後核准：開始是 KYC 核准時間
	foodStartsAtSubmit uuid.UUID // KYC 先核准、登錄後送出：開始是送出時間

	bindRetry           uuid.UUID // 綁定重試用完的待辦對象（申請 ID）
	bindRetryStore      int64
	bindRetryDraftStore int64 // 綁定重試用完、申請被改回草稿的店家：不是待辦
}

type riskMember struct {
	id       uuid.UUID
	store    int64
	reviewBy time.Time
}

func newReviewWorld(t *testing.T, f *fixture) *reviewWorld {
	t.Helper()
	ctx := context.Background()
	w := &reviewWorld{f: f, want: map[string]map[string]span{}}
	for _, k := range reviewKinds {
		w.want[k.kind] = map[string]span{}
	}
	add := func(kind, id string, store int64, start, due time.Time) {
		w.want[kind][id] = span{start: start, due: due, store: store}
	}
	const (
		hour = time.Hour
		day  = 24 * time.Hour
	)

	// 待分案：食安 4 小時、其他類別 72 小時；已分案與已有結果的不是。
	_, sR := f.open(t)
	ra := f.reportBy(t, sR.StoreID, policy.Fraud)
	add("REPORT_TRIAGE", ra.ID.String(), sR.StoreID, f.now, f.now.Add(72*hour))
	f.tick(hour)
	rb := f.reportBy(t, sR.StoreID, policy.FoodSafety)
	add("REPORT_TRIAGE", rb.ID.String(), sR.StoreID, f.now, f.now.Add(4*hour))
	f.tick(hour)
	f.mustResolve(t, f.reportBy(t, sR.StoreID, policy.Fraud).ID, OutcomeNoViolation)

	// 待決定：每個嚴重度都有期限（逐一取自 SeverityValues，新增嚴重度而 SQL 的 CASE 沒有對應時，期限是 NULL 或這裡查不到時長，測試失敗）；已結案的不是。
	_, sK := f.open(t)
	severityDue := map[Severity]time.Duration{
		SeverityCritical: 24 * hour, SeverityHigh: 72 * hour, SeverityMedium: 7 * day, SeverityLow: 14 * day,
	}
	severityCategory := map[Severity]policy.Category{
		SeverityCritical: policy.Fraud, SeverityHigh: policy.Privacy, SeverityMedium: policy.MisleadingInfo, SeverityLow: policy.Counterfeit,
	}
	for _, sev := range SeverityValues() {
		due, ok := severityDue[sev]
		if !ok {
			t.Fatalf("嚴重度 %s 沒有預期的決定期限：審閱政策（seller.review_case_decision 的 CASE）與這張表要一起補", sev)
		}
		category, ok := severityCategory[sev]
		if !ok {
			category = policy.Fraud
		}
		f.tick(hour)
		opened, _ := f.newCase(t, sK.StoreID, category, sev, 1)
		add("CASE_DECISION", opened.ID.String(), sK.StoreID, f.now, f.now.Add(due))
	}
	closed, _ := f.newCase(t, sK.StoreID, policy.ProhibitedGoods, SeverityHigh, 1)
	_, err := f.decide(t, closed.ID, DecisionNoAction, "")
	must(t, err)

	// 詢問：沒結束的案件不在待決定；回覆後的開始是「收到」的時間（不是記下的時間），管理員撤回後的開始是撤回時間。
	_, sQ := f.open(t)
	f.tick(hour)
	open, _ := f.newCase(t, sQ.StoreID, policy.ProhibitedGoods, SeverityHigh, 1)
	openInquiry := f.mustInquire(t, open.ID)
	w.openInquiry, w.openInquiryCase, w.openInquiryDue = openInquiry.Inquiry.ID, open.ID, openInquiry.Inquiry.DueAt
	f.tick(hour)
	replied, _ := f.newCase(t, sQ.StoreID, policy.UnlicensedSale, SeverityHigh, 1)
	repliedInquiry := f.mustInquire(t, replied.ID)
	f.tick(2 * hour)
	received := f.now.Add(-time.Hour)
	_, err = f.reply(t, repliedInquiry.Inquiry.ID, received)
	must(t, err)
	add("CASE_DECISION", replied.ID.String(), sQ.StoreID, received, received.Add(72*hour))
	f.tick(hour)
	withdrawn, _ := f.newCase(t, sQ.StoreID, policy.FakeCertification, SeverityMedium, 1)
	withdrawnInquiry := f.mustInquire(t, withdrawn.ID)
	f.tick(90 * time.Minute)
	_, err = f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: withdrawnInquiry.Inquiry.ID, AdminID: 9})
	must(t, err)
	add("CASE_DECISION", withdrawn.ID.String(), sQ.StoreID, f.now, f.now.Add(7*day))

	// 待轉報：食安、決定採取措施、還沒有轉報紀錄；已轉報、已標記不轉報、不採取措施、非食安的都不是。
	foodCase := func(category policy.Category, decision Decision) (AdminCase, int64) {
		_, s := f.open(t)
		c, _ := f.newCase(t, s.StoreID, category, SeverityLow, 1)
		if decision == DecisionActionTaken {
			w.risks = append(w.risks, riskOf(f.riskFromCase(t, s.StoreID, c.ID)))
		}
		f.tick(hour)
		_, err := f.decide(t, c.ID, decision, "")
		must(t, err)
		return c, s.StoreID
	}
	pending, sPending := foodCase(policy.FoodSafety, DecisionActionTaken)
	add("CASE_FDA_REFERRAL", pending.ID.String(), sPending, f.now, f.now.Add(24*hour))
	waived, _ := foodCase(policy.FoodSafety, DecisionActionTaken)
	_, err = f.module.WaiveFDAReferral(ctx, WaiveFDAReferralInput{CaseID: waived.ID, AdminID: 9, Reason: "不轉報"})
	must(t, err)
	referred, _ := foodCase(policy.FoodSafety, DecisionActionTaken)
	_, err = f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: referred.ID, AdminID: 9, Channel: FDAOnlineForm})
	must(t, err)
	foodCase(policy.FoodSafety, DecisionNoAction)
	foodCase(policy.Fraud, DecisionActionTaken)

	// 主管機關來函：沒完成也沒撤銷的；暫不告知的對象在確認日期（台北）到了之後進入，已告知與由措施告知的不是。
	accountA, sA := f.open(t)
	accountB, _ := f.open(t)
	accountC, _ := f.open(t)
	f.tick(hour)
	open1 := f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountA}))
	add("AUTHORITY_ORDER", open1.ID.String(), 0, f.now, f.now.Add(72*hour))
	f.tick(hour)
	revoked := f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountB}))
	_, err = f.module.RevokeAuthorityOrder(ctx, RevokeAuthorityOrderInput{ID: revoked.ID, AdminID: 9, ExpectedVersion: revoked.Version, Reason: "登記錯誤"})
	must(t, err)
	f.tick(hour)
	done := f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountA, StoreID: &sA.StoreID}, AuthorityTargetInput{AccountID: accountC}, AuthorityTargetInput{AccountID: accountB}))
	recheck, later := day2026(10, 5), day2026(10, 8)
	notified := f.now
	_, err = f.completeOrder(done, nil, []TargetNoticeInput{
		{Position: 0, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &recheck, Note: "偵查不公開"}},
		{Position: 1, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeWithheld, RecheckOn: &later}},
		{Position: 2, Notice: AuthorityNoticeInput{Kind: AuthorityNoticeNotified, NotifiedAt: &notified, Note: "電話告知"}},
	})
	must(t, err)
	w.withheld.order, w.withheld.position, w.withheld.recheck, w.withheld.store = done.ID, 0, recheck, sA.StoreID

	// 申覆與解除請求：待處理的；解除請求還要措施在生效中（措施已結束的不是，儲存的請求仍是 PENDING）。
	f.tick(hour)
	appealAccount, sAppeal, appealMeasure := f.appealable(t)
	appealed := f.appeal(t, appealAccount, appealMeasure, "不成立")
	w.appealMeasure, w.appealID = appealMeasure.ID, appealed.Pending().ID
	add("MEASURE_APPEAL", w.appealID.String(), sAppeal.StoreID, f.now, f.now.Add(7*day))
	f.tick(hour)
	liftAccount, sLift, liftMeasure := f.remediable(t)
	w.risks = append(w.risks, riskOf(liftMeasure))
	lifted, err := f.module.RequestRemediation(ctx, f.remediationInput(t, liftAccount, liftMeasure.ID, "已改善"))
	must(t, err)
	w.liftMeasure, w.liftID = liftMeasure.ID, lifted.Pending().ID
	add("MEASURE_LIFT_REQUEST", w.liftID.String(), sLift.StoreID, f.now, f.now.Add(7*day))
	f.tick(hour)
	// 固定期間的 RISK 停權在世界建完之前到期：儲存的請求仍是 PENDING，但措施已不在生效中，推導為 CLOSED，不是待辦。
	mootAccount, sMoot := f.open(t)
	mootEnds := f.now.Add(2 * hour)
	mootMeasure := f.riskMeasure(t, sMoot.StoreID, &mootEnds)
	_, err = f.module.RequestRemediation(ctx, f.remediationInput(t, mootAccount, mootMeasure.ID, "已改善"))
	must(t, err)
	w.mootMeasure = mootMeasure.ID

	// 複審：RISK、生效中、review_by 前 24 小時進入。建立後 20 小時到期的已進入，60 小時到期的（建立後 36 小時才進入）還沒；已結束的不是。
	reviewRisk := func(in time.Duration) riskMember {
		_, s := f.open(t)
		review := f.now.Add(in)
		r, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
			AdminID: 7, StoreID: s.StoreID, Kind: MeasureSuspension, StatementToSeller: "疑似詐欺", Note: "內部紀錄", ReviewBy: &review, Risk: &RiskBasis{Ground: RiskFraud},
		})
		must(t, err)
		m := riskOf(r.Measure)
		w.risks = append(w.risks, m)
		return m
	}
	f.tick(hour)
	w.reviewSoon, w.reviewLater = reviewRisk(20*hour), reviewRisk(60*hour)
	add("MEASURE_REVIEW", w.reviewSoon.id.String(), w.reviewSoon.store, w.reviewSoon.reviewBy.Add(-24*hour), w.reviewSoon.reviewBy)

	// 食品業者登錄：開始是 max(送出, 目前核准的商家認證的核准時間)；等商家認證、店家已關閉、已核准的不是。
	f.tick(hour)
	accountF1, sF1 := f.open(t)
	f.submitFood(t, sF1, "F1-1")
	f.tick(2 * hour)
	f1 := f.submitKYCFor(t, accountF1, 101)
	must(t, approve(f, f1))
	w.foodStartsAtKYC = f.pendingID(t, sF1)
	add("FOOD_REGISTRATION", w.foodStartsAtKYC.String(), sF1.StoreID, f.now, f.now.Add(7*day))

	f.tick(hour)
	accountF2, sF2 := f.open(t)
	f2 := f.submitKYCFor(t, accountF2, 102)
	must(t, approve(f, f2))
	f.tick(3 * hour)
	f.submitFood(t, sF2, "F2-1")
	w.foodStartsAtSubmit = f.pendingID(t, sF2)
	add("FOOD_REGISTRATION", w.foodStartsAtSubmit.String(), sF2.StoreID, f.now, f.now.Add(7*day))

	f.tick(hour)
	accountF3, sF3 := f.open(t)
	f.submitFood(t, sF3, "F3-1")
	f3 := f.submitKYCFor(t, accountF3, 103) // 審核中：登錄還在等商家認證，但商家認證本身是待辦
	add("KYC_APPLICATION", f3.ID.String(), sF3.StoreID, f.now, f.now.Add(7*day))

	f.tick(hour)
	accountF4, sF4 := f.open(t)
	f4 := f.submitKYCFor(t, accountF4, 104)
	must(t, approve(f, f4))
	f.submitFood(t, sF4, "F4-1")
	_, err = f.module.CloseCurrent(ctx, accountF4)
	must(t, err)

	f.tick(hour)
	accountF5, sF5 := f.open(t)
	f5 := f.submitKYCFor(t, accountF5, 105)
	must(t, approve(f, f5))
	f.submitFood(t, sF5, "F5-1")
	f.mustDecide(t, f.pendingID(t, sF5), FoodRegApproved)

	// 商家認證：審核中的；草稿、被退回、已核准的不是。
	f.tick(hour)
	accountK1, sK1 := f.open(t)
	k1 := f.submitKYCFor(t, accountK1, 106)
	add("KYC_APPLICATION", k1.ID.String(), sK1.StoreID, f.now, f.now.Add(7*day))
	accountK2, _ := f.open(t)
	k2 := f.submitKYCFor(t, accountK2, 107)
	f.reject(t, k2, anIssue()...)
	accountK3, _ := f.open(t)
	f.savedDraft(t, kycDraft(accountK3, taiwanID(108), "012345678908"))

	// 重新綁定 PayUni：綁定重試用完（PAYUNI_UNAVAILABLE）、店家營運中、申請已核准的；開始是綁定失敗的時刻，期限 24 小時。
	// 其他碼（資料不完整、PayUni 永久拒絕）、申請被改回草稿、店家已關閉、還在綁定、已綁定的都不是。
	bindFailed := func(serial int, errs ...error) (int64, int64, KYCApplication) {
		accountID, s := f.open(t)
		k := f.submitKYCFor(t, accountID, serial)
		must(t, approve(f, k))
		f.payuni.bindErr = errs
		_, err := f.module.SyncPayUni(ctx, s.StoreID, true)
		must(t, err)
		return accountID, s.StoreID, k
	}
	f.tick(hour)
	_, sB1, kB1 := bindFailed(110, errNetwork)
	w.bindRetry, w.bindRetryStore = kB1.ID, sB1
	add("PAYUNI_BIND_RETRY", kB1.ID.String(), sB1, f.now, f.now.Add(24*hour))
	f.tick(hour)
	bindFailed(111, apiErr("AGENT03005"))
	f.tick(hour)
	_, sB3, kB3 := bindFailed(112, errNetwork)
	_, err = f.db.Conn(ctx).Exec(ctx, `UPDATE seller.kyc_applications SET status = 'DRAFT', decided_at = NULL WHERE application_id = $1`, kB3.ID)
	must(t, err)
	w.bindRetryDraftStore = sB3
	accountB4, _, _ := bindFailed(113, errNetwork)
	_, err = f.module.CloseCurrent(ctx, accountB4)
	must(t, err)
	accountB5, sB5 := f.open(t)
	kB5 := f.submitKYCFor(t, accountB5, 114)
	must(t, approve(f, kB5))
	_ = sB5
	accountB6, sB6 := f.open(t)
	kB6 := f.submitKYCFor(t, accountB6, 115)
	must(t, approve(f, kB6))
	f.payuni.bindErr = nil
	_, err = f.module.SyncPayUni(ctx, sB6.StoreID, true)
	must(t, err)

	f.tick(hour)
	w.at = f.now
	return w
}

func riskOf(m Measure) riskMember {
	if m.ReviewBy == nil {
		panic("RISK 措施沒有 review_by")
	}
	return riskMember{id: m.ID, store: m.StoreID, reviewBy: *m.ReviewBy}
}

// day2026 是 2026 年某月某日的 DATE（UTC 午夜，Go 端 DATE 的形狀）。
func day2026(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

// 每一種函式一張表：成員集合、開始、期限與店家，反例（差一點不是）不在結果裡。時長的字面值寫在 newReviewWorld，不從 SQL 或 Go 常數讀：
// mutation——任一分支的時長（食安 4 小時、其他 72 小時、四個嚴重度、詢問逾期 24 小時、轉報 24 小時、7 天）改掉，這裡就失敗。
func TestReviewFunctionsReturnTheDesignedMembersStartsAndDeadlines(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := newReviewWorld(t, f)
	for _, k := range reviewKinds {
		t.Run(k.kind, func(t *testing.T) {
			f.wantReview(t, k.kind, k.fn, w.at, w.want[k.kind])
		})
	}
	// 沒有空的種類：每一種至少有一個成員（複審的在 at 已進入；詢問逾期與告知的成員由邊界測試涵蓋，這兩種在 at 還沒有成員）。
	for _, k := range reviewKinds {
		if len(w.want[k.kind]) == 0 && k.kind != "CASE_INQUIRY_OVERDUE" && k.kind != "AUTHORITY_TARGET_NOTICE" {
			t.Errorf("%s：世界裡沒有成員，反例沒有對照", k.kind)
		}
	}
}

// 邊界 ±1µs：詢問逾期（due_at 當下進入）、告知被指名者（確認日期的台北 00:00 進入）、複審（review_by 前 24 小時進入）、解除請求（措施的 ends_at 當下離開）。
// mutation：把 <= 改成 <、台北日改成 UTC 日、24 小時改成 23 小時 59 分，各自讓一個斷言失敗。
func TestReviewBoundariesAreExactToTheMicrosecond(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	w := newReviewWorld(t, f)
	us := time.Microsecond
	member := func(fn string, at time.Time, id string) (reviewRow, bool) {
		r, ok := f.reviewSet(t, fn, at)[id]
		return r, ok
	}

	// 詢問逾期：due_at 當下進入，開始是 due_at，期限再 24 小時。
	due := w.openInquiryDue
	id := w.openInquiry.String()
	if _, ok := member("review_case_inquiry_overdue", due.Add(-us), id); ok {
		t.Errorf("詢問逾期：due_at 前 1µs 不該進入")
	}
	if _, ok := member("review_case_inquiry_overdue", w.at, id); ok {
		t.Errorf("詢問逾期：還沒到 due_at（%v）不該進入", due)
	}
	for _, at := range []time.Time{due, due.Add(us)} {
		r, ok := member("review_case_inquiry_overdue", at, id)
		if !ok || !r.Start.Equal(due) || !r.Due.Equal(due.Add(24*time.Hour)) || r.CaseID == nil || *r.CaseID != w.openInquiryCase || r.InquiryID == nil {
			t.Errorf("詢問逾期：%v 應進入，開始 %v、期限 %v（預期 %v、%v）：%+v", at, fmtTime(r.Start), fmtTime(r.Due), due, due.Add(24*time.Hour), r)
		}
	}

	// 告知被指名者：確認日期 10-05（台北）的 00:00 進入（不是 UTC 的 00:00，差 8 小時）；10-08 的那個對象晚進入。
	midnight := time.Date(2026, 10, 5, 0, 0, 0, 0, twtime.Taipei)
	item := fmt.Sprintf("%s/%05d", w.withheld.order, w.withheld.position)
	for _, tt := range []struct {
		at   time.Time
		want bool
	}{
		{midnight.Add(-us), false},
		{midnight, true},
		{midnight.Add(us), true},
		{time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), true}, // UTC 的 10-05 00:00 已經是台北 08:00
		{time.Date(2026, 10, 4, 15, 59, 59, 999999000, time.UTC), false},
	} {
		r, ok := member("review_authority_target_notice", tt.at, item)
		if ok != tt.want {
			t.Errorf("告知：%v 進入=%v，預期 %v", tt.at, ok, tt.want)
		}
		if ok && (!r.Start.Equal(midnight) || !r.Due.Equal(midnight.Add(72*time.Hour)) || r.Position == nil || *r.Position != 0 ||
			r.StoreID == nil || *r.StoreID != w.withheld.store || r.AccountID == nil || r.OrderID == nil || *r.OrderID != w.withheld.order) {
			t.Errorf("告知：%v 的開始 %v、期限 %v（預期 %v、%v）：%+v", tt.at, fmtTime(r.Start), fmtTime(r.Due), midnight, midnight.Add(72*time.Hour), r)
		}
	}
	// 位置 1（確認日期 10-08、沒有指名店家）：10-08 的台北 00:00 才進入，店家欄是 NULL。
	second := fmt.Sprintf("%s/%05d", w.withheld.order, 1)
	secondMidnight := time.Date(2026, 10, 8, 0, 0, 0, 0, twtime.Taipei)
	if _, ok := member("review_authority_target_notice", secondMidnight.Add(-us), second); ok {
		t.Errorf("告知：位置 1 在 10-08 前不該進入")
	}
	if r, ok := member("review_authority_target_notice", secondMidnight, second); !ok || r.StoreID != nil || r.StoreName != nil || r.AccountID == nil {
		t.Errorf("告知：位置 1 在 10-08 進入，沒有指名店家時 store 是 NULL、帳號有值：%+v", r)
	}
	// 已告知的對象（位置 2）永遠不是。
	if _, ok := member("review_authority_target_notice", secondMidnight.Add(240*time.Hour), fmt.Sprintf("%s/%05d", w.withheld.order, 2)); ok {
		t.Errorf("告知：已告知的對象不該進入")
	}

	// 複審：review_by 前 24 小時進入，之後仍在（review_by 過了也還在，直到措施結束或撤銷）。
	risk := w.reviewLater
	start := risk.reviewBy.Add(-24 * time.Hour)
	rid := risk.id.String()
	if _, ok := member("review_measure_review", start.Add(-us), rid); ok {
		t.Errorf("複審：進入時間前 1µs 不該進入")
	}
	for _, at := range []time.Time{start, start.Add(us), risk.reviewBy, risk.reviewBy.Add(us), risk.reviewBy.Add(48 * time.Hour)} {
		r, ok := member("review_measure_review", at, rid)
		if !ok || !r.Start.Equal(start) || !r.Due.Equal(risk.reviewBy) || r.StoreID == nil || *r.StoreID != risk.store {
			t.Errorf("複審：%v 應在結果裡，開始 %v、期限 %v（預期 %v、%v）", at, fmtTime(r.Start), fmtTime(r.Due), start, risk.reviewBy)
		}
	}

	// 解除請求只在措施生效中：固定期間的 RISK 停權，ends_at 當下離開（effective_measures：ends_at > at）；進入時間是提出時間。
	accountID, s := f.open(t)
	ends := f.now.Add(10 * 24 * time.Hour)
	m := f.riskMeasure(t, s.StoreID, &ends)
	submitted := f.now
	requested, err := f.module.RequestRemediation(ctx, f.remediationInput(t, accountID, m.ID, "已改善"))
	must(t, err)
	lid := requested.Pending().ID.String()
	for _, tt := range []struct {
		at   time.Time
		want bool
	}{{ends.Add(-us), true}, {ends, false}, {ends.Add(us), false}} {
		r, ok := member("review_measure_lift_request", tt.at, lid)
		if ok != tt.want {
			t.Errorf("解除請求：%v 在結果裡=%v，預期 %v", tt.at, ok, tt.want)
		}
		if ok && (!r.Start.Equal(submitted) || !r.Due.Equal(submitted.Add(7*24*time.Hour))) {
			t.Errorf("解除請求：開始 %v、期限 %v", fmtTime(r.Start), fmtTime(r.Due))
		}
	}
}

// goReviewSets 是逐種套用指令真正用的 Go 定義得到的成員（item_id 的集合）：Report.State()、Case.Open() 加 AwaitingSeller()、
// CaseFacts.Availability().RecordReferral（加決定採取措施）、CaseInquiry.Open() 加 DueAt、AuthorityOrder.Open、AuthorityOrderTarget 的告知欄位、
// Measure.Pending／PendingRemediation、FoodRegistrationReview.CanApprove、KYC 的 SUBMITTED，以及複審到期分頁（收件匣的成員）。
// 正式程式沒有第二份成員規則，這些只在測試裡當比對的對象。時間用 f.now（模組的時鐘）。
// 同時回傳待決定的案件（比對開始時間用）。
func (f *fixture) goReviewSets(t *testing.T) (sets map[string]map[string]bool, decisionStarts map[string]time.Time) {
	t.Helper()
	ctx := context.Background()
	now := f.now
	sets = map[string]map[string]bool{}
	for _, k := range reviewKinds {
		sets[k.kind] = map[string]bool{}
	}
	decisionStarts = map[string]time.Time{}
	for _, r := range f.allReports(t, ReportListInput{}) {
		if r.State() == ReportStateAwaitingTriage {
			sets["REPORT_TRIAGE"][r.ID.String()] = true
		}
	}
	for _, c := range f.allCases(t, CaseListInput{}) {
		if c.Open() && !c.AwaitingSeller() {
			sets["CASE_DECISION"][c.ID.String()] = true
			// 開始時間由 fixture 的事實算：max(開案, 各詢問回覆收到或管理員撤回的時間)；不讀函式或 DecideDeadline（那是被比對的對象）。
			start := c.OpenedAt
			for _, i := range f.inquiriesOf(t, c.ID) {
				if r := inquiryClockRestart(i); r != nil && r.After(start) {
					start = *r
				}
			}
			decisionStarts[c.ID.String()] = start
		}
		if referralPending(c.Case) {
			sets["CASE_FDA_REFERRAL"][c.ID.String()] = true
		}
		var after *InquiryCursor
		for {
			page, err := f.module.CaseInquiries(ctx, CaseInquiryListInput{CaseID: c.ID, PageSize: 2, After: after})
			must(t, err)
			for _, i := range page.Items {
				if i.Open() && !now.Before(i.DueAt) {
					sets["CASE_INQUIRY_OVERDUE"][i.ID.String()] = true
				}
			}
			if page.Next == nil {
				break
			}
			after = page.Next
		}
	}
	var afterOrder *AuthorityCursor
	for {
		page, err := f.module.AuthorityOrders(ctx, AuthorityOrderListInput{PageSize: 3, After: afterOrder})
		must(t, err)
		for _, o := range page.Items {
			if o.Open() {
				sets["AUTHORITY_ORDER"][o.ID.String()] = true
			}
			for _, target := range o.Targets {
				if n := target.Notice; n != nil && n.Kind == AuthorityNoticeWithheld && n.RecheckOn != nil && !n.RecheckOn.After(twtime.Day(now)) {
					sets["AUTHORITY_TARGET_NOTICE"][fmt.Sprintf("%s/%05d", o.ID, target.Position)] = true
				}
			}
		}
		if page.Next == nil {
			break
		}
		afterOrder = page.Next
	}
	var afterMeasure *MeasureCursor
	for {
		page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{PageSize: 3, After: afterMeasure})
		must(t, err)
		for _, m := range page.Items {
			if p := m.Pending(); p != nil && p.Kind == RequestKindAppeal {
				sets["MEASURE_APPEAL"][p.ID.String()] = true
			}
			if p := m.PendingRemediation(); p != nil {
				sets["MEASURE_LIFT_REQUEST"][p.ID.String()] = true
			}
		}
		if page.Next == nil {
			break
		}
		afterMeasure = page.Next
	}
	// 複審到期分頁：收件匣的成員，與 review_measure_review 讀同一支函式。
	afterMeasure = nil
	for {
		page, err := f.module.AdminMeasures(ctx, AdminMeasureListInput{PageSize: 2, ReviewDue: true, After: afterMeasure})
		must(t, err)
		for _, m := range page.Items {
			sets["MEASURE_REVIEW"][m.ID.String()] = true
		}
		if page.Next == nil {
			break
		}
		afterMeasure = page.Next
	}
	for _, inbox := range []FoodRegInbox{FoodRegInboxDecidable, FoodRegInboxAwaitingKYC} {
		inbox := inbox
		var after *FoodRegistrationCursor
		for {
			page, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{Inbox: &inbox, PageSize: 2, After: after})
			must(t, err)
			for _, r := range page.Items {
				if r.CanApprove() {
					sets["FOOD_REGISTRATION"][r.ID.String()] = true
				}
			}
			if page.Next == nil {
				break
			}
			after = page.Next
		}
	}
	var afterKYC *KYCCursor
	for {
		page, err := f.module.AdminKYCApplications(ctx, KYCListInput{PageSize: 3, After: afterKYC})
		must(t, err)
		for _, a := range page.Items {
			if a.Status == KYCStatusSubmitted {
				sets["KYC_APPLICATION"][a.ID.String()] = true
			}
		}
		if page.Next == nil {
			break
		}
		afterKYC = page.Next
	}
	// 重新綁定 PayUni：賣家看到的 PayUni 狀態是「等平台重新送出」（RETRY_LATER）的店家，店家營運中、申請已核准。
	afterKYC = nil
	for {
		page, err := f.module.AdminKYCApplications(ctx, KYCListInput{PageSize: 3, After: afterKYC})
		must(t, err)
		for _, a := range page.Items {
			if a.Status != KYCStatusApproved {
				continue
			}
			mine, err := f.module.MyKYC(ctx, a.AccountID)
			if apperr.KindOf(err) == apperr.KindNotFound {
				continue
			}
			must(t, err)
			if mine.PayUni != nil && mine.PayUni.State == PayUniState("RETRY_LATER") {
				sets["PAYUNI_BIND_RETRY"][a.ID.String()] = true
			}
		}
		if page.Next == nil {
			break
		}
		afterKYC = page.Next
	}
	return sets, decisionStarts
}

// 重新綁定 PayUni 待辦的生命週期：開始時間是綁定失敗的時刻（失敗期間店家改地址推後的是 updated_at，不是開始時間）；管理員重新綁定後待辦消失；
// 申請被改回草稿的店家不在待辦裡，管理員重新綁定被擋下（綁定 job 會把還沒審核的草稿送給 PayUni）。
// mutation：把開始時間改讀 updated_at，改地址之後開始時間會往後跑；拿掉 Rebind 的申請檢查，草稿的店家會被重新綁定。
func TestPayUniBindRetryStartIsTheFailureTimeAndEndsWithRebind(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := newReviewWorld(t, f)
	ctx := context.Background()
	rows := f.reviewOf(t, "review_payuni_bind_retry", w.at)
	var before *reviewRow
	for i := range rows {
		if rows[i].KYCID != nil && *rows[i].KYCID == w.bindRetry {
			before = &rows[i]
		}
	}
	if before == nil {
		t.Fatalf("待辦不在結果裡：%+v", rows)
	}
	f.tick(3 * time.Hour)
	setAddress(f, t, w.bindRetryStore, "改過的地址")
	if r := f.registration(t, w.bindRetryStore); !r.UpdatedAt.After(*before.Start) {
		t.Fatalf("改地址應該推後 updated_at：%+v", r)
	}
	for _, r := range f.reviewOf(t, "review_payuni_bind_retry", f.now) {
		if r.KYCID != nil && *r.KYCID == w.bindRetry && (!r.Start.Equal(*before.Start) || !r.Due.Equal(*before.Due)) {
			t.Fatalf("改地址之後開始 %v、期限 %v，要維持 %v、%v", fmtTime(r.Start), fmtTime(r.Due), fmtTime(before.Start), fmtTime(before.Due))
		}
	}

	_, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: w.bindRetryDraftStore, AdminID: 9})
	if apperr.KindOf(err) != apperr.KindFailedPrecondition || !strings.Contains(err.Error(), "賣家正在修改商家認證，重新核准後會自動重新綁定。") {
		t.Fatalf("申請被改回草稿時重新綁定要被擋下：%v", err)
	}
	if r := f.registration(t, w.bindRetryDraftStore); r.BindStatus != BindStatusFailed {
		t.Fatalf("被擋下之後註冊不該改變：%+v", r)
	}

	if _, err := f.module.RebindPayUni(ctx, RebindPayUniInput{StoreID: w.bindRetryStore, AdminID: 9}); err != nil {
		t.Fatal(err)
	}
	for _, r := range f.reviewOf(t, "review_payuni_bind_retry", f.now) {
		if r.KYCID != nil && *r.KYCID == w.bindRetry {
			t.Fatalf("重新綁定之後待辦要消失：%+v", r)
		}
	}
}

// 後台待辦 PAYUNI_BIND_RETRY 與重新綁定的可用性是單向關係：待辦有列的店家，重新綁定此刻一定可做（原因清單是空的）。反過來不成立：其他綁定失敗的碼刻意不進待辦
// （5a F16，客服接手）。資料集裡有一家申請被改回草稿的店家（bindRetryDraftStore）：它的原因是 KYC_NOT_APPROVED，不能在待辦裡。
// mutation：review_payuni_bind_retry 拿掉 k.status = 'APPROVED' → 草稿的店家進待辦，這裡失敗。
func TestBindRetryQueueIsASubsetOfRebindableStores(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := newReviewWorld(t, f)
	ctx := context.Background()
	rows := f.reviewOf(t, "review_payuni_bind_retry", w.at)
	if len(rows) == 0 {
		t.Fatal("資料集沒有任何重新綁定的待辦，斷言沒有意義")
	}
	for _, r := range rows {
		view, err := f.module.PayUniRegistrationOf(ctx, *r.StoreID)
		must(t, err)
		if len(view.RebindUnavailable) != 0 {
			t.Errorf("店家 %d 在待辦裡，重新綁定卻不能做：%v", *r.StoreID, view.RebindUnavailable)
		}
	}
	// 夾具：被改回草稿的店家不在待辦，原因是申請不是已核准。
	view, err := f.module.PayUniRegistrationOf(ctx, w.bindRetryDraftStore)
	must(t, err)
	if !slices.Equal(view.RebindUnavailable, []StoreActionUnavailableReason{StoreReasonKYCNotApproved}) {
		t.Fatalf("draft store reasons=%v", view.RebindUnavailable)
	}
	for _, r := range rows {
		if *r.StoreID == w.bindRetryDraftStore {
			t.Fatalf("申請被改回草稿的店家不該在待辦裡：%+v", r)
		}
	}
}

// 成員 ⇔ Go 判定式：同一份資料集，逐種比較函式與判定式的成員集合；世界建完的那一刻（at）與一週後（詢問逾期、告知、複審都進入）各比一次。
// 待決定的開始時間再與由開案與詢問的事實算出的值比（含管理員撤回、不含系統撤回）。
// mutation：任一支函式的條件（例如詢問逾期的 <=、食品業者登錄的店家狀態、解除請求的生效中）改掉，那一種的集合就不等於判定式的集合。
func TestReviewMembersAgreeWithTheGoPredicates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := newReviewWorld(t, f)
	for _, tt := range []struct {
		name string
		at   time.Time
	}{{"建完的那一刻", w.at}, {"一週後", w.at.Add(7 * 24 * time.Hour)}} {
		f.now = tt.at
		goSets, starts := f.goReviewSets(t)
		for _, k := range reviewKinds {
			got := f.reviewSet(t, k.fn, tt.at)
			if len(got) != len(goSets[k.kind]) {
				t.Errorf("%s／%s：函式 %d 個，判定式 %d 個", tt.name, k.kind, len(got), len(goSets[k.kind]))
			}
			for id := range got {
				if !goSets[k.kind][id] {
					t.Errorf("%s／%s：%s 在函式裡，判定式說不是", tt.name, k.kind, id)
				}
			}
			for id := range goSets[k.kind] {
				if _, ok := got[id]; !ok {
					t.Errorf("%s／%s：%s 判定式說是，函式裡沒有", tt.name, k.kind, id)
				}
			}
			if tt.name == "一週後" && len(got) == 0 {
				t.Errorf("%s／%s：資料集裡沒有成員，比對是空轉", tt.name, k.kind)
			}
		}
		for id, start := range starts {
			if r := f.reviewSet(t, "review_case_decision", tt.at)[id]; r.Start == nil || !r.Start.Equal(start) {
				t.Errorf("%s：案件 %s 待決定的開始 %v，事實算出 %v", tt.name, id, fmtTime(r.Start), start)
			}
		}
	}
}

// 依 ID 批次讀的計畫：`WHERE <對象鍵> = ANY(ids)` 推進到底層表（函式是單一 SELECT、被內聯），在底層表的掃描節點上就是條件（Index Cond，或資料集小、
// planner 選了部分索引時的 Filter），而不是先算出全部再由外層過濾。函式沒有被內聯（Function Scan）或條件留在外層（Subquery Scan 的 Filter、
// Join Filter）時，底層表的節點上看不到 `= ANY`。EXPLAIN 不跑函式；計畫形狀與 TestExposableStoresJoinProbesMeasuresByAccount 同一個理由。
// mutation：把 review_report_triage 改成在函式內 UNION ALL 兩段、或改成 plpgsql，這裡失敗。
func TestReviewDeadlinePushdown(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := newReviewWorld(t, f)
	ctx := context.Background()
	for _, tt := range []struct{ fn, key, table, column string }{
		{"review_report_triage", "report_id", "reports", "id"},
		{"review_case_decision", "case_id", "cases", "id"},
		{"review_case_fda_referral", "case_id", "cases", "id"},
		{"review_authority_order", "authority_order_id", "authority_orders", "id"},
		{"review_measure_appeal", "request_id", "measure_requests", "id"},
		{"review_measure_lift_request", "request_id", "measure_requests", "id"},
		{"review_measure_review", "measure_id", "measures", "id"},
		{"review_food_registration", "food_registration_id", "food_registrations", "id"},
		{"review_kyc_application", "kyc_application_id", "kyc_applications", "application_id"},
		{"review_payuni_bind_retry", "kyc_application_id", "kyc_applications", "application_id"},
	} {
		var plan []byte
		err := f.db.InTx(ctx, func(ctx context.Context) error {
			ids := []uuid.UUID{uuid.NewV7()}
			return f.db.Conn(ctx).QueryRow(ctx, fmt.Sprintf(`EXPLAIN (FORMAT JSON) SELECT r.%s::uuid, r.start_at::timestamptz, r.due_at::timestamptz
				FROM seller.%s($1::timestamptz) r WHERE r.%s = ANY($2::uuid[])`, tt.key, tt.fn, tt.key), w.at, ids).Scan(&plan)
		})
		must(t, err)
		if strings.Contains(string(plan), "Function Scan") || strings.Contains(string(plan), "Subquery Scan") {
			t.Errorf("%s：函式沒有被內聯：\n%s", tt.fn, plan)
		}
		pushed := false
		for _, n := range planNodes(t, plan) {
			if n["Relation Name"] != tt.table {
				continue
			}
			for _, field := range []string{"Index Cond", "Filter"} {
				if cond, _ := n[field].(string); anyOn(tt.column).MatchString(cond) {
					pushed = true
				}
			}
		}
		if !pushed {
			t.Errorf("%s：`%s = ANY` 沒有推進到 %s 的掃描節點（Index Cond 或 Filter 沒有 %s = ANY）：\n%s", tt.fn, tt.key, tt.table, tt.column, plan)
		}
	}
}

// planNodes 攤平 EXPLAIN (FORMAT JSON) 的全部計畫節點。
func planNodes(t *testing.T, plan []byte) []map[string]any {
	t.Helper()
	var doc []struct {
		Plan map[string]any `json:"Plan"`
	}
	must(t, json.Unmarshal(plan, &doc))
	var out []map[string]any
	var walk func(n map[string]any)
	walk = func(n map[string]any) {
		out = append(out, n)
		children, _ := n["Plans"].([]any)
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
	}
	for _, d := range doc {
		walk(d.Plan)
	}
	return out
}

// anyOn 比對計畫條件裡 `(<column> = ANY (` 的寫法。
func anyOn(column string) *regexp.Regexp {
	return regexp.MustCompile(`\(` + regexp.QuoteMeta(column) + ` = ANY \(`)
}

func wantDeadline(t *testing.T, label string, got *Deadline, start, due time.Time) {
	t.Helper()
	if got == nil {
		t.Errorf("%s：義務還在，期限要有值", label)
		return
	}
	if !got.Start.Equal(start) || !got.Due.Equal(due) {
		t.Errorf("%s：開始 %v、期限 %v，預期 %v、%v", label, got.Start, got.Due, start, due)
	}
}

func wantNoDeadline(t *testing.T, label string, got *Deadline) {
	t.Helper()
	if got != nil {
		t.Errorf("%s：義務已經結束，不該有期限：%+v", label, *got)
	}
}

// 資源的 *_deadline 在義務還在時有值（開始與期限等於設計 §1.2 的值），義務結束後沒有；清單與單筆讀同一份。
func TestResourceDeadlinesExistWhileTheObligationDoes(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	const hour = time.Hour

	// 通報：待分案有分案期限（食安 4 小時、其他 72 小時），分案後沒有。
	_, sR := f.open(t)
	food := f.reportBy(t, sR.StoreID, policy.FoodSafety)
	fraud := f.reportBy(t, sR.StoreID, policy.Fraud)
	created := f.now
	wantDeadline(t, "食安通報", f.reportOf(t, food.ID).TriageDeadline, created, created.Add(4*hour))
	wantDeadline(t, "其他類別通報", f.reportOf(t, fraud.ID).TriageDeadline, created, created.Add(72*hour))
	for _, r := range f.allReports(t, ReportListInput{}) {
		switch r.ID {
		case food.ID:
			wantDeadline(t, "清單的食安通報", r.TriageDeadline, created, created.Add(4*hour))
		case fraud.ID:
			wantDeadline(t, "清單的其他類別通報", r.TriageDeadline, created, created.Add(72*hour))
		}
	}
	f.tick(hour)
	triaged := f.mustTriage(t, food.ID, policy.FoodSafety, severityPtr(SeverityHigh))
	wantNoDeadline(t, "分案後的通報", f.reportOf(t, food.ID).TriageDeadline)
	wantNoDeadline(t, "分案後的通報（分案的回應）", triaged.TriageDeadline)
	opened := f.now

	// 案件：待決定有決定期限（嚴重度 HIGH 72 小時），詢問期間沒有，回覆後從收到的時間重新起算；結案後沒有。
	c := f.caseOf(t, *triaged.CaseID)
	wantDeadline(t, "待決定", c.DecideDeadline, opened, opened.Add(72*hour))
	wantNoDeadline(t, "還沒有決定的案件沒有轉報期限", c.FDAReferralDeadline)
	f.tick(hour)
	inquiry := f.mustInquire(t, c.ID)
	wantNoDeadline(t, "等賣家的案件（詢問的回應）", inquiry.Case.DecideDeadline)
	wantNoDeadline(t, "等賣家的案件", f.caseOf(t, c.ID).DecideDeadline)
	wantDeadline(t, "詢問的回覆期限", &Deadline{Start: inquiry.Inquiry.CreatedAt, Due: inquiry.Inquiry.DueAt}, f.now, f.now.Add(InquiryDefaultDue))
	f.tick(2 * hour)
	received := f.now.Add(-hour)
	_, err := f.reply(t, inquiry.Inquiry.ID, received)
	must(t, err)
	wantDeadline(t, "回覆後的待決定", f.caseOf(t, c.ID).DecideDeadline, received, received.Add(72*hour))
	f.riskFromCase(t, sR.StoreID, c.ID)
	f.tick(hour)
	decided, err := f.decide(t, c.ID, DecisionActionTaken, "")
	must(t, err)
	wantNoDeadline(t, "結案後的決定期限", decided.DecideDeadline)
	wantDeadline(t, "食安案件結案後的轉報期限", f.caseOf(t, c.ID).FDAReferralDeadline, f.now, f.now.Add(24*hour))
	for _, listed := range f.allCases(t, CaseListInput{}) {
		if listed.ID == c.ID {
			wantDeadline(t, "清單的轉報期限", listed.FDAReferralDeadline, f.now, f.now.Add(24*hour))
		}
	}
	_, err = f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: c.ID, AdminID: 9, Channel: FDAOnlineForm})
	must(t, err)
	wantNoDeadline(t, "記錄轉報後", f.caseOf(t, c.ID).FDAReferralDeadline)

	// 主管機關來函：處理中有期限（收文到登記時存下的 due_at），完成後沒有。
	accountA, _ := f.open(t)
	f.tick(hour)
	order := f.createOrder(t, f.orderIn(AuthorityDataRequest, AuthorityTargetInput{AccountID: accountA}))
	wantDeadline(t, "處理中的來函", order.Deadline, order.ReceivedAt, order.DueAt)
	wantDeadline(t, "處理中的來函（固定值）", order.Deadline, f.now, f.now.Add(72*hour))
	done, err := f.completeOrder(order, nil, f.notifyAll(order))
	must(t, err)
	wantNoDeadline(t, "完成的來函", done.Deadline)
	wantNoDeadline(t, "完成的來函（重讀）", f.orderOf(t, order.ID).Deadline)

	// 措施請求：待處理有裁定期限（7 天），裁定後沒有；複審：進入 review_by 前 24 小時後有，措施結束後沒有。
	f.tick(hour)
	appealAccount, _, appealable := f.appealable(t)
	f.appeal(t, appealAccount, appealable, "不成立")
	submitted := f.now
	admin, err := f.module.Measure(ctx, appealable.ID)
	must(t, err)
	wantDeadline(t, "待處理的申覆", admin.Pending().DecideDeadline, submitted, submitted.Add(7*24*hour))
	wantNoDeadline(t, "申覆的措施沒有複審", admin.ReviewDeadline)
	f.tick(hour)
	_, err = f.module.DecideAppeal(ctx, DecideAppealInput{MeasureID: appealable.ID, AdminID: 9, ExpectedVersion: admin.Version, Outcome: RequestRejected, Reason: "維持"})
	must(t, err)
	after, err := f.module.Measure(ctx, appealable.ID)
	must(t, err)
	for _, r := range after.Requests {
		wantNoDeadline(t, "裁定後的申覆", r.DecideDeadline)
	}

	_, sRisk := f.open(t)
	review := f.now.Add(30 * hour)
	risk, err := f.module.CreateMeasure(ctx, CreateMeasureInput{
		AdminID: 7, StoreID: sRisk.StoreID, Kind: MeasureSuspension, StatementToSeller: "疑似詐欺", Note: "內部紀錄", ReviewBy: &review, Risk: &RiskBasis{Ground: RiskFraud},
	})
	must(t, err)
	wantNoDeadline(t, "review_by 前 24 小時以前", risk.Measure.ReviewDeadline)
	wantNoDeadline(t, "review_by 前 24 小時以前（重讀）", f.measureOf(t, risk.Measure.ID).ReviewDeadline)
	f.tick(7 * hour)
	wantDeadline(t, "進入複審", f.measureOf(t, risk.Measure.ID).ReviewDeadline, review.Add(-24*hour), review)
	ended, err := f.lift(ctx, risk.Measure)
	must(t, err)
	wantNoDeadline(t, "結束的措施沒有複審期限", ended.ReviewDeadline)
	wantNoDeadline(t, "結束的措施沒有複審期限（重讀）", f.measureOf(t, risk.Measure.ID).ReviewDeadline)

	// 食品業者登錄：等商家認證沒有期限；商家認證核准後從核准時間起算 7 天；裁定後沒有。商家認證：審核中有期限（送出起算 7 天），核准後沒有。
	f.tick(hour)
	accountF, sF := f.open(t)
	f.submitFood(t, sF, "D-1")
	reviewOf := func() FoodRegistrationReview {
		got, err := f.module.FoodRegistrationReview(ctx, f.pendingID(t, sF))
		must(t, err)
		return got
	}
	wantNoDeadline(t, "等商家認證的登錄", reviewOf().DecideDeadline)
	f.tick(hour)
	kyc := f.submitKYCFor(t, accountF, 301)
	submittedKYC := f.now
	wantDeadline(t, "審核中的商家認證（送出的回應不帶，重讀）", mustStoreKYC(t, f, sF.StoreID).DecideDeadline, submittedKYC, submittedKYC.Add(7*24*hour))
	wantNoDeadline(t, "商家認證審核中，登錄還不能裁定", reviewOf().DecideDeadline)
	f.tick(hour)
	must(t, approve(f, kyc))
	approvedAt := f.now
	wantNoDeadline(t, "核准後的商家認證", mustStoreKYC(t, f, sF.StoreID).DecideDeadline)
	wantDeadline(t, "商家認證核准後的登錄", reviewOf().DecideDeadline, approvedAt, approvedAt.Add(7*24*hour))
	f.mustDecide(t, f.pendingID(t, sF), FoodRegApproved)
	approvedReview, err := f.module.FoodRegistrationReview(ctx, f.foodIDs(t, sF)[0])
	must(t, err)
	wantNoDeadline(t, "裁定後的登錄", approvedReview.DecideDeadline)
}

func mustStoreKYC(t *testing.T, f *fixture, storeID int64) KYCApplication {
	t.Helper()
	a, ok, err := f.module.StoreKYC(context.Background(), storeID)
	must(t, err)
	if !ok {
		t.Fatalf("店家 %d 沒有商家認證", storeID)
	}
	return a
}

// foodIDs 是店家全部的登錄 ID（新到舊）。
func (f *fixture) foodIDs(t *testing.T, s Store) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	rows, err := f.db.Conn(ctx).Query(ctx, "SELECT id FROM seller.food_registrations WHERE store_id = $1 ORDER BY submitted_at DESC, id DESC", s.StoreID)
	must(t, err)
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		must(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	must(t, rows.Err())
	return ids
}

// 食品業者登錄的「待審核」收件匣依裁定期限由近到遠（期限相同時依 id），不是依送出時間；翻頁（每頁 1、2、3、10 筆）不漏不重。
// 資料集：先送出的登錄最晚核准商家認證（開始最晚）、商家認證先核准的登錄開始是送出時間、兩筆開始時間相同（只能靠 id 定序）。
// mutation：排序改回送出時間、或 keyset 少了 id，順序或翻頁失敗。
func TestFoodRegistrationInboxIsOrderedByDeadlineAndPagesWithoutGapsOrDuplicates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	const hour = time.Hour
	type seed struct {
		number string
		store  Store
		start  time.Time
	}
	var seeds []*seed
	newStore := func(number string) (*seed, int64) {
		accountID, s := f.open(t)
		sd := &seed{number: number, store: s}
		seeds = append(seeds, sd)
		return sd, accountID
	}
	approveKYC := func(accountID int64, serial int) {
		must(t, approve(f, f.submitKYCFor(t, accountID, serial)))
	}
	a, accountA := newStore("A") // 最早送出、商家認證最晚核准 → 開始最晚
	f.submitFood(t, a.store, a.number)
	f.tick(hour)
	e, accountE := newStore("E") // 商家認證核准後送出，開始最早
	approveKYC(accountE, 202)
	f.submitFood(t, e.store, e.number)
	e.start = f.now
	f.tick(hour)
	b, accountB := newStore("B")
	approveKYC(accountB, 203)
	f.submitFood(t, b.store, b.number)
	b.start = f.now
	f.tick(hour)
	c, accountC := newStore("C") // C 與 D：開始時間相同
	approveKYC(accountC, 204)
	f.submitFood(t, c.store, c.number)
	c.start = f.now
	d, accountD := newStore("D")
	approveKYC(accountD, 205)
	f.submitFood(t, d.store, d.number)
	d.start = f.now
	f.tick(hour)
	approveKYC(accountA, 201)
	a.start = f.now

	ids := map[string]uuid.UUID{}
	for _, sd := range seeds {
		ids[sd.number] = f.pendingID(t, sd.store)
	}
	// 預期順序：開始（= 期限 − 7 天）由早到晚，相同時依 id。
	order := []string{"E", "B", "C", "D", "A"}
	if ids["C"].String() > ids["D"].String() {
		order = []string{"E", "B", "D", "C", "A"}
	}
	byNumber := map[string]*seed{"A": a, "E": e, "B": b, "C": c, "D": d}
	// 函式本身依 (期限, id) 排序後的順序必須就是預期（函式與收件匣讀同一份）。
	rows := f.reviewOf(t, "review_food_registration", f.now)
	slices.SortFunc(rows, func(x, y reviewRow) int {
		if c := x.Due.Compare(*y.Due); c != 0 {
			return c
		}
		return strings.Compare(x.ItemID, y.ItemID)
	})
	var fromFunction []string
	for _, r := range rows {
		for number, id := range ids {
			if r.ItemID == id.String() {
				fromFunction = append(fromFunction, number)
			}
		}
	}
	if !slices.Equal(fromFunction, order) {
		t.Fatalf("函式的 (期限, id) 順序 %v，預期 %v", fromFunction, order)
	}
	for _, size := range []int32{1, 2, 3, 10} {
		var got []string
		var after *FoodRegistrationCursor
		for pages := 0; ; pages++ {
			if pages > 10 {
				t.Fatalf("每頁 %d 筆：翻不完", size)
			}
			page, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{PageSize: size, After: after})
			must(t, err)
			for _, item := range page.Items {
				got = append(got, item.RegistrationNumber)
				sd := byNumber[item.RegistrationNumber]
				wantDeadline(t, "登錄 "+item.RegistrationNumber, item.DecideDeadline, sd.start, sd.start.Add(7*24*hour))
			}
			if page.Next == nil {
				break
			}
			after = page.Next
		}
		if !slices.Equal(got, order) {
			t.Errorf("每頁 %d 筆：順序 %v，預期依期限 %v", size, got, order)
		}
	}
	// 等商家認證的隊列照舊依送出時間，沒有期限。
	_, waiting := f.open(t)
	f.submitFood(t, waiting, "W")
	awaiting := FoodRegInboxAwaitingKYC
	page, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{Inbox: &awaiting, PageSize: 10})
	must(t, err)
	if len(page.Items) != 1 || page.Items[0].RegistrationNumber != "W" || page.Items[0].DecideDeadline != nil {
		t.Fatalf("等商家認證的隊列：%+v", page.Items)
	}
}

// inquiryClockRestart 是測試的 oracle：這筆詢問讓案件「待決定」重新起算的時間（回覆收到的時間，或管理員撤回的時間；系統撤回與還沒結束的是 nil）。
// 正式程式的規則只在 SQL 的 seller.review_case_decision，這裡由 fixture 的事實直接寫出。
func inquiryClockRestart(i AdminInquiry) *time.Time {
	switch {
	case i.Reply != nil:
		t := i.Reply.ReceivedAt
		return &t
	case i.Withdrawn != nil && i.Withdrawn.Cause == InquiryWithdrawAdmin:
		t := i.Withdrawn.At
		return &t
	}
	return nil
}

// referralPending 是測試的 oracle：食安案件決定採取措施、還沒有轉報紀錄（標記不轉報也是一筆紀錄）。前置條件取指令用的 Availability.RecordReferral。
func referralPending(c Case) bool {
	return c.Decision != nil && *c.Decision == DecisionActionTaken && len((CaseFacts{Case: c}).Availability().RecordReferral) == 0
}

// 食品業者登錄收件匣的兩個分頁互補：「可以裁定」（讀 review_food_registration）與「等商家認證」（AdminFoodRegistrationAwaitingKYC 自己的條件）沒有交集，
// 聯集正好是全部「待審而且店家營業中」的登錄；店家已關閉與已裁定的兩邊都沒有。
// mutation：任一邊的條件改動（例如等商家認證漏掉店家營運中、審閱函式改了商家認證的定義），聯集就缺或交集就非空。
func TestFoodRegistrationInboxTabsPartitionPendingOfActiveStores(t *testing.T) {
	t.Parallel()
	f := setup(t)
	w := newReviewWorld(t, f)
	f.now = w.at
	ctx := context.Background()
	tab := func(inbox FoodRegInbox) map[string]bool {
		out := map[string]bool{}
		var after *FoodRegistrationCursor
		for {
			page, err := f.module.FoodRegistrations(ctx, FoodRegistrationListInput{Inbox: &inbox, PageSize: 2, After: after})
			must(t, err)
			for _, r := range page.Items {
				if out[r.ID.String()] {
					t.Fatalf("%s：%s 重複", inbox, r.ID)
				}
				out[r.ID.String()] = true
			}
			if page.Next == nil {
				return out
			}
			after = page.Next
		}
	}
	decidable, awaiting := tab(FoodRegInboxDecidable), tab(FoodRegInboxAwaitingKYC)
	all := map[string]bool{}
	rows, err := f.db.Conn(ctx).Query(ctx, `SELECT f.id::text FROM seller.food_registrations f JOIN seller.stores s ON s.store_id = f.store_id
		WHERE f.state = 'PENDING' AND s.status = 'ACTIVE'`)
	must(t, err)
	for rows.Next() {
		var id string
		must(t, rows.Scan(&id))
		all[id] = true
	}
	must(t, rows.Err())
	rows.Close()
	if len(decidable) == 0 || len(awaiting) == 0 || len(all) != len(decidable)+len(awaiting) {
		t.Fatalf("資料集要兩個分頁都有成員：可以裁定 %d、等商家認證 %d、待審且營業中 %d", len(decidable), len(awaiting), len(all))
	}
	for id := range decidable {
		if awaiting[id] {
			t.Errorf("%s 同時在兩個分頁", id)
		}
		if !all[id] {
			t.Errorf("%s 在「可以裁定」，卻不是待審而且店家營業中", id)
		}
	}
	for id := range awaiting {
		if !all[id] {
			t.Errorf("%s 在「等商家認證」，卻不是待審而且店家營業中", id)
		}
	}
	for id := range all {
		if !decidable[id] && !awaiting[id] {
			t.Errorf("%s 待審而且店家營業中，兩個分頁都沒有", id)
		}
	}
	// 「可以裁定」就是審閱函式的成員。
	fn := f.reviewSet(t, "review_food_registration", f.now)
	if len(fn) != len(decidable) {
		t.Errorf("審閱函式 %d 個，可以裁定的分頁 %d 個", len(fn), len(decidable))
	}
	for id := range decidable {
		if _, ok := fn[id]; !ok {
			t.Errorf("%s 在分頁，審閱函式裡沒有", id)
		}
	}
}

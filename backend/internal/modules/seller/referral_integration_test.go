//go:build integration

package seller_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// 轉報 v2（3c-b-2）：追蹤日期、不轉報、主管機關的回覆。

// taipeiDay 是 t 的台北日期，以 UTC 午夜表示（DATE 欄位在 Go 端的形狀）。
func taipeiDay(t time.Time) time.Time { return twtime.Day(t) }

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func (f *fixture) referFood(t *testing.T) AdminCase {
	t.Helper()
	_, s := f.open(t)
	c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityHigh, 1)
	return c
}

// 追蹤日期的預設是轉報日（台北）加 30 天，台北的日界不是 UTC 的日界；明確給的日期不早於今天（台北）。
func TestReferralFollowUpDefaultsToThirtyTaipeiDays(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	record := func(c AdminCase, followUp *time.Time) (RecordFDAReferralResult, error) {
		return f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: c.ID, AdminID: 14, Channel: FDAOnlineForm, FollowUpOn: followUp})
	}
	// 台北 2026-10-03 00:00 的前一秒（UTC 10-02 15:59:59）還是 10-02，加 30 天是 11-01；整點是 10-03，加 30 天是 11-02。
	boundary := time.Date(2026, 10, 3, 0, 0, 0, 0, twtime.Taipei)
	for _, tt := range []struct {
		name string
		at   time.Time
		want time.Time
	}{
		{"台北日界前一秒", boundary.Add(-time.Second), day(2026, 11, 1)},
		{"台北日界", boundary, day(2026, 11, 2)},
	} {
		name := tt.name
		// 有序的清單，而且先設時鐘再開案：案件的 updated_at 不能早於 opened_at（cases_updated_check），兩個案例的時鐘不是遞增時，後開的案件會違反它。
		f.now = tt.at
		c := f.referFood(t)
		r, err := record(c, nil)
		must(t, err)
		got := r.Case.Referral.FDA.FollowUpOn
		if got == nil || !got.Equal(tt.want) {
			t.Errorf("%s：追蹤日期 %v，預期 %v", name, got, tt.want)
		}
	}
	// 明確給的日期：今天可以、昨天不行；被拒絕時什麼都不寫。
	c := f.referFood(t)
	f.now = day(2026, 10, 10).Add(4 * time.Hour) // 台北 10-10 12:00
	_, err := record(c, ptr(day(2026, 10, 9)))
	if fields := invalidFields(t, err); len(fields) != 1 || fields[0] != "FollowUpOn" {
		t.Fatalf("昨天要被拒絕：%v", err)
	}
	if err := recordedNothing(f, c); err != "" {
		t.Fatal(err)
	}
	r, err := record(c, ptr(day(2026, 10, 10)))
	must(t, err)
	if got := r.Case.Referral.FDA.FollowUpOn; got == nil || !got.Equal(day(2026, 10, 10)) {
		t.Fatalf("今天可以：%v", got)
	}
}

func recordedNothing(f *fixture, c AdminCase) string {
	var n int64
	if err := f.db.Pool().QueryRow(context.Background(), "SELECT count(*) FROM seller.case_referrals WHERE case_id = $1", c.ID).Scan(&n); err != nil {
		return err.Error()
	}
	if n != 0 {
		return "被拒絕的轉報寫入了"
	}
	return ""
}

// 不轉報與記錄轉報是同一列，互斥（兩個方向）；決定之前就可以不轉報；不轉報離開待轉報；標記過回原本的紀錄、不動版本。
func TestWaivingTheReferralExcludesRecordingItAndTheOtherWayAround(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	waive := func(c AdminCase, reason string) (WaiveFDAReferralResult, error) {
		return f.module.WaiveFDAReferral(ctx, WaiveFDAReferralInput{CaseID: c.ID, AdminID: 14, Reason: reason})
	}
	record := func(c AdminCase) (RecordFDAReferralResult, error) {
		return f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: c.ID, AdminID: 14, Channel: FDAHotline1919})
	}
	food := f.referFood(t)
	_, s := f.open(t)
	fraud, _ := f.newCase(t, s.StoreID, policy.Fraud, SeverityLow, 1)
	_, err := waive(fraud, "x")
	wantBlocked(t, err, "平台判定類別是食安的案件才能轉報主管機關。")
	_, err = f.module.WaiveFDAReferral(ctx, WaiveFDAReferralInput{CaseID: uuid.NewV7(), AdminID: 14, Reason: "x"})
	wantKind(t, err, apperr.KindNotFound)
	for name, reason := range map[string]string{"空白": "  ", "太長": strings.Repeat("字", 2001)} {
		_, err := waive(food, reason)
		if fields := invalidFields(t, err); len(fields) != 1 || fields[0] != "Reason" {
			t.Errorf("%s：%v", name, fields)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.case_referrals") != 0 {
		t.Fatal("被拒絕的不轉報寫入了")
	}
	f.now = f.now.Add(time.Hour)
	first, err := waive(food, " 主管機關已經介入 ")
	must(t, err)
	w := first.Case.Referral
	if first.AlreadyWaived || !w.Waived() || w.Waiver.Reason != "主管機關已經介入" || w.Waiver.WaivedByAdminID != 14 || !w.Waiver.WaivedAt.Equal(f.now) || w.FDA != nil ||
		first.Case.Version != food.Version+1 || !first.Case.UpdatedAt.Equal(f.now) {
		t.Fatalf("first=%+v", first.Case.Case)
	}
	if len(first.Case.Availability.RecordReferral) == 0 || len(first.Case.Availability.WaiveReferral) == 0 || len(first.Case.Availability.RecordReferralReply) == 0 || len(first.Case.Availability.Export) != 0 {
		t.Fatalf("不轉報之後：記錄、再標記、回覆都不行，匯出照樣可以：%+v", first.Case.Availability)
	}
	_, err = record(food)
	wantBlocked(t, err, "這個案件已標記不轉報。")
	f.now = f.now.Add(time.Hour)
	again, err := waive(food, "另一個原因")
	must(t, err)
	if !again.AlreadyWaived || again.Case.Referral.Waiver.Reason != "主管機關已經介入" || again.Case.Version != first.Case.Version || !again.Case.UpdatedAt.Equal(first.Case.UpdatedAt) {
		t.Fatalf("標記過時回原本的紀錄、不動版本：%+v", again.Case.Case)
	}
	// 反方向：記錄轉報之後不能標記不轉報（錯誤，不是 already）。
	other := f.referFood(t)
	_, err = record(other)
	must(t, err)
	_, err = waive(other, "x")
	wantBlocked(t, err, "這個案件先前已記錄轉報。")
	// 決定採取措施之後，標記不轉報的案件不在待轉報；沒有轉報紀錄的在。
	_, s3 := f.open(t)
	decided, _ := f.newCase(t, s3.StoreID, policy.FoodSafety, SeverityLow, 1)
	f.riskFromCase(t, s3.StoreID, decided.ID)
	_, err = f.decide(t, decided.ID, DecisionActionTaken, "")
	must(t, err)
	if !referralPending(f.caseOf(t, decided.ID).Case) {
		t.Fatal("還沒有轉報紀錄的在待轉報")
	}
	_, err = waive(decided, "不轉報")
	must(t, err)
	if referralPending(f.caseOf(t, decided.ID).Case) {
		t.Fatal("不轉報也是一筆紀錄，離開待轉報")
	}
	// 標記不轉報之後最後一筆通報可以移走（案件空了照樣以 NO_REPORTS 關閉）；已轉報的不行（TestMoveIsBlockedBy...）。
	_, s4 := f.open(t)
	emptied, members := f.newCase(t, s4.StoreID, policy.FoodSafety, SeverityLow, 1)
	_, err = waive(emptied, "不轉報")
	must(t, err)
	if _, err := f.move(t, members[0].ID, policy.Fraud, severityPtr(SeverityLow)); err != nil {
		t.Fatalf("不轉報的案件可以移走最後一筆通報：%v", err)
	}
	f.assertCaseInvariants(t)
}

// 主管機關的回覆：只在已轉報的列；收到的日期在 [轉報日（台北）, 今天（台北）]；追蹤日期依請求更新、清掉或不變；只新增，舊到新。
func TestReferralRepliesAreAppendedOnlyToARecordedReferral(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	reply := func(c AdminCase, in RecordFDAReferralReplyInput) (AdminCase, error) {
		in.CaseID, in.AdminID = c.ID, 21
		if in.Body == "" {
			in.Body = "已收件"
		}
		return f.module.RecordFDAReferralReply(ctx, in)
	}
	food := f.referFood(t)
	_, err := reply(food, RecordFDAReferralReplyInput{ReceivedOn: taipeiDay(f.now)})
	wantBlocked(t, err, "這個案件還沒有記錄轉報。")
	waived := f.referFood(t)
	_, err = f.module.WaiveFDAReferral(ctx, WaiveFDAReferralInput{CaseID: waived.ID, AdminID: 14, Reason: "x"})
	must(t, err)
	_, err = reply(waived, RecordFDAReferralReplyInput{ReceivedOn: taipeiDay(f.now)})
	wantBlocked(t, err, "這個案件已標記不轉報。")
	_, err = f.module.RecordFDAReferralReply(ctx, RecordFDAReferralReplyInput{CaseID: uuid.NewV7(), AdminID: 21, Body: "x", ReceivedOn: taipeiDay(f.now)})
	wantKind(t, err, apperr.KindNotFound)

	// 台北 2026-10-01 12:00 轉報，預設追蹤日期 10-31。
	referred, err := f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: food.ID, AdminID: 14, Channel: FDAHotline1919})
	must(t, err)
	if got := referred.Case.Referral.FDA.FollowUpOn; got == nil || !got.Equal(day(2026, 10, 31)) {
		t.Fatalf("預設追蹤日期：%v", got)
	}
	f.now = f.now.Add(48 * time.Hour) // 台北 10-03 12:00
	for name, tt := range map[string]struct {
		in    RecordFDAReferralReplyInput
		field string
	}{
		"沒有收到的日期":      {RecordFDAReferralReplyInput{}, "ReceivedOn"},
		"早於轉報日":        {RecordFDAReferralReplyInput{ReceivedOn: day(2026, 9, 30)}, "ReceivedOn"},
		"晚於今天":         {RecordFDAReferralReplyInput{ReceivedOn: day(2026, 10, 4)}, "ReceivedOn"},
		"內容太長":         {RecordFDAReferralReplyInput{ReceivedOn: day(2026, 10, 2), Body: strings.Repeat("字", 2001)}, "Body"},
		"追蹤日期早於今天":     {RecordFDAReferralReplyInput{ReceivedOn: day(2026, 10, 2), NextFollowUpOn: ptr(day(2026, 10, 2))}, "NextFollowUpOn"},
		"下一個追蹤日期與不再追蹤": {RecordFDAReferralReplyInput{ReceivedOn: day(2026, 10, 2), NextFollowUpOn: ptr(day(2026, 12, 1)), ClearFollowUp: true}, "ClearFollowUp"},
	} {
		_, err := reply(food, tt.in)
		if fields := invalidFields(t, err); len(fields) != 1 || fields[0] != tt.field {
			t.Errorf("%s：%v", name, fields)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.case_referral_replies") != 0 {
		t.Fatal("被拒絕的回覆寫入了")
	}
	// 轉報當天與今天都可以；追蹤日期不變。
	before := f.caseOf(t, food.ID)
	f.now = f.now.Add(time.Minute)
	c1, err := reply(food, RecordFDAReferralReplyInput{ReceivedOn: day(2026, 10, 1), Body: " 移送衛生局 "})
	must(t, err)
	r1 := c1.Referral.FDA
	if len(r1.Replies) != 1 || r1.Replies[0].Body != "移送衛生局" || !r1.Replies[0].ReceivedOn.Equal(day(2026, 10, 1)) || r1.Replies[0].RecordedByAdminID != 21 ||
		!r1.Replies[0].RecordedAt.Equal(f.now) || r1.FollowUpOn == nil || !r1.FollowUpOn.Equal(day(2026, 10, 31)) || c1.Version != before.Version+1 || !c1.UpdatedAt.Equal(f.now) {
		t.Fatalf("c1=%+v", r1)
	}
	// 取代追蹤日期（今天可以），再記一則；舊到新。
	f.now = f.now.Add(time.Minute)
	c2, err := reply(food, RecordFDAReferralReplyInput{ReceivedOn: day(2026, 10, 3), Body: "結果通知", NextFollowUpOn: ptr(day(2026, 10, 3))})
	must(t, err)
	r2 := c2.Referral.FDA
	if len(r2.Replies) != 2 || r2.Replies[0].Body != "移送衛生局" || r2.Replies[1].Body != "結果通知" || r2.FollowUpOn == nil || !r2.FollowUpOn.Equal(day(2026, 10, 3)) || c2.Version != c1.Version+1 {
		t.Fatalf("c2=%+v", r2)
	}
	// 不再追蹤：清掉；回覆不改轉報本身。
	f.now = f.now.Add(time.Minute)
	c3, err := reply(food, RecordFDAReferralReplyInput{ReceivedOn: day(2026, 10, 3), Body: "結案", ClearFollowUp: true})
	must(t, err)
	r3 := c3.Referral.FDA
	if len(r3.Replies) != 3 || r3.FollowUpOn != nil || r3.Channel != FDAHotline1919 || !r3.ReportedAt.Equal(referred.Case.Referral.FDA.ReportedAt) {
		t.Fatalf("c3=%+v", r3)
	}
	// 清單的讀取也帶回覆（AdminCases 與 Case 同一條路徑）。
	page, err := f.module.AdminCases(ctx, CaseListInput{PageSize: 50})
	must(t, err)
	found := false
	for _, item := range page.Items {
		if item.ID == food.ID {
			found = len(item.Referral.FDA.Replies) == 3
		}
	}
	if !found {
		t.Fatal("清單要帶回覆")
	}
	f.assertCaseInvariants(t)
}

// DB 保證回覆只掛在已轉報（REFERRED）的列：直接寫入不經過模組也一樣；不轉報的列不能有管道、案號、追蹤日期，要有原因。
func TestReferralTablesRejectWhatTheModuleWouldRefuse(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	waived, referred := f.referFood(t), f.referFood(t)
	_, err := f.module.WaiveFDAReferral(ctx, WaiveFDAReferralInput{CaseID: waived.ID, AdminID: 14, Reason: "x"})
	must(t, err)
	_, err = f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: referred.ID, AdminID: 14, Channel: FDAOther})
	must(t, err)
	reply := func(caseID uuid.UUID, outcome, body string, admin int) string {
		return fmt.Sprintf("INSERT INTO seller.case_referral_replies (id, case_id, outcome, body, received_on, recorded_by_admin_id, recorded_at) VALUES (uuidv7(), '%s', '%s', '%s', current_date, %d, now())",
			caseID, outcome, body, admin)
	}
	f.wantViolates(t, "回覆掛在不轉報的列", "case_referral_replies_referral_fkey", reply(waived.ID, "REFERRED", "x", 1))
	f.wantViolates(t, "回覆的 outcome 不是 REFERRED", "case_referral_replies_outcome_check", reply(waived.ID, "WAIVED", "x", 1))
	f.wantViolates(t, "回覆是空字串", "case_referral_replies_body_check", reply(referred.ID, "REFERRED", "", 1))
	f.wantViolates(t, "回覆的管理員是 0", "case_referral_replies_admin_check", reply(referred.ID, "REFERRED", "x", 0))
	_, s := f.open(t)
	c, _ := f.newCase(t, s.StoreID, policy.FoodSafety, SeverityLow, 1)
	insert := func(values string) string {
		return "INSERT INTO seller.case_referrals (case_id, category, outcome, channel, reference_no, follow_up_on, waiver_reason, follow_up_set_at, recorded_at, recorded_by_admin_id) VALUES ('" +
			c.ID.String() + "', 'FOOD_SAFETY', " + values + ", now(), 1)"
	}
	for name, values := range map[string]string{
		"已轉報沒有管道":       "'REFERRED', NULL, NULL, NULL, NULL, NULL",
		"已轉報帶原因":        "'REFERRED', 'OTHER', NULL, NULL, 'x', NULL",
		"不轉報帶管道":        "'WAIVED', 'OTHER', NULL, NULL, 'x', NULL",
		"不轉報帶案號":        "'WAIVED', NULL, 'A1', NULL, 'x', NULL",
		"不轉報帶追蹤日期":      "'WAIVED', NULL, NULL, current_date, 'x', now()",
		"不轉報沒有原因":       "'WAIVED', NULL, NULL, NULL, NULL, NULL",
		"不轉報的原因是空字串":    "'WAIVED', NULL, NULL, NULL, '', NULL",
		"不轉報的原因超過 2000": "'WAIVED', NULL, NULL, NULL, '" + strings.Repeat("x", 2001) + "', NULL",
		"outcome 不認得":   "'DONE', 'OTHER', NULL, NULL, NULL, NULL",
	} {
		f.wantViolates(t, name, "case_referrals_outcome_check", insert(values))
	}
	// 追蹤日期與設定它的時間成對（追蹤逾期只看這個時間之後的回覆）。
	f.wantViolates(t, "追蹤日期沒有設定時間", "case_referrals_follow_up_check", insert("'REFERRED', 'OTHER', NULL, current_date, NULL, NULL"))
	f.wantViolates(t, "設定時間沒有追蹤日期", "case_referrals_follow_up_check", insert("'REFERRED', 'OTHER', NULL, NULL, NULL, now()"))
}

// 主管機關的回覆帶 request_id（AIP-155）：同一位管理員、同樣的內容重送（回應遺失之後再送一次）不會多記一則、案件的版本不再前進；內容不同回 invalid_argument；
// 另一位管理員帶同一個值是他自己的一則。
// mutation：拿掉鎖住案件之後的重送查詢 → 第二次送出撞唯一鍵（或多一則），斷言失敗。
func TestReferralReplyRequestIDMakesAResendSafe(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	c := f.referFood(t)
	_, err := f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: c.ID, AdminID: 14, Channel: FDAOnlineForm})
	must(t, err)
	id := uuid.NewV4()
	in := RecordFDAReferralReplyInput{CaseID: c.ID, AdminID: 21, Body: "已移送", ReceivedOn: taipeiDay(f.now), RequestID: &id}
	first, err := f.module.RecordFDAReferralReply(ctx, in)
	must(t, err)
	f.now = f.now.Add(time.Minute)
	again, err := f.module.RecordFDAReferralReply(ctx, in)
	must(t, err)
	if again.Version != first.Version || len(again.Referral.FDA.Replies) != 1 {
		t.Fatalf("重送不多記一則、版本不前進：version %d → %d，replies=%d", first.Version, again.Version, len(again.Referral.FDA.Replies))
	}
	changed := in
	changed.Body = "不同的內容"
	_, err = f.module.RecordFDAReferralReply(ctx, changed)
	wantInvalid(t, err, "RequestID", "這次送出的內容和同一個請求先前送出的不同")
	other := in
	other.AdminID = 22
	got, err := f.module.RecordFDAReferralReply(ctx, other)
	must(t, err)
	if len(got.Referral.FDA.Replies) != 2 {
		t.Fatalf("另一位管理員帶同一個值是他自己的一則：%d", len(got.Referral.FDA.Replies))
	}
}

// 追蹤逾期由伺服器推導（FDAReferral.FollowUpOverdue）：追蹤日期當天不逾期、隔天（台北）逾期；記下回覆並換成下一個追蹤日期之後不逾期，
// 新的日期過了又逾期（不是「有任何回覆就永遠不逾期」）；回覆而不換日期也算收到回覆；清掉追蹤日期就不逾期。
// mutation：FollowUpOverdue 改成「沒有任何回覆」→ 第二輪逾期的斷言失敗；SetReferralFollowUp 不更新 follow_up_set_at → 同上。
func TestReferralFollowUpOverdueRestartsWithEachNewFollowUpDate(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	c := f.referFood(t)
	r, err := f.module.RecordFDAReferral(ctx, RecordFDAReferralInput{CaseID: c.ID, AdminID: 14, Channel: FDAOnlineForm})
	must(t, err)
	followUp := *r.Case.Referral.FDA.FollowUpOn
	overdue := func(label string, want bool) {
		t.Helper()
		got, err := f.module.Case(ctx, c.ID)
		must(t, err)
		if got.Referral.FDA.FollowUpOverdue(got.ReadAt) != want {
			t.Fatalf("%s：overdue=%v，期望 %v（now=%v follow_up=%v）", label, !want, want, f.now, got.Referral.FDA.FollowUpOn)
		}
	}
	f.now = time.Date(followUp.Year(), followUp.Month(), followUp.Day(), 23, 59, 0, 0, twtime.Taipei)
	overdue("追蹤日期當天", false)
	f.now = f.now.Add(2 * time.Minute)
	overdue("追蹤日期隔天（台北）", true)

	next := taipeiDay(f.now).AddDate(0, 0, 10)
	_, err = f.module.RecordFDAReferralReply(ctx, RecordFDAReferralReplyInput{CaseID: c.ID, AdminID: 21, Body: "已移送", ReceivedOn: taipeiDay(f.now), NextFollowUpOn: &next})
	must(t, err)
	overdue("換成下一個追蹤日期之後", false)
	f.now = time.Date(next.Year(), next.Month(), next.Day()+1, 9, 0, 0, 0, twtime.Taipei)
	overdue("下一個追蹤日期過了、之後沒有回覆", true)

	_, err = f.module.RecordFDAReferralReply(ctx, RecordFDAReferralReplyInput{CaseID: c.ID, AdminID: 21, Body: "結果通知", ReceivedOn: taipeiDay(f.now)})
	must(t, err)
	overdue("設定之後收到回覆（不換日期）", false)

	_, err = f.module.RecordFDAReferralReply(ctx, RecordFDAReferralReplyInput{CaseID: c.ID, AdminID: 21, Body: "結案", ReceivedOn: taipeiDay(f.now), ClearFollowUp: true})
	must(t, err)
	overdue("清掉追蹤日期", false)
}

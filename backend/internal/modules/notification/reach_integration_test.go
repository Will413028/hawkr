//go:build integration

package notification

import (
	"context"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/deliver"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/reach"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var reachClock = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

var resendActor = adminaudit.Actor{AdminID: 7, Action: "/hawkr.admin.notification.v1alpha1.NotificationAdminService/ResendDeliveries"}

func (f *fixture) reachService() *reach.Service {
	return reach.New(reach.Deps{
		DB: f.db, Audit: adminaudit.New(f.db, func() time.Time { return reachClock }), Jobs: f.jobs, Registry: testRegistry{},
		Now: func() time.Time { return reachClock }, NewID: uuid.NewV7,
	})
}

func (f *fixture) seedIntent(t *testing.T, typ string, created time.Time) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	f.mustExec(t, `INSERT INTO notification.intents (id, type, idempotency_key, params, occurred_at, created_at) VALUES ($1, $2, $3, '{}', $4, $4)`, id, typ, typ+":"+id.String(), created)
	return id
}

func (f *fixture) seedDelivery(t *testing.T, intent uuid.UUID, account int64, channel, status, reason string, updated time.Time) uuid.UUID {
	t.Helper()
	id := uuid.NewV7()
	var r *string
	if reason != "" {
		r = &reason
	}
	f.mustExec(t, `INSERT INTO notification.deliveries (id, intent_id, account_id, audience, channel, status, failure_reason, created_at, updated_at, sent_at)
		VALUES ($1, $2, $3, 'BUYER', $4, $5, $6, $7, $7, CASE WHEN $5 = 'SENT' THEN $7::timestamptz END)`, id, intent, account, channel, status, r, updated)
	return id
}

func (f *fixture) stateKey(t *testing.T, id uuid.UUID) string {
	t.Helper()
	s := f.state(t, id)
	reason := "-"
	if s.Reason != nil {
		reason = *s.Reason
	}
	return s.Status + "/" + reason
}

func (f *fixture) auditCount(t *testing.T) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM adminaudit.records`)
}

func (f *fixture) resendWindow(in reach.ResendInput) reach.ResendInput {
	in.Actor = resendActor
	in.FailStart, in.FailEnd = reachClock.Add(-48*time.Hour), reachClock.Add(-time.Minute)
	return in
}

// 重送只動永久失敗且原因可重送、落在失敗時間窗內的投遞：改回待送、清掉失敗原因、同一個 transaction 排一個與原 job 不同鍵的渠道 job；
// 已送出、待送、不可重送的原因（來源事實已不成立、停寄名單、收件人已刪除）與窗外的一律不動；第二次呼叫什麼都不再選中；dry run 不改資料、不排 job、不寫稽核紀錄。
func TestResendRequeuesOnlyEligibleFailedDeliveriesOnce(t *testing.T) {
	f := setupIntake(t)
	svc := f.reachService()
	intent := f.seedIntent(t, "account_suspended", reachClock.Add(-6*time.Hour))
	failedAt := reachClock.Add(-3 * time.Hour)
	eligible := []uuid.UUID{
		f.seedDelivery(t, intent, 1, "EMAIL", "FAILED_PERMANENT", deliver.ReasonNoEmailAddress, failedAt),
		f.seedDelivery(t, intent, 2, "PUSH", "FAILED_PERMANENT", deliver.ReasonNoDeviceToken, failedAt.Add(time.Minute)),
		f.seedDelivery(t, intent, 3, "EMAIL", "FAILED_PERMANENT", deliver.ReasonRetriesExhausted, failedAt.Add(2*time.Minute)),
	}
	untouched := map[string]uuid.UUID{
		"no_longer_valid":  f.seedDelivery(t, intent, 4, "EMAIL", "FAILED_PERMANENT", deliver.ReasonNoLongerValid, failedAt),
		"email_suppressed": f.seedDelivery(t, intent, 5, "EMAIL", "FAILED_PERMANENT", deliver.ReasonSuppressed, failedAt),
		"recipient_gone":   f.seedDelivery(t, intent, 6, "EMAIL", "FAILED_PERMANENT", deliver.ReasonRecipientGone, failedAt),
		"render_failed":    f.seedDelivery(t, intent, 7, "EMAIL", "FAILED_PERMANENT", deliver.ReasonRenderFailed, failedAt),
		"sent":             f.seedDelivery(t, intent, 8, "EMAIL", "SENT", "", failedAt),
		"pending":          f.seedDelivery(t, intent, 9, "EMAIL", "PENDING", "", failedAt),
		"outside window":   f.seedDelivery(t, intent, 10, "EMAIL", "FAILED_PERMANENT", deliver.ReasonRetriesExhausted, reachClock.Add(-72*time.Hour)),
	}
	before := map[uuid.UUID]string{}
	for _, id := range untouched {
		before[id] = f.stateKey(t, id)
	}

	dry, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{DryRun: true}))
	if err != nil || dry.Count != 3 || dry.HasMore {
		t.Fatalf("dry run = %+v, %v", dry, err)
	}
	if jobs, _ := f.jobs.deliverJobs(); len(jobs) != 0 || f.auditCount(t) != 0 || f.state(t, eligible[0]).Status != "FAILED_PERMANENT" {
		t.Fatalf("dry run 不該改資料、排 job 或寫稽核：jobs=%d audit=%d", len(jobs), f.auditCount(t))
	}

	res, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{}))
	if err != nil || res.Count != 3 || res.HasMore {
		t.Fatalf("resend = %+v, %v", res, err)
	}
	for _, id := range eligible {
		if s := f.state(t, id); s.Status != "PENDING" || s.Reason != nil {
			t.Fatalf("重送後 %s = %+v，預期 PENDING 且沒有失敗原因", id, s)
		}
	}
	for name, id := range untouched {
		if got := f.stateKey(t, id); got != before[id] {
			t.Errorf("%s 被改動：%s → %s", name, before[id], got)
		}
	}
	jobs, unique := f.jobs.deliverJobs()
	if len(jobs) != 3 || slices.Contains(unique, false) {
		t.Fatalf("排了 %d 個渠道 job（唯一鍵 %v），預期 3 個且都以參數唯一", len(jobs), unique)
	}
	for _, j := range jobs {
		if !slices.Contains(eligible, j.DeliveryID) || j.RequeuedAt != reachClock.UnixMicro() {
			t.Errorf("渠道 job %+v 不是重送排的", j)
		}
	}
	if f.auditCount(t) != 1 {
		t.Fatalf("稽核紀錄 %d 筆，預期 1 筆", f.auditCount(t))
	}

	again, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{}))
	if err != nil || again.Count != 0 {
		t.Fatalf("第二次 = %+v, %v，預期沒有投遞再被選中", again, err)
	}
	if jobs, _ := f.jobs.deliverJobs(); len(jobs) != 3 {
		t.Fatalf("第二次又排了 job：%d", len(jobs))
	}
}

// 批次上限：MaxCount 限制一次的筆數並回報還有更多（依失敗時間由舊到新）。
func TestResendHonorsTheBatchLimit(t *testing.T) {
	f := setupIntake(t)
	svc := f.reachService()
	account := f.seedIntent(t, "account_suspended", reachClock.Add(-6*time.Hour))
	like := f.seedIntent(t, "test_push", reachClock.Add(-6*time.Hour))
	failedAt := reachClock.Add(-3 * time.Hour)
	old := f.seedDelivery(t, account, 1, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, failedAt)
	mid := f.seedDelivery(t, account, 2, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, failedAt.Add(time.Minute))
	last := f.seedDelivery(t, like, 3, "PUSH", "FAILED_PERMANENT", deliver.ReasonPushRejected, failedAt.Add(2*time.Minute))

	r1, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{MaxCount: 2}))
	if err != nil || r1.Count != 2 || !r1.HasMore {
		t.Fatalf("第一批 = %+v, %v", r1, err)
	}
	if f.state(t, old).Status != "PENDING" || f.state(t, mid).Status != "PENDING" || f.state(t, last).Status != "FAILED_PERMANENT" {
		t.Fatal("第一批應只取失敗時間最舊的兩筆")
	}
	r2, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{MaxCount: 2}))
	if err != nil || r2.Count != 1 || r2.HasMore || f.state(t, last).Status != "PENDING" {
		t.Fatalf("第二批 = %+v, %v", r2, err)
	}

}

// 條件：分類、渠道、收件人與失敗原因篩選只選符合的；不可重送的原因與 INBOX 渠道回 InvalidInput。
func TestResendFiltersSelectOnlyMatchingDeliveries(t *testing.T) {
	f2 := setupIntake(t)
	svc2 := f2.reachService()
	failedAt := reachClock.Add(-3 * time.Hour)
	acct := f2.seedIntent(t, "account_suspended", reachClock.Add(-6*time.Hour))
	lk := f2.seedIntent(t, "test_push", reachClock.Add(-6*time.Hour))
	a := f2.seedDelivery(t, acct, 1, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, failedAt)
	b := f2.seedDelivery(t, acct, 2, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, failedAt)
	c := f2.seedDelivery(t, lk, 1, "PUSH", "FAILED_PERMANENT", deliver.ReasonPushRejected, failedAt)
	d := f2.seedDelivery(t, acct, 3, "EMAIL", "FAILED_PERMANENT", deliver.ReasonNoEmailAddress, failedAt)
	for name, tc := range map[string]struct {
		in   reach.ResendInput
		want []uuid.UUID
	}{
		"分類":   {reach.ResendInput{Category: model.CategoryAccount}, []uuid.UUID{a, b, d}},
		"渠道":   {reach.ResendInput{Channel: model.ChannelPush}, []uuid.UUID{c}},
		"收件人":  {reach.ResendInput{AccountID: 1}, []uuid.UUID{a, c}},
		"失敗原因": {reach.ResendInput{Reasons: []string{deliver.ReasonNoEmailAddress}}, []uuid.UUID{d}},
	} {
		in := f2.resendWindow(tc.in)
		in.DryRun = true
		res, err := svc2.Resend(context.Background(), in)
		if err != nil || res.Count != len(tc.want) {
			t.Errorf("%s：%+v, %v，預期 %d 筆", name, res, err, len(tc.want))
		}
	}
	for name, in := range map[string]reach.ResendInput{
		"不可重送的原因": {Reasons: []string{deliver.ReasonNoLongerValid}},
		"渠道是收件匣":  {Channel: model.ChannelInbox},
	} {
		if _, err := svc2.Resend(context.Background(), f2.resendWindow(in)); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%s：%v", name, err)
		}
	}
}

// 重送是把投遞排回渠道 job，送出仍走全部檢查：重送後收件人有了 token 就送出一次，已送出的再重送不會選中、再執行 job 也不重送；
// 收件人在重送前已刪除時，投遞被重排但送出前仍標永久失敗（recipient_gone），provider 沒被呼叫。
func TestResentDeliveryIsSentOnceAndNeverToADeletedRecipient(t *testing.T) {
	f := setupIntake(t)
	svc := f.reachService()
	live, gone := f.newAccount(t), f.newAccount(t)
	f.addToken(t, live, "BUYER", "tok-live")
	f.addToken(t, gone, "BUYER", "tok-gone")
	intent := f.seedIntent(t, "test_push", reachClock.Add(-6*time.Hour))
	f.mustExec(t, `UPDATE notification.intents SET params = $2 WHERE id = $1`, intent, `{"account_id": 1, "event_id": "e-resend"}`)
	failedAt := reachClock.Add(-3 * time.Hour)
	liveID := f.seedDelivery(t, intent, live, "PUSH", "FAILED_PERMANENT", deliver.ReasonNoDeviceToken, failedAt)
	goneID := f.seedDelivery(t, intent, gone, "PUSH", "FAILED_PERMANENT", deliver.ReasonNoDeviceToken, failedAt)
	if err := f.identity.DeleteAccount(context.Background(), gone); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{}))
	if err != nil || res.Count != 2 {
		t.Fatalf("resend = %+v, %v", res, err)
	}
	for _, id := range []uuid.UUID{liveID, goneID} {
		if err := f.deliver(id, false); err != nil {
			t.Fatal(err)
		}
	}
	if s := f.state(t, liveID); s.Status != "SENT" {
		t.Fatalf("在世收件人 = %+v，預期 SENT", s)
	}
	if s := f.state(t, goneID); s.Status != "FAILED_PERMANENT" || s.Reason == nil || *s.Reason != deliver.ReasonRecipientGone || s.Title != nil {
		t.Fatalf("已刪除的收件人 = %+v，預期 recipient_gone 且沒有送出文字", s)
	}
	if n := f.push.Delivered(); n != 1 {
		t.Fatalf("推播送出 %d 次，預期 1 次", n)
	}
	again, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{}))
	if err != nil || again.Count != 0 {
		t.Fatalf("已送出與 recipient_gone 的不該再被選中：%+v, %v", again, err)
	}
	if err := f.deliver(liveID, false); err != nil {
		t.Fatal(err)
	}
	if n := f.push.Delivered(); n != 1 {
		t.Fatalf("重複執行 job 後送出 %d 次，預期仍是 1 次", n)
	}
}

// 失敗時間窗以失敗時間（updated_at）計：重送後又失敗的投遞晚於窗的終點，依 has_more 連續呼叫不會重複選到它。
func TestResentDeliveryThatFailsAgainIsNotSelectedAgainInTheSameWindow(t *testing.T) {
	f := setupIntake(t)
	svc := f.reachService()
	intent := f.seedIntent(t, "account_suspended", reachClock.Add(-6*time.Hour))
	id := f.seedDelivery(t, intent, 1, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, reachClock.Add(-3*time.Hour))
	if res, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{})); err != nil || res.Count != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	f.mustExec(t, `UPDATE notification.deliveries SET status = 'FAILED_PERMANENT', failure_reason = 'email_rejected' WHERE id = $1`, id)
	if res, err := svc.Resend(context.Background(), f.resendWindow(reach.ResendInput{})); err != nil || res.Count != 0 {
		t.Fatalf("再失敗的投遞不該在同一個窗被選中：%+v, %v", res, err)
	}
}

// 未觸達：近 N 天交易必要型別的通知，某個收件人的所有投遞都永久失敗才算。任一渠道送出、還在待送、刻意不送、收件人已刪除、非交易型別、窗外、沒有投遞紀錄的都不算；
// 只有收件匣列的不算觸達。回應只有參考 ID，分頁依意圖建立時間新到舊。
func TestUnreachedCountsOnlyTransactionalRecipientsWithoutAnySuccessfulChannel(t *testing.T) {
	f := setupIntake(t)
	svc := f.reachService()
	failedAt := reachClock.Add(-2 * time.Hour)
	x := f.seedIntent(t, "account_suspended", reachClock.Add(-24*time.Hour))
	unreachedBoth := f.seedDelivery(t, x, 1, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, failedAt)
	f.seedDelivery(t, x, 1, "PUSH", "FAILED_PERMANENT", deliver.ReasonNoDeviceToken, failedAt)
	f.seedDelivery(t, x, 2, "EMAIL", "SENT", "", failedAt)
	f.seedDelivery(t, x, 3, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, failedAt)
	f.seedDelivery(t, x, 3, "PUSH", "SENT", "", failedAt)
	inboxOnly := f.seedDelivery(t, x, 4, "EMAIL", "FAILED_PERMANENT", deliver.ReasonNoEmailAddress, failedAt)
	f.mustExec(t, `INSERT INTO notification.inbox_items (id, account_id, audience, type, params, occurred_at, created_at, updated_at) VALUES ($1, 4, 'BUYER', 'account_suspended', '{}', $2, $2, $2)`, uuid.NewV7(), failedAt)
	f.seedDelivery(t, x, 5, "EMAIL", "PENDING", "", failedAt)
	f.seedDelivery(t, x, 6, "EMAIL", "FAILED_PERMANENT", deliver.ReasonNoLongerValid, failedAt)
	f.seedDelivery(t, x, 7, "EMAIL", "FAILED_PERMANENT", deliver.ReasonRecipientGone, failedAt)
	like := f.seedIntent(t, "test_push", reachClock.Add(-24*time.Hour))
	f.seedDelivery(t, like, 8, "PUSH", "FAILED_PERMANENT", deliver.ReasonPushRejected, failedAt)
	f.seedIntent(t, "account_suspended", reachClock.Add(-24*time.Hour))
	old := f.seedIntent(t, "account_suspended", reachClock.Add(-30*24*time.Hour))
	f.seedDelivery(t, old, 9, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, reachClock.Add(-30*24*time.Hour))
	newer := f.seedIntent(t, "account_unsuspended", reachClock.Add(-12*time.Hour))
	newerRef := f.seedDelivery(t, newer, 10, "EMAIL", "FAILED_PERMANENT", deliver.ReasonEmailRejected, failedAt)

	page, err := svc.Unreached(context.Background(), reach.UnreachedInput{Days: 7})
	if err != nil || page.Count != 3 || len(page.Notifications) != 3 || page.Next != nil {
		t.Fatalf("7 天 = %+v, %v，預期 3 筆（收件人 1、4、10）", page, err)
	}
	refs := []uuid.UUID{page.Notifications[0].DeliveryID, page.Notifications[1].DeliveryID, page.Notifications[2].DeliveryID}
	if refs[0] != newerRef || !slices.Contains(refs, inboxOnly) || !slices.Contains(refs, unreachedBoth) {
		t.Fatalf("明細 = %v，預期新到舊且含收件人 1（兩個渠道）與只有收件匣的收件人 4", refs)
	}
	first := page.Notifications[0]
	if first.Type != "account_unsuspended" || first.Category != model.CategoryAccount || first.AccountID != 10 || first.Audience != model.AudienceBuyer || len(first.Channels) != 1 {
		t.Fatalf("第一筆 = %+v", first)
	}
	var both reach.Notification
	for _, n := range page.Notifications {
		if n.AccountID == 1 {
			both = n
		}
	}
	if len(both.Channels) != 2 || both.Channels[0].Channel != model.ChannelEmail || both.Channels[1].Channel != model.ChannelPush || both.Channels[1].Reason != deliver.ReasonNoDeviceToken {
		t.Fatalf("收件人 1 的渠道 = %+v", both.Channels)
	}

	p1, err := svc.Unreached(context.Background(), reach.UnreachedInput{Days: 7, PageSize: 2})
	if err != nil || p1.Count != 3 || len(p1.Notifications) != 2 || p1.Next == nil {
		t.Fatalf("第一頁 = %+v, %v", p1, err)
	}
	p2, err := svc.Unreached(context.Background(), reach.UnreachedInput{Days: 7, PageSize: 2, After: p1.Next})
	if err != nil || len(p2.Notifications) != 1 || p2.Next != nil || p2.Notifications[0].DeliveryID == p1.Notifications[0].DeliveryID || p2.Notifications[0].DeliveryID == p1.Notifications[1].DeliveryID {
		t.Fatalf("第二頁 = %+v, %v", p2, err)
	}
	wide, err := svc.Unreached(context.Background(), reach.UnreachedInput{Days: 60})
	if err != nil || wide.Count != 4 {
		t.Fatalf("60 天 = %+v, %v，預期含 30 天前的那筆共 4 筆", wide, err)
	}
	if _, err := svc.Unreached(context.Background(), reach.UnreachedInput{Days: -1}); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("天數為負：%v", err)
	}
}

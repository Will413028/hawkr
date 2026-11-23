//go:build integration

package payment_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	. "github.com/Will413028/hawkr/backend/internal/modules/payment"
)

const kindChargeCancel = "payment.charge_cancel"

func (f *chargeFixture) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	_, err := f.db.Conn(ctx).Exec(ctx, sql, args...)
	must(t, err)
}

// attemptIDs 是某個種類的 job 排入過的嘗試 ID（依排入順序，不去重）。
func (f *chargeFixture) attemptIDs(t *testing.T, kind string) []string {
	t.Helper()
	f.jobs.mu.Lock()
	defer f.jobs.mu.Unlock()
	var out []string
	for _, a := range f.jobs.args {
		if a.Kind() != kind {
			continue
		}
		raw, err := json.Marshal(a)
		must(t, err)
		var v struct {
			AttemptID string `json:"attempt_id"`
		}
		must(t, json.Unmarshal(raw, &v))
		out = append(out, v.AttemptID)
	}
	return out
}

func (f *chargeFixture) attemptOf(t *testing.T, external string) string {
	t.Helper()
	return f.text(t, "SELECT id::text FROM payment.payment_attempts WHERE external_payment_id = $1", external)
}

func (f *chargeFixture) effectiveExternal(t *testing.T, orderNo string) string {
	t.Helper()
	return f.text(t, "SELECT a.external_payment_id FROM payment.payment_attempts a JOIN payment.payments p ON p.id = a.payment_id WHERE p.order_no = $1 AND a.superseded_at IS NULL", orderNo)
}

// repay 把付款建立、讓舊的 PaymentIntent 在 provider 那邊取消並同步到本地、再讓買家重付：回傳付款、舊與新的外部 ID。
func (f *chargeFixture) repay(t *testing.T, orderNo string) (Payment, string, string) {
	t.Helper()
	f.order(orderNo, 11, 7, 100)
	p := f.create(t, 11, orderNo)
	oldExt := f.externalOf(t, orderNo)
	must(t, f.fake.Settle(oldExt, PaymentStateCanceled, f.now))
	f.sync(t, p.ID)
	if f.statusOf(t, orderNo) != PaymentCanceled {
		t.Fatalf("狀態=%s", f.statusOf(t, orderNo))
	}
	again := f.create(t, 11, orderNo)
	newExt := f.externalOf(t, orderNo)
	if again.ID != p.ID || newExt == "" || newExt == oldExt {
		t.Fatalf("重付=%+v 外部 ID %s → %s", again, oldExt, newExt)
	}
	return p, oldExt, newExt
}

// 新建付款的列帶著說明寫進去（payments_description_check 不接受空字串），同時寫第一個有效的嘗試，冪等鍵與列上的相同。
// mutation：ensureRow 的 InsertPayment 拿掉 Description → 每筆新付款的 INSERT 違反 CHECK，建立失敗。
func TestANewPaymentIsWrittenWithItsDescriptionAndItsFirstAttempt(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.orders.put(PaymentOrder{OrderNo: "O-N1", BuyerID: 11, StoreID: 7, Amount: 100, Payable: true, Description: "藍色 T 恤"})
	f.fake.FailCreate(errors.New("逾時"))
	p := f.create(t, 11, "O-N1")
	if p.Status != PaymentCreating || f.text(t, "SELECT description FROM payment.payments WHERE order_no = 'O-N1'") != "藍色 T 恤" {
		t.Fatalf("付款=%+v", p)
	}
	if f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE payment_id = $1 AND superseded_at IS NULL AND idempotency_key::text = $2", p.ID, f.keyOf(t, "O-N1")) != 1 {
		t.Fatal("沒有帶著列上冪等鍵的有效嘗試")
	}
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if f.statusOf(t, "O-N1") != PaymentAwaitingPayment || f.effectiveExternal(t, "O-N1") != f.externalOf(t, "O-N1") {
		t.Fatalf("補做之後狀態=%s", f.statusOf(t, "O-N1"))
	}
}

// 重付之後，舊 PaymentIntent 晚到成功：事件依外部 ID 經歷程表對回付款列，付款列記成功（成功優先於取消）並發事件，
// 舊嘗試成為有效的、新的 PaymentIntent 排取消（job，transaction 外呼叫 provider）；事件標記已處理。
// mutation：PaymentByExternal 改回只比付款列上的外部 ID → 舊 ID 對不到列、事件被丟棄，付款列沒有入帳，失敗。
func TestALateSuccessOfTheOldIntentAfterARepayIsRecordedAndTheNewIntentIsCanceled(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, oldExt, newExt := f.repay(t, "O-R2a")
	must(t, f.fake.Settle(oldExt, PaymentStateSucceeded, f.now))
	f.receive(t, "evt_old", oldExt)
	if got := f.paymentIDs(t, kindChargeSync); !containsID(got, p.ID) {
		t.Fatalf("舊 ID 的事件沒有對回付款列：%v", got)
	}
	f.sync(t, p.ID)
	if f.statusOf(t, "O-R2a") != PaymentSucceeded || f.externalOf(t, "O-R2a") != oldExt || f.effectiveExternal(t, "O-R2a") != oldExt || f.changesTo(PaymentSucceeded) != 1 || !f.processed(t, "evt_old") {
		t.Fatalf("狀態=%s 外部 ID=%s 有效=%s 事件 %d 次 已處理=%v", f.statusOf(t, "O-R2a"), f.externalOf(t, "O-R2a"), f.effectiveExternal(t, "O-R2a"), f.changesTo(PaymentSucceeded), f.processed(t, "evt_old"))
	}
	if f.text(t, "SELECT external_charge_id FROM payment.payments WHERE order_no = 'O-R2a'") == "" || f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE payment_id = $1 AND superseded_at IS NULL", p.ID) != 1 {
		t.Fatal("入帳沒有記 charge，或有效的嘗試不是一個")
	}
	cancels := slices.Compact(slices.Sorted(slices.Values(f.attemptIDs(t, kindChargeCancel))))
	newAttempt := f.attemptOf(t, newExt)
	if len(cancels) != 1 || cancels[0] != newAttempt {
		t.Fatalf("取消 job=%v，新嘗試=%s", cancels, newAttempt)
	}
	must(t, f.module.CancelPaymentAttempt(ctx, uuid.MustParse(newAttempt)))
	if _, _, cancelCalls := f.fake.Calls(); cancelCalls != 1 {
		t.Fatalf("取消呼叫 %d 次", cancelCalls)
	}
	if f.text(t, "SELECT state FROM payment.payment_attempts WHERE id = $1", newAttempt) != "CANCELED" || f.statusOf(t, "O-R2a") != PaymentSucceeded || f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE excess_paid_at IS NOT NULL") != 0 {
		t.Fatal("取消之後新嘗試應是 CANCELED、付款列仍入帳、沒有重複入帳")
	}
	f.noViolations(t)
}

// 舊 PaymentIntent 晚到成功被採用的同一次同步裡，原本有效的新 PaymentIntent 就要排取消：不能靠之後再讀到新 PaymentIntent 的快照才補排
// （那條路徑只在新的還是等待付款或處理中才會排）。這裡新 PaymentIntent 在 provider 那邊已是 canceled，讀它的快照不會觸發取消，
// 取消 job 只可能來自採用舊嘗試的那一步（採用時嘗試列上新嘗試還沒結案）。
// mutation：applyLate 的 lateAdopt 拿掉「原有效嘗試排取消」→ 沒有任何取消 job，失敗。
func TestAdoptingTheLateSuccessOfTheOldIntentSchedulesTheNewIntentCancelInTheSameSync(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	p, oldExt, newExt := f.repay(t, "O-R2d")
	must(t, f.fake.Settle(newExt, PaymentStateCanceled, f.now))
	must(t, f.fake.Settle(oldExt, PaymentStateSucceeded, f.now))
	f.receive(t, "evt_old_adopt", oldExt)
	f.sync(t, p.ID)
	if f.statusOf(t, "O-R2d") != PaymentSucceeded || f.effectiveExternal(t, "O-R2d") != oldExt {
		t.Fatalf("狀態=%s 有效=%s", f.statusOf(t, "O-R2d"), f.effectiveExternal(t, "O-R2d"))
	}
	cancels := slices.Compact(slices.Sorted(slices.Values(f.attemptIDs(t, kindChargeCancel))))
	newAttempt := f.attemptOf(t, newExt)
	if len(cancels) != 1 || cancels[0] != newAttempt {
		t.Fatalf("取消 job=%v，新嘗試=%s", cancels, newAttempt)
	}
}

// 兩個 PaymentIntent 都成功：付款列保持第一個入帳的，第二個記成重複入帳（後台查得到、記 error log），不發第二次入帳事件，不自動退款（沒有退款列）。
// mutation：lateOutcomeOf 把「列已入帳又成功」當成採用 → 第二筆覆蓋第一筆，失敗。
func TestTwoSucceededIntentsAreFlaggedForTheBackOfficeAndNeverRefundedAutomatically(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, oldExt, newExt := f.repay(t, "O-R2b")
	must(t, f.fake.Settle(newExt, PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	must(t, f.fake.Settle(oldExt, PaymentStateSucceeded, f.now))
	f.receive(t, "evt_excess", oldExt)
	f.sync(t, p.ID)
	if f.statusOf(t, "O-R2b") != PaymentSucceeded || f.externalOf(t, "O-R2b") != newExt || f.changesTo(PaymentSucceeded) != 1 || !f.processed(t, "evt_excess") {
		t.Fatalf("狀態=%s 外部 ID=%s 入帳事件 %d 次 已處理=%v", f.statusOf(t, "O-R2b"), f.externalOf(t, "O-R2b"), f.changesTo(PaymentSucceeded), f.processed(t, "evt_excess"))
	}
	excess, err := f.module.ExcessPaidPayments(ctx, uuid.UUID{}, 10)
	must(t, err)
	if len(excess) != 1 || excess[0].PaymentID != p.ID || excess[0].ExternalPaymentID != oldExt || excess[0].AttemptID.String() != f.attemptOf(t, oldExt) {
		t.Fatalf("重複入帳=%+v", excess)
	}
	if f.count(t, "SELECT count(*) FROM payment.refunds") != 0 || len(f.attemptIDs(t, kindChargeCancel)) != 0 || !strings.Contains(f.logs.String(), "重複入帳") {
		t.Fatal("不該自動退款或取消，而且要記 error log")
	}
	f.sync(t, p.ID)
	if f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE excess_paid_at IS NOT NULL") != 1 || f.changesTo(PaymentSucceeded) != 1 {
		t.Fatal("重複同步不該改變結果")
	}
}

// 取消與入帳競態：舊 PaymentIntent 晚到成功、新的排了取消，取消送出前新的也入帳了：取消的結果是 provider 的入帳快照，記成重複入帳而不是取消。
func TestACancelThatLosesTheRaceToAPaymentMarksTheExcessPayment(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, oldExt, newExt := f.repay(t, "O-R2c")
	must(t, f.fake.Settle(oldExt, PaymentStateSucceeded, f.now))
	f.receive(t, "evt_race", oldExt)
	f.sync(t, p.ID)
	must(t, f.fake.Settle(newExt, PaymentStateSucceeded, f.now))
	must(t, f.module.CancelPaymentAttempt(ctx, uuid.MustParse(f.attemptOf(t, newExt))))
	if f.statusOf(t, "O-R2c") != PaymentSucceeded || f.externalOf(t, "O-R2c") != oldExt || f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE external_payment_id = $1 AND excess_paid_at IS NOT NULL", newExt) != 1 {
		t.Fatalf("狀態=%s 外部 ID=%s", f.statusOf(t, "O-R2c"), f.externalOf(t, "O-R2c"))
	}
	if f.count(t, "SELECT count(*) FROM payment.refunds") != 0 {
		t.Fatal("不自動退款")
	}
}

// 舊 PaymentIntent 的同步不能把舊 ID 寫回剛重設的列：重付後新的建立還沒有結果（列是 CREATING、沒有外部 ID），讀到舊 PaymentIntent 的快照只更新歷程表，付款列不動。
// mutation：applySnapshot 拿掉「被取代的嘗試只更新嘗試列」的分支（一律寫付款列）→ 付款列被寫上舊外部 ID，失敗。
func TestAnOldIntentSyncNeverWritesItsIDBackOntoAResetRow(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-R3", 11, 7, 100)
	p := f.create(t, 11, "O-R3")
	oldExt := f.externalOf(t, "O-R3")
	must(t, f.fake.Settle(oldExt, PaymentStateCanceled, f.now))
	f.sync(t, p.ID)
	f.fake.FailCreate(errors.New("逾時"))
	if again := f.create(t, 11, "O-R3"); again.Status != PaymentCreating {
		t.Fatalf("重付的建立應是 pending：%+v", again)
	}
	must(t, f.fake.FailAttempt(oldExt, "card_declined"))
	f.receive(t, "evt_stale_old", oldExt)
	f.sync(t, p.ID)
	if f.statusOf(t, "O-R3") != PaymentCreating || f.externalOf(t, "O-R3") != "" || f.effectiveExternal(t, "O-R3") != "" {
		t.Fatalf("付款列被舊 PaymentIntent 改寫：狀態=%s 外部 ID=%q", f.statusOf(t, "O-R3"), f.externalOf(t, "O-R3"))
	}
	if f.text(t, "SELECT state FROM payment.payment_attempts WHERE external_payment_id = $1", oldExt) != "AWAITING_PAYMENT" || !f.processed(t, "evt_stale_old") {
		t.Fatal("舊嘗試的狀態沒有記進歷程表，或事件沒有處理完")
	}
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if f.statusOf(t, "O-R3") != PaymentAwaitingPayment || f.externalOf(t, "O-R3") == oldExt {
		t.Fatalf("新的建立之後：狀態=%s 外部 ID=%s", f.statusOf(t, "O-R3"), f.externalOf(t, "O-R3"))
	}
}

// 已派送但還沒同步的舊 PaymentIntent 事件由掃描補排同步並處理完：事件的外部 ID 經歷程表對到付款，不會在 inbox 永遠未處理。
// mutation：PaymentIDsByExternal 改回只比付款列上的外部 ID → 掃描排不出同步，事件永遠未處理，失敗。
func TestAnEventOfASupersededIntentIsSweptAndFullyProcessed(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	p, oldExt, _ := f.repay(t, "O-R2d")
	h := http.Header{}
	h.Set(ChargeFakeEventHeader, "evt_swept")
	must(t, f.module.ReceivePaymentEvent(ctx, h, []byte(oldExt)))
	f.now = f.now.Add(2 * time.Minute)
	f.jobs.mu.Lock()
	f.jobs.args, f.jobs.opts = nil, nil
	f.jobs.mu.Unlock()
	if _, synced, err := f.module.SweepPayments(ctx); err != nil || synced < 1 {
		t.Fatalf("掃描：%d %v", synced, err)
	}
	if got := f.paymentIDs(t, kindChargeSync); !containsID(got, p.ID) {
		t.Fatalf("同步 job=%v", got)
	}
	f.sync(t, p.ID)
	if !f.processed(t, "evt_swept") {
		t.Fatal("舊 PaymentIntent 的事件沒有處理完")
	}
}

// 建立中的列屬於別的 provider、而且呼叫過（結果不明）：目前設定的 provider 查不到舊 provider 的物件，所以不依時間改綁，進 ANOMALY（發 MoneyAnomaly、不呼叫 provider、不換鍵）。
// mutation：create() 拿掉 CreateCalledAt 的檢查（呼叫過的列也走 rebind）且 RebindCreatingPayment 拿掉 create_called_at 的條件 → 過了查無的時刻被改綁並建出第二個 PaymentIntent，失敗。
func TestACreatingRowOfAnotherProviderThatWasCalledGoesToAnomalyInsteadOfBeingRebound(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-R4", 11, 7, 100)
	f.fake.FailCreate(errors.New("逾時"))
	p := f.create(t, 11, "O-R4")
	oldKey := f.keyOf(t, "O-R4")
	f.exec(t, "UPDATE payment.payments SET provider = 'retired_provider' WHERE order_no = 'O-R4'")
	f.exec(t, "UPDATE payment.payment_attempts SET provider = 'retired_provider' WHERE payment_id = $1", p.ID)
	f.now = f.now.Add(PaymentNotFoundAfter + time.Minute)
	must(t, f.module.CreatePaymentIntent(ctx, p.ID, false))
	if f.statusOf(t, "O-R4") != PaymentAnomaly || f.text(t, "SELECT failure_reason FROM payment.payments WHERE order_no = 'O-R4'") != "provider_changed_after_call" {
		t.Fatalf("狀態=%s", f.statusOf(t, "O-R4"))
	}
	if f.text(t, "SELECT provider FROM payment.payments WHERE order_no = 'O-R4'") != "retired_provider" || f.keyOf(t, "O-R4") != oldKey || f.fake.Intents() != 0 || f.fake.LookupCalls() != 0 {
		t.Fatal("不該改綁、換鍵、呼叫或查回目前的 provider")
	}
	if len(f.anomalies) != 1 || f.anomalies[0].Kind != AnomalyPayment || f.anomalies[0].ObjectID != p.ID || !f.anomalyInTx[0] {
		t.Fatalf("告警事件=%+v", f.anomalies)
	}
	if f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE payment_id = $1", p.ID) != 1 {
		t.Fatal("不該寫新的嘗試")
	}
}

// 從沒呼叫過的列屬於別的 provider：直接改成目前的（沒有東西可能已經建在舊 provider）。
func TestACreatingRowOfAnotherProviderThatWasNeverCalledIsReboundAtOnce(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-R4b", 11, 7, 100)
	f.fake.FailCreate(errors.New("逾時"))
	p := f.create(t, 11, "O-R4b")
	f.exec(t, "UPDATE payment.payments SET provider = 'retired_provider', create_called_at = NULL WHERE order_no = 'O-R4b'")
	f.exec(t, "UPDATE payment.payment_attempts SET provider = 'retired_provider', called_at = NULL WHERE payment_id = $1", p.ID)
	must(t, f.module.CreatePaymentIntent(context.Background(), p.ID, false))
	if f.text(t, "SELECT provider FROM payment.payments WHERE order_no = 'O-R4b'") != ChargeFakeName || f.statusOf(t, "O-R4b") != PaymentAwaitingPayment {
		t.Fatalf("狀態=%s", f.statusOf(t, "O-R4b"))
	}
}

// 回快照的 provider 與嘗試登記的不同（呼叫進行中別的設定改綁了這一列）：快照丟棄，不把這個 provider 的 ID 寫到標成另一個 provider 的列。
// mutation：applySnapshot 拿掉 provider 比對 → 付款列被寫上外部 ID，失敗。
func TestASnapshotForAnAttemptOfAnotherProviderIsDiscarded(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	f.order("O-R4c", 11, 7, 100)
	f.fake.OnCreate = func() {
		f.exec(t, "UPDATE payment.payments SET provider = 'retired_provider' WHERE order_no = 'O-R4c'")
		f.exec(t, "UPDATE payment.payment_attempts SET provider = 'retired_provider' WHERE payment_id = (SELECT id FROM payment.payments WHERE order_no = 'O-R4c')")
	}
	p := f.create(t, 11, "O-R4c")
	if p.Status != PaymentCreating || f.externalOf(t, "O-R4c") != "" || f.count(t, "SELECT count(*) FROM payment.payment_attempts WHERE external_payment_id IS NOT NULL") != 0 {
		t.Fatalf("別的 provider 的快照被寫入：%+v 外部 ID=%q", p, f.externalOf(t, "O-R4c"))
	}
}

// 入帳卻還沒有 charge 的暫時性快照：不寫入、事件保持未處理（掃描與重試會重讀）、錯誤不是永久失敗；provider 補齊之後入帳並標記事件。
// mutation：入帳缺 charge 時改回 ErrAnomaly（Sync 標記事件已處理）→ 事件被標記，補齊之後也不會重讀，失敗。
func TestAnIncompleteSuccessSnapshotKeepsItsEventsUnprocessedUntilItIsComplete(t *testing.T) {
	t.Parallel()
	f := setupCharge(t)
	ctx := context.Background()
	f.order("O-R5", 11, 7, 100)
	p := f.create(t, 11, "O-R5")
	ext := f.externalOf(t, "O-R5")
	must(t, f.fake.Settle(ext, PaymentStateCanceled, f.now))
	f.sync(t, p.ID)
	must(t, f.fake.SetState(ext, PaymentStateSucceeded))
	f.receive(t, "evt_nocharge", ext)
	_, err := f.module.SyncPayment(ctx, p.ID)
	if err == nil || errors.Is(err, ErrChargePermanent) {
		t.Fatalf("暫時性的不一致應回一般錯誤：%v", err)
	}
	if f.processed(t, "evt_nocharge") || f.statusOf(t, "O-R5") != PaymentCanceled {
		t.Fatalf("事件已處理=%v 狀態=%s", f.processed(t, "evt_nocharge"), f.statusOf(t, "O-R5"))
	}
	f.now = f.now.Add(2 * time.Minute)
	f.jobs.mu.Lock()
	f.jobs.args, f.jobs.opts = nil, nil
	f.jobs.mu.Unlock()
	if _, synced, err := f.module.SweepPayments(ctx); err != nil || synced != 1 {
		t.Fatalf("掃描要為未處理的事件重排同步：%d %v", synced, err)
	}
	must(t, f.fake.Settle(ext, PaymentStateSucceeded, f.now))
	f.sync(t, p.ID)
	if f.statusOf(t, "O-R5") != PaymentSucceeded || !f.processed(t, "evt_nocharge") {
		t.Fatalf("補齊之後：狀態=%s 已處理=%v", f.statusOf(t, "O-R5"), f.processed(t, "evt_nocharge"))
	}
}

package charge

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
)

// 新建付款寫列的參數帶說明：payments_description_check 不接受空字串，沒帶說明的列每一筆都寫不進去；付款金額是各訂單應收的總額。
// mutation：newPaymentParams 拿掉 Description → 這個測試與 integration 的新建付款測試失敗。
func TestNewPaymentParamsCarryTheDescriptionAndTheTotalOfTheOrders(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 23, 4, 0, 0, 0, time.UTC)
	orders := []Order{{OrderNo: "O-1", StoreID: 7, BuyerID: 11, Amount: 100, Description: " 藍色 T 恤 "}, {OrderNo: "O-2", StoreID: 8, BuyerID: 11, Amount: 250}}
	got := newPaymentParams(uuid.NewV7(), 11, orders, FakeName, now)
	if got.Description != "藍色 T 恤" || got.BuyerID != 11 || got.Amount != 350 || got.Provider != FakeName || !got.Now.Equal(now) {
		t.Fatalf("參數=%+v", got)
	}
	if empty := newPaymentParams(uuid.NewV7(), 11, []Order{{OrderNo: "O-2", Amount: 5}}, FakeName, now); empty.Description != "O-2" {
		t.Fatalf("沒有商品名稱時說明=%q", empty.Description)
	}
}

// 入帳卻還沒有 charge 或入帳時間是暫時性的不一致：不是永久失敗（job 會重試），也不是 anomaly（事件不被標記已處理）。
// mutation：checkSucceededSnapshot 改回 ErrAnomaly → 這個測試失敗。
func TestAnIncompleteSuccessIsTransientNotAnAnomaly(t *testing.T) {
	t.Parallel()
	paid := time.Date(2026, 11, 23, 4, 0, 0, 0, time.UTC)
	for name, snap := range map[string]Snapshot{
		"沒有 charge": {State: StateSucceeded, PaidAt: paid},
		"沒有入帳時間":    {State: StateSucceeded, ChargeID: "ch_1"},
		"兩者都沒有":     {State: StateSucceeded},
	} {
		err := checkSucceededSnapshot(snap)
		if !errors.Is(err, errIncompleteSuccess) || errors.Is(err, ErrAnomaly) || errors.Is(err, ErrPermanent) {
			t.Errorf("%s：%v", name, err)
		}
	}
	if err := checkSucceededSnapshot(Snapshot{State: StateSucceeded, ChargeID: "ch_1", PaidAt: paid}); err != nil {
		t.Errorf("完整的入帳：%v", err)
	}
	if err := checkSucceededSnapshot(Snapshot{State: StateAwaitingPayment}); err != nil {
		t.Errorf("不是入帳的快照：%v", err)
	}
}

// 舊 PaymentIntent 的快照對付款列的處置：晚到成功且列還沒入帳 → 列記成功；列已入帳又成功 → 重複入帳；列已入帳而它還沒結案 → 排取消；其餘只記在嘗試列。
// mutation：lateOutcomeOf 把「列已入帳又成功」也當成 lateAdopt → 第二筆入帳覆蓋第一筆，這個測試失敗。
func TestLateOutcomeFollowsTheDecisionTable(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		row  model.PaymentStatus
		snap State
		want lateOutcome
	}{
		{"late success while the new intent is awaiting", model.PaymentAwaitingPayment, StateSucceeded, lateAdopt},
		{"late success while the row is creating", model.PaymentCreating, StateSucceeded, lateAdopt},
		{"late success while the row is canceled", model.PaymentCanceled, StateSucceeded, lateAdopt},
		{"both succeeded", model.PaymentSucceeded, StateSucceeded, lateExcess},
		{"paid row, old intent still awaiting", model.PaymentSucceeded, StateAwaitingPayment, lateCancel},
		{"paid row, old intent processing", model.PaymentSucceeded, StateProcessing, lateCancel},
		{"paid row, old intent canceled", model.PaymentSucceeded, StateCanceled, lateRecord},
		{"unpaid row, old intent canceled", model.PaymentAwaitingPayment, StateCanceled, lateRecord},
		{"unpaid row, old intent awaiting", model.PaymentAwaitingPayment, StateAwaitingPayment, lateRecord},
	} {
		if got := lateOutcomeOf(c.row, c.snap); got != c.want {
			t.Errorf("%s：%d，want %d", c.name, got, c.want)
		}
	}
}

// 嘗試的狀態值與 payment_attempts_state_check 逐項相同，而且每個有效快照狀態都對得到一個值、不認得的不對應（空字串不是有效值）。
func TestAttemptStatesMatchTheSchemaCheck(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`CONSTRAINT payment_attempts_state_check CHECK \(state IS NULL OR state IN \(([^)]*)\)\)`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("schema.sql 沒有 payment_attempts_state_check")
	}
	var want []string
	for _, v := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllSubmatch(m[1], -1) {
		want = append(want, string(v[1]))
	}
	var got []string
	for _, s := range []State{StateAwaitingPayment, StateProcessing, StateSucceeded, StateCanceled} {
		v := attemptState(s)
		if v == "" {
			t.Fatalf("%s 沒有對應的嘗試狀態", s)
		}
		got = append(got, v)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Go=%v schema=%v", got, want)
	}
	if attemptState(StateUnknown) != "" || attemptState("") != "" {
		t.Fatal("不認得的狀態不得對應成有效值")
	}
}

// 結案（入帳或取消）的嘗試之外都還要再讀；沒讀過（nil）不算結案。
func TestAttemptClosedOnlyForSucceededOrCanceled(t *testing.T) {
	t.Parallel()
	s := func(v string) *string { return &v }
	if attemptClosed(nil) || attemptClosed(s("AWAITING_PAYMENT")) || attemptClosed(s("PROCESSING")) || !attemptClosed(s("SUCCEEDED")) || !attemptClosed(s("CANCELED")) {
		t.Fatal("attemptClosed 判斷錯誤")
	}
}

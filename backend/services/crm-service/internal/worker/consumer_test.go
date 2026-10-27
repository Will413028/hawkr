package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	"github.com/Will413028/hawkr/go-pkg/mq"
)

// fail-loud：order.completed 數值欄位 parse 失敗必須回傳 error（ResilientConsumer 重試 / DLQ），
// 禁止吞成 0——amount 吞 0 會讓買家消費金額統計靜默歸零。
// handler 在 parse 階段即返回，不觸及 usecase / repo，故以 nil 依賴呼叫即可驗證。
func TestOrderCompletedHandler_ParseFailure_ReturnsError(t *testing.T) {
	handler := orderCompletedHandler(nil)

	cases := []struct {
		name    string
		body    string
		wantSub string
	}{
		{
			"amount 非數字",
			`{"order_no":"O1","buyer_id":"1","store_id":"2","total_actual_amount":"abc"}`,
			"total_actual_amount",
		},
		{
			"seller id 非數字",
			`{"order_no":"O1","buyer_id":"1","store_id":"x","total_actual_amount":"100"}`,
			"store_id",
		},
		{
			"buyer id 缺失（空字串）",
			`{"order_no":"O1","store_id":"2","total_actual_amount":"100"}`,
			"buyer_id",
		},
		{
			"event_id 缺失（空字串）",
			`{"order_no":"O1","buyer_id":"1","store_id":"2","total_actual_amount":"100"}`,
			"event_id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := handler(context.Background(), "order.completed", []byte(tc.body))
			if err == nil {
				t.Fatalf("期望 parse 失敗回傳 error，實得 nil（吞 0 迴歸）")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error 應含欄位名 %q，實得: %v", tc.wantSub, err)
			}
		})
	}
}

// fail-loud：refund.completed 的 event_id / order_no / refund_outcome 缺失或未知值
// 必須回傳 error（ResilientConsumer 重試 / DLQ），不得靜默處理。
func TestRefundCompletedHandler_ParseFailure_ReturnsError(t *testing.T) {
	handler := refundCompletedHandler(nil)

	cases := []struct {
		name    string
		body    string
		wantSub string
	}{
		{
			"event_id 缺失",
			`{"order_no":"O1","refund_outcome":"PARTIAL_REFUND","refund_amount":"100"}`,
			"event_id",
		},
		{
			"order_no 缺失",
			`{"event_id":"e1","refund_outcome":"PARTIAL_REFUND","refund_amount":"100"}`,
			"order_no",
		},
		{
			"refund_outcome 未知值",
			`{"event_id":"e1","case_no":"CASE-1","order_no":"O1","refund_outcome":"REJECT_APPEAL","refund_amount":"100"}`,
			"refund_outcome",
		},
		{
			"PARTIAL_REFUND 的 refund_amount 非數字",
			`{"event_id":"e1","case_no":"CASE-1","order_no":"O1","refund_outcome":"PARTIAL_REFUND","refund_amount":"abc"}`,
			"refund_amount",
		},
		{
			// event_id 不再是 ledger 冪等鍵（RefundSyncWorker 每小時重投都帶新 UUID），
			// 但 case_no / external_ref_id 兩者皆缺時仍無法推導穩定 dedup key，須 fail-loud。
			"case_no 與 external_ref_id 皆缺失",
			`{"event_id":"e1","order_no":"O1","refund_outcome":"PARTIAL_REFUND","refund_amount":"100"}`,
			"external_ref_id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := handler(context.Background(), "aftersales.refund.completed", []byte(tc.body))
			if err == nil {
				t.Fatalf("期望 parse 失敗回傳 error，實得 nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error 應含欄位名 %q，實得: %v", tc.wantSub, err)
			}
		})
	}
}

// FULL_REFUND 不需要 refund_amount 可解析——self-netting 只用 ledger 歷史，
// 驗證這個 case 不會在 parse 階段被誤擋。缺依賴（uc=nil）會在呼叫 RecordRefund
// 時 panic，因此只驗證它「通過了 parse 階段」而非驗證完整流程。
func TestRefundCompletedHandler_FullRefund_SkipsAmountParsing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected a panic from the nil usecase past the parse stage (proves refund_amount parsing was skipped for FULL_REFUND)")
		}
	}()
	handler := refundCompletedHandler(nil)
	_ = handler(context.Background(), "aftersales.refund.completed",
		[]byte(`{"event_id":"e1","case_no":"CASE-1","order_no":"O1","refund_outcome":"FULL_REFUND","refund_amount":"not-a-number"}`))
}

// event_id is NOT a stable idempotency key for aftersales.refund.completed —
// aftersales-service's RefundSyncWorker republishes every DONE refund task
// older than 1h indefinitely, with a fresh UUID event_id each time. Mirrors
// order-service's own consumer of this same event, which dedups on
// financial_log's external_txn_id, falling back to "AS_REFUND_"+case_no.
func TestRefundDedupKey_PrefersExternalRefID(t *testing.T) {
	key, err := refundDedupKey("ext-123", "CASE-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "ext-123" {
		t.Fatalf("expected external_ref_id to be used, got %q", key)
	}
}

func TestRefundDedupKey_FallsBackToCaseNo(t *testing.T) {
	key, err := refundDedupKey("", "CASE-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "AS_REFUND_CASE-456" {
		t.Fatalf("expected AS_REFUND_ prefix + case_no, got %q", key)
	}
}

func TestRefundDedupKey_BothEmpty_ReturnsError(t *testing.T) {
	if _, err := refundDedupKey("", ""); err == nil {
		t.Fatalf("expected error when both external_ref_id and case_no are empty")
	}
}

func TestRefundDedupKey_TooLong_ReturnsError(t *testing.T) {
	// external_ref_id/case_no are externally-sourced, unlike event_id
	// (always a bounded UUID) — must fail loud rather than let a too-long
	// value surface as an opaque DB VARCHAR(64) constraint error.
	tooLong := strings.Repeat("x", 65)
	if _, err := refundDedupKey(tooLong, ""); err == nil {
		t.Fatalf("expected error when derived dedup key exceeds 64 chars")
	}
	if _, err := refundDedupKey("", tooLong); err == nil {
		t.Fatalf("expected error when AS_REFUND_+case_no exceeds 64 chars")
	}
}

func TestRefundDedupKey_StableAcrossDifferentEventIDs(t *testing.T) {
	// Simulates RefundSyncWorker's hourly republish: same case_no/
	// external_ref_id, a different (fresh UUID) event_id each time. The
	// computed dedup key must be identical regardless — event_id is
	// deliberately never consulted by this function.
	key1, err := refundDedupKey("ext-123", "CASE-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	key2, err := refundDedupKey("ext-123", "CASE-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key1 != key2 {
		t.Fatalf("expected stable dedup key across calls, got %q vs %q", key1, key2)
	}
}

// #2847：買家註銷後必須抹除 crm_customers 的個資快取。
//
// fail-loud：buyerId 缺失時**不得**靜默跳過——那會讓一筆該抹除的個資留著，
// 而且沒有任何訊號。handler 在檢查階段即返回，不觸及 usecase / repo。
func TestBuyerDeletedHandler_MissingBuyerID_ReturnsError(t *testing.T) {
	handler := buyerDeletedHandler(nil)

	// 有 accountId 但沒有 buyerId：最容易被誤判成「可以跳過」的形狀，
	// 因為事件本身看起來是完整的。
	err := handler(context.Background(), "buyer.deleted", []byte(`{"accountId":123}`))
	if err == nil {
		t.Fatal("buyerId 缺失必須回 error 走重試／DLQ，不得靜默跳過——" +
			"靜默跳過等於一筆個資永久留在 crm_customers 且無人知道")
	}
	if !strings.Contains(err.Error(), "buyerId") {
		t.Errorf("error 應指出缺的是 buyerId，實際 = %v", err)
	}
}

func TestRefundCompletedHandler_PayloadFailuresAreClassifiedPermanent(t *testing.T) {
	handler := refundCompletedHandler(nil)

	cases := []struct {
		name string
		body string
	}{
		{"payload 不是合法 JSON", `not-json`},
		{"event_id 缺失", `{"order_no":"O1","refund_outcome":"PARTIAL_REFUND","refund_amount":"100"}`},
		{"order_no 缺失", `{"event_id":"e1","refund_outcome":"PARTIAL_REFUND","refund_amount":"100"}`},
		{"refund_outcome 未知值", `{"event_id":"e1","case_no":"C1","order_no":"O1","refund_outcome":"REJECT_APPEAL"}`},
		{"refund_amount 不是數字", `{"event_id":"e1","case_no":"C1","order_no":"O1","refund_outcome":"PARTIAL_REFUND","refund_amount":"abc"}`},
		{"dedup key 兩個來源都空", `{"event_id":"e1","order_no":"O1","refund_outcome":"PARTIAL_REFUND","refund_amount":"100"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := handler(context.Background(), "aftersales.refund.completed", []byte(tc.body))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, mq.ErrPermanent) {
				t.Errorf("payload 缺陷重試幾次都一樣，必須標成 permanent：%v", err)
			}
		})
	}
}

func TestClassifyRefundError_OnlyOrderNotFoundIsPermanent(t *testing.T) {
	notFound := fmt.Errorf("resolve order O1: %w", usecase.ErrOrderNotFound)
	if err := classifyRefundError(notFound); !errors.Is(err, mq.ErrPermanent) {
		t.Error("訂單不存在是永久性失敗，重試不會讓它變存在")
	}
	if err := classifyRefundError(notFound); !errors.Is(err, usecase.ErrOrderNotFound) {
		t.Error("分類後仍要保留原因，否則上游無法判讀")
	}

	transient := errors.New("connection refused")
	if err := classifyRefundError(transient); errors.Is(err, mq.ErrPermanent) {
		t.Error("下游不可用是暫時性失敗，必須維持重試")
	}
	if classifyRefundError(nil) != nil {
		t.Error("nil 必須維持 nil")
	}
}

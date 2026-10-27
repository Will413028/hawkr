package mq

import (
	"strings"
	"testing"
	"time"
)

// TestParseEventTime_BothQueueFormats 守 §6.3.2.2 陷阱表第 1 列：兩條 queue 的
// occurred_at 格式**不同**。
//
// C2 走 Java 的 formatIso8601（帶 Z，RFC3339 可解）；C1 走 LocalDateTime.toString()
// ——無 Z、無時區，而且秒與奈秒同時為 0 時連秒都省略。只用 RFC3339 解 C1 必定失敗，
// 而失敗若被吞成零值，法定期限會落在 1970。
func TestParseEventTime_BothQueueFormats(t *testing.T) {
	cases := map[string]struct {
		in   string
		want time.Time
	}{
		"C2 RFC3339 帶 Z":    {"2026-08-25T09:41:02Z", time.Date(2026, 8, 25, 9, 41, 2, 0, time.UTC)},
		"C2 RFC3339 帶小數":    {"2026-08-25T09:41:02.5Z", time.Date(2026, 8, 25, 9, 41, 2, 500000000, time.UTC)},
		"C1 無時區帶秒":          {"2026-08-25T09:41:02", time.Date(2026, 8, 25, 9, 41, 2, 0, time.UTC)},
		"C1 無時區帶奈秒":         {"2026-08-25T09:41:02.123456789", time.Date(2026, 8, 25, 9, 41, 2, 123456789, time.UTC)},
		"C1 秒與奈秒皆 0 時連秒都省略": {"2026-08-25T09:41", time.Date(2026, 8, 25, 9, 41, 0, 0, time.UTC)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseEventTime(c.in)
			if err != nil {
				t.Fatalf("parseEventTime(%q): %v", c.in, err)
			}
			if !got.Equal(c.want) {
				t.Fatalf("parseEventTime(%q) = %v, want %v", c.in, got, c.want)
			}
			if got.Location() != time.UTC {
				t.Fatalf("必須回 UTC，got %v", got.Location())
			}
		})
	}
}

// TestParseEventTime_FailsLoud 守「解析不出來就回 error」——不得回零值。
func TestParseEventTime_FailsLoud(t *testing.T) {
	for _, in := range []string{"", "  ", "yesterday", "2026-08-25", "1756114862"} {
		if _, err := parseEventTime(in); err == nil {
			t.Errorf("parseEventTime(%q) 應失敗，卻成功了", in)
		}
	}
}

// TestDedupKeys 守 §6.3.5.2 的鍵組成。前綴打錯不會報錯，只會讓唯一鍵落在另一個
// 命名空間 ⇒ 去重整個失效。
func TestDedupKeys(t *testing.T) {
	if got := orderPaidDedupKey("O123"); got != "ORDER_PAID:O123" {
		t.Errorf("orderPaidDedupKey = %q", got)
	}
	if got := refundDedupKey("TXN-1"); got != "REFUND:TXN-1" {
		t.Errorf("refundDedupKey = %q", got)
	}
}

// TestDedupKeyLengthGuard 守欄寬。
//
// business_dedup_key 是 varchar(160)，上界情境是 "REFUND:"(7) ＋ refund_ref(128) ＝ 135。
// 這一支釘住「135 進得去、161 被擋下」——擋下是 fail-loud 的 error，不是靜默截斷
// （截斷後的鍵會與另一筆退款撞在一起）。
func TestDedupKeyLengthGuard(t *testing.T) {
	worstCase := refundDedupKey(strings.Repeat("x", 128))
	if len(worstCase) != 135 {
		t.Fatalf("最壞情境長度 = %d，want 135（設計書 §6.3.5.3 的推導）", len(worstCase))
	}
	if err := checkDedupKey(worstCase); err != nil {
		t.Fatalf("135 bytes 必須通過：%v", err)
	}
	if err := checkDedupKey(strings.Repeat("y", dedupKeyMaxLen+1)); err == nil {
		t.Fatal("超過 160 bytes 必須被擋下")
	}
}

// TestPaymentFactsPaidAtCoalesces 守 §5.3.4 的 COALESCE：ATM 的 webhook 通常沒有
// provider_paid_at，那是正常值不是異常。
func TestPaymentFactsPaidAtCoalesces(t *testing.T) {
	payTime := time.Date(2026, 8, 25, 1, 0, 0, 0, time.UTC)
	providerPaid := time.Date(2026, 8, 25, 0, 30, 0, 0, time.UTC)

	if got := (PaymentFacts{PayTime: payTime}).PaidAt(); !got.Equal(payTime) {
		t.Errorf("provider_paid_at 為 NULL 時應退回 pay_time，got %v", got)
	}
	if got := (PaymentFacts{PayTime: payTime, ProviderPaidAt: &providerPaid}).PaidAt(); !got.Equal(providerPaid) {
		t.Errorf("provider_paid_at 非空時應優先，got %v", got)
	}
	var zero time.Time
	if got := (PaymentFacts{PayTime: payTime, ProviderPaidAt: &zero}).PaidAt(); !got.Equal(payTime) {
		t.Errorf("provider_paid_at 是零值時應退回 pay_time，got %v", got)
	}
}

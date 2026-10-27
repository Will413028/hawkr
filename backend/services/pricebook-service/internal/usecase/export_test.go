package usecase

import (
	"bytes"
	"strings"
	"testing"
)

// TestObservationCSVIsDeterministic 驗證 WriteObservationCSV 的排序與小數位
// 是固定的：兩邊各自產生後要能直接 diff。
//
// fixture 修正（preflight §C Task 9、task-9-brief.md Step 3 註記）：原始
// fixture 未設 HasPrice/HasVolume，依 WriteObservationCSV 的邏輯會輸出空
// 欄位而非數字，跟這裡的 want 對不上——這三列都是「有值」的觀測，故補上
// HasPrice: true, HasVolume: true。
func TestObservationCSVIsDeterministic(t *testing.T) {
	rows := []ObservationRow{
		{MarketCode: "109", CropCode: "LA1", TradeDate: "2026-07-26", AvgPrice: 46.2, VolumeKg: 10957, HasPrice: true, HasVolume: true},
		{MarketCode: "104", CropCode: "LA2", TradeDate: "2026-07-26", AvgPrice: 23.7, VolumeKg: 44650, HasPrice: true, HasVolume: true},
		{MarketCode: "104", CropCode: "LA1", TradeDate: "2026-07-26", AvgPrice: 46.2, VolumeKg: 10957, HasPrice: true, HasVolume: true},
	}
	var buf bytes.Buffer
	if err := WriteObservationCSV(&buf, rows); err != nil {
		t.Fatalf("WriteObservationCSV 失敗: %v", err)
	}
	got := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := []string{
		"market_code,crop_code,trade_date,avg_price,volume_kg",
		"104,LA1,2026-07-26,46.2000,10957.000",
		"104,LA2,2026-07-26,23.7000,44650.000",
		"109,LA1,2026-07-26,46.2000,10957.000",
	}
	if len(got) != len(want) {
		t.Fatalf("行數 = %d，預期 %d：\n%s", len(got), len(want), buf.String())
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 行 = %q，預期 %q", i, got[i], want[i])
		}
	}
}

// TestObservationCSVNullPriceIsEmptyNotZero 驗證缺值渲染成空字串，不是 0。
// 寫成 0 會讓對帳兩邊「看起來一致」而掩蓋差異——這正是 Task 9 存在的理由。
func TestObservationCSVNullPriceIsEmptyNotZero(t *testing.T) {
	rows := []ObservationRow{{MarketCode: "104", CropCode: "LA1", TradeDate: "2026-07-27", HasPrice: false}}
	var buf bytes.Buffer
	if err := WriteObservationCSV(&buf, rows); err != nil {
		t.Fatalf("WriteObservationCSV 失敗: %v", err)
	}
	if !strings.Contains(buf.String(), "104,LA1,2026-07-27,,") {
		t.Errorf("缺值未寫成空欄位:\n%s", buf.String())
	}
}

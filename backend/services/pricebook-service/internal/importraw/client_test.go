package importraw

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeLegacyDay(t *testing.T, legacyDir, date, jsonBody string) {
	t.Helper()
	dayDir := filepath.Join(legacyDir, "date="+date)
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		t.Fatalf("建立 legacy fixture 目錄失敗: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dayDir, "data.json"), []byte(jsonBody), 0o644); err != nil {
		t.Fatalf("寫入 legacy fixture 失敗: %v", err)
	}
}

func writeBronzeSnapshot(t *testing.T, bronzeDir, date, snapshotID, jsonBody string) {
	t.Helper()
	snapDir := filepath.Join(bronzeDir, "partition="+date, "snapshot="+snapshotID)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatalf("建立 bronze fixture 目錄失敗: %v", err)
	}
	if err := os.WriteFile(filepath.Join(snapDir, "payload.json"), []byte(jsonBody), 0o644); err != nil {
		t.Fatalf("寫入 bronze fixture 失敗: %v", err)
	}
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("解析測試日期失敗: %v", err)
	}
	return d
}

const legacyDayFixture = `[
 {"交易日期":"2022-08-12","種類代碼":"N05","作物代號":"11","作物名稱":"椰子","市場代號":"104","市場名稱":"台北二","上價":48.0,"中價":23.1,"下價":17.4,"平均價":27.0,"交易量":1230.0}
]`

const bronzeDayFixtureA = `[
 {"交易日期":"114.11.20","種類代碼":"N05","作物代號":"11","作物名稱":"椰子","市場代號":"104","市場名稱":"台北二","上價":19.0,"中價":14.2,"下價":13.0,"平均價":14.9,"交易量":940.0}
]`

const bronzeDayFixtureB = `[
 {"交易日期":"114.11.20","種類代碼":"N05","作物代號":"31","作物名稱":"釋迦","市場代號":"104","市場名稱":"台北二","上價":127.5,"中價":86.3,"下價":60.3,"平均價":89.4,"交易量":239.0}
]`

func TestFileClientFetchRangeReadsLegacyDay(t *testing.T) {
	dir := t.TempDir()
	legacyDir := filepath.Join(dir, "price")
	writeLegacyDay(t, legacyDir, "2022-08-12", legacyDayFixture)

	client := NewFileClient(legacyDir, filepath.Join(dir, "bronze"))
	d := mustParseDate(t, "2022-08-12")

	records, err := client.FetchRange(context.Background(), d, d, "", "104")
	if err != nil {
		t.Fatalf("FetchRange 失敗: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("預期 1 筆，實得 %d 筆", len(records))
	}
	if records[0].TransDate != "111.08.12" {
		t.Errorf("legacy 西元日期應轉成民國格式，實得 %q", records[0].TransDate)
	}
	if records[0].CropName != "椰子" || records[0].AvgPrice != 27.0 {
		t.Errorf("其餘欄位應正確帶過，實得 %+v", records[0])
	}
}

func TestFileClientFetchRangeReadsBronzeDayUnionsSnapshots(t *testing.T) {
	dir := t.TempDir()
	bronzeDir := filepath.Join(dir, "bronze")
	writeBronzeSnapshot(t, bronzeDir, "2025-11-20", "snap-a", bronzeDayFixtureA)
	writeBronzeSnapshot(t, bronzeDir, "2025-11-20", "snap-b", bronzeDayFixtureB)

	client := NewFileClient(filepath.Join(dir, "price"), bronzeDir)
	d := mustParseDate(t, "2025-11-20")

	records, err := client.FetchRange(context.Background(), d, d, "", "104")
	if err != nil {
		t.Fatalf("FetchRange 失敗: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("預期兩個 snapshot union 後共 2 筆，實得 %d 筆: %+v", len(records), records)
	}
	for _, r := range records {
		if r.TransDate != "114.11.20" {
			t.Errorf("bronze 民國日期應原樣帶過，實得 %q", r.TransDate)
		}
	}
}

func TestFileClientFetchRangeSpansLegacyAndBronzeBoundary(t *testing.T) {
	dir := t.TempDir()
	legacyDir := filepath.Join(dir, "price")
	bronzeDir := filepath.Join(dir, "bronze")
	writeLegacyDay(t, legacyDir, "2025-11-19", `[
	 {"交易日期":"2025-11-19","種類代碼":"N05","作物代號":"11","作物名稱":"椰子","市場代號":"104","市場名稱":"台北二","上價":20.0,"中價":15.0,"下價":10.0,"平均價":15.0,"交易量":100.0}
	]`)
	writeBronzeSnapshot(t, bronzeDir, "2025-11-20", "snap-a", bronzeDayFixtureA)

	client := NewFileClient(legacyDir, bronzeDir)
	start := mustParseDate(t, "2025-11-19")
	end := mustParseDate(t, "2025-11-20")

	records, err := client.FetchRange(context.Background(), start, end, "", "104")
	if err != nil {
		t.Fatalf("FetchRange 失敗: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("跨 legacy/bronze 邊界應各取 1 筆共 2 筆，實得 %d 筆: %+v", len(records), records)
	}
}

func TestFileClientFetchRangeFiltersByMarketCode(t *testing.T) {
	dir := t.TempDir()
	legacyDir := filepath.Join(dir, "price")
	writeLegacyDay(t, legacyDir, "2022-08-12", `[
	 {"交易日期":"2022-08-12","種類代碼":"N05","作物代號":"11","作物名稱":"椰子","市場代號":"104","市場名稱":"台北二","上價":48.0,"中價":23.1,"下價":17.4,"平均價":27.0,"交易量":1230.0},
	 {"交易日期":"2022-08-12","種類代碼":"N05","作物代號":"12","作物名稱":"芭樂","市場代號":"108","市場名稱":"台中","上價":30.0,"中價":20.0,"下價":10.0,"平均價":20.0,"交易量":500.0}
	]`)

	client := NewFileClient(legacyDir, filepath.Join(dir, "bronze"))
	d := mustParseDate(t, "2022-08-12")

	records, err := client.FetchRange(context.Background(), d, d, "", "108")
	if err != nil {
		t.Fatalf("FetchRange 失敗: %v", err)
	}
	if len(records) != 1 || records[0].MarketCode != "108" {
		t.Errorf("應只回市場 108 的資料，實得 %+v", records)
	}
}

func TestFileClientFetchRangeCachesDayAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	legacyDir := filepath.Join(dir, "price")
	writeLegacyDay(t, legacyDir, "2022-08-12", `[
	 {"交易日期":"2022-08-12","種類代碼":"N05","作物代號":"11","作物名稱":"椰子","市場代號":"104","市場名稱":"台北二","上價":48.0,"中價":23.1,"下價":17.4,"平均價":27.0,"交易量":1230.0},
	 {"交易日期":"2022-08-12","種類代碼":"N05","作物代號":"12","作物名稱":"芭樂","市場代號":"108","市場名稱":"台中","上價":30.0,"中價":20.0,"下價":10.0,"平均價":20.0,"交易量":500.0}
	]`)

	client := NewFileClient(legacyDir, filepath.Join(dir, "bronze"))
	d := mustParseDate(t, "2022-08-12")

	if _, err := client.FetchRange(context.Background(), d, d, "", "104"); err != nil {
		t.Fatalf("第一次 FetchRange 失敗: %v", err)
	}

	// 刪掉來源檔案：第二次呼叫若還能拿到市場 108 的資料，證明是走 dayCache，
	// 不是重新讀檔（重新讀檔會因為檔案已刪除而失敗或回空）。
	if err := os.RemoveAll(legacyDir); err != nil {
		t.Fatalf("清除 fixture 失敗: %v", err)
	}

	records, err := client.FetchRange(context.Background(), d, d, "", "108")
	if err != nil {
		t.Fatalf("第二次 FetchRange 失敗（不應該真的重新讀檔）: %v", err)
	}
	if len(records) != 1 || records[0].MarketCode != "108" {
		t.Errorf("預期從快取拿到市場 108 的 1 筆資料，實得 %+v", records)
	}
}

func TestFileClientFetchRestDaysScansDistinctMarketsAndCaches(t *testing.T) {
	dir := t.TempDir()
	legacyDir := filepath.Join(dir, "price")
	bronzeDir := filepath.Join(dir, "bronze")
	writeLegacyDay(t, legacyDir, "2022-08-12", `[
	 {"交易日期":"2022-08-12","種類代碼":"N05","作物代號":"11","作物名稱":"椰子","市場代號":"104","市場名稱":"台北二","上價":48.0,"中價":23.1,"下價":17.4,"平均價":27.0,"交易量":1230.0}
	]`)
	writeBronzeSnapshot(t, bronzeDir, "2025-11-20", "snap-a", `[
	 {"交易日期":"114.11.20","種類代碼":"N05","作物代號":"12","作物名稱":"芭樂","市場代號":"108","市場名稱":"台中","上價":30.0,"中價":20.0,"下價":10.0,"平均價":20.0,"交易量":500.0}
	]`)

	client := NewFileClient(legacyDir, bronzeDir)

	days, err := client.FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("FetchRestDays 失敗: %v", err)
	}
	codes := make(map[string]bool)
	for _, d := range days {
		codes[d.MarketCode] = true
	}
	if !codes["104"] || !codes["108"] || len(codes) != 2 {
		t.Errorf("預期 distinct 市場代號為 {104,108}，實得 %+v", days)
	}

	// 刪掉來源檔案：第二次呼叫若還能拿到相同結果，證明是走快取，不是重新掃描。
	if err := os.RemoveAll(legacyDir); err != nil {
		t.Fatalf("清除 fixture 失敗: %v", err)
	}
	if err := os.RemoveAll(bronzeDir); err != nil {
		t.Fatalf("清除 fixture 失敗: %v", err)
	}
	daysAgain, err := client.FetchRestDays(context.Background())
	if err != nil {
		t.Fatalf("第二次 FetchRestDays 失敗（不應該真的重新掃描）: %v", err)
	}
	if len(daysAgain) != len(days) {
		t.Errorf("第二次應從快取回傳相同結果，實得 %+v", daysAgain)
	}
}

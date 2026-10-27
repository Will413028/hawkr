package importraw

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
)

// FileClient 是 priceClient 的檔案版實作：FetchRange 讀本機 legacy/bronze
// JSON 檔案而非打 MOA API，FetchRestDays 回傳從檔案掃描出的市場清單（供
// PriceIngest.listMarketCodes 沿用既有的市場列舉邏輯，不依賴任何網路呼叫）。
//
// legacyDir 是 price/ 目錄（其下為 date=YYYY-MM-DD/data.json，西元日期）；
// bronzeDir 是 bronze/source=afa_legacy/dataset=farm_trans_data/ 目錄（其下為
// partition=YYYY-MM-DD/snapshot=*/payload.json，民國日期、可能多個分頁
// snapshot）。同一天兩邊都有資料時（交界日）全部 union，重疊由下游
// PriceIngest.ingestBatch 的 digest 去重吸收，這裡不用特別處理。
//
// 非執行緒安全：dayCache/marketCodes/marketNames 皆為裸 map/slice、無鎖保護。
// PriceIngest.Handle 對市場的處理是循序執行，目前這樣沒問題；若未來把市場
// 迴圈平行化，FileClient 需要額外加鎖。
type FileClient struct {
	legacyDir string
	bronzeDir string

	dayCache    map[string][]moa.TradeRecord
	marketCodes []string
	marketNames map[string]string
}

// NewFileClient 建構 FileClient。
func NewFileClient(legacyDir, bronzeDir string) *FileClient {
	return &FileClient{
		legacyDir: legacyDir,
		bronzeDir: bronzeDir,
		dayCache:  make(map[string][]moa.TradeRecord),
	}
}

// FetchRange 逐日走訪 [start, end]（含頭尾），依 marketCode 過濾回傳。
// cropCode 永遠忽略——呼叫端（PriceIngest.Handle 的市場迴圈）只按市場過濾，
// 與既有 moa.Client.FetchRange 的呼叫慣例一致。
func (c *FileClient) FetchRange(ctx context.Context, start, end time.Time, cropCode, marketCode string) ([]moa.TradeRecord, error) {
	var result []moa.TradeRecord
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dayRecords, err := c.loadDay(d.Format("2006-01-02"))
		if err != nil {
			return nil, err
		}
		for _, r := range dayRecords {
			if r.MarketCode == marketCode {
				result = append(result, r)
			}
		}
	}
	return result, nil
}

// FetchRestDays 回傳從 legacy/bronze 檔案掃描出的 distinct 市場清單，包成
// []moa.RestDay 供 listMarketCodes 沿用（Date 欄位填零值即可——
// listMarketCodes 只讀 .MarketCode，不使用 Date）。首次呼叫時掃描全部已知
// 日期並快取，之後直接回傳快取結果。
//
// 🔴 FetchRestDays 的回傳值只保證 .MarketCode 正確；.Date/.MarketType 皆為
// 零值，僅供 PriceIngest.listMarketCodes 使用（該函式只讀 .MarketCode）。
// FileClient 不得注入 RestDayIngest——那支 usecase 會實際讀取 .Date/.MarketType
// 寫入 market_rest_day，零值會寫出錯誤的休市日曆。
func (c *FileClient) FetchRestDays(ctx context.Context) ([]moa.RestDay, error) {
	if c.marketCodes != nil {
		return c.syntheticRestDays(), nil
	}

	dates, err := c.allKnownDates()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]string)
	for _, date := range dates {
		records, err := c.loadDay(date)
		if err != nil {
			return nil, err
		}
		for _, r := range records {
			if _, ok := seen[r.MarketCode]; !ok {
				seen[r.MarketCode] = r.MarketName
			}
		}
	}

	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	c.marketCodes = codes
	c.marketNames = seen
	return c.syntheticRestDays(), nil
}

func (c *FileClient) syntheticRestDays() []moa.RestDay {
	days := make([]moa.RestDay, 0, len(c.marketCodes))
	for _, code := range c.marketCodes {
		days = append(days, moa.RestDay{MarketCode: code, MarketName: c.marketNames[code]})
	}
	return days
}

// allKnownDates 回傳 legacy 與 bronze 兩邊目錄下所有出現過的日期字串（union、
// 排序），供 FetchRestDays 逐日掃描用。
func (c *FileClient) allKnownDates() ([]string, error) {
	seen := make(map[string]struct{})

	legacyMatches, err := filepath.Glob(filepath.Join(c.legacyDir, "date=*"))
	if err != nil {
		return nil, fmt.Errorf("掃描 legacy 目錄失敗: %w", err)
	}
	for _, m := range legacyMatches {
		seen[strings.TrimPrefix(filepath.Base(m), "date=")] = struct{}{}
	}

	bronzeMatches, err := filepath.Glob(filepath.Join(c.bronzeDir, "partition=*"))
	if err != nil {
		return nil, fmt.Errorf("掃描 bronze 目錄失敗: %w", err)
	}
	for _, m := range bronzeMatches {
		seen[strings.TrimPrefix(filepath.Base(m), "partition=")] = struct{}{}
	}

	dates := make([]string, 0, len(seen))
	for d := range seen {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates, nil
}

// loadDay 回傳指定日期（YYYY-MM-DD）的全部原始列，legacy 與 bronze 兩邊皆有
// 的話 union 起來。結果快取在 dayCache，同一天被不同市場重複查詢
// （FetchRange 逐市場呼叫）時直接命中快取，不重新讀檔解析。
func (c *FileClient) loadDay(date string) ([]moa.TradeRecord, error) {
	if cached, ok := c.dayCache[date]; ok {
		return cached, nil
	}

	var records []moa.TradeRecord

	legacyPath := filepath.Join(c.legacyDir, "date="+date, "data.json")
	if data, err := os.ReadFile(legacyPath); err == nil {
		recs, err := parseRawRecords(data)
		if err != nil {
			return nil, fmt.Errorf("解析 legacy %s 失敗: %w", date, err)
		}
		records = append(records, recs...)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("讀取 legacy %s 失敗: %w", date, err)
	}

	bronzeMatches, err := filepath.Glob(filepath.Join(c.bronzeDir, "partition="+date, "snapshot=*", "payload.json"))
	if err != nil {
		return nil, fmt.Errorf("掃描 bronze %s 失敗: %w", date, err)
	}
	for _, m := range bronzeMatches {
		data, err := os.ReadFile(m)
		if err != nil {
			return nil, fmt.Errorf("讀取 bronze %s 失敗: %w", m, err)
		}
		recs, err := parseRawRecords(data)
		if err != nil {
			return nil, fmt.Errorf("解析 bronze %s 失敗: %w", m, err)
		}
		records = append(records, recs...)
	}

	c.dayCache[date] = records
	return records, nil
}

func parseRawRecords(data []byte) ([]moa.TradeRecord, error) {
	var raws []rawRecord
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, err
	}
	records := make([]moa.TradeRecord, 0, len(raws))
	for _, r := range raws {
		tr, err := r.toTradeRecord()
		if err != nil {
			return nil, err
		}
		records = append(records, tr)
	}
	return records, nil
}

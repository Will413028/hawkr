// Package importraw 把 資料團隊提供的 legacy/bronze 原始 JSON 匯出檔轉成
// moa.TradeRecord，供 FileClient（priceClient 的檔案版實作）重用
// usecase.PriceIngest.Handle/Backfill.Handle 的既有正確性邏輯寫入
// price_observation，完全不打 MOA API。
package importraw

import (
	"fmt"
	"strings"
	"time"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
)

// rawRecord 對應 legacy 與 bronze 兩種來源共用的中文欄位 JSON 結構——兩者
// 欄位名稱完全相同，差別只在 TransDate 的日期格式（legacy 是西元
// YYYY-MM-DD，bronze 是民國 YYY.MM.DD，moa.TradeRecord 統一用民國格式，
// 與 moa.ParseROCDate 的解析邏輯對齊）。
type rawRecord struct {
	TransDate     string  `json:"交易日期"`
	TcType        string  `json:"種類代碼"`
	CropCode      string  `json:"作物代號"`
	CropName      string  `json:"作物名稱"`
	MarketCode    string  `json:"市場代號"`
	MarketName    string  `json:"市場名稱"`
	UpperPrice    float64 `json:"上價"`
	MiddlePrice   float64 `json:"中價"`
	LowerPrice    float64 `json:"下價"`
	AvgPrice      float64 `json:"平均價"`
	TransQuantity float64 `json:"交易量"`
}

// toTradeRecord 轉成 moa.TradeRecord。TransDate 有兩種可能格式，且不是單純以
// 資料來源（legacy/bronze）或日期區間切分——實測 legacy 封存 2022-08-12~
// 2022-09-29 是西元格式，但 2022-09-30 起卻混雜出現民國格式（同一份封存內
// 兩種格式交錯），bronze 封存則全部是民國格式。用內容本身判斷格式（含 "-"
// 視為西元、否則視為民國並驗證是合法的民國日期），不靠呼叫端傳一個「這是
// legacy 還是 bronze」的旗標——那個假設在真實資料上不成立。moa.TradeRecord
// 統一用民國格式，與 moa.ParseROCDate 的解析邏輯對齊。
func (r rawRecord) toTradeRecord() (moa.TradeRecord, error) {
	transDate := r.TransDate
	if strings.Contains(transDate, "-") {
		t, err := time.Parse("2006-01-02", transDate)
		if err != nil {
			return moa.TradeRecord{}, fmt.Errorf("西元格式 TransDate 解析失敗 %q: %w", r.TransDate, err)
		}
		transDate = moa.ToROCDate(t)
	} else if _, err := moa.ParseROCDate(transDate); err != nil {
		return moa.TradeRecord{}, fmt.Errorf("無法辨識的 TransDate 格式 %q: %w", r.TransDate, err)
	}
	return moa.TradeRecord{
		TransDate:     transDate,
		TcType:        r.TcType,
		CropCode:      r.CropCode,
		CropName:      r.CropName,
		MarketCode:    r.MarketCode,
		MarketName:    r.MarketName,
		UpperPrice:    r.UpperPrice,
		MiddlePrice:   r.MiddlePrice,
		LowerPrice:    r.LowerPrice,
		AvgPrice:      r.AvgPrice,
		TransQuantity: r.TransQuantity,
	}, nil
}

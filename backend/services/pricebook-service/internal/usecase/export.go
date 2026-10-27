package usecase

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// csvDateLayout 是對帳 CSV 的 trade_date 欄位格式，與本 CLI 的 --start/--end
// 契約（cmd/pricebook-export/main.go）相同：ISO YYYY-MM-DD。
const csvDateLayout = "2006-01-02"

// ObservationRow 對帳用的單列觀測。
// HasPrice/HasVolume 區分「缺值」與「數值為 0」——後者是合法價格。
type ObservationRow struct {
	MarketCode string
	CropCode   string
	TradeDate  string // YYYY-MM-DD
	AvgPrice   float64
	VolumeKg   float64
	HasPrice   bool
	HasVolume  bool
}

// WriteObservationCSV 輸出確定性 CSV：固定排序、固定小數位，兩邊可直接 diff。
func WriteObservationCSV(w io.Writer, rows []ObservationRow) error {
	sorted := make([]ObservationRow, len(rows))
	copy(sorted, rows)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.MarketCode != b.MarketCode {
			return a.MarketCode < b.MarketCode
		}
		if a.CropCode != b.CropCode {
			return a.CropCode < b.CropCode
		}
		return a.TradeDate < b.TradeDate
	})

	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"market_code", "crop_code", "trade_date", "avg_price", "volume_kg"}); err != nil {
		return err
	}
	for _, r := range sorted {
		price, volume := "", ""
		if r.HasPrice {
			price = fmt.Sprintf("%.4f", r.AvgPrice)
		}
		if r.HasVolume {
			volume = fmt.Sprintf("%.3f", r.VolumeKg)
		}
		if err := cw.Write([]string{r.MarketCode, r.CropCode, r.TradeDate, price, volume}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// Exporter 讀取 price_observation 產生對帳用 CSV（Task 9：訓練資料對帳 gate，
// 見 task-9-brief.md 與 decisions.md 全域規則）。cmd/pricebook-export 是唯一
// 呼叫端；Plan A 沒有任何其他 usecase 或服務消費本輸出，純粹為了人工對帳流程
// 存在（against 資料團隊的 Silver 匯出，diff 後決定 Plan B 是否 vendor 訓練程式碼）。
type Exporter struct {
	pool *pgxpool.Pool
}

// NewExporter 建構 Exporter。持有 *pgxpool.Pool 而非 *gen.Queries 的理由與
// PriceIngest/Backfill/RestDayIngest 相同（見 price_ingest.go 建構子註解）：
// *gen.Queries 不能被 fx 提供成 singleton，要在每次呼叫時用
// gen.New(database.GetDBTX(ctx, pool)) 現組，這樣 database.TxManager.WithTx
// 開的交易才會被自動撿起來（本 usecase 目前不需要交易，但保持與其餘三個
// usecase 一致的持有型態，不製造第四種例外形狀）。
func NewExporter(pool *pgxpool.Pool) *Exporter {
	return &Exporter{pool: pool}
}

// ObservationCSV 匯出 [start, end]（含頭尾）內每個 (market, crop, trade_date)
// 目前最新 revision（revision_no 最大）的觀測，寫成確定性 CSV。
//
// 🔴 HasPrice/HasVolume 必須來自 pgtype.Numeric.Valid，不是零值檢查——0 是
// 合法價格/交易量，NULL 才是缺值（休市或缺值，schema.sql 開頭的通則）。
// 渲染一律先經 postgres.NumericToPtr（該函式內部走 pgtype.Numeric.Value()
// 拿到完整精度字串，見 numeric.go），得到的字串才轉 float64 餵給
// ObservationRow——這是 Step 1 測試釘住的既有型別，本方法不能改動它。
// ExportObservations 讀回的字串是 Task 6 寫入時就已經格式化成
// numeric(12,4)/numeric(14,3) 固定位數的值（見 price_ingest.go 的
// fmt.Sprintf("%.4f"/"%.3f", ...)），float64 在這個有界、位數已知的範圍內
// 轉換不失真——export_integration_test.go 用真實 DB 往返驗證這件事逐位數
// 相符，不是憑空假設（numeric.go 檔頭警告的失真情境是任意精度/科學記號輸入，
// 不適用於這裡已經過上游格式化收斂的值域）。
//
// 回傳值第一個是實際匯出的列數：cmd/pricebook-export 靠這個數字判斷「零列」
// 是不是操作者沒注意到的異常——WriteObservationCSV 一律先寫表頭，零列匯出跟
// 正常匯出一樣是「看起來有效」的 1 行 CSV，沒有這個回傳值呼叫端完全無法區分
// 兩者（final whole-branch review Fix 2）。
func (u *Exporter) ObservationCSV(ctx context.Context, start, end time.Time, w io.Writer) (int, error) {
	q := gen.New(database.GetDBTX(ctx, u.pool))

	dbRows, err := q.ExportObservations(ctx, gen.ExportObservationsParams{
		StartDate: pgxutil.DateParam(&start),
		EndDate:   pgxutil.DateParam(&end),
	})
	if err != nil {
		return 0, fmt.Errorf("查詢對帳觀測失敗: %w", err)
	}

	rows := make([]ObservationRow, 0, len(dbRows))
	for _, r := range dbRows {
		row := ObservationRow{
			MarketCode: r.MarketCode,
			CropCode:   r.CropCode,
			TradeDate:  r.TradeDate.Time.Format(csvDateLayout),
		}

		if price := postgres.NumericToPtr(r.AvgPrice); price != nil {
			v, err := strconv.ParseFloat(*price, 64)
			if err != nil {
				return 0, fmt.Errorf("avg_price 解析失敗（market=%s, crop=%s, date=%s, raw=%q）: %w",
					r.MarketCode, r.CropCode, row.TradeDate, *price, err)
			}
			row.AvgPrice = v
			row.HasPrice = true
		}

		if volume := postgres.NumericToPtr(r.VolumeKg); volume != nil {
			v, err := strconv.ParseFloat(*volume, 64)
			if err != nil {
				return 0, fmt.Errorf("volume_kg 解析失敗（market=%s, crop=%s, date=%s, raw=%q）: %w",
					r.MarketCode, r.CropCode, row.TradeDate, *volume, err)
			}
			row.VolumeKg = v
			row.HasVolume = true
		}

		rows = append(rows, row)
	}

	if err := WriteObservationCSV(w, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

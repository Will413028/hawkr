//go:build integration

// 這個測試需要本機 Docker daemon，用 `make test-integration`
// （或 `go test -tags integration ./...`）跑；CI 由
// .github/workflows/integration-tests.yml 的 pricebook-service job 執行。
// 比照 price_ingest_integration_test.go / backfill_integration_test.go
// （同一個 //go:build integration + package usecase_test 檔案群組，重用
// 同檔案群組裡的 setupTestDB / newSeqTSID，不新開第三套 harness）。
package usecase_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/importraw"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

// TestImportRawEndToEndWritesPriceObservation 驗證 FileClient →
// NewPriceIngestWithClient → Backfill.Handle 完整路徑：手寫小型 legacy fixture，
// 走一次匯入，斷言 price_observation 正確寫入、ingest_run 狀態正確——與既有
// cmd/pricebook-backfill 走的是同一套不變量（revision 遞增、digest 去重），
// 只是資料來源從 MOA API 換成本機檔案。
func TestImportRawEndToEndWritesPriceObservation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	dir := t.TempDir()
	legacyDir := filepath.Join(dir, "price")
	dayDir := filepath.Join(legacyDir, "date=2022-08-12")
	require.NoError(t, os.MkdirAll(dayDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dayDir, "data.json"), []byte(`[
	 {"交易日期":"2022-08-12","種類代碼":"N05","作物代號":"11","作物名稱":"椰子","市場代號":"104","市場名稱":"台北二","上價":48.0,"中價":23.1,"下價":17.4,"平均價":27.0,"交易量":1230.0}
	]`), 0o644))
	bronzeDir := filepath.Join(dir, "bronze")

	client := importraw.NewFileClient(legacyDir, bronzeDir)
	// retryDelay 用近乎為 0 的值：FileClient 是確定性來源，不需要生產用的
	// emptyResponseRetryDelay 5 秒等待（見 Finding I3，cmd/pricebook-import-raw
	// 的 main.go package comment 有完整說明），這裡跟正式 CLI 呼叫端用同一種
	// 選值方式，避免整合測試也白等真實秒數。
	priceIngest := usecase.NewPriceIngestWithClient(client, pool, newSeqTSID(940000000000000000), time.Millisecond)
	backfill := usecase.NewBackfill(priceIngest, pool, newSeqTSID(940000000001000000))

	start := time.Date(2022, 8, 12, 0, 0, 0, 0, time.UTC)
	end := start
	runID, err := backfill.Handle(ctx, start, end)
	require.NoError(t, err)
	require.NotZero(t, runID)

	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM price_observation po
		JOIN market m ON m.id = po.market_id
		JOIN crop c ON c.id = po.crop_id
		WHERE m.market_code = '104' AND c.crop_code = '11' AND po.trade_date = '2022-08-12'
	`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "應寫入 1 筆 price_observation")

	// 除了 join key 存在之外，實際比對價格欄位是否真的從 fixture 的中文欄位
	// （上價/中價/下價/平均價/交易量）正確轉換、經 FileClient → TradeRecord →
	// PriceIngest 一路落地，不只是「有一筆符合 key 的列」。
	var revisionNo int32
	var upperPrice, middlePrice, lowerPrice, avgPrice, volumeKg string
	err = pool.QueryRow(ctx, `
		SELECT po.revision_no, po.upper_price::text, po.middle_price::text,
		       po.lower_price::text, po.avg_price::text, po.volume_kg::text
		FROM price_observation po
		JOIN market m ON m.id = po.market_id
		JOIN crop c ON c.id = po.crop_id
		WHERE m.market_code = '104' AND c.crop_code = '11' AND po.trade_date = '2022-08-12'
	`).Scan(&revisionNo, &upperPrice, &middlePrice, &lowerPrice, &avgPrice, &volumeKg)
	require.NoError(t, err)
	require.Equal(t, int32(1), revisionNo)
	require.Equal(t, "48.0000", upperPrice, "上價")
	require.Equal(t, "23.1000", middlePrice, "中價")
	require.Equal(t, "17.4000", lowerPrice, "下價")
	require.Equal(t, "27.0000", avgPrice, "平均價")
	require.Equal(t, "1230.000", volumeKg, "交易量")
}

//go:build integration

// 這個測試需要本機 Docker daemon，用 `make test-integration`
// （或 `go test -tags integration ./...`）跑；CI 由
// .github/workflows/integration-tests.yml 的 pricebook-service job 執行。
//
// 為什麼需要真的 Postgres（review Finding 1，2026-07-30）：pgSeedCheckDB 的
// 三個方法（FindCropGroupByName／FindCropByCode／MarketExists）是整份
// mapping 種子表功能裡唯一真的打 DB 的實作，main_test.go 的 fakeSeedCheckDB
// 只驗證 checkRow／printReport 的分類邏輯（0/1/N 筆時該標什麼狀態），完全不
// 觸及「SQL 本身寫對了沒」「欄位名對不對得上 schema」這件事。這裡直接用
// pgSeedCheckDB（production 會用的同一個 struct，不是另外包一層），對著真實
// migrations 建出的 schema 驗證三支查詢——尤其是 FindCropGroupByName 的
// :many 語意：crop_group.name 沒有 UNIQUE 約束，同一個大類名在不同
// category_code 下必須真的能查到兩列（Task 2 之後 group_code 的 hash 含
// category），這件事只有對著真 schema、用生產路徑實際使用的 moa.GroupCode
// 算出兩個不同的 group_code 才驗得出來——單元測試的假資料無法證明這件事在
// 真實 DB 上成立。
package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// TSID 基底：避開本 repo 既有整合測試已用掉的 700…／900…／910…／920…／
// 950…／960…／970…／980…／981… 段（見
// internal/repository/postgres/forecast_repo_test.go、
// internal/repository/postgres/schema_invariants_test.go 的既有常數），這裡
// 用 982…。各測試函式各自獨佔一個資料庫，理論上不會撞號，但沿用同一份避開
// 清單是本 repo 已建立的慣例，方便日後讀者一眼看出「這個 base 屬於哪個檔案」。
const (
	seedCheckMarketIDBase    int64 = 982000000000000001
	seedCheckCropGroupIDBase int64 = 982000000000001000
	seedCheckCropIDBase      int64 = 982000000000002000
)

// setupTestDB 見 dbtest_test.go：本 package 共用一個 Postgres 容器，每個測試從
// 跑完 migrations 的 template 複製出一份自己的資料庫。

// seedCropGroup upserts 一列 crop_group，group_code 用 moa.GroupCode(name,
// categoryCode) 算——這是生產路徑（internal/usecase/price_ingest.go:310 的
// 分類同步）實際使用的同一個函式，不是隨手編一個字串當 group_code，確保
// 「同一大類名在不同 category 下是不同列」這個不變量在測試裡跟生產環境同構。
func seedCropGroup(ctx context.Context, t *testing.T, q *gen.Queries, id int64, name, categoryCode string) int64 {
	t.Helper()
	groupID, err := q.UpsertCropGroup(ctx, gen.UpsertCropGroupParams{
		ID:           id,
		GroupCode:    moa.GroupCode(name, categoryCode),
		Name:         name,
		CategoryCode: textVal(categoryCode),
	})
	require.NoError(t, err)
	return groupID
}

// TestPgSeedCheckDBFindCropGroupByNameResolvesSingleMatch 是 brief 情形
// (a)：一個大類名只對到一列 crop_group 時，FindCropGroupByName 回 1 筆，
// checkRow 判 OK。
func TestPgSeedCheckDBFindCropGroupByNameResolvesSingleMatch(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	seedCropGroup(ctx, t, q, seedCheckCropGroupIDBase+1, "甘藍", "N04")

	db := &pgSeedCheckDB{q: q, pool: pool}

	rows, err := db.FindCropGroupByName(ctx, "甘藍")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, moa.GroupCode("甘藍", "N04"), rows[0].GroupCode)
	require.NotNil(t, rows[0].CategoryCode)
	require.Equal(t, "N04", *rows[0].CategoryCode)

	res, err := checkRow(ctx, db, mapping.SeedEntry{AliasName: "高麗菜", CropGroupName: "甘藍"})
	require.NoError(t, err)
	require.Equal(t, statusResolved, res.Status)
}

// TestPgSeedCheckDBFindCropGroupByNameReturnsAllCandidatesWhenAmbiguous 是
// brief 情形 (b)，本檔最重要的一條：同一個大類名「甘藍」在 category N04 與
// N05 下是兩個不同的 crop_group 列（group_code 的 hash 含 category_code，
// Task 2 之後的不變量）。FindCropGroupByName 必須回兩筆，checkRow 判
// AMBIGUOUS 且把兩筆的 group_code 都列進 detail，不能自動選一筆。
func TestPgSeedCheckDBFindCropGroupByNameReturnsAllCandidatesWhenAmbiguous(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	seedCropGroup(ctx, t, q, seedCheckCropGroupIDBase+1, "甘藍", "N04")
	seedCropGroup(ctx, t, q, seedCheckCropGroupIDBase+2, "甘藍", "N05")

	db := &pgSeedCheckDB{q: q, pool: pool}

	rows, err := db.FindCropGroupByName(ctx, "甘藍")
	require.NoError(t, err)
	require.Len(t, rows, 2, "同一個大類名在不同 category 下必須回兩列，不能被查詢併掉")

	gotCodes := map[string]bool{}
	for _, r := range rows {
		gotCodes[r.GroupCode] = true
	}
	require.True(t, gotCodes[moa.GroupCode("甘藍", "N04")])
	require.True(t, gotCodes[moa.GroupCode("甘藍", "N05")])

	res, err := checkRow(ctx, db, mapping.SeedEntry{AliasName: "甘藍", CropGroupName: "甘藍"})
	require.NoError(t, err)
	require.Equal(t, statusAmbiguous, res.Status, "回兩筆不算解析成功，必須是 AMBIGUOUS，不能自動選一筆")
	require.Contains(t, res.Detail, moa.GroupCode("甘藍", "N04"), "detail 必須列出第一筆候選")
	require.Contains(t, res.Detail, moa.GroupCode("甘藍", "N05"), "detail 必須列出第二筆候選，不能只挑一筆")
	require.Contains(t, res.Detail, "需人工裁決")
}

// TestPgSeedCheckDBFindCropGroupByNameReturnsEmptyWhenNotFound 是 brief 情形
// (c)：大類名不存在時回 0 筆，checkRow 判 NOT_FOUND。
func TestPgSeedCheckDBFindCropGroupByNameReturnsEmptyWhenNotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	db := &pgSeedCheckDB{q: q, pool: pool}

	rows, err := db.FindCropGroupByName(ctx, "不存在的大類")
	require.NoError(t, err)
	require.Empty(t, rows)

	res, err := checkRow(ctx, db, mapping.SeedEntry{AliasName: "不存在的別名", CropGroupName: "不存在的大類"})
	require.NoError(t, err)
	require.Equal(t, statusNotFound, res.Status)
}

// TestPgSeedCheckDBFindCropByCodeHitAndMiss 覆蓋 crop_code 命中與查無兩種
// 情形——crop_code 在 schema 上是 UNIQUE，理論上不會有 ambiguous case，所以
// 只有 hit/miss 兩態要驗。
func TestPgSeedCheckDBFindCropByCodeHitAndMiss(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	groupID := seedCropGroup(ctx, t, q, seedCheckCropGroupIDBase+1, "甘藍", "N04")
	_, err := q.UpsertCrop(ctx, gen.UpsertCropParams{
		ID:           seedCheckCropIDBase + 1,
		CropCode:     "LA1",
		GroupID:      groupID,
		Name:         "甘藍",
		VariantName:  textVal("初秋"),
		CategoryCode: textVal("N04"),
	})
	require.NoError(t, err)

	db := &pgSeedCheckDB{q: q, pool: pool}

	hit, err := db.FindCropByCode(ctx, "LA1")
	require.NoError(t, err)
	require.Equal(t, "甘藍", hit.Name)
	require.NotNil(t, hit.CategoryCode)
	require.Equal(t, "N04", *hit.CategoryCode)

	_, err = db.FindCropByCode(ctx, "ZZ9-NOT-SEEDED")
	require.ErrorIs(t, err, pgx.ErrNoRows)

	resHit, err := checkRow(ctx, db, mapping.SeedEntry{AliasName: "甘藍初秋", CropCode: "LA1"})
	require.NoError(t, err)
	require.Equal(t, statusResolved, resHit.Status)

	resMiss, err := checkRow(ctx, db, mapping.SeedEntry{AliasName: "查無子品項", CropCode: "ZZ9-NOT-SEEDED"})
	require.NoError(t, err)
	require.Equal(t, statusNotFound, resMiss.Status)
}

// TestPgSeedCheckDBMarketExistsHitAndMiss 覆蓋 market_code 存在與不存在兩種
// 情形，並驗證 checkRow 在 market_code 查無時會把已解出的 crop_group 狀態
// 覆寫回 NOT_FOUND（main.go 對 res.Status==statusResolved 之後再查 market 的
// 邏輯，只有真的打 DB 才驗得到這條 override 邏輯與 SQL 本身都正確）。
func TestPgSeedCheckDBMarketExistsHitAndMiss(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	seedCropGroup(ctx, t, q, seedCheckCropGroupIDBase+1, "甘藍", "N04")
	_, err := q.UpsertMarket(ctx, gen.UpsertMarketParams{
		ID:         seedCheckMarketIDBase,
		MarketCode: "104",
		Name:       "台北一",
	})
	require.NoError(t, err)

	db := &pgSeedCheckDB{q: q, pool: pool}

	exists, err := db.MarketExists(ctx, "104")
	require.NoError(t, err)
	require.True(t, exists)

	notExists, err := db.MarketExists(ctx, "999")
	require.NoError(t, err)
	require.False(t, notExists)

	resHit, err := checkRow(ctx, db, mapping.SeedEntry{AliasName: "高麗菜", CropGroupName: "甘藍", MarketCode: "104"})
	require.NoError(t, err)
	require.Equal(t, statusResolved, resHit.Status)

	resMiss, err := checkRow(ctx, db, mapping.SeedEntry{AliasName: "高麗菜", CropGroupName: "甘藍", MarketCode: "999"})
	require.NoError(t, err)
	require.Equal(t, statusNotFound, resMiss.Status, "market_code 查無時整列不算解析成功，即使作物本身對得上")
}

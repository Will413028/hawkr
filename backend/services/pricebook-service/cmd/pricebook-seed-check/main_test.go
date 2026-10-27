package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

// fakeSeedCheckDB 是 seedCheckDB 的假實作，讓 checkRow／printReport 的邏輯
// （尤其是 AMBIGUOUS 分支）不需要真的起一個 Postgres 就能單元測試。
type fakeSeedCheckDB struct {
	groupsByName map[string][]gen.FindCropGroupByNameRow
	cropsByCode  map[string]gen.FindCropByCodeRow
	marketExists map[string]bool
}

func (f *fakeSeedCheckDB) FindCropGroupByName(_ context.Context, name string) ([]gen.FindCropGroupByNameRow, error) {
	return f.groupsByName[name], nil
}

func (f *fakeSeedCheckDB) FindCropByCode(_ context.Context, cropCode string) (gen.FindCropByCodeRow, error) {
	row, ok := f.cropsByCode[cropCode]
	if !ok {
		return gen.FindCropByCodeRow{}, pgx.ErrNoRows
	}
	return row, nil
}

func (f *fakeSeedCheckDB) MarketExists(_ context.Context, marketCode string) (bool, error) {
	return f.marketExists[marketCode], nil
}

func textVal(s string) *string {
	return &s
}

// TestCheckRowCropGroupResolved 驗證 crop_group_name 命中恰一筆時標記 OK，
// detail 帶出 group_code 與 category。
func TestCheckRowCropGroupResolved(t *testing.T) {
	db := &fakeSeedCheckDB{
		groupsByName: map[string][]gen.FindCropGroupByNameRow{
			"甘藍": {{GroupCode: "moa-group:aaa", Name: "甘藍", CategoryCode: textVal("N04")}},
		},
	}
	entry := mapping.SeedEntry{AliasName: "高麗菜", MatchKind: mapping.MatchKindConsumerAlias, CropGroupName: "甘藍"}

	res, err := checkRow(context.Background(), db, entry)
	require.NoError(t, err)
	require.Equal(t, statusResolved, res.Status)
	require.Equal(t, "crop_group:甘藍", res.Target)
	require.Contains(t, res.Detail, "moa-group:aaa")
	require.Contains(t, res.Detail, "N04")
}

// TestCheckRowCropGroupNotFound 驗證 crop_group_name 查無時標記 NOT_FOUND。
func TestCheckRowCropGroupNotFound(t *testing.T) {
	db := &fakeSeedCheckDB{groupsByName: map[string][]gen.FindCropGroupByNameRow{}}
	entry := mapping.SeedEntry{AliasName: "高麗菜", MatchKind: mapping.MatchKindConsumerAlias, CropGroupName: "甘藍"}

	res, err := checkRow(context.Background(), db, entry)
	require.NoError(t, err)
	require.Equal(t, statusNotFound, res.Status)
}

// TestCheckRowCropGroupAmbiguousListsAllRowsAndDoesNotAutoPick 是 Task 13
// brief 明講的核心行為：同一個大類名在不同 category 下是不同的 crop_group
// 列時（例如「甘藍」同時存在於 N04 蔬菜與 N05 水果），CLI 必須把全部命中列都
// 印出來、標記為需人工裁決——不能自己選一筆當作 OK。
func TestCheckRowCropGroupAmbiguousListsAllRowsAndDoesNotAutoPick(t *testing.T) {
	db := &fakeSeedCheckDB{
		groupsByName: map[string][]gen.FindCropGroupByNameRow{
			"甘藍": {
				{GroupCode: "moa-group:n04xxx", Name: "甘藍", CategoryCode: textVal("N04")},
				{GroupCode: "moa-group:n05yyy", Name: "甘藍", CategoryCode: textVal("N05")},
			},
		},
	}
	entry := mapping.SeedEntry{AliasName: "甘藍", MatchKind: mapping.MatchKindCategoryL3, CropGroupName: "甘藍"}

	res, err := checkRow(context.Background(), db, entry)
	require.NoError(t, err)
	require.Equal(t, statusAmbiguous, res.Status, "回多筆不算解析成功，必須是 AMBIGUOUS 而不是 OK")
	require.Contains(t, res.Detail, "moa-group:n04xxx", "必須印出第一筆的 group_code")
	require.Contains(t, res.Detail, "N04")
	require.Contains(t, res.Detail, "moa-group:n05yyy", "必須印出第二筆的 group_code，不能只挑一筆")
	require.Contains(t, res.Detail, "N05")
	require.Contains(t, res.Detail, "需人工裁決")
}

// TestCheckRowCropCodeResolvedAndNotFound 驗證 crop_code 分支的 OK／NOT_FOUND
// （crop_code 在 schema 上是 UNIQUE，理論上不會有 AMBIGUOUS）。
func TestCheckRowCropCodeResolvedAndNotFound(t *testing.T) {
	db := &fakeSeedCheckDB{
		cropsByCode: map[string]gen.FindCropByCodeRow{
			"LA1": {CropCode: "LA1", Name: "甘藍-初秋", CategoryCode: textVal("N04")},
		},
	}

	resolved, err := checkRow(context.Background(), db, mapping.SeedEntry{AliasName: "甘藍初秋", CropCode: "LA1"})
	require.NoError(t, err)
	require.Equal(t, statusResolved, resolved.Status)
	require.Equal(t, "crop:LA1", resolved.Target)
	require.Contains(t, resolved.Detail, "甘藍-初秋")

	notFound, err := checkRow(context.Background(), db, mapping.SeedEntry{AliasName: "不存在的子品項", CropCode: "ZZ9"})
	require.NoError(t, err)
	require.Equal(t, statusNotFound, notFound.Status)
}

// TestCheckRowMarketCodeOverridesResolvedToNotFound 驗證即使
// crop_group_name／crop_code 唯一命中，market_code 若非空且查無，整列仍不算
// 解析成功。
func TestCheckRowMarketCodeOverridesResolvedToNotFound(t *testing.T) {
	db := &fakeSeedCheckDB{
		groupsByName: map[string][]gen.FindCropGroupByNameRow{
			"甘藍": {{GroupCode: "moa-group:aaa", Name: "甘藍", CategoryCode: textVal("N04")}},
		},
		marketExists: map[string]bool{"104": true},
	}

	ok := mapping.SeedEntry{AliasName: "高麗菜", CropGroupName: "甘藍", MarketCode: "104"}
	res, err := checkRow(context.Background(), db, ok)
	require.NoError(t, err)
	require.Equal(t, statusResolved, res.Status)

	missingMarket := mapping.SeedEntry{AliasName: "高麗菜", CropGroupName: "甘藍", MarketCode: "999"}
	res2, err := checkRow(context.Background(), db, missingMarket)
	require.NoError(t, err)
	require.Equal(t, statusNotFound, res2.Status, "市場查無時，即使作物本身對得上，整列也不算解析成功")
	require.Contains(t, res2.Detail, "999")
}

// TestCheckRowPropagatesDBError 驗證 DB 查詢本身出錯（非「查無資料」）時
// checkRow 把錯誤原樣往外傳，不會誤判成 NOT_FOUND。
func TestCheckRowPropagatesDBError(t *testing.T) {
	boom := errors.New("boom: connection reset")
	db := &erroringSeedCheckDB{err: boom}
	entry := mapping.SeedEntry{AliasName: "高麗菜", CropGroupName: "甘藍"}

	_, err := checkRow(context.Background(), db, entry)
	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}

type erroringSeedCheckDB struct{ err error }

func (e *erroringSeedCheckDB) FindCropGroupByName(context.Context, string) ([]gen.FindCropGroupByNameRow, error) {
	return nil, e.err
}
func (e *erroringSeedCheckDB) FindCropByCode(context.Context, string) (gen.FindCropByCodeRow, error) {
	return gen.FindCropByCodeRow{}, e.err
}
func (e *erroringSeedCheckDB) MarketExists(context.Context, string) (bool, error) {
	return false, e.err
}

// TestPrintReportCountsOnlyResolvedAndListsAmbiguousDetail 驗證
// printReport 的摘要計數只算 OK，且輸出裡看得到 AMBIGUOUS 列的完整候選清單
// ——用真的 mapping.Seed（透過 LoadSeed 讀 repo 內實際的種子表）驅動，
// 確保這條路徑跟真正的 CLI 執行路徑一致，只有 DB 層是假的。
func TestPrintReportCountsOnlyResolvedAndListsAmbiguousDetail(t *testing.T) {
	seed, err := mapping.LoadSeed("../../reference/product_crop_seed.csv", "../../reference/product_crop_seed.manifest.json")
	require.NoError(t, err)

	db := &fakeSeedCheckDB{
		groupsByName: map[string][]gen.FindCropGroupByNameRow{},
		cropsByCode:  map[string]gen.FindCropByCodeRow{},
	}
	// 把「甘藍」佈成 ambiguous（兩個 category），其餘 crop_group_name 一律
	// 佈成恰一筆命中，藉此同時驗證「多數列 OK、一列 AMBIGUOUS」時摘要計數
	// 是否正確排除了那一列。
	for _, entry := range seed.Entries() {
		if entry.CropGroupName == "" {
			continue
		}
		if entry.CropGroupName == "甘藍" {
			db.groupsByName[entry.CropGroupName] = []gen.FindCropGroupByNameRow{
				{GroupCode: "moa-group:n04", Name: "甘藍", CategoryCode: textVal("N04")},
				{GroupCode: "moa-group:n05", Name: "甘藍", CategoryCode: textVal("N05")},
			}
			continue
		}
		if _, seeded := db.groupsByName[entry.CropGroupName]; !seeded {
			db.groupsByName[entry.CropGroupName] = []gen.FindCropGroupByNameRow{
				{GroupCode: "moa-group:" + entry.CropGroupName, Name: entry.CropGroupName, CategoryCode: textVal("N04")},
			}
		}
	}

	var buf bytes.Buffer
	resolved, err := printReport(context.Background(), &buf, db, seed)
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "AMBIGUOUS")
	require.Contains(t, out, "moa-group:n04")
	require.Contains(t, out, "moa-group:n05")

	// 種子表裡 alias_name=甘藍／高麗菜 都指向 crop_group_name=甘藍，兩列都會
	// 因為 ambiguous 而不算解析成功；其餘所有列都佈成恰一筆命中。
	ambiguousAliasCount := 0
	for _, e := range seed.Entries() {
		if e.CropGroupName == "甘藍" {
			ambiguousAliasCount++
		}
	}
	require.Equal(t, seed.Len()-ambiguousAliasCount, resolved,
		"resolved 計數必須排除 AMBIGUOUS 的列，不能把「有命中（即使多筆）」誤算成功")
}

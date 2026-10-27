// pricebook-geography-check 是人工複核工具：印出 market geography registry
// 每個 active 設施解析出的座標、縣市、review-required 旗標。不是部署閘門，
// 比照 cmd/pricebook-seed-check 的既有慣例——這支不需要連 DB（地理登錄表是
// 純鏡射檔查詢，沒有跟本服務 DB 交叉驗證的需求，見設計書 Testing 段）。
//
// 跑法（從 services/pricebook-service/ 目錄）：
//
//	go run ./cmd/pricebook-geography-check
package main

import (
	"fmt"
	"os"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	registry, err := geography.Load(
		"reference/market_registry.v1.csv",
		"reference/market_registry.v1.manifest.json",
		"reference/taiwan_boundary.taiwan-atlas-2021.9.20.geojson",
		"reference/taiwan_boundary.v1.manifest.json",
	)
	if err != nil {
		return fmt.Errorf("載入 geography registry 失敗: %w", err)
	}

	facilities := registry.ResolvedFacilities()
	fmt.Printf("%d 個 active 設施：\n\n", len(facilities))

	resolved, unresolved := 0, 0
	for _, f := range facilities {
		review := ""
		if f.NeedsCoordinateReview() {
			review = "  [NEEDS REVIEW]"
		}
		if f.CountyCode == "" {
			unresolved++
			fmt.Printf("MISSING COUNTY  facility_id=%-30s market_code=%-6s categories=%v lon=%s lat=%s%s\n",
				f.FacilityID, f.MarketCode, f.CategoryCodes, floatOrDash(f.Longitude), floatOrDash(f.Latitude), review)
			continue
		}
		resolved++
		fmt.Printf("OK              facility_id=%-30s market_code=%-6s categories=%v -> county=%s (%s)%s\n",
			f.FacilityID, f.MarketCode, f.CategoryCodes, f.CountyCode, f.CountyName, review)
	}

	fmt.Printf("\n%d resolved, %d missing county\n", resolved, unresolved)
	return nil
}

func floatOrDash(f *float64) string {
	if f == nil {
		return "-"
	}
	return fmt.Sprintf("%v", *f)
}

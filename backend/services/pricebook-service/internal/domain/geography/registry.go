// Package geography 載入並查詢「(market_code, category_code, valid_on) →
// 實體市場設施座標」的地理登錄表——issue #2248（spatial-surface BFF）與
// #2249 Slice C（regional-comparison）都依賴這一層才能繼續設計。
//
// 範圍邊界（讀這段前不要假設本套件做得比這裡寫的多）：
//   - 只有 registry/boundary 兩份鏡射檔的載入、驗證、查詢，與一次性的
//     facility→縣市 point-in-polygon 賦值。不含 #2248 的 spatial-surface
//     usecase/proto/gateway，不含 #2249 Slice C 的 regional-comparison
//     usecase，不含行政區價格 area-weighted rollup 本身——這些都是後續
//     子專案的範圍（設計書 Non-Goals）。
//   - CSV／GeoJSON 本身是唯一真相；同目錄的 .manifest.json 只是完整性校驗
//     用的 sha256 + 來源清單，不是另一份資料（同 mapping 套件既有慣例）。
//
// 資料來源：兩份鏡射檔皆逐位元組複製自
// <org>/Hawkr_Transformer_Pricebook（commit
// 1374695cd071cafd112f244abc68101b7ef555f6），細節見
// reference/MARKET_GEOGRAPHY.md。這個套件不重新蒐集或修正任何地理資料，
// 只鏡射、驗證、查詢。
package geography

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// lifecycle_status 的兩個已知值——上游 contract（market_registry.v1.yaml）
// 明文列舉。這是唯一有 allowed 清單的欄位；coordinate_status 上游只宣告
// string 型別、沒有限定枚舉，這裡的驗證嚴格度刻意對齊上游契約，不比它更嚴
// 或更鬆。
const (
	LifecycleStatusActive            = "active"
	LifecycleStatusCatalogOnlyLegacy = "catalog_only_legacy"
)

var knownLifecycleStatuses = map[string]bool{
	LifecycleStatusActive:            true,
	LifecycleStatusCatalogOnlyLegacy: true,
}

// RegistryEntry 是 market_registry.v1.csv 的一列——一個實體市場設施。
// CategoryCodes／MarketTypes 是從 pipe-delimited 欄位拆出來的清單：一個
// 設施可以同時服務多個類別（例：moa-facility-104-produce 服務 N04|N05）。
type RegistryEntry struct {
	FacilityID       string
	LifecycleStatus  string
	MarketID         string
	MarketCode       string
	SourceMarketName string
	FacilityName     string
	CategoryCodes    []string
	MarketTypes      []string
	Address          string
	Longitude        *float64 // nil 僅見於 catalog_only_legacy 列（座標未解析）
	Latitude         *float64
	SurfaceRegion    string
	CoordinateStatus string
}

// IsActive 回傳這一列是否為 lifecycle_status=active——只有 active 列會被
// geography.Load 納入 ResolveFacility 的查詢索引。
func (e RegistryEntry) IsActive() bool {
	return e.LifecycleStatus == LifecycleStatusActive
}

// NeedsCoordinateReview 回傳上游是否標記這筆座標信心較低。上游 contract 對
// coordinate_status 沒有宣告封閉枚舉，只能用字尾判斷——已知真實值涵蓋
// approved_official_point（可信）、conflict_review_required／
// provisional_review_required（待複核）、unresolved_identity（legacy 列，
// 座標本身不存在）。
func (e RegistryEntry) NeedsCoordinateReview() bool {
	return strings.HasSuffix(e.CoordinateStatus, "_review_required")
}

var (
	// ErrRegistryDigestMismatch 代表鏡射檔被改過但 manifest 沒有同步更新——
	// 比照 mapping.LoadSeed 的既有先例，這裡回 error 而不是印警告後繼續：
	// 警告會被忽略，error 會讓服務啟動硬性中止。
	ErrRegistryDigestMismatch           = errors.New("geography: market registry sha256 與 manifest 不符")
	ErrRegistryHeaderMismatch           = errors.New("geography: market registry 表頭不符預期")
	ErrUnknownLifecycleStatus           = errors.New("geography: 未知的 lifecycle_status")
	ErrActiveFacilityMissingCoordinates = errors.New("geography: active 設施缺少座標")
	ErrRegistryNoRows                   = errors.New("geography: market registry 沒有任何資料列")
)

type registryManifest struct {
	SHA256 string `json:"sha256"`
}

var expectedRegistryHeader = []string{
	"registry_version", "facility_id", "lifecycle_status", "market_id",
	"market_code", "source_market_name", "facility_name", "category_codes",
	"market_types", "address", "address_aliases", "longitude", "latitude",
	"surface_region", "coordinate_status", "identity_source",
	"address_source", "coordinate_source", "coordinate_record_id",
	"reviewed_on", "notes",
}

// LoadMarketRegistry 讀取 csvPath 與 manifestPath，驗證兩者一致且每一列
// 合法，回傳全部列（含 catalog_only_legacy）。呼叫端（geography.Load）自行
// 篩選 active 列。
//
// 驗證項目（任一失敗即整份拒收，不做部分載入——地理資料是空間計算的地基，
// 載入一半比完全沒有更危險）：
//  1. csvPath 的 sha256 必須與 manifestPath 記錄的一致（ErrRegistryDigestMismatch）。
//  2. CSV 表頭必須恰為 expectedRegistryHeader（順序、欄位皆同）。
//  3. 每一列的 lifecycle_status 必須是已知值（ErrUnknownLifecycleStatus）。
//  4. lifecycle_status=active 的列必須有 longitude／latitude
//     （ErrActiveFacilityMissingCoordinates）——對齊上游 contract 自己的
//     invariant「active_rows_require_address_wgs84_coordinate...」。
func LoadMarketRegistry(csvPath, manifestPath string) ([]RegistryEntry, error) {
	csvBytes, err := os.ReadFile(csvPath)
	if err != nil {
		return nil, fmt.Errorf("讀取 market registry %s 失敗: %w", csvPath, err)
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("讀取 manifest %s 失敗: %w", manifestPath, err)
	}

	var mf registryManifest
	if err := json.Unmarshal(manifestBytes, &mf); err != nil {
		return nil, fmt.Errorf("解析 manifest %s 失敗: %w", manifestPath, err)
	}

	sum := sha256.Sum256(csvBytes)
	digest := hex.EncodeToString(sum[:])
	if digest != mf.SHA256 {
		return nil, fmt.Errorf("%w：%s 實際 sha256=%s，manifest %s 記錄=%s",
			ErrRegistryDigestMismatch, csvPath, digest, manifestPath, mf.SHA256)
	}

	r := csv.NewReader(bytes.NewReader(csvBytes))
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("讀取 %s 表頭失敗: %w", csvPath, err)
	}
	if !equalHeader(header, expectedRegistryHeader) {
		return nil, fmt.Errorf("%w：得到 %v，預期 %v", ErrRegistryHeaderMismatch, header, expectedRegistryHeader)
	}
	r.FieldsPerRecord = len(expectedRegistryHeader)

	var entries []RegistryEntry
	rowNum := 1
	for {
		rowNum++
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s 第 %d 列格式錯誤: %w", csvPath, rowNum, err)
		}

		entry := RegistryEntry{
			FacilityID:       strings.TrimSpace(record[1]),
			LifecycleStatus:  strings.TrimSpace(record[2]),
			MarketID:         strings.TrimSpace(record[3]),
			MarketCode:       strings.TrimSpace(record[4]),
			SourceMarketName: strings.TrimSpace(record[5]),
			FacilityName:     strings.TrimSpace(record[6]),
			CategoryCodes:    splitPipe(record[7]),
			MarketTypes:      splitPipe(record[8]),
			Address:          strings.TrimSpace(record[9]),
			SurfaceRegion:    strings.TrimSpace(record[13]),
			CoordinateStatus: strings.TrimSpace(record[14]),
		}

		if !knownLifecycleStatuses[entry.LifecycleStatus] {
			return nil, fmt.Errorf("%s 第 %d 列（facility_id=%s）: %w：%q",
				csvPath, rowNum, entry.FacilityID, ErrUnknownLifecycleStatus, entry.LifecycleStatus)
		}

		lon := strings.TrimSpace(record[11])
		lat := strings.TrimSpace(record[12])
		if entry.IsActive() && (lon == "" || lat == "") {
			return nil, fmt.Errorf("%s 第 %d 列（facility_id=%s）: %w",
				csvPath, rowNum, entry.FacilityID, ErrActiveFacilityMissingCoordinates)
		}
		if lon != "" {
			v, err := strconv.ParseFloat(lon, 64)
			if err != nil {
				return nil, fmt.Errorf("%s 第 %d 列（facility_id=%s）longitude 不是合法數字: %w", csvPath, rowNum, entry.FacilityID, err)
			}
			entry.Longitude = &v
		}
		if lat != "" {
			v, err := strconv.ParseFloat(lat, 64)
			if err != nil {
				return nil, fmt.Errorf("%s 第 %d 列（facility_id=%s）latitude 不是合法數字: %w", csvPath, rowNum, entry.FacilityID, err)
			}
			entry.Latitude = &v
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%s: %w", csvPath, ErrRegistryNoRows)
	}

	return entries, nil
}

func splitPipe(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func equalHeader(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if strings.TrimSpace(got[i]) != want[i] {
			return false
		}
	}
	return true
}

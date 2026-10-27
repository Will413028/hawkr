package geography_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
)

// realRegistryPaths 指向本 repo 實際會 commit 的鏡射檔——happy path 測試直接
// 用它，同時驗證「LoadMarketRegistry 的邏輯正確」與「market_registry.v1.csv
// 與 .manifest.json 彼此一致」，比照 mapping 套件既有的 realSeedPaths 慣例。
func realRegistryPaths(t *testing.T) (csvPath, manifestPath string) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "reference"))
	require.NoError(t, err)
	return filepath.Join(dir, "market_registry.v1.csv"), filepath.Join(dir, "market_registry.v1.manifest.json")
}

const registryHeader = "registry_version,facility_id,lifecycle_status,market_id,market_code,source_market_name,facility_name,category_codes,market_types,address,address_aliases,longitude,latitude,surface_region,coordinate_status,identity_source,address_source,coordinate_source,coordinate_record_id,reviewed_on,notes\n"

// writeRegistryFixture 把 csvContent 寫進暫存目錄，並產生與其內容相符（或依
// wrongDigest 蓄意錯誤）的 manifest。
func writeRegistryFixture(t *testing.T, csvContent string, wrongDigest string) (csvPath, manifestPath string) {
	t.Helper()
	dir := t.TempDir()
	csvPath = filepath.Join(dir, "registry.csv")
	manifestPath = filepath.Join(dir, "registry.manifest.json")

	require.NoError(t, os.WriteFile(csvPath, []byte(csvContent), 0o644))

	digest := wrongDigest
	if digest == "" {
		sum := sha256.Sum256([]byte(csvContent))
		digest = hex.EncodeToString(sum[:])
	}
	manifestBytes, err := json.Marshal(map[string]any{
		"schema":    "hawkr.market-registry-mirror/v1",
		"file":      "registry.csv",
		"sha256":    digest,
		"row_count": 1,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, manifestBytes, 0o644))
	return csvPath, manifestPath
}

func TestLoadMarketRegistry_RealFile_LoadsAll25Rows(t *testing.T) {
	csvPath, manifestPath := realRegistryPaths(t)

	entries, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.NoError(t, err)
	require.Len(t, entries, 25, "reference/market_registry.v1.csv 目前應有 25 列；若此斷言失敗，代表 CSV 列數變了但測試沒有同步更新")
}

func TestLoadMarketRegistry_RealFile_Market400HasTwoDisjointFacilities(t *testing.T) {
	csvPath, manifestPath := realRegistryPaths(t)
	entries, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.NoError(t, err)

	var produce, flower *geography.RegistryEntry
	for i := range entries {
		e := entries[i]
		if e.MarketCode != "400" {
			continue
		}
		if e.FacilityID == "moa-facility-400-produce" {
			produce = &entries[i]
		}
		if e.FacilityID == "moa-facility-400-flower" {
			flower = &entries[i]
		}
	}
	require.NotNil(t, produce, "moa-facility-400-produce must exist")
	require.NotNil(t, flower, "moa-facility-400-flower must exist")

	require.NotNil(t, produce.Longitude)
	require.NotNil(t, produce.Latitude)
	require.InDelta(t, 120.658677, *produce.Longitude, 1e-6)
	require.InDelta(t, 24.199409, *produce.Latitude, 1e-6)
	require.ElementsMatch(t, []string{"N04", "N05"}, produce.CategoryCodes)

	require.NotNil(t, flower.Longitude)
	require.NotNil(t, flower.Latitude)
	require.InDelta(t, 120.633526002155, *flower.Longitude, 1e-6)
	require.InDelta(t, 24.1476822435762, *flower.Latitude, 1e-6)
	require.ElementsMatch(t, []string{"N06"}, flower.CategoryCodes)

	require.NotEqual(t, *produce.Longitude, *flower.Longitude, "market_code 400's two facilities must be genuinely different coordinates")
}

func TestLoadMarketRegistry_RealFile_LegacyRowHasNoCoordinates(t *testing.T) {
	csvPath, manifestPath := realRegistryPaths(t)
	entries, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.NoError(t, err)

	var legacy *geography.RegistryEntry
	for i := range entries {
		if entries[i].FacilityID == "moa-facility-100-legacy-unresolved" {
			legacy = &entries[i]
		}
	}
	require.NotNil(t, legacy)
	require.False(t, legacy.IsActive())
	require.Nil(t, legacy.Longitude)
	require.Nil(t, legacy.Latitude)
}

func TestLoadMarketRegistry_DigestMismatch_ReturnsError(t *testing.T) {
	csvPath, manifestPath := writeRegistryFixture(t, registryHeader, "0000000000000000000000000000000000000000000000000000000000000000")

	_, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrRegistryDigestMismatch)
}

func TestLoadMarketRegistry_HeaderMismatch_ReturnsError(t *testing.T) {
	csvPath, manifestPath := writeRegistryFixture(t, "wrong,header\n", "")

	_, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrRegistryHeaderMismatch)
}

func TestLoadMarketRegistry_UnknownLifecycleStatus_ReturnsError(t *testing.T) {
	row := "v1,fac-1,bogus_status,moa:1,1,name,facility,N04,V,addr,,120.0,24.0,taiwan_main_island,approved_official_point,src,src,src,,2026-08-08,\n"
	csvPath, manifestPath := writeRegistryFixture(t, registryHeader+row, "")

	_, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrUnknownLifecycleStatus)
}

func TestLoadMarketRegistry_ActiveRowMissingCoordinates_ReturnsError(t *testing.T) {
	row := "v1,fac-1,active,moa:1,1,name,facility,N04,V,addr,,,,taiwan_main_island,approved_official_point,src,src,src,,2026-08-08,\n"
	csvPath, manifestPath := writeRegistryFixture(t, registryHeader+row, "")

	_, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrActiveFacilityMissingCoordinates)
}

func TestLoadMarketRegistry_NoRows_ReturnsError(t *testing.T) {
	csvPath, manifestPath := writeRegistryFixture(t, registryHeader, "")

	_, err := geography.LoadMarketRegistry(csvPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrRegistryNoRows)
}

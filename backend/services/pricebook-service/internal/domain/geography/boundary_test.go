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

func realBoundaryPaths(t *testing.T) (geojsonPath, manifestPath string) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "reference"))
	require.NoError(t, err)
	return filepath.Join(dir, "taiwan_boundary.taiwan-atlas-2021.9.20.geojson"),
		filepath.Join(dir, "taiwan_boundary.v1.manifest.json")
}

func writeBoundaryFixture(t *testing.T, content string, wrongDigest string) (geojsonPath, manifestPath string) {
	t.Helper()
	dir := t.TempDir()
	geojsonPath = filepath.Join(dir, "boundary.geojson")
	manifestPath = filepath.Join(dir, "boundary.manifest.json")

	require.NoError(t, os.WriteFile(geojsonPath, []byte(content), 0o644))

	digest := wrongDigest
	if digest == "" {
		sum := sha256.Sum256([]byte(content))
		digest = hex.EncodeToString(sum[:])
	}
	manifestBytes, err := json.Marshal(map[string]any{
		"schema":        "hawkr.taiwan-boundary-mirror/v1",
		"file":          "boundary.geojson",
		"sha256":        digest,
		"feature_count": 1,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, manifestBytes, 0o644))
	return geojsonPath, manifestPath
}

func TestLoadBoundary_RealFile_Loads22Counties(t *testing.T) {
	geojsonPath, manifestPath := realBoundaryPaths(t)

	b, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.NoError(t, err)
	require.Len(t, b.Regions, 22, "reference/taiwan_boundary.taiwan-atlas-2021.9.20.geojson 目前應有 22 個 feature；若此斷言失敗，代表檔案變了但測試沒有同步更新")
}

// Digest 是 lineage 用的（issue #2248 的 boundaryDigest）：必須就是與
// manifest 核對過的那個 sha256，不是版本字串、也不是另外算的雜湊。
func TestLoadBoundary_RealFile_ExposesManifestDigest(t *testing.T) {
	geojsonPath, manifestPath := realBoundaryPaths(t)

	b, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.NoError(t, err)

	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest struct {
		SHA256 string `json:"sha256"`
	}
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	require.NotEmpty(t, manifest.SHA256)
	require.Equal(t, manifest.SHA256, b.Digest)
	require.NotEmpty(t, b.Version)
}

func TestLoadBoundary_RealFile_TaipeiCityHallResolvesToTaipeiCounty(t *testing.T) {
	geojsonPath, manifestPath := realBoundaryPaths(t)
	b, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.NoError(t, err)

	// moa-facility-104-produce（臺北市中山區民族東路1號）的真實座標，來自
	// market_registry.v1.csv——用已經驗證過的真實地址座標當測試點，而不是
	// 憑印象湊一個「台北市政府座標」，避免把未經查證的座標寫進測試。
	region, ok := b.CountyContaining(121.537226, 25.068013)
	require.True(t, ok, "臺北市中山區民族東路1號 should resolve to a county")
	require.Equal(t, "63000", region.CountyCode)
	require.Equal(t, "Taipei City", region.CountyNameEN)
}

func TestLoadBoundary_RealFile_OceanPointResolvesToNoCounty(t *testing.T) {
	geojsonPath, manifestPath := realBoundaryPaths(t)
	b, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.NoError(t, err)

	// 台灣海峽中線附近，不應落在任何縣市界線內。
	_, ok := b.CountyContaining(119.5, 24.0)
	require.False(t, ok)
}

func TestLoadBoundary_DigestMismatch_ReturnsError(t *testing.T) {
	fc := `{"type":"FeatureCollection","boundary_version":"test","features":[]}`
	geojsonPath, manifestPath := writeBoundaryFixture(t, fc, "0000000000000000000000000000000000000000000000000000000000000000")

	_, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrBoundaryDigestMismatch)
}

func TestLoadBoundary_NoFeatures_ReturnsError(t *testing.T) {
	fc := `{"type":"FeatureCollection","boundary_version":"test","features":[]}`
	geojsonPath, manifestPath := writeBoundaryFixture(t, fc, "")

	_, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrBoundaryNoFeatures)
}

func TestLoadBoundary_UnclosedRing_ReturnsError(t *testing.T) {
	fc := `{"type":"FeatureCollection","boundary_version":"test","features":[
		{"type":"Feature","properties":{"county_code":"99999","county_name":"test","county_name_en":"Test","surface_region":"taiwan_main_island","interpolation_epsg":3826},
		 "geometry":{"type":"Polygon","coordinates":[[[120.0,24.0],[120.1,24.0],[120.1,24.1],[120.0,24.05]]]}}
	]}`
	geojsonPath, manifestPath := writeBoundaryFixture(t, fc, "")

	_, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.ErrorIs(t, err, geography.ErrBoundaryUnclosedRing)
}

func TestLoadBoundary_NonFiniteCoordinate_ReturnsError(t *testing.T) {
	// brief 原案用 [null,null]，但 Go 標準庫 encoding/json 對 [2]float64 這種固定陣列，
	// unmarshal JSON null 是 no-op（保留零值 0.0，err==nil）——這是 encoding/json 對
	// null 的通用行為（"unmarshaling a JSON null into any other Go type has no effect
	// on the value and produces no error"），不是 orb 這個套件特有的。用
	// `encoding/json` 直接對 `[2]float64` unmarshal `[null,null]` 已驗證得到
	// err=nil、value=[0,0]，跟本檔一開始的假設矛盾。改用字串座標值：對 float64 型別
	// unmarshal 字串會是型別不符，真的會在 JSON decode 階段報錯（已驗證得到
	// "json: cannot unmarshal string into Go value of type float64"），符合這個測試
	// 原本要驗證的語意（畸形座標必須在載入時被拒絕，不能悄悄變成合法座標）。
	fc := `{"type":"FeatureCollection","boundary_version":"test","features":[
		{"type":"Feature","properties":{"county_code":"99999","county_name":"test","county_name_en":"Test","surface_region":"taiwan_main_island","interpolation_epsg":3826},
		 "geometry":{"type":"Polygon","coordinates":[[[120.0,24.0],[120.1,24.0],[120.1,24.1],["invalid","invalid"],[120.0,24.0]]]}}
	]}`
	geojsonPath, manifestPath := writeBoundaryFixture(t, fc, "")

	_, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.Error(t, err)
}

func TestLoadBoundary_RealFile_BBoxWGS84CoversAllRegions(t *testing.T) {
	geojsonPath, manifestPath := realBoundaryPaths(t)
	b, err := geography.LoadBoundary(geojsonPath, manifestPath)
	require.NoError(t, err)

	bbox := b.BBoxWGS84()
	// Taiwan (incl. Kinmen/Matsu/Penghu) roughly spans lon 118-123, lat 21.8-26.4.
	// Loose bounds — this pins "the union covers every region", not exact values
	// that would break on the next boundary-file update.
	require.Less(t, bbox[0], 120.0, "minLon should reach west past Kinmen (~118.2)")
	require.Less(t, bbox[1], 23.0, "minLat should reach south past the southern tip (~21.9)")
	require.Greater(t, bbox[2], 121.5, "maxLon should reach east past the main island (~122.0)")
	require.Greater(t, bbox[3], 25.0, "maxLat should reach north past Matsu (~26.2)")

	// Union of every region's own bound must fully contain each region's bound.
	for _, r := range b.Regions {
		rb := r.Geometry.Bound()
		require.LessOrEqual(t, bbox[0], rb.Min[0], "county %s", r.CountyCode)
		require.LessOrEqual(t, bbox[1], rb.Min[1], "county %s", r.CountyCode)
		require.GreaterOrEqual(t, bbox[2], rb.Max[0], "county %s", r.CountyCode)
		require.GreaterOrEqual(t, bbox[3], rb.Max[1], "county %s", r.CountyCode)
	}
}

func TestBoundary_BBoxWGS84_EmptyRegions_ReturnsZeroValue(t *testing.T) {
	var b geography.Boundary
	require.Equal(t, [4]float64{}, b.BBoxWGS84())
}

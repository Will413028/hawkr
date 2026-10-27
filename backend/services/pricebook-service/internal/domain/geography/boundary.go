// boundary.go 載入並查詢台灣縣市行政界線（issue #2248「行政區聚合」的地理
// 底圖）。鏡射檔案本身即真相；驗證涵蓋 sha256 與這個套件用得到的幾何不變量
// （geometry 型別、ring 是否閉合、座標是否為有限數值）——這些同時是 #2248
// 之後渲染 boundary 欄位時必須滿足的不變量，這裡先做掉，下一個子專案不用
// 重新驗證一次。
package geography

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

// CountyRegion 是行政界線 GeoJSON 的一個 feature——一個縣市。欄位名直接對應
// 上游 GeoJSON 的 properties（county_code／county_name／county_name_en／
// surface_region／interpolation_epsg）——issue #2248 範例裡的 "TPE" 這種
// 縮寫在上游資料中不存在，wire-level regionCode 要用哪個欄位是下一個子專案
// 的決定（設計書 Architecture 段落已記錄這個 naming note）。
type CountyRegion struct {
	CountyCode        string
	CountyName        string
	CountyNameEN      string
	SurfaceRegion     string
	InterpolationEPSG int
	Geometry          orb.Geometry // orb.Polygon 或 orb.MultiPolygon
}

// Contains 回傳 (lon, lat) 是否落在這個縣市的界線內。
func (c CountyRegion) Contains(lon, lat float64) bool {
	point := orb.Point{lon, lat}
	switch g := c.Geometry.(type) {
	case orb.Polygon:
		return planar.PolygonContains(g, point)
	case orb.MultiPolygon:
		return planar.MultiPolygonContains(g, point)
	default:
		return false
	}
}

type boundaryProperties struct {
	CountyCode        string `json:"county_code"`
	CountyName        string `json:"county_name"`
	CountyNameEN      string `json:"county_name_en"`
	SurfaceRegion     string `json:"surface_region"`
	InterpolationEPSG int    `json:"interpolation_epsg"`
}

type boundaryEnvelope struct {
	BoundaryVersion string `json:"boundary_version"`
}

type boundaryManifest struct {
	SHA256 string `json:"sha256"`
}

var (
	ErrBoundaryDigestMismatch      = errors.New("geography: boundary geojson sha256 與 manifest 不符")
	ErrBoundaryInvalidGeometry     = errors.New("geography: boundary geometry 型別不是 Polygon/MultiPolygon")
	ErrBoundaryUnclosedRing        = errors.New("geography: boundary ring 未閉合")
	ErrBoundaryNonFiniteCoordinate = errors.New("geography: boundary 座標非有限數值")
	ErrBoundaryNoFeatures          = errors.New("geography: boundary geojson 沒有任何 feature")
)

// Boundary 是載入後的行政界線集合。Digest 是鏡射檔本身的 sha256（與
// manifest 核對過的那一份），保留下來供下游 lineage 回報「這張圖是哪一版
// 界線畫出來的」——issue #2248 的 spatial-surface 回應要求 boundaryDigest，
// 沒有它就只能回空字串或另外再讀一次檔案算一次。
type Boundary struct {
	Version string
	Digest  string
	Regions []CountyRegion
}

// LoadBoundary 讀取 geojsonPath 與 manifestPath，驗證兩者一致且每個 feature
// 的幾何合法，回傳可查詢的 Boundary。
//
// 驗證項目（任一失敗即整份拒收）：
//  1. geojsonPath 的 sha256 必須與 manifestPath 記錄的一致（ErrBoundaryDigestMismatch）。
//  2. 每個 feature 的 geometry 必須是 Polygon 或 MultiPolygon（ErrBoundaryInvalidGeometry）。
//  3. 每個 ring 必須首尾閉合、至少 4 個點（ErrBoundaryUnclosedRing）。
//  4. 每個座標必須是有限數值，不得為 NaN/Inf（ErrBoundaryNonFiniteCoordinate）。
func LoadBoundary(geojsonPath, manifestPath string) (Boundary, error) {
	geoBytes, err := os.ReadFile(geojsonPath)
	if err != nil {
		return Boundary{}, fmt.Errorf("讀取 boundary %s 失敗: %w", geojsonPath, err)
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return Boundary{}, fmt.Errorf("讀取 manifest %s 失敗: %w", manifestPath, err)
	}

	var mf boundaryManifest
	if err := json.Unmarshal(manifestBytes, &mf); err != nil {
		return Boundary{}, fmt.Errorf("解析 manifest %s 失敗: %w", manifestPath, err)
	}

	sum := sha256.Sum256(geoBytes)
	digest := hex.EncodeToString(sum[:])
	if digest != mf.SHA256 {
		return Boundary{}, fmt.Errorf("%w：%s 實際 sha256=%s，manifest %s 記錄=%s",
			ErrBoundaryDigestMismatch, geojsonPath, digest, manifestPath, mf.SHA256)
	}

	var envelope boundaryEnvelope
	if err := json.Unmarshal(geoBytes, &envelope); err != nil {
		return Boundary{}, fmt.Errorf("解析 %s 的 boundary_version 失敗: %w", geojsonPath, err)
	}

	fc := geojson.FeatureCollectionOf[boundaryProperties]{}
	if err := json.Unmarshal(geoBytes, &fc); err != nil {
		return Boundary{}, fmt.Errorf("解析 %s 的 GeoJSON features 失敗: %w", geojsonPath, err)
	}
	if len(fc.Features) == 0 {
		return Boundary{}, fmt.Errorf("%s: %w", geojsonPath, ErrBoundaryNoFeatures)
	}

	regions := make([]CountyRegion, 0, len(fc.Features))
	for i, feat := range fc.Features {
		if err := validateGeometry(feat.Geometry); err != nil {
			return Boundary{}, fmt.Errorf("%s feature[%d]（county_code=%s）: %w",
				geojsonPath, i, feat.Properties.CountyCode, err)
		}
		regions = append(regions, CountyRegion{
			CountyCode:        feat.Properties.CountyCode,
			CountyName:        feat.Properties.CountyName,
			CountyNameEN:      feat.Properties.CountyNameEN,
			SurfaceRegion:     feat.Properties.SurfaceRegion,
			InterpolationEPSG: feat.Properties.InterpolationEPSG,
			Geometry:          feat.Geometry,
		})
	}

	return Boundary{Version: envelope.BoundaryVersion, Digest: digest, Regions: regions}, nil
}

// CountyContaining 回傳第一個包含 (lon, lat) 的縣市；找不到回
// (CountyRegion{}, false)。
func (b Boundary) CountyContaining(lon, lat float64) (CountyRegion, bool) {
	for _, region := range b.Regions {
		if region.Contains(lon, lat) {
			return region, true
		}
	}
	return CountyRegion{}, false
}

// BBoxWGS84 回傳所有縣市界線聯集的 WGS84 bounding box：
// [minLon, minLat, maxLon, maxLat]。空 Boundary（測試用零值）回傳全零值，
// 不 panic——呼叫端（usecase.finish）不需要為這個邊界情況另外判斷。
func (b Boundary) BBoxWGS84() [4]float64 {
	if len(b.Regions) == 0 {
		return [4]float64{}
	}
	bound := b.Regions[0].Geometry.Bound()
	for _, r := range b.Regions[1:] {
		bound = bound.Union(r.Geometry.Bound())
	}
	return [4]float64{bound.Min[0], bound.Min[1], bound.Max[0], bound.Max[1]}
}

func validateGeometry(geom orb.Geometry) error {
	switch g := geom.(type) {
	case orb.Polygon:
		return validatePolygon(g)
	case orb.MultiPolygon:
		for _, p := range g {
			if err := validatePolygon(p); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("%w：得到 %T", ErrBoundaryInvalidGeometry, geom)
	}
}

func validatePolygon(p orb.Polygon) error {
	for _, ring := range p {
		if len(ring) < 4 {
			return fmt.Errorf("%w：ring 只有 %d 個點", ErrBoundaryUnclosedRing, len(ring))
		}
		first, last := ring[0], ring[len(ring)-1]
		if first[0] != last[0] || first[1] != last[1] {
			return fmt.Errorf("%w：首點 %v 與尾點 %v 不同", ErrBoundaryUnclosedRing, first, last)
		}
		for _, pt := range ring {
			if math.IsNaN(pt[0]) || math.IsNaN(pt[1]) || math.IsInf(pt[0], 0) || math.IsInf(pt[1], 0) {
				return fmt.Errorf("%w：%v", ErrBoundaryNonFiniteCoordinate, pt)
			}
		}
	}
	return nil
}

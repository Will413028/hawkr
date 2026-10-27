// geography.go 是本套件的組裝入口：載入 market registry 與 boundary 兩份
// 鏡射資料，對每個 active 設施做一次性 point-in-polygon 賦值，組出可供
// ResolveFacility／RegionsInScope 查詢的 Registry。
package geography

import (
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
)

// Facility 是一個已解析縣市的 active 設施——RegistryEntry 加上
// point-in-polygon 賦值出的縣市欄位。CountyCode 空字串代表這個設施的座標
// 沒有落在目前鏡射的任何縣市界線內（不會讓服務啟動失敗，由呼叫端決定要不要
// 排除——見套件文件 Error Handling）。
type Facility struct {
	RegistryEntry
	CountyCode   string
	CountyName   string
	CountyNameEN string
}

type facilityKey struct {
	MarketCode   string
	CategoryCode string
}

// ErrDuplicateFacilityKey 代表兩個 active 列宣稱同一個 (market_code,
// category_code) 組合——上游 contract 的 invariant
// 「one_active_facility_per_market_id_and_category_code」保證這不該發生；
// 這裡是防禦性檢查，不是預期路徑。
var ErrDuplicateFacilityKey = errors.New("geography: 同一個 (market_code, category_code) 有超過一個 active 設施")

// Registry 是組裝完成的地理登錄表，服務生命週期內常駐記憶體，查詢不觸發
// I/O。
type Registry struct {
	boundary   Boundary
	facilities []Facility
	index      map[facilityKey]Facility
}

// Load 載入 registry CSV + boundary GeoJSON（各自搭配自己的 manifest），組出
// Registry。任一來源載入失敗、或發現重複的 (market_code, category_code)
// 鍵，整體失敗——地理資料是空間計算的地基，載入一半比完全沒有更危險。
func Load(registryCSV, registryManifest, boundaryGeoJSON, boundaryManifest string) (Registry, error) {
	entries, err := LoadMarketRegistry(registryCSV, registryManifest)
	if err != nil {
		return Registry{}, fmt.Errorf("載入 market registry 失敗: %w", err)
	}

	boundary, err := LoadBoundary(boundaryGeoJSON, boundaryManifest)
	if err != nil {
		return Registry{}, fmt.Errorf("載入 boundary 失敗: %w", err)
	}

	var facilities []Facility
	index := make(map[facilityKey]Facility)

	for _, e := range entries {
		if !e.IsActive() {
			continue
		}

		f := Facility{RegistryEntry: e}
		if e.Longitude != nil && e.Latitude != nil {
			if county, ok := boundary.CountyContaining(*e.Longitude, *e.Latitude); ok {
				f.CountyCode = county.CountyCode
				f.CountyName = county.CountyName
				f.CountyNameEN = county.CountyNameEN
			}
		}

		for _, cat := range e.CategoryCodes {
			key := facilityKey{MarketCode: e.MarketCode, CategoryCode: cat}
			if existing, dup := index[key]; dup {
				return Registry{}, fmt.Errorf("%w：market_code=%s category_code=%s（facility_id=%s 與 facility_id=%s）",
					ErrDuplicateFacilityKey, e.MarketCode, cat, existing.FacilityID, e.FacilityID)
			}
			index[key] = f
		}

		facilities = append(facilities, f)
	}

	return Registry{boundary: boundary, facilities: facilities, index: index}, nil
}

// ResolveFacility 回傳 (market_code, category_code) 在 on 這一天對應的
// active 實體設施。valid_from/valid_to 目前的鏡射快照全部是 nil（永遠
// 有效），on 參數先接起來，等未來快照真的帶有效期間時不需要改介面。
func (r Registry) ResolveFacility(marketCode, categoryCode string, on forecast.Date) (Facility, bool) {
	f, ok := r.index[facilityKey{MarketCode: marketCode, CategoryCode: categoryCode}]
	return f, ok
}

// RegionsInScope 回傳目前鏡射範圍內的所有縣市（含其界線幾何）。行政區價格
// area-weighted rollup 的實作是下一個子專案的範圍，這裡只交付資料介面。
func (r Registry) RegionsInScope() []CountyRegion {
	return r.boundary.Regions
}

// BoundaryVersion / BoundaryDigest 回傳目前鏡射界線的版本字串與檔案 sha256。
// 消費端是 issue #2248 spatial-surface 回應的 lineage 區塊：那份回應要宣告
// 「這張圖是用哪一版界線畫的」，而界線資料只有這裡有。
func (r Registry) BoundaryVersion() string { return r.boundary.Version }

func (r Registry) BoundaryDigest() string { return r.boundary.Digest }

// BBoxWGS84 回傳目前鏡射界線聯集的 WGS84 bounding box，供 spatial-surface
// 回應的 boundary.bboxWgs84 使用。
func (r Registry) BBoxWGS84() [4]float64 { return r.boundary.BBoxWGS84() }

// ResolvedFacilities 回傳所有已解析（含縣市賦值）的 active 設施，依 registry
// CSV 原始順序——供 cmd/pricebook-geography-check 之類的診斷工具逐一列印。
func (r Registry) ResolvedFacilities() []Facility {
	return r.facilities
}

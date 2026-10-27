package model

import (
	"slices"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// MaxBatchSize 是一次批次查詢最多接受的店家 ID 數量（與 buyer 的批次查詢同一個上限）。
const MaxBatchSize = 100

// BatchIDs 驗證批次大小及每個 ID，空清單是合法的空查詢。
func BatchIDs(ids []int64) error {
	if len(ids) > MaxBatchSize {
		return apperr.InvalidInput("批次最多接受 %d 個 ID", MaxBatchSize)
	}
	return Positive(ids...)
}

// ExposableStore 是可以公開曝光的店家。PublishedAt 是這家店的公開時間：第一次可公開曝光的時間（stores.first_exposed_at，由
// 公告函式記下、永久保留）；此刻可曝光但還沒被記下（對帳最多落後一個週期）時是讀取當下，視為剛公開。給「新店
// 曝光」這類以公開起算的邏輯用；從開店、申請或核准起算的話，審核拖得久、或核准後才補齊 logo 的店會在第一次公開之前就用掉新店期。
type ExposableStore struct {
	StoreID     int64
	PublishedAt time.Time
}

// Requirement 是能力啟用或店家公開曝光的一項條件；CapabilitiesOf 與 MissingExposure 回報還沒滿足的條件。
// 每個條件的判斷互不重疊：關閉、等待刪除、停權各是自己的條件。
type Requirement string

const (
	// RequirementActive 店家沒有關閉。
	RequirementActive Requirement = "STORE_ACTIVE"
	// RequirementNotSuspended 帳號沒有生效中的停權。
	RequirementNotSuspended Requirement = "NOT_SUSPENDED"
	// RequirementKYCApproved KYC 流程已核准（含 PayUni 核准）。
	RequirementKYCApproved Requirement = "KYC_APPROVED"
	// RequirementStoreName 有店名。
	RequirementStoreName Requirement = "STORE_NAME"
	// RequirementLogo 有 logo。
	RequirementLogo Requirement = "LOGO"
	// RequirementTownCode 有鄉鎮市區代碼。
	RequirementTownCode Requirement = "TOWN_CODE"
	// RequirementNotPendingDeletion 店家沒有正在隨帳號刪除。
	RequirementNotPendingDeletion Requirement = "NOT_PENDING_DELETION"
)

// Requirements 回傳全部條件，順序固定（與 proto 的 SellerRequirement 數字一致）：MissingExposure 與 CapabilitiesOf 依這個順序回報。
func Requirements() []Requirement {
	return []Requirement{
		RequirementActive, RequirementNotSuspended, RequirementKYCApproved, RequirementStoreName, RequirementLogo, RequirementTownCode,
		RequirementNotPendingDeletion,
	}
}

// MissingRequirementsMetadataKey 是 SELLER_REQUIREMENT_MISSING 的 ErrorInfo.metadata 放缺的條件的 key。
const MissingRequirementsMetadataKey = "missingRequirements"

// NewRequirementMissing 建立「前置條件沒有滿足」的錯誤：FailedPrecondition，reason 是 SELLER_REQUIREMENT_MISSING，缺的條件
// 放在 metadata（MissingRequirementsMetadataKey：SellerRequirement 的值名，不加前綴，依列舉的數字由小到大以逗號連接）。
func NewRequirementMissing(missing []Requirement) *apperr.Error {
	all := Requirements()
	sorted := slices.Clone(missing)
	slices.SortFunc(sorted, func(a, b Requirement) int { return slices.Index(all, a) - slices.Index(all, b) })
	labels := make([]string, len(sorted))
	for i, r := range sorted {
		labels[i] = string(r)
	}
	return apperr.FailedPrecondition("還沒滿足執行此操作的條件").
		WithReason(apperr.ReasonSellerRequirementMissing).WithMetadata(MissingRequirementsMetadataKey, strings.Join(labels, ","))
}

// MissingExposure 回報店家此刻還沒滿足的公開曝光條件（探索、目錄等對買家公開的清單都以它說明還缺什麼，CapabilitiesOf 的
// CapPublicListing 缺的條件就是它），依 Requirements 的順序，全部滿足時為空。條件的每個述詞都不在 Go：與時間無關的在
// seller.store_requirement_flags（s.Conditions），停權在 seller.effective_measures（s.Sanctions）；可不可以曝光本身由
// seller.pub_exposable_stores(at) 回答（exposure.Classify 讀它），這裡的「全部條件的 AND」與那個函式的 AND 由資料集測試
// （TestExposureRuleAgreesBetweenSQLAndGoOnADataset）鎖住。條件：
//   - 店家沒有關閉、沒有正在隨帳號刪除（生命週期是營運中 ACTIVE）；
//   - 帳號沒有生效中的停權；
//   - KYC 流程已核准（含 PayUni 核准）；
//   - 店名、logo 與鄉鎮市區代碼都有值。
//
// 「至少一個上架商品」不在這裡：它由曝光端檢查。
func MissingExposure(s Store) []Requirement {
	var missing []Requirement
	for _, r := range Requirements() {
		if !met(r, s) {
			missing = append(missing, r)
		}
	}
	return missing
}

// met 把讀出來的條件對應到 r：停權看 Sanctions，其餘看 Conditions；沒有推導過的一律不滿足（fail-closed）。
func met(r Requirement, s Store) bool {
	if r == RequirementNotSuspended {
		return !s.Suspended()
	}
	c := s.Conditions
	if !c.Derived {
		return false
	}
	switch r {
	case RequirementActive:
		return c.Active
	case RequirementNotPendingDeletion:
		return c.NotPendingDeletion
	case RequirementKYCApproved:
		return c.KYCApproved
	case RequirementStoreName:
		return c.StoreName
	case RequirementLogo:
		return c.Logo
	case RequirementTownCode:
		return c.TownCode
	case RequirementNotSuspended:
	}
	return false
}

// StoreExposure 是一個店家（含已關閉的 stint，帶推導的停權）、它此刻是否可以公開曝光與是否有公開的店家卡片，只給 legacy 的
// SellerService 組 SellerMeta。
type StoreExposure struct {
	Store     Store
	Exposable bool
	// HasPublicCard 是 HasPublicCard 規則的結果：曾經公開過或此刻可以公開曝光。
	HasPublicCard bool
}

// HasPublicCard 回報店家有沒有公開的店家卡片（StoreCard 的可見性規則，唯一的定義點）：first_exposed_at 有值（曾經公開過），
// 或此刻可以公開曝光（涵蓋公告與對帳還沒追上的空窗）。
func HasPublicCard(firstExposedAt *time.Time, exposable bool) bool {
	return firstExposedAt != nil || exposable
}

// PublicStore 是可以公開曝光的店家連同公開資料最後更新的時間（UpdateTime，給 sitemap 的 lastmod）：stores.updated_at、
// first_exposed_at、公告值最後一次翻轉的時間（store_exposures.exposable_changed_at）與食品業者登錄最後一次改變公開字號的時間
// （核准、清除或撤銷，StoreFoodChangedAt）中最晚的一個，所以店家第一次公開、停權結束後重新曝光、字號核准、清除或撤銷時 lastmod 都會前進；重發同一個公告值不會。
type PublicStore struct {
	Store
	UpdateTime time.Time
}

// PublicStoreUpdateTime 是 PublicStore.UpdateTime 的唯一算法；沒有值的時間（還沒公開、還沒公告、沒有核准、清除或撤銷過字號）略過。
func PublicStoreUpdateTime(updatedAt time.Time, firstExposedAt, announcedAt, foodChangedAt *time.Time) time.Time {
	latest := updatedAt
	for _, t := range []*time.Time{firstExposedAt, announcedAt, foodChangedAt} {
		if t != nil && t.After(latest) {
			latest = *t
		}
	}
	return latest
}

// PublicStoreCursor 是公開店家清單的 keyset 位置：最後一筆的 store_id。
type PublicStoreCursor struct {
	StoreID int64
}

// Valid 回報位置是不是完整的。
func (c PublicStoreCursor) Valid() bool { return c.StoreID > 0 }

// ListPublicStoresInput 是公開店家清單的輸入：PageSize 預設 100、上限 1000，After 是上一頁最後一筆（nil 是第一頁）。
type ListPublicStoresInput struct {
	PageSize int32
	After    *PublicStoreCursor
}

// PublicStorePage 是公開店家清單的一頁：Items 依 store_id 由小到大，Next 是下一頁的位置（nil 是最後一頁）。
type PublicStorePage struct {
	Items []PublicStore
	Next  *PublicStoreCursor
}

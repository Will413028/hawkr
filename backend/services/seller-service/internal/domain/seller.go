package domain

import (
	"context"
	"math"
	"strings"
	"time"
)

const RatingDecimals = 2

func RoundRating(r float64) float64 {
	scale := math.Pow10(RatingDecimals)
	return math.Round(r*scale) / scale
}

type SellerStatus string

const (
	SellerStatusActive              SellerStatus = "active"
	SellerStatusClosed              SellerStatus = "closed"               // ② reshape: replaces the pre-2026-07 'deleted' vocabulary
	SellerStatusSuspended           SellerStatus = "suspended"            // 32 §13 / 44 §13.2: admin manual suspension (不服勸導)
	SellerStatusSuspendedGovernance SellerStatus = "suspended_governance" // B-169: governance full_suspension
)

const DefaultSellerLogo = "public/defaults/seller_logo.png"
const DefaultSellerCover = "public/defaults/seller_cover.png"

const OverallCertNotStarted = "not_started"

type Seller struct {
	UpdatedAt               time.Time
	CreatedAt               time.Time
	CurrentCertificationID  *int64
	DeletedAt               *time.Time
	CertifiedAt             *time.Time
	PIIClearedAt            *time.Time
	PayuniAccountUnlinkedAt *time.Time
	Prize                   string

	District            string
	AddressDetail       string
	PostalCode          string
	Birthday            string
	Gender              string
	StoreName           string
	Description         string
	Email               string
	City                string
	ContactPhone        string
	Logo                string
	CoverKey            string
	SeoTitle            string
	SeoDescription      string
	WebsiteURL          string
	FoodBusinessRegNo   string
	Rating              float64
	SellerType          SellerType
	Status              SellerStatus
	AccountID           int64
	StoreID             int64
	StoreCode           int64
	Version             int64
	OnboardingCompleted bool
	IsProcessedFood     bool
}

func (m *Seller) IsActive() bool { return m.Status == SellerStatusActive }

func CanAcceptOrders(payuniStatus string, sellerStatus SellerStatus) bool {
	return payuniStatus == PayuniStatusActive.String() && sellerStatus == SellerStatusActive
}

// CanManageProducts 回報店家此刻能不能管理商品、把商品轉成 ACTIVE：只看店家的營運狀態（status 本身含停權），不要求 PayUni 啟用。
// 與 CanAcceptOrders 的差別就是 PayUni：賣家可以在 PayUni 核准之前先準備商品。
func CanManageProducts(sellerStatus SellerStatus) bool {
	return sellerStatus == SellerStatusActive
}

func MeetsCertificationPrecondition(hasPublishedPost bool, description, websiteURL string) bool {
	if strings.TrimSpace(description) != "" && strings.TrimSpace(websiteURL) != "" {
		return true
	}
	return hasPublishedPost
}

func (m *Seller) IsClosed() bool { return m.Status == SellerStatusClosed }

func (m *Seller) MarkClosed(now time.Time) {
	m.Status = SellerStatusClosed
	m.DeletedAt = &now
	m.PayuniAccountUnlinkedAt = &now
	m.UpdatedAt = now
}

func (m *Seller) UpdateProfile(storeName, description, logo, coverKey, contactPhone, city, district, addressDetail, postalCode, birthday, gender, prize, websiteURL string, now time.Time) {
	m.StoreName = storeName
	m.Description = description
	m.Logo = logo
	m.CoverKey = coverKey
	m.ContactPhone = contactPhone
	m.City = city
	m.District = district
	m.AddressDetail = addressDetail
	m.PostalCode = postalCode
	m.Birthday = birthday
	m.Gender = gender
	m.Prize = prize
	m.WebsiteURL = websiteURL
	m.UpdatedAt = now
}

func (m *Seller) FullAddress() string {
	return m.City + m.District + m.AddressDetail
}

type AdminSellerRow struct {
	Seller
	CertStatus         string
	PayuniMerStatus    string
	PayuniMemberStatus string
}

type SellerProfileRow struct {
	Seller                   *Seller
	CertStatus               string
	PayuniApprovedAt         *time.Time
	CertSellerType           string
	PayuniMerStatus          string
	PayuniRegistrationStatus string
	PayuniMemberStatus       string
}

type SellerListFilter struct {
	CertStatuses    []string
	Keyword         *string
	LifecycleStatus string
	Limit           int32
	Offset          int32
}

type SellerStats struct {
	Total      int64
	Pending    int64
	Approved   int64
	Rejected   int64
	MonthlyNew int64
}

type SellerMetaRow struct {
	StoreName    string
	Logo         string
	CertStatus   string
	PayuniStatus string
	Status       string
	ZipCode      string
	City         string
	District     string
	AccountID    int64
	StoreID      int64
	StoreCode    int64
}

type PublicCityStores struct {
	CityCode        string
	CityName        string
	StoreIDs        []int64
	PreviewStoreIDs []int64
}

type PublicProductStores struct {
	known bool
	ids   map[int64]struct{}
	order []int64
}

func NewPublicProductStores(storeIDs []int64) PublicProductStores {
	s := PublicProductStores{
		known: true,
		ids:   make(map[int64]struct{}, len(storeIDs)),
		order: make([]int64, 0, len(storeIDs)),
	}
	for _, id := range storeIDs {
		if _, dup := s.ids[id]; dup {
			continue
		}
		s.ids[id] = struct{}{}
		s.order = append(s.order, id)
	}
	return s
}

func UnknownPublicProductStores() PublicProductStores { return PublicProductStores{} }

func (s PublicProductStores) Known() bool { return s.known }

func (s PublicProductStores) Has(storeID int64) bool {
	if !s.known {
		return true
	}
	_, ok := s.ids[storeID]
	return ok
}

func (s PublicProductStores) IDs() []int64 {
	if s.order == nil {
		return []int64{}
	}
	return s.order
}

type SellerRepository interface {
	GetByAccountID(ctx context.Context, accountID int64) (*Seller, error)
	GetCurrentByStoreID(ctx context.Context, storeID int64) (*Seller, error)
	GetProfileByAccountID(ctx context.Context, accountID int64) (*SellerProfileRow, error)
	GetPublicProfileByStoreIDOrCode(ctx context.Context, storeIDOrCode int64) (*SellerProfileRow, error)
	GetAccountIDByStoreID(ctx context.Context, storeID int64) (int64, error)
	GetProfileByStoreID(ctx context.Context, storeID int64) (*SellerProfileRow, error)
	GetByStoreIDs(ctx context.Context, storeIDs []int64) ([]*Seller, error)
	GetApprovedExcluding(ctx context.Context, excludeStoreIDs []int64, withPublicProducts PublicProductStores, limit int32) ([]*Seller, error)
	GetSellerMetaByStoreIDs(ctx context.Context, storeIDs []int64) ([]*SellerMetaRow, error)
	GetPublicStoreIDsByCity(ctx context.Context, cityAliases []string) ([]int64, error)
	GetPublicStoreIDsGroupedByCity(ctx context.Context) (map[string][]int64, error)
	GetPrincipalsByStoreIDs(ctx context.Context, ids []int64) (map[int64]int64, []int64, error)
	GetStoreIDsByAccountID(ctx context.Context, accountID int64) ([]int64, error)
	Create(ctx context.Context, merchant *Seller) error
	Update(ctx context.Context, merchant *Seller, expectedVersion *int64) error
	UpdateCurrentCertificationID(ctx context.Context, accountID int64, certID int64) error
	UpdateLogo(ctx context.Context, storeID int64, logo string, expectedVersion *int64) error
	UpdateCover(ctx context.Context, storeID int64, coverKey string, expectedVersion *int64) error
	UpdateSEO(ctx context.Context, accountID int64, seoTitle, seoDescription string) error
	UpdateFoodBusinessRegistration(ctx context.Context, accountID int64, isProcessedFood bool, regNo string, updatedAt time.Time) error
	CompleteOnboarding(ctx context.Context, accountID int64) error
	ListForAdmin(ctx context.Context, filter SellerListFilter) ([]*AdminSellerRow, error)
	CountForAdmin(ctx context.Context, filter SellerListFilter) (int64, error)
	GetStatsForAdmin(ctx context.Context) (*SellerStats, error)
	UpdateRatingByStoreID(ctx context.Context, storeID int64, rating float64) error
	GetByAccountIDForUpdate(ctx context.Context, accountID int64) (*Seller, error)
	RetireCurrentStint(ctx context.Context, accountID int64, now time.Time) error
	UpdateStatusByStoreID(ctx context.Context, storeID int64, status SellerStatus, unlinkedAt *time.Time, deletedAt *time.Time, now time.Time) error
	UpdateStatusCurrentByAccountID(ctx context.Context, accountID int64, status SellerStatus, now time.Time) error
	ClearPIIFields(ctx context.Context, storeID int64, clearedAt time.Time) error
	ListPendingPIICleanup(ctx context.Context, before time.Time, limit int) ([]int64, error)
}

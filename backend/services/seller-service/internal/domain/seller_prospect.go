package domain

import (
	"context"
	"errors"
	"time"
)

var ErrProspectMergeNotAllowed = errors.New("prospect merge is not allowed")

// ErrProspectMediaConflict 是換圖被述詞擋下的統稱：版本不符、列不是 ACTIVE、
// 或已被合併。刻意不細分——三者對 Admin 的處置相同（重新讀取再換一次），
// 而細分會把「這一列的狀態」洩漏給只該知道成敗的呼叫端。
var ErrProspectMediaConflict = errors.New("prospect media update conflicts with current state")

type SellerProspect struct {
	ID               int64
	DisplayName      string
	VendorType       string
	Status           string
	OutreachStatus   string
	TaxID            *string
	ConvertedStoreID *int64
	ConvertedAt      *time.Time

	MatchKeys ProspectMatchKeys

	PublicEmail   string
	PublicPhone   string
	PublicWebsite string
	PublicSocial  string

	Category        string
	Region          string
	ProductSummary  string
	Positioning     string
	ResearchSummary string
	PriorityLetter  string
	PriorityStars   int

	CreatedByAccountID *int64
	UpdatedByAccountID *int64
	CreatedAt          time.Time
	UpdatedAt          time.Time

	MergedIntoProspectID *int64

	// 品牌圖片。Key 是 media-service 轉正後的 objectKey，顯示 URL 由 Gateway 組。
	// SourceURL 是素材的來源頁面，只作稽核；後端不會去取那個網址。
	LogoKey        string
	CoverKey       string
	LogoSourceURL  string
	CoverSourceURL string
	MediaVersion   int64
}

type ProspectInvite struct {
	ID         int64
	ProspectID int64
	ExpiresAt  time.Time
	SentAt     *time.Time
	ClickedAt  *time.Time
	ConsumedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

type ProspectSearchHit struct {
	Prospect       SellerProspect
	HitCategory    bool
	HitRegion      bool
	HitKeywords    bool
	HitCategoryIDs bool
}

type ProspectSearchParams struct {
	Category string
	Region   string
	Keywords string

	CategoryIDs []string
	Limit       int
}

type ProspectAdminListParams struct {
	Status         string
	OutreachStatus string
	VendorType     string
	Category       string
	Region         string
	Keyword        string
	Page           int
	PageSize       int
}

type ProspectDuplicatePair struct {
	Left  SellerProspect
	Right SellerProspect
}

type ProspectTaxIDMatch struct {
	Prospect                 SellerProspect
	StoreID                  int64
	StoreName                string
	CertificationCompanyName string
	CertificationStatus      string
}

// ProspectMediaUpdate 是一次換圖的輸入。ObjectKey 已經是 media-service 轉正後的
// 正式 key——這一層不接受外部圖片網址，換圖的 bytes 一律先走 staged-token flow。
type ProspectMediaUpdate struct {
	ProspectID           int64
	Kind                 string // logo | cover
	ObjectKey            string
	SourceURL            string
	ExpectedMediaVersion int64
	OperatorAccountID    *int64
}

type SellerProspectRepository interface {
	Create(ctx context.Context, in SellerProspect) (SellerProspect, error)
	Search(ctx context.Context, params ProspectSearchParams) ([]ProspectSearchHit, error)

	FindByTaxID(ctx context.Context, taxID string) (SellerProspect, error)

	FindByMatchKeys(ctx context.Context, keys ProspectMatchKeys) ([]SellerProspect, error)

	MarkConverted(ctx context.Context, prospectID, storeID int64, operatorAccountID *int64) (int64, error)

	FindUsableInviteByToken(ctx context.Context, token string) (ProspectInvite, error)

	ConsumeInvite(ctx context.Context, inviteID int64) error

	CreateInvite(ctx context.Context, prospectID int64, token string, expiresAt time.Time) (ProspectInvite, error)

	RevokeInvite(ctx context.Context, prospectID, inviteID int64) (int64, error)

	ListForAdmin(ctx context.Context, params ProspectAdminListParams) ([]SellerProspect, int64, error)

	GetByID(ctx context.Context, id int64) (SellerProspect, error)
	ListInvites(ctx context.Context, prospectID int64) ([]ProspectInvite, error)

	Update(ctx context.Context, in SellerProspect) (SellerProspect, error)

	SetArchived(ctx context.Context, id int64, archived bool, operatorID *int64) (int64, error)

	// UpdateMedia 換一張品牌圖片。expectedMediaVersion 是樂觀鎖基準；版本不符、
	// 列不是 ACTIVE、或已被合併，都回 ErrProspectMediaConflict 而不是 not found
	// ——呼叫端要能分辨「這一列不給改」與「這一列不存在」。
	UpdateMedia(ctx context.Context, in ProspectMediaUpdate) (SellerProspect, error)

	ListDuplicatePairs(ctx context.Context) ([]ProspectDuplicatePair, error)

	Merge(ctx context.Context, winnerID, loserID int64, operatorID *int64) (SellerProspect, error)

	CurrentStoreExists(ctx context.Context, storeID int64) (bool, error)

	ListTaxIDMatches(ctx context.Context) ([]ProspectTaxIDMatch, error)
}

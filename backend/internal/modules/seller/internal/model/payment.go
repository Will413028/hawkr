package model

import (
	"encoding/json"
	"time"
)

// PaymentAccountStatus 是收款帳戶的中立狀態（Stripe 計畫 D13），由外部帳戶的快照推導。
type PaymentAccountStatus string

// 收款帳戶的狀態。
const (
	// PaymentNotCreated 本地有列、還沒有外部帳戶。
	PaymentNotCreated PaymentAccountStatus = "NOT_CREATED"
	// PaymentUnderReview 轉帳能力還沒啟用，等金流商處理，賣家沒有要做的事。
	PaymentUnderReview PaymentAccountStatus = "UNDER_REVIEW"
	// PaymentActionRequired 賣家要補資料或完成開通（含新建立的帳戶）。
	PaymentActionRequired PaymentAccountStatus = "ACTION_REQUIRED"
	// PaymentEnabled 轉帳能力已啟用。
	PaymentEnabled PaymentAccountStatus = "ENABLED"
	// PaymentDisabled 被金流商拒絕、不支援或限制，賣家補件解不開。
	PaymentDisabled PaymentAccountStatus = "DISABLED"
)

// PaymentAccountStatusValues 回傳全部狀態，與 payment_accounts_status_check 相同。
func PaymentAccountStatusValues() []PaymentAccountStatus {
	return []PaymentAccountStatus{PaymentNotCreated, PaymentUnderReview, PaymentActionRequired, PaymentEnabled, PaymentDisabled}
}

// RequirementCollection 是誰向賣家收身分與銀行資料（port 的能力描述，也存在帳戶列上）。
type RequirementCollection string

// 收件方式。
const (
	// CollectionProvider 金流商收件（託管頁面為預設）。
	CollectionProvider RequirementCollection = "PROVIDER"
	// CollectionPlatform 平台收件（金流商的內嵌元件）。
	CollectionPlatform RequirementCollection = "PLATFORM"
)

// RequirementCollectionValues 回傳全部收件方式，與 payment_accounts_collection_check 相同。
func RequirementCollectionValues() []RequirementCollection {
	return []RequirementCollection{CollectionProvider, CollectionPlatform}
}

// OnboardingSurface 是開通用的介面。
type OnboardingSurface string

// 開通介面。
const (
	// SurfaceHosted 金流商託管的頁面（連結）。
	SurfaceHosted OnboardingSurface = "HOSTED"
	// SurfaceEmbedded 嵌在自己網頁的元件（client secret）。
	SurfaceEmbedded OnboardingSurface = "EMBEDDED"
)

// PaymentAccount 是店家的收款帳戶：外部帳戶的本地快取，不含個資。
type PaymentAccount struct {
	StoreID               int64
	Provider              string
	ExternalAccountID     *string
	RequirementCollection RequirementCollection
	Status                PaymentAccountStatus
	TransfersActive       bool
	PayoutsActive         bool
	// Requirements 是金流商回報的待補條件原始 JSON（物件）。
	Requirements json.RawMessage
	// StatusReason 是導出目前狀態的金流商原因碼（只給後台）。
	StatusReason *string
	FetchedAt    *time.Time
	// CreateFailedAt 是建立重試用完的時刻；只有還沒有外部帳戶的列才有。
	CreateFailedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// OnboardingSession 是開通用的 session：託管用 URL，內嵌用 ClientSecret。
type OnboardingSession struct {
	Surface      OnboardingSurface
	URL          string
	ClientSecret string
	ExpiresAt    *time.Time
}

// CreatePaymentOnboardingSessionInput 是賣家取得開通 session 的輸入；ReturnURL 與 RefreshURL 由呼叫方（handler）組出。
type CreatePaymentOnboardingSessionInput struct {
	AccountID  int64
	ReturnURL  string
	RefreshURL string
}

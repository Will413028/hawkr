package model

import (
	"encoding/json"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
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

// PaymentAccountStatusValues 回傳全部狀態，與 accounts_status_check 相同。
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

// RequirementCollectionValues 回傳全部收件方式，與 accounts_collection_check 相同。
func RequirementCollectionValues() []RequirementCollection {
	return []RequirementCollection{CollectionProvider, CollectionPlatform}
}

// EmbeddedRuntime 是呼叫端能承載的內嵌開通執行環境（呼叫端宣告，空清單表示不能內嵌）。
type EmbeddedRuntime string

// 內嵌開通執行環境。
const (
	// RuntimeStripeConnectJS 是 Stripe Connect.js（瀏覽器）。
	RuntimeStripeConnectJS EmbeddedRuntime = "STRIPE_CONNECT_JS"
)

// EmbeddedRuntimeValues 回傳全部內嵌開通執行環境。
func EmbeddedRuntimeValues() []EmbeddedRuntime { return []EmbeddedRuntime{RuntimeStripeConnectJS} }

// PaymentAccount 是擁有者（店家）的收款帳戶：外部帳戶的本地快取（聯絡信箱不對外）。
type PaymentAccount struct {
	StoreID  int64
	Provider string
	// ExternalAccountID 第一步拿到就有，不代表已建立完成（看 Status：NOT_CREATED 就是還沒完成，D59）。
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
	// CreateFailedAt 是建立重試用完的時刻；只有還沒完成建立的列才有。
	CreateFailedAt *time.Time
	// PurgeBlockedAt 與 PurgeBlockedReason 是清理進終局的時刻與原因（D59）；沒有進終局時是 nil。
	PurgeBlockedAt     *time.Time
	PurgeBlockedReason *string
	// PurgePendingAt 是清理已要求、還沒完成的時刻（D63：後台顯示「關閉中」）；沒有等待清理時是 nil。
	PurgePendingAt *time.Time
	// CreateUnresolved 表示建立結果不明（呼叫過、沒有外部 ID）的建立失敗列已超過金流商保存冪等鍵的期限（30 天，D63）：同一把鍵不再保證回同一個帳戶，
	// 管理員重試建立會換新鍵並建立新的帳戶，等於聲明金流商那邊沒有這家店的帳戶，需要明確確認。保存期內（含已超過 7 天自動重送期限的）為假：重試以同一把鍵重送，不需要確認。
	CreateUnresolved bool
	// PurgeReplayPending 表示管理員要求的「以同一把鍵重送」排隊中或進行中（K1）：這段期間後端拒絕刪除對應與再次重試關閉，後台隱藏兩個動作。
	PurgeReplayPending bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// OnboardingSession 是開通用的 session，只有兩種形狀：HostedSession（託管連結）與各金流商的內嵌 session（目前只有 StripeEmbeddedSession）。
// 它是封閉的 interface（方法未匯出，只有本套件的型別能實作），每種形狀只帶自己用得到的欄位，所以沒有「型態欄位加攤平欄位」的第三種組合；
// 錯誤時回 nil，不回零值。
type OnboardingSession interface {
	onboardingSession()
}

// HostedSession 是金流商託管的開通頁面：一次性、短效的連結。
type HostedSession struct {
	URL string
}

// StripeEmbeddedSession 是 Stripe Connect.js 內嵌元件的初始化資料：client secret 與公開金鑰（不是 secret）。
type StripeEmbeddedSession struct {
	ClientSecret   string
	PublishableKey string
}

func (HostedSession) onboardingSession()         {}
func (StripeEmbeddedSession) onboardingSession() {}

// PayoutAccountChanged 是同步 job 寫入帳戶快照的 transaction 內發布的事件（Stripe 計畫 D28）：seller 以同 transaction 的訂閱者更新開通閘門讀的
// transfers_active 副本，並把 BankFingerprints 寫成防詐比對 hash（D12）。每次同步都發布（fingerprint 可能在轉帳能力不變時改變）。
// BankFingerprints 是 provider 的外部銀行帳號識別碼明文：發布方與訂閱者都不保存、不寫 log。
type PayoutAccountChanged struct {
	OccurredAt       time.Time
	StoreID          int64
	Provider         string
	TransfersActive  bool
	BankFingerprints []string
}

// Positive 檢查 ID 都是正數。
func Positive(ids ...int64) error {
	for _, id := range ids {
		if id <= 0 {
			return apperr.InvalidInput("ID 必須為正數")
		}
	}
	return nil
}

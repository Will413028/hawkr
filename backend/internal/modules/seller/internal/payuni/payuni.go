// Package payuni 是賣家的 PayUni 子商店註冊（ADR 2026-10-19-seller-kyc-payuni-consolidated「核准與 PayUni 綁定」）：
// 綁定意圖、向 PayUni 註冊與同步的 job、PayUni 審核狀態的觀測與套用，以及 cert 軸與 PayUni 軸合成的流程狀態。
//
// 結構（backend AGENTS.md 規則 4、5）：
//   - 意圖在業務的 transaction 內寫入（核准、賣家改聯絡欄位、店家改地址、重新綁定），同一個 transaction 排入以 store_id
//     為唯一鍵的 sync job；job 在 transaction 外讀目前的資料、呼叫 PayUni，再於自己的 transaction 寫結果與事件。
//   - 沒有任何地方在 transaction 內呼叫 PayUni，也不存請求本體。
//   - 外部進來的狀態（webhook、輪詢、手動刷新、綁定）都先追加成一筆觀測，再經同一個套用函式更新目前的狀態：只有事件時間
//     比目前新的才會寫入。
package payuni

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	pu "github.com/Will413028/hawkr/backend/internal/platform/payuni"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

// Client 是呼叫 PayUni 的 port：真正的 HTTP client 是 platform/payuni，測試用替身。傳輸錯誤（連線、逾時、非 200、回應解
// 不開）回一般錯誤，job 重試；PayUni 的業務失敗回 *pu.APIError（platform/payuni 的 AsAPIError）。實作不得把傳輸錯誤轉成
// 業務結果（盤點 D16）。
type Client interface {
	BindMerchant(ctx context.Context, req pu.BindMerchantRequest) error
	UpdateMerchant(ctx context.Context, req pu.UpdateMerchantRequest) error
	MerchantStatus(ctx context.Context, merID string) (pu.MerchantStatus, error)
}

// BindingKYC 是綁定要送的 KYC 資料（解密後，只存在於 job 執行期間的記憶體）。IDNumber 是這個申請裡的人的身分證號：
// 個人賣家是本人的，企業是負責人的。
type BindingKYC struct {
	SellerType                      model.KYCSellerType
	RealName, EnglishName, IDNumber string
	ProductType, WebsiteURL         string
	CompanyName, TaxID              string
}

// KYCSource 讀綁定要送的 KYC 資料（由 kyc 子領域實作：解密在那裡，payuni 不碰金鑰）。
type KYCSource interface {
	BindingData(ctx context.Context, applicationID uuid.UUID) (BindingKYC, error)
	// CompanyPhone 是申請上的公司電話（企業賣家才有，個人是空字串）：bind 與 update 的 ContactTel 與 MBCTel 都從這一個讀取點來，不複製。
	CompanyPhone(ctx context.Context, applicationID uuid.UUID) (string, error)
}

// Deps 外部依賴；clock 由組裝點注入。Client 為 nil 表示沒有設定 PayUni：bind job 把註冊記為失敗（ErrNotConfigured），
// 不呼叫任何東西，輪詢也不排。StorefrontBaseURL 是店家頁的網站根網址（沒有填官網時，MerURL 是它加 /sellers/<store_code>），
// 空字串表示沒有，這時沒有官網的申請記為資料不完整。
type Deps struct {
	DB                *db.DB
	Events            *events.Bus
	Jobs              jobs.Enqueuer
	Client            Client
	KYC               KYCSource
	Exposure          *exposure.Service
	StorefrontBaseURL string
	Now               func() time.Time
	Logger            *slog.Logger
}

// Service 是 PayUni 註冊的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.KYC == nil {
		panic("payuni: KYC is required")
	}
	return &Service{d: d}
}

// 送給 PayUni 的固定參數（舊 seller-service 的 client 沿用，業務的決定，不是設定）。
const (
	merType      = 2
	merMCC       = 3
	goodsRate    = "100,0,0,0"
	payment      = "CREDIT,ApplePay,GooglePay,ATM"
	deliverTime  = 7
	escrowType   = 1
	merUnitPrice = 1000
	sellType     = 1
)

const (
	// callTimeout 是單次呼叫 PayUni 的上限（client 自己也有 deadline；這是 job 內再多一層，涵蓋 bind 後接著查詢的情形）。
	callTimeout = 20 * time.Second
	// jobTimeout 是一次 sync job 的上限：最多兩次呼叫加資料庫（遠小於 River 的 rescuer 判定 job 卡住的時間，所以 River 的唯一鍵就能保證同一個店家同時只有一個 job）。
	jobTimeout = 60 * time.Second
	// writeTimeout 是寫結果的 transaction 的上限（job 的 ctx 逾時被取消後仍要能寫入）。
	writeTimeout = 15 * time.Second
	// syncMaxAttempts 是 sync job 暫時性失敗時 River 重試的上限（退避是 River 預設的 attempt^4 秒，約 80 分鐘內用完）；
	// 最後一次仍失敗就記為 FAILED（bind）或同步失敗（update），由管理員或賣家再送。
	syncMaxAttempts = 8
	// pollInterval 是同一筆註冊兩次輪詢的最短間隔（舊 seller-service 沿用）。
	pollInterval = 2 * time.Hour
	// resyncIdle 是有資料沒送出、閒置多久之後由掃描補排 job（保底，平常 job 一定存在）。
	resyncIdle = 10 * time.Minute
	// resubmitPollWindow 是 PayUni 拒絕之後重新送出再核准的註冊，輪詢繼續問新結果的期間（之後只剩 webhook）。PayUni 沒有
	// 規定重審多久，沒有對真實的 PayUni 驗證過（CARRYOVER）。
	resubmitPollWindow = 7 * 24 * time.Hour
	// sweepBatch 是掃描一次排入的上限。
	sweepBatch = 100
)

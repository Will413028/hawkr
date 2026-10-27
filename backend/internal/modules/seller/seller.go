// Package seller 是 seller 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package seller

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/block"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/closing"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/devseed"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/foodreg"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/governance"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/kyc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/measure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/onboarding"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/payuni"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/profile"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/prospect"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/purge"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/suspension"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/views"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

//go:embed db/migrations/*.sql db/migrations/atlas.sum
var migrationFiles embed.FS

// Migrations 回傳本模組的 migration，給 cmd/hawkr 登記。
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationFiles, "db/migrations")
	if err != nil {
		panic(err) // 只有 embed 的路徑寫錯才會發生，編譯後的 binary 不可能遇到
	}
	return sub
}

// Orders 是自己關店時確認店家有沒有未結訂單的 port（訂單在 Java 期間由 internal/legacy/orderservice 實作，ordering 搬進
// 單體後改由它的公開 API 實作）。accountdeletion 的刪帳號用同一個實作。查詢失敗時回錯誤，關店一律拒絕（fail-closed）。
type Orders interface {
	HasOpenStoreOrders(ctx context.Context, storeID int64) (bool, error)
}

// Deps 是模組需要的外部依賴；clock 由組裝點注入。StoreCode 是 store_code 的產生器，測試以外不給（預設是
// 年份加密碼學亂數）。KYCKeys 是 KYC 個資的加密金鑰環，KYCMAC 是比對用的 HMAC 金鑰環（兩者必須是不同的金鑰，
// seal.Distinct），都是必填。PayUni 是呼叫 PayUni 的 client（platform/payuni 實作），沒有設定時給 nil：綁定記為失敗
// （PAYUNI_NOT_CONFIGURED），輪詢不排；StorefrontBaseURL 是店家頁的網站根網址（申請沒有填官網時，MerURL 由它組成）。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.ReportingEnqueuer
	Identity *identity.Module
	Media    media.Media
	Orders   Orders
	KYCKeys  *seal.Keyring
	KYCMAC   *seal.MAC
	PayUni   PayUniClient
	// Registry 是登記資料來源（公司型申請送件時查詢並存快照；serve 與 worker 都需要，serve 排入、worker 執行），必填。
	Registry Registry
	// ProductStores 是商品所屬店家的 port，彙總瀏覽事件的 job 用；必填。
	ProductStores ProductStores
	// StorefrontBaseURL 見上；空字串表示沒有。
	StorefrontBaseURL string
	Now               func() time.Time
	StoreCode         func(now time.Time) (int64, error)
	Logger            *slog.Logger
}

// Module 是 seller 模組。
type Module struct {
	onboarding *onboarding.Service
	profile    *profile.Service
	suspension *suspension.Service
	closing    *closing.Service
	mediaSync  *media.RefSyncer
	governance *governance.Service
	measure    *measure.Service
	kyc        *kyc.Service
	foodreg    *foodreg.Service
	payuni     *payuni.Service
	purge      *purge.Service
	prospect   *prospect.Service
	views      *views.Service
	block      *block.Service
	devSeed    *devseed.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil || d.Events == nil || d.Jobs == nil || d.Identity == nil || d.Media == nil || d.Orders == nil || d.KYCKeys == nil || d.KYCMAC == nil || d.ProductStores == nil || d.Registry == nil {
		panic("seller: missing dependency")
	}
	if d.Now == nil {
		panic("seller: Now is required")
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if err := seal.Distinct(d.KYCKeys, d.KYCMAC); err != nil {
		panic("seller: " + err.Error())
	}
	exposures := exposure.New(exposure.Deps{Events: d.Events})
	suspensions := suspension.New(suspension.Deps{DB: d.DB, Events: d.Events, Jobs: d.Jobs, Exposure: exposures, Now: d.Now})
	closings := closing.New(closing.Deps{
		DB: d.DB, Events: d.Events, Jobs: d.Jobs, Identity: d.Identity, Orders: d.Orders, Exposure: exposures, Now: d.Now,
	})
	kycService := kyc.New(kyc.Deps{
		DB: d.DB, Events: d.Events, Jobs: d.Jobs, Media: d.Media, Exposure: exposures, Keys: d.KYCKeys, MAC: d.KYCMAC, Registry: d.Registry, Now: d.Now, Logger: d.Logger,
	})
	return &Module{
		onboarding: onboarding.New(onboarding.Deps{
			DB: d.DB, Events: d.Events, Jobs: d.Jobs, Identity: d.Identity, Now: d.Now, StoreCode: d.StoreCode,
		}),
		profile: profile.New(profile.Deps{
			DB: d.DB, Events: d.Events, Jobs: d.Jobs, Media: d.Media, Exposure: exposures, StorefrontBaseURL: d.StorefrontBaseURL,
			Now: d.Now, Logger: d.Logger,
		}),
		suspension: suspensions,
		closing:    closings,
		mediaSync:  mediaref.NewSyncer(d.DB, d.Media, d.Jobs, d.Now, d.Logger),
		governance: governance.New(governance.Deps{
			DB: d.DB, Events: d.Events, Jobs: d.Jobs, Media: d.Media, Identity: d.Identity, Now: d.Now, Logger: d.Logger,
		}),
		measure: measure.New(measure.Deps{
			DB: d.DB, Events: d.Events, Jobs: d.Jobs, Media: d.Media, Identity: d.Identity, Linked: kycService, Closing: closings, Exposure: exposures, Now: d.Now,
		}),
		kyc:     kycService,
		foodreg: foodreg.New(foodreg.Deps{DB: d.DB, Events: d.Events, Jobs: d.Jobs, Exposure: exposures, Now: d.Now}),
		payuni: payuni.New(payuni.Deps{
			DB: d.DB, Events: d.Events, Jobs: d.Jobs, Client: d.PayUni, KYC: kycService, Exposure: exposures,
			StorefrontBaseURL: d.StorefrontBaseURL, Now: d.Now, Logger: d.Logger,
		}),
		purge: purge.New(purge.Deps{DB: d.DB, Events: d.Events, Jobs: d.Jobs, Now: d.Now}),
		prospect: prospect.New(prospect.Deps{
			DB: d.DB, Events: d.Events, Jobs: d.Jobs, Media: d.Media, Now: d.Now,
		}),
		block:   block.New(block.Deps{DB: d.DB, Now: d.Now}),
		views:   views.New(views.Deps{DB: d.DB, ProductStores: d.ProductStores, Now: d.Now}),
		devSeed: devseed.New(devseed.Deps{DB: d.DB, KYC: kycService, Events: d.Events, Jobs: d.Jobs, Now: d.Now}),
	}
}

// Reader 是只讀店家的 seller：只需要資料庫，不需要事件、job、media、KYC 金鑰或 PayUni。給只讀店家狀態的組裝使用（運維子命令
// admin-role、delegation-eligibility 的 delegation 依賴；characterization 測試的組裝），它們不該為了讀一列店家而備齊 serve 的 seller 設定。
// 讀取的規則與 Module.Store 是同一份（同一支 profile.Service.Store）。
type Reader struct{ profile *profile.Service }

// NewReader 建立唯讀的 seller；db、now 必填。
func NewReader(d *db.DB, now func() time.Time) *Reader {
	if d == nil || now == nil {
		panic("seller: NewReader missing dependency")
	}
	return &Reader{profile: profile.New(profile.Deps{DB: d, Now: now})}
}

// Store 讀取指定的 stint，包含已關閉的；語意同 Module.Store。
func (r *Reader) Store(ctx context.Context, storeID int64) (Store, error) {
	return r.profile.Store(ctx, storeID)
}

// Status 是 stint 的生命週期。
type Status = model.Status

// Profile 是店家可以編輯的欄位。
type Profile = model.Profile

// Store 是一個 stint。
type Store = model.Store

// Sanctions 是帳號讀取當下生效中的停權措施，停權狀態唯一的來源；nil（沒有推導過）視為停權。
type Sanctions = model.Sanctions

// NoSanctions 是推導的結果：帳號沒有生效中的停權措施。
func NoSanctions() *Sanctions { return model.NoSanctions() }

// SanctionedBy 是推導的結果：帳號有這些生效中的停權措施。
func SanctionedBy(measures ...EffectiveMeasure) *Sanctions { return model.SanctionedBy(measures...) }

// ExposableStore 是可以公開曝光的店家（StoreID 與 PublishedAt：公開時間，stores.first_exposed_at，沒被記下時是讀取當下）。
type ExposableStore = model.ExposableStore

// PublicStore 是可以公開曝光的店家連同公開資料最後更新的時間（UpdateTime）。
type PublicStore = model.PublicStore

// PublicStoreCursor 是公開店家清單的 keyset 位置。
type PublicStoreCursor = model.PublicStoreCursor

// ListPublicStoresInput 是公開店家清單的輸入。
type ListPublicStoresInput = model.ListPublicStoresInput

// PublicStorePage 是公開店家清單的一頁。
type PublicStorePage = model.PublicStorePage

// CardStatus 是店家卡片上的營運狀態：OPEN 是此刻可公開曝光，CLOSED 是已關閉，其餘是 PAUSED。
type CardStatus = model.CardStatus

// StoreCard 是曾經公開過的店家的卡片資料：店名、logo、封面與營運狀態。
type StoreCard = model.StoreCard

const (
	// CardOpen 營業中：此刻可以公開曝光。
	CardOpen = model.CardOpen
	// CardPaused 暫停：沒有關閉，但此刻不可公開曝光。
	CardPaused = model.CardPaused
	// CardClosed 已關閉。
	CardClosed = model.CardClosed
)

// CardStatusValues 回傳全部卡片狀態。
func CardStatusValues() []CardStatus { return model.CardStatusValues() }

// Requirement 是能力啟用或店家公開曝光的一項條件。
type Requirement = model.Requirement

const (
	// RequirementActive 店家沒有關閉。
	RequirementActive = model.RequirementActive
	// RequirementNotSuspended 帳號沒有生效中的停權。
	RequirementNotSuspended = model.RequirementNotSuspended
	// RequirementKYCApproved KYC 流程已核准（含 PayUni 核准）。
	RequirementKYCApproved = model.RequirementKYCApproved
	// RequirementStoreName 有店名。
	RequirementStoreName = model.RequirementStoreName
	// RequirementLogo 有 logo。
	RequirementLogo = model.RequirementLogo
	// RequirementTownCode 有鄉鎮市區代碼。
	RequirementTownCode = model.RequirementTownCode
	// RequirementNotPendingDeletion 店家沒有正在隨帳號刪除。
	RequirementNotPendingDeletion = model.RequirementNotPendingDeletion
)

// Requirements 回傳全部條件，依 CapabilitiesOf 回報的順序（與 proto 的 SellerRequirement 數字一致）。
func Requirements() []Requirement { return model.Requirements() }

// MissingRequirementsMetadataKey 是 SELLER_REQUIREMENT_MISSING 的 ErrorInfo.metadata 放缺的條件的 key。
const MissingRequirementsMetadataKey = model.MissingRequirementsMetadataKey

// CapabilityStatus 是一個能力此刻的狀態：啟用，或沒有啟用與缺的條件。
type CapabilityStatus = model.CapabilityStatus

// MaxBatchSize 是 ExposableStores 一次最多接受的 ID 數量。
const MaxBatchSize = model.MaxBatchSize

// ApplyInput 是申請開店的輸入。
type ApplyInput = model.ApplyInput

// AgreementKind 是賣家入駐時要同意的文件種類。
type AgreementKind = model.AgreementKind

// Agreement 是一份文件的現行版本與網址。
type Agreement = model.Agreement

// AgreementAcceptance 是賣家對一份文件的同意（client 顯示給賣家的版本）。
type AgreementAcceptance = model.AgreementAcceptance

// 文件種類。
const (
	AgreementSellerTerms      = model.AgreementSellerTerms
	AgreementKYCPrivacyNotice = model.AgreementKYCPrivacyNotice
	AgreementPayUniTerms      = model.AgreementPayUniTerms

	AgreementFoodRegistrationNotice = model.AgreementFoodRegistrationNotice
)

// CurrentAgreement 回傳一種文件的現行版本與網址；不認得的種類回 false。
func CurrentAgreement(k AgreementKind) (Agreement, bool) {
	a, ok := model.CurrentAgreements[k]
	return a, ok
}

// AgreementKinds 回傳全部文件種類，依契約的數字順序。
func AgreementKinds() []AgreementKind { return model.AgreementKinds() }

// CurrentApplyAcceptances 是 Apply 要求的同意（SELLER_TERMS 的現行版本），CurrentKYCAcceptances 是 SubmitKYC 要求的同意
// （KYC_PRIVACY_NOTICE 與 PAYUNI_TERMS 的現行版本）：給測試組出合法的輸入，不是 client 版本的來源（client 讀 GetSellerAgreements）。
func CurrentApplyAcceptances() []AgreementAcceptance {
	return currentAcceptances(model.ApplyAgreements)
}

// CurrentFoodRegistrationAcceptances 見 CurrentApplyAcceptances（SubmitFoodRegistration 要求的 FOOD_REGISTRATION_NOTICE）。
func CurrentFoodRegistrationAcceptances() []AgreementAcceptance {
	return currentAcceptances(model.SubmitFoodRegistrationAgreements)
}

// CurrentKYCAcceptances 見 CurrentApplyAcceptances。
func CurrentKYCAcceptances() []AgreementAcceptance {
	return currentAcceptances(model.SubmitKYCAgreements)
}

func currentAcceptances(kinds []AgreementKind) []AgreementAcceptance {
	out := make([]AgreementAcceptance, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, AgreementAcceptance{Kind: k, Version: model.CurrentAgreements[k].Version})
	}
	return out
}

// UpdateProfileInput 是 profile 的部分更新。
type UpdateProfileInput = model.UpdateProfileInput

// ClosedBy 是關閉 stint 的原因（賣家自己、終止合作或隨帳號刪除）。
type ClosedBy = model.ClosedBy

// Closure 是 stint 目前這一次關閉的紀錄：關店時間與原因；終止合作另有造成關店的措施。
type Closure = model.Closure

// StoreOpening 是帳號此刻能不能申請開店與不能的原因（依優先序）。
type StoreOpening = model.StoreOpening

// OpeningBlocker 是帳號不能申請開店的一個原因。
type OpeningBlocker = model.OpeningBlocker

// OpeningUnavailability 是帳號不能申請開店的一個原因與它給使用者看的一句話（語系取自 context 的 uitext.LocaleFrom）。
type OpeningUnavailability = model.OpeningUnavailability

const (
	// OpeningTerminated 帳號有生效中的終止合作（條款第 21.3 條）。
	OpeningTerminated = model.OpeningTerminated
	// OpeningSuspended 帳號停權中。
	OpeningSuspended = model.OpeningSuspended
	// OpeningAccountDeleting 帳號正在刪除。
	OpeningAccountDeleting = model.OpeningAccountDeleting
	// OpeningStoreExists 帳號已經有目前的店家。
	OpeningStoreExists = model.OpeningStoreExists
)

// OpeningBlockerValues 回傳全部原因，依優先序。
func OpeningBlockerValues() []OpeningBlocker { return model.OpeningBlockerValues() }

// StoreCursor 是後台店家清單的 keyset 位置。
type StoreCursor = model.StoreCursor

// StoreListInput 是後台店家清單的輸入。
type StoreListInput = model.StoreListInput

// StoreListItem 是後台店家清單的一列。
type StoreListItem = model.StoreListItem

// StorePage 是一頁店家。
type StorePage = model.StorePage

// StoreStats 是後台的店家統計。
type StoreStats = model.StoreStats

const (
	// ClosedByOwner 賣家自己關店。
	ClosedByOwner = model.ClosedByOwner
	// ClosedByTermination 終止合作生效後由系統關店。
	ClosedByTermination = model.ClosedByTermination
	// ClosedByDeletion 隨帳號刪除關閉。
	ClosedByDeletion = model.ClosedByDeletion
)

// ClosedByValues 回傳全部關閉者。
func ClosedByValues() []ClosedBy { return model.ClosedByValues() }

const (
	// Active 營業中的 stint。
	Active = model.Active
	// PendingDeletion 隨帳號刪除等待未結訂單結束的 stint。
	PendingDeletion = model.PendingDeletion
	// Closed 已關閉的 stint。
	Closed = model.Closed
)

// StatusValues 回傳全部儲存的生命週期狀態。
func StatusValues() []Status { return model.StatusValues() }

// Capability 是賣家能做的一類事（能力檢查 ADR 的粗粒度能力）。
type Capability = model.Capability

// Capabilities 回傳賣家能做的動作類能力（方法可以宣告、gateway 的 header 可以帶），不含狀態 CapPublicListing。
func Capabilities() []Capability { return model.Capabilities() }

// AllCapabilities 回傳 CapabilitiesOf 回報的全部能力：Capabilities() 加上狀態 CapPublicListing，順序與 proto 的 SellerCapability 一致。
func AllCapabilities() []Capability { return model.AllCapabilities() }

// ActiveCapabilities 回傳 CapabilitiesOf 結果中啟用的動作類能力（不含狀態 CapPublicListing）。
func ActiveCapabilities(statuses []CapabilityStatus) []Capability {
	return model.ActiveCapabilities(statuses)
}

// CapabilitiesOf 回報店家 s（模組讀出的店家，例如 CurrentStore 或 ActingStore 的結果）每個能力此刻的狀態與缺的條件，依
// AllCapabilities 的順序（唯一的推導點）。條件不在 Go 重算：讀店家時由 seller 的 SQL 帶上（Store.Conditions 與停權的 Sanctions），
// CapPublicListing 缺的條件就是公開曝光的規則（seller.pub_exposable_stores 的同一組條件）。「至少一個上架商品」由曝光端檢查。
func CapabilitiesOf(s Store) []CapabilityStatus {
	return model.CapabilitiesOf(s)
}

// Authorize 在店家 s 沒有能力 c 時回錯誤：已關閉回 NotFound；停權造成的回 PermissionDenied（帶 ReasonSellerSuspended）；其他前置條件沒有滿足
// （等待刪除、KYC 流程未核准）回 FailedPrecondition（帶 ReasonSellerRequirementMissing）。s 要是模組讀出的店家（CurrentStore、Store 或
// ActingStore 的結果），沒有推導過條件的一律拒絕。給已經拿到店家、需要依資源決定能力的呼叫方（例如 social 的店家身分寫入）。
func Authorize(s Store, c Capability) error { return model.Authorize(s, c) }

// Conditions 是店家與時間無關的條件此刻是否滿足，讀店家時由 seller 的 SQL 帶上（Store.Conditions）；沒有讀過時每一項都不滿足。
type Conditions = model.Conditions

const (
	CapAccountSelfRead  = model.CapAccountSelfRead
	CapStorefrontWrite  = model.CapStorefrontWrite
	CapProductManage    = model.CapProductManage
	CapOrderFulfill     = model.CapOrderFulfill
	CapOrderContract    = model.CapOrderContract
	CapMarketing        = model.CapMarketing
	CapContentContract  = model.CapContentContract
	CapChatSend         = model.CapChatSend
	CapChatRead         = model.CapChatRead
	CapNotification     = model.CapNotification
	CapAccountClose     = model.CapAccountClose
	CapInvoiceManage    = model.CapInvoiceManage
	CapInvoiceTaxRemedy = model.CapInvoiceTaxRemedy
	CapShippingSetup    = model.CapShippingSetup
	CapPublicListing    = model.CapPublicListing
)

// Apply 申請開店：全部輸入先驗證，建立 stint、寫入 profile、授予 SELLER 在同一個 transaction。已經有目前的店家時
// 回 AlreadyExists；帳號停權中（含先前 stint 上未解除的停權）或等待刪除時回 FailedPrecondition。
func (m *Module) Apply(ctx context.Context, in ApplyInput) (Store, error) {
	return m.onboarding.Apply(ctx, in)
}

// StoreOpening 回答帳號此刻能不能申請開店與不能的原因（依優先序：終止合作、停權、帳號正在刪除、已有店家）。它與 Apply 的前置條件是同一個狀態與
// 同一個判斷，所以 CanApply 為 true 時 Apply 不會被前置條件擋下。唯讀；不要求有店家或賣家角色。
func (m *Module) StoreOpening(ctx context.Context, accountID int64) (StoreOpening, error) {
	return m.onboarding.Opening(ctx, accountID)
}

// CurrentStore 讀取帳號目前的店家（status 不是 CLOSED），沒有時回 NotFound。停權的店家照樣回傳，Suspended 標示停權。
func (m *Module) CurrentStore(ctx context.Context, accountID int64) (Store, error) {
	return m.profile.CurrentStore(ctx, accountID)
}

// Store 讀取指定的 stint，包含已關閉的。
func (m *Module) Store(ctx context.Context, storeID int64) (Store, error) {
	return m.profile.Store(ctx, storeID)
}

// ExposableStores 從 storeIDs（最多 MaxBatchSize 個）挑出可以公開曝光的店家：營運中、沒有生效中的停權、KYC 流程已核准（含
// PayUni）、店名、logo 與鄉鎮市區代碼齊全（規則只有 seller.pub_exposable_stores）。依輸入順序回傳，不存在或不可曝光的略過；「至少一個上架
// 商品」由曝光端檢查。任何讀取失敗都回錯誤（fail-closed）。
func (m *Module) ExposableStores(ctx context.Context, storeIDs []int64) ([]ExposableStore, error) {
	return m.profile.ExposableStores(ctx, storeIDs)
}

// PublicStores 從 storeIDs（最多 MaxBatchSize 個）挑出可以公開曝光的店家並回傳完整資料：篩選、順序與失敗行為與
// ExposableStores 相同（同一個規則 seller.pub_exposable_stores）。給需要店家資料而且只能對公開的店家使用的呼叫方（例如追蹤）。
func (m *Module) PublicStores(ctx context.Context, storeIDs []int64) ([]Store, error) {
	return m.profile.PublicStores(ctx, storeIDs)
}

// PublicStoreByCode 以 store_code 讀取可以公開曝光的店家（店面頁）；不存在與不可曝光都回同一個 NotFound，不接受
// store_id。任何讀取失敗都回錯誤（fail-closed）。
func (m *Module) PublicStoreByCode(ctx context.Context, storeCode int64) (PublicStore, error) {
	return m.profile.PublicStoreByCode(ctx, storeCode)
}

// ListPublicStores 列出最後公告為可曝光、且此刻經同一個 classify 判定仍可曝光的店家（規則與 PublicStoreByCode 相同），依 store_id 由小到大的
// keyset 分頁（預設 100、上限 1000）：一頁可以少於 PageSize，結束只看 Next 是不是 nil；剛變得可曝光的店在公告之後才出現。讀取失敗回錯誤，不回部分結果。
func (m *Module) ListPublicStores(ctx context.Context, in ListPublicStoresInput) (PublicStorePage, error) {
	return m.profile.ListPublicStores(ctx, in)
}

// StoreCards 從 storeIDs（最多 MaxBatchSize 個）挑出曾經公開過的店家（first_exposed_at 有值，或此刻可公開曝光），回傳卡片資料
// （店名、logo、封面、營運狀態），依輸入順序；不存在與從沒公開過的略過。停權與關閉的店照樣回傳，個資清除之後店名是 nil。任何讀取失敗都回錯誤。
func (m *Module) StoreCards(ctx context.Context, storeIDs []int64) ([]StoreCard, error) {
	return m.profile.StoreCards(ctx, storeIDs)
}

// StoreExposure 是店家（含已關閉的 stint）與它此刻是否可以公開曝光。
type StoreExposure = model.StoreExposure

// BatchStores 只為 legacy 的 SellerService（GetSellerBatch）存在，退場：呼叫方（im、aftersales）搬進單體、改呼叫 Store 或 StoreCards
// 時：批次讀取店家（最多 MaxBatchSize 個），含已關閉的 stint，每個帶推導的停權；依輸入順序，不存在的略過；不套用曝光規則。
func (m *Module) BatchStores(ctx context.Context, storeIDs []int64) ([]Store, error) {
	return m.profile.BatchStores(ctx, storeIDs)
}

// StoresWithExposure 只為 legacy 的 SellerService（BatchGetSellerMeta）存在，退場：Java 的 ordering、catalog 搬進單體（B4、B8）、
// 改呼叫 ExposableStores 與 Store.Can 時：BatchStores 加上每個店家此刻是否可以公開曝光（規則仍只有 seller.pub_exposable_stores）。
func (m *Module) StoresWithExposure(ctx context.Context, storeIDs []int64) ([]StoreExposure, error) {
	return m.profile.StoresWithExposure(ctx, storeIDs)
}

// ResolvePrincipals 只為 legacy 的 SellerService（ResolvePrincipals）存在，退場：呼叫方搬進單體、改呼叫
// Store 時：回傳存活（沒有關閉）的 stint 的 store_id 到所屬帳號的對照，已關閉與不存在的 ID 不在結果裡（舊契約的 unresolved：
// 關店在舊 seller-service 等於軟刪除）；最多 MaxBatchSize 個。
func (m *Module) ResolvePrincipals(ctx context.Context, storeIDs []int64) (map[int64]int64, error) {
	return m.profile.ResolvePrincipals(ctx, storeIDs)
}

// StoreIDsByAccount 只為 legacy 的 SellerService（ListStoreIDsByAccount）存在，退場：im 與 notification 的個資清除搬進單體、由
// accountdeletion 的參與者動作取代時：帳號全部的 stint（含已關閉的），依 store_id。
func (m *Module) StoreIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	return m.profile.StoreIDsByAccount(ctx, accountID)
}

// ExposableStoreIDsByCounty 只為 legacy 的 SellerService（ListPublicStoreIDsByCity、ListPublicStoresByCity）存在，退場：product 的
// 縣市篩選搬進單體（B4）、改以 catalog 的讀取模型時：每個縣市（內政部 5 碼縣市代碼）裡最後公告為可曝光的店家 ID，依 store_id；
// 對每個要求的縣市都有鍵。讀的是公告表（與 StoreExposureChanged 同一份），不是現算的規則，落後最多一個對帳週期。
func (m *Module) ExposableStoreIDsByCounty(ctx context.Context, countyCodes []string) (map[string][]int64, error) {
	return m.profile.ExposableStoreIDsByCounty(ctx, countyCodes)
}

// ActingStore 回傳帳號目前的店家，只在它有能力 c 時成功：停權造成的拒絕回 PermissionDenied，其他前置條件沒有滿足（等待刪除、
// KYC 未核准）回 FailedPrecondition（SELLER_REQUIREMENT_MISSING），沒有店家回 NotFound。每次讀 DB；回傳錯誤時呼叫方一律拒絕（fail-closed）。
func (m *Module) ActingStore(ctx context.Context, accountID int64, c Capability) (Store, error) {
	return m.suspension.ActingStore(ctx, accountID, c)
}

// UpdateProfile 部分更新 profile，可同時換 logo 與封面（LogoToken、CoverToken；換圖必須在 transaction 外呼叫），需要
// CapStorefrontWrite；ExpectedVersion 不符回 VersionConflict，沒有任何改變時回傳目前的店家、不寫入。
func (m *Module) UpdateProfile(ctx context.Context, in UpdateProfileInput) (Store, error) {
	return m.profile.UpdateProfile(ctx, in)
}

// CompleteOnboarding 標記新手引導完成，只設旗標、不重新驗證其他欄位；需要 CapStorefrontWrite，版本不符回
// VersionConflict，已完成時回傳目前的店家（冪等）。
func (m *Module) CompleteOnboarding(ctx context.Context, storeID, expectedVersion int64) (Store, error) {
	return m.profile.CompleteOnboarding(ctx, storeID, expectedVersion)
}

// CloseCurrent 關閉帳號目前的店家（關店；帳號與買家留著）；停權中也可以關，等待刪除時回 FailedPrecondition。
func (m *Module) CloseCurrent(ctx context.Context, accountID int64) (Store, error) {
	return m.closing.CloseCurrent(ctx, accountID)
}

// EffectTermination 是 seller.termination_effect job 的內容（給 worker 與測試呼叫）：終止合作生效時對齊停權的效果，沒有未結訂單就以
// TERMINATION 關店；有未結訂單或措施已撤銷、已結束就什麼都不做。重跑沒有副作用。
func (m *Module) EffectTermination(ctx context.Context, measureID uuid.UUID) error {
	return m.closing.EffectTermination(ctx, measureID)
}

// AdvanceTerminations 是店家的訂單結案（order.completed）時的加速：店家所屬帳號生效中的終止合作各跑一次 EffectTermination。事件只加速收斂，
// ReconcileTerminations 是保底。
func (m *Module) AdvanceTerminations(ctx context.Context, storeID int64) error {
	return m.closing.AdvanceStore(ctx, storeID)
}

// ReconcileTerminations 是 seller.reconcile_terminations job（每小時）的內容：此刻生效中的終止合作而帳號還有目前 stint 的，逐一跑
// EffectTermination；回傳掃描的措施數。
func (m *Module) ReconcileTerminations(ctx context.Context) (int, error) {
	return m.closing.ReconcileTerminations(ctx)
}

// AdminStores 是後台的店家清單（預設只列目前的店家，店名、電話或 store_code 搜尋，依 KYC 申請狀態篩選，新到舊的 keyset）；
// 每一列帶推導的停權狀態與 KYC 流程狀態。
func (m *Module) AdminStores(ctx context.Context, in StoreListInput) (StorePage, error) {
	return m.profile.AdminList(ctx, in)
}

// AdminStoreStats 是後台的店家統計：目前的店家總數、依 KYC 申請狀態的家數與台北本月新開的家數，同一個 snapshot。
func (m *Module) AdminStoreStats(ctx context.Context) (StoreStats, error) {
	return m.profile.AdminStats(ctx)
}

// LockForDeletion 在呼叫方的 transaction 內鎖住店家所屬帳號的店家列（持有到 transaction 結束）。accountdeletion 在派發買家的刪除事件之前，
// 先依單體規則 4 的順序（帳號列、買家列、店家列、上層模組）鎖完買家與店家。必須在 transaction 內呼叫；只由 accountdeletion 在已鎖住帳號列之後呼叫。
func (m *Module) LockForDeletion(ctx context.Context, storeID int64) error {
	return m.closing.Lock(ctx, storeID)
}

// BeginDeletion 讓店家隨帳號刪除進入等待；重送安全。只由 accountdeletion 在已鎖住帳號列的 transaction 內呼叫。
func (m *Module) BeginDeletion(ctx context.Context, storeID int64) error {
	return m.closing.BeginDeletion(ctx, storeID)
}

// CloseForDeletion 關閉店家；重送安全。只由 accountdeletion 在已鎖住帳號列、同一個 transaction 刪除帳號時呼叫。
func (m *Module) CloseForDeletion(ctx context.Context, storeID int64) error {
	return m.closing.CloseForDeletion(ctx, storeID)
}

// SyncMedia 同步一個 media ref 的引用：Link 業務列目前的資產，Unlink 退場的資產。ref 是店家的 logo 與封面（ref_id 是
// store_id）、站外業者的 logo 與封面（prospect_id）、KYC 證件（application_id）或治理佐證（通報或違規的 ID）。它是
// seller.media_sync job 的內容，給 worker 與測試呼叫；回傳是否要再跑一輪。
func (m *Module) SyncMedia(ctx context.Context, ref media.Ref) (media.SyncResult, error) {
	return m.mediaSync.Sync(ctx, ref)
}

// SweepMedia 執行 media 引用的保底掃描：清掉過期的退場紀錄，補排版本比已同步的新的 ref；回傳補排的 ref 數。它是
// seller.media_sweep job 的內容，給 worker 與測試呼叫。
func (m *Module) SweepMedia(ctx context.Context) (int, error) {
	return m.mediaSync.Sweep(ctx)
}

// AddWorkers 登記本模組的 River worker。
func (m *Module) AddWorkers(workers *river.Workers) {
	mediaref.AddWorkers(workers, m.mediaSync)
	m.suspension.AddWorkers(workers)
	m.closing.AddWorkers(workers)
	m.measure.AddWorkers(workers)
	m.governance.AddWorkers(workers)
	m.payuni.AddWorkers(workers)
	m.kyc.AddWorkers(workers)
	m.purge.AddWorkers(workers)
	m.profile.AddWorkers(workers)
	m.views.AddWorkers(workers)
}

// PeriodicJobs 回傳本模組的排程 job：第一次公開的記錄（每 15 分鐘）、措施的對帳、終止合作的對帳（每小時）、措施的結案掃描（每 5 分鐘）、匿名通報人聯絡欄位的清理、關店滿保留期的個資清理（啟動即執行、每小時
// 補跑）、商品瀏覽事件的彙總（每 5 分鐘）與過期事件的清理（每小時），以及 media 引用的保底掃描（每 15 分鐘）與 PayUni 的掃描（補排遺失的 sync、排輪詢，啟動即執行、每 15
// 分鐘），以及 KYC 登記資料查詢的掃描（補排遺失的查詢 job、把過期的 PENDING 收成終態，啟動即執行、每 1 分鐘）。
func (m *Module) PeriodicJobs() []*river.PeriodicJob {
	jobs := append(m.suspension.PeriodicJobs(), m.closing.PeriodicJobs()...)
	jobs = append(jobs, m.measure.PeriodicJobs()...)
	jobs = append(jobs, m.governance.PeriodicJobs()...)
	jobs = append(jobs, m.purge.PeriodicJobs()...)
	jobs = append(jobs, m.profile.PeriodicJobs()...)
	jobs = append(jobs, m.views.PeriodicJobs()...)
	jobs = append(jobs, m.payuni.PeriodicJobs()...)
	jobs = append(jobs, m.kyc.PeriodicJobs()...)
	return append(jobs, mediaref.PeriodicJobs()...)
}

// ReconcileExposure 讓每家店最後公告的可曝光與現在算出來的一致（不同的更新並發 StoreExposureChanged，可曝光時記下
// first_exposed_at），回傳掃描的店家數；候選是所有還沒關閉的店加上最後公告為可曝光的店。它是 seller.reconcile_exposure job
// （每 15 分鐘）的內容，給 worker 與測試呼叫。
func (m *Module) ReconcileExposure(ctx context.Context) (int, error) {
	return m.profile.ReconcileExposure(ctx)
}

// DevSeedStore 是本機開發種子的一家店（明文 fixture）。
type DevSeedStore = devseed.Store

// DevSeedAccountRange 是 01-account.sql 會刪除重建的一段帳號 ID（含兩端）。
type DevSeedAccountRange = devseed.AccountRange

// SeedDevStores 在一個 transaction 內把本機開發的種子店家還原成 fixture（upsert，只動這幾家店與它們的子列，不碰別的店家）：固定的 store_id 與
// store_code、已核准的 KYC（識別欄位以執行中的金鑰加密）、已綁定而且 PayUni 核准的註冊、logo；公告列保留，version 只增不減。只給本機開發的
// dev-seed 子命令使用：它直接寫列，繞過開店、送出、審核與綁定的狀態轉移；profile 有變時發 StoreProfileChanged，刪掉的證件與佐證經退場機制釋放
// media 的引用；停權、關閉、等待刪除這些有下游消費者的生命週期狀態不還原，前提檢查發現就拒絕。寫完之後要 ReannounceExposures 讓下游知道可曝光。
// rebuilt 是 01-account.sql 重建的帳號區間，區間內有種子以外的未關閉店家時整個拒絕（見 CheckDevSeed）。必須在 transaction 外呼叫。
func (m *Module) SeedDevStores(ctx context.Context, stores []DevSeedStore, rebuilt []DevSeedAccountRange) error {
	return m.devSeed.Seed(ctx, stores, rebuilt)
}

// CheckDevSeed 不寫任何資料，確認 SeedDevStores 可以安全執行（在 01-account.sql 之前呼叫）：rebuilt 內沒有種子以外的未關閉店家，也沒有種子以外的店家
// 佔用種子的 store_code，種子店家沒有生效中的停權或處分、停權期間，也不是 CLOSED 或 PENDING_DELETION。不成立時回的錯誤列出店家與處理方式。
func (m *Module) CheckDevSeed(ctx context.Context, stores []DevSeedStore, rebuilt []DevSeedAccountRange) error {
	return m.devSeed.Check(ctx, stores, rebuilt)
}

// ReannounceExposures 由 seller 全量重新公告可曝光：每一家店（含已關閉的）都以現在算出來的值更新公告並發 StoreExposureChanged，不比對
// 最後公告的值，版本照常加一，回傳公告的店家數。用在種子重寫之後，以及 Java 的 store_exposure 重建或死信處理之後（下游依版本丟棄舊的
// 事件，所以重發一定新於它已有的值）。B4 的 catalog 改成同一個 transaction 的讀取模型時刪除。cmd/hawkr 的 reannounce-exposures 呼叫它。
func (m *Module) ReannounceExposures(ctx context.Context) (int, error) {
	return m.profile.ReannounceExposures(ctx)
}

// PurgePII 跑關店之後的兩段清理（ADR 2026-10-19-seller-kyc-payuni-consolidated「關店與個資保留」與 2026-10-20 修訂）：第一段（退場）在一般關店 30 天
// （StoreRetireAfter；終止合作關店不早於申覆期限）後清公開內容、釋放 logo 與封面並發 StoreRetired；第二段在關店 184 天（StoreRetention）後清註冊、身分與連線資料
// （店家聯絡、KYC、PayUni 的自由文字、食品登錄字號、同意紀錄的來源）。申覆的 legal hold 與主管機關來函的保存擋兩段。分批處理到沒有候選為止，回傳第二段清理了個資的店家數（退場看 retired_at）；
// 重跑是 no-op。它是 seller.purge_pii job 的內容，給 worker 與測試呼叫。
func (m *Module) PurgePII(ctx context.Context) (int, error) {
	return m.purge.Run(ctx)
}

// ExpireMeasure 在措施結束時重算帳號的停權期間（以 max(endsAt, 現在) 判斷）；它是 seller.measure_expiry job 的內容，
// 給 worker 與測試呼叫。
func (m *Module) ExpireMeasure(ctx context.Context, measureID uuid.UUID, endsAt time.Time) error {
	return m.suspension.Expire(ctx, measureID, endsAt)
}

// CloseEndedMeasureRequests 把已結束的措施上失去標的的待處理解除請求以 CLOSED（ENDED）結案，回傳結案的請求數；它是
// seller.measure_close_ended_requests job 的內容，給 worker 與測試呼叫。
func (m *Module) CloseEndedMeasureRequests(ctx context.Context) (int, error) {
	return m.measure.CloseEndedRequests(ctx)
}

// ReconcileSuspensions 修補停權期間與措施不一致的帳號（只掃描有進行中期間或有未撤銷停權措施的帳號），回傳掃描的帳號數；
// 它是 seller.reconcile_suspensions job 的內容。
func (m *Module) ReconcileSuspensions(ctx context.Context) (int, error) {
	return m.suspension.Reconcile(ctx)
}

package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

const checkoutPreviewTTL = 2 * time.Minute

const invoiceDisclosureTermsVersion = "2026-08-30"

const (
	disclosureReceiptTitle = "本筆交易由賣家開立收據"
	disclosureReceiptBody  = "本賣家經國稅局核定免用統一發票，本筆交易將由平台代製收據，不會開立電子發票。" +
		"收據可於「我的發票」查看與下載。"

	disclosureSelfIssueTitle = "本筆交易由賣家自行開立發票"
	disclosureSelfIssueBody  = "本賣家的稅務資格尚未完成平台代開設定，本筆交易的統一發票由賣家自行開立並交付，" +
		"平台不會代為開立。如未收到發票，請直接與賣家聯繫。"
)

var (
	reMobileBarcode = regexp.MustCompile(`^/[0-9A-Z+\-.]{7}$`)
	reCitizenCert   = regexp.MustCompile(`^[A-Z]{2}[0-9]{14}$`)
	reDonationCode  = regexp.MustCompile(`^[0-9]{3,7}$`)
)

const buyerNameMaxRunes = 60

// Checkout 承載結帳鏈的兩支內部 RPC 的全部業務判斷。
//
// 它刻意**沒有** TxManager：兩支都是唯讀的。9326／9327 的「必須在 CheckoutConfirm 的既有
// 交易內完成」（§6.1.2）指的是 **order-service 那一邊**的交易 —— 跨服務 RPC 進不去別人的
// 交易，本服務能給的是「呼叫當下的現況」，最終判定與累加仍在 order-service 的建單交易內。
type Checkout struct {
	issuers  domain.IssuerRepository
	profiles domain.SellerTaxProfileRepository
	accounts domain.ProviderAccountRepository
	prefs    domain.BuyerPreferenceRepository

	now func() time.Time
	log *zap.Logger
}

// NewCheckout 組出 usecase。
func NewCheckout(
	issuers domain.IssuerRepository,
	profiles domain.SellerTaxProfileRepository,
	accounts domain.ProviderAccountRepository,
	prefs domain.BuyerPreferenceRepository,
	log *zap.Logger,
) *Checkout {
	return &Checkout{
		issuers:  issuers,
		profiles: profiles,
		accounts: accounts,
		prefs:    prefs,
		now:      func() time.Time { return time.Now().UTC() },
		log:      log,
	}
}

// WithClock 注入時間來源（測試用）。
func (u *Checkout) WithClock(now func() time.Time) *Checkout {
	if now != nil {
		u.now = now
	}
	return u
}

// CheckoutAmount 是發票金額拆解（proto InvoiceAmountBreakdown）。
type CheckoutAmount struct {
	ProductAmount  int64
	ShippingAmount int64
	DiscountAmount int64
	// InvoiceAmount ＝ max(0, product + shipping − discount)，含稅。
	InvoiceAmount int64
	// TaxCategory 是賣家的預設課稅別（1 應稅／2 零稅率／3 免稅）。
	TaxCategory int16
}

// CheckoutDisclosure 是法定告知區塊（proto InvoiceDisclosure）。
type CheckoutDisclosure struct {
	Required     bool
	TermsVersion string
	Title        string
	Body         string
}

// CheckoutIntakeLimit 是待審接單上限（proto SellerIntakeLimit）。
//
// Applicable=false ⇒ 該賣家不在待審期間 ⇒ 對外回 null（§6.1.2「非待審期間為 null」）。
// 用一個獨立的布林而不是「四個數字都是 0 就當成 null」：後者分不出
// 「不在待審期間」與「在待審期間但一單都還沒接」，而那兩者的 UI 完全不同。
type CheckoutIntakeLimit struct {
	Applicable      bool
	OrderCountUsed  int32
	OrderCountLimit int32
	AmountUsed      int64
	AmountLimit     int64
	Blocked         bool
}

// CheckoutProfile 是買家常用發票資料的對外投影（proto InvoiceProfileInfo）。
//
// DisplayValue 恆為遮罩後的值；明碼不出服務邊界（§6.1.1 遮罩段）。
type CheckoutProfile struct {
	Found        bool
	ProfileID    int64
	CarrierType  string // CodeMeta InvoiceCarrierType 的 Value
	DisplayValue string
	BuyerTaxID   string
	BuyerName    string
	IsDefault    bool
	Label        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ResolveModeInput 是 preview 的全部輸入。
type ResolveModeInput struct {
	StoreID int64
	// BuyerID 是買家業務鍵（buyers.buyer_id），不是 account_id。<=0 表示訪客／未解析，
	// 此時不查常用發票資料（查不到與沒有買家是兩件事，不可互相冒充）。
	BuyerID        int64
	ProductAmount  int64
	ShippingAmount int64
	DiscountAmount int64
	CheckoutID     string
}

// ResolveModeOutput 是 preview 的結果。
type ResolveModeOutput struct {
	InvoiceMode         string
	DocumentKind        string
	NotApplicableReason string
	PreviewExpiresAt    time.Time
	Amount              CheckoutAmount
	Disclosure          CheckoutDisclosure
	DefaultProfile      CheckoutProfile
	IntakeLimit         CheckoutIntakeLimit
}

// CheckoutInvoiceSpec 是 confirm 帶進來的買家發票輸入（proto InvoiceInputSpec）。
//
// Present=false 代表整包 `invoice` 被省略 —— 那是**合法且常見**的形狀（§6.1.2(b)：
// 已上架的 App 直接呼叫這支端點，改成必填會立刻打爛線上結帳），走缺省鏈。
type CheckoutInvoiceSpec struct {
	Present       bool
	ProfileID     int64
	CarrierType   string // CodeMeta InvoiceCarrierType 的 Value
	Value         string
	BuyerTaxID    string
	BuyerName     string
	Email         string
	SaveAsDefault bool
}

// ValidateInvoiceInput 是 confirm 的全部輸入。
type ValidateInvoiceInput struct {
	StoreID         int64
	BuyerID         int64
	OrderNo         string
	Invoice         CheckoutInvoiceSpec
	Amount          CheckoutAmount
	SnapshotVersion int32
	IdempotencyKey  string
}

// ValidateInvoiceOutput 是 confirm 的結果。
//
// BizCode 為 0 且 Valid 為 true 才是放行。兩個欄位刻意都存在而不是只留一個：
// 呼叫端要的是「能不能成單」（Valid），前端要的是「為什麼不行」（BizCode）。
type ValidateInvoiceOutput struct {
	Valid                bool
	BizCode              int32
	Message              string
	SnapshotJSON         []byte
	ResolvedCarrierType  string
	ResolvedDisplayValue string
	InvoiceMode          string
	IntakeLimit          CheckoutIntakeLimit
}

// ResolveMode 解析這一筆結帳的開票模式、金額拆解、法定告知與待審接單上限。
//
// 🔴 履約模式一律經 domain.ResolveFulfillmentMode —— 與 order.paid 的 Prepare 同一支。
//
// **preview 與實際開票的兩個已知差異**（都不影響 invoice_mode，這是它們可被接受的理由）：
//
//	① InvoiceDate 用「現在」而不是付款時點（付款還沒發生）。只有跨越 automation_cutover_at
//	   的那一瞬間會讓 preview 與實際落點不同，而那是切點本身的語意。
//	② fail-closed 三格（平台出資券／和單／混合稅別）在 preview 一律以 false 餵入：
//	   結帳當下沒有 order 快照可判。三格影響的是 document.status 與 manual_reason_code
//	   （AWAITING_MANUAL），**不影響 fulfillment_mode 與 not_applicable_reason**
//	   —— §7.1.3 的 AM 葉節點回的仍是 AMEGO_AUTO。所以 preview 對外承諾的那兩個值不受影響。
func (u *Checkout) ResolveMode(ctx context.Context, in ResolveModeInput) (ResolveModeOutput, error) {
	now := u.now()

	facts, err := u.sellerFacts(ctx, in.StoreID)
	if err != nil {
		return ResolveModeOutput{}, err
	}

	profile, err := u.defaultProfile(ctx, in.BuyerID)
	if err != nil {
		return ResolveModeOutput{}, err
	}

	amount := buildCheckoutAmount(in.ProductAmount, in.ShippingAmount, in.DiscountAmount, facts.profile)
	decision := domain.ResolveFulfillmentMode(fulfillmentInputFor(facts, amount.InvoiceAmount, now, profile.hasTaxID()))

	return ResolveModeOutput{
		InvoiceMode:         decision.FulfillmentMode,
		DocumentKind:        decision.DocumentKind,
		NotApplicableReason: decision.NotApplicableReason,
		PreviewExpiresAt:    now.Add(checkoutPreviewTTL),
		Amount:              amount,
		Disclosure:          disclosureFor(decision.FulfillmentMode),
		DefaultProfile:      profile,
		IntakeLimit:         intakeLimitOf(facts.profile),
	}, nil
}

// ValidateInput 驗證買家的發票輸入並回傳可持久化的快照。
//
// 檢查順序（每一格失敗即回，不累積）：
//
//  1. 法定告知           → 9314
//  2. 常用發票資料歸屬    → 9302（Tier A：查不到與不屬於你同碼同訊息，§8.1.4）
//  3. 逐欄格式           → 9305（§6.1.1 驗證表）
//  4. 三者互斥           → 9306（走 domain.IssueDraft.Validate 的 A15，不另寫一套）
//  5. 待審接單上限        → 9326
//
// **9327（和單）不在這裡**：本 request 沒有、也不該有 `consolidated` 欄 —— 那是訂單的性質，
// 由 order-service 在自己的建單交易內判定（§6.1.2 該列）。在這裡加一個欄位讓呼叫端自己回報
// 自己是不是和單，等於把一道 fail-closed 檢查改成自我申報。
func (u *Checkout) ValidateInput(ctx context.Context, in ValidateInvoiceInput) (ValidateInvoiceOutput, error) {
	now := u.now()

	facts, err := u.sellerFacts(ctx, in.StoreID)
	if err != nil {
		return ValidateInvoiceOutput{}, err
	}

	amount := buildCheckoutAmount(in.Amount.ProductAmount, in.Amount.ShippingAmount, in.Amount.DiscountAmount, facts.profile)
	decision := domain.ResolveFulfillmentMode(fulfillmentInputFor(facts, amount.InvoiceAmount, now, false))
	mode := decision.FulfillmentMode
	limit := intakeLimitOf(facts.profile)

	reject := func(r checkoutRejection) (ValidateInvoiceOutput, error) {
		return ValidateInvoiceOutput{
			Valid:       false,
			BizCode:     r.Code,
			Message:     r.Message,
			InvoiceMode: mode,
			IntakeLimit: limit,
		}, nil
	}

	identity, rej, err := u.resolveIdentity(ctx, in.BuyerID, in.Invoice)
	if err != nil {
		return ValidateInvoiceOutput{}, err
	}
	if !rej.ok() {
		return reject(rej)
	}

	if code := u.assertBuyerIdentity(amount, identity); code != 0 {
		return reject(rejectWith(code))
	}

	if limit.Blocked {
		return reject(rejectWith(bizcode.SellerInvoiceIntakeLimitReached))
	}

	snapshot, err := marshalOrderInvoiceSnapshot(in, identity)
	if err != nil {
		return ValidateInvoiceOutput{}, err
	}

	return ValidateInvoiceOutput{
		Valid:                true,
		BizCode:              0,
		Message:              "success",
		SnapshotJSON:         snapshot,
		ResolvedCarrierType:  identity.CarrierType,
		ResolvedDisplayValue: identity.DisplayValue,
		InvoiceMode:          mode,
		IntakeLimit:          limit,
	}, nil
}

type checkoutSellerFacts struct {
	issuer  domain.Issuer
	profile domain.SellerTaxProfile
	account domain.ProviderAccount
}

func (u *Checkout) sellerFacts(ctx context.Context, storeID int64) (checkoutSellerFacts, error) {
	var out checkoutSellerFacts
	if storeID <= 0 {
		return out, fmt.Errorf("invoice: checkout requires a store_id (got %d)", storeID)
	}

	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err != nil {
		if !isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
			return out, err
		}
		return out, nil
	}
	out.issuer = issuer

	profile, err := u.profiles.GetByIssuer(ctx, issuer.ID)
	if err != nil && !isBizCode(err, bizcode.SellerTaxProfileNotFound) {
		return out, err
	}
	out.profile = profile

	account, err := u.accounts.GetByIssuer(ctx, issuer.ID, providerAmego)
	if err != nil && !isBizCode(err, bizcode.InvoiceProviderAccountNotFound) {
		return out, err
	}
	out.account = account

	return out, nil
}

func fulfillmentInputFor(f checkoutSellerFacts, invoiceAmount int64, now time.Time, buyerHasTaxID bool) domain.FulfillmentInput {
	return domain.FulfillmentInput{
		TotalActualAmount:       invoiceAmount,
		SellerReviewStatus:      f.profile.ReviewStatus,
		ProviderStatus:          f.account.Status,
		AutomationCutoverAt:     f.account.AutomationCutoverAt,
		InvoiceDate:             now,
		BuyerHasTaxID:           buyerHasTaxID,
		SellerReceiptAuthorized: f.profile.ReceiptAuthorizedAt != nil,
	}
}

func buildCheckoutAmount(product, shipping, discount int64, profile domain.SellerTaxProfile) CheckoutAmount {
	total := product + shipping - discount
	if total < 0 {
		total = 0
	}
	cat := profile.DefaultTaxCategory
	if cat == 0 {
		cat = int16(domain.TaxCategoryTaxable)
	}
	return CheckoutAmount{
		ProductAmount:  product,
		ShippingAmount: shipping,
		DiscountAmount: discount,
		InvoiceAmount:  total,
		TaxCategory:    cat,
	}
}

// intakeLimitOf 由賣家稅務資格算出待審接單上限。
//
// **判準一律讀該賣家那一列**（§6.1.2 C5 第 1 條）：讀 env 會讓一次調參回溯改變所有既有賣家。
// 「待審期間」＝ review_status ∈ {SELF_ISSUE_PENDING, SUPPLEMENT_REQUIRED} —— 值域取自
// §5.1.2 的 chk_stp_pending_window 與同節那條 partial index，不是自己挑的兩個狀態。
// IntakeLimitApplies 是「這位賣家現在適不適用待審接單上限」的**唯一**判準。
//
// 🔴 判準是 review_status，**不是「limit 欄位有沒有設定」**：那兩欄有 DB 預設值
// （50／100000），拿它們判等於恆為真（#3089 第 10 項：dev 上 6/6 列都是預設值，
// 於是 ADM 每一頁都渲染「待審期間接單限制 0/50 筆」）。
func IntakeLimitApplies(p domain.SellerTaxProfile) bool {
	return p.ReviewStatus == codemeta.SellerTaxReviewStatusSelfIssuePending.Value ||
		p.ReviewStatus == codemeta.SellerTaxReviewStatusSupplementRequired.Value
}

func intakeLimitOf(p domain.SellerTaxProfile) CheckoutIntakeLimit {
	if !IntakeLimitApplies(p) {
		return CheckoutIntakeLimit{}
	}
	return CheckoutIntakeLimit{
		Applicable:      true,
		OrderCountUsed:  p.PendingOrderCount,
		OrderCountLimit: p.PendingOrderLimitCount,
		AmountUsed:      p.PendingOrderAmount,
		AmountLimit:     p.PendingOrderLimitAmount,
		Blocked: p.PendingOrderCount >= p.PendingOrderLimitCount ||
			p.PendingOrderAmount >= p.PendingOrderLimitAmount,
	}
}

func disclosureFor(mode string) CheckoutDisclosure {
	switch mode {
	case codemeta.InvoiceFulfillmentModeAmegoAuto.Value:
		return CheckoutDisclosure{Required: false}
	case codemeta.InvoiceFulfillmentModePlatformReceipt.Value:
		return CheckoutDisclosure{
			Required:     true,
			TermsVersion: invoiceDisclosureTermsVersion,
			Title:        disclosureReceiptTitle,
			Body:         disclosureReceiptBody,
		}
	default:
		return CheckoutDisclosure{
			Required:     true,
			TermsVersion: invoiceDisclosureTermsVersion,
			Title:        disclosureSelfIssueTitle,
			Body:         disclosureSelfIssueBody,
		}
	}
}

type checkoutRejection struct {
	Code    int32
	Message string
}

func (r checkoutRejection) ok() bool { return r.Code == 0 }

func rejectWith(code int32) checkoutRejection {
	return checkoutRejection{Code: code, Message: bizcode.Message(code)}
}

type checkoutIdentity struct {
	SelectionMode string
	PreferenceID  int64
	CarrierType   string
	CarrierValue  string
	DisplayValue  string
	BuyerTaxID    string
	BuyerName     string
	DonationCode  string

	HasCarrier bool

	CarrierValueVisible bool
}

func (p CheckoutProfile) hasTaxID() bool {
	return p.Found && domain.IsBusinessTaxID(p.BuyerTaxID)
}

func (u *Checkout) defaultProfile(ctx context.Context, buyerID int64) (CheckoutProfile, error) {
	if buyerID <= 0 {
		return CheckoutProfile{}, nil
	}
	pref, found, err := u.prefs.GetDefault(ctx, buyerID)
	if err != nil {
		return CheckoutProfile{}, err
	}
	if !found {
		return CheckoutProfile{}, nil
	}
	return toCheckoutProfile(pref), nil
}

func toCheckoutProfile(p domain.BuyerPreference) CheckoutProfile {
	return CheckoutProfile{
		Found:        true,
		ProfileID:    p.ID,
		CarrierType:  p.PreferenceKind,
		DisplayValue: preferenceDisplayValue(p),
		BuyerTaxID:   derefString(p.TaxID),
		BuyerName:    derefString(p.TaxTitle),
		IsDefault:    p.IsDefault,
		Label:        derefString(p.Label),
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func preferenceDisplayValue(p domain.BuyerPreference) string {
	switch p.PreferenceKind {
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		return derefString(p.TaxID)
	case codemeta.InvoiceCarrierTypeDonation.Value:
		return derefString(p.DonationCode)
	default:
		return derefString(p.CarrierValueMask)
	}
}

func (u *Checkout) resolveIdentity(ctx context.Context, buyerID int64, spec CheckoutInvoiceSpec) (checkoutIdentity, checkoutRejection, error) {
	return resolveInvoiceIdentity(ctx, u.prefs, buyerID, spec)
}

func resolveInvoiceIdentity(
	ctx context.Context, prefs domain.BuyerPreferenceRepository, buyerID int64, spec CheckoutInvoiceSpec,
) (checkoutIdentity, checkoutRejection, error) {
	switch {
	case spec.Present && spec.ProfileID > 0:
		return identityFromPreference(ctx, prefs, buyerID, spec.ProfileID)
	case spec.Present && (spec.CarrierType != "" || spec.BuyerTaxID != "" || spec.Value != ""):
		id, rej := identityFromManualInput(spec)
		return id, rej, nil
	default:
		return checkoutIdentity{
			SelectionMode: codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value,
		}, checkoutRejection{}, nil
	}
}

func identityFromPreference(
	ctx context.Context, prefs domain.BuyerPreferenceRepository, buyerID, profileID int64,
) (checkoutIdentity, checkoutRejection, error) {
	if buyerID <= 0 {
		return checkoutIdentity{}, rejectWith(bizcode.InvoiceProfileNotFound), nil
	}
	if prefs == nil {
		return checkoutIdentity{}, rejectWith(bizcode.InvoiceProfileNotFound), nil
	}
	pref, err := prefs.GetByID(ctx, profileID, buyerID)
	if err != nil {
		if isBizCode(err, bizcode.InvoiceProfileNotFound) {
			return checkoutIdentity{}, rejectWith(bizcode.InvoiceProfileNotFound), nil
		}
		return checkoutIdentity{}, checkoutRejection{}, err
	}

	id := checkoutIdentity{
		SelectionMode: codemeta.InvoiceCarrierSelectionModeExplicitPreference.Value,
		PreferenceID:  pref.ID,
		CarrierType:   pref.PreferenceKind,
		DisplayValue:  preferenceDisplayValue(pref),
		BuyerTaxID:    derefString(pref.TaxID),
		BuyerName:     derefString(pref.TaxTitle),
		DonationCode:  derefString(pref.DonationCode),
	}
	switch pref.PreferenceKind {
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		id.SelectionMode = codemeta.InvoiceCarrierSelectionModeNone.Value
		id.PreferenceID = 0
	case codemeta.InvoiceCarrierTypeDonation.Value:
		id.SelectionMode = codemeta.InvoiceCarrierSelectionModeDonation.Value
		id.PreferenceID = 0
	default:
		id.HasCarrier = true
	}
	if code := validateIdentityShape(id); code != 0 {
		return checkoutIdentity{}, rejectWith(code), nil
	}
	return id, checkoutRejection{}, nil
}

const errUnsavedOneOffCarrier = "請先將此載具儲存為常用發票資料後再結帳"

const carrierPresentPlaceholder = "«carrier-present»"

func identityFromManualInput(spec CheckoutInvoiceSpec) (checkoutIdentity, checkoutRejection) {
	kind, ok := codemeta.InvoiceCarrierTypeFromValue(spec.CarrierType)
	if !ok {
		return checkoutIdentity{}, rejectWith(bizcode.InvoiceCarrierInvalid)
	}

	id := checkoutIdentity{CarrierType: kind.Value}
	switch kind.Value {
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		id.SelectionMode = codemeta.InvoiceCarrierSelectionModeNone.Value
		id.BuyerTaxID = spec.BuyerTaxID
		id.BuyerName = spec.BuyerName
		id.DisplayValue = spec.BuyerTaxID
		id.CarrierValue = spec.Value
	case codemeta.InvoiceCarrierTypeDonation.Value:
		id.SelectionMode = codemeta.InvoiceCarrierSelectionModeDonation.Value
		id.DonationCode = spec.Value
		id.DisplayValue = spec.Value
		id.BuyerTaxID = spec.BuyerTaxID
		id.BuyerName = spec.BuyerName
	default:
		id.CarrierValue = spec.Value
		id.CarrierValueVisible = true
		id.HasCarrier = true
		id.DisplayValue = maskCarrierValue(spec.Value)
		if code := validateIdentityShape(id); code != 0 {
			return checkoutIdentity{}, rejectWith(code)
		}
		return checkoutIdentity{}, checkoutRejection{
			Code:    bizcode.InvoiceCarrierInvalid,
			Message: errUnsavedOneOffCarrier,
		}
	}

	if code := validateIdentityShape(id); code != 0 {
		return checkoutIdentity{}, rejectWith(code)
	}
	return id, checkoutRejection{}
}

func validateIdentityShape(id checkoutIdentity) int32 {
	switch id.CarrierType {
	case codemeta.InvoiceCarrierTypeMobileBarcode.Value:
		if !id.CarrierValueVisible {
			return 0
		}
		if !reMobileBarcode.MatchString(id.CarrierValue) {
			return bizcode.InvoiceCarrierInvalid
		}
	case codemeta.InvoiceCarrierTypeCitizenCert.Value:
		if !id.CarrierValueVisible {
			return 0
		}
		if !reCitizenCert.MatchString(id.CarrierValue) {
			return bizcode.InvoiceCarrierInvalid
		}
	case codemeta.InvoiceCarrierTypeDonation.Value:
		if !reDonationCode.MatchString(id.DonationCode) {
			return bizcode.InvoiceCarrierInvalid
		}
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		if id.CarrierValue != "" {
			return bizcode.InvoiceCarrierInvalid
		}
		if !domain.IsBusinessTaxID(id.BuyerTaxID) {
			return bizcode.InvoiceCarrierInvalid
		}
		n := utf8.RuneCountInString(strings.TrimSpace(id.BuyerName))
		if n < 1 || n > buyerNameMaxRunes {
			return bizcode.InvoiceCarrierInvalid
		}
	case "":
		if id.CarrierValue != "" || id.DonationCode != "" || id.HasCarrier {
			return bizcode.InvoiceCarrierInvalid
		}
	default:
		return bizcode.InvoiceCarrierInvalid
	}
	return 0
}

func (u *Checkout) assertBuyerIdentity(amount CheckoutAmount, id checkoutIdentity) int32 {
	draft := forecastDraft(amount, id)
	if draft == nil {
		return 0
	}
	err := draft.Validate()
	if err == nil {
		return 0
	}

	var ie *domain.InvariantError
	if !errors.As(err, &ie) {
		u.logger().Error("invoice.checkout.validate.unexpected_error", zap.Error(err))
		return 0
	}
	if string(ie.Code) == domain.CodeBuyerInfoConflict {
		return bizcode.InvoiceTargetConflict
	}
	u.logger().Warn("invoice.checkout.validate.will_need_manual_review",
		zap.String("manual_reason_code", string(ie.Code)),
		zap.String("detail", ie.Detail))
	return 0
}

func forecastDraft(amount CheckoutAmount, id checkoutIdentity) *domain.IssueDraft {
	snap := domain.OrderSnapshot{
		TotalOriginalAmount: amount.ProductAmount,
		ShippingFee:         amount.ShippingAmount,
		DiscountAmount:      amount.DiscountAmount,
		TotalActualAmount:   amount.InvoiceAmount,
		DiscountFunding:     domain.DiscountFundingSeller,
		Items: []domain.OrderItem{{
			OrderItemID: 1,
			ProductName: "結帳預估",
			Quantity:    1,
			UnitPrice:   amount.ProductAmount,
		}},
	}

	cat := domain.TaxCategory(amount.TaxCategory)
	if _, ok := domain.TaxCategoryFromDB(amount.TaxCategory); !ok {
		cat = domain.TaxCategoryTaxable
	}

	buyerIdentifier := domain.B2CBuyerIdentifier
	if domain.IsBusinessTaxID(id.BuyerTaxID) {
		buyerIdentifier = id.BuyerTaxID
	}

	draft, err := domain.NewIssueDraft(snap, domain.SellerDefaultTaxResolver{Default: cat}, buyerIdentifier)
	if err != nil {
		return nil
	}

	if kind, ok := codemeta.InvoiceCarrierTypeFromValue(id.CarrierType); ok {
		if amegoCarrier, ok := domain.CarrierTypeToAmego(kind); ok {
			draft.CarrierType = amegoCarrier
		}
	}
	draft.CarrierValue = id.CarrierValue
	if draft.CarrierValue == "" && id.HasCarrier {
		draft.CarrierValue = carrierPresentPlaceholder
	}
	draft.DonationCode = id.DonationCode
	return draft
}

type orderInvoiceSnapshot struct {
	OrderNo                  string  `json:"order_no"`
	SnapshotVersion          int32   `json:"snapshot_version"`
	InvoiceKind              string  `json:"invoice_kind"`
	CarrierSelectionMode     string  `json:"carrier_selection_mode"`
	BuyerInvoicePreferenceID *string `json:"buyer_invoice_preference_id"`
	BuyerTaxID               *string `json:"buyer_tax_id"`
	BuyerTitle               *string `json:"buyer_title"`
	DonationCode             *string `json:"donation_code"`
}

func invoiceKindOf(buyerTaxID string) string {
	if domain.IsBusinessTaxID(buyerTaxID) {
		return codemeta.InvoiceDocumentKindB2b.Value
	}
	return codemeta.InvoiceDocumentKindB2c.Value
}

func marshalOrderInvoiceSnapshot(in ValidateInvoiceInput, id checkoutIdentity) ([]byte, error) {
	kind := invoiceKindOf(id.BuyerTaxID)

	version := in.SnapshotVersion
	if version < 1 {
		version = 1
	}

	snap := orderInvoiceSnapshot{
		OrderNo:              in.OrderNo,
		SnapshotVersion:      version,
		InvoiceKind:          kind,
		CarrierSelectionMode: id.SelectionMode,
	}
	if id.PreferenceID > 0 {
		snap.BuyerInvoicePreferenceID = strPtr(fmt.Sprintf("%d", id.PreferenceID))
	}
	if kind == codemeta.InvoiceDocumentKindB2b.Value {
		snap.BuyerTaxID = strPtr(id.BuyerTaxID)
		snap.BuyerTitle = strPtr(truncateRunes(id.BuyerName, buyerNameMaxRunes))
	}
	if id.DonationCode != "" {
		snap.DonationCode = strPtr(id.DonationCode)
	}
	return json.Marshal(snap)
}

func maskCarrierValue(v string) string {
	r := []rune(v)
	switch {
	case len(r) == 0:
		return ""
	case len(r) <= 5:
		return strings.Repeat("*", len(r)-1) + string(r[len(r)-1])
	default:
		return string(r[:4]) + strings.Repeat("*", len(r)-5) + string(r[len(r)-1])
	}
}

func strPtr(s string) *string { return &s }

func (u *Checkout) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}

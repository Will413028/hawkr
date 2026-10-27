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

// ---------------------------------------------------------------------------
// Checkout：結帳鏈的兩支內部 RPC（§6.1.2、§6.2.4 的 order-service ②③）
// ---------------------------------------------------------------------------
//
// 這一層回答結帳鏈的兩個問題，**兩個都不寫任何一列資料**：
//
//	preview  → ResolveMode      「這位賣家對這一筆會走哪個開票模式？金額怎麼拆？要不要告知？」
//	confirm  → ValidateInput    「買家填的發票資料合不合法？賣家還接不接得了單？快照長什麼樣？」
//
// 🔴 **履約模式的判定只有一份**：ResolveMode 與 order.paid 的 Prepare 走同一支
// domain.ResolveFulfillmentMode。兩份實作漂移的症狀是「preview 說會開 B2B、實際開出 B2C」，
// 而**沒有任何測試會紅** —— 兩邊各自的測試都會通過自己那一份。守住這件事的是
// checkout_consistency_test.go 的等值斷言，不是本註解。

// checkoutPreviewTTL 是 preview 結果的有效期。
//
// 🔴 這個值的真正擁有者是**結帳鎖**（cart-service 的 `CartService.LOCK_TTL_MINUTES = 2`），
// 不是本服務：§6.1.2 明文「前端自算 TTL 會與 checkout lock 過期時間漂移」，而同一個理由
// 也適用於「invoice-service 自己挑一個不一樣的數字」。目前兩邊沒有共用的 SSOT，
// 本常數刻意與 LOCK_TTL_MINUTES 同值並在此指名來源；那一邊改了、這一邊沒跟，
// 症狀是買家看到「請重新計算」而金額其實還有效（或反過來，更糟）。
const checkoutPreviewTTL = 2 * time.Minute

// invoiceDisclosureTermsVersion 是法定告知的條款版本。
//
// confirm 會拿買家回填的 disclosureVersion 與本值比對（不符 ⇒ 9314），所以它是一個
// **對外契約**：改動它會讓所有停在結帳頁、preview 已取得舊版本的買家在按下付款時被擋，
// 必須當成一次前端可見的變更來安排，不是順手改字串。
const invoiceDisclosureTermsVersion = "2026-08-30"

// 法定告知的標題與內文。
//
// 兩段而不是一段：`PLATFORM_RECEIPT`（免用統一發票）與 `SELLER_SELF_ISSUE`／`LEGACY_MANUAL`
// （賣家自行開立）要告知的是不同的事實，共用一段文案會讓其中一種買家讀到不成立的敘述。
const (
	disclosureReceiptTitle = "本筆交易由賣家開立收據"
	disclosureReceiptBody  = "本賣家經國稅局核定免用統一發票，本筆交易將由平台代製收據，不會開立電子發票。" +
		"收據可於「我的發票」查看與下載。"

	disclosureSelfIssueTitle = "本筆交易由賣家自行開立發票"
	disclosureSelfIssueBody  = "本賣家的稅務資格尚未完成平台代開設定，本筆交易的統一發票由賣家自行開立並交付，" +
		"平台不會代為開立。如未收到發票，請直接與賣家聯繫。"
)

// 載具與買受人欄位的形狀規則（§6.1.1 逐欄驗證表）。
//
// ⚠️ 這一組是本 repo 內**唯一**的實作，不是第二份：全服務 grep 不到任何其他載具格式檢查
// （profile CRUD 端點尚未落地）。CRUD 端點落地時必須共用這裡，不得各寫一份 —— 兩份規則
// 漂移的症狀是「常用資料存得進去，結帳時被擋」。
var (
	reMobileBarcode = regexp.MustCompile(`^/[0-9A-Z+\-.]{7}$`)
	reCitizenCert   = regexp.MustCompile(`^[A-Z]{2}[0-9]{14}$`)
	reDonationCode  = regexp.MustCompile(`^[0-9]{3,7}$`)
)

// buyerNameMaxRunes 是 AMEGO BuyerName 的上限，同時是 order_invoice_snapshots.buyer_title
// 的 varchar(60)。
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

// ---------------------------------------------------------------------------
// 值型別
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// ResolveMode（checkout preview）
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// ValidateInput（checkout confirm）
// ---------------------------------------------------------------------------

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
	// buyerHasTaxID 傳 false：它在 §7.1.3 只決定 document_kind（E 節點 B2B／B2C），
	// **不影響 fulfillment_mode**，而本 RPC 只回 invoice_mode。決策必須在解析買家身分
	// **之前**算出來，因為每一個拒絕分支都要帶著 mode 回去（前端據此決定顯不顯示法定告知）。
	decision := domain.ResolveFulfillmentMode(fulfillmentInputFor(facts, amount.InvoiceAmount, now, false))
	mode := decision.FulfillmentMode
	limit := intakeLimitOf(facts.profile)

	reject := func(r checkoutRejection) (ValidateInvoiceOutput, error) {
		// 🔴 非 0 業務碼一律帶著 invoice_mode 與 intake_limit 回去：§6.1.2 對 9326 明文要求
		// `data.intakeLimit` 帶現況（HTTP 200 ＋ preserveData），而其餘幾碼前端也要靠 mode
		// 決定要不要顯示法定告知。回 code≠0 ＋ 空 data 會讓前端只剩下 parse message 一條路。
		return ValidateInvoiceOutput{
			Valid:       false,
			BizCode:     r.Code,
			Message:     r.Message,
			InvoiceMode: mode,
			IntakeLimit: limit,
		}, nil
	}

	// 1. 法定告知 —— **不再是擋門**（DEC-20260831-210934）。
	//
	// 原本這裡檢查「required 且已勾選且版本相符」，不過就回 9314。裁決把它改成
	// 只顯示不強制：preview 仍回 disclosure.title／body 讓前端顯示，但 confirm
	// 不檢查任何回填值（那兩個欄位已從 proto 移除、欄號 reserved）。
	//
	// 裁決依據：58 號 L1–L13 法規表查無「付款前必須告知」的法源，命名裡的「法定」
	// 從未被證實；競品實查（蝦皮以「結帳頁有無電子發票欄位」為訊號、momo 寫在規則頁）
	// 兩者皆無結帳時的強制確認。
	//
	// 🔴 disclosureFor 保留且仍被 preview 呼叫——**它決定要不要顯示、顯示哪一段文案**，
	// 那個職責沒有消失。刪掉它會讓免用統一發票的買家完全不知道自己拿到的是收據。

	// 2＋3. 解析買家發票身分（含歸屬驗證與逐欄格式）。
	identity, rej, err := u.resolveIdentity(ctx, in.BuyerID, in.Invoice)
	if err != nil {
		return ValidateInvoiceOutput{}, err
	}
	if !rej.ok() {
		return reject(rej)
	}

	// 4. 三者互斥：走開立側的同一條斷言（A15），不另寫一套。
	if code := u.assertBuyerIdentity(amount, identity); code != 0 {
		return reject(rejectWith(code))
	}

	// 5. 待審接單上限。放在最後：它是賣家側的競態，越接近寫入越準；
	//    前面幾格是買家自己改得掉的輸入錯誤，先告訴他那些比較有用。
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

// ---------------------------------------------------------------------------
// 賣家事實
// ---------------------------------------------------------------------------

// checkoutSellerFacts 是決策樹需要的三份賣家事實。
type checkoutSellerFacts struct {
	issuer  domain.Issuer
	profile domain.SellerTaxProfile
	account domain.ProviderAccount
}

// sellerFacts 讀 issuer／稅務資格／provider 帳戶。
//
// 🔴 deny-by-default，與 Prepare 逐字相同：「查不到」不是錯誤，是決策樹的一個輸入值
// （空字串 ⇒ default 分支 ⇒ LEGACY_MANUAL ＋ TAX_PROFILE_NOT_APPROVED）。
// 把它當錯誤往上拋，結帳頁會在「這位賣家還沒送審」時直接 500 —— 而那是完全正常的狀態。
//
// ⚠️ issuer 查不到同樣不是錯誤：賣家從未進過發票流程時本來就沒有 invoice_issuers 列。
// 這一格與 Prepare 不同（那裡 issuer 是必然存在的，因為 order.paid 之前賣家已經開過店），
// 差異寫在這裡是刻意的：preview 會在賣家生命週期更早的時點被呼叫。
func (u *Checkout) sellerFacts(ctx context.Context, storeID int64) (checkoutSellerFacts, error) {
	var out checkoutSellerFacts
	if storeID <= 0 {
		return out, fmt.Errorf("invoice: checkout requires a store_id (got %d)", storeID)
	}

	issuer, err := u.issuers.GetByRef(ctx, codemeta.InvoiceIssuerTypeSeller.Value, storeID)
	if err != nil {
		// ⚠️ IssuerRepository.GetByRef 的「查無此列」回的是 `InvoiceProviderAccountNotFound`
		// （9330）而不是一個 issuer 專屬的碼 —— 那是既有實作的現況（seller_repo.go 的
		// GetByRef），不是這裡挑的。碼本身沒有 issuer 語意，所以這一格只能靠它，
		// 而不能靠「碼名看起來對不對」。
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

// fulfillmentInputFor 把賣家事實與金額折成決策樹的輸入。
//
// 🔴 本函式是 ResolveMode 與 ValidateInput 共用的唯一一份組裝：兩支 RPC 若各自組
// FulfillmentInput，preview 與 confirm 就會對同一筆訂單說出不同的 invoice_mode，
// 而買家在 confirm 那一刻已經看不到 preview 的畫面了。
func fulfillmentInputFor(f checkoutSellerFacts, invoiceAmount int64, now time.Time, buyerHasTaxID bool) domain.FulfillmentInput {
	return domain.FulfillmentInput{
		TotalActualAmount:   invoiceAmount,
		SellerReviewStatus:  f.profile.ReviewStatus,
		ProviderStatus:      f.account.Status,
		AutomationCutoverAt: f.account.AutomationCutoverAt,
		InvoiceDate:         now,
		BuyerHasTaxID:       buyerHasTaxID,
		// 🔴 這一欄**必須跟 Prepare 傳同一個值**：preview 與實際落庫用同一棵樹，
		// 兩邊輸入不同就會出現「結帳時說會代製收據、付款後卻落 AWAITING_MANUAL」，
		// 而兩句話都是後端講的，買家無從判斷（同 buildCheckoutAmount 的 doc comment）。
		SellerReceiptAuthorized: f.profile.ReceiptAuthorizedAt != nil,
		// fail-closed 三格恆 false，理由見 ResolveMode 的 doc comment ②。
	}
}

// buildCheckoutAmount 算出發票金額拆解。
//
// InvoiceAmount 夾在 0 以上，與 domain.OrderSnapshot.TotalActualAmount 的定義
// （max(0, S + F − D)）一致 —— 兩邊用不同公式，preview 說「不開票」而實際開了（或反過來）
// 就會發生，而兩者都不會報錯。
func buildCheckoutAmount(product, shipping, discount int64, profile domain.SellerTaxProfile) CheckoutAmount {
	total := product + shipping - discount
	if total < 0 {
		total = 0
	}
	cat := profile.DefaultTaxCategory
	if cat == 0 {
		// DB DEFAULT 的語意是應稅，不是「未知」。與 taxResolverFor 的處理一致。
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
		// 兩個上限是 OR，不是 AND（§6.1.2 C5 第 2 條）。
		Blocked: p.PendingOrderCount >= p.PendingOrderLimitCount ||
			p.PendingOrderAmount >= p.PendingOrderLimitAmount,
	}
}

// disclosureFor 決定要不要顯示法定告知，以及顯示哪一段。
//
// 🔴 preview 與 confirm 共用本函式：confirm 拿買家回填的版本與 preview 給的版本比對，
// 兩邊各算一次「要不要告知」就會出現「preview 沒要求、confirm 卻擋下來」。
//
// AMEGO_AUTO 以外的三種模式都要告知，理由不同但都是「平台不會代開這張發票」：
// PLATFORM_RECEIPT 是免用統一發票的法定告知，另外兩種是自行開票提示（§6.1.2 B01 的
// `invalid` 那一列明文「只有這一個觸發點」）。
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
		// SELLER_SELF_ISSUE / LEGACY_MANUAL。
		return CheckoutDisclosure{
			Required:     true,
			TermsVersion: invoiceDisclosureTermsVersion,
			Title:        disclosureSelfIssueTitle,
			Body:         disclosureSelfIssueBody,
		}
	}
}

// ---------------------------------------------------------------------------
// 買家發票身分
// ---------------------------------------------------------------------------

// checkoutRejection 是一個業務層拒絕：碼 ＋ 給前端的訊息。
//
// Message 預設取 CodeMeta 的碼表訊息；只有在同一個碼承載了兩種不同處置時才覆寫
// （目前唯一一處：一次性手動載具，見 identityFromManualInput）。覆寫是**例外不是慣例**——
// 前端不解析 message（§6.1.2 對 9314／9305 分碼的理由），訊息只給人看。
type checkoutRejection struct {
	Code    int32
	Message string
}

// ok 回報這是不是「沒有拒絕」。
func (r checkoutRejection) ok() bool { return r.Code == 0 }

func rejectWith(code int32) checkoutRejection {
	return checkoutRejection{Code: code, Message: bizcode.Message(code)}
}

// checkoutIdentity 是解析後的買家發票身分。
type checkoutIdentity struct {
	// SelectionMode ＝ CodeMeta InvoiceCarrierSelectionMode 的 Value，
	// 也是 order_invoice_snapshots.carrier_selection_mode 要落的值。
	SelectionMode string
	// PreferenceID 只在 EXPLICIT_PREFERENCE 時非 0。
	PreferenceID int64
	// CarrierType ＝ CodeMeta InvoiceCarrierType 的 Value（空字串 ＝ 無載具）。
	CarrierType  string
	CarrierValue string
	// DisplayValue 恆為遮罩後的值。明碼只活在本 struct 內、不出服務邊界。
	DisplayValue string
	BuyerTaxID   string
	BuyerName    string
	DonationCode string

	// HasCarrier ＝ 這個身分帶了一個載具（手機條碼／自然人憑證）。
	//
	// 🔴 它與 `CarrierValue != ""` **不等價**，這是本檔最容易寫錯的一處：
	// 來自常用發票資料的載具，值是密文、本層刻意不解密（顯示用遮罩就夠了）⇒
	// CarrierValue 是空的，而買家**確實選了載具**。A15 問的是「選了幾種」不是「值是什麼」，
	// 拿空字串去數，統編＋載具的組合會被判成「只選了統編」而放行。
	HasCarrier bool

	// CarrierValueVisible ＝ CarrierValue 裡是明文（只有本次手動輸入才會是 true）。
	// 逐欄格式驗證只能對看得見的值做；看不見的那些在寫入端點（§6.1.1 端點 2／3）驗過。
	CarrierValueVisible bool
}

func (p CheckoutProfile) hasTaxID() bool {
	return p.Found && domain.IsBusinessTaxID(p.BuyerTaxID)
}

// defaultProfile 取買家的預設常用發票資料（preview 的 defaultProfile 欄）。
//
// 沒有預設是**正常分支**（§6.1.2(b) 第 ② 步：無載具發票），回 Found=false，
// 對外是空 message 而不是錯誤。
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

// preferenceDisplayValue 取一列常用發票資料的遮罩顯示值。
//
// 🔴 載具型一律用已落庫的 carrier_value_mask，**不解密**：顯示不需要明碼，而每一次解密
// 都是一次明碼可能外洩的機會（§6.1.1 遮罩段）。統編與捐贈碼本身不是 PII
// （§5.4.3 表註解逐字：「公開營業登記資訊」「公告代碼」）⇒ 直接顯示。
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

// resolveIdentity 解析買家這一次的發票身分，並做逐欄格式驗證。
//
// 第二個回傳值是業務碼：0 ＝ 通過。error 只留給真正的故障（DB 掛了），
// 業務結果不走 error —— 兩者混在一起，呼叫端就分不出「該回 409」與「該重試」。
//
// **缺省鏈只有兩步**（§6.1.2(b) 的紅字方框、D6／D7／ADR-22）：
// ① 買家有存活的 is_default profile → 用它 ② 否則 → 無載具發票。
// 沒有第三步；`InvoiceCarrierType` 也沒有能承載光貿會員載具的第 5 個值。
func (u *Checkout) resolveIdentity(ctx context.Context, buyerID int64, spec CheckoutInvoiceSpec) (checkoutIdentity, checkoutRejection, error) {
	return resolveInvoiceIdentity(ctx, u.prefs, buyerID, spec)
}

// resolveInvoiceIdentity 是「買家的發票身分」的**唯一**一份推導。
//
// 🔴 它從 Checkout 的方法抽出來，是因為 R03（付款後修改、開立前直接改）也需要它
// ——而在 2026-09-04 之前 correction_intake 根本沒做這件事：ReviseSnapshotInput
// 的 7 個業務欄位只填了 3 個，invoice_kind／carrier_selection_mode／
// buyer_invoice_preference_id／donation_code 四個從未賦值 ⇒ order-service 驗
// invoice_kind 必填而回 400，R03 從上線起就沒有成功過一次（#3085）。
//
// ⚠️ **不要在 correction 那邊複製一份**：兩邊各自算同一件事必然漂移，而症狀是
// 「更正之後買家原本選的載具消失了」——沒有任何東西會報錯。
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
		// 整包省略、或帶了一個什麼都沒填的 invoice ⇒ 缺省鏈。
		//
		// ⚠️ 這裡**不解析**買家的預設載具去填快照：§5.4.3 明文
		// 「`USE_BUYER_DEFAULT` 由 invoice-service 在建立文件時解析，不在結帳同步呼叫」。
		// 快照只記下模式，真正的解析在 order.paid（client/order/carrier.go）——
		// 在這裡解析並落進快照，等於把「下單當下的預設」凍結成事實，而買家在付款前
		// 改了預設時，兩邊會指向不同的載具且沒有任何東西會報錯。
		return checkoutIdentity{
			SelectionMode: codemeta.InvoiceCarrierSelectionModeUseBuyerDefault.Value,
		}, checkoutRejection{}, nil
	}
}

// identityFromPreference 走「指定既有常用發票資料」。
//
// 🔴 buyerID 是查詢述詞的一部分（寫進 SQL 的 WHERE，repository 保證），不是查回來再比對：
// 「這筆不存在」與「這筆不屬於你」刻意不可分辨，同碼（9302）、同訊息、同一行程式碼
// （§8.1.4(b)）。這裡若補一個 `if pref.BuyerID != buyerID` 分支，Tier A 當場失效。
func identityFromPreference(
	ctx context.Context, prefs domain.BuyerPreferenceRepository, buyerID, profileID int64,
) (checkoutIdentity, checkoutRejection, error) {
	if buyerID <= 0 {
		// 沒有買家鍵就驗不了歸屬。這裡**不得**退化成「不驗」（I-47）。
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
	// carrier_selection_mode 由 preference_kind 決定，不是「指定了 profile 就是
	// EXPLICIT_PREFERENCE」：chk_ois_b2b 要求 B2B 恆為 NONE、chk_ois_donation_pair 是
	// 雙向等式 ⇒ 統編型與捐贈型的 profile 各自落 NONE／DONATION，而
	// buyer_invoice_preference_id 只跟著真正的載具走（chk_ois_exclusive 三選一）。
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

// errUnsavedOneOffCarrier 是「一次性手動載具無處可存」的訊息。
//
// 🔴 這不是格式錯，但只能借 9305 回報 —— 理由與代價寫在 identityFromManualInput。
const errUnsavedOneOffCarrier = "請先將此載具儲存為常用發票資料後再結帳"

// carrierPresentPlaceholder 是「有載具但本層看不到值」的佔位。
//
// 只出現在 forecastDraft 組出的預測草稿裡，**永遠不會被送去開票**（開票路徑的載具值
// 來自 client/order/carrier.go 的解密）。刻意用一個明顯不像載具號碼的字串：
// 萬一它真的漏進了哪個送出路徑，那是一眼看得出來的錯，而不是一張開錯載具的發票。
const carrierPresentPlaceholder = "«carrier-present»"

// identityFromManualInput 走「本次手動輸入」。
//
// 🔴 **只有統編與捐贈碼可以純手動**。手機條碼／自然人憑證條碼在 Phase 1 必須先存成
// 常用發票資料（§6.1.1 端點 2）再以 `profileId` 帶進來，理由是快照存不下它：
//
//	§5.4.3 的 order_invoice_snapshots 是**零 PII** 表，載具值一律留在 invoice-service，
//	它只有 buyer_invoice_preference_id 一個指標欄；而 carrier_selection_mode 的值域
//	（chk_ois_mode）只有 USE_BUYER_DEFAULT / EXPLICIT_PREFERENCE / NONE / DONATION 四值，
//	**沒有一個代表「這一單自己填的一次性載具」**。
//
// 於是純手動載具只有三條路，兩條是錯的：
//   - 落 USE_BUYER_DEFAULT ⇒ 開票時解析成買家的預設（或無載具）⇒ **買家填的載具被靜默吞掉**，
//     而發票不可回收、載具歸戶落空。
//   - 落 EXPLICIT_PREFERENCE ＋ 空 id ⇒ chk_ois_pref_pair 是雙向等式 ⇒ order-service
//     那一側 INSERT 直接被 DB 擋掉 ⇒ 結帳 500。
//   - **擋在結帳**（本實作）：買家看得到、改得掉，而且還沒付錢。
//
// 這是設計書 §6.1.2(b)（允許手動輸入）與 §5.4.3（快照沒有欄位承載它）的矛盾，
// 已列進交付回報；本波取 fail-closed 的那一邊。
func identityFromManualInput(spec CheckoutInvoiceSpec) (checkoutIdentity, checkoutRejection) {
	kind, ok := codemeta.InvoiceCarrierTypeFromValue(spec.CarrierType)
	if !ok {
		// 值域外的 carrierType（含「前端為『不使用載具』自造了一個 wire 值」這個
		// §6.1.2(b) 明令禁止的形狀）⇒ 格式錯，不猜。
		//
		// ⚠️ 只帶 buyerTaxId 而不帶 carrierType 也走這裡：統編必須明說 carrierType=TAX_ID，
		// 由欄位有沒有值去推語意，就是在憑證上猜買受人身分。
		return checkoutIdentity{}, rejectWith(bizcode.InvoiceCarrierInvalid)
	}

	id := checkoutIdentity{CarrierType: kind.Value}
	switch kind.Value {
	case codemeta.InvoiceCarrierTypeTaxId.Value:
		id.SelectionMode = codemeta.InvoiceCarrierSelectionModeNone.Value
		id.BuyerTaxID = spec.BuyerTaxID
		id.BuyerName = spec.BuyerName
		id.DisplayValue = spec.BuyerTaxID
		// TAX_ID 的 value 必須為空（§6.1.1）：帶了值代表前端把載具塞進了統編那一格。
		id.CarrierValue = spec.Value
	case codemeta.InvoiceCarrierTypeDonation.Value:
		id.SelectionMode = codemeta.InvoiceCarrierSelectionModeDonation.Value
		id.DonationCode = spec.Value
		id.DisplayValue = spec.Value
		// 捐贈同時帶統編 ⇒ 三選一違規，交給 A15 判（9306），不在這裡當成格式錯。
		id.BuyerTaxID = spec.BuyerTaxID
		id.BuyerName = spec.BuyerName
	default: // MOBILE_BARCODE / CITIZEN_CERT
		id.CarrierValue = spec.Value
		id.CarrierValueVisible = true
		id.HasCarrier = true
		id.DisplayValue = maskCarrierValue(spec.Value)
		// 先驗形狀再擋：形狀本來就錯的值，回「請先存成常用資料」只會讓買家存一份錯的。
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

// validateIdentityShape 是 §6.1.1 逐欄驗證表的唯一實作。回 0 表示通過。
func validateIdentityShape(id checkoutIdentity) int32 {
	switch id.CarrierType {
	case codemeta.InvoiceCarrierTypeMobileBarcode.Value:
		if !id.CarrierValueVisible {
			// 來自常用發票資料：值是密文、本層不解密 ⇒ 沒有東西可比對正則。
			// 這裡**不是**放行一個沒驗過的值：寫入端點（§6.1.1 端點 2／3）驗過同一組正則，
			// 而 chk_bip_shape 保證這種 kind 的密文非空。
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
		// value 必須為空；統編走 domain.IsBusinessTaxID（8 碼數字，**不驗檢查碼**）；
		// 抬頭 1–60 字。
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
		// 無載具：不得夾帶任何載具值。
		if id.CarrierValue != "" || id.DonationCode != "" || id.HasCarrier {
			return bizcode.InvoiceCarrierInvalid
		}
	default:
		return bizcode.InvoiceCarrierInvalid
	}
	return 0
}

// assertBuyerIdentity 用開立側的同一條斷言驗「統編／載具／捐贈碼三選一」。
//
// 🔴 **不另寫一套互斥判斷**：A15 已經在 domain.IssueDraft.Validate 內，而 AMEGO 三種違規
// 組合實測全部通過（§6.1.1「9306 必須由我方強制」）⇒ 這是唯一的攔截點，兩份實作漂移
// 等於漏掉其中一種組合，而憑證已經開出去了。
//
// **為什麼只有 A15 會擋下結帳，其餘斷言不會**：§7.1.3 對 A1–A14、A20–A23 失敗的處置是
// `AWAITING_MANUAL`（轉人工），**不是拒絕成單**。把它們在這裡升級成結帳阻擋，會讓
// 例如零稅率賣家（A10）的買家完全結不了帳，而設計書的 confirm 失敗清單裡沒有這一條、
// CodeMeta 也沒有對應的碼。所以其餘斷言在這裡只留一行 WARN：它預告了這張單會落進
// ADM 的人工佇列，而真正的 fail-closed 判定仍在 order.paid 那一次（不被本函式繞過）。
func (u *Checkout) assertBuyerIdentity(amount CheckoutAmount, id checkoutIdentity) int32 {
	draft := forecastDraft(amount, id)
	if draft == nil {
		// 零元結帳：沒有可送出的行，A15 沒有東西可斷言。零元訂單本來就不開票
		// （§7.1.3 的 NA1），互斥與否不影響任何憑證。
		return 0
	}
	err := draft.Validate()
	if err == nil {
		return 0
	}

	var ie *domain.InvariantError
	if !errors.As(err, &ie) {
		// Validate 只回 *InvariantError；真的出現別種，代表 domain 換了契約而這裡沒跟上。
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

// forecastDraft 組出「若現在就開票會長什麼樣」的草稿，供 A15 斷言。
//
// 🔴 它是**預測**不是事實，三個刻意的簡化都寫在這裡，不要當成開票路徑的替身：
//
//	① 品項合成一行（結帳 preview／confirm 的契約只帶金額拆解，沒有逐項明細）。
//	   影響：A12／A13 恆過、A21 恆成立。真正的逐行斷言在 order.paid。
//	② DiscountFunding 填 Seller。平台出資券的判定資料（券的 created_by）結帳當下拿不到，
//	   而 §7.1.3 的 FC 三格是在 order.paid 判的 —— 在這裡填 Unknown 只會讓每一筆用了券的
//	   結帳都印一行 WARN，不會多攔到任何東西。**那道 fail-closed 檢查沒有被繞過**。
//	③ 稅別用賣家預設值（Phase 1 整張訂單同一個課稅別，與 taxResolverFor 同一個前提）。
//
// nil 回傳表示零元（domain.ErrZeroAmount），呼叫端當成「沒有東西可斷言」。
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
		// ErrZeroAmount 是正常分支；其餘（負數金額）同樣沒有可斷言的草稿，
		// 金額本身的合法性由 order-service 的既有結帳驗證負責。
		return nil
	}

	if kind, ok := codemeta.InvoiceCarrierTypeFromValue(id.CarrierType); ok {
		if amegoCarrier, ok := domain.CarrierTypeToAmego(kind); ok {
			draft.CarrierType = amegoCarrier
		}
	}
	draft.CarrierValue = id.CarrierValue
	if draft.CarrierValue == "" && id.HasCarrier {
		// A15 只問「選了幾種買受人識別」，不看載具值本身。來自常用發票資料的載具是密文，
		// 本層不解密 ⇒ 用一個非空佔位讓 A15 數得到它。少了這一行，「統編 ＋ 已存的載具」
		// 會被判成只選了統編而放行，而 AMEGO 對那種組合實測照收。
		draft.CarrierValue = carrierPresentPlaceholder
	}
	draft.DonationCode = id.DonationCode
	return draft
}

// ---------------------------------------------------------------------------
// 快照
// ---------------------------------------------------------------------------

// orderInvoiceSnapshot 是 order-service 要寫進 `order_invoice_snapshots` 的那一列
// （§5.4.3 的 V12 DDL），逐欄對應。
//
// 🔴 **零 PII**：沒有載具值、沒有買家 email、沒有收件資訊。表註解逐字寫著
// 「零 PII，密文一律留在 invoice-service」—— 多塞一欄進來，那句話就變成假的，
// 而 order-service 那邊不會有任何東西擋住它。
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

// invoiceKindOf 由買受人統編有無決定 B2C／B2B。
//
// 🔴 判準是**買受人統編**不是別的：proto 的 invoice_kind 值域只有這兩個
// （沒有 RECEIPT——收據是 document_kind 不是 invoice_kind）。
// 這一份是唯一實作，結帳與 R03 兩條路共用（#3085：R03 原本一個字都沒填）。
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
		// chk_ois_version 要求 >= 1。呼叫端沒帶時取 1（第一版），**不是**本服務在遞增
		// snapshot_version —— I-50 明訂唯一寫入者是 order-service。
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

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

// maskCarrierValue 依 §5.1.4 carrier_value_mask 的形狀（例：`/ABC***9`）遮罩。
//
// 保留前 4 碼與末 1 碼，中間一律 `*`。太短的值只留末 1 碼 —— 遮罩的用途是讓買家認出
// 「是不是我那一支」，不是讓人拼回原值。
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

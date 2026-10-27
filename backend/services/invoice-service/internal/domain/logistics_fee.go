package domain

import (
	"errors"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// 本檔是「平台向賣家收物流費」這一軸的開票規則（W5-2／W5-3／W5-5）。
//
// 🔴 它與同一個 package 裡的買家發票軸（invoice_lines.go／amount.go）是**兩條不同的軸**，
// 刻意放在同一個 package 而不是自己的子 package：
//
//	① 兩軸唯一會被混淆的數字（買家運費 vs 賣家物流費）因此在同一個命名空間裡並列，
//	   誰是誰一眼看得到，而不是躲在兩個 import 後面；
//	② internal/domain 的純度 gate（purity_test.go）的掃描目錄只有 {".", "fsm"}
//	   —— 開一個新子 package 會直接落在那道 gate 的觀測窗外，而它不會有任何訊號。
//
// 三條對外保證，全部各自有測試與變異：
//
//	W5-2  開票基礎是 Seller Logistics Charge，**不是** Buyer Shipping Charge
//	W5-3  補收／退還 ⇒ Document Required；會計形式未定案 ⇒ MANUAL_DOCUMENT_REQUIRED（真的擋住）
//	W5-5  買家退款 **不會** 取消賣家物流費；要減免只能走責任歸屬
//
// ============================================================================
// 🔴 交付狀態：本檔目前**零呼叫端**，而那是刻意的（不是忘了接）
// ============================================================================
//
// 一句話自檢（「真實使用者從哪個畫面、按哪顆按鈕會用到它」）今天**答不出來**，
// 所以這裡逐字寫下缺口，而不是讓下一個人自己去發現：
//
//	缺口 1（外部依賴，owner=會計師）：平台對賣家收物流服務費的正式會計／稅務科目
//	  尚未確認。設計書三處逐字禁止自動開立 —— 70 號 §EV-A1、58 號 §355、58 號 §10810。
//	  本檔的 LogisticsAccountingFormDecided() 恆為 false 就是那條禁令的程式化。
//	  ⚠️ 因此「接上自動開票」不是待辦，是**現在不准做的事**。
//
//	缺口 2（契約衝突，需先裁決）：觸發時點兩份設計書不一致 ——
//	  58 號的開票矩陣寫「送達完成（ADR-003）」，70 號 §7 的事件表寫
//	  `logistics.receivable.settled`（＝收款完成之後）。依 CLAUDE.md
//	  「權威來源互相矛盾 ⇒ 先指出矛盾並停止會造成契約漂移的修改」，這裡停手。
//
//	缺口 3（同一波的四面登記）：70 號 §7 明訂的三條 routing key
//	  （logistics.cost.incurred／logistics.receivable.settled／
//	  logistics.receivable.unconfirmed）今天在 topology.yaml、pkg/mq、MqConstants
//	  **三處都是零登記**，而 70 號 §12 逐字要求它們「同一個 PR 內一次做完」。
//
// ⇒ 缺口 1 解除之後，接線形狀已經查清楚、不需要新的 usecase：
//    invoice 側的 usecase.AdminIssuance.Create（internal/usecase/admin_issuance.go）
//    的 allowedIssuanceCombos 已經含 {PLATFORM, SELLER, manual_lines}，
//    而 LogisticsFeeInvoiceDraft 的欄位與 ManualIssuanceInput 一一對得上 ——
//    缺的只是一個 adapter 與一個生產端。
//    進度與裁決追在 #3384

// ============================================================================
// W5-2 開票基礎
// ============================================================================

// LogisticsFeeDescription 是 Platform → Seller 物流費發票的固定品名。
//
// 刻意不叫「運費」（ShippingDescription）：那四個字在本平台已經是買家那一側的品名，
// 兩張發票上出現同樣的字會讓對帳的人分不出手上這張是哪一軸。
const LogisticsFeeDescription = "物流服務費"

var (
	// ErrLogisticsCostNegative 等三條是本軸的輸入不變式。
	//
	// 一律 fail-loud 而不是夾到 0：夾到 0 會讓一筆算錯的補貼永遠不被發現
	// （與 payment-service LogisticsAmounts.accrualAmount 同一條理由）。
	ErrLogisticsCostNegative    = errors.New("invoice/logistics: estimatedCost 不得為負")
	ErrLogisticsSubsidyNegative = errors.New("invoice/logistics: platformSubsidy 不得為負")
	ErrLogisticsChargeNegative  = errors.New("invoice/logistics: 本期認列為負——補貼或責任調整超過成本")

	// ErrLogisticsChargeNotPositive 表示這一期算出來是 0 元。
	//
	// 0 元不是「一張 0 元的發票」而是「不該存在的發票」（與 admin_issuance 的
	// ErrIssuanceTotalNotPositive 同一條判準）。呼叫端據此不建文件。
	ErrLogisticsChargeNotPositive = errors.New("invoice/logistics: 本期認列為 0，不開立")

	// ErrLogisticsRecipientTaxIDRequired：Platform → Seller 是 B2B，收受人統編必填。
	//
	// 缺它就會走到 IsB2B == false 的分支 ⇒ 表頭 TaxAmount 帶 0 ⇒ 平台開給賣家的
	// 發票變成二聯式，賣家拿不到可扣抵的進項憑證，而 AMEGO 不會拒絕這張票。
	ErrLogisticsRecipientTaxIDRequired = errors.New("invoice/logistics: Platform→Seller 發票的收受人統編必填")

	// ErrLogisticsBuyerRefundNegative 是 W5-5 的輸入守衛。
	ErrLogisticsBuyerRefundNegative = errors.New("invoice/logistics: 買家退款金額不得為負")
)

// LogisticsFeeBasis 是「平台這一期要向賣家開多少物流費發票」的完整輸入。
//
// 🔴 **四個欄位裡只有前三個參與金額**。第四個 BuyerShippingCharge 放進同一個 struct
// 是刻意的設計，不是順手帶的：它是本軸上唯一一個會被誤用的數字（名字裡有 shipping、
// 就在隔壁的訂單快照裡、而且多數情況下數值接近），把它擺進視野並在型別註解上標明
// 「不參與」，比讓它留在呼叫端的區域變數裡安全。
//
// 兩者為什麼不相等（CodeMeta LedgerAccountCode 已把這件事訂成 SSOT）：
//   - 買家付的運費 → PLATFORM_LOGISTICS_RECOVERY（平台物流費回收）
//   - 平台向賣家收的物流服務費 → PLATFORM_LOGISTICS_SERVICE_INCOME
//     該碼的描述逐字寫著「🔴 不等於買家付的運費……ADR-006 三條線互不推導」。
//
// 最刺眼的一組值：賣家做免運活動 ⇒ BuyerShippingCharge = 0，而平台照樣付了外送費
// ⇒ EstimatedCost > 0。拿買家運費當基礎會開出一張 0 元發票（或根本不開），
// 平台實付的物流成本就永遠收不回來，且帳上不會有任何東西叫。
type LogisticsFeeBasis struct {
	// EstimatedCost 是本期物流通道的預估成本（含稅、元）。
	EstimatedCost int64
	// PlatformSubsidy 是平台補貼掉、不向賣家收的部分。
	PlatformSubsidy int64
	// ResponsibilityAdjustment 是依 ADR-004 責任歸屬產生的調整，**可正可負**。
	// 這是唯一能讓賣家少付的合法途徑（見 ApplyBuyerRefund 的說明）。
	ResponsibilityAdjustment int64

	// BuyerShippingCharge 是買家在訂單上實際被收的運費。
	//
	// 🔴 **它永遠不參與本軸任何金額計算**，只供對帳與稽核呈現。
	// 任何讀到這個欄位來決定發票金額的程式碼都是缺陷。
	BuyerShippingCharge int64
}

// SellerLogisticsCharge 是本期要向賣家認列的物流費（＝開票基礎）。
//
//	charge = EstimatedCost − PlatformSubsidy + ResponsibilityAdjustment
//
// 🔴 與 payment-service 的 LogisticsAmounts.accrualAmount 是**同一條算式**，
// 兩邊各自實作是因為跨語言無法共用；任何一邊改動都必須同時改另一邊。
// 🔴 **不含 carried_residual**：結轉是前期已認列、尚未收足的餘額，開票再認列一次
// 就是重複開立收入憑證。residual 屬於「收款」不屬於「開票」。
func SellerLogisticsCharge(b LogisticsFeeBasis) (int64, error) {
	if b.EstimatedCost < 0 {
		return 0, fmt.Errorf("%w, got %d", ErrLogisticsCostNegative, b.EstimatedCost)
	}
	if b.PlatformSubsidy < 0 {
		return 0, fmt.Errorf("%w, got %d", ErrLogisticsSubsidyNegative, b.PlatformSubsidy)
	}
	charge := b.EstimatedCost - b.PlatformSubsidy + b.ResponsibilityAdjustment
	if charge < 0 {
		return 0, fmt.Errorf("%w（estimated=%d, subsidy=%d, adjustment=%d）",
			ErrLogisticsChargeNegative, b.EstimatedCost, b.PlatformSubsidy, b.ResponsibilityAdjustment)
	}
	return charge, nil
}

// LogisticsFeeInvoiceDraft 是一張 Platform → Seller 物流費發票的草稿。
//
// 與買家軸的 IssueDraft 分開一個型別而不是共用：兩者的收受人語意相反
// （這裡的賣家是**進項憑證持有人**，不是開票人），共用會讓「誰開給誰」
// 退化成一個要靠欄位值去猜的問題。
type LogisticsFeeInvoiceDraft struct {
	// IssuerType 恆為 PLATFORM，IssuerRefID 恆為 0（chk_invoice_issuers_ref）。
	IssuerType  string
	IssuerRefID int64

	// RecipientType 恆為 SELLER；RecipientRefID 是 sellers.store_id（store surrogate，
	// **不是 account_id**）。
	RecipientType  string
	RecipientRefID int64
	RecipientTaxID string

	// DocumentKind 恆為 B2B：收受人是營業人，必須拿得到可扣抵的進項憑證。
	DocumentKind string

	Lines  []Line
	Header Header

	// SellerLogisticsCharge 是本張發票的開票基礎，逐字留存供對帳。
	SellerLogisticsCharge int64
	// BuyerShippingChargeForReconciliation 是同一期買家付的運費，**不是**開票基礎。
	// 落進草稿只為了讓對帳報表能把兩個數字並排；改它不會改變任何金額。
	BuyerShippingChargeForReconciliation int64
}

// BuildLogisticsFeeInvoice 組出 Platform → Seller 的物流費發票草稿。
//
// 🔴 表頭一律走既有的 BuildHeader（紅線 15：不重複實作），並以 recipientTaxID
// 當它的 buyerIdentifier —— 這一軸的「買受人」就是賣家。
func BuildLogisticsFeeInvoice(storeID int64, recipientTaxID string, b LogisticsFeeBasis) (LogisticsFeeInvoiceDraft, error) {
	if storeID <= 0 {
		return LogisticsFeeInvoiceDraft{}, fmt.Errorf("invoice/logistics: storeID 必須 > 0，got %d", storeID)
	}
	if !IsBusinessTaxID(recipientTaxID) {
		return LogisticsFeeInvoiceDraft{}, fmt.Errorf("%w，got %q", ErrLogisticsRecipientTaxIDRequired, recipientTaxID)
	}

	charge, err := SellerLogisticsCharge(b)
	if err != nil {
		return LogisticsFeeInvoiceDraft{}, err
	}
	if charge == 0 {
		return LogisticsFeeInvoiceDraft{}, ErrLogisticsChargeNotPositive
	}

	lines := []Line{{
		LineSeq:     1,
		LineType:    codemeta.InvoiceLineTypeShipping.Value,
		Description: LogisticsFeeDescription,
		Quantity:    1,
		UnitPrice:   charge,
		NetAmount:   charge,
		TaxCategory: TaxCategoryTaxable,
		// 本軸沒有折扣：平台補貼已經在 charge 裡減掉了，再開一行負數折扣
		// 等於同一筆補貼被認兩次。
		AllocatedDiscount: 0,
		AllowanceableBase: charge,
	}}

	return LogisticsFeeInvoiceDraft{
		IssuerType:                           codemeta.InvoiceIssuerTypePlatform.Value,
		IssuerRefID:                          0,
		RecipientType:                        codemeta.InvoiceRecipientTypeSeller.Value,
		RecipientRefID:                       storeID,
		RecipientTaxID:                       recipientTaxID,
		DocumentKind:                         codemeta.InvoiceDocumentKindB2b.Value,
		Lines:                                lines,
		Header:                               BuildHeader(lines, recipientTaxID),
		SellerLogisticsCharge:                charge,
		BuyerShippingChargeForReconciliation: b.BuyerShippingCharge,
	}, nil
}

// ============================================================================
// W5-5 買家退款 ≠ 取消賣家物流費
// ============================================================================

// ApplyBuyerRefund 回報一筆買家退款之後，賣家該期的物流費認列金額。
//
// 🔴 **回傳值恆等於退款前的金額**，這正是本函式存在的理由。
//
// 買家退款走的是「買賣價金」軸，賣家物流費走的是「履約成本」軸——包裹已經被外送員
// 載走、平台已經付錢給通道，買家事後退款不會讓那筆成本消失。把兩者綁在一起
// （退款 ⇒ 物流費歸零）會讓平台替每一筆退款吸收一次物流成本，而帳上只看得到
// 一筆正常的退款。
//
// 要讓賣家少付只有一條路：由 payment-service 依 ADR-004 判定成因（ResponsibilityRules），
// 把結果填進 LogisticsFeeBasis.ResponsibilityAdjustment 再重算 —— 那是「責任」，
// 不是「退款」。兩者在某些案例上會同時發生，但**因果方向是責任，不是退款金額**。
//
// buyerRefundAmount 收進來只為了守衛與留痕；它**不會**被用來計算。
func ApplyBuyerRefund(b LogisticsFeeBasis, buyerRefundAmount int64) (int64, error) {
	if buyerRefundAmount < 0 {
		return 0, fmt.Errorf("%w, got %d", ErrLogisticsBuyerRefundNegative, buyerRefundAmount)
	}
	return SellerLogisticsCharge(b)
}

// ============================================================================
// W5-3 補收／退還 ⇒ Document Required
// ============================================================================

// LogisticsAdjustmentDirection 是一筆物流費財務調整的**錢流方向**。
//
// 🔴 <b>它不是 V9 的 `logistics_adjustments.kind`，兩者問的是不同的問題</b>——
// 這一格曾經也叫 Kind，而同名不同義是最貴的一種命名錯誤：
//
//	V9 的 kind ∈ {RESPONSIBILITY, RECONCILIATION}  問「這筆調整**憑什麼**發生」
//	本型別        ∈ {補收, 退還}                     問「錢**往哪個方向**流」
//
// 兩者正交：一筆 RECONCILIATION 可以是補收也可以是退還。V9 那組值有 SSOT
// （`V9__add_logistics_finance_tables.sql` 的 `chk_logadj_kind` ＋ 70 號設計書 §268
// ＋ `LogisticsReconciliationService.ADJ_KIND_RECONCILIATION`）；本型別**沒有**，
// 因為它不落庫——它由 payment 側那一列的兩個金額欄推導出來，
// 見 LogisticsAdjustmentDirectionOf。
//
// ⚠️ 刻意**不進 CodeMeta**：目前沒有任何欄位存它，另存一份碼表就會有兩個真實來源、
// 必然漂移。🔴 哪天它真的要落庫或跨服務暴露，那時才升格進 CodeMeta ——
// 升格前不得把這些字串寫進 DB、proto 或對外 response。
type LogisticsAdjustmentDirection string

const (
	// LogisticsAdjustmentSupplementaryCollection：補收（平台向賣家多收一筆）。
	// 對應 payment 側 `logistics_adjustments.seller_chargeable > 0`。
	LogisticsAdjustmentSupplementaryCollection LogisticsAdjustmentDirection = "SUPPLEMENTARY_COLLECTION"
	// LogisticsAdjustmentRefundToSeller：退還（平台退還賣家多收的部分）。
	// 對應 payment 側 `logistics_adjustments.seller_credit > 0`。
	LogisticsAdjustmentRefundToSeller LogisticsAdjustmentDirection = "REFUND_TO_SELLER"
)

// ErrLogisticsAdjustmentDirectionAmbiguous 表示兩個金額欄同時為正或同時為零。
//
// 🔴 一律 fail-loud：ADR-008 §1「應收與貸記是兩個科目，不是一個科目的正負」——
// 同一列同時掛著兩邊，代表產生它的那支 reconciliation 算錯了，
// 而挑一邊當方向會把一個帳務錯誤變成一張開出去的憑證。
var ErrLogisticsAdjustmentDirectionAmbiguous = errors.New(
	"invoice/logistics: 補收與退還必須恰好一邊為正，無法判定調整方向")

// LogisticsAdjustmentDirectionOf 由 payment 側那一列的兩個金額欄推導方向。
//
// 🔴 <b>方向是推導出來的，不是另外存一個碼</b>：存第二個碼就會有「碼說補收、
// 金額說退還」的那一天，而憑證是照碼開的。
func LogisticsAdjustmentDirectionOf(sellerChargeable, sellerCredit int64) (LogisticsAdjustmentDirection, error) {
	if sellerChargeable < 0 || sellerCredit < 0 {
		return "", fmt.Errorf("%w（chargeable=%d, credit=%d：金額欄恆為正，負值代表上游算錯）",
			ErrLogisticsAdjustmentDirectionAmbiguous, sellerChargeable, sellerCredit)
	}
	switch {
	case sellerChargeable > 0 && sellerCredit == 0:
		return LogisticsAdjustmentSupplementaryCollection, nil
	case sellerCredit > 0 && sellerChargeable == 0:
		return LogisticsAdjustmentRefundToSeller, nil
	default:
		return "", fmt.Errorf("%w（chargeable=%d, credit=%d）",
			ErrLogisticsAdjustmentDirectionAmbiguous, sellerChargeable, sellerCredit)
	}
}

// LogisticsAdjustmentDisposition 是一筆財務調整該怎麼處置。
type LogisticsAdjustmentDisposition string

// LogisticsDispositionDocumentRequired：補收與退還**都**需要一張憑證。
//
// 目前是唯一的值，而且刻意留成一個具名型別而不是 bool：W5-3 的重點是
// 「有沒有憑證義務」這件事本身要被講出來，而不是靠呼叫端記得。
const LogisticsDispositionDocumentRequired LogisticsAdjustmentDisposition = "DOCUMENT_REQUIRED"

// ErrLogisticsAdjustmentDirectionInvalid 表示方向不在值域內。
var ErrLogisticsAdjustmentDirectionInvalid = errors.New("invoice/logistics: 未知的財務調整方向")

// DispositionOfLogisticsAdjustment 回報這筆調整的憑證義務。
//
// 🔴 **正面列舉、沒有 default 放行**：日後新增一種調整方向而忘了在這裡指派處置時，
// 行為是回錯誤（fail-closed），不是靜默當成「不用開憑證」。
func DispositionOfLogisticsAdjustment(direction LogisticsAdjustmentDirection) (LogisticsAdjustmentDisposition, error) {
	switch direction {
	case LogisticsAdjustmentSupplementaryCollection:
		return LogisticsDispositionDocumentRequired, nil
	case LogisticsAdjustmentRefundToSeller:
		return LogisticsDispositionDocumentRequired, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrLogisticsAdjustmentDirectionInvalid, direction)
	}
}

// LogisticsAdjustmentDocStatus 是「這筆調整的憑證走到哪了」。
//
// ⚠️ 與 LogisticsAdjustmentDirection 同樣刻意不進 CodeMeta（見上）。
type LogisticsAdjustmentDocStatus string

const (
	// LogisticsDocStatusManualDocumentRequired：🔴 **會計形式尚未確認，必須人工處置。**
	//
	// 這是一個**真的會擋住**的狀態，不是一個 log 完就往下走的標記：
	// PrepareLogisticsAdjustmentDocument 在這個狀態下回錯誤且**不回草稿**。
	//
	// 為什麼不能先自動開一張再說：補收與退還在稅務上的正確形式（開新發票／開折讓單／
	// 開收據／不開憑證只走帳）尚未定案（W5-3 逐字）。自動開票等於替一個還沒有人決定的
	// 問題選一個答案，而且是選在**已經送進國稅局**之後才會被發現的那一種。
	LogisticsDocStatusManualDocumentRequired LogisticsAdjustmentDocStatus = "MANUAL_DOCUMENT_REQUIRED"

	// LogisticsDocStatusReadyToIssue：會計形式已定案，可以走自動線。
	//
	// ⚠️ 今天沒有任何路徑產生這個值（LogisticsAccountingFormDecided 恆為 false）。
	// 它存在是為了讓「擋住」與「放行」是同一個型別上的兩個值，而不是靠一個 bool 的反面。
	LogisticsDocStatusReadyToIssue LogisticsAdjustmentDocStatus = "READY_TO_ISSUE"
)

// ErrLogisticsManualDocumentRequired 是本軸的 fail-closed 出口。
//
// 呼叫端收到它必須**停手並把案件交給人**，不得 fallback 成任何自動開立路徑。
var ErrLogisticsManualDocumentRequired = errors.New(
	"invoice/logistics: 會計形式尚未確認，本筆調整必須人工開立憑證（MANUAL_DOCUMENT_REQUIRED）")

// LogisticsAccountingFormDecided 回報「補收／退還要開哪一種憑證」是否已經定案。
//
// 🔴 **目前恆為 false**，這是對現實的陳述不是 feature flag：W5-3 逐字寫著會計形式
// 尚未確認。定案的那天，正確做法是改本函式並補上對應的憑證形式，
// **不是**在呼叫端加一條旁路 —— 旁路會讓這個唯一的抉擇點失去唯一性。
func LogisticsAccountingFormDecided() bool { return false }

// InitialLogisticsAdjustmentDocStatus 是一筆新調整建立當下該落的憑證狀態。
func InitialLogisticsAdjustmentDocStatus() LogisticsAdjustmentDocStatus {
	if !LogisticsAccountingFormDecided() {
		return LogisticsDocStatusManualDocumentRequired
	}
	return LogisticsDocStatusReadyToIssue
}

// LogisticsAdjustment 是一筆待處置的物流費財務調整。
type LogisticsAdjustment struct {
	StoreID        int64
	RecipientTaxID string
	Direction      LogisticsAdjustmentDirection
	// Amount 是調整金額，恆為正；方向由 Direction 表達。
	Amount    int64
	DocStatus LogisticsAdjustmentDocStatus
}

// PrepareLogisticsAdjustmentDocument 嘗試把一筆調整轉成可送出的發票草稿。
//
// 🔴 **狀態檢查在最前面，且失敗時回零值草稿**：把守衛放在算完金額之後，
// 「擋住了」與「算好了但沒送」在呼叫端長得一模一樣，而後者很容易被下一個人
// 順手接上送出路徑。
//
// 🔴 退還（REFUND_TO_SELLER）即使將來定案了也**不會**從這裡出去：
// 退還在稅務上是折讓或退回憑證，不是一張新的銷項發票。所以本函式對它一律回錯誤，
// 不是「等定案後再說」。
func PrepareLogisticsAdjustmentDocument(adj LogisticsAdjustment) (LogisticsFeeInvoiceDraft, error) {
	disposition, err := DispositionOfLogisticsAdjustment(adj.Direction)
	if err != nil {
		return LogisticsFeeInvoiceDraft{}, err
	}
	if disposition != LogisticsDispositionDocumentRequired {
		return LogisticsFeeInvoiceDraft{}, fmt.Errorf(
			"invoice/logistics: 處置 %q 沒有對應的開立路徑", disposition)
	}

	// 🔴 fail-closed：未知的狀態值一律當成「要人工」，不是當成可以放行。
	if adj.DocStatus != LogisticsDocStatusReadyToIssue {
		return LogisticsFeeInvoiceDraft{}, fmt.Errorf("%w（direction=%s, status=%s）",
			ErrLogisticsManualDocumentRequired, adj.Direction, adj.DocStatus)
	}

	if adj.Direction == LogisticsAdjustmentRefundToSeller {
		return LogisticsFeeInvoiceDraft{}, fmt.Errorf(
			"%w（退還須走折讓／退回憑證，不得開立新的銷項發票）", ErrLogisticsManualDocumentRequired)
	}

	if adj.Amount <= 0 {
		return LogisticsFeeInvoiceDraft{}, ErrLogisticsChargeNotPositive
	}

	// 補收的開票基礎就是補收金額本身：它已經是「賣家要多付的物流費」，
	// 不再經過補貼與責任調整（那兩者在算出這筆補收時就已經套用過）。
	return BuildLogisticsFeeInvoice(adj.StoreID, adj.RecipientTaxID, LogisticsFeeBasis{
		EstimatedCost: adj.Amount,
	})
}

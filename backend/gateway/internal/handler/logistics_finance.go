package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/payment"
	"github.com/gin-gonic/gin"
)

// ==================== 物流財務（線 B）====================
//
// 賣家面回答「我被收了多少物流費、收到哪一步了」；Admin 面回答「這筆應收／這次收款／
// 這份 provider 帳單現在是什麼狀態」。全部唯讀。
//
// 🔴 **欄位名是業務語意，不是 DB 欄名**：DB 的 `estimated_cost`／`target_net`／`actual_fee`
// 對賣家沒有意義，對外一律用 `estimatedProviderCost`／`sellerLogisticsCharge`／
// `payuniCollectionFee`。前端照著接就是可以直接放進畫面的字。
//
// 🔴 **`null` 不是 `0`**：下面幾個指標型欄位刻意用 `*string`（JSON `null`）而不是空字串或 "0"：
//   actualProviderCost  null = 尚未對帳（provider 帳單還沒到）
//   payuniCollectionFee null = 尚未查詢 PayUni 撥款結果
//   collectionResidual  null = 同上（"0" 代表「查到了而且正好收足」，語意完全不同）
//   fundDate            null = 尚未撥款
//   sellerSubsidy       null = **本服務不是這個維度的來源**（線 A 的資料）
// ⚠️ 一律**不加** `omitempty`：加了之後零值欄位會整個從 JSON 消失，前端拿到 `undefined`
// 而不是 `null`，兩者在 TypeScript 的處理路徑不同（#2220 的教訓）。

// ---- 賣家面 DTO ----

// LogisticsCollectionStatusCountResp 應收狀態分佈的一列。
type LogisticsCollectionStatusCountResp struct {
	Status string `json:"status"` // LogisticsReceivableStatus value
	// StatusLabel 來自後端的 CodeMeta enum，前端不必自己維護第二份對照表。
	StatusLabel string `json:"statusLabel"`
	Count       string `json:"count"`
}

// SellerLogisticsFinanceSummaryResp 賣家物流財務總覽（W6-3 的十一個維度）。
//
// 🔴 sellerCredit（平台欠賣家）與 receivableBalance（賣家欠平台）是**兩個科目**，
// 永遠不得合成一個帶正負的淨額：兩者可以同時存在（這期欠、上期多收），合成之後
// 「平台到底欠我多少」就再也追不回來了。
type SellerLogisticsFinanceSummaryResp struct {
	StoreId     string `json:"storeId"`
	CreatedFrom string `json:"createdFrom"`
	CreatedTo   string `json:"createdTo"`

	EstimatedProviderCost string `json:"estimatedProviderCost"` // 1/11 暫估 provider 成本
	ActualProviderCost    string `json:"actualProviderCost"`    // 2/11 只含已對帳的部分
	UnreconciledCount     string `json:"unreconciledCount"`     // 2/11 的缺口：尚未對帳的筆數
	SellerLogisticsCharge string `json:"sellerLogisticsCharge"` // 3/11 賣家實際被收的物流費
	PayuniCollectionFee   string `json:"payuniCollectionFee"`   // 4/11 實際手續費（非預估）
	PendingFeeQueryCount  string `json:"pendingFeeQueryCount"`  // 4/11 的缺口：尚未查到撥款結果的筆數
	PlatformSubsidy       string `json:"platformSubsidy"`       // 5/11 平台吸收
	// SellerSubsidy 6/11：賣家對「買家」的運費補貼（線 A）。
	// 🔴 恆為 null —— payment-service 沒有這個維度的資料來源，而回 "0" 會讓
	// 「查不到」長得跟「賣家真的沒補貼」一樣。它也**永遠不得**與 platformSubsidy 合併：
	// 賣家替買家出運費，不會讓物流商少收平台一毛錢。
	SellerSubsidy            *string                              `json:"sellerSubsidy"`
	ResponsibilityAdjustment string                               `json:"responsibilityAdjustment"` // 7/11 這筆錢算誰的
	ReconciliationAdjustment string                               `json:"reconciliationAdjustment"` // 8/11 帳單與暫估的差
	CollectionStatusCounts   []LogisticsCollectionStatusCountResp `json:"collectionStatusCounts"`   // 9/11
	CollectionResidual       string                               `json:"collectionResidual"`       // 10/11 正=未收足、負=多收
	SellerCredit             string                               `json:"sellerCredit"`             // 11/11 平台欠賣家
	ReceivableBalance        string                               `json:"receivableBalance"`        // 賣家欠平台（與上一欄不同科目）

	LastActivityAt string `json:"lastActivityAt"`
	AgingDays      string `json:"agingDays"`
	// HasAccount false ⇒ 上面兩個餘額是 "0"，但那是「還沒有帳戶」不是「已結清」。
	HasAccount bool `json:"hasAccount"`
}

// SellerLogisticsReceivableResp 單筆應收（欄位語意與總覽逐一對應）。
type SellerLogisticsReceivableResp struct {
	ReceivableNo        string `json:"receivableNo"`
	OrderNo             string `json:"orderNo"`
	ShipmentId          string `json:"shipmentId"`
	Provider            string `json:"provider"` // ShippingMethod value
	ProviderDeliveryRef string `json:"providerDeliveryRef"`

	EstimatedProviderCost    string  `json:"estimatedProviderCost"`
	ActualProviderCost       *string `json:"actualProviderCost"` // null = 尚未對帳
	SellerLogisticsCharge    string  `json:"sellerLogisticsCharge"`
	PayuniCollectionFee      *string `json:"payuniCollectionFee"` // null = 尚未查詢
	PlatformSubsidy          string  `json:"platformSubsidy"`
	SellerSubsidy            *string `json:"sellerSubsidy"` // 恆為 null，見總覽 DTO 的說明
	ResponsibilityAdjustment string  `json:"responsibilityAdjustment"`
	ReconciliationAdjustment string  `json:"reconciliationAdjustment"`
	CollectionStatus         string  `json:"collectionStatus"`
	CollectionStatusLabel    string  `json:"collectionStatusLabel"`
	CollectionResidual       *string `json:"collectionResidual"` // null = 尚未查詢
	SellerCredit             string  `json:"sellerCredit"`

	CarriedResidual     string `json:"carriedResidual"`
	ChargeBasisFrozenAt string `json:"chargeBasisFrozenAt"`
	InvoiceDocumentId   string `json:"invoiceDocumentId"`
	StatementLineId     string `json:"statementLineId"`
	CreatedAt           string `json:"createdAt"`
	UpdatedAt           string `json:"updatedAt"`
	AgingDays           string `json:"agingDays"`
}

// SellerLogisticsReceivableListResp 賣家應收明細分頁。
type SellerLogisticsReceivableListResp struct {
	Records  []SellerLogisticsReceivableResp `json:"records"`
	Total    string                          `json:"total"`
	Page     string                          `json:"page"`
	PageSize string                          `json:"pageSize"`
}

// LogisticsLedgerEntryResp 一筆帳本分錄。
type LogisticsLedgerEntryResp struct {
	LineSeq     string `json:"lineSeq"`
	AccountCode string `json:"accountCode"`
	OwnerType   string `json:"ownerType"`
	OwnerRefId  string `json:"ownerRefId"`
	// Direction DEBIT / CREDIT。🔴 amount 恆為正，正負由本欄表達——前端不得把兩者
	// 折成一個帶正負的數字再顯示，那會讓借貸平衡看不出來。
	Direction string `json:"direction"`
	Amount    string `json:"amount"`
	Memo      string `json:"memo"`
}

// LogisticsLedgerTxnResp 一筆帳本交易（表頭 + 分錄）。
type LogisticsLedgerTxnResp struct {
	TxnNo            string                     `json:"txnNo"`
	TxnType          string                     `json:"txnType"`
	OrderNo          string                     `json:"orderNo"`
	SettlementPeriod string                     `json:"settlementPeriod"`
	SourceType       string                     `json:"sourceType"`
	SourceRef        string                     `json:"sourceRef"`
	PostedAt         string                     `json:"postedAt"`
	Entries          []LogisticsLedgerEntryResp `json:"entries"`
}

// LogisticsLedgerListResp 賣家帳本流水分頁。
type LogisticsLedgerListResp struct {
	Records  []LogisticsLedgerTxnResp `json:"records"`
	Total    string                   `json:"total"`
	Page     string                   `json:"page"`
	PageSize string                   `json:"pageSize"`
}

// ---- Admin 面 DTO ----

// AdminLogisticsReceivableResp Admin 應收列表／詳情共用的單筆。
type AdminLogisticsReceivableResp struct {
	ReceivableNo        string  `json:"receivableNo"`
	StoreId             string  `json:"storeId"`
	OrderNo             string  `json:"orderNo"`
	ShipmentId          string  `json:"shipmentId"`
	Provider            string  `json:"provider"`
	ProviderDeliveryRef string  `json:"providerDeliveryRef"`
	EstimatedCost       string  `json:"estimatedCost"`
	ActualCost          *string `json:"actualCost"` // null = 尚未對帳
	PlatformSubsidy     string  `json:"platformSubsidy"`
	TargetNet           string  `json:"targetNet"`
	CarriedResidual     string  `json:"carriedResidual"`
	Status              string  `json:"status"`
	StatusLabel         string  `json:"statusLabel"`
	StatementLineId     string  `json:"statementLineId"`
	InvoiceDocumentId   string  `json:"invoiceDocumentId"`
	ChargeBasisFrozenAt string  `json:"chargeBasisFrozenAt"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
	AgingDays           string  `json:"agingDays"`
	// SubmitLeaseUntil 是提交 worker 的**租約**，🔴 不是「已送出」旗標。
	// 非空只代表此刻有某個 pod 正在處理它，租約到期後會自動回到候選集合。
	SubmitLeaseUntil string `json:"submitLeaseUntil"`
}

// AdminLogisticsReceivableListResp Admin 應收分頁。
type AdminLogisticsReceivableListResp struct {
	Records  []AdminLogisticsReceivableResp `json:"records"`
	Total    string                         `json:"total"`
	Page     string                         `json:"page"`
	PageSize string                         `json:"pageSize"`
}

// AdminLogisticsCollectionResp 一次 PayUni 收款嘗試。
type AdminLogisticsCollectionResp struct {
	CollectionNo string `json:"collectionNo"`
	ReceivableNo string `json:"receivableNo"`
	StoreId      string `json:"storeId"`
	TradeNo      string `json:"tradeNo"`
	GrossAmount  string `json:"grossAmount"`
	// 以下四欄 null = 尚未查詢 PayUni，不是 0。
	NetProceeds      *string `json:"netProceeds"`
	Residual         *string `json:"residual"`
	ActualFee        *string `json:"actualFee"`
	FundDate         *string `json:"fundDate"`
	SubmitStatus     string  `json:"submitStatus"`
	SubmitMessage    string  `json:"submitMessage"`
	SubmittedAt      string  `json:"submittedAt"`
	ExpectedFundDate string  `json:"expectedFundDate"`
	// QueryLeaseUntil 是查詢 worker 的租約，🔴 不是「已查過」旗標。
	QueryLeaseUntil string `json:"queryLeaseUntil"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// AdminLogisticsCollectionListResp Admin 收款嘗試分頁。
type AdminLogisticsCollectionListResp struct {
	Records  []AdminLogisticsCollectionResp `json:"records"`
	Total    string                         `json:"total"`
	Page     string                         `json:"page"`
	PageSize string                         `json:"pageSize"`
}

// AdminLogisticsAdjustmentResp 一張責任／對帳調整單。
type AdminLogisticsAdjustmentResp struct {
	AdjustmentNo string `json:"adjustmentNo"`
	// ReceivableNo 空字串 = 無母單（未送達不建應收，但取消費仍然發生）。
	ReceivableNo        string `json:"receivableNo"`
	StoreId             string `json:"storeId"`
	ProviderDeliveryRef string `json:"providerDeliveryRef"`
	Kind                string `json:"kind"` // RESPONSIBILITY / RECONCILIATION
	// Amount 允許負數：這是業務調整單，正負就是它的語意。
	Amount              string `json:"amount"`
	ResponsibilityCause string `json:"responsibilityCause"`
	StatementLineId     string `json:"statementLineId"`
	// CreatedBy 是 ADM 操作者的 account_id（不是 storeId）；系統產生時為空字串。
	CreatedBy string `json:"createdBy"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"createdAt"`
}

// AdminLogisticsAdjustmentListResp Admin 調整單分頁。
type AdminLogisticsAdjustmentListResp struct {
	Records  []AdminLogisticsAdjustmentResp `json:"records"`
	Total    string                         `json:"total"`
	Page     string                         `json:"page"`
	PageSize string                         `json:"pageSize"`
}

// AdminSellerLogisticsAccountResp 賣家物流帳戶的兩個科目。
type AdminSellerLogisticsAccountResp struct {
	StoreId string `json:"storeId"`
	// ReceivableBalance 賣家欠平台；CreditBalance 平台欠賣家。
	// 🔴 兩個科目，不是一個欄位的正負——不得在前端合成淨額顯示。
	ReceivableBalance string `json:"receivableBalance"`
	CreditBalance     string `json:"creditBalance"`
	LastActivityAt    string `json:"lastActivityAt"`
	AgingDays         string `json:"agingDays"`
	// Exists false ⇒ 上面兩個餘額是 "0"，但那是「還沒有帳戶」不是「已結清」。
	Exists bool `json:"exists"`
}

// AdminLogisticsReceivableDetailResp Admin 應收詳情。
type AdminLogisticsReceivableDetailResp struct {
	Receivable    AdminLogisticsReceivableResp    `json:"receivable"`
	Collections   []AdminLogisticsCollectionResp  `json:"collections"`
	Adjustments   []AdminLogisticsAdjustmentResp  `json:"adjustments"`
	SellerAccount AdminSellerLogisticsAccountResp `json:"sellerAccount"`
}

// AdminProviderStatementResp 一份 provider 帳單的彙總。
//
// 🔴 mismatchCount / mismatchAmount 是**推導值**，不是 reconStatus 的第五個碼值：
// DB 的值域只有 UNMATCHED / MATCHED / DISPUTED / WRITTEN_OFF。「金額對不上」在本模型的
// 落點是 kind=RECONCILIATION 的調整單，所以這兩欄由那張表推出來。
type AdminProviderStatementResp struct {
	Provider        string `json:"provider"`
	StatementRef    string `json:"statementRef"`
	StatementPeriod string `json:"statementPeriod"`
	LineCount       string `json:"lineCount"`
	TotalAmount     string `json:"totalAmount"`
	MatchedCount    string `json:"matchedCount"`
	UnmatchedCount  string `json:"unmatchedCount"` // = 尚未比對（unresolved）
	DisputedCount   string `json:"disputedCount"`
	WrittenOffCount string `json:"writtenOffCount"`
	MismatchCount   string `json:"mismatchCount"`
	MismatchAmount  string `json:"mismatchAmount"`
	FirstImportedAt string `json:"firstImportedAt"`
	LastImportedAt  string `json:"lastImportedAt"`
}

// AdminProviderStatementListResp Admin 帳單分頁。
type AdminProviderStatementListResp struct {
	Records  []AdminProviderStatementResp `json:"records"`
	Total    string                       `json:"total"`
	Page     string                       `json:"page"`
	PageSize string                       `json:"pageSize"`
}

// AdminProviderSettlementLineResp 一列帳單明細。
type AdminProviderSettlementLineResp struct {
	LineId              string `json:"lineId"`
	Provider            string `json:"provider"`
	StatementRef        string `json:"statementRef"`
	StatementPeriod     string `json:"statementPeriod"`
	ProviderShipmentRef string `json:"providerShipmentRef"`
	OrderNo             string `json:"orderNo"`
	FeeType             string `json:"feeType"`     // ProviderFeeType value
	Amount              string `json:"amount"`      // 恆為正
	ReconStatus         string `json:"reconStatus"` // UNMATCHED / MATCHED / DISPUTED / WRITTEN_OFF
	LedgerTxnNo         string `json:"ledgerTxnNo"`
	ExceptionCaseId     string `json:"exceptionCaseId"`
	// ImportedBy 是 ADM 操作者的 account_id（不是 storeId）。
	ImportedBy string `json:"importedBy"`
	ImportedAt string `json:"importedAt"`
	// 推導出來的 mismatch：本列有沒有對帳調整單、金額多少。
	HasReconciliationAdjustment    bool   `json:"hasReconciliationAdjustment"`
	ReconciliationAdjustmentAmount string `json:"reconciliationAdjustmentAmount"`
}

// AdminProviderStatementDetailResp Admin 帳單詳情。
type AdminProviderStatementDetailResp struct {
	Statement AdminProviderStatementResp        `json:"statement"`
	Lines     []AdminProviderSettlementLineResp `json:"lines"`
	// AwaitingReviewCount = 這份帳單牽動的案件中尚待人工覆核的件數（manual review）。
	AwaitingReviewCount string `json:"awaitingReviewCount"`
}

// AdminFulfillmentCaseResolutionResp 一張處置提案（manual review 的載體）。
type AdminFulfillmentCaseResolutionResp struct {
	ResolutionId   string `json:"resolutionId"`
	ResolutionType string `json:"resolutionType"`
	Reason         string `json:"reason"`
	Status         string `json:"status"` // AWAITING_REVIEW / APPROVED / REJECTED
	RequiresReview bool   `json:"requiresReview"`
	// 以下皆為 ADM 操作者的 account_id（不是 storeId）。
	ProposedByAccountId string `json:"proposedByAccountId"`
	ProposedAt          string `json:"proposedAt"`
	ReviewedByAccountId string `json:"reviewedByAccountId"`
	ReviewComment       string `json:"reviewComment"`
	ReviewedAt          string `json:"reviewedAt"`

	// ── 責任判定軸（V13）────────────────────────────────────────────────────
	//
	// 🔴 ProposedResponsibilityCause 是 maker 主張的成因，也是**提案的標的物**：
	// approve 時原封不動寫進 case.resolutionCode 並成為 N16 的 responsibilityCause，
	// 覆核者改不了它。值域 BUYER_CAUSED / SELLER_CAUSED / PROVIDER_CAUSED /
	// PLATFORM_CAUSED（🔴 不存在 SYSTEM_CAUSED）。非責任判定型別的提案此欄為空字串。
	ProposedResponsibilityCause string `json:"proposedResponsibilityCause"`
	// 提案當下 server 算出的影響預覽快照，「元」的整數字串。
	// 🔴 **只有退款金額一個維度**：platformFeeDelta／sellerNetDelta 在現行商流下沒有
	// canonical 算法（訂單層沒有平台代收），填 0 會讓覆核者看到「平台與賣家都不受影響」
	// 的假結論，比不顯示更糟。不得加回來。
	ImpactPreviewRefundAmount string `json:"impactPreviewRefundAmount"`
	// 快照計算時間（ISO 8601 UTC）。⚠️ 不參與 stale 比較——重算必然產生新時間戳，
	// 納入比較會讓每次覆核都判 9575，那道守衛就永遠是狼來了。
	ImpactComputedAt string `json:"impactComputedAt"`
}

// AdminFulfillmentExceptionCaseResp 一張維運例外案件。
type AdminFulfillmentExceptionCaseResp struct {
	CaseNo        string `json:"caseNo"`
	CaseId        string `json:"caseId"`
	SourceService string `json:"sourceService"` // shipping / order / payment
	Category      string `json:"category"`
	Severity      string `json:"severity"` // P0 / P1 / P2
	SubjectType   string `json:"subjectType"`
	SubjectRef    string `json:"subjectRef"`
	StoreId       string `json:"storeId"`
	Status        string `json:"status"` // OPEN / IN_PROGRESS / RESOLVED / DISMISSED
	// 🔴 離開 OPEN ⇒ 必有承辦人。OPEN 時為空字串。
	StatusEnteredAt     string `json:"statusEnteredAt"`
	OwnerAccountId      string `json:"ownerAccountId"`
	OwnerAssignedAt     string `json:"ownerAssignedAt"`
	ResolutionCode      string `json:"resolutionCode"`
	ResolutionNote      string `json:"resolutionNote"`
	ResolvedByAccountId string `json:"resolvedByAccountId"`
	ResolvedAt          string `json:"resolvedAt"`
	// OccurredAt 是來源端的發生時間，與 createdAt 是兩個時間，不可互相冒充。
	OccurredAt string `json:"occurredAt"`
	CreatedAt  string `json:"createdAt"`

	// ── 責任判定軸的 immutable event context（V14）──────────────────────────
	//
	// 🔴 **建案當下凍結，此後不可改**。覆核（A-5）時無從指定——它取的是同一列，
	// 所以 N17（unresolved）與 N16（finalized）不可能帶不同的 orderNo。orderNo 正是
	// order-service 把訂單從 SHIPPING_ABNORMAL(35) 推到 DELIVERY_FAILED_NO_REFUND(54)
	// 的 CAS 錨；對不上時那道 CAS 永遠 rows=0，而全鏈路沒有任何錯誤訊息。
	//
	// ⚠️ 三欄都可能是空字串：generic exception case（理賠／對帳未匹配…）本來就沒有
	// 它們；shipmentId 連責任案件都可能沒有——建單失敗（CREATE_FAILED 990）也是一種
	// 履約失敗，而那時 shipment 還不存在。
	OrderNo    string `json:"orderNo"`
	ShipmentId string `json:"shipmentId"`
	// ShippingMethod 碼值。🔴 provider-neutral：責任判定不限特定物流商。
	ShippingMethod string `json:"shippingMethod"`

	Resolutions []AdminFulfillmentCaseResolutionResp `json:"resolutions"`
}

// AdminFulfillmentExceptionCaseListResp Admin 例外案件分頁。
type AdminFulfillmentExceptionCaseListResp struct {
	Records  []AdminFulfillmentExceptionCaseResp `json:"records"`
	Total    string                              `json:"total"`
	Page     string                              `json:"page"`
	PageSize string                              `json:"pageSize"`
}

// ---- 責任判定 maker-checker 的 request／response DTO（65）----

// 覆核決定的兩個合法值。🔴 只有這兩個——沒有 SKIP／DEFER／NEED_INFO：那些都是
// 「還沒決定」，而還沒決定的正確表示法是提案維持 AWAITING_REVIEW，不是第三個決定值。
const (
	responsibilityDecisionApprove = "APPROVE"
	responsibilityDecisionReject  = "REJECT"
)

// ProposeResponsibilityResolutionRequest A-4 maker 提案。
//
// 🔴 **只有三個欄位，刻意不收金額與身分**：
//   - 沒有 impactPreview／refundAmount／platformFeeDelta／sellerNetDelta —— 影響預覽由
//     server 依 responsibilityCause ＋ orders.total_actual_amount 算出並持久化。
//     由 client 給金額 ＝ 讓被覆核的那個數字由被覆核者自己決定。
//   - 沒有 proposedByAccountId —— 提案者取自 gRPC auth context（gateway 轉發的
//     x-account-id）。由 request 讀等於讓呼叫端宣稱自己是誰，雙人制就形同虛設。
//
// ⚠️ resolutionType 與 responsibilityCause 的**值域**刻意不在 gateway 驗：它們是
// CodeMeta 碼值（ResponsibilityCause）與 DB CHECK（chk_fcr_type）守的東西，在這裡複製
// 一份白名單，下次碼表加值時就會多一個沒有人記得要改的地方。gateway 只驗「有沒有填」，
// 值域由 payment-service fail-closed 回 9571。decision 是另一回事（見下）。
type ProposeResponsibilityResolutionRequest struct {
	// 本波只接受 RESPONSIBILITY_ATTRIBUTE，其餘 chk_fcr_type 值回 9571。
	ResolutionType string `json:"resolutionType" binding:"required"`
	// maker 主張的成因：BUYER_CAUSED / SELLER_CAUSED / PROVIDER_CAUSED / PLATFORM_CAUSED。
	ResponsibilityCause string `json:"responsibilityCause" binding:"required"`
	// 提案理由（稽核必要，不得空白）。
	Reason string `json:"reason" binding:"required"`
}

// ProposeResponsibilityResolutionResp A-4 的回應。
type ProposeResponsibilityResolutionResp struct {
	// 新建提案的 TSID（字串）。覆核（A-5）時要帶回來。
	ResolutionId string `json:"resolutionId"`
	CaseNo       string `json:"caseNo"`
	// 案件在本次提案後的狀態（OPEN → IN_PROGRESS，承辦人同時被指派為提案者）。
	CaseStatus string `json:"caseStatus"`
	// server 算出的影響預覽：「元」的整數字串 ＋ ISO 8601 UTC。
	ImpactPreviewRefundAmount string `json:"impactPreviewRefundAmount"`
	ImpactComputedAt          string `json:"impactComputedAt"`
}

// ReviewResponsibilityResolutionRequest A-5 checker 覆核。
//
// 🔴 **只有兩個欄位**：覆核者能表達的只有「批准或駁回**這一筆提案**」。
//   - 沒有 responsibilityCause —— 取自 maker 的提案列（proposed_responsibility_cause）。
//     若成因在 approve 當下才第一次出現，雙人制保證的就只剩「有兩個人碰過這張單」，
//     而不是「第二個人批准了第一個人提的那一件事」：覆核者可以獨自決定不退款，
//     而帳面上雙人制完全成立。
//   - 沒有 orderNo／shipmentId／shippingMethod —— 取自建案當下凍結的案件列。由呼叫端
//     再傳一次，N17（unresolved）與 N16（finalized）就可以帶不同的 orderNo，而 orderNo
//     正是 order-service 那道狀態 CAS 的錨：對不上時 rows=0，全鏈路沒有任何錯誤訊息。
//   - 沒有 refundAmount —— 同 A-4，金額不由呼叫端決定。
//   - 沒有 reviewedByAccountId —— 覆核者取自 gRPC auth context。
//
// ⚠️ 加回任何一欄都會讓雙人制退化成「有兩個人碰過這張單」。
type ReviewResponsibilityResolutionRequest struct {
	// 🔴 只有 APPROVE / REJECT 兩值，大小寫敏感。
	Decision string `json:"decision" binding:"required"`
	// 覆核意見。APPROVE 時會寫進 case.resolutionNote（結案留痕）。
	ReviewComment string `json:"reviewComment"`
}

// ReviewResponsibilityResolutionResp A-5 的回應。
type ReviewResponsibilityResolutionResp struct {
	CaseNo       string `json:"caseNo"`
	ResolutionId string `json:"resolutionId"`
	// 提案的新狀態：APPROVED / REJECTED。
	ResolutionStatus string `json:"resolutionStatus"`
	// 案件的新狀態：APPROVE ⇒ RESOLVED；REJECT ⇒ 維持 IN_PROGRESS（可再提案）。
	CaseStatus string `json:"caseStatus"`
	// 🔴 只有 APPROVE 才有值 ＝ 寫進 case.resolutionCode 並成為 N16 的成因。
	// REJECT 時為空字串。
	ResponsibilityCause string `json:"responsibilityCause"`
	// 🔴 APPROVE 是否真的排出了 N16。
	// ⚠️ false 代表這一次是**重送**（案件或提案已 final ⇒ CAS rows=0 ⇒ 不發第二筆），
	// 那是冪等的正常結果**不是失敗**；前端不得把它當成「沒做到」而重試到底。
	N16Enqueued bool `json:"n16Enqueued"`
}

// ==================== 賣家面 handler ====================

// GetSellerLogisticsFinanceSummary 賣家物流財務總覽
//
// @Summary      賣家物流財務總覽
// @Description  賣家自己的物流費彙總：暫估/實際 provider 成本、被收的物流費、PayUni 手續費、平台補貼、責任與對帳調整、收款狀態分佈、收款殘差、貸記與應收餘額。sellerSubsidy 恆為 null（該維度的資料在訂單側，不在金流側）。
// @Tags         賣家物流財務 (Seller-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        created_from  query  string  false  "期間下界，ISO 8601 UTC"
// @Param        created_to    query  string  false  "期間上界，ISO 8601 UTC"
// @Success      200  {object}  object{code=int,message=string,data=SellerLogisticsFinanceSummaryResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /app/logistics_finance [get]
func (h *Holder) GetSellerLogisticsFinanceSummary(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	resp, err := h.Payment.GetSellerLogisticsFinanceSummary(c.Request.Context(),
		&pb.GetSellerLogisticsFinanceSummaryRequest{
			StoreId:     int64(storeID),
			CreatedFrom: c.Query("created_from"),
			CreatedTo:   c.Query("created_to"),
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	response.OK(c, sellerFinanceSummaryDTO(resp.GetData()))
}

// ListSellerLogisticsReceivables 賣家物流應收明細
//
// @Summary      賣家物流應收明細
// @Description  賣家自己的每一筆物流應收，含該筆的十一個財務維度。actualProviderCost / payuniCollectionFee / collectionResidual 為 null 代表「尚未對帳／尚未查詢」，不是 0。
// @Tags         賣家物流財務 (Seller-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page          query  string  false  "頁碼（整數字串）"  default(1)
// @Param        page_size     query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        status        query  string  false  "應收狀態，多值逗號分隔；合法值：PENDING / SUBMITTED / SETTLED / UNCONFIRMED / FAILED / CLOSED / WRITTEN_OFF"
// @Param        created_from  query  string  false  "期間下界，ISO 8601 UTC"
// @Param        created_to    query  string  false  "期間上界，ISO 8601 UTC"
// @Success      200  {object}  object{code=int,message=string,data=SellerLogisticsReceivableListResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/logistics_finance/receivables [get]
func (h *Holder) ListSellerLogisticsReceivables(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	resp, err := h.Payment.ListSellerLogisticsReceivables(c.Request.Context(),
		&pb.ListSellerLogisticsReceivablesRequest{
			StoreId:     int64(storeID),
			Page:        c.DefaultQuery("page", "1"),
			PageSize:    c.DefaultQuery("page_size", "20"),
			Status:      c.Query("status"),
			CreatedFrom: c.Query("created_from"),
			CreatedTo:   c.Query("created_to"),
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]SellerLogisticsReceivableResp, 0, len(d.GetRecords()))
	for _, r := range d.GetRecords() {
		records = append(records, sellerReceivableDTO(r))
	}
	response.Page(c, records, d.GetTotal(), d.GetPage(), d.GetPageSize())
}

// ListSellerLogisticsLedger 賣家物流帳本流水
//
// @Summary      賣家物流帳本流水
// @Description  賣家自己的雙分錄帳本（表頭 + 分錄）。amount 恆為正，借貸方向由 direction 表達，前端不得折成帶正負的單一數字。
// @Tags         賣家物流財務 (Seller-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page       query  string  false  "頁碼（整數字串）"  default(1)
// @Param        page_size  query  string  false  "每頁筆數，最大 100"  default(20)
// @Success      200  {object}  object{code=int,message=string,data=LogisticsLedgerListResp}
// @Failure      401  {object}  object{code=int,message=string}
// @Router       /app/logistics_finance/ledger [get]
func (h *Holder) ListSellerLogisticsLedger(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}
	resp, err := h.Payment.ListSellerLogisticsLedgerEntries(c.Request.Context(),
		&pb.ListSellerLogisticsLedgerEntriesRequest{
			StoreId:  int64(storeID),
			Page:     c.DefaultQuery("page", "1"),
			PageSize: c.DefaultQuery("page_size", "20"),
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]LogisticsLedgerTxnResp, 0, len(d.GetRecords()))
	for _, t := range d.GetRecords() {
		entries := make([]LogisticsLedgerEntryResp, 0, len(t.GetEntries()))
		for _, e := range t.GetEntries() {
			entries = append(entries, LogisticsLedgerEntryResp{
				LineSeq:     e.GetLineSeq(),
				AccountCode: e.GetAccountCode(),
				OwnerType:   e.GetOwnerType(),
				OwnerRefId:  e.GetOwnerRefId(),
				Direction:   e.GetDirection(),
				Amount:      e.GetAmount(),
				Memo:        e.GetMemo(),
			})
		}
		records = append(records, LogisticsLedgerTxnResp{
			TxnNo:            t.GetTxnNo(),
			TxnType:          t.GetTxnType(),
			OrderNo:          t.GetOrderNo(),
			SettlementPeriod: t.GetSettlementPeriod(),
			SourceType:       t.GetSourceType(),
			SourceRef:        t.GetSourceRef(),
			PostedAt:         t.GetPostedAt(),
			Entries:          entries,
		})
	}
	response.Page(c, records, d.GetTotal(), d.GetPage(), d.GetPageSize())
}

// ==================== Admin 面 handler ====================

// ListLogisticsReceivablesForAdmin Admin 物流應收列表
//
// @Summary      Admin 物流應收列表
// @Description  依賣家 / 狀態 / 物流商 / 帳齡篩選物流應收。min_aging_days 只計「尚未進終態」的列——已結清或已核銷的不算逾期。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page                   query  string  false  "頁碼"  default(1)
// @Param        page_size              query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        store_id               query  string  false  "賣家 store_id（非 account_id）"
// @Param        status                 query  string  false  "應收狀態，多值逗號分隔；合法值：PENDING / SUBMITTED / SETTLED / UNCONFIRMED / FAILED / CLOSED / WRITTEN_OFF"
// @Param        provider               query  string  false  "ShippingMethod value"
// @Param        order_no               query  string  false  "訂單號"
// @Param        provider_delivery_ref  query  string  false  "物流商配送單號（對帳關聯鍵）"
// @Param        created_from           query  string  false  "ISO 8601 UTC"
// @Param        created_to             query  string  false  "ISO 8601 UTC"
// @Param        min_aging_days         query  string  false  "只列尚未進終態且已存在 N 天以上者"
// @Param        unreconciled_only      query  string  false  "只列尚未對帳者；合法值：true / false"  default(false)
// @Success      200  {object}  object{code=int,message=string,data=AdminLogisticsReceivableListResp}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/logistics/receivables [get]
func (h *Holder) ListLogisticsReceivablesForAdmin(c *gin.Context) {
	resp, err := h.Payment.ListLogisticsReceivablesForAdmin(c.Request.Context(),
		&pb.ListLogisticsReceivablesForAdminRequest{
			Page:                c.DefaultQuery("page", "1"),
			PageSize:            c.DefaultQuery("page_size", "20"),
			StoreId:             c.Query("store_id"),
			Status:              c.Query("status"),
			Provider:            c.Query("provider"),
			OrderNo:             c.Query("order_no"),
			ProviderDeliveryRef: c.Query("provider_delivery_ref"),
			CreatedFrom:         c.Query("created_from"),
			CreatedTo:           c.Query("created_to"),
			MinAgingDays:        c.Query("min_aging_days"),
			UnreconciledOnly:    c.DefaultQuery("unreconciled_only", "false") == "true",
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminLogisticsReceivableResp, 0, len(d.GetRecords()))
	for _, r := range d.GetRecords() {
		records = append(records, adminReceivableDTO(r))
	}
	response.Page(c, records, d.GetTotal(), d.GetPage(), d.GetPageSize())
}

// GetLogisticsReceivableForAdmin Admin 物流應收詳情
//
// @Summary      Admin 物流應收詳情
// @Description  單筆應收 + 全部 PayUni 收款嘗試 + 調整單 + 該賣家的帳戶餘額。receivableBalance（賣家欠平台）與 creditBalance（平台欠賣家）是兩個科目，不得合成淨額顯示。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        receivable_no  path  string  true  "應收編號（TSID）"
// @Success      200  {object}  object{code=int,message=string,data=AdminLogisticsReceivableDetailResp}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/logistics/receivables/{receivable_no} [get]
func (h *Holder) GetLogisticsReceivableForAdmin(c *gin.Context) {
	receivableNo := c.Param("receivable_no")
	if receivableNo == "" {
		response.BadRequest(c, "receivable_no 為必填")
		return
	}
	resp, err := h.Payment.GetLogisticsReceivableForAdmin(c.Request.Context(),
		&pb.GetLogisticsReceivableForAdminRequest{ReceivableNo: receivableNo})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	collections := make([]AdminLogisticsCollectionResp, 0, len(d.GetCollections()))
	for _, a := range d.GetCollections() {
		collections = append(collections, adminCollectionDTO(a))
	}
	adjustments := make([]AdminLogisticsAdjustmentResp, 0, len(d.GetAdjustments()))
	for _, adj := range d.GetAdjustments() {
		adjustments = append(adjustments, adminAdjustmentDTO(adj))
	}
	acc := d.GetSellerAccount()
	response.OK(c, AdminLogisticsReceivableDetailResp{
		Receivable:  adminReceivableDTO(d.GetReceivable()),
		Collections: collections,
		Adjustments: adjustments,
		SellerAccount: AdminSellerLogisticsAccountResp{
			StoreId:           acc.GetStoreId(),
			ReceivableBalance: acc.GetReceivableBalance(),
			CreditBalance:     acc.GetCreditBalance(),
			LastActivityAt:    acc.GetLastActivityAt(),
			AgingDays:         acc.GetAgingDays(),
			Exists:            acc.GetExists(),
		},
	})
}

// ListLogisticsCollectionsForAdmin Admin PayUni 收款嘗試列表
//
// @Summary      Admin PayUni 收款嘗試列表
// @Description  每一次 agent_fee 送出與其撥款結果。netProceeds / residual / actualFee / fundDate 為 null 代表尚未查詢到結果，不是 0。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page                     query  string  false  "頁碼"  default(1)
// @Param        page_size                query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        receivable_no            query  string  false  "應收編號"
// @Param        trade_no                 query  string  false  "我方代理商自訂單號"
// @Param        store_id                 query  string  false  "賣家 store_id（非 account_id）"
// @Param        submit_status            query  string  false  "PayUni 送出狀態碼"
// @Param        pending_fund_date_only   query  string  false  "只列尚未查到撥款日者；合法值：true / false"  default(false)
// @Param        residual_only            query  string  false  "只列收款殘差不為 0 者；合法值：true / false"  default(false)
// @Param        created_from             query  string  false  "ISO 8601 UTC"
// @Param        created_to               query  string  false  "ISO 8601 UTC"
// @Success      200  {object}  object{code=int,message=string,data=AdminLogisticsCollectionListResp}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/logistics/collections [get]
func (h *Holder) ListLogisticsCollectionsForAdmin(c *gin.Context) {
	resp, err := h.Payment.ListLogisticsCollectionsForAdmin(c.Request.Context(),
		&pb.ListLogisticsCollectionsForAdminRequest{
			Page:                c.DefaultQuery("page", "1"),
			PageSize:            c.DefaultQuery("page_size", "20"),
			ReceivableNo:        c.Query("receivable_no"),
			TradeNo:             c.Query("trade_no"),
			StoreId:             c.Query("store_id"),
			SubmitStatus:        c.Query("submit_status"),
			PendingFundDateOnly: c.DefaultQuery("pending_fund_date_only", "false") == "true",
			ResidualOnly:        c.DefaultQuery("residual_only", "false") == "true",
			CreatedFrom:         c.Query("created_from"),
			CreatedTo:           c.Query("created_to"),
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminLogisticsCollectionResp, 0, len(d.GetRecords()))
	for _, a := range d.GetRecords() {
		records = append(records, adminCollectionDTO(a))
	}
	response.Page(c, records, d.GetTotal(), d.GetPage(), d.GetPageSize())
}

// ListLogisticsAdjustmentsForAdmin Admin 物流調整單列表
//
// @Summary      Admin 物流調整單列表
// @Description  責任歸屬（RESPONSIBILITY）與對帳差額（RECONCILIATION）兩類調整單。amount 允許負數。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page                  query  string  false  "頁碼"  default(1)
// @Param        page_size             query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        store_id              query  string  false  "賣家 store_id（非 account_id）"
// @Param        receivable_no         query  string  false  "應收編號"
// @Param        kind                  query  string  false  "合法值：RESPONSIBILITY / RECONCILIATION"
// @Param        responsibility_cause  query  string  false  "合法值：SELLER_CAUSED / BUYER_CAUSED / PLATFORM_CAUSED / PROVIDER_CAUSED"
// @Param        statement_line_id     query  string  false  "provider 帳單明細列 id"
// @Param        created_from          query  string  false  "ISO 8601 UTC"
// @Param        created_to            query  string  false  "ISO 8601 UTC"
// @Success      200  {object}  object{code=int,message=string,data=AdminLogisticsAdjustmentListResp}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/logistics/adjustments [get]
func (h *Holder) ListLogisticsAdjustmentsForAdmin(c *gin.Context) {
	resp, err := h.Payment.ListLogisticsAdjustmentsForAdmin(c.Request.Context(),
		&pb.ListLogisticsAdjustmentsForAdminRequest{
			Page:                c.DefaultQuery("page", "1"),
			PageSize:            c.DefaultQuery("page_size", "20"),
			StoreId:             c.Query("store_id"),
			ReceivableNo:        c.Query("receivable_no"),
			Kind:                c.Query("kind"),
			ResponsibilityCause: c.Query("responsibility_cause"),
			StatementLineId:     c.Query("statement_line_id"),
			CreatedFrom:         c.Query("created_from"),
			CreatedTo:           c.Query("created_to"),
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminLogisticsAdjustmentResp, 0, len(d.GetRecords()))
	for _, adj := range d.GetRecords() {
		records = append(records, adminAdjustmentDTO(adj))
	}
	response.Page(c, records, d.GetTotal(), d.GetPage(), d.GetPageSize())
}

// ListProviderStatementsForAdmin Admin 物流商帳單列表
//
// @Summary      Admin 物流商帳單列表
// @Description  一份帳單一列，含比對狀態分佈。unmatchedCount = 尚未比對；mismatchCount 是推導值（該列有對帳調整單），不是 reconStatus 的碼值。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page              query  string  false  "頁碼"  default(1)
// @Param        page_size         query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        provider          query  string  false  "ShippingMethod value（3003 / 3004）"
// @Param        statement_period  query  string  false  "帳單期別，格式 YYYY-MM"
// @Param        unsettled_only    query  string  false  "只列仍有未比對或爭議明細者；合法值：true / false"  default(false)
// @Success      200  {object}  object{code=int,message=string,data=AdminProviderStatementListResp}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/logistics/provider_statements [get]
func (h *Holder) ListProviderStatementsForAdmin(c *gin.Context) {
	resp, err := h.Payment.ListProviderStatementsForAdmin(c.Request.Context(),
		&pb.ListProviderStatementsForAdminRequest{
			Page:            c.DefaultQuery("page", "1"),
			PageSize:        c.DefaultQuery("page_size", "20"),
			Provider:        c.Query("provider"),
			StatementPeriod: c.Query("statement_period"),
			UnsettledOnly:   c.DefaultQuery("unsettled_only", "false") == "true",
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminProviderStatementResp, 0, len(d.GetRecords()))
	for _, s := range d.GetRecords() {
		records = append(records, adminStatementDTO(s))
	}
	response.Page(c, records, d.GetTotal(), d.GetPage(), d.GetPageSize())
}

// GetProviderStatementForAdmin Admin 物流商帳單詳情
//
// @Summary      Admin 物流商帳單詳情
// @Description  單份帳單的全部明細列與比對狀態。provider 為必填 query——帳單編號只在同一個物流商內唯一，不帶會跨物流商撈到別人的帳單。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        statement_no  path   string  true  "帳單編號（statement_ref）"
// @Param        provider      query  string  true  "ShippingMethod value（3003 / 3004）"
// @Success      200  {object}  object{code=int,message=string,data=AdminProviderStatementDetailResp}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Router       /admin/logistics/provider_statements/{statement_no} [get]
func (h *Holder) GetProviderStatementForAdmin(c *gin.Context) {
	statementRef := c.Param("statement_no")
	if statementRef == "" {
		response.BadRequest(c, "statement_no 為必填")
		return
	}
	provider := c.Query("provider")
	if provider == "" {
		// 🔴 不給預設值：statement_ref 只在同一個 provider 內唯一（uq_provsettle_line 的第一欄
		// 就是 provider），猜一個 provider 會讓 ADM 看到別家物流商的帳單而毫無察覺。
		response.BadRequest(c, "provider 為必填（帳單編號只在同一物流商內唯一）")
		return
	}
	resp, err := h.Payment.GetProviderStatementForAdmin(c.Request.Context(),
		&pb.GetProviderStatementForAdminRequest{StatementRef: statementRef, Provider: provider})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	lines := make([]AdminProviderSettlementLineResp, 0, len(d.GetLines()))
	for _, l := range d.GetLines() {
		lines = append(lines, AdminProviderSettlementLineResp{
			LineId:                         l.GetLineId(),
			Provider:                       l.GetProvider(),
			StatementRef:                   l.GetStatementRef(),
			StatementPeriod:                l.GetStatementPeriod(),
			ProviderShipmentRef:            l.GetProviderShipmentRef(),
			OrderNo:                        l.GetOrderNo(),
			FeeType:                        l.GetFeeType(),
			Amount:                         l.GetAmount(),
			ReconStatus:                    l.GetReconStatus(),
			LedgerTxnNo:                    l.GetLedgerTxnNo(),
			ExceptionCaseId:                l.GetExceptionCaseId(),
			ImportedBy:                     l.GetImportedBy(),
			ImportedAt:                     l.GetImportedAt(),
			HasReconciliationAdjustment:    l.GetHasReconciliationAdjustment(),
			ReconciliationAdjustmentAmount: l.GetReconciliationAdjustmentAmount(),
		})
	}
	response.OK(c, AdminProviderStatementDetailResp{
		Statement:           adminStatementDTO(d.GetStatement()),
		Lines:               lines,
		AwaitingReviewCount: d.GetAwaitingReviewCount(),
	})
}

// ListFulfillmentExceptionCasesForAdmin Admin 履約例外案件列表
//
// @Summary      Admin 履約例外案件列表
// @Description  維運例外案件佇列，含人工覆核提案（resolutions）。awaiting_review_only=true 只列尚有待覆核提案者。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page                  query  string  false  "頁碼"  default(1)
// @Param        page_size             query  string  false  "每頁筆數，最大 100"  default(20)
// @Param        status                query  string  false  "案件狀態，多值逗號分隔；合法值：OPEN / IN_PROGRESS / RESOLVED / DISMISSED"
// @Param        severity              query  string  false  "合法值：P0 / P1 / P2"
// @Param        category              query  string  false  "案件分類碼"
// @Param        subject_type          query  string  false  "合法值：ORDER / SHIPMENT / SHIPMENT_BATCH / CONFIG"
// @Param        subject_ref           query  string  false  "案件主體識別碼"
// @Param        store_id              query  string  false  "賣家 store_id（非 account_id）"
// @Param        awaiting_review_only  query  string  false  "只列尚有待覆核提案者；合法值：true / false"  default(false)
// @Success      200  {object}  object{code=int,message=string,data=AdminFulfillmentExceptionCaseListResp}
// @Failure      403  {object}  object{code=int,message=string}
// @Router       /admin/logistics/exception_cases [get]
func (h *Holder) ListFulfillmentExceptionCasesForAdmin(c *gin.Context) {
	resp, err := h.Payment.ListFulfillmentExceptionCasesForAdmin(c.Request.Context(),
		&pb.ListFulfillmentExceptionCasesForAdminRequest{
			Page:               c.DefaultQuery("page", "1"),
			PageSize:           c.DefaultQuery("page_size", "20"),
			Status:             c.Query("status"),
			Severity:           c.Query("severity"),
			Category:           c.Query("category"),
			SubjectType:        c.Query("subject_type"),
			SubjectRef:         c.Query("subject_ref"),
			StoreId:            c.Query("store_id"),
			AwaitingReviewOnly: c.DefaultQuery("awaiting_review_only", "false") == "true",
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	records := make([]AdminFulfillmentExceptionCaseResp, 0, len(d.GetRecords()))
	for _, cse := range d.GetRecords() {
		// 🔴 List 與 Detail 共用同一支轉換：兩邊各寫一份的話，責任軸那三欄補在一邊
		// 另一邊就會靜默少欄——而少欄的症狀是前端拿到 undefined，沒有任何東西會轉紅。
		records = append(records, adminFulfillmentExceptionCaseDTO(cse))
	}
	response.Page(c, records, d.GetTotal(), d.GetPage(), d.GetPageSize())
}

// GetFulfillmentExceptionCaseForAdmin Admin 履約例外案件詳情
//
// @Summary      Admin 履約例外案件詳情
// @Description  單一例外案件的完整決策脈絡：建案當下凍結的事件 context（orderNo / shipmentId / shippingMethod）、提案歷程、maker 主張的成因，以及提案當下 server 算出的影響預覽快照。與列表分開是刻意的——列表是佇列視圖（分頁、篩選），本支是責任判定的決策依據。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        case_no  path  string  true  "案件編號（case_no 業務單號，不是 case_id）"
// @Success      200  {object}  object{code=int,message=string,data=AdminFulfillmentExceptionCaseResp}
// @Failure      400  {object}  object{code=int,message=string}  "case_no 缺漏"
// @Failure      403  {object}  object{code=int,message=string}  "非管理員"
// @Failure      404  {object}  object{code=int,message=string}  "9570 案件不存在"
// @Router       /admin/logistics/exception_cases/{case_no} [get]
func (h *Holder) GetFulfillmentExceptionCaseForAdmin(c *gin.Context) {
	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}
	resp, err := h.Payment.GetFulfillmentExceptionCaseForAdmin(c.Request.Context(),
		&pb.GetFulfillmentExceptionCaseForAdminRequest{CaseNo: caseNo})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	response.OK(c, adminFulfillmentExceptionCaseDTO(resp.GetData()))
}

// ProposeResponsibilityResolutionForAdmin Admin 提出責任判定提案（A-4 maker）
//
// @Summary      Admin 提出責任判定提案
// @Description  maker 對一張履約例外案件主張責任成因並提出處置提案。影響預覽（退款金額）由後端依成因與訂單實付金額算出後持久化，**不接受呼叫端給金額**；提案者身分取自登入 token，**不接受 request 指定**。案件狀態 OPEN → IN_PROGRESS，承辦人同時被指派為提案者。本波只支援 resolutionType=RESPONSIBILITY_ATTRIBUTE，其餘型別 fail-closed 回 9571。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string                                  true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$。重送同一鍵回放首次結果，避免多開一張提案"
// @Param        case_no          path    string                                  true  "案件編號"
// @Param        body             body    ProposeResponsibilityResolutionRequest  true  "提案內容"
// @Success      200  {object}  object{code=int,message=string,data=ProposeResponsibilityResolutionResp}
// @Failure      400  {object}  object{code=int,message=string}  "欄位缺漏或 Idempotency-Key 不合法"
// @Failure      403  {object}  object{code=int,message=string}  "非管理員"
// @Failure      404  {object}  object{code=int,message=string}  "9570 案件不存在"
// @Failure      409  {object}  object{code=int,message=string}  "9571 案件目前狀態不允許此操作（含本波不支援的 resolutionType）"
// @Router       /admin/logistics/exception_cases/{case_no}/resolutions [post]
func (h *Holder) ProposeResponsibilityResolutionForAdmin(c *gin.Context) {
	caseNo := c.Param("case_no")
	if caseNo == "" {
		response.BadRequest(c, "case_no is required")
		return
	}
	var req ProposeResponsibilityResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.Payment.ProposeResponsibilityResolutionForAdmin(c.Request.Context(),
		&pb.ProposeResponsibilityResolutionForAdminRequest{
			CaseNo:              caseNo,
			ResolutionType:      req.ResolutionType,
			ResponsibilityCause: req.ResponsibilityCause,
			Reason:              req.Reason,
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	response.OK(c, ProposeResponsibilityResolutionResp{
		ResolutionId:              d.GetResolutionId(),
		CaseNo:                    d.GetCaseNo(),
		CaseStatus:                d.GetCaseStatus(),
		ImpactPreviewRefundAmount: d.GetImpactPreviewRefundAmount(),
		ImpactComputedAt:          d.GetImpactComputedAt(),
	})
}

// ReviewResponsibilityResolutionForAdmin Admin 覆核責任判定提案（A-5 checker）
//
// @Summary      Admin 覆核責任判定提案
// @Description  checker 批准或駁回一筆責任判定提案。APPROVE ⇒ 提案的成因原封不動成為案件結論並排入 N16 事件（訂單據此離開 SHIPPING_ABNORMAL）；REJECT ⇒ 案件維持 IN_PROGRESS 可再提案。**成因、事件 context 與覆核者身分都不由 request 指定**：分別取自 maker 的提案列、建案當下凍結的案件列與登入 token。覆核者不得為提案者（9573）。
// @Tags         Admin 物流財務 (Admin-Logistics-Finance)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key  header  string                                 true  "冪等鍵，^[A-Za-z0-9._:-]{8,128}$。重送同一鍵回放首次結果，避免重複覆核"
// @Param        case_no          path    string                                 true  "案件編號"
// @Param        resolution_id    path    string                                 true  "提案 ID（A-4 回傳的 resolutionId）"
// @Param        body             body    ReviewResponsibilityResolutionRequest  true  "覆核決定"
// @Success      200  {object}  object{code=int,message=string,data=ReviewResponsibilityResolutionResp}
// @Failure      400  {object}  object{code=int,message=string}  "decision 不是 APPROVE/REJECT，或 Idempotency-Key 不合法"
// @Failure      403  {object}  object{code=int,message=string}  "非管理員；9573 覆核者不得為提案者"
// @Failure      404  {object}  object{code=int,message=string}  "9570 案件不存在；9572 處置提案不存在"
// @Failure      409  {object}  object{code=int,message=string}  "9571 案件狀態不允許；9574 本提案已完成覆核；9575 影響預覽已過期，請重新產生後再覆核"
// @Router       /admin/logistics/exception_cases/{case_no}/resolutions/{resolution_id}/approvals [post]
func (h *Holder) ReviewResponsibilityResolutionForAdmin(c *gin.Context) {
	caseNo := c.Param("case_no")
	resolutionID := c.Param("resolution_id")
	if caseNo == "" || resolutionID == "" {
		response.BadRequest(c, "case_no and resolution_id are required")
		return
	}
	var req ReviewResponsibilityResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// 🔴 值域在 gateway 驗（與 resolutionType／responsibilityCause 的處置刻意不同）：
	// decision 不是碼表，它是**本端點的動作枚舉**，兩個值寫死在 proto 契約裡、不會隨
	// CodeMeta 成長，所以這裡不存在「複製一份會漂移」的問題。
	//
	// 而放行未知值的後果不對稱：一個沒被任何人核准的提案，只要下游哪天把 decision 的
	// 比較寫成「不是 REJECT 就當 APPROVE」，就會被當成批准送進 N16——那是一筆買家收不到
	// 退款的終局事件，且事後沒有任何東西看得出來它不是人批的。fail-closed 在這裡。
	switch req.Decision {
	case responsibilityDecisionApprove, responsibilityDecisionReject:
	default:
		response.BadRequest(c, "decision must be "+responsibilityDecisionApprove+" or "+responsibilityDecisionReject)
		return
	}
	resp, err := h.Payment.ReviewResponsibilityResolutionForAdmin(c.Request.Context(),
		&pb.ReviewResponsibilityResolutionForAdminRequest{
			CaseNo:        caseNo,
			ResolutionId:  resolutionID,
			Decision:      req.Decision,
			ReviewComment: req.ReviewComment,
		})
	if err != nil {
		response.GrpcErr(c, "payment", err)
		return
	}
	if resp.GetCode() != 0 {
		response.GrpcResult(c, resp.GetCode(), resp.GetMessage(), nil)
		return
	}
	d := resp.GetData()
	response.OK(c, ReviewResponsibilityResolutionResp{
		CaseNo:              d.GetCaseNo(),
		ResolutionId:        d.GetResolutionId(),
		ResolutionStatus:    d.GetResolutionStatus(),
		CaseStatus:          d.GetCaseStatus(),
		ResponsibilityCause: d.GetResponsibilityCause(),
		N16Enqueued:         d.GetN16Enqueued(),
	})
}

// ==================== DTO 轉換 ====================

// optStr 把 proto3 optional 轉成 *string。
//
// 🔴 未設值一律回 nil（JSON null），**不得**回 "" 或 "0"：呼叫端要分得出
// 「還沒算出來」與「算出來是 0」，那兩件事的後續處置完全不同。
func optStr(has bool, v string) *string {
	if !has {
		return nil
	}
	out := v
	return &out
}

// adminFulfillmentCaseResolutionDTO 一張處置提案 proto → DTO。
func adminFulfillmentCaseResolutionDTO(r *pb.AdminFulfillmentCaseResolutionRecord) AdminFulfillmentCaseResolutionResp {
	return AdminFulfillmentCaseResolutionResp{
		ResolutionId:                r.GetResolutionId(),
		ResolutionType:              r.GetResolutionType(),
		Reason:                      r.GetReason(),
		Status:                      r.GetStatus(),
		RequiresReview:              r.GetRequiresReview(),
		ProposedByAccountId:         r.GetProposedByAccountId(),
		ProposedAt:                  r.GetProposedAt(),
		ReviewedByAccountId:         r.GetReviewedByAccountId(),
		ReviewComment:               r.GetReviewComment(),
		ReviewedAt:                  r.GetReviewedAt(),
		ProposedResponsibilityCause: r.GetProposedResponsibilityCause(),
		ImpactPreviewRefundAmount:   r.GetImpactPreviewRefundAmount(),
		ImpactComputedAt:            r.GetImpactComputedAt(),
	}
}

// adminFulfillmentExceptionCaseDTO 一張例外案件 proto → DTO。
//
// 🔴 **List（A-1）與 Detail（A-2）唯一的轉換點**。兩支端點回的是同一個 proto record，
// 各自 inline 組一次的話，補欄位時漏掉一邊不會編譯錯、不會讓任何測試轉紅——症狀只是
// 前端在某一個畫面拿到 undefined。
func adminFulfillmentExceptionCaseDTO(cse *pb.AdminFulfillmentExceptionCaseRecord) AdminFulfillmentExceptionCaseResp {
	resolutions := make([]AdminFulfillmentCaseResolutionResp, 0, len(cse.GetResolutions()))
	for _, r := range cse.GetResolutions() {
		resolutions = append(resolutions, adminFulfillmentCaseResolutionDTO(r))
	}
	return AdminFulfillmentExceptionCaseResp{
		CaseNo:              cse.GetCaseNo(),
		CaseId:              cse.GetCaseId(),
		SourceService:       cse.GetSourceService(),
		Category:            cse.GetCategory(),
		Severity:            cse.GetSeverity(),
		SubjectType:         cse.GetSubjectType(),
		SubjectRef:          cse.GetSubjectRef(),
		StoreId:             cse.GetStoreId(),
		Status:              cse.GetStatus(),
		StatusEnteredAt:     cse.GetStatusEnteredAt(),
		OwnerAccountId:      cse.GetOwnerAccountId(),
		OwnerAssignedAt:     cse.GetOwnerAssignedAt(),
		ResolutionCode:      cse.GetResolutionCode(),
		ResolutionNote:      cse.GetResolutionNote(),
		ResolvedByAccountId: cse.GetResolvedByAccountId(),
		ResolvedAt:          cse.GetResolvedAt(),
		OccurredAt:          cse.GetOccurredAt(),
		CreatedAt:           cse.GetCreatedAt(),
		OrderNo:             cse.GetOrderNo(),
		ShipmentId:          cse.GetShipmentId(),
		ShippingMethod:      cse.GetShippingMethod(),
		Resolutions:         resolutions,
	}
}

func sellerFinanceSummaryDTO(s *pb.SellerLogisticsFinanceSummary) SellerLogisticsFinanceSummaryResp {
	counts := make([]LogisticsCollectionStatusCountResp, 0, len(s.GetCollectionStatusCounts()))
	for _, c := range s.GetCollectionStatusCounts() {
		counts = append(counts, LogisticsCollectionStatusCountResp{
			Status:      c.GetStatus(),
			StatusLabel: c.GetStatusLabel(),
			Count:       c.GetCount(),
		})
	}
	return SellerLogisticsFinanceSummaryResp{
		StoreId:               s.GetStoreId(),
		CreatedFrom:           s.GetCreatedFrom(),
		CreatedTo:             s.GetCreatedTo(),
		EstimatedProviderCost: s.GetEstimatedProviderCost(),
		ActualProviderCost:    s.GetActualProviderCost(),
		UnreconciledCount:     s.GetUnreconciledCount(),
		SellerLogisticsCharge: s.GetSellerLogisticsCharge(),
		PayuniCollectionFee:   s.GetPayuniCollectionFee(),
		PendingFeeQueryCount:  s.GetPendingFeeQueryCount(),
		PlatformSubsidy:       s.GetPlatformSubsidy(),
		// 🔴 上游永遠不設值 ⇒ 這裡永遠是 null。不要「順手」填 0。
		SellerSubsidy:            optStr(s.SellerSubsidy != nil, s.GetSellerSubsidy()),
		ResponsibilityAdjustment: s.GetResponsibilityAdjustment(),
		ReconciliationAdjustment: s.GetReconciliationAdjustment(),
		CollectionStatusCounts:   counts,
		CollectionResidual:       s.GetCollectionResidual(),
		SellerCredit:             s.GetSellerCredit(),
		ReceivableBalance:        s.GetReceivableBalance(),
		LastActivityAt:           s.GetLastActivityAt(),
		AgingDays:                s.GetAgingDays(),
		HasAccount:               s.GetHasAccount(),
	}
}

func sellerReceivableDTO(r *pb.SellerLogisticsReceivableRecord) SellerLogisticsReceivableResp {
	return SellerLogisticsReceivableResp{
		ReceivableNo:             r.GetReceivableNo(),
		OrderNo:                  r.GetOrderNo(),
		ShipmentId:               r.GetShipmentId(),
		Provider:                 r.GetProvider(),
		ProviderDeliveryRef:      r.GetProviderDeliveryRef(),
		EstimatedProviderCost:    r.GetEstimatedProviderCost(),
		ActualProviderCost:       optStr(r.ActualProviderCost != nil, r.GetActualProviderCost()),
		SellerLogisticsCharge:    r.GetSellerLogisticsCharge(),
		PayuniCollectionFee:      optStr(r.PayuniCollectionFee != nil, r.GetPayuniCollectionFee()),
		PlatformSubsidy:          r.GetPlatformSubsidy(),
		SellerSubsidy:            optStr(r.SellerSubsidy != nil, r.GetSellerSubsidy()),
		ResponsibilityAdjustment: r.GetResponsibilityAdjustment(),
		ReconciliationAdjustment: r.GetReconciliationAdjustment(),
		CollectionStatus:         r.GetCollectionStatus(),
		CollectionStatusLabel:    r.GetCollectionStatusLabel(),
		CollectionResidual:       optStr(r.CollectionResidual != nil, r.GetCollectionResidual()),
		SellerCredit:             r.GetSellerCredit(),
		CarriedResidual:          r.GetCarriedResidual(),
		ChargeBasisFrozenAt:      r.GetChargeBasisFrozenAt(),
		InvoiceDocumentId:        r.GetInvoiceDocumentId(),
		StatementLineId:          r.GetStatementLineId(),
		CreatedAt:                r.GetCreatedAt(),
		UpdatedAt:                r.GetUpdatedAt(),
		AgingDays:                r.GetAgingDays(),
	}
}

func adminReceivableDTO(r *pb.AdminLogisticsReceivableSummary) AdminLogisticsReceivableResp {
	return AdminLogisticsReceivableResp{
		ReceivableNo:        r.GetReceivableNo(),
		StoreId:             r.GetStoreId(),
		OrderNo:             r.GetOrderNo(),
		ShipmentId:          r.GetShipmentId(),
		Provider:            r.GetProvider(),
		ProviderDeliveryRef: r.GetProviderDeliveryRef(),
		EstimatedCost:       r.GetEstimatedCost(),
		ActualCost:          optStr(r.ActualCost != nil, r.GetActualCost()),
		PlatformSubsidy:     r.GetPlatformSubsidy(),
		TargetNet:           r.GetTargetNet(),
		CarriedResidual:     r.GetCarriedResidual(),
		Status:              r.GetStatus(),
		StatusLabel:         r.GetStatusLabel(),
		StatementLineId:     r.GetStatementLineId(),
		InvoiceDocumentId:   r.GetInvoiceDocumentId(),
		ChargeBasisFrozenAt: r.GetChargeBasisFrozenAt(),
		CreatedAt:           r.GetCreatedAt(),
		UpdatedAt:           r.GetUpdatedAt(),
		AgingDays:           r.GetAgingDays(),
		SubmitLeaseUntil:    r.GetSubmitLeaseUntil(),
	}
}

func adminCollectionDTO(a *pb.AdminLogisticsCollectionRecord) AdminLogisticsCollectionResp {
	return AdminLogisticsCollectionResp{
		CollectionNo:     a.GetCollectionNo(),
		ReceivableNo:     a.GetReceivableNo(),
		StoreId:          a.GetStoreId(),
		TradeNo:          a.GetTradeNo(),
		GrossAmount:      a.GetGrossAmount(),
		NetProceeds:      optStr(a.NetProceeds != nil, a.GetNetProceeds()),
		Residual:         optStr(a.Residual != nil, a.GetResidual()),
		ActualFee:        optStr(a.ActualFee != nil, a.GetActualFee()),
		FundDate:         optStr(a.FundDate != nil, a.GetFundDate()),
		SubmitStatus:     a.GetSubmitStatus(),
		SubmitMessage:    a.GetSubmitMessage(),
		SubmittedAt:      a.GetSubmittedAt(),
		ExpectedFundDate: a.GetExpectedFundDate(),
		QueryLeaseUntil:  a.GetQueryLeaseUntil(),
		CreatedAt:        a.GetCreatedAt(),
		UpdatedAt:        a.GetUpdatedAt(),
	}
}

func adminAdjustmentDTO(adj *pb.AdminLogisticsAdjustmentRecord) AdminLogisticsAdjustmentResp {
	return AdminLogisticsAdjustmentResp{
		AdjustmentNo:        adj.GetAdjustmentNo(),
		ReceivableNo:        adj.GetReceivableNo(),
		StoreId:             adj.GetStoreId(),
		ProviderDeliveryRef: adj.GetProviderDeliveryRef(),
		Kind:                adj.GetKind(),
		Amount:              adj.GetAmount(),
		ResponsibilityCause: adj.GetResponsibilityCause(),
		StatementLineId:     adj.GetStatementLineId(),
		CreatedBy:           adj.GetCreatedBy(),
		Reason:              adj.GetReason(),
		CreatedAt:           adj.GetCreatedAt(),
	}
}

func adminStatementDTO(s *pb.AdminProviderStatementSummary) AdminProviderStatementResp {
	return AdminProviderStatementResp{
		Provider:        s.GetProvider(),
		StatementRef:    s.GetStatementRef(),
		StatementPeriod: s.GetStatementPeriod(),
		LineCount:       s.GetLineCount(),
		TotalAmount:     s.GetTotalAmount(),
		MatchedCount:    s.GetMatchedCount(),
		UnmatchedCount:  s.GetUnmatchedCount(),
		DisputedCount:   s.GetDisputedCount(),
		WrittenOffCount: s.GetWrittenOffCount(),
		MismatchCount:   s.GetMismatchCount(),
		MismatchAmount:  s.GetMismatchAmount(),
		FirstImportedAt: s.GetFirstImportedAt(),
		LastImportedAt:  s.GetLastImportedAt(),
	}
}

package handler

import (
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"

	"github.com/Will413028/hawkr/gateway/internal/response"
)

// 本檔是 58 號《電子發票服務》§6.1 對外 REST 的 DTO 層。
//
// 為什麼每個 endpoint 都要有自己的 Gateway DTO、不能把 proto struct 直接放進 `data`：
// protoc-gen-go 產生的 tag 是 `json:"foo_bar,omitempty"` ⇒ 直出等於欄名 snake_case
// ＋ 任何零值（false／""／0）整個欄位從回應消失，前端拿到 undefined（§6.1.0(7)、
// 10 檔 §2）。本檔的 DTO 一律不帶 omitempty，「data key 必須存在」才成立。
//
// 型別規約（§6.1.0(8)）：ID、金額、計數、版本、頁碼一律 string；時間 string 且為
// ISO 8601 UTC 秒精度。proto 側已全部宣告成 string，這裡逐欄照抄不做二次轉型。

// ==================== 回應 payload DTO（§6.1.0(15) 的 Success `data` 欄）====================

type ActOnInvoiceComplaintForAdminData struct {
	ComplaintId    string                   `json:"complaintId"`
	Status         string                   `json:"status"`
	Version        string                   `json:"version"`
	Recommendation SuspensionRecommendation `json:"recommendation"`
	AuditLogId     string                   `json:"auditLogId"`
}

type AdminInvoiceDocumentDetail struct {
	SellerView        SellerInvoiceDocumentDetail `json:"sellerView"`
	ProviderExchanges []ProviderExchangeItem      `json:"providerExchanges"`
	IdempotencyKey    string                      `json:"idempotencyKey"`
	RelatedJobs       []InvoiceJobInfo            `json:"relatedJobs"`
	// DocumentVersion 是 invoice_documents.version，端點 73 的 expectedVersion 取它。
	DocumentVersion string `json:"documentVersion"`
	// CanResumeFromManual ＝ 可以按「送回自動線」（端點 73）。判準由後端一支述詞決定，
	// 前端**不得自行從 status 推導**——那會讓按鈕亮在一個 CAS 必定 rows=0 的狀態上。
	CanResumeFromManual bool `json:"canResumeFromManual"`
}

type AdminInvoiceJobDetail struct {
	Job               InvoiceJobInfo         `json:"job"`
	OrderSnapshotJson string                 `json:"orderSnapshotJson"`
	AmountBreakdown   InvoiceAmountBreakdown `json:"amountBreakdown"`
	ProviderExchanges []ProviderExchangeItem `json:"providerExchanges"`
	IdempotencyKey    string                 `json:"idempotencyKey"`
	ShipmentStatus    string                 `json:"shipmentStatus"`
	// shipmentStatusDegraded ＝ 這一筆的 shipmentStatus 沒有取到（shipping-service
	// 不可用）。true ⇒ 空字串是「查不到」不是「還沒出貨」（#3089 第 7 項）。
	ShipmentStatusDegraded bool                  `json:"shipmentStatusDegraded"`
	AuditLogs              []InvoiceAuditLogItem `json:"auditLogs"`
	ManualUnlock           ManualUnlockInfo      `json:"manualUnlock"`
}

type BuyerInvoiceDocumentItem struct {
	Summary    InvoiceDocumentSummary `json:"summary"`
	IssuerName string                 `json:"issuerName"`
}

type ComplaintResponseItem struct {
	ResponseId string `json:"responseId"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	Note       string `json:"note"`
	OccurredAt string `json:"occurredAt"`
}

type CreateManualIssuanceForAdminData struct {
	JobId      string `json:"jobId"`
	DocumentId string `json:"documentId"`
	Status     string `json:"status"`
	AuditLogId string `json:"auditLogId"`
}

type DeactivateProviderAccountForAdminData struct {
	StoreId             string `json:"storeId"`
	Status              string `json:"status"`
	DeactivatedAt       string `json:"deactivatedAt"`
	AffectedScopeNotice string `json:"affectedScopeNotice"`
	PendingOrderCount   string `json:"pendingOrderCount"`
	AuditLogId          string `json:"auditLogId"`
}

type DecideTaxReviewForAdminData struct {
	StoreId            string `json:"storeId"`
	ReviewStatus       string `json:"reviewStatus"`
	Version            string `json:"version"`
	AuditLogId         string `json:"auditLogId"`
	RevokedEffectiveAt string `json:"revokedEffectiveAt"`
}

type GetInvoiceOverviewForAdminData struct {
	Kpis         []InvoiceKpiItem `json:"kpis"`
	PriorityJobs []InvoiceJobInfo `json:"priorityJobs"`
}

type InvoiceAmountBreakdown struct {
	ProductAmount  string `json:"productAmount"`
	ShippingAmount string `json:"shippingAmount"`
	DiscountAmount string `json:"discountAmount"`
	InvoiceAmount  string `json:"invoiceAmount"`
	TaxCategory    string `json:"taxCategory"`
}

type InvoiceAuditLogItem struct {
	AuditLogId       string `json:"auditLogId"`
	OccurredAt       string `json:"occurredAt"`
	ActorAccountId   string `json:"actorAccountId"`
	ActorDisplayName string `json:"actorDisplayName"`
	OperationType    string `json:"operationType"`
	TargetKind       string `json:"targetKind"`
	TargetId         string `json:"targetId"`
	BeforeJson       string `json:"beforeJson"`
	AfterJson        string `json:"afterJson"`
	Reason           string `json:"reason"`
	RequestId        string `json:"requestId"`
	TermsVersion     string `json:"termsVersion"`
	Result           string `json:"result"`
}

type InvoiceComplaintInfo struct {
	ComplaintId           string                  `json:"complaintId"`
	CaseNo                string                  `json:"caseNo"`
	OrderNo               string                  `json:"orderNo"`
	StoreId               string                  `json:"storeId"`
	Status                string                  `json:"status"`
	ExpectedDocKind       string                  `json:"expectedDocKind"`
	DueAt                 string                  `json:"dueAt"`
	RemainingBusinessDays string                  `json:"remainingBusinessDays"`
	OverdueBusinessDays   string                  `json:"overdueBusinessDays"`
	BuyerMaskedName       string                  `json:"buyerMaskedName"`
	Description           string                  `json:"description"`
	Responses             []ComplaintResponseItem `json:"responses"`
	Version               string                  `json:"version"`
	CreatedAt             string                  `json:"createdAt"`
	ReopenCount           string                  `json:"reopenCount"`
	BuyerReviewDueAt      string                  `json:"buyerReviewDueAt"`
	ReopenAllowedAfter    string                  `json:"reopenAllowedAfter"`
}

type InvoiceCorrectionData struct {
	CorrectionId     string `json:"correctionId"`
	HandlingMode     string `json:"handlingMode"`
	Status           string `json:"status"`
	StatusNotice     string `json:"statusNotice"`
	CreatedAt        string `json:"createdAt"`
	RejectReasonCode string `json:"rejectReasonCode"`
}

type InvoiceDocumentDetail struct {
	Summary        InvoiceDocumentSummary `json:"summary"`
	IssuerName     string                 `json:"issuerName"`
	IssuerTaxId    string                 `json:"issuerTaxId"`
	RecipientName  string                 `json:"recipientName"`
	RecipientTaxId string                 `json:"recipientTaxId"`
	SalesAmount    string                 `json:"salesAmount"`
	TaxAmount      string                 `json:"taxAmount"`
	TotalAmount    string                 `json:"totalAmount"`
	TaxCategory    string                 `json:"taxCategory"`
	Lines          []InvoiceLine          `json:"lines"`
	Timeline       []InvoiceTimelineItem  `json:"timeline"`
	InvoiceFileKey string                 `json:"invoiceFileKey"`
}

type InvoiceDocumentSummary struct {
	DocumentId     string `json:"documentId"`
	DocumentKind   string `json:"documentKind"`
	Status         string `json:"status"`
	InvoiceNumber  string `json:"invoiceNumber"`
	IssuedAt       string `json:"issuedAt"`
	TotalAmount    string `json:"totalAmount"`
	CarrierSummary string `json:"carrierSummary"`
	// CarrierType 是 CarrierSummary 的判別欄（#3084）：
	// 非空（MOBILE_BARCODE／CITIZEN_CERT）⇒ summary 裝的是載具，標籤用「載具」；
	// 空 ＋ summary 非空 ⇒ 裝的是買受人統編，標籤用「買受人統編」；
	// 空 ＋ summary 為空 ⇒ 「無載具」。
	// 🔴 不要用「summary 看起來像不像統編」去猜：8 碼數字兩者都可能。
	CarrierType           string `json:"carrierType"`
	RelatedDocumentId     string `json:"relatedDocumentId"`
	FileAvailable         bool   `json:"fileAvailable"`
	FileUnavailableReason string `json:"fileUnavailableReason"`
}

type InvoiceFileData struct {
	DocumentId     string `json:"documentId"`
	InvoiceFileKey string `json:"invoiceFileKey"`
	FileName       string `json:"fileName"`
	Style          string `json:"style"`
}

type InvoiceInputSpec struct {
	ProfileId     string `json:"profileId"`
	CarrierType   string `json:"carrierType"`
	Value         string `json:"value"`
	BuyerTaxId    string `json:"buyerTaxId"`
	BuyerName     string `json:"buyerName"`
	Email         string `json:"email"`
	SaveAsDefault bool   `json:"saveAsDefault"`
}

type InvoiceJobInfo struct {
	JobId                    string      `json:"jobId"`
	OrderNo                  string      `json:"orderNo"`
	StoreId                  string      `json:"storeId"`
	OperationType            string      `json:"operationType"`
	Status                   string      `json:"status"`
	Summary                  string      `json:"summary"`
	AttemptCount             string      `json:"attemptCount"`
	MaxAttempts              string      `json:"maxAttempts"`
	LastAttemptAt            string      `json:"lastAttemptAt"`
	NextRetryAt              string      `json:"nextRetryAt"`
	RetryPolicy              RetryPolicy `json:"retryPolicy"`
	ShipmentUnaffectedNotice string      `json:"shipmentUnaffectedNotice"`
	ProviderRequestId        string      `json:"providerRequestId"`
	Version                  string      `json:"version"`
	CanRetry                 bool        `json:"canRetry"`
	CanCancel                bool        `json:"canCancel"`
	ManualReasonCode         string      `json:"manualReasonCode"`
	NeedsFundingDecision     bool        `json:"needsFundingDecision"`
	ExternalInvoiceNo        string      `json:"externalInvoiceNo"`
	ExternalInvoiceDate      string      `json:"externalInvoiceDate"`
	ReportedAt               string      `json:"reportedAt"`
	CanReportExternal        bool        `json:"canReportExternal"`
	// DocumentKind 是 B2B／B2C／RECEIPT。賣家中心靠它分辨發票與收據兩個並列區塊
	// （#3066 裁決 B）。⚠️ 前端不得從 operationType 推導——那個 enum 沒有 RECEIPT。
	DocumentKind string `json:"documentKind"`
}

type InvoiceJobMutationData struct {
	JobId               string `json:"jobId"`
	Status              string `json:"status"`
	Version             string `json:"version"`
	AuditLogId          string `json:"auditLogId"`
	LastProviderCheckAt string `json:"lastProviderCheckAt"`
	ProviderRequestId   string `json:"providerRequestId"`
	NextStep            string `json:"nextStep"`
}

type InvoiceKpiItem struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	WindowLabel string `json:"windowLabel"`
	Definition  string `json:"definition"`
}

type InvoiceLine struct {
	LineType    string `json:"lineType"`
	Name        string `json:"name"`
	Quantity    string `json:"quantity"`
	UnitPrice   string `json:"unitPrice"`
	NetAmount   string `json:"netAmount"`
	TaxCategory string `json:"taxCategory"`
}

type InvoiceProfileInfo struct {
	ProfileId    string `json:"profileId"`
	CarrierType  string `json:"carrierType"`
	DisplayValue string `json:"displayValue"`
	BuyerTaxId   string `json:"buyerTaxId"`
	BuyerName    string `json:"buyerName"`
	IsDefault    bool   `json:"isDefault"`
	Label        string `json:"label"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type InvoiceTimelineItem struct {
	OccurredAt string `json:"occurredAt"`
	Event      string `json:"event"`
	Note       string `json:"note"`
}

type IssuanceIssuer struct {
	Kind    string `json:"kind"`
	StoreId string `json:"storeId"`
}

type IssuanceRecipient struct {
	Kind         string `json:"kind"`
	BuyerId      string `json:"buyerId"`
	StoreId      string `json:"storeId"`
	Name         string `json:"name"`
	TaxId        string `json:"taxId"`
	Email        string `json:"email"`
	CarrierType  string `json:"carrierType"`
	CarrierValue string `json:"carrierValue"`
}

type IssuanceSource struct {
	Kind    string        `json:"kind"`
	OrderNo string        `json:"orderNo"`
	Lines   []InvoiceLine `json:"lines"`
}

type ManualUnlockInfo struct {
	UnlockId              string `json:"unlockId"`
	Status                string `json:"status"`
	RequestedByAccountId  string `json:"requestedByAccountId"`
	ApprovedByAccountId   string `json:"approvedByAccountId"`
	ProviderCheckEvidence string `json:"providerCheckEvidence"`
	ProviderCheckedAt     string `json:"providerCheckedAt"`
	RequestedAt           string `json:"requestedAt"`
	ExpiresAt             string `json:"expiresAt"`

	// J28 的證據。**覆核者要看得到它**——沒有它，第二個人是在對一份看不見證據的
	// 請求按同意，雙人確認就只剩儀式。空字串＝這一列走的是 CANCELLED_BY_ADMIN 那條路徑。
	CancelEvidenceKind      string `json:"cancelEvidenceKind"`
	CancelEvidenceObjectKey string `json:"cancelEvidenceObjectKey"`
}

type OrderInvoiceData struct {
	OrderNo              string                   `json:"orderNo"`
	InvoiceMode          string                   `json:"invoiceMode"`
	State                string                   `json:"state"`
	StateNotice          string                   `json:"stateNotice"`
	Documents            []InvoiceDocumentSummary `json:"documents"`
	CanRequestCorrection bool                     `json:"canRequestCorrection"`
	CanFileComplaint     bool                     `json:"canFileComplaint"`
	ComplaintOpensAt     string                   `json:"complaintOpensAt"`
	ExistingComplaintId  string                   `json:"existingComplaintId"`

	// LatestCorrectionId 是這張訂單**最近一次**修改申請的 id（端點 12 的入參）；
	// 從未申請過時為空字串。它是買家端取得 correction_id 的唯一管道 ——
	// 少了它，送出申請之後重新載入頁面就再也看不到進度與婉拒理由。
	LatestCorrectionId  string `json:"latestCorrectionId"`
	NotifiedEmailMasked string `json:"notifiedEmailMasked"`
	ShippingFeeNotice   string `json:"shippingFeeNotice"`
	SnapshotVersion     string `json:"snapshotVersion"`
}

type ProviderAccountItem struct {
	StoreId             string `json:"storeId"`
	StoreName           string `json:"storeName"`
	SellerTaxId         string `json:"sellerTaxId"`
	Status              string `json:"status"`
	CredentialLast4     string `json:"credentialLast4"`
	LastVerifiedAt      string `json:"lastVerifiedAt"`
	AutomationCutoverAt string `json:"automationCutoverAt"`
}

type ProviderCutoverItem struct {
	Version            string `json:"version"`
	CutoverAt          string `json:"cutoverAt"`
	DeactivatedAt      string `json:"deactivatedAt"`
	Mode               string `json:"mode"`
	CreatedByAccountId string `json:"createdByAccountId"`
	CreatedAt          string `json:"createdAt"`
	Reason             string `json:"reason"`
}

type ProviderExchangeItem struct {
	ExchangeId         string `json:"exchangeId"`
	OccurredAt         string `json:"occurredAt"`
	Endpoint           string `json:"endpoint"`
	ProviderCode       string `json:"providerCode"`
	ProviderMessage    string `json:"providerMessage"`
	RequestId          string `json:"requestId"`
	MaskedRequestBody  string `json:"maskedRequestBody"`
	MaskedResponseBody string `json:"maskedResponseBody"`
}

type ResendInvoiceNotificationData struct {
	Resent   bool   `json:"resent"`
	SentAt   string `json:"sentAt"`
	ToMasked string `json:"toMasked"`
}

type RetryPolicy struct {
	InitialIntervalSeconds string `json:"initialIntervalSeconds"`
	BackoffMultiplier      string `json:"backoffMultiplier"`
	MaxIntervalSeconds     string `json:"maxIntervalSeconds"`
	MaxAttempts            string `json:"maxAttempts"`
}

type RevokeProviderAccountForAdminData struct {
	StoreId             string `json:"storeId"`
	Status              string `json:"status"`
	RevokedAt           string `json:"revokedAt"`
	AffectedScopeNotice string `json:"affectedScopeNotice"`
	PendingOrderCount   string `json:"pendingOrderCount"`
	AuditLogId          string `json:"auditLogId"`
}

type SelfIssueAttestationInfo struct {
	// TermsVersion 是**這個賣家已簽**的版本；未簽為空字串。
	TermsVersion string `json:"termsVersion"`
	TermsKey     string `json:"termsKey"`
	AcceptedAt   string `json:"acceptedAt"`
	// AcceptedByAccountId 是 accountId 不是 storeId（接受條款是自然人的意思表示）。
	AcceptedByAccountId string `json:"acceptedByAccountId"`
	Effective           bool   `json:"effective"`
	// CurrentTermsVersion 是**當期生效**的條款版本，恆非空。端點 22 要求原樣回送本欄
	// （不符即 400）⇒ 未簽過的賣家沒有它就永遠簽不成功。
	CurrentTermsVersion string `json:"currentTermsVersion"`
}

type SellerIntakeLimit struct {
	OrderCountUsed  string `json:"orderCountUsed"`
	OrderCountLimit string `json:"orderCountLimit"`
	AmountUsed      string `json:"amountUsed"`
	AmountLimit     string `json:"amountLimit"`
	Blocked         bool   `json:"blocked"`
}

type SellerInvoiceDocumentDetail struct {
	Detail                InvoiceDocumentDetail `json:"detail"`
	FundedBy              string                `json:"fundedBy"`
	CreatedBy             string                `json:"createdBy"`
	CutoverVersion        string                `json:"cutoverVersion"`
	ProviderRequestId     string                `json:"providerRequestId"`
	ManualReasonCode      string                `json:"manualReasonCode"`
	NextResendAvailableAt string                `json:"nextResendAvailableAt"`
	LatestJobId           string                `json:"latestJobId"`
}

type SellerInvoiceDocumentItem struct {
	Summary        InvoiceDocumentSummary `json:"summary"`
	OrderNo        string                 `json:"orderNo"`
	PaidAt         string                 `json:"paidAt"`
	BuyerKind      string                 `json:"buyerKind"`
	InvoiceMode    string                 `json:"invoiceMode"`
	ShipmentStatus string                 `json:"shipmentStatus"`
	UpdatedAt      string                 `json:"updatedAt"`
	LatestJobId    string                 `json:"latestJobId"`
}

type SellerInvoiceOverviewData struct {
	TaxReviewStatus       string                   `json:"taxReviewStatus"`
	InvoiceMode           string                   `json:"invoiceMode"`
	ProviderStatus        string                   `json:"providerStatus"`
	PendingDocumentCount  string                   `json:"pendingDocumentCount"`
	OpenComplaintCount    string                   `json:"openComplaintCount"`
	OverdueComplaintCount string                   `json:"overdueComplaintCount"`
	DeadlineAt            string                   `json:"deadlineAt"`
	ExtensionCount        string                   `json:"extensionCount"`
	CanRequestExtension   bool                     `json:"canRequestExtension"`
	SelfIssueEffective    bool                     `json:"selfIssueEffective"`
	IntakeLimit           SellerIntakeLimit        `json:"intakeLimit"`
	RecentDocuments       []InvoiceDocumentSummary `json:"recentDocuments"`
}

type SellerProviderAccountData struct {
	Provider            string `json:"provider"`
	Status              string `json:"status"`
	StatusNotice        string `json:"statusNotice"`
	Version             string `json:"version"`
	SellerTaxId         string `json:"sellerTaxId"`
	CredentialLast4     string `json:"credentialLast4"`
	CredentialUpdatedAt string `json:"credentialUpdatedAt"`
	LastVerifiedAt      string `json:"lastVerifiedAt"`
	AutomationCutoverAt string `json:"automationCutoverAt"`
	ConsentVersion      string `json:"consentVersion"`
	LastRequestId       string `json:"lastRequestId"`
	PendingOrderCount   string `json:"pendingOrderCount"`
}

type SellerReceiptAuthorizationData struct {
	Eligible                       bool   `json:"eligible"`
	IneligibleReason               string `json:"ineligibleReason"`
	ApprovalBasisKey               string `json:"approvalBasisKey"`
	ApprovedAt                     string `json:"approvedAt"`
	ApprovalValidUntil             string `json:"approvalValidUntil"`
	AuthorizedAt                   string `json:"authorizedAt"`
	ConsentVersion                 string `json:"consentVersion"`
	ReceiptTemplateKey             string `json:"receiptTemplateKey"`
	Version                        string `json:"version"`
	ApprovalBasisAssetId           string `json:"approvalBasisAssetId"`
	ApprovalBasisAvailable         bool   `json:"approvalBasisAvailable"`
	ApprovalBasisUnavailableReason string `json:"approvalBasisUnavailableReason"`
}

type SellerReceivedInvoiceItem struct {
	Summary       InvoiceDocumentSummary `json:"summary"`
	IssuerName    string                 `json:"issuerName"`
	RecipientNote string                 `json:"recipientNote"`
}

type SellerTaxIdentity struct {
	EntityType      string `json:"entityType"`
	TaxId           string `json:"taxId"`
	LegalName       string `json:"legalName"`
	ResponsibleName string `json:"responsibleName"`
	AddressCity     string `json:"addressCity"`
	AddressDistrict string `json:"addressDistrict"`
	AddressDetail   string `json:"addressDetail"`
	ClaimedStatus   string `json:"claimedStatus"`
}

type SellerTaxProfileData struct {
	StoreId              string                   `json:"storeId"`
	ReviewStatus         string                   `json:"reviewStatus"`
	Version              string                   `json:"version"`
	SubmittedAt          string                   `json:"submittedAt"`
	DeadlineAt           string                   `json:"deadlineAt"`
	ExtensionCount       string                   `json:"extensionCount"`
	CanRequestExtension  bool                     `json:"canRequestExtension"`
	ReapplyCooldownUntil string                   `json:"reapplyCooldownUntil"`
	TaxIdentity          SellerTaxIdentity        `json:"taxIdentity"`
	Documents            []TaxDocumentInfo        `json:"documents"`
	AdmNotes             []TaxReviewNote          `json:"admNotes"`
	SelfIssueAttestation SelfIssueAttestationInfo `json:"selfIssueAttestation"`
	IntakeLimit          SellerIntakeLimit        `json:"intakeLimit"`
	// ReceiptAuthorized ＝ 賣家已完成收據代製授權（S05，端點 29）。
	//
	// 🔴 **它是 T08「核定免用統一發票」的前置條件**（未授權 ⇒ 9354）。裁決
	// DEC-20260905-015836 之後端點 29 的 allowed-from 含四個「審核進行中」狀態
	// ＋ APPROVED_INVOICE_EXEMPT ⇒ 賣家在核定**之前**就簽得了。
	//
	// ⚠️ 本段在 2026-09-05 之前寫的是相反的話（「要等核定之後才簽得了、核定當下
	// 必然 false」）。那是裁決前的狀態，照它施工會做出一個不擋的核定按鈕。
	// 同一段話在 api/proto/invoice/invoice.proto 上已同步更正。
	ReceiptAuthorized   bool   `json:"receiptAuthorized"`
	ReceiptAuthorizedAt string `json:"receiptAuthorizedAt"`
	// PendingExtensionRequest ＝ 尚未被端點 41 裁決的展延申請；沒有時為 null。
	//
	// 🔴 端點 20 只寫一列 invoice_status_history、不碰 seller_tax_profiles 的任何一欄
	// ⇒ 在本欄出現之前，「賣家申請了展延」沒有任何讀取面出口：A02 的
	// filter=UNDER_REVIEW 把申請過與沒申請過的混在同一頁，A03 詳情只有
	// extensionCount（准了幾次）與 canRequestExtension（賣家還能不能按）。
	// ADM 因此不知道該按那顆「核准展延」，賣家按下去也看不出任何差別。
	//
	// pointer 而非值型別：本檔一律不帶 omitempty（key 必須存在），所以「沒有申請」
	// 的表達是 null，不是一個每欄都空的物件——後者前端分不出「沒申請」與「申請了
	// 但理由是空字串」。
	PendingExtensionRequest *TaxExtensionRequestInfo `json:"pendingExtensionRequest"`
}

// TaxExtensionRequestInfo 是一筆尚未裁決的展延申請（端點 20 → 41）。
type TaxExtensionRequestInfo struct {
	RequestedAt string `json:"requestedAt"`
	// 賣家送出時填的理由（端點 20 的 reason，必填）。ADM 決定准不准的唯一依據。
	Reason string `json:"reason"`
}

type SuspensionRecommendation struct {
	ShouldSuspend           bool   `json:"shouldSuspend"`
	StoreId                 string `json:"storeId"`
	ViolationCountIn_90Days string `json:"violationCountIn90Days"`
	ThresholdReached        bool   `json:"thresholdReached"`
	SuggestedSuspendDays    string `json:"suggestedSuspendDays"`
	SuggestedReason         string `json:"suggestedReason"`
}

type TaxDocumentInfo struct {
	DocumentId  string `json:"documentId"`
	Kind        string `json:"kind"`
	FileName    string `json:"fileName"`
	FileSize    string `json:"fileSize"`
	FileKey     string `json:"fileKey"`
	UploadedAt  string `json:"uploadedAt"`
	FileAssetId string `json:"fileAssetId"`
	// FileDownloadPath 只在 ADM 端點 39 的回應填：invoice 持有授權的稅籍佐證下載路徑，
	// 由擁有者回傳、client 不組（API 設計規範 §十三）。賣家回應不填（omitempty，JSON 不變）。
	FileDownloadPath string `json:"fileDownloadPath,omitempty"`
}

type TaxReviewItem struct {
	StoreId           string   `json:"storeId"`
	StoreName         string   `json:"storeName"`
	EntityType        string   `json:"entityType"`
	ClaimedStatus     string   `json:"claimedStatus"`
	ReviewStatus      string   `json:"reviewStatus"`
	SubmittedAt       string   `json:"submittedAt"`
	DeadlineAt        string   `json:"deadlineAt"`
	RemainingDays     string   `json:"remainingDays"`
	ExtensionCount    string   `json:"extensionCount"`
	SelfIssueAccepted bool     `json:"selfIssueAccepted"`
	RiskFlags         []string `json:"riskFlags"`

	// ReceiptAuthorized ＝ 收據代製授權（S05，端點 29）已簽。
	// 它是 T08「核定免用統一發票」的前置條件（未授權 ⇒ 9354），佇列上看得見
	// ADM 才不必逐一點進 A03 才知道哪一列現在核定得了。
	ReceiptAuthorized bool `json:"receiptAuthorized"`
}

type TaxReviewNote struct {
	NoteId    string `json:"noteId"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// ==================== proto → DTO 轉換 ====================

// convInvActOnInvoiceComplaintForAdminData 把 proto payload 轉成對外 DTO。
func convInvActOnInvoiceComplaintForAdminData(m *pb.ActOnInvoiceComplaintForAdminData) ActOnInvoiceComplaintForAdminData {
	var d ActOnInvoiceComplaintForAdminData
	if m == nil {
		return d
	}
	d.ComplaintId = m.GetComplaintId()
	d.Status = m.GetStatus()
	d.Version = m.GetVersion()
	d.Recommendation = convInvSuspensionRecommendation(m.GetRecommendation())
	d.AuditLogId = m.GetAuditLogId()
	return d
}

// convInvAdminInvoiceDocumentDetail 把 proto payload 轉成對外 DTO。
func convInvAdminInvoiceDocumentDetail(m *pb.AdminInvoiceDocumentDetail) AdminInvoiceDocumentDetail {
	var d AdminInvoiceDocumentDetail
	if m == nil {
		return d
	}
	d.SellerView = convInvSellerInvoiceDocumentDetail(m.GetSellerView())
	d.ProviderExchanges = make([]ProviderExchangeItem, 0, len(m.GetProviderExchanges()))
	for _, it := range m.GetProviderExchanges() {
		d.ProviderExchanges = append(d.ProviderExchanges, convInvProviderExchangeItem(it))
	}
	d.IdempotencyKey = m.GetIdempotencyKey()
	d.RelatedJobs = make([]InvoiceJobInfo, 0, len(m.GetRelatedJobs()))
	for _, it := range m.GetRelatedJobs() {
		d.RelatedJobs = append(d.RelatedJobs, convInvInvoiceJobInfo(it))
	}
	d.DocumentVersion = m.GetDocumentVersion()
	d.CanResumeFromManual = m.GetCanResumeFromManual()
	return d
}

// convInvAdminInvoiceJobDetail 把 proto payload 轉成對外 DTO。
func convInvAdminInvoiceJobDetail(m *pb.AdminInvoiceJobDetail) AdminInvoiceJobDetail {
	var d AdminInvoiceJobDetail
	if m == nil {
		return d
	}
	d.Job = convInvInvoiceJobInfo(m.GetJob())
	d.OrderSnapshotJson = m.GetOrderSnapshotJson()
	d.AmountBreakdown = convInvInvoiceAmountBreakdown(m.GetAmountBreakdown())
	d.ProviderExchanges = make([]ProviderExchangeItem, 0, len(m.GetProviderExchanges()))
	for _, it := range m.GetProviderExchanges() {
		d.ProviderExchanges = append(d.ProviderExchanges, convInvProviderExchangeItem(it))
	}
	d.IdempotencyKey = m.GetIdempotencyKey()
	d.ShipmentStatus = m.GetShipmentStatus()
	d.ShipmentStatusDegraded = m.GetShipmentStatusDegraded()
	d.AuditLogs = make([]InvoiceAuditLogItem, 0, len(m.GetAuditLogs()))
	for _, it := range m.GetAuditLogs() {
		d.AuditLogs = append(d.AuditLogs, convInvInvoiceAuditLogItem(it))
	}
	d.ManualUnlock = convInvManualUnlockInfo(m.GetManualUnlock())
	return d
}

// convInvBuyerInvoiceDocumentItem 把 proto payload 轉成對外 DTO。
func convInvBuyerInvoiceDocumentItem(m *pb.BuyerInvoiceDocumentItem) BuyerInvoiceDocumentItem {
	var d BuyerInvoiceDocumentItem
	if m == nil {
		return d
	}
	d.Summary = convInvInvoiceDocumentSummary(m.GetSummary())
	d.IssuerName = m.GetIssuerName()
	return d
}

// convInvComplaintResponseItem 把 proto payload 轉成對外 DTO。
func convInvComplaintResponseItem(m *pb.ComplaintResponseItem) ComplaintResponseItem {
	var d ComplaintResponseItem
	if m == nil {
		return d
	}
	d.ResponseId = m.GetResponseId()
	d.Actor = m.GetActor()
	d.Action = m.GetAction()
	d.Note = m.GetNote()
	d.OccurredAt = m.GetOccurredAt()
	return d
}

// convInvCreateManualIssuanceForAdminData 把 proto payload 轉成對外 DTO。
func convInvCreateManualIssuanceForAdminData(m *pb.CreateManualIssuanceForAdminData) CreateManualIssuanceForAdminData {
	var d CreateManualIssuanceForAdminData
	if m == nil {
		return d
	}
	d.JobId = m.GetJobId()
	d.DocumentId = m.GetDocumentId()
	d.Status = m.GetStatus()
	d.AuditLogId = m.GetAuditLogId()
	return d
}

// convInvDeactivateProviderAccountForAdminData 把 proto payload 轉成對外 DTO。
func convInvDeactivateProviderAccountForAdminData(m *pb.DeactivateProviderAccountForAdminData) DeactivateProviderAccountForAdminData {
	var d DeactivateProviderAccountForAdminData
	if m == nil {
		return d
	}
	d.StoreId = m.GetStoreId()
	d.Status = m.GetStatus()
	d.DeactivatedAt = m.GetDeactivatedAt()
	d.AffectedScopeNotice = m.GetAffectedScopeNotice()
	d.PendingOrderCount = m.GetPendingOrderCount()
	d.AuditLogId = m.GetAuditLogId()
	return d
}

// convInvDecideTaxReviewForAdminData 把 proto payload 轉成對外 DTO。
func convInvDecideTaxReviewForAdminData(m *pb.DecideTaxReviewForAdminData) DecideTaxReviewForAdminData {
	var d DecideTaxReviewForAdminData
	if m == nil {
		return d
	}
	d.StoreId = m.GetStoreId()
	d.ReviewStatus = m.GetReviewStatus()
	d.Version = m.GetVersion()
	d.AuditLogId = m.GetAuditLogId()
	d.RevokedEffectiveAt = m.GetRevokedEffectiveAt()
	return d
}

// convInvGetInvoiceOverviewForAdminData 把 proto payload 轉成對外 DTO。
func convInvGetInvoiceOverviewForAdminData(m *pb.GetInvoiceOverviewForAdminData) GetInvoiceOverviewForAdminData {
	var d GetInvoiceOverviewForAdminData
	if m == nil {
		return d
	}
	d.Kpis = make([]InvoiceKpiItem, 0, len(m.GetKpis()))
	for _, it := range m.GetKpis() {
		d.Kpis = append(d.Kpis, convInvInvoiceKpiItem(it))
	}
	d.PriorityJobs = make([]InvoiceJobInfo, 0, len(m.GetPriorityJobs()))
	for _, it := range m.GetPriorityJobs() {
		d.PriorityJobs = append(d.PriorityJobs, convInvInvoiceJobInfo(it))
	}
	return d
}

// convInvInvoiceAmountBreakdown 把 proto payload 轉成對外 DTO。
func convInvInvoiceAmountBreakdown(m *pb.InvoiceAmountBreakdown) InvoiceAmountBreakdown {
	var d InvoiceAmountBreakdown
	if m == nil {
		return d
	}
	d.ProductAmount = m.GetProductAmount()
	d.ShippingAmount = m.GetShippingAmount()
	d.DiscountAmount = m.GetDiscountAmount()
	d.InvoiceAmount = m.GetInvoiceAmount()
	d.TaxCategory = m.GetTaxCategory()
	return d
}

// convInvInvoiceAuditLogItem 把 proto payload 轉成對外 DTO。
func convInvInvoiceAuditLogItem(m *pb.InvoiceAuditLogItem) InvoiceAuditLogItem {
	var d InvoiceAuditLogItem
	if m == nil {
		return d
	}
	d.AuditLogId = m.GetAuditLogId()
	d.OccurredAt = m.GetOccurredAt()
	d.ActorAccountId = m.GetActorAccountId()
	d.ActorDisplayName = m.GetActorDisplayName()
	d.OperationType = m.GetOperationType()
	d.TargetKind = m.GetTargetKind()
	d.TargetId = m.GetTargetId()
	d.BeforeJson = m.GetBeforeJson()
	d.AfterJson = m.GetAfterJson()
	d.Reason = m.GetReason()
	d.RequestId = m.GetRequestId()
	d.TermsVersion = m.GetTermsVersion()
	d.Result = m.GetResult()
	return d
}

// convInvInvoiceComplaintInfo 把 proto payload 轉成對外 DTO。
func convInvInvoiceComplaintInfo(m *pb.InvoiceComplaintInfo) InvoiceComplaintInfo {
	var d InvoiceComplaintInfo
	if m == nil {
		return d
	}
	d.ComplaintId = m.GetComplaintId()
	d.CaseNo = m.GetCaseNo()
	d.OrderNo = m.GetOrderNo()
	d.StoreId = m.GetStoreId()
	d.Status = m.GetStatus()
	d.ExpectedDocKind = m.GetExpectedDocKind()
	d.DueAt = m.GetDueAt()
	d.RemainingBusinessDays = m.GetRemainingBusinessDays()
	d.OverdueBusinessDays = m.GetOverdueBusinessDays()
	d.BuyerMaskedName = m.GetBuyerMaskedName()
	d.Description = m.GetDescription()
	d.Responses = make([]ComplaintResponseItem, 0, len(m.GetResponses()))
	for _, it := range m.GetResponses() {
		d.Responses = append(d.Responses, convInvComplaintResponseItem(it))
	}
	d.Version = m.GetVersion()
	d.CreatedAt = m.GetCreatedAt()
	d.ReopenCount = m.GetReopenCount()
	d.BuyerReviewDueAt = m.GetBuyerReviewDueAt()
	d.ReopenAllowedAfter = m.GetReopenAllowedAfter()
	return d
}

// convInvInvoiceCorrectionData 把 proto payload 轉成對外 DTO。
func convInvInvoiceCorrectionData(m *pb.InvoiceCorrectionData) InvoiceCorrectionData {
	var d InvoiceCorrectionData
	if m == nil {
		return d
	}
	d.CorrectionId = m.GetCorrectionId()
	d.HandlingMode = m.GetHandlingMode()
	d.Status = m.GetStatus()
	d.StatusNotice = m.GetStatusNotice()
	d.CreatedAt = m.GetCreatedAt()
	d.RejectReasonCode = m.GetRejectReasonCode()
	return d
}

// convInvInvoiceDocumentDetail 把 proto payload 轉成對外 DTO。
func convInvInvoiceDocumentDetail(m *pb.InvoiceDocumentDetail) InvoiceDocumentDetail {
	var d InvoiceDocumentDetail
	if m == nil {
		return d
	}
	d.Summary = convInvInvoiceDocumentSummary(m.GetSummary())
	d.IssuerName = m.GetIssuerName()
	d.IssuerTaxId = m.GetIssuerTaxId()
	d.RecipientName = m.GetRecipientName()
	d.RecipientTaxId = m.GetRecipientTaxId()
	d.SalesAmount = m.GetSalesAmount()
	d.TaxAmount = m.GetTaxAmount()
	d.TotalAmount = m.GetTotalAmount()
	d.TaxCategory = m.GetTaxCategory()
	d.Lines = make([]InvoiceLine, 0, len(m.GetLines()))
	for _, it := range m.GetLines() {
		d.Lines = append(d.Lines, convInvInvoiceLine(it))
	}
	d.Timeline = make([]InvoiceTimelineItem, 0, len(m.GetTimeline()))
	for _, it := range m.GetTimeline() {
		d.Timeline = append(d.Timeline, convInvInvoiceTimelineItem(it))
	}
	d.InvoiceFileKey = m.GetInvoiceFileKey()
	return d
}

// convInvInvoiceDocumentSummary 把 proto payload 轉成對外 DTO。
func convInvInvoiceDocumentSummary(m *pb.InvoiceDocumentSummary) InvoiceDocumentSummary {
	var d InvoiceDocumentSummary
	if m == nil {
		return d
	}
	d.DocumentId = m.GetDocumentId()
	d.DocumentKind = m.GetDocumentKind()
	d.Status = m.GetStatus()
	d.InvoiceNumber = m.GetInvoiceNumber()
	d.IssuedAt = m.GetIssuedAt()
	d.TotalAmount = m.GetTotalAmount()
	d.CarrierSummary = m.GetCarrierSummary()
	d.CarrierType = m.GetCarrierType()
	d.RelatedDocumentId = m.GetRelatedDocumentId()
	d.FileAvailable = m.GetFileAvailable()
	d.FileUnavailableReason = m.GetFileUnavailableReason()
	return d
}

// convInvInvoiceFileData 把 proto payload 轉成對外 DTO。
func convInvInvoiceFileData(m *pb.InvoiceFileData) InvoiceFileData {
	var d InvoiceFileData
	if m == nil {
		return d
	}
	d.DocumentId = m.GetDocumentId()
	d.InvoiceFileKey = m.GetInvoiceFileKey()
	d.FileName = m.GetFileName()
	d.Style = m.GetStyle()
	return d
}

// convInvInvoiceJobInfo 把 proto payload 轉成對外 DTO。
func convInvInvoiceJobInfo(m *pb.InvoiceJobInfo) InvoiceJobInfo {
	var d InvoiceJobInfo
	if m == nil {
		return d
	}
	d.JobId = m.GetJobId()
	d.OrderNo = m.GetOrderNo()
	d.StoreId = m.GetStoreId()
	d.OperationType = m.GetOperationType()
	d.Status = m.GetStatus()
	d.Summary = m.GetSummary()
	d.AttemptCount = m.GetAttemptCount()
	d.MaxAttempts = m.GetMaxAttempts()
	d.LastAttemptAt = m.GetLastAttemptAt()
	d.NextRetryAt = m.GetNextRetryAt()
	d.RetryPolicy = convInvRetryPolicy(m.GetRetryPolicy())
	d.ShipmentUnaffectedNotice = m.GetShipmentUnaffectedNotice()
	d.ProviderRequestId = m.GetProviderRequestId()
	d.Version = m.GetVersion()
	d.CanRetry = m.GetCanRetry()
	d.CanCancel = m.GetCanCancel()
	d.ManualReasonCode = m.GetManualReasonCode()
	d.NeedsFundingDecision = m.GetNeedsFundingDecision()
	d.ExternalInvoiceNo = m.GetExternalInvoiceNo()
	d.ExternalInvoiceDate = m.GetExternalInvoiceDate()
	d.ReportedAt = m.GetReportedAt()
	d.CanReportExternal = m.GetCanReportExternal()
	d.DocumentKind = m.GetDocumentKind()
	return d
}

// convInvInvoiceJobMutationData 把 proto payload 轉成對外 DTO。
func convInvInvoiceJobMutationData(m *pb.InvoiceJobMutationData) InvoiceJobMutationData {
	var d InvoiceJobMutationData
	if m == nil {
		return d
	}
	d.JobId = m.GetJobId()
	d.Status = m.GetStatus()
	d.Version = m.GetVersion()
	d.AuditLogId = m.GetAuditLogId()
	d.LastProviderCheckAt = m.GetLastProviderCheckAt()
	d.ProviderRequestId = m.GetProviderRequestId()
	d.NextStep = m.GetNextStep()
	return d
}

// convInvInvoiceKpiItem 把 proto payload 轉成對外 DTO。
func convInvInvoiceKpiItem(m *pb.InvoiceKpiItem) InvoiceKpiItem {
	var d InvoiceKpiItem
	if m == nil {
		return d
	}
	d.Key = m.GetKey()
	d.Value = m.GetValue()
	d.WindowLabel = m.GetWindowLabel()
	d.Definition = m.GetDefinition()
	return d
}

// convInvInvoiceLine 把 proto payload 轉成對外 DTO。
func convInvInvoiceLine(m *pb.InvoiceLine) InvoiceLine {
	var d InvoiceLine
	if m == nil {
		return d
	}
	d.LineType = m.GetLineType()
	d.Name = m.GetName()
	d.Quantity = m.GetQuantity()
	d.UnitPrice = m.GetUnitPrice()
	d.NetAmount = m.GetNetAmount()
	d.TaxCategory = m.GetTaxCategory()
	return d
}

// convInvInvoiceProfileInfo 把 proto payload 轉成對外 DTO。
func convInvInvoiceProfileInfo(m *pb.InvoiceProfileInfo) InvoiceProfileInfo {
	var d InvoiceProfileInfo
	if m == nil {
		return d
	}
	d.ProfileId = m.GetProfileId()
	d.CarrierType = m.GetCarrierType()
	d.DisplayValue = m.GetDisplayValue()
	d.BuyerTaxId = m.GetBuyerTaxId()
	d.BuyerName = m.GetBuyerName()
	d.IsDefault = m.GetIsDefault()
	d.Label = m.GetLabel()
	d.CreatedAt = m.GetCreatedAt()
	d.UpdatedAt = m.GetUpdatedAt()
	return d
}

// convInvInvoiceTimelineItem 把 proto payload 轉成對外 DTO。
func convInvInvoiceTimelineItem(m *pb.InvoiceTimelineItem) InvoiceTimelineItem {
	var d InvoiceTimelineItem
	if m == nil {
		return d
	}
	d.OccurredAt = m.GetOccurredAt()
	d.Event = m.GetEvent()
	d.Note = m.GetNote()
	return d
}

// convInvManualUnlockInfo 把 proto payload 轉成對外 DTO。
func convInvManualUnlockInfo(m *pb.ManualUnlockInfo) ManualUnlockInfo {
	var d ManualUnlockInfo
	if m == nil {
		return d
	}
	d.UnlockId = m.GetUnlockId()
	d.Status = m.GetStatus()
	d.RequestedByAccountId = m.GetRequestedByAccountId()
	d.ApprovedByAccountId = m.GetApprovedByAccountId()
	d.ProviderCheckEvidence = m.GetProviderCheckEvidence()
	d.ProviderCheckedAt = m.GetProviderCheckedAt()
	d.RequestedAt = m.GetRequestedAt()
	d.ExpiresAt = m.GetExpiresAt()
	d.CancelEvidenceKind = m.GetCancelEvidenceKind()
	d.CancelEvidenceObjectKey = m.GetCancelEvidenceObjectKey()
	return d
}

// convInvOrderInvoiceData 把 proto payload 轉成對外 DTO。
func convInvOrderInvoiceData(m *pb.OrderInvoiceData) OrderInvoiceData {
	var d OrderInvoiceData
	if m == nil {
		return d
	}
	d.OrderNo = m.GetOrderNo()
	d.InvoiceMode = m.GetInvoiceMode()
	d.State = m.GetState()
	d.StateNotice = m.GetStateNotice()
	d.Documents = make([]InvoiceDocumentSummary, 0, len(m.GetDocuments()))
	for _, it := range m.GetDocuments() {
		d.Documents = append(d.Documents, convInvInvoiceDocumentSummary(it))
	}
	d.CanRequestCorrection = m.GetCanRequestCorrection()
	d.CanFileComplaint = m.GetCanFileComplaint()
	d.ComplaintOpensAt = m.GetComplaintOpensAt()
	d.ExistingComplaintId = m.GetExistingComplaintId()
	d.LatestCorrectionId = m.GetLatestCorrectionId()
	d.NotifiedEmailMasked = m.GetNotifiedEmailMasked()
	d.ShippingFeeNotice = m.GetShippingFeeNotice()
	d.SnapshotVersion = m.GetSnapshotVersion()
	return d
}

// convInvProviderAccountItem 把 proto payload 轉成對外 DTO。
func convInvProviderAccountItem(m *pb.ProviderAccountItem) ProviderAccountItem {
	var d ProviderAccountItem
	if m == nil {
		return d
	}
	d.StoreId = m.GetStoreId()
	d.StoreName = m.GetStoreName()
	d.SellerTaxId = m.GetSellerTaxId()
	d.Status = m.GetStatus()
	d.CredentialLast4 = m.GetCredentialLast4()
	d.LastVerifiedAt = m.GetLastVerifiedAt()
	d.AutomationCutoverAt = m.GetAutomationCutoverAt()
	return d
}

// convInvProviderCutoverItem 把 proto payload 轉成對外 DTO。
func convInvProviderCutoverItem(m *pb.ProviderCutoverItem) ProviderCutoverItem {
	var d ProviderCutoverItem
	if m == nil {
		return d
	}
	d.Version = m.GetVersion()
	d.CutoverAt = m.GetCutoverAt()
	d.DeactivatedAt = m.GetDeactivatedAt()
	d.Mode = m.GetMode()
	d.CreatedByAccountId = m.GetCreatedByAccountId()
	d.CreatedAt = m.GetCreatedAt()
	d.Reason = m.GetReason()
	return d
}

// convInvProviderExchangeItem 把 proto payload 轉成對外 DTO。
func convInvProviderExchangeItem(m *pb.ProviderExchangeItem) ProviderExchangeItem {
	var d ProviderExchangeItem
	if m == nil {
		return d
	}
	d.ExchangeId = m.GetExchangeId()
	d.OccurredAt = m.GetOccurredAt()
	d.Endpoint = m.GetEndpoint()
	d.ProviderCode = m.GetProviderCode()
	d.ProviderMessage = m.GetProviderMessage()
	d.RequestId = m.GetRequestId()
	d.MaskedRequestBody = m.GetMaskedRequestBody()
	d.MaskedResponseBody = m.GetMaskedResponseBody()
	return d
}

// convInvResendInvoiceNotificationData 把 proto payload 轉成對外 DTO。
func convInvResendInvoiceNotificationData(m *pb.ResendInvoiceNotificationData) ResendInvoiceNotificationData {
	var d ResendInvoiceNotificationData
	if m == nil {
		return d
	}
	d.Resent = m.GetResent()
	d.SentAt = m.GetSentAt()
	d.ToMasked = m.GetToMasked()
	return d
}

// convInvRetryPolicy 把 proto payload 轉成對外 DTO。
func convInvRetryPolicy(m *pb.RetryPolicy) RetryPolicy {
	var d RetryPolicy
	if m == nil {
		return d
	}
	d.InitialIntervalSeconds = m.GetInitialIntervalSeconds()
	d.BackoffMultiplier = m.GetBackoffMultiplier()
	d.MaxIntervalSeconds = m.GetMaxIntervalSeconds()
	d.MaxAttempts = m.GetMaxAttempts()
	return d
}

// convInvRevokeProviderAccountForAdminData 把 proto payload 轉成對外 DTO。
func convInvRevokeProviderAccountForAdminData(m *pb.RevokeProviderAccountForAdminData) RevokeProviderAccountForAdminData {
	var d RevokeProviderAccountForAdminData
	if m == nil {
		return d
	}
	d.StoreId = m.GetStoreId()
	d.Status = m.GetStatus()
	d.RevokedAt = m.GetRevokedAt()
	d.AffectedScopeNotice = m.GetAffectedScopeNotice()
	d.PendingOrderCount = m.GetPendingOrderCount()
	d.AuditLogId = m.GetAuditLogId()
	return d
}

// convInvSelfIssueAttestationInfo 把 proto payload 轉成對外 DTO。
func convInvSelfIssueAttestationInfo(m *pb.SelfIssueAttestationInfo) SelfIssueAttestationInfo {
	var d SelfIssueAttestationInfo
	if m == nil {
		return d
	}
	d.TermsVersion = m.GetTermsVersion()
	d.TermsKey = m.GetTermsKey()
	d.AcceptedAt = m.GetAcceptedAt()
	d.AcceptedByAccountId = m.GetAcceptedByAccountId()
	d.Effective = m.GetEffective()
	d.CurrentTermsVersion = m.GetCurrentTermsVersion()
	return d
}

// convInvSellerIntakeLimit 把 proto payload 轉成對外 DTO。
func convInvSellerIntakeLimit(m *pb.SellerIntakeLimit) SellerIntakeLimit {
	var d SellerIntakeLimit
	if m == nil {
		return d
	}
	d.OrderCountUsed = m.GetOrderCountUsed()
	d.OrderCountLimit = m.GetOrderCountLimit()
	d.AmountUsed = m.GetAmountUsed()
	d.AmountLimit = m.GetAmountLimit()
	d.Blocked = m.GetBlocked()
	return d
}

// convInvSellerInvoiceDocumentDetail 把 proto payload 轉成對外 DTO。
func convInvSellerInvoiceDocumentDetail(m *pb.SellerInvoiceDocumentDetail) SellerInvoiceDocumentDetail {
	var d SellerInvoiceDocumentDetail
	if m == nil {
		return d
	}
	d.Detail = convInvInvoiceDocumentDetail(m.GetDetail())
	d.FundedBy = m.GetFundedBy()
	d.CreatedBy = m.GetCreatedBy()
	d.CutoverVersion = m.GetCutoverVersion()
	d.ProviderRequestId = m.GetProviderRequestId()
	d.ManualReasonCode = m.GetManualReasonCode()
	d.NextResendAvailableAt = m.GetNextResendAvailableAt()
	d.LatestJobId = m.GetLatestJobId()
	return d
}

// InvoiceDocumentsPageData 是端點 30／47 的分頁 data，比標準分頁多一個布林。
//
// 🔴 **內嵌 response.PageData 而不是自己列四個欄位**（§16 禁止 handler 自組分頁 gin.H）：
// records／total／page／pageSize 的 JSON key 全部沿用同一份 struct tag，
// 不可能漂移出 `page_size` 這種 snake 變體。
//
// shipmentStatusDegraded ＝ 這一頁的 shipmentStatus 沒有取到（shipping-service
// 不可用／逾時），**不是**「這些訂單都還沒出貨」。少了它，兩者在前端塌成同一個畫面：
// 每一列都是空字串，而空字串在這一欄會被讀成「還沒出貨」（ADM A12 頁面逐字寫過
// 這件事）——那正是 #3019 的原始症狀換一個形式重演。
type InvoiceDocumentsPageData struct {
	response.PageData
	ShipmentStatusDegraded bool `json:"shipmentStatusDegraded"`
}

// convInvSellerInvoiceDocumentItem 把 proto payload 轉成對外 DTO。
func convInvSellerInvoiceDocumentItem(m *pb.SellerInvoiceDocumentItem) SellerInvoiceDocumentItem {
	var d SellerInvoiceDocumentItem
	if m == nil {
		return d
	}
	d.Summary = convInvInvoiceDocumentSummary(m.GetSummary())
	d.OrderNo = m.GetOrderNo()
	d.PaidAt = m.GetPaidAt()
	d.BuyerKind = m.GetBuyerKind()
	d.InvoiceMode = m.GetInvoiceMode()
	d.ShipmentStatus = m.GetShipmentStatus()
	d.UpdatedAt = m.GetUpdatedAt()
	d.LatestJobId = m.GetLatestJobId()
	return d
}

// convInvSellerInvoiceOverviewData 把 proto payload 轉成對外 DTO。
func convInvSellerInvoiceOverviewData(m *pb.SellerInvoiceOverviewData) SellerInvoiceOverviewData {
	var d SellerInvoiceOverviewData
	if m == nil {
		return d
	}
	d.TaxReviewStatus = m.GetTaxReviewStatus()
	d.InvoiceMode = m.GetInvoiceMode()
	d.ProviderStatus = m.GetProviderStatus()
	d.PendingDocumentCount = m.GetPendingDocumentCount()
	d.OpenComplaintCount = m.GetOpenComplaintCount()
	d.OverdueComplaintCount = m.GetOverdueComplaintCount()
	d.DeadlineAt = m.GetDeadlineAt()
	d.ExtensionCount = m.GetExtensionCount()
	d.CanRequestExtension = m.GetCanRequestExtension()
	d.SelfIssueEffective = m.GetSelfIssueEffective()
	d.IntakeLimit = convInvSellerIntakeLimit(m.GetIntakeLimit())
	d.RecentDocuments = make([]InvoiceDocumentSummary, 0, len(m.GetRecentDocuments()))
	for _, it := range m.GetRecentDocuments() {
		d.RecentDocuments = append(d.RecentDocuments, convInvInvoiceDocumentSummary(it))
	}
	return d
}

// convInvSellerProviderAccountData 把 proto payload 轉成對外 DTO。
func convInvSellerProviderAccountData(m *pb.SellerProviderAccountData) SellerProviderAccountData {
	var d SellerProviderAccountData
	if m == nil {
		return d
	}
	d.Provider = m.GetProvider()
	d.Status = m.GetStatus()
	d.StatusNotice = m.GetStatusNotice()
	d.Version = m.GetVersion()
	d.SellerTaxId = m.GetSellerTaxId()
	d.CredentialLast4 = m.GetCredentialLast4()
	d.CredentialUpdatedAt = m.GetCredentialUpdatedAt()
	d.LastVerifiedAt = m.GetLastVerifiedAt()
	d.AutomationCutoverAt = m.GetAutomationCutoverAt()
	d.ConsentVersion = m.GetConsentVersion()
	d.LastRequestId = m.GetLastRequestId()
	d.PendingOrderCount = m.GetPendingOrderCount()
	return d
}

// convInvSellerReceiptAuthorizationData 把 proto payload 轉成對外 DTO。
func convInvSellerReceiptAuthorizationData(m *pb.SellerReceiptAuthorizationData) SellerReceiptAuthorizationData {
	var d SellerReceiptAuthorizationData
	if m == nil {
		return d
	}
	d.Eligible = m.GetEligible()
	d.IneligibleReason = m.GetIneligibleReason()
	d.ApprovalBasisKey = m.GetApprovalBasisKey()
	d.ApprovedAt = m.GetApprovedAt()
	d.ApprovalValidUntil = m.GetApprovalValidUntil()
	d.AuthorizedAt = m.GetAuthorizedAt()
	d.ConsentVersion = m.GetConsentVersion()
	d.ReceiptTemplateKey = m.GetReceiptTemplateKey()
	d.Version = m.GetVersion()
	d.ApprovalBasisAssetId = m.GetApprovalBasisAssetId()
	d.ApprovalBasisAvailable = m.GetApprovalBasisAvailable()
	d.ApprovalBasisUnavailableReason = m.GetApprovalBasisUnavailableReason()
	return d
}

// convInvSellerReceivedInvoiceItem 把 proto payload 轉成對外 DTO。
func convInvSellerReceivedInvoiceItem(m *pb.SellerReceivedInvoiceItem) SellerReceivedInvoiceItem {
	var d SellerReceivedInvoiceItem
	if m == nil {
		return d
	}
	d.Summary = convInvInvoiceDocumentSummary(m.GetSummary())
	d.IssuerName = m.GetIssuerName()
	d.RecipientNote = m.GetRecipientNote()
	return d
}

// convInvSellerTaxIdentity 把 proto payload 轉成對外 DTO。
func convInvSellerTaxIdentity(m *pb.SellerTaxIdentity) SellerTaxIdentity {
	var d SellerTaxIdentity
	if m == nil {
		return d
	}
	d.EntityType = m.GetEntityType()
	d.TaxId = m.GetTaxId()
	d.LegalName = m.GetLegalName()
	d.ResponsibleName = m.GetResponsibleName()
	d.AddressCity = m.GetAddressCity()
	d.AddressDistrict = m.GetAddressDistrict()
	d.AddressDetail = m.GetAddressDetail()
	d.ClaimedStatus = m.GetClaimedStatus()
	return d
}

// convInvSellerTaxProfileData 把 proto payload 轉成對外 DTO。
func convInvSellerTaxProfileData(m *pb.SellerTaxProfileData) SellerTaxProfileData {
	var d SellerTaxProfileData
	if m == nil {
		return d
	}
	d.StoreId = m.GetStoreId()
	d.ReviewStatus = m.GetReviewStatus()
	d.Version = m.GetVersion()
	d.SubmittedAt = m.GetSubmittedAt()
	d.DeadlineAt = m.GetDeadlineAt()
	d.ExtensionCount = m.GetExtensionCount()
	d.CanRequestExtension = m.GetCanRequestExtension()
	d.ReapplyCooldownUntil = m.GetReapplyCooldownUntil()
	d.TaxIdentity = convInvSellerTaxIdentity(m.GetTaxIdentity())
	d.Documents = make([]TaxDocumentInfo, 0, len(m.GetDocuments()))
	for _, it := range m.GetDocuments() {
		d.Documents = append(d.Documents, convInvTaxDocumentInfo(it))
	}
	d.AdmNotes = make([]TaxReviewNote, 0, len(m.GetAdmNotes()))
	for _, it := range m.GetAdmNotes() {
		d.AdmNotes = append(d.AdmNotes, convInvTaxReviewNote(it))
	}
	d.SelfIssueAttestation = convInvSelfIssueAttestationInfo(m.GetSelfIssueAttestation())
	d.IntakeLimit = convInvSellerIntakeLimit(m.GetIntakeLimit())
	d.ReceiptAuthorized = m.GetReceiptAuthorized()
	d.ReceiptAuthorizedAt = m.GetReceiptAuthorizedAt()
	// nil 要原樣傳成 null，不得折成一個每欄皆空的物件——前端靠「有沒有這個物件」
	// 決定要不要渲染「賣家已申請展延」那一塊，空物件會讓它對每個賣家都渲染。
	if r := m.GetPendingExtensionRequest(); r != nil {
		d.PendingExtensionRequest = &TaxExtensionRequestInfo{
			RequestedAt: r.GetRequestedAt(),
			Reason:      r.GetReason(),
		}
	}
	return d
}

// convInvSuspensionRecommendation 把 proto payload 轉成對外 DTO。
func convInvSuspensionRecommendation(m *pb.SuspensionRecommendation) SuspensionRecommendation {
	var d SuspensionRecommendation
	if m == nil {
		return d
	}
	d.ShouldSuspend = m.GetShouldSuspend()
	d.StoreId = m.GetStoreId()
	d.ViolationCountIn_90Days = m.GetViolationCountIn_90Days()
	d.ThresholdReached = m.GetThresholdReached()
	d.SuggestedSuspendDays = m.GetSuggestedSuspendDays()
	d.SuggestedReason = m.GetSuggestedReason()
	return d
}

// convInvTaxDocumentInfo 把 proto payload 轉成對外 DTO。
func convInvTaxDocumentInfo(m *pb.TaxDocumentInfo) TaxDocumentInfo {
	var d TaxDocumentInfo
	if m == nil {
		return d
	}
	d.DocumentId = m.GetDocumentId()
	d.Kind = m.GetKind()
	d.FileName = m.GetFileName()
	d.FileSize = m.GetFileSize()
	d.FileKey = m.GetFileKey()
	d.UploadedAt = m.GetUploadedAt()
	d.FileAssetId = m.GetFileAssetId()
	return d
}

// convInvTaxReviewItem 把 proto payload 轉成對外 DTO。
func convInvTaxReviewItem(m *pb.TaxReviewItem) TaxReviewItem {
	var d TaxReviewItem
	if m == nil {
		return d
	}
	d.StoreId = m.GetStoreId()
	d.StoreName = m.GetStoreName()
	d.EntityType = m.GetEntityType()
	d.ClaimedStatus = m.GetClaimedStatus()
	d.ReviewStatus = m.GetReviewStatus()
	d.SubmittedAt = m.GetSubmittedAt()
	d.DeadlineAt = m.GetDeadlineAt()
	d.RemainingDays = m.GetRemainingDays()
	d.ExtensionCount = m.GetExtensionCount()
	d.SelfIssueAccepted = m.GetSelfIssueAccepted()
	d.ReceiptAuthorized = m.GetReceiptAuthorized()
	d.RiskFlags = append([]string{}, m.GetRiskFlags()...)
	return d
}

// convInvTaxReviewNote 把 proto payload 轉成對外 DTO。
func convInvTaxReviewNote(m *pb.TaxReviewNote) TaxReviewNote {
	var d TaxReviewNote
	if m == nil {
		return d
	}
	d.NoteId = m.GetNoteId()
	d.Content = m.GetContent()
	d.CreatedAt = m.GetCreatedAt()
	return d
}

// ==================== DTO → proto 轉換（入站 body 的子 message）====================

// toPbInvInvoiceInputSpec 把入站 DTO 轉成 proto request 的子 message。
func toPbInvInvoiceInputSpec(d InvoiceInputSpec) *pb.InvoiceInputSpec {
	m := &pb.InvoiceInputSpec{}
	m.ProfileId = d.ProfileId
	m.CarrierType = d.CarrierType
	m.Value = d.Value
	m.BuyerTaxId = d.BuyerTaxId
	m.BuyerName = d.BuyerName
	m.Email = d.Email
	m.SaveAsDefault = d.SaveAsDefault
	return m
}

// toPbInvInvoiceLine 把入站 DTO 轉成 proto request 的子 message。
func toPbInvInvoiceLine(d InvoiceLine) *pb.InvoiceLine {
	m := &pb.InvoiceLine{}
	m.LineType = d.LineType
	m.Name = d.Name
	m.Quantity = d.Quantity
	m.UnitPrice = d.UnitPrice
	m.NetAmount = d.NetAmount
	m.TaxCategory = d.TaxCategory
	return m
}

// toPbInvIssuanceIssuer 把入站 DTO 轉成 proto request 的子 message。
func toPbInvIssuanceIssuer(d IssuanceIssuer) *pb.IssuanceIssuer {
	m := &pb.IssuanceIssuer{}
	m.Kind = d.Kind
	m.StoreId = d.StoreId
	return m
}

// toPbInvIssuanceRecipient 把入站 DTO 轉成 proto request 的子 message。
func toPbInvIssuanceRecipient(d IssuanceRecipient) *pb.IssuanceRecipient {
	m := &pb.IssuanceRecipient{}
	m.Kind = d.Kind
	m.BuyerId = d.BuyerId
	m.StoreId = d.StoreId
	m.Name = d.Name
	m.TaxId = d.TaxId
	m.Email = d.Email
	m.CarrierType = d.CarrierType
	m.CarrierValue = d.CarrierValue
	return m
}

// toPbInvIssuanceSource 把入站 DTO 轉成 proto request 的子 message。
func toPbInvIssuanceSource(d IssuanceSource) *pb.IssuanceSource {
	m := &pb.IssuanceSource{}
	m.Kind = d.Kind
	m.OrderNo = d.OrderNo
	for _, it := range d.Lines {
		m.Lines = append(m.Lines, toPbInvInvoiceLine(it))
	}
	return m
}

// toPbInvSellerTaxIdentity 把入站 DTO 轉成 proto request 的子 message。
func toPbInvSellerTaxIdentity(d SellerTaxIdentity) *pb.SellerTaxIdentity {
	m := &pb.SellerTaxIdentity{}
	m.EntityType = d.EntityType
	m.TaxId = d.TaxId
	m.LegalName = d.LegalName
	m.ResponsibleName = d.ResponsibleName
	m.AddressCity = d.AddressCity
	m.AddressDistrict = d.AddressDistrict
	m.AddressDetail = d.AddressDetail
	m.ClaimedStatus = d.ClaimedStatus
	return m
}

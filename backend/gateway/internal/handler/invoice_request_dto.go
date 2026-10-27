package handler

// 58 號《電子發票服務》§6.1 的入站 body DTO。
//
// 規約（§6.1.0(17)）：Request DTO 一律各自獨立即使欄位相同——共用的那一份會被任一
// 邊加欄污染（例：端點 3 不共用 CreateBuyerInvoiceProfileRequest，setDefault 語意不同
// 且 carrierType 不可變更）。欄位語意與值域回查 §6.2.1，本檔不複寫第二份欄位表。

// CreateBuyerInvoiceProfileRequest 是端點 2 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type CreateBuyerInvoiceProfileRequest struct {
	CarrierType string `json:"carrierType"`
	Value       string `json:"value"`
	BuyerTaxId  string `json:"buyerTaxId"`
	BuyerName   string `json:"buyerName"`
	Label       string `json:"label"`
	SetDefault  bool   `json:"setDefault"`
}

// UpdateBuyerInvoiceProfileRequest 是端點 3 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type UpdateBuyerInvoiceProfileRequest struct {
	Value      string `json:"value"`
	BuyerTaxId string `json:"buyerTaxId"`
	BuyerName  string `json:"buyerName"`
	Label      string `json:"label"`
}

// CreateInvoiceCorrectionRequest 是端點 11 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type CreateInvoiceCorrectionRequest struct {
	ExpectedSnapshotVersion string           `json:"expectedSnapshotVersion"`
	NewInvoice              InvoiceInputSpec `json:"newInvoice"`
	Reason                  string           `json:"reason"`
}

// CreateInvoiceComplaintRequest 是端點 13 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type CreateInvoiceComplaintRequest struct {
	ReasonCode  string `json:"reasonCode"`
	Description string `json:"description"`
}

// SaveSellerTaxProfileDraftRequest 是端點 18 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type SaveSellerTaxProfileDraftRequest struct {
	TaxIdentity          SellerTaxIdentity `json:"taxIdentity"`
	EvidenceObjectKeys   []string          `json:"evidenceObjectKeys"`
	ExpectedVersion      string            `json:"expectedVersion"`
	EvidenceStagedTokens []string          `json:"evidenceStagedTokens"`
}

// SubmitSellerTaxProfileRequest 是端點 19 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type SubmitSellerTaxProfileRequest struct {
	ExpectedVersion string `json:"expectedVersion"`
	// AcceptedTerms 是送審表單上「我已閱讀並同意自行開票責任條款」的勾選（裁決 C）。
	//
	// 🔴 false／缺席 ⇒ 後端回 **9325**（HTTP 409），且一筆都不寫。它與端點 22 是
	// 兩條路：這裡是「送審時一併簽」，端點 22 是補簽（S04）。
	AcceptedTerms bool `json:"acceptedTerms"`
}

// RequestSellerTaxExtensionRequest 是端點 20 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type RequestSellerTaxExtensionRequest struct {
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expectedVersion"`
}

// AcceptSelfIssueAttestationRequest 是端點 22 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type AcceptSelfIssueAttestationRequest struct {
	TermsVersion string `json:"termsVersion"`
	Accepted     bool   `json:"accepted"`
}

// PutSellerProviderCredentialsRequest 是端點 24 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type PutSellerProviderCredentialsRequest struct {
	Provider        string `json:"provider"`
	SellerTaxId     string `json:"sellerTaxId"`
	AppKey          string `json:"appKey"`
	ConsentVersion  string `json:"consentVersion"`
	ExpectedVersion string `json:"expectedVersion"`
}

// VerifySellerProviderAccountRequest 是端點 25 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type VerifySellerProviderAccountRequest struct {
	ExpectedVersion string `json:"expectedVersion"`
}

// ActivateSellerProviderAccountRequest 是端點 26 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ActivateSellerProviderAccountRequest struct {
	CutoverAt       string `json:"cutoverAt"`
	ExpectedVersion string `json:"expectedVersion"`
}

// RotateSellerProviderCredentialsRequest 是端點 27 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type RotateSellerProviderCredentialsRequest struct {
	AppKey          string `json:"appKey"`
	ExpectedVersion string `json:"expectedVersion"`
}

// ConfirmSellerReceiptAuthorizationRequest 是端點 29 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ConfirmSellerReceiptAuthorizationRequest struct {
	ConsentVersion  string `json:"consentVersion"`
	Authorized      bool   `json:"authorized"`
	ExpectedVersion string `json:"expectedVersion"`
}

// RespondSellerInvoiceComplaintRequest 是端點 36 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type RespondSellerInvoiceComplaintRequest struct {
	Action          string `json:"action"`
	Note            string `json:"note"`
	ExpectedVersion string `json:"expectedVersion"`
}

// DecideTaxReviewForAdminRequest 是端點 40 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type DecideTaxReviewForAdminRequest struct {
	Decision           string `json:"decision"`
	Reason             string `json:"reason"`
	ApprovalValidUntil string `json:"approvalValidUntil"`
	Confirmed          bool   `json:"confirmed" binding:"required"`
	ExpectedVersion    string `json:"expectedVersion"`
	// 收據代製授權的核定依據文件（58 §6.1.9）：只有 APPROVE_INVOICE_EXEMPT 用得到，
	// 兩欄同進同出，且必須是該賣家自己在 S02 上傳的稅籍文件之一（驗證在 invoice-service）。
	ApprovalBasisKey     string `json:"approvalBasisKey"`
	ApprovalBasisAssetId string `json:"approvalBasisAssetId"`
}

// DecideTaxExtensionForAdminRequest 是端點 41 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type DecideTaxExtensionForAdminRequest struct {
	Decision        string `json:"decision"`
	ExtendDays      string `json:"extendDays"`
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expectedVersion"`
}

// DeactivateProviderAccountForAdminRequest 是端點 45 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type DeactivateProviderAccountForAdminRequest struct {
	Reason          string `json:"reason"`
	Confirmed       bool   `json:"confirmed"`
	ExpectedVersion string `json:"expectedVersion"`
}

// ResendInvoiceNotificationForAdminRequest 是端點 49 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ResendInvoiceNotificationForAdminRequest struct {
	Reason string `json:"reason"`
}

// RetryInvoiceJobForAdminRequest 是端點 52 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type RetryInvoiceJobForAdminRequest struct {
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expectedVersion"`
	FundingDecision string `json:"fundingDecision"`
}

// ResumeInvoiceDocumentForAdminRequest 是端點 73 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
//
// ExpectedVersion 是 **document** 的版本（端點 48 的 `documentVersion`），不是 job 的：
// 這條路徑上還沒有 job，那正是本端點存在的理由。
type ResumeInvoiceDocumentForAdminRequest struct {
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expectedVersion"`
	FundingDecision string `json:"fundingDecision"`
}

// CancelInvoiceJobForAdminRequest 是端點 53 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type CancelInvoiceJobForAdminRequest struct {
	ReasonCode                   string `json:"reasonCode"`
	ReasonDetail                 string `json:"reasonDetail"`
	AllowManualAfterCancellation bool   `json:"allowManualAfterCancellation"`
	ExpectedVersion              string `json:"expectedVersion"`
}

// RequestManualUnlockForAdminRequest 是端點 54 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type RequestManualUnlockForAdminRequest struct {
	ProviderCheckEvidence string `json:"providerCheckEvidence"`
	ProviderCheckedAt     string `json:"providerCheckedAt"`
	Reason                string `json:"reason"`
	ExpectedVersion       string `json:"expectedVersion"`

	// CancelEvidence* 只在 job 是 CANCEL_BLOCKED_UNKNOWN 時填（J28）。
	// kind 只收 MOF_PLATFORM_SCREENSHOT／AMEGO_RECONCILE_FILE 兩個需人判讀的種類；
	// 另兩個 CodeMeta 值是 provider 直接作證、由系統自動落地，人不該替它蓋章。
	// 從 CANCELLED_BY_ADMIN 發起時帶值 ⇒ 400。
	CancelEvidenceKind      string `json:"cancelEvidenceKind"`
	CancelEvidenceObjectKey string `json:"cancelEvidenceObjectKey"`
	// ADM 上傳證據檔後拿到的 staged token（#3108）。與 objectKey **二擇一**：
	// 前端有檔案就送 token（後端換發成 objectKey），只有 key 就直接送 key。
	CancelEvidenceStagedToken string `json:"cancelEvidenceStagedToken"`
}

// ApproveManualUnlockForAdminRequest 是端點 55 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ApproveManualUnlockForAdminRequest struct {
	Approved        bool   `json:"approved"`
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expectedVersion"`
}

// ReportManualResultForAdminRequest 是端點 56 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ReportManualResultForAdminRequest struct {
	InvoiceNo       string `json:"invoiceNo"`
	InvoiceDate     string `json:"invoiceDate"`
	TotalAmount     string `json:"totalAmount"`
	Note            string `json:"note"`
	ExpectedVersion string `json:"expectedVersion"`
}

// CreateManualIssuanceForAdminRequest 是端點 57 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type CreateManualIssuanceForAdminRequest struct {
	Issuer       IssuanceIssuer    `json:"issuer"`
	Recipient    IssuanceRecipient `json:"recipient"`
	Source       IssuanceSource    `json:"source"`
	DocumentKind string            `json:"documentKind"`
	Reason       string            `json:"reason"`
	Confirmed    bool              `json:"confirmed" binding:"required"`
}

// ActOnInvoiceComplaintForAdminRequest 是端點 60 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ActOnInvoiceComplaintForAdminRequest struct {
	Action          string `json:"action"`
	Reason          string `json:"reason"`
	Confirmed       bool   `json:"confirmed" binding:"required"`
	ExpectedVersion string `json:"expectedVersion"`
}

// ReportExternalIssuanceRequest 是端點 62 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ReportExternalIssuanceRequest struct {
	InvoiceNumber     string   `json:"invoiceNumber"`
	InvoiceDate       string   `json:"invoiceDate"`
	EvidenceAssetKeys []string `json:"evidenceAssetKeys"`
	Note              string   `json:"note"`
	ExpectedVersion   string   `json:"expectedVersion"`
}

// RejectInvoiceComplaintResponseRequest 是端點 63 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type RejectInvoiceComplaintResponseRequest struct {
	Reason          string `json:"reason" binding:"required"`
	ExpectedVersion string `json:"expectedVersion"`
}

// WithdrawInvoiceComplaintRequest 是端點 64 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type WithdrawInvoiceComplaintRequest struct {
	ExpectedVersion string `json:"expectedVersion"`
}

// ReopenInvoiceComplaintRequest 是端點 65 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ReopenInvoiceComplaintRequest struct {
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expectedVersion"`
}

// ReactivateProviderAccountForAdminRequest 是端點 66 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type ReactivateProviderAccountForAdminRequest struct {
	Reason          string `json:"reason"`
	Confirmed       bool   `json:"confirmed"`
	ExpectedVersion string `json:"expectedVersion"`
}

// RevokeProviderAccountForAdminRequest 是端點 67 的請求 body。名稱與 §6.2.1 該 RPC 的 Request message 同名（§6.1.0(17)）；
// 身分（buyer_id／store_id／actor_account_id）、path param 與 Idempotency-Key 不在 body 內。
type RevokeProviderAccountForAdminRequest struct {
	Reason          string `json:"reason"`
	Confirmed       bool   `json:"confirmed"`
	ExpectedVersion string `json:"expectedVersion"`
}

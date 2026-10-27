// Package domain 的 repository 契約層。
//
// 本檔是 §4.2.1 目錄表「domain/repository（介面）」那一列的落點，只放兩樣東西：
//
//	① 各聚合的框架無關實體（pgtype / json.RawMessage 之外的一切都是純 Go 型別）
//	② repository 介面本身
//
// 為什麼實體與介面同檔：介面沒有它引用的型別就不成立，拆兩檔只會讓「改欄位要動哪些檔」
// 這件事變模糊。domain 其餘檔案是純函式（金額不變式、期別換算），與本檔零耦合。
//
// 🔴 本檔刻意沒有「AMEGO AppKey 明文」這種欄位。明文只在
// internal/repository/postgres/mapper.go 的解密函式與 WithDecryptedCredential 的
// callback 區域變數裡存在（58 §8.1.1(c)(h)）——把它放進任何一個會被回傳、被 log、
// 被序列化的 struct，write-only 語意當場失效，而那不會有任何編譯錯或測試紅。
package domain

import (
	"context"
	"encoding/json"
	"time"
)

// ProviderAccount 是 invoice_provider_accounts 的**去敏投影**，不是整列鏡射。
//
// 缺席的四欄是刻意的，逐欄理由（58 §8.1.1(b)(f)、§8.1.3）：
//   - credential_ciphertext / pending_ciphertext：密文出了本層就再也沒有邊界能擋住它被
//     寫進 log 或 response；需要明文的唯一路徑是 WithDecryptedCredential。
//   - credential_fingerprint / pending_fingerprint：§8.1.3 的 C 類憑證衍生值。
//     uq_ipa_fingerprint 的語意讓「兩個帳戶用同一把 key」變成任何拿到兩段前綴的人
//     都能自行驗證的事實 ⇒ Phase 1 一律不外露到 API、DTO、畫面或 log。
//
// credential_key_version 留著：它是維運面的事實（這一列由哪一版 master key 加密），
// 不是憑證本身，master key 輪替的驗證述詞需要讀它。
type ProviderAccount struct {
	ID                   int64
	IssuerID             int64
	Provider             string
	ProviderTaxID        string
	CredentialKeyVersion int16
	CredentialLast4      string
	Status               string
	StatusReason         *string
	VerifiedAt           *time.Time
	AutomationCutoverAt  *time.Time
	LastHealthCheckedAt  *time.Time
	HealthCheckNextAt    time.Time
	BreakerOpenedAt      *time.Time
	BreakerProbeSuccess  int16
	// HasPendingCredential 取代 pending_ciphertext 的存在性判斷：呼叫端要知道的是
	// 「現在是不是輪替中」，不是那串密文長什麼樣。
	HasPendingCredential bool
	PendingLast4         *string
	RotationStartedAt    *time.Time
	Version              int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	// CredentialUpdatedAt 是憑證專用的時間戳（#3051）。
	//
	// 🔴 **不要拿 UpdatedAt 當它**：健康檢查輪詢每分鐘改寫 updated_at（dev 實查同一列
	// version 已被推到 225，而憑證只設定過一次），而這一欄是賣家唯一能察覺
	// 「我的 AppKey 被換過嗎」的訊號——AppKey write-only，平台與賣家都看不到全文。
	// 舊列由 migration 以 created_at 回填 ⇒ 實務上非空，型別仍留指標因為欄位可為 NULL。
	CredentialUpdatedAt *time.Time
}

// ============================================================================
// 實體（gen row 的框架無關鏡射）
// ============================================================================

// Issuer mirrors the InvoiceIssuer row with framework-free types.
type Issuer struct {
	ID          int64
	IssuerType  string
	IssuerRefID int64
	DisplayName string
	Status      string
	RetiredAt   *time.Time
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SellerTaxProfile mirrors the SellerTaxProfile row with framework-free types.
type SellerTaxProfile struct {
	ID                int64
	IssuerID          int64
	ReviewStatus      string
	ClaimedObligation string
	EntityTaxType     *string
	LegalName         *string
	TaxID             *string
	ResponsiblePerson *string
	BusinessAddress   *string
	// AddressCity／AddressDistrict：三段地址的前兩段（#3031）。
	// business_address 是串接後的完整地址，折起來拆不回去 ⇒ 回程要還原到表單就得分開存。
	AttestedByAccountID     *int64
	AddressCity             *string
	AddressDistrict         *string
	DefaultTaxCategory      int16
	EvidenceObjectKeys      json.RawMessage
	AttestationVersion      *string
	AttestedAt              *time.Time
	SubmittedAt             *time.Time
	PendingValidUntil       *time.Time
	ExtensionCount          int16
	PendingOrderLimitCount  int32
	PendingOrderLimitAmount int64
	PendingOrderCount       int32
	PendingOrderAmount      int64
	ReviewNote              *string
	// ReviewNoteAt 是 ReviewNote **最後一次真的改變**的時點（#3089 第 11 項）。
	//
	// 🔴 不得用整列的 UpdatedAt 代替：BumpPendingOrderCounters 會為每一筆
	// 已付款訂單推進它 ⇒ A03 上那則備註的時間會隨接單量一直往前跳。
	ReviewNoteAt       *time.Time
	ApprovedAt         *time.Time
	CooldownUntil      *time.Time
	ApprovalValidUntil *time.Time
	RecheckReason      *string
	// 收據代製授權四欄（§5.1.2／§6.1.9 S05，端點 28／29）。
	//
	// ReceiptAuthorizedAt／ReceiptConsentVersion 同進同出（chk_stp_receipt_auth_pair）：
	// 兩者都非 nil ＝ 已授權；都是 nil ＝ 未授權或已取消。
	// ReceiptApprovalBasisKey／AssetID 同理，且由 **ADM** 在端點 40 指名，不是賣家自己填。
	ReceiptAuthorizedAt         *time.Time
	ReceiptConsentVersion       *string
	ReceiptApprovalBasisKey     *string
	ReceiptApprovalBasisAssetID *int64
	ViolationWindowDays         int16
	ViolationThreshold          int16
	SuspensionDays              int16
	Version                     int64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
}

// Document mirrors the InvoiceDocument row with framework-free types.
type Document struct {
	ID                          int64
	IssuerID                    int64
	IssuerNameSnapshot          string
	IssuerTaxIDSnapshot         *string
	SourceType                  string
	SourceRef                   *string
	TriggerType                 string
	RecipientType               string
	RecipientRefID              *int64
	RecipientName               string
	RecipientTaxID              string
	RecipientEmailCiphertext    *string
	RecipientEmailHash          *string
	RecipientPhoneCiphertext    *string
	CarrierType                 *string
	CarrierId1Ciphertext        *string
	CarrierId2Ciphertext        *string
	CarrierValueHash            *string
	DonationCode                *string
	DocumentKind                string
	FulfillmentMode             string
	Status                      string
	NotApplicableReason         *string
	ManualReasonCode            *string
	CancelReasonCode            *string
	CancelReason                *string
	SnapshotVersion             int32
	IssueAttemptSeq             int32
	ProviderAccountVersion      *int64
	SupersededByID              *int64
	OriginalDocumentID          *int64
	SalesAmount                 int64
	FreeTaxSalesAmount          int64
	ZeroTaxSalesAmount          int64
	TaxAmount                   int64
	TotalAmount                 int64
	TaxType                     int16
	TaxRateBps                  int32
	AllowanceTotalAmount        int64
	VoidedAt                    *time.Time
	Provider                    *string
	ProviderRequestNo           *string
	ProviderInvoiceNo           *string
	ProviderRandomCode          *string
	ProviderIssuedAt            *time.Time
	ExternalInvoiceNo           *string
	ExternalInvoiceDate         *time.Time
	ExternalReportedAt          *time.Time
	ProviderUploadStatus        *int16
	ProviderUploadCheckedAt     *time.Time
	UploadRecheckNextAt         *time.Time
	UploadRecheckAttempts       int16
	ReceiptNo                   *string
	ReceiptAuthorizationVersion *string
	PdfObjectKey                *string
	PdfFetchNextAt              *time.Time
	PdfFetchAttempts            int16
	InvoiceDateTaipei           *time.Time
	TaxPeriodCode               *string
	IssueNotBefore              time.Time
	// PaidAt 是該筆訂單的收款時點（COALESCE(provider_paid_at, pay_time)）。
	//
	// 🔴 **與 IssueNotBefore 不是同一件事，即使 Phase 1 的自動開票路徑上兩者相等**：
	// IssueNotBefore 問「最早什麼時候可以開」，PaidAt 問「錢什麼時候進來」。
	// ADM 手動開票與作廢重開的新票，IssueNotBefore 是「現在」而收款早就發生過
	// ⇒ 拿 IssueNotBefore 當 paidAt 會在那兩條路徑上給出錯的日期，而它是賣家對帳的依據。
	// 落成自己的欄位而不是查詢時跨服務取：收款時點是**開票當下就固定的事實**，
	// 不會再變；而 S06 一頁 20 列，逐列打 payment-service 是 20 次網路往返。
	PaidAt              *time.Time
	StatutoryDeadlineAt *time.Time
	SourceSnapshot      json.RawMessage
	// NotificationResentAt／NotificationResendCount 是重寄窗口（§6.1.10 端點 32／49／71）。
	// 兩者同進同出（chk_invdoc_resend_pair）；次數同時是 notification_ref_id 的去重種子。
	NotificationResentAt    *time.Time
	NotificationResendCount int32
	Version                 int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// DocumentLine mirrors the InvoiceDocumentLine row with framework-free types.
type DocumentLine struct {
	ID                int64
	DocumentID        int64
	LineSeq           int32
	LineType          string
	OrderItemID       *int64
	SkuID             *int64
	Description       string
	Unit              *string
	Quantity          int32
	UnitPrice         int64
	NetAmount         int64
	TaxCategory       int16
	TaxRateBps        int32
	Remark            *string
	AllocatedDiscount int64
	AllowanceableBase int64
	AllowancedAmount  int64
	CreatedAt         time.Time
}

// Operation mirrors the InvoiceOperation row with framework-free types.
type Operation struct {
	ID                  int64
	DocumentID          int64
	AdjustmentID        *int64
	OperationType       string
	OperationKey        string
	ProviderRequestNo   string
	Status              string
	RequestSnapshotHash string
	AttemptCount        int32
	MaxAttempts         int32
	NextAttemptAt       time.Time
	HeldUntil           *time.Time
	RecheckListPage     int16
	StatutoryDeadlineAt *time.Time
	ProviderCode        *string
	ProviderMessage     *string
	UnknownResult       bool
	UnknownSince        *time.Time
	CancelRequestedBy   *int64
	CancelRequestedAt   *time.Time
	CancelConfirmedBy   *int64
	CancelConfirmedAt   *time.Time
	CancelReasonCode    *string
	CancelReason        *string
	StartedAt           *time.Time
	CompletedAt         *time.Time
	LastError           *string
	Version             int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Adjustment mirrors the InvoiceAdjustment row with framework-free types.
type Adjustment struct {
	ID                      int64
	DocumentID              int64
	AdjustmentType          string
	Status                  string
	TriggerKind             string
	RefundNo                *string
	CorrectionRequestID     *int64
	AllowanceNumber         *string
	AllowanceDateTaipei     *time.Time
	NetAmountExTax          *int64
	TaxAmount               *int64
	GrossAmount             *int64
	VoidReasonCode          *string
	VoidReasonDetail        *string
	BlockedReasonCode       *string
	CancelReasonCode        *string
	ProviderConfirmedAt     *time.Time
	ProviderUploadStatus    *int16
	ProviderUploadCheckedAt *time.Time
	PdfObjectKey            *string
	Version                 int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// AdjustmentLine mirrors the InvoiceAdjustmentLine row with framework-free types.
type AdjustmentLine struct {
	ID                  int64
	AdjustmentID        int64
	LineSeq             int32
	SourceLineID        *int64
	OriginalInvoiceNo   string
	OriginalInvoiceDate *time.Time
	OriginalDescription string
	Quantity            int32
	UnitPriceExTax      int64
	NetAmountExTax      int64
	TaxAmount           int64
	TaxCategory         int16
	CreatedAt           time.Time
}

// Complaint mirrors the InvoiceComplaint row with framework-free types.
type Complaint struct {
	ID                 int64
	DocumentID         int64
	IssuerID           int64
	OrderNo            *string
	ComplainantBuyerID int64
	ReasonCode         string
	Description        *string
	Status             string
	SlaDueAt           time.Time
	OverdueAt          *time.Time
	BuyerReviewDueAt   *time.Time
	FirstRespondedAt   *time.Time
	ResolvedAt         *time.Time
	ResolutionCode     *string
	ResolutionNote     *string
	ReopenCount        int16
	ReopenAllowedAfter *time.Time
	DecidedByAccountID *int64
	DecidedAt          *time.Time
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ComplaintEvent mirrors the InvoiceComplaintEvent row with framework-free types.
type ComplaintEvent struct {
	ID                   int64
	ComplaintID          int64
	EventType            string
	Action               *string
	ActorType            string
	ActorAccountID       *int64
	Message              *string
	AttachmentObjectKeys json.RawMessage
	OccurredAt           time.Time
}

// CorrectionRequest mirrors the InvoiceCorrectionRequest row with framework-free types.
type CorrectionRequest struct {
	ID                      int64
	DocumentID              int64
	RequestedByBuyerID      int64
	ExpectedSnapshotVersion int32
	BeforePayload           json.RawMessage
	AfterPayload            json.RawMessage
	Status                  string
	HandlingMode            *string
	RejectReasonCode        *string
	RejectReasonDetail      *string
	VoidAdjustmentID        *int64
	ReissuedDocumentID      *int64
	Version                 int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
	CompletedAt             *time.Time
}

// BuyerPreference mirrors the BuyerInvoicePreference row with framework-free types.
type BuyerPreference struct {
	ID                     int64
	BuyerID                int64
	PreferenceKind         string
	CarrierType            *string
	CarrierValueCiphertext *string
	CarrierValueHash       *string
	CarrierValueMask       *string
	TaxID                  *string
	TaxTitle               *string
	DonationCode           *string
	Label                  *string
	IsDefault              bool
	SupersededAt           *time.Time
	SupersededByID         *int64
	DeletedAt              *time.Time
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// InboxEvent mirrors the InvoiceInboxEvent row with framework-free types.
type InboxEvent struct {
	ID                int64
	EventID           string
	EventType         string
	RoutingKey        string
	Exchange          string
	BusinessDedupKey  string
	AggregateRef      *string
	Payload           json.RawMessage
	Status            string
	Disposition       *string
	DispositionReason *string
	AttemptCount      int32
	NextAttemptAt     time.Time
	DeferredUntil     *time.Time
	LastError         *string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	ProcessedAt       *time.Time
	PayloadPurgedAt   *time.Time
}

// OutboxEvent mirrors the InvoiceOutboxEvent row with framework-free types.
type OutboxEvent struct {
	ID            int64
	AggregateType string
	AggregateID   int64
	EventType     string
	Exchange      string
	RoutingKey    string
	Payload       json.RawMessage
	Status        string
	AttemptCount  int32
	NextAttemptAt time.Time
	LastError     *string
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// StatusHistoryEntry mirrors the InvoiceStatusHistory row with framework-free types.
type StatusHistoryEntry struct {
	ID             int64
	SubjectType    string
	SubjectID      int64
	FromStatus     *string
	ToStatus       string
	ReasonCode     *string
	ReasonDetail   *string
	ActorType      string
	ActorAccountID *int64
	CorrelationID  *string
	OccurredAt     time.Time
}

// ProviderAuditLogEntry mirrors the InvoiceProviderAuditLog row with framework-free types.
type ProviderAuditLogEntry struct {
	ID              int64
	OperationID     *int64
	IssuerID        int64
	Provider        string
	Endpoint        string
	AttemptSeq      int32
	RequestMasked   json.RawMessage
	ResponseMasked  json.RawMessage
	RawBodyHash     string
	HttpStatus      *int32
	ProviderCode    *string
	ProviderMessage *string
	LatencyMs       *int32
	ErrorKind       *string
	CorrelationID   *string
	OccurredAt      time.Time
}

// AdminAuditLogEntry mirrors the InvoiceAdminAuditLog row with framework-free types.
type AdminAuditLogEntry struct {
	ID                     int64
	ActorAccountID         int64
	Action                 string
	ResourceType           string
	ResourceID             int64
	BeforeState            json.RawMessage
	AfterState             json.RawMessage
	ReasonCode             string
	ReasonDetail           string
	ApprovalActorAccountID *int64
	TraceID                *string
	RequestID              *string
	CorrelationID          *string
	ClientIp               *string
	UserAgent              *string
	OccurredAt             time.Time
}

// ============================================================================
// 寫入參數（domain 側）
// ============================================================================

// NewProviderAccount 是首次綁定憑證的輸入。AppKey 是**明文**，方向是往內（§8.1.1(h) 第 3 段）；
// repository 負責加密、算 fingerprint 與 last4，呼叫端不得自己算其中任何一個。
type NewProviderAccount struct {
	ID                int64
	IssuerID          int64
	Provider          string
	ProviderTaxID     string
	AppKey            string
	Status            string
	HealthCheckNextAt time.Time
}

// StartCredentialRotation 是「賣家送了一把新 AppKey」的輸入（§8.1.1(e) 第一條邊）。
type StartCredentialRotation struct {
	ID              int64
	ProviderTaxID   string
	AppKey          string
	ExpectedVersion int64
}

// PromoteCredential 把 pending_* 扶正成 credential_*（§8.1.1(e) 第二條邊）。
//
// 它帶的是明文而不是「把 pending 搬到 credential」的純 SQL：pending 密文的解密與
// 重新加密都必須經過 mapper 的收斂點，而 credential_key_version 一律寫成當前版本
// （§8.1.1(d) 末段的不變式：沒有 pending_key_version 欄）。
type PromoteCredential struct {
	ID              int64
	ProviderTaxID   string
	AppKey          string
	NextStatus      string
	ExpectedVersion int64
}

// UpdateProviderAccountStatus 是 ADM／賣家路徑的狀態轉移（CAS）。
type UpdateProviderAccountStatus struct {
	ID                int64
	NextStatus        string
	StatusReason      *string
	VerifiedAt        *time.Time
	HealthCheckNextAt time.Time
	ExpectedVersion   int64
}

// ResubmitProviderCredential 是端點 24 覆寫既有列的輸入。AppKey 是**明文**，方向往內
// （與 NewProviderAccount.AppKey 同一個約定）：加密、fingerprint 與 last4 都由 repository
// 的 deriveCredential 算，呼叫端不得自己算其中任何一個。
type ResubmitProviderCredential struct {
	ID                int64
	ProviderTaxID     string
	AppKey            string
	HealthCheckNextAt time.Time
	ExpectedVersion   int64
}

// LandProviderVerification 是驗證結果的落點（P03–P06，以及端點 44 從 ACTIVE／DEGRADED
// 直落錯誤狀態的那一格）。
//
// 🔴 VerifiedAt **必須逐次明給**：它是 sqlc.narg，傳 nil 會把欄位清成 NULL。落 ACTIVE 時
// 給 now，落失敗狀態時給該列**原本的值**——清空它會讓已設定 automation_cutover_at 的列
// 當場 23514（chk_ipa_cutover_requires_verified）。
type LandProviderVerification struct {
	ID           int64
	NextStatus   string
	StatusReason string
	VerifiedAt   *time.Time
	// BreakerOpenedAt 是「這個帳戶從什麼時候開始連不上」的錨，由 §7.3.4 P08 的 24 小時
	// 上界讀。落 PROVIDER_UNAVAILABLE 時給 now、其餘狀態給 nil；漏給＝自動重驗靜默失效。
	BreakerOpenedAt *time.Time
	NextCheckAt     time.Time
	ExpectedVersion int64
	Now             time.Time
}

// RecordHealthCheckResult 是 INV.hc 一輪巡檢的結果落點。
//
// NextCheckAt 由呼叫端算（巡檢週期，不是租約）：少了它，claim 推上去的租約會變成
// 事實上的巡檢週期（§5.1.3 query 註解）。
type RecordHealthCheckResult struct {
	ID                  int64
	CheckedAt           time.Time
	NextCheckAt         time.Time
	NextStatus          string
	StatusReason        *string
	BreakerOpenedAt     *time.Time
	BreakerProbeSuccess int16
}

// FailOpenOperationsForIssuer 是 P14／P16 集合式副作用的輸入。
//
// CancelReasonCode 的值域是 CodeMeta InvoiceCancelReason（P14／P16 一律
// PROVIDER_SUSPENDED_BY_ADMIN），**不是** InvoiceManualReason——後者是 P7 事件的
// review_reason_code，兩張碼表名字相近、語意不同（見交付回報的契約缺口欄）。
type FailOpenOperationsForIssuer struct {
	IssuerID         int64
	CancelReasonCode string
	LastError        string
	Now              time.Time
}

// ProviderAccountRepository 是 invoice_provider_accounts 的唯一存取面。
type ProviderAccountRepository interface {
	GetByID(ctx context.Context, id int64) (ProviderAccount, error)
	GetByIssuer(ctx context.Context, issuerID int64, provider string) (ProviderAccount, error)
	ListActive(ctx context.Context) ([]ProviderAccount, error)

	// FetchDueHealthChecks / ClaimHealthCheck 是 INV.hc 的候選與搶佔。
	// ClaimHealthCheck 回 false 代表沒搶到（別的副本先到、或述詞已不成立），不是錯誤。
	FetchDueHealthChecks(ctx context.Context, now, unavailableSinceFloor time.Time, limit int32) ([]ProviderAccount, error)
	ClaimHealthCheck(ctx context.Context, id int64, now, leaseUntil, unavailableSinceFloor time.Time) (bool, error)
	RecordHealthCheckResult(ctx context.Context, in RecordHealthCheckResult) error

	Create(ctx context.Context, in NewProviderAccount) error
	StartCredentialRotation(ctx context.Context, in StartCredentialRotation) error
	PromoteCredential(ctx context.Context, in PromoteCredential) error
	UpdateStatus(ctx context.Context, in UpdateProviderAccountStatus) error

	// ── §6.1.8／§6.1.13 的狀態轉移（端點 24／25／26／27／44／45／66／67）──
	//
	// 🔴 七支的 allowed-from **全部寫在 SQL 的 WHERE**（db/queries/invoice_provider_accounts.sql），
	// 本層與 usecase 都不重述那份清單。回 false ＝ 守衛不成立（allowed-from 不符或版本被
	// 搶先），呼叫端必須回明確業務碼——把 false 當成功往下走就是 §7.3.0 規則 4 的靜默失敗。

	// ResubmitCredential 是端點 24 的覆寫分支（既有列）。落地即 CREDENTIAL_VERIFYING。
	// taken=true ＝ 撞 uq_ipa_fingerprint（這組統編＋AppKey 已被另一個同時有效的 issuer
	// 綁走）⇒ 呼叫端回 9337 並**不得再下任何一句 SQL**：PostgreSQL 的交易此刻已 aborted。
	ResubmitCredential(ctx context.Context, in ResubmitProviderCredential) (ok bool, taken bool, err error)

	// StartVerification 是 P02／P07／P15 的共同入口（→ CREDENTIAL_VERIFYING）。
	StartVerification(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error)
	// LandVerification 是 P03／P04／P05／P06 的共同落點（allowed-from ＝ CREDENTIAL_VERIFYING）。
	LandVerification(ctx context.Context, in LandProviderVerification) (bool, error)
	// FailVerificationFromActive 是 §6.1.13 端點 44 定版 ③ 的前半：ACTIVE／DEGRADED 的憑證
	// 確實壞了 ⇒ 直接落成賣家修得掉的狀態，**不經過 CREDENTIAL_VERIFYING**（§7.3.4 沒有
	// ACTIVE → CREDENTIAL_VERIFYING 這條邊）。
	FailVerificationFromActive(ctx context.Context, in LandProviderVerification) (bool, error)
	// TouchHealthCheck 是同一條定版 ③ 的後半：**status 完全不動**，只寫「真的打了 AMEGO」。
	TouchHealthCheck(ctx context.Context, id, expectedVersion int64, now, nextCheckAt time.Time) (bool, error)

	// AbortCredentialRotation 是 P13（新 key 驗證失敗 ⇒ 回 ACTIVE 並保留原 key）。
	AbortCredentialRotation(ctx context.Context, id, expectedVersion int64, statusReason string, now time.Time) (bool, error)

	// SetAutomationCutover 是端點 26（I-61：只能在 status='ACTIVE' 當下設定，一經設定永不清空）。
	SetAutomationCutover(ctx context.Context, id, expectedVersion int64, cutoverAt, now time.Time) (bool, error)

	// Suspend／Revoke 是 P14／P16。兩者都**不動** automation_cutover_at（歷史訂單的模式判定
	// 要用它），也都不動 verified_at（清空它會讓已設切點的列當場 23514）。
	Suspend(ctx context.Context, id, expectedVersion int64, reason string, now time.Time) (bool, error)
	Revoke(ctx context.Context, id, expectedVersion int64, reason string, now time.Time) (bool, error)

	// MatchesCurrentCredential 回答 §8.1.1(f) U3：賣家送來的這把 key 是不是就是現行那把。
	// 比對在 repository 內完成，fingerprint 不外流（§8.1.3 C 類）。
	MatchesCurrentCredential(ctx context.Context, id int64, providerTaxID, appKey string) (bool, error)

	// WithDecryptedCredential 是 AppKey 明文唯一的出境形式，而且它不出境：明文只以參數
	// 形式進入 fn 的區域變數，永遠不是回傳值 ⇒ 不可能被誤存進 struct、log 或 response。
	// fn 回傳的 error 原樣往上傳。
	WithDecryptedCredential(ctx context.Context, id int64, fn func(providerTaxID, appKey string) error) error

	// ── 讀取視圖 ──
	// GetByStoreID 經 invoice_issuers 解析；查無列回 ErrNoRows ⇒ 單筆端點翻 9330。
	GetByStoreID(ctx context.Context, storeID int64, provider string) (ProviderAccount, error)
	// ListForAdmin 的 FROM 是 invoice_issuers 而不是本表，且是 LEFT JOIN：
	// 「查無 provider account 列」要投影成 NOT_STARTED，而那是上線當天的全體。
	// Item.Account 為 nil 就是那個情況，**不得當成錯誤或跳過該列**。
	ListForAdmin(ctx context.Context, in AdminProviderAccountQuery) ([]AdminProviderAccountItem, error)
	CountForAdmin(ctx context.Context, in AdminProviderAccountQuery) (int64, error)
}

// IssuerRepository 讀 invoice_issuers。
type IssuerRepository interface {
	GetByID(ctx context.Context, id int64) (Issuer, error)
	GetByRef(ctx context.Context, issuerType string, issuerRefID int64) (Issuer, error)

	// Create 建立開票主體。inserted=false ＝ ON CONFLICT DO NOTHING 的 rows=0
	// （uq_invoice_issuers_ref 已有這個 (issuer_type, issuer_ref_id)），呼叫端改讀 GetByRef。
	//
	// 🔴 這是賣家進入發票流程的**唯一**建列點（端點 18 的草稿存檔）。在它接上之前
	// invoice_issuers 沒有任何寫入路徑 ⇒ seller_tax_profiles 的 FK 永遠找不到對象，
	// 整條 S02 一列都建不出來。
	Create(ctx context.Context, in NewIssuer) (Issuer, bool, error)

	// UpdateDisplayName 把法定名稱同步進 display_name（開票時的銷售人名稱）。
	// 值相同時 rows=0，那是 no-op 不是守衛不成立。
	UpdateDisplayName(ctx context.Context, id int64, displayName string, now time.Time) (bool, error)
}

// NewIssuer 是建立一列 invoice_issuers 的輸入。
//
// DisplayName 是**法定名稱**不是店名（§5.1.1）。賣家第一次存草稿時可能還沒填，
// 呼叫端要放一個可辨識的佔位字串，並在法定名稱出現時以 UpdateDisplayName 覆蓋。
type NewIssuer struct {
	ID          int64
	IssuerType  string
	IssuerRefID int64
	DisplayName string
	Now         time.Time
}

// EvidenceKeys 把 evidence_object_keys 折成有序的 objectKey 清單。
//
// 清單的順序就是 TaxDocumentInfo.document_id 的定義（1 起算的位置）：GetTaxReview 的
// 附件清單與後台的稅籍文件下載端點都從這裡取，兩邊的「第 N 份」才是同一份。
// 解不開時回 nil：DDL 的 CHECK 保證它是陣列，解不開代表資料層已壞，
// 而讓詳情頁 500 對 ADM 沒有幫助；下載端點則因此一律查無。
func (p SellerTaxProfile) EvidenceKeys() []string {
	if len(p.EvidenceObjectKeys) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(p.EvidenceObjectKeys, &out); err != nil {
		return nil
	}
	return out
}

// SellerTaxProfileRepository 讀寫 seller_tax_profiles。
type SellerTaxProfileRepository interface {
	GetByID(ctx context.Context, id int64) (SellerTaxProfile, error)
	GetByIssuer(ctx context.Context, issuerID int64) (SellerTaxProfile, error)
	CountApprovalRecheckDue(ctx context.Context, now time.Time) (int64, error)
	// BumpPendingOrderCounters 累加待審期間的接單額度（§5.1.2 口徑：已付款訂單，
	// **只在建立 invoice_documents 的同交易內累加**）。
	//
	// 🔴 呼叫端必須先確認賣家處於待審窗口內（SELF_ISSUE_PENDING／SUPPLEMENT_REQUIRED）。
	// 無條件累加會壞掉整個機制：這兩個計數器**沒有任何遞減或重置路徑**（遞減需要有人在
	// 窗口滑出時掃描，那是《後端服務責任邊界規範》禁止的 reconciliation job）⇒ 一個
	// 歷史悠久的賣家哪天進入待審，會被累積總數瞬間擋死，而症狀是結帳回 9326。
	BumpPendingOrderCounters(ctx context.Context, id, orderAmount int64, now time.Time) error
	// ExpirePending 是集合式前置 UPDATE（無外部副作用 ⇒ 不需要 claim），回被改到的列。
	ExpirePending(ctx context.Context, now, cooldownUntil time.Time, limit int32) ([]SellerTaxProfile, error)

	// ConfirmReceiptAuthorization 是端點 29（S05 賣家同意／取消平台代製收據）。
	//
	// 🔴 資格守衛在 SQL：`review_status = 'APPROVED_INVOICE_EXEMPT'` ⇒ rows=0 回 9335。
	// authorized=false 時 AuthorizedAt／ConsentVersion 都傳 nil（兩欄同進同出，
	// chk_stp_receipt_auth_pair 會擋只清一欄）。
	ConfirmReceiptAuthorization(ctx context.Context, in ConfirmReceiptAuthorization) (bool, error)

	// ── 讀取視圖 ──
	// GetByStoreID 經 invoice_issuers 解析 store_id（seller_tax_profiles 上只有
	// issuer_id）。賣家路徑用它就是 owner-scoped（store_id 由 Gateway resolver 注入）。
	GetByStoreID(ctx context.Context, storeID int64) (SellerTaxProfile, error)
	ListReviewsForAdmin(ctx context.Context, in AdminTaxReviewQuery) ([]AdminTaxReviewItem, error)
	CountReviewsForAdmin(ctx context.Context, in AdminTaxReviewQuery) (int64, error)

	// ── 寫入面：M3 稅務資格狀態機（§7.3.3 T01–T16）──
	//
	// 🔴 每一支回的 bool 是 **CAS 的 rows>0**，不是「有沒有出錯」。allowed-from、
	// expectedVersion、冷卻、延長次數全部寫在 SQL 的 WHERE（db/queries/seller_tax_profiles.sql）
	// ⇒ false ＝ 守衛不成立，呼叫端必須回明確業務碼。**把 false 當成功往下走就是靜默失敗**：
	// ADM 按下去沒反應、賣家送出後狀態沒動，兩者都不會有任何錯誤訊號。
	//
	// 🔴 這一組**不重複判斷守衛**：Go 端先讀一次是為了回可分辨的碼（9321／9322／9323），
	// 不是正確性防線——兩個分頁同時送出時，應用層的 if 會各自讀到舊值再各自寫。

	// Create 建 DRAFT（T01 的 `[*]` 形態）。inserted=false ＝ 該 issuer 已有 profile。
	Create(ctx context.Context, id, issuerID int64, now time.Time) (SellerTaxProfile, bool, error)
	// ResetToDraft 是 T01 的 REJECTED／EXPIRED → DRAFT，7 天冷卻寫在 SQL 的 WHERE。
	// ⚠️ 它**沒有** expectedVersion：冷卻本身就是併發守衛，而重申的入口是「賣家又按了一次」。
	ResetToDraft(ctx context.Context, id int64, now time.Time) (bool, error)
	// UpdateDraft 是端點 18 的草稿存檔（狀態不動，allowed-from 只有 DRAFT）。
	UpdateDraft(ctx context.Context, in UpdateSellerTaxDraft) (bool, error)
	// Submit 是 T02 DRAFT → SUBMITTED。pendingValidUntil 由呼叫端算好（單一時鐘，決策 C4）。
	//
	// 🔴 attestationVersion 是**自行開票責任條款的簽署記錄**，與狀態、兩個時鐘欄在
	// **同一句 UPDATE** 裡寫（裁決 C）。不變式：送審成功 ⇒ 條款必已簽，
	// 由 schema 的 chk_stp_attested_when_committed 保證，不是靠呼叫順序或呼叫端紀律。
	// ⚠️ **空字串不是「保留原值」而是錯誤**：它會撞 chk_stp_attestation_version_nonblank
	// 當場 23514、一列不寫。呼叫端本來就不該傳空（版本取服務端常數，
	// accepted_terms=false 在進交易前已被擋成 9325），傳了就是程式錯誤，要吵。
	// Submit：attestedBy 是**簽署人**的 account_id（#3031）。條款是自然人的意思表示，
	// 只有版本與時間、沒有人，爭議時舉不出簽署主體。0 ⇒ 寫 NULL。
	Submit(ctx context.Context, id, expectedVersion int64, attestationVersion string, attestedBy int64, pendingValidUntil, now time.Time) (bool, error)
	// Attest 是 T04 SUBMITTED → SELF_ISSUE_PENDING（**不重算 pending_valid_until**）。
	Attest(ctx context.Context, id, expectedVersion int64, attestationVersion string, attestedBy int64, now time.Time) (bool, error)
	// RequestSupplement 是 T05 UNDER_REVIEW → SUPPLEMENT_REQUIRED（不重置 60 天）。
	RequestSupplement(ctx context.Context, id, expectedVersion int64, note string, now time.Time) (bool, error)
	// Approve 是 T07／T08。nextStatus 必須是 APPROVED_INVOICE_REQUIRED／APPROVED_INVOICE_EXEMPT
	// 之一，傳錯值由 chk_stp_review_status 當場 23514（不是靜默寫進去）。
	// Approve 是 T07／T08。核定免用統一發票時，**收據代製授權的核定依據文件在同一句
	// CAS 裡一起寫**（§6.1.9 由 ADM 在端點 40 指名）——不另開一句 UPDATE，理由見
	// db/queries/seller_tax_profiles.sql 的紅字（第二句的 expected_version 會變成
	// 「第一句 +1」，那是在 Go 端重建一份 SQL 已經算過的算式）。
	Approve(ctx context.Context, in ApproveSellerTaxProfile) (bool, error)
	// Reject 是 T09。cooldownUntil ＝ now + 7 天，少寫它被駁回的賣家可以立刻無限重送。
	Reject(ctx context.Context, id, expectedVersion int64, note string, cooldownUntil, now time.Time) (bool, error)
	// ExtendPending 是 T11（狀態不變，就地改寫 pending_valid_until ＋ extension_count = 1）。
	ExtendPending(ctx context.Context, id, expectedVersion int64, pendingValidUntil time.Time, note string, now time.Time) (bool, error)
	// ReopenReview 是 T12／T13／T15 → UNDER_REVIEW（四個來源狀態共用一句）。
	ReopenReview(ctx context.Context, id, expectedVersion int64, recheckReason string, now time.Time) (bool, error)
	// Revoke 是 T14 APPROVED_* → REVOKED。
	Revoke(ctx context.Context, id, expectedVersion int64, note string, now time.Time) (bool, error)
	// KeepUnderReview 是 T16 的自轉移（UNDER_REVIEW → UNDER_REVIEW，只留痕）。
	KeepUnderReview(ctx context.Context, id, expectedVersion int64, note string, now time.Time) (bool, error)
}

// UpdateSellerTaxDraft 是端點 18 草稿存檔的輸入。
//
// 🔴 EvidenceObjectKeys 進來時**已經是換發後的 objectKey**（§6.1.7 換發表：端點 18 在寫本表
// 之前把 evidenceStagedTokens[] 換成 objectKey）。T02 的守衛讀的正是本欄 ⇒ 這裡少寫，
// 賣家永遠送不出審核，而畫面上草稿存檔是成功的。
//
// 空字串一律寫成 NULL（repository 負責）：tax_id 有 chk_stp_tax_id 的 8 碼 regex，
// 空字串會當場 23514 而不是「還沒填」。
type UpdateSellerTaxDraft struct {
	ID                 int64
	ExpectedVersion    int64
	ClaimedObligation  string
	EntityTaxType      string
	LegalName          string
	TaxID              string
	ResponsiblePerson  string
	BusinessAddress    string
	AddressCity        string
	AddressDistrict    string
	DefaultTaxCategory int16
	EvidenceObjectKeys []string
	ReviewNote         string
	Now                time.Time
}

// ============================================================================
// 開票主鏈的寫入參數（§7.1.2 步驟 8–9、步驟 14）
// ============================================================================

// NewDocument 是建立一列 invoice_documents 的輸入。
//
// 🔴 四個 recipient 欄位是**明文**，方向是往內（與 NewProviderAccount.AppKey 同一個約定）：
// repository 負責加密與算 HMAC lookup hash，呼叫端不得自己算其中任何一個，也不得把密文
// 從別處抄進來。明文的活動範圍到 repository 的加密函式為止。
//
// 空字串一律寫成 NULL。這對三組欄位是**語意的一部分**而不是便利：
//   - chk_invdoc_carrier_pair：(carrier_type IS NULL) = (carrier_id1_ciphertext IS NULL)
//   - chk_invdoc_exclusive：統編／載具／捐贈碼三選一（AMEGO 不擋，只能我方擋）
//   - chk_invdoc_na_reason：(status='NOT_APPLICABLE') = (not_applicable_reason IS NOT NULL)
//     ——雙向等式，傳錯組合當場 23514，不可能靜默寫進一列「NOT_APPLICABLE 但說不出原因」。
type NewDocument struct {
	ID                  int64
	IssuerID            int64
	IssuerNameSnapshot  string
	IssuerTaxIDSnapshot string
	SourceType          string
	SourceRef           string
	TriggerType         string
	RecipientType       string
	RecipientRefID      *int64
	RecipientName       string
	RecipientTaxID      string

	// 以下四欄為明文，由 repository 加密。
	RecipientEmail string
	RecipientPhone string
	CarrierID1     string
	CarrierID2     string

	// CarrierType 存的是 AMEGO **原生詞彙**（"3J0002"／"CQ0001"），不是 CodeMeta
	// InvoiceCarrierType 的 Value（§C.3）。用 domain.CarrierTypeToAmego 轉。
	CarrierType  string
	DonationCode string

	DocumentKind        string
	FulfillmentMode     string
	Status              string
	NotApplicableReason string

	SnapshotVersion        int32
	IssueAttemptSeq        int32
	ProviderAccountVersion *int64
	OriginalDocumentID     *int64

	SalesAmount        int64
	FreeTaxSalesAmount int64
	ZeroTaxSalesAmount int64
	TaxAmount          int64
	TotalAmount        int64
	TaxType            int16
	TaxRateBps         int32

	Provider          string
	ProviderRequestNo string

	// ReceiptNo 由 chk_invdoc_receipt 這條雙向等式綁定 document_kind='RECEIPT'
	// ⇒ 兩者必須同一句寫，號碼由 NextReceiptNo 取。
	ReceiptNo                   string
	ReceiptAuthorizationVersion string

	InvoiceDateTaipei *time.Time
	TaxPeriodCode     string

	IssueNotBefore time.Time
	// PaidAt 見 Document.PaidAt。nil ＝ 這份文件沒有對應的收款事件
	// （ADM 手動開票）或還沒有（尚未實作的路徑）。
	PaidAt *time.Time
	// StatutoryDeadlineAt 由 chk_invdoc_deadline_presence 綁定 document_kind，
	// 且**與 status 無關**：NOT_APPLICABLE 的 B2C 文件一樣必須有期限（domain.StatutoryDeadline）。
	StatutoryDeadlineAt *time.Time

	SourceSnapshot json.RawMessage
	Now            time.Time
}

// NewDocumentLine 是建立一列 invoice_document_lines 的輸入。
//
// 三個 CHECK 決定呼叫端必須先算好才送（本層不補算、不校正）：
//
//	chk_invline_amount          net_amount = unit_price × quantity
//	chk_invline_sign            DISCOUNT 行 net_amount < 0，其餘 >= 0
//	chk_invline_allowance_shape DISCOUNT 行三個 allowance 欄恆 0；其餘 base = net − allocated
//
// OrderItemID／SkuID 為 0 表示 NULL（SHIPPING／DISCOUNT 行）。
type NewDocumentLine struct {
	LineSeq           int32
	LineType          string
	OrderItemID       int64
	SkuID             int64
	Description       string
	Unit              string
	Quantity          int32
	UnitPrice         int64
	NetAmount         int64
	TaxCategory       int16
	TaxRateBps        int32
	Remark            string
	AllocatedDiscount int64
	AllowanceableBase int64
}

// MarkDocumentResolvedExternally 是「賣家自行開立並回報號碼」的落庫輸入
// （端點 62 賣家自報 ＝ 端點 56 ADM 代錄，同一段 domain 邏輯）。
//
// 🔴 三欄同進同出（chk_invdoc_external_no）：只寫號碼、另兩欄留空會在寫入當下 23514。
// ExternalInvoiceDate 是那張發票上印的**台灣曆日**（UTC+8 日界），ExternalReportedAt 是
// 平台收到回報的 UTC 時刻——兩者不是同一件事，不得互相推導。
type MarkDocumentResolvedExternally struct {
	ID                  int64
	ExternalInvoiceNo   string
	ExternalInvoiceDate time.Time
	ExternalReportedAt  time.Time
	Now                 time.Time
}

// MarkDocumentIssued 是 ISSUING → ISSUED 的輸入（§7.1.2 步驟 14）。
//
// PdfFetchNextAt／UploadRecheckNextAt 是兩組**候選集合的入口**，由呼叫端傳入而非寫死 now()：
// 載具發票沒有 PDF 可下載 ⇒ 傳 nil，那一列一開始就不進 INV.pdf 的候選集合。
// MarkReceiptIssued 是平台代製收據落 ISSUED 的全部輸入（#3026）。
//
// 欄位刻意比 MarkDocumentIssued 少很多：收據沒有發票號碼、沒有隨機碼、沒有稅期別，
// 也不進 AMEGO 的上傳輪詢與 PDF 抓檔候選集合（PDF 是我方即時產出、同一句寫進去的）。
type MarkReceiptIssued struct {
	ID int64
	// PdfObjectKey 是收據 PDF 落 media secure/ 之後的 objectKey。
	// §25：只存 objectKey，禁 URL／presigned／JWT。
	PdfObjectKey string
	// AuthorizationVersion 是 seller_tax_profiles.receipt_consent_version 的快照。
	// 來源會被「取消授權」清成 NULL ⇒ 不在開立當下快照就再也查不到他同意的是哪一版。
	AuthorizationVersion string
	// IssuedAt 落 provider_issued_at —— 收據沒有 provider，但那一欄是所有讀取路徑
	// （grpc/query_read.go 的 IssuedAt）唯一的「開立時點」來源。
	IssuedAt time.Time
	Now      time.Time
}

type MarkDocumentIssued struct {
	ID                  int64
	Provider            string
	ProviderInvoiceNo   string
	ProviderRandomCode  string
	ProviderIssuedAt    *time.Time
	InvoiceDateTaipei   *time.Time
	TaxPeriodCode       string
	PdfFetchNextAt      *time.Time
	UploadRecheckNextAt *time.Time
	Now                 time.Time
}

// RecipientSecrets 是 WithDecryptedRecipient 的 callback 參數：一列文件的全部明文收件人資料。
//
// 🔴 它**只在 callback 的生命週期內存在**。把它存進任何會被回傳、被 log、被序列化的
// struct，write-only 語意當場失效，而那不會有任何編譯錯或測試紅。
// carrier_value 是**憑證**不是資料（§10.3 判準：拿到它能做什麼）——單獨持有即可查該人全部
// 消費紀錄 ⇒ 與 donation_code 一樣不得落 log。
type RecipientSecrets struct {
	Email      string
	Phone      string
	CarrierID1 string
	CarrierID2 string
}

// DocumentRepository 讀 invoice_documents 與其明細，並承載 INV.pdf／INV.rc 第 4 組的租約。
type DocumentRepository interface {
	// ── 建立（§7.1.2 步驟 8）──
	// Create 走 ON CONFLICT DO NOTHING（**不指名 conflict target**：本表有四個 partial
	// unique，指名其一 ⇒ 另三個衝突時 23505 並讓整個交易 aborted 25P02）。
	// inserted=false ⇒ 這份文件已建過（重複消費 order.paid），回傳的是既有那一列。
	//
	// 🔴 M1 只有兩條建立邊（`[*]` → PENDING／`[*]` → NOT_APPLICABLE）。要建成
	// AWAITING_MANUAL 的路徑是「先建 PENDING、**同一交易內**接 MarkAwaitingManual」——
	// 那條邊在 M1 上存在，而 `[*]` → AWAITING_MANUAL 不存在。
	Create(ctx context.Context, in NewDocument) (doc Document, inserted bool, err error)
	// InsertLines 一次寫齊明細行（COPY）。呼叫端必須已算好每一行的三個 CHECK 所要求的值。
	InsertLines(ctx context.Context, documentID int64, lines []NewDocumentLine, now time.Time) error

	// LockForCorrection 是 R01 的第一步：同一交易內鎖住該列再判定。
	//
	// 🔴 兩個 partial unique 擋的是「同時存在兩筆進行中的修改／調整」，**擋不住**
	// 「讀到 v1、判定 expectedSnapshotVersion 相符、期間 R03 把它推成 v2、
	// 然後用過期的判定結果建了 correction」。而 expected_snapshot_version 一旦寫進
	// invoice_correction_requests 就是既成事實，且它是後續 R03 送去 order-service 的
	// 樂觀鎖基準 ⇒ 錯了會在跨服務那一層才炸，那時已經隔了一個 usecase 的距離。
	//
	// **必須在交易內呼叫**：交易外的 FOR UPDATE 拿到鎖之後立刻釋放，等於沒鎖。
	LockForCorrection(ctx context.Context, documentID int64) (Document, error)

	// DeleteLines 清空一份**尚未送出**文件的明細行，供 R03 的「重建 lines」使用。
	//
	// 🔴 述詞限定 PENDING／AWAITING_MANUAL（在 SQL 內，不是呼叫端的責任）：
	// 一份已 ISSUED 的發票被就地換掉明細而號碼不變，稅務上那是偽造。
	// 回傳刪除筆數；0 是合法的（本來就沒有明細）。
	DeleteLines(ctx context.Context, documentID int64) (int64, error)

	// ReviseSnapshot 是 §7.3.5 R03（開立前直接改）：把更正後的收件人資料與新的
	// snapshot_version 寫回**同一列**（就地更新的五條依據見該 query 的註解）。
	//
	// 🔴 snapshot_version 只能是 ORD.ReviseOrderInvoiceSnapshotInternal 的回傳值
	// （I-50）——本方法是複製，呼叫端不得自己 +1。
	// 🔴 明文進、密文出：加密與雜湊由 repository 用金鑰環做（與 NewDocument 一致）。
	//
	// 回 false ＝ CAS rows=0（版本被別人推進，或 status 已離開 PENDING／AWAITING_MANUAL）。
	// **不是錯誤，但絕不能當成成功**：那代表這份文件可能已經送出去了。
	ReviseSnapshot(ctx context.Context, in ReviseDocumentSnapshot) (bool, error)
	// NextReceiptNo 取平台代製收據編號（SEQUENCE，跳號可接受、不可重複）。
	// chk_invdoc_receipt 是雙向等式 ⇒ RECEIPT 文件必須在建列的同一句帶上它。
	NextReceiptNo(ctx context.Context) (string, error)

	// ── M1 狀態轉移（allowed-from 的 SSOT 是 domain/fsm.DocumentTransitions）──
	// 每一支回 false 都代表守衛述詞不成立（該列已被別人改走，或本來就不在允許的起點）。
	// 🔴 回 false 不得當成功繼續往下：那正是「rows=0 靜默失敗」被放行的形狀。
	MarkIssuing(ctx context.Context, id int64, now time.Time) (bool, error)
	MarkIssued(ctx context.Context, in MarkDocumentIssued) (bool, error)
	// MarkIssueFailed 是 ISSUING → ISSUE_FAILED，**同一句寫 reasonCode**。
	//
	// 🔴 reasonCode 不是選配：它是賣家在賣家中心看得到的唯一「為什麼」
	// （`InvoiceJobInfo.manual_reason_code`），而 `invoice.issue_failed.seller`
	// 的通知逐字叫他去那裡查看原因。落 status_history 而不落 document 的話，
	// 那一欄恆為空、畫面上只剩一個狀態 badge（2026-09-05 實查）。
	MarkIssueFailed(ctx context.Context, id int64, reasonCode string, now time.Time) (bool, error)
	MarkPendingRetry(ctx context.Context, id int64, now time.Time) (bool, error)
	MarkAwaitingManual(ctx context.Context, id int64, manualReasonCode string, now time.Time) (bool, error)
	// MarkReceiptIssued 是平台代製收據的 ISSUING → ISSUED（#3026）。
	// 它與 MarkIssued 互斥：DB 約束不允許 RECEIPT 帶 provider 欄位。
	MarkReceiptIssued(ctx context.Context, in MarkReceiptIssued) (bool, error)
	// ResumeFromManual 是 AWAITING_MANUAL → PENDING（ADM 給出判定，回自動線）。
	// expectedVersion 是 **document 自己的**版本，與 job 的版本是兩列各自遞增的值。
	ResumeFromManual(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error)
	// ── 作廢軸（M1 的 VOIDING 三條邊）────────────────────────────────────
	//
	// 三支各守自己的來源狀態，**不共用一個「改狀態」的通用方法**：作廢的三條邊
	// 各自帶不同的欄位等式（`voided_at` 與 VOIDED 由 chk_invdoc_voided_pair 綁成
	// 雙向），一個通用 setter 會讓那些等式落到呼叫端去記，而呼叫端記不住。

	// MarkVoiding 是 ISSUED → VOIDING，觸發者是 VOID operation 的 READY→SENDING。
	// 守衛「本 document 無生效 ALLOWANCE」在 usecase（要看 invoice_adjustments），
	// 本方法只守狀態。
	MarkVoiding(ctx context.Context, id int64, now time.Time) (bool, error)
	// MarkVoided 是 VOIDING → VOIDED，觸發者是 invoice_query 確認 invoice_type = C0501。
	// ⚠️ VOIDED **不是終態**：作廢後重開時它還會走 VOIDED → SUPERSEDED。
	MarkVoided(ctx context.Context, id int64, now time.Time) (bool, error)
	// RestoreToIssued 是 VOIDING → ISSUED，觸發者是 f0501 被 provider 拒絕（R05）。
	// **買家手上那張憑證仍然有效**，只是內容改不了。與 MarkIssued 分開是必須的：
	// 那一句會覆寫 provider_invoice_no 等回填欄。
	RestoreToIssued(ctx context.Context, id int64, now time.Time) (bool, error)
	// MarkSuperseded 是 VOIDED → SUPERSEDED，觸發者是重開的新 document 轉 ISSUED（R10）。
	MarkSuperseded(ctx context.Context, id, supersededBy int64, now time.Time) (bool, error)
	// CancelBeforeIssue 是 PENDING／AWAITING_MANUAL → CANCELLED_BEFORE_ISSUE。
	// 作廢重開的 R05 用它收掉那張已經建好、但因為作廢被拒而永遠不會開的重開票。
	CancelBeforeIssue(ctx context.Context, id int64, reasonCode, reason string, now time.Time) (bool, error)

	// MarkResolvedExternally 是 ISSUE_FAILED／AWAITING_MANUAL → RESOLVED_EXTERNALLY。
	// 三個 external_* 欄由 chk_invdoc_external_no 綁成雙向等式 ⇒ 同一句一起寫。
	MarkResolvedExternally(ctx context.Context, in MarkDocumentResolvedExternally) (bool, error)

	// WithDecryptedRecipient 是收件人明文的**唯一**出口（形狀照 WithDecryptedCredential）。
	// 明文只在 fn 的生命週期內存在，不得逸出 callback。
	WithDecryptedRecipient(ctx context.Context, id int64, fn func(RecipientSecrets) error) error

	GetByID(ctx context.Context, id int64) (Document, error)
	GetBySourceRef(ctx context.Context, sourceRef string) (Document, error)
	// GetByProviderRequestNo 是手動開票（端點 57）的業務冪等鍵查詢。
	// 🔴 查不到回 9301，而呼叫端必須把它當**正常結果**（可以建新的），不是錯誤。
	GetByProviderRequestNo(ctx context.Context, providerRequestNo string) (Document, error)
	// GetForBuyer 的授權述詞在 SQL 裡（recipient_ref_id 必須相符）：查無列與越權
	// 不可區分，兩者都回 9301（§8.1.4 Tier A）。
	GetForBuyer(ctx context.Context, sourceRef string, buyerID int64) (Document, error)
	ListLines(ctx context.Context, documentID int64) ([]DocumentLine, error)

	// AddAllowanceTotal 是 I4 的單句 CAS：折讓生效時累加 allowance_total_amount。
	//
	// 🔴 與 adjustment 的狀態轉移**同交易**。漏掉它，allowance_total_amount 會停在
	// 舊值 ⇒ 下一次折讓的上限判斷用的是錯的基數，而 chk_invdoc_allowance_cap
	// 要到累計真的超過發票金額時才會發現。
	AddAllowanceTotal(ctx context.Context, documentID, delta int64, now time.Time) (bool, error)
	// AddLineAllowancedAmount 逐行累加已折讓額（**含稅**，與 allowanceable_base 同語意）。
	//
	// 少加稅額的後果是下一次折讓的剩餘額被高估，於是累計超過發票金額、
	// AMEGO 4040173 才發現 —— 那時已經送出去了。
	AddLineAllowancedAmount(ctx context.Context, lineID, delta int64, now time.Time) (bool, error)
	FetchApproachingStatutoryDeadlines(ctx context.Context, threshold time.Time, limit int32) ([]Document, error)

	// ── 讀取視圖（每一支 List 都配一支述詞逐字相同的 Count）──
	//
	// 🔴 ListOrderDocumentsForBuyer 回 slice 不是單筆：一張訂單在 VOID_AND_REISSUE
	// 之後會有多份文件（superseded_by_id／original_document_id 串成鏈）。既有的
	// GetForBuyer 是 :one ⇒ 命中兩列時取任意一列而不報錯，症狀是買家看到作廢前的
	// 舊發票而畫面完全正常。它保留給「只需要一份就夠」的既有呼叫端。
	ListOrderDocumentsForBuyer(ctx context.Context, sourceRef string, buyerID int64) ([]Document, error)
	// GetForBuyerByID 是 document-scoped（不帶 order_no）：方向三的發票沒有訂單。
	// ⚠️ 查不到**不得**改用 order 述詞再查一次（§8.1.4(a) 兩套述詞不得互相回退）。
	GetForBuyerByID(ctx context.Context, id, buyerID int64) (Document, error)
	ListForBuyer(ctx context.Context, in BuyerDocumentQuery) ([]Document, error)
	CountForBuyer(ctx context.Context, in BuyerDocumentQuery) (int64, error)

	GetForSeller(ctx context.Context, id, storeID int64) (SellerDocumentItem, error)

	// ClaimNotificationResend 是端點 32／49／71 的重寄窗口（§6.1.10）。
	//
	// 🔴 窗口守衛在 SQL 的 WHERE（`notification_resent_at <= windowStart`）：
	// claimed=false ＝ **還在 10 分鐘窗口內** ⇒ 呼叫端回 9352，絕不當成功往下走。
	// 回的 seq 是**遞增後**的次數，同時是 notification_ref_id 的去重種子——沿用開立
	// 當下那個 ref id 的第二封信會被 notification-service 依 reference_id 靜默吞掉。
	//
	// ⚠️ 本支**不做擁有者判定**：三個入口的歸屬鍵不同（store／buyer／無），
	// 呼叫端必須先用 GetForSeller／GetForBuyerByID／GetByID 取得那一列才呼叫它。
	ClaimNotificationResend(ctx context.Context, id int64, windowStart, now time.Time) (seq int32, claimed bool, err error)
	ListForSeller(ctx context.Context, in SellerDocumentQuery) ([]SellerDocumentItem, error)
	CountForSeller(ctx context.Context, in SellerDocumentQuery) (int64, error)
	// ListRecentForIssuer 給端點 16 的固定筆數區塊，刻意不與 ListForSeller 共用：
	// 那一支帶 9 個篩選參數，總覽為了呼叫它得傳一整組「不篩選」的空值，
	// 而任何一個傳錯都會靜默少列。
	ListRecentForIssuer(ctx context.Context, issuerID int64, limit int32) ([]Document, error)

	ListReceivedForSeller(ctx context.Context, in ReceivedDocumentQuery) ([]ReceivedDocumentItem, error)
	CountReceivedForSeller(ctx context.Context, in ReceivedDocumentQuery) (int64, error)

	ListForAdmin(ctx context.Context, in AdminDocumentQuery) ([]AdminDocumentItem, error)
	CountForAdmin(ctx context.Context, in AdminDocumentQuery) (int64, error)

	// INV.pdf：候選 → 搶佔 → 三種落點。
	FetchDuePdfFetches(ctx context.Context, now time.Time, limit int32) ([]Document, error)
	ClaimPdfFetch(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	MarkPdfFetchDone(ctx context.Context, id int64, objectKey string, now time.Time) error
	MarkPdfFetchStopped(ctx context.Context, id int64, now time.Time) error
	UpdatePdfFetchAttempt(ctx context.Context, id int64, nextAttemptAt, now time.Time) error

	// INV.rc 第 4 組（上傳輪詢）：與第 2 組落在不同的表，所以兩組租約長度不同是安全的。
	FetchDueUploadRechecks(ctx context.Context, now time.Time, limit int32) ([]Document, error)
	ClaimUploadRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	RecordUploadRecheckResult(ctx context.Context, id int64, providerUploadStatus *int16, now time.Time, nextAt *time.Time) error
}

// MarkRecheckResult 是 INV.rc 對帳有結論／仍不確定兩條路的共用輸入。
type MarkRecheckResult struct {
	ID              int64
	NextStatus      string
	NextAttemptAt   time.Time
	ProviderCode    *string
	ProviderMessage *string
	CompletedAt     *time.Time
	LastError       *string
	// CancelEvidenceKind／CancelConfirmedAt 只有 J24 會帶（CodeMeta
	// InvoiceCancelEvidenceKind 的 PROVIDER_QUERY_71）；J23／J25 一律 nil。
	//
	// 🔴 兩欄在 SQL 那一側是 COALESCE(narg, 原值) 而**不是**無條件覆寫：
	// J23／J25 走同一支 query，無條件覆寫會把端點 54 已經寫好的 A4 取消證據抹掉
	// （J27 會把 CANCEL_BLOCKED_UNKNOWN 推回 PROVIDER_STATUS_RECHECK，那之後
	// J23／J25 都可能發生）。
	CancelEvidenceKind *string
	CancelConfirmedAt  *time.Time
	Now                time.Time
}

// UpdateRecheckAttempt 是「仍不確定且留在重查迴圈」的落點（J31 / PROVIDER_STATUS_RECHECK）。
type UpdateRecheckAttempt struct {
	ID              int64
	NextStatus      string
	NextAttemptAt   time.Time
	RecheckListPage int16
	ProviderCode    *string
	ProviderMessage *string
	Now             time.Time
}

// OperationRepository 是 invoice_operations 的存取面，同時服務 INV.op 與 INV.rc。
//
// 兩支的候選述詞落在同一欄（next_attempt_at）而狀態集合不相交——那是正確性條件不是風格
// 問題（§4.3.3）：交集非空 ⇒ 60s 租約會在 120s 租約還在打 f0401 的途中合法搶走它 ⇒ 重複開票。
type OperationRepository interface {
	// ── 建立與同事務判定（J01–J04）──
	// Create 走 ON CONFLICT DO NOTHING **不指名 target**：本表有兩個唯一索引
	// （uq_invop_operation_key 與 partial 的 uq_invop_open_adjustment_per_document）。
	// inserted=false 有**兩種**意義，呼叫端必須分辨：同一把 key 重送（冪等，讀既有列即可）
	// vs 序列閘門已被別的調整佔用。分辨方式是先讀 operation_key，不是猜。
	Create(ctx context.Context, in NewOperation) (op Operation, inserted bool, err error)
	GetByID(ctx context.Context, id int64) (Operation, error)
	GetByKey(ctx context.Context, operationKey string) (Operation, error)

	// ListByDocument 回一份文件底下的全部 operation（created_at DESC）。
	//
	// 🔴 operation 的擁有者判定一律經 invoice_documents（§8.1.4 末段）：
	// invoice_operations 完全沒有訂單／store 欄，不得自行反正規化一個 order_no。
	ListByDocument(ctx context.Context, documentID int64) ([]Operation, error)

	// ── 讀取視圖（三表 join：operation → document → issuer）──
	//
	// 🔴 每一支都經 invoice_documents 解析擁有者與訂單號，理由同 ListByDocument
	// 的紅字。GetViewForSeller 的 store_id 是述詞的一部分 ⇒ 越權與不存在同回 9340。
	GetViewForSeller(ctx context.Context, id, storeID int64) (OperationView, error)
	GetViewForAdmin(ctx context.Context, id int64) (OperationView, error)
	// ListOrderExternalIssuances 是子路由 62R。allowedStatuses 由呼叫端以**常數**
	// 傳入（MANUAL_PROCESS_ALLOWED／RESOLVED_EXTERNALLY）：述詞是這條路由的安全
	// 邊界不是效能優化，放寬就退化成 S08 的等價物，而 S08 對停權賣家是 BLOCK。
	// 🔴 呼叫端不得把 allowedStatuses 接成使用者可控輸入。
	ListOrderExternalIssuances(ctx context.Context, storeID int64, orderNo string, allowedStatuses []string, page Page) ([]OperationView, error)
	CountOrderExternalIssuances(ctx context.Context, storeID int64, orderNo string, allowedStatuses []string) (int64, error)
	ListViewsForAdmin(ctx context.Context, in AdminOperationQuery) ([]OperationView, error)
	CountViewsForAdmin(ctx context.Context, in AdminOperationQuery) (int64, error)
	// ListPriorityViewsForAdmin 排序鍵是 statutory_deadline_at（NULLS LAST）
	// **不是 created_at**：作廢重開的 job 建得晚、期限卻繼承原發票。
	ListPriorityViewsForAdmin(ctx context.Context, in PriorityOperationQuery) ([]OperationView, error)

	// Promote 是 J02／J03／J04（CREATED → READY／AWAITING_TRIGGER／NOT_APPLICABLE），
	// **建列的同一事務內**判定。三條邊共用一句，差別只在目標值與 next_attempt_at。
	// ⚠️ J04 的「寫 not_applicable_reason」落在 invoice_documents，本表沒有那個欄位。
	Promote(ctx context.Context, id int64, nextStatus string, nextAttemptAt, now time.Time) (bool, error)
	// CancelBeforeIssue 是 J06 → CANCELLED_BEFORE_ISSUE。allowed-from 自 2026-09-01
	// 起含 CREATED（#2909）：R04 建的 REISSUE operation 停在 CREATED 等作廢確認，
	// 作廢被拒時要收掉它。
	CancelBeforeIssue(ctx context.Context, id int64, reasonCode, reason string, now time.Time) (bool, error)

	// ── SENDING 之後的三個非重試落點 ──
	// MarkWaitingConfirmation 是 J13（逾時／連線中斷／JSON 解析失敗／分類 = RECONCILE）。
	// unknown_since 走 COALESCE：它是**事實時間戳**，也是重查退避階梯的分桶依據，
	// 崩潰重來也不會變 —— 這正是這條路不用計數器的理由。
	MarkWaitingConfirmation(ctx context.Context, in MarkOperationUnknown) (bool, error)
	// MarkRetryExhausted 是 J14（RETRY_SCHEDULED → FAILED_RETRY_EXHAUSTED）。
	// 🔴 **不可沿用 MarkFailed**：那一句固定寫 FAILED_PERMANENT，而把 FAILED_* 拆成三個的
	// 唯一理由就是「ADM 按下重試安不安全」必須由狀態本身回答。寫錯值 ⇒ 一筆本來按一下就好
	// 的失敗，被要求「先修正資料」而其實沒有資料可修。
	MarkRetryExhausted(ctx context.Context, id int64, lastError string, now time.Time) (bool, error)
	// MarkSellerActionRequired 是 J32（PERMANENT 中「賣家法定前置未完成」子類，3040111 字軌不足）。
	// 與 J11 分開是刻意的：兩者錯誤分類同為 PERMANENT，但一個要通知賣家去財政部配字軌、
	// 一個要通知 ADM 改資料——行動指引完全相反。
	MarkSellerActionRequired(ctx context.Context, in MarkOperationProviderResult) (bool, error)

	// ConfirmCancel 是 **J28**（CANCEL_BLOCKED_UNKNOWN → CANCELLED_BY_ADMIN，
	// ADM 雙人確認 ＋ 證據）。
	//
	// 🔴 它在 2026-09-04 之前**零呼叫端**（連 adapter 都不存在）：SQL 與 sqlc 生成物
	// 一直都在，但 cancel_evidence_kind 是必填的 arg 而 proto 沒有那個欄位
	// ⇒ 那條邊沒有任何輸入通道。後果是 CANCEL_BLOCKED_UNKNOWN 與
	// PROVIDER_STATUS_RECHECK 之間無限循環（J26 ↔ J27），而設計書 §7.5.6 逐字寫著
	// 它「**永不自動終結**，唯一出邊是 ADM 雙人確認（J28）」——連帶端點 54/55/62
	// 對這一類案件整條不可用。
	//
	// 回 false ＝ CAS rows=0（狀態已不在 CANCEL_BLOCKED_UNKNOWN，或這個確認已經做過）。
	// 述詞含 `cancel_confirmed_at IS NULL` ⇒ 重送同一個確認是冪等的，不會把時間戳往後推。
	ConfirmCancel(ctx context.Context, in ConfirmOperationCancel) (bool, error)

	// INV.op：pkg/outbox.Repository[T] 的五方法契約。
	FetchDue(ctx context.Context, now time.Time, limit int32) ([]Operation, error)
	Claim(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	MarkDone(ctx context.Context, id int64, now time.Time) (bool, error)
	MarkFailed(ctx context.Context, id int64, lastError string, now time.Time) (bool, error)

	// MarkFailedWithProviderResult 與 MarkFailed 同一條邊，但**同時保存 provider 的
	// 原話**（#3082）。給「這一次就是 provider 明確拒絕」的呼叫者用；
	// MarkFailed 留給重試預算用完那一類（它手上沒有 provider 回應，
	// 硬塞兩個空值會把先前真的有 code 的記錄洗掉）。
	MarkFailedWithProviderResult(ctx context.Context, in MarkOperationProviderResult) (bool, error)
	UpdateAttempt(ctx context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, lastError *string, now time.Time) error

	// INV.rc：自寫 claim 迴圈。⚠️ 不得改接 outbox.Worker[T]——它的 MarkDone 會覆蓋業務狀態欄
	// （§4.3.3 裁決理由 2），而 UpdateInvoiceOperationAttempt 會把列寫成 RETRY_SCHEDULED
	// ⇒ 下一輪被 INV.op 撈走真的重送一次 f0401 ⇒ 重複開票。
	FetchPendingRechecks(ctx context.Context, now time.Time, limit int32) ([]Operation, error)
	ClaimRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error)
	MarkRecheckResolved(ctx context.Context, in MarkRecheckResult) (bool, error)
	MarkRecheckUnresolved(ctx context.Context, in MarkRecheckResult) (bool, error)
	UpdateRecheckAttempt(ctx context.Context, in UpdateRecheckAttempt) (bool, error)

	// Hold 是 J08（READY／RETRY_SCHEDULED → HELD），也是 §7.3.5 **R03 的守衛本身**。
	//
	// 🔴 回 false ＝ rows=0 ⇒ **R03 這條邊不成立**，correction 必須改走 R04 作廢重開。
	// 那不是錯誤、也不能重試：述詞刻意不含 SENDING，意思就是「已經在打 AMEGO 的那一筆
	// 攔不下來」。把 rows=0 當成暫時性失敗去重試，會在對方已經開出發票之後才改快照。
	//
	// heldUntil 由呼叫端給（now + 60s）：chk_invop_held_pair 是雙向等式，兩欄同一句寫。
	// HELD 不在兩支 worker 的候選集合裡，靠 ReleaseExpiredHolds 掃回。
	Hold(ctx context.Context, id int64, now, heldUntil time.Time) (bool, error)

	// ReleaseHold 是 R03 的收尾：HELD → READY，**同一事務內**，不等租約到期。
	//
	// 🔴 少了它不會報錯、不會有測試紅，只是那張發票有 60 秒開不出去——
	// HELD 不在兩支 worker 的候選集合裡，要等 ReleaseExpiredHolds 掃回才動得了。
	ReleaseHold(ctx context.Context, id int64, now time.Time) (bool, error)

	// 三支集合式前置 UPDATE（RunOnce 的前置步驟，不需要 claim——推出候選集合的寫入
	// 就是狀態轉移本身，第二個副本 rows=0。《多副本排程搶佔規範》例外 A）。
	// 少了 PromoteDueTriggers，J05（AWAITING_TRIGGER → READY）沒有任何元件會執行。
	//
	// 🔴 PromoteSellerActionRequired（J33）是 2026-09-04 補的：在那之前它零執行者，
	// 而 CodeMeta 對 SELLER_ACTION_REQUIRED 的 description 逐字寫著「由 INV.trk 的
	// RunOnce 前置步驟 promote 回 READY」——那個步驟不存在（#3089 A 組第 3 項）。
	// 它由 INV.trk 呼叫（不是 INV.op），因為觸發條件是**字軌餘量恢復**這個外部事實，
	// 而只有 INV.trk 會去量它。issuer_id 必帶：字軌是 per-issuer 的資源。
	ReleaseExpiredHolds(ctx context.Context, now time.Time) (int64, error)
	PromoteDueTriggers(ctx context.Context, now time.Time) (int64, error)
	PromoteSellerActionRequired(ctx context.Context, issuerID int64, now time.Time) (int64, error)

	// CountCrossPeriodPending 是 M20 的資料源（§8.2.3）：發票日期屬已結束期別、
	// 且仍有非終態 VOID／ALLOWANCE／VOID_ALLOWANCE operation 的筆數。唯讀。
	CountCrossPeriodPending(ctx context.Context, now time.Time) (int64, error)

	// ── ADM 寫入面（§6.1.14 端點 52／53／56）──
	//
	// 🔴 四支回的 bool 一律是 **CAS 的 rows>0**，不是「有沒有出錯」：allowed-from 與
	// expectedVersion 都寫在 SQL 的 WHERE ⇒ false ＝ 守衛不成立，呼叫端必須回明確業務碼。
	// 把 false 當成功往下走，就是 §7.3.0 規則 4 說的靜默失敗。

	// AdminRetryAfterFix 是 J18（FAILED_PERMANENT → READY）。attempt_count **不歸零**。
	AdminRetryAfterFix(ctx context.Context, id, expectedVersion int64, requestSnapshotHash string, now time.Time) (bool, error)
	// AdminResetAttempts 是 J19（FAILED_RETRY_EXHAUSTED → READY）＋ J34（SELLER_ACTION_REQUIRED
	// → READY）。attempt_count 歸零：那兩條邊的語意是「重試本來就是安全的」。
	AdminResetAttempts(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error)
	// RequestCancel 是 J21（五個來源狀態 → CANCEL_REQUESTED）。
	RequestCancel(ctx context.Context, in RequestOperationCancel) (bool, error)
	// MarkResolvedExternally 是 J30（MANUAL_PROCESS_ALLOWED → RESOLVED_EXTERNALLY，終態）。
	// 端點 56（ADM 代錄）與端點 62（賣家自報）**共用這一支**，不得各寫一份 CAS。
	MarkResolvedExternally(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error)

	// AllowManualProcess 是 J29（CANCELLED_BY_ADMIN → MANUAL_PROCESS_ALLOWED，端點 55
	// 第二位管理員覆核解鎖）。
	//
	// 🔴 它自帶**第二道**雙人守衛：SQL 的述詞是 `cancel_requested_by <> $actor`，守的是
	// 「覆核的人不得是當初提出取消的人」；invoice_manual_unlocks 那一句 CAS 守的是
	// 「覆核的人不得是發起解鎖的人」。兩個發起人未必是同一位 ADM ⇒ 兩道都要，
	// 而且兩道都在 SQL 裡。
	AllowManualProcess(ctx context.Context, id, expectedVersion, actorAccountID int64, now time.Time) (bool, error)

	// ── M4 P14／P16 的集合式副作用（端點 45 停用／67 撤銷）──
	//
	// FailOpenForIssuer 把該 issuer **全部未終態** operation 轉 FAILED_PERMANENT。
	// 它是集合式、無外部副作用的單句 UPDATE ⇒ 依《多副本排程搶佔規範》例外 A 不需要 claim。
	// 回傳的是實際被改到的列數。
	//
	// CountOpenForIssuer 的述詞與它**逐字相同**，供端點 45／67 的 pendingOrderCount 使用。
	// 🔴 兩者必須在**同一交易**內呼叫（§7.4 C-8）：分兩個交易讀，ADM 看到的數字與實際被
	// 打掉的數量不一致，而 dialog 開啟到按下之間又付款了一筆正是它要防的那個窗口。
	FailOpenForIssuer(ctx context.Context, in FailOpenOperationsForIssuer) (int64, error)
	CountOpenForIssuer(ctx context.Context, issuerID int64) (int64, error)
}

// RequestOperationCancel 是 J21 的輸入（端點 53）。
//
// ReasonCode 的值域是 CodeMeta InvoiceCancelReason 的 4 值；ADM 從 A06 手動取消時
// 一律是 ADM_MANUAL，細分類寫進必填的 ReasonDetail。收一套寫另一套的後果是 A05 的
// 「依取消原因分類統計」查出空集合，而空集合看起來就像今天沒有這類案件。
type RequestOperationCancel struct {
	ID              int64
	ExpectedVersion int64
	ActorAccountID  int64
	ReasonCode      string
	ReasonDetail    string
	Now             time.Time
}

// NewOperation 是建立一列 invoice_operations 的輸入（J01）。
//
// OperationKey 由純函式 domain.OperationKeyFor 推導，**全域唯一、重試不換鍵**。
// StatutoryDeadlineAt 是 invoice_documents 同名欄的反正規化副本（I-52），必須在建列的
// 同一交易內複製；它只進 FetchDueInvoiceOperations 的 ORDER BY，**不得進 WHERE**——
// 進 WHERE 會讓逾期的 row 掉出候選集合並永久停止重試。
//
// chk_invop_adjustment_pair 是雙向等式：VOID／ALLOWANCE／VOID_ALLOWANCE 必須帶
// AdjustmentID，ISSUE／QUERY／REISSUE 必須不帶（0）。傳錯當場 23514。
type NewOperation struct {
	ID                  int64
	DocumentID          int64
	AdjustmentID        int64 // 0 表示 NULL
	OperationType       string
	OperationKey        string
	ProviderRequestNo   string
	RequestSnapshotHash string
	MaxAttempts         int32
	NextAttemptAt       time.Time
	StatutoryDeadlineAt *time.Time
	Now                 time.Time
}

// MarkOperationUnknown 是 J13 的輸入。
type MarkOperationUnknown struct {
	ID              int64
	NextAttemptAt   time.Time
	ProviderCode    string
	ProviderMessage string
	LastError       string
	Now             time.Time
}

// ConfirmOperationCancel 是 J28 的輸入（#3089 A 組第 2 項）。
//
// 🔴 ExpectedVersion 必須是**該 operation 當下的 version**。端點 55 在同一交易內
// 先做 J28 再做 J29，而 J28 的 SQL 有 `version = version + 1` ⇒ J29 要用
// **ExpectedVersion + 1**。兩次都傳同一個值的話 J29 會 rows=0 而**靜默**停在
// CANCELLED_BY_ADMIN。
type ConfirmOperationCancel struct {
	ID                int64
	ExpectedVersion   int64
	ConfirmedBy       int64
	EvidenceKind      string
	EvidenceObjectKey *string
	Now               time.Time
}

// MarkOperationProviderResult 是 J32 的輸入（同形狀日後可供其他 provider 結果落點共用）。
type MarkOperationProviderResult struct {
	ID              int64
	ProviderCode    string
	ProviderMessage string
	LastError       string
	Now             time.Time
}

// NewOutboxEvent 是出向事件的落庫輸入。它必須與業務寫入同一個交易（例外 A：同 tx outbox）。
type NewOutboxEvent struct {
	AggregateType string
	AggregateID   int64
	EventType     string
	Exchange      string
	RoutingKey    string
	Payload       json.RawMessage
	NextAttemptAt time.Time
	Now           time.Time
}

// OutboxRepository 是 invoice_outbox_events 的存取面（INV.ob）。
type OutboxRepository interface {
	Insert(ctx context.Context, in NewOutboxEvent) (OutboxEvent, error)
	FetchPending(ctx context.Context, now time.Time, limit int32) ([]OutboxEvent, error)
	Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (bool, error)
	MarkDone(ctx context.Context, id int64, now time.Time) error

	// Park 是「重試次數用盡」的歸宿：**停泊而非終結**。
	//
	// 🔴 這裡刻意**沒有** MarkFailed。曾經有，而它 `SET status='failed'` ——
	// 本表的候選述詞（FetchPending／Claim）與 idx_invoutbox_pending 全部是
	// status='pending'，而且沒有任何時間欄能讓 'failed' 回到候選集合 ⇒ 那是一個
	// 標準的**永久旗標**（《多副本排程搶佔規範》規則⑧禁止的形狀）。
	// 更糟的是 CountPending／OldestPendingCreatedAt 的述詞也是 'pending' ⇒
	// 一列落到 'failed' 的同時就退出 outbox_oldest_pending_age_seconds，
	// 告警 InvoiceOutboxLag 會在事件**真正卡死那一刻**由 firing 轉 resolved。
	//
	// Park 保留 status='pending'（所以積壓指標看得到它、告警持續亮著），只把
	// next_attempt_at 推到 parkUntil（所以不會變成每 tick 重撈的熱迴圈）。
	// 回傳 false 代表那一列已不在 'pending'（例如被維運 SQL 隔離成 'failed'）。
	Park(ctx context.Context, id int64, parkUntil time.Time, lastError string) (bool, error)

	UpdateAttempt(ctx context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, lastError *string) error
	CountPending(ctx context.Context) (int64, error)
	OldestPendingCreatedAt(ctx context.Context) (*time.Time, error)
	DeleteDoneBefore(ctx context.Context, cutoff time.Time, limit int32) (int64, error)
}

// NewInboxEvent 是入向事件的落庫輸入（兩道去重：UNIQUE(event_id) ＋ 業務鍵）。
type NewInboxEvent struct {
	EventID          string
	EventType        string
	RoutingKey       string
	Exchange         string
	BusinessDedupKey string
	AggregateRef     *string
	Payload          json.RawMessage
	Status           string
	DeferredUntil    *time.Time
	NextAttemptAt    time.Time
	OccurredAt       time.Time
	Now              time.Time
}

// InboxRepository 是 invoice_inbox_events 的存取面。這張表沒有**自己的**排程器（§4.1.3）：
// DEFERRED 的列由開票成功後的反向認領推進（§6.3.6，主路徑），到期重試（兜底）掛在
// INV.dl 那一輪順帶跑（internal/worker/deadline_worker.go）。
type InboxRepository interface {
	// Insert 走 ON CONFLICT DO NOTHING（不指名 target：本表有兩個唯一鍵）。
	// inserted=false 代表這則事件已收過，是正常的去重結果不是錯誤（此時 id 為 0）。
	//
	// 🔴 id 是必要的回傳值：所有 Mark* 都以它為鍵。少了它，這一列永遠停在 RECEIVED
	// ＋ next_attempt_at=now，也就是永久留在 idx_invinbox_due 的兜底候選集合裡，
	// INV.dl 每一輪都會白撈它一次（Prepare 冪等所以不出錯，只是白燒）。
	Insert(ctx context.Context, in NewInboxEvent) (id int64, inserted bool, err error)
	FetchDue(ctx context.Context, now time.Time, limit int32) ([]InboxEvent, error)

	// FetchDueByType 是 FetchDue 限定單一 event_type 的版本，INV.dl 的兜底掃描用它。
	//
	// 🔴 型別過濾必須在 SQL 而不是在 Go 端 skip：被跳過的列每一輪都會重新佔掉 batch
	// 名額而永遠不推進 ⇒ 一種事件的積壓會靜默吃光另一種的處理量。
	FetchDueByType(ctx context.Context, eventType string, now time.Time, limit int32) ([]InboxEvent, error)

	// FetchDueActionableByType 同上但**排除 DEFERRED**：DEFERRED 的歸宿是反向認領
	// （order.paid 到達時 reclaimDeferred 撿走），兜底掃描碰它就會在失敗時翻成 FAILED
	// ⇒ 反向認領的述詞永久不成立，而且完全無聲。退款路徑一律用這一支。
	FetchDueActionableByType(ctx context.Context, eventType string, now time.Time, limit int32) ([]InboxEvent, error)

	Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (bool, error)

	// UpdateAttempt 記一次「這輪失敗、待會再試」：**不動 status**，只寫次數、退避與錯誤。
	//
	// 與 MarkFailed 的分工是 pkg/outbox.Worker 的契約：未達 MaxAttempts 走本方法（列留在
	// 原狀態、帶退避回到候選集合），達到才走 MarkFailed（status='FAILED'）。
	// attemptCount 是**絕對值**（Worker 算好的 newAttempt），不是增量。
	UpdateAttempt(ctx context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, lastError *string) (bool, error)

	MarkProcessed(ctx context.Context, id int64, now time.Time) (bool, error)
	MarkSkipped(ctx context.Context, id int64, disposition, reason string, now time.Time) (bool, error)
	MarkDeferred(ctx context.Context, id int64, deferredUntil time.Time) (bool, error)
	MarkFailed(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time) (bool, error)
	ListDeferredByAggregateRef(ctx context.Context, aggregateRef string) ([]InboxEvent, error)
	CountStaleDeferred(ctx context.Context, cutoff time.Time) (int64, error)
	PurgeProcessedPayloads(ctx context.Context, now, cutoff time.Time, limit int32) (int64, error)
}

// CancelledOrder 是一則「這張訂單已取消」的本地事實（invoice_cancelled_orders 一列）。
//
// 它刻意**不帶結論欄**（沒有「不會開發票」這種欄位）：那個結論由讀路徑依「有沒有
// invoice_documents 列」現算——先付款開票、事後才取消的訂單有 document，它的 state
// 由 document status 決定而不看本表。把結論寫進表 ⇒ 兩處各自為政，而它們只在
// 退款／作廢那條路徑上才會分歧，正是最難測到的那一條。
type CancelledOrder struct {
	OrderNo string
	// BuyerID ＝ order.cancelled payload 的 buyer_id（＝ orders.buyer_id，營運鍵
	// 不是 account_id）。它是讀路徑的**授權述詞**，見 CancelledOrderRepository。
	BuyerID int64
	// CancelReason ＝ OrderCancelReason.value()。買家自願取消未付款訂單時 producer
	// 不寫這一欄 ⇒ 可為空字串。**不對買家顯示**（內部代碼，含 SELLER_SUSPENDED）。
	CancelReason string
	OccurredAt   time.Time
	Now          time.Time
}

// CancelledOrderRepository 是 invoice_cancelled_orders 的存取面（#2904）。
//
// 為什麼需要它：端點 8 在「這張訂單沒有任何 invoice_documents 列」時本來一律回
// state=AWAITING_PAYMENT ＋「付款完成後會立即開立發票」，而那句話對已取消的訂單
// 永遠不會成真。invoice-service 的讀路徑看不到訂單狀態 ⇒ 需要一個由 order.cancelled
// 落地的本地事實。
type CancelledOrderRepository interface {
	// Insert 走 ON CONFLICT DO NOTHING（不指名 target，理由見 SQL 註解）。
	// inserted=false ＝ 這張訂單已經記過了，是正常的去重結果不是錯誤。
	Insert(ctx context.Context, in CancelledOrder) (inserted bool, err error)

	// IsCancelledForBuyer 是端點 8 的讀路徑。
	//
	// 🔴 **buyerID 是授權述詞不是過濾條件**（§8.1.4 Tier A）。少了它，任何登入買家
	// 拿別人的 order_no 來問都會得到 true，而訂單號是可枚舉的 ⇒ 訂單存在性 oracle。
	// 介面簽章把它列為必填參數就是為了讓「忘了傳」變成編譯錯而不是靜默的授權破口。
	IsCancelledForBuyer(ctx context.Context, orderNo string, buyerID int64) (bool, error)
}

// NewAdjustment 是 G01／G02／G03 三條建立邊的共用輸入。
//
// 🔴 chk_invadj_allowance_shape 是**雙向**形狀約束：ALLOWANCE 必須帶 AllowanceNumber／
// AllowanceDateTaipei ＋ 三個金額欄；**VOID 必須全部為 NULL**。傳錯組合當場 23514。
// R04 建的是 VOID ⇒ 那六個欄位一個都不能給。
type NewAdjustment struct {
	ID         int64
	DocumentID int64
	// AdjustmentType：VOID／ALLOWANCE／VOID_ALLOWANCE。
	AdjustmentType string
	// Status：PENDING（可調整性判定通過，同一事務內建 operation）／
	// BLOCKED_UNKNOWN_DOCUMENT／BLOCKED_PENDING_CORRECTION。
	// ⚠️ 被閘門擋下的調整**不是拒絕建立**而是建成 BLOCKED_*：「先不建列、之後補建」
	// 那條路徑沒有唯一鍵保護，會在重試時建出第二筆折讓。
	Status      string
	TriggerKind string

	RefundNo            *string
	CorrectionRequestID *int64

	// 以下六欄 ALLOWANCE 專用，VOID 一律 nil（見上方 chk 說明）。
	AllowanceNumber     *string
	AllowanceDateTaipei *time.Time
	NetAmountExTax      *int64
	TaxAmount           *int64
	GrossAmount         *int64

	VoidReasonCode   *string
	VoidReasonDetail *string

	Now time.Time
}

// NewAdjustmentLine 是一列待寫入的折讓明細（§5A.9.3 的輸出）。
//
// 金額語意與 invoice_document_lines **相反**：這裡的 UnitPriceExTax／NetAmountExTax
// 是**未稅**，因為 AMEGO 折讓的 UnitPrice／Amount 固定未稅，而開立端 DetailVat=1
// 是含稅 ⇒ 兩套 DTO 不可共用金額欄位（58 §5A.9.3 逐字）。
type NewAdjustmentLine struct {
	LineSeq             int32
	SourceLineID        *int64
	OriginalInvoiceNo   string
	OriginalInvoiceDate time.Time
	OriginalDescription string
	Quantity            int32
	UnitPriceExTax      int64
	NetAmountExTax      int64
	TaxAmount           int64
	TaxCategory         int16
}

// MarkAdjustmentCompleted 是 G09／G13／G17 的入參。
//
// ProviderUploadStatus／ProviderUploadCheckedAt 是查詢路徑（G13／G17）才有的欄位；
// 送件成功（G09）時兩者為 nil —— 那時還沒查過上傳狀態。
type MarkAdjustmentCompleted struct {
	ID                      int64
	Now                     time.Time
	ProviderUploadStatus    *int16
	ProviderUploadCheckedAt *time.Time
}

// AdjustmentRepository 讀 invoice_adjustments 與折讓明細。
type AdjustmentRepository interface {
	// Insert 建一筆調整（G01／G02／G03 共用）。
	//
	// 🔴 SQL 的 ON CONFLICT DO NOTHING **不指名 target**：本表有三個 partial unique，
	// 指名其一會讓另兩個衝突時 23505 並使整個交易 aborted（25P02）。
	//
	// 回 (adjustment, created)：created=false ⇒ 這筆調整**已經建過**（唯一鍵擋下），
	// 回的是既有列。呼叫端據此決定要不要建 operation——重複建會送出第二次作廢。
	Insert(ctx context.Context, in NewAdjustment) (Adjustment, bool, error)
	GetByID(ctx context.Context, id int64) (Adjustment, error)
	ListByDocument(ctx context.Context, documentID int64) ([]Adjustment, error)
	ListStaleBlocked(ctx context.Context, staleBefore time.Time, limit int32) ([]Adjustment, error)
	ListLines(ctx context.Context, adjustmentID int64) ([]AdjustmentLine, error)

	// ── M7 的執行軸（§7.3.7 G08–G15）───────────────────────────────────
	//
	// 🔴 每一支都是 CAS（allowed-from 寫在 SQL 的 WHERE 裡），回 false 代表
	// 「這一列已經不在我以為的狀態」——呼叫端必須把它當衝突處理，不可當成功。
	// SQL 早就寫好了（db/queries/invoice_adjustments.sql，19 支），缺的一直是
	// 這一層 adapter 與上面的執行者。

	// InsertLines 建折讓明細（G01 的一部分，與 adjustment 同交易）。
	InsertLines(ctx context.Context, adjustmentID int64, lines []NewAdjustmentLine, now time.Time) error
	// NextAllowanceNumber 取我方自產的折讓單號（§5.3.3）。
	//
	// 🔴 **送出前先落庫**：AMEGO 不回傳單號，我方送什麼就是什麼。先取號再送，
	// 逾時重送時單號不換（§7.2.2 細節 5），否則同一筆退款會產生兩張折讓單。
	NextAllowanceNumber(ctx context.Context) (string, error)

	// MarkSending 是 G08（PENDING → SENDING）。
	MarkSending(ctx context.Context, id int64, now time.Time) (bool, error)
	// MarkCompleted 是 G09／G13／G17（→ COMPLETED）。
	MarkCompleted(ctx context.Context, in MarkAdjustmentCompleted) (bool, error)
	// MarkFailedPermanent 是 G10（SENDING → FAILED_PERMANENT ＋ blocked_reason_code）。
	MarkFailedPermanent(ctx context.Context, id int64, reasonCode string, now time.Time) (bool, error)
	// Reschedule 是 G11／G14（→ PENDING，可安全補送）。
	Reschedule(ctx context.Context, id int64, now time.Time) (bool, error)
	// MarkWaitingConfirmation 是 G12（SENDING → WAITING_CONFIRMATION）。
	MarkWaitingConfirmation(ctx context.Context, id int64, now time.Time) (bool, error)
	// MarkFailedUnknown 是 G15（WAITING_CONFIRMATION → FAILED_UNKNOWN，逾 24h）。
	MarkFailedUnknown(ctx context.Context, id int64, now time.Time) (bool, error)

	// GetByRefund 是 uq_invadj_allowance_refund 的反查（I-11）：「這筆退款開過折讓了嗎」。
	//
	// 🔴 冪等一律走這一句，**不要**靠 INSERT 撞唯一鍵——PostgreSQL 上撞唯一鍵會讓整個
	// 交易 aborted（25P02），之後連 catch 後的 SELECT 都會被拒絕。
	GetByRefund(ctx context.Context, documentID int64, refundNo string) (Adjustment, bool, error)

	// CancelUnsentForIssuer 是 P14／P16 的副作用：憑證停用／撤銷之後，該 issuer 底下
	// 還沒送出去的調整永遠送不出去了。allowed-from 只取「還沒送出去」的四個
	// ——WAITING_CONFIRMATION／FAILED_UNKNOWN 的折讓可能已經在 AMEGO 端生效。
	CancelUnsentForIssuer(ctx context.Context, issuerID int64, cancelReasonCode string, now time.Time) (int64, error)

	// AdminRetry／AdminCancel 是 ADM 對這筆調整的 operation 按端點 52／53 時的
	// **同一事務內連動**（G16／G19）。少了 AdminRetry，重試會必然再失敗：
	// operation 回 READY 而 adjustment 還在 FAILED_PERMANENT ⇒ 下一輪 claim 的
	// MarkSending（allowed-from = PENDING）rows=0。
	AdminRetry(ctx context.Context, id int64, expectedVersion int64, now time.Time) (bool, error)
	AdminCancel(ctx context.Context, id int64, expectedVersion int64, cancelReasonCode string, now time.Time) (bool, error)
}

// NewComplaint 是 K01（買家提出申訴）的落庫輸入。
//
// SlaDueAt 由呼叫端用 BusinessCalendar.ComplaintSlaDueAt 算好傳入：該欄 NOT NULL 且
// 必須在建案當下落庫（§5.1.14），不能掛在 shipping-service 的可用性上。
type NewComplaint struct {
	ID                 int64
	DocumentID         int64
	IssuerID           int64
	OrderNo            string
	ComplainantBuyerID int64
	ReasonCode         string
	Description        string
	SlaDueAt           time.Time
	Now                time.Time
}

// NewComplaintEvent 是申訴時間軸的一列（append-only）。
//
// 🔴 Action 只裝 InvoiceComplaintAction 的 6 個生成常數（賣家／ADM 在畫面上點了哪一顆）。
// 買家發起的三條轉移（K07／K11／K13）一律留空 —— 買家的動作不進那個軸（§7.3.5 逐字）。
type NewComplaintEvent struct {
	ComplaintID    int64
	EventType      string
	Action         string
	ActorType      string
	ActorAccountID int64
	Message        string
	Now            time.Time
}

// DecideComplaint 是 K08／K09 的輸入（端點 60 的 DECIDE_RESOLVED／DECIDE_VIOLATION）。
//
// ReopenAllowedAfter 是 narg：K08 傳 now + 24h（K13 重開邊的守衛讀它）；K09 傳 nil
// （VIOLATION_CONFIRMED 是終態，沒有重開邊，寫了也沒有讀者）。
type DecideComplaint struct {
	ID                 int64
	ExpectedVersion    int64
	NextStatus         string
	ResolutionCode     string
	ResolutionNote     string
	ReopenAllowedAfter *time.Time
	DecidedByAccountID int64
	Now                time.Time
}

// ComplaintRepository 讀 invoice_complaints 與時間軸。三支集合式 UPDATE 由 SLA 排程呼叫。
type ComplaintRepository interface {
	GetByID(ctx context.Context, id int64) (Complaint, error)

	// ── 買家發起的寫入（K01／K07／K11／K13，§6.1.6 端點 13／63／64／65）──
	//
	// 🔴 四支的擁有者述詞全部在 SQL 的 WHERE 裡（K01 是建列、其餘三支帶
	// complainant_buyer_id）：Tier A 的「越權與不存在不可分辨」不能靠 Go 端的 if——
	// 那會多出一個可以回不同 code／message 的分支（§8.1.4(b)）。
	//
	// 🔴 三支狀態轉移回 false 一律代表**守衛不成立**（allowed-from 不符、版本被人搶先、
	// 重開次數用完、冷卻未到），不得當成功往下走：那正是 rows=0 靜默失敗的形狀。

	// Create 走 ON CONFLICT DO NOTHING（不指名 target）。inserted=false ⇒ 這份 document
	// 已有一件未結案的申訴（uq_invcomp_open_per_document），呼叫端改回 9310 並帶既有 id。
	Create(ctx context.Context, in NewComplaint) (c Complaint, inserted bool, err error)

	// CountWithdrawnByDocument 是 K01 守衛②的資料面：撤回不是免費的重來鍵。
	// WITHDRAWN 不在 uq_invcomp_open_per_document 的述詞內 ⇒ 沒有這個計數，
	// 撤回→重提可以無限循環，同時繞過 reopen_count ≤ 2 的上限。
	CountWithdrawnByDocument(ctx context.Context, documentID int64) (int64, error)

	// EscalateByBuyer 是 K07（端點 63 買家不接受）。allowed-from 只有 SELLER_RESPONDED。
	EscalateByBuyer(ctx context.Context, id, buyerID, expectedVersion int64, now time.Time) (bool, error)

	// Withdraw 是 K11（端點 64 買家撤回）。allowed-from ＝ OPEN／OVERDUE／SELLER_RESPONDED。
	Withdraw(ctx context.Context, id, buyerID, expectedVersion int64, now time.Time) (bool, error)

	// Reopen 是 K13（端點 65 買家重開）。三條守衛（狀態、reopen_count < 2、冷卻已過）
	// 全部在同一句 UPDATE 的 WHERE，讀的是轉移前的舊值——放應用層 if 擋不住兩個分頁
	// 同時按下（各自讀到 0 再各自寫 1）。
	Reopen(ctx context.Context, id, buyerID, expectedVersion int64, slaDueAt, now time.Time) (bool, error)

	// ── ADM 發起的寫入（K08／K09／K10，§6.1.16 端點 60）──
	//
	// 沒有 buyerID 述詞：ADM 本來就看得到全部案件，Tier A 在這條路上不成立。
	// 兩支回 false 的意義與買家側三支完全相同（守衛不成立，回 9336）。

	// DecideEscalated 是 K08（→ RESOLVED）與 K09（→ VIOLATION_CONFIRMED），共用一句。
	DecideEscalated(ctx context.Context, in DecideComplaint) (bool, error)
	// ReturnToSeller 是 K10（ESCALATED_TO_ADM → OPEN），sla_due_at 由呼叫端以工作天曆重算。
	ReturnToSeller(ctx context.Context, id, expectedVersion int64, slaDueAt, now time.Time) (bool, error)

	// ── 賣家發起的寫入（K02／K03，§6.1.11 端點 36）──
	//
	// 🔴 兩支的擁有者述詞（經 invoice_issuers 解析的 store_id）與 expectedVersion 都在
	// SQL 的 WHERE，與買家側三支同一條規則：先 SELECT 再 UPDATE 的寫法在兩句之間有窗口，
	// 而且把 Tier A 的保證搬到了呼叫端的 if 上（§8.1.4(b)）。
	//
	// 🔴 **兩條邊不可合併**：K02 收 OPEN／OVERDUE，K03 只收 OPEN。合成一句 2 值
	// allowed-from ⇒「逾期之後才請求平台協助」會靜默成功而狀態沒變。

	// MarkSellerResponded 是 K02（OPEN／OVERDUE → SELLER_RESPONDED）。
	// buyerReviewDueAt 由呼叫端算（now + 72h）：它是 K06「逾期視為接受」唯一的觸發依據，
	// 少寫**不會報錯**——那一列只是從 K06 的候選集合裡消失，該案永遠不會自動結案。
	MarkSellerResponded(ctx context.Context, id, storeID, expectedVersion int64, buyerReviewDueAt, now time.Time) (bool, error)
	// EscalateBySeller 是 K03（OPEN → ESCALATED_TO_ADM，賣家選「需要平台協助」）。
	EscalateBySeller(ctx context.Context, id, storeID, expectedVersion int64, now time.Time) (bool, error)

	// AppendEvent 是時間軸的唯一寫入點，必須與狀態轉移同一個交易。
	AppendEvent(ctx context.Context, in NewComplaintEvent) (ComplaintEvent, error)

	// GetForBuyer 是買家面的授權讀取：complainant_buyer_id 寫在 SQL 的 WHERE
	// （§8.1.4(b)），查不到與不屬於你由同一行程式碼回同一個業務碼。
	//
	// 🔴 不得用 GetByID ＋ Go 端比對 ComplainantBuyerID 代替：那會多出一個可以回不同
	// code／message 的分支，Tier A 防枚舉當場失效而所有測試照樣綠（§8.6 T-10）。
	GetForBuyer(ctx context.Context, id, buyerID int64) (Complaint, error)
	ListEvents(ctx context.Context, complaintID int64) ([]ComplaintEvent, error)
	// CountConfirmedViolationOrders 是滑動窗即時計算，不維護 violation_count 計數器欄。
	CountConfirmedViolationOrders(ctx context.Context, issuerID int64, windowStart time.Time) (int64, error)

	// ── 讀取視圖 ──
	// GetForSeller 的 storeID 是述詞的一部分（經 invoice_issuers 解析），
	// 與 GetForBuyer 同一條規則：越權與不存在由同一行程式碼回同一個業務碼。
	GetForSeller(ctx context.Context, id, storeID int64) (Complaint, error)
	ListForBuyerOrder(ctx context.Context, in BuyerComplaintQuery) ([]Complaint, error)
	CountForBuyerOrder(ctx context.Context, in BuyerComplaintQuery) (int64, error)
	ListForSeller(ctx context.Context, in SellerComplaintQuery) ([]Complaint, error)
	CountForSeller(ctx context.Context, in SellerComplaintQuery) (int64, error)
	ListForAdmin(ctx context.Context, in AdminComplaintQuery) ([]AdminComplaintItem, error)
	CountForAdmin(ctx context.Context, in AdminComplaintQuery) (int64, error)
	MarkOverdue(ctx context.Context, now time.Time, limit int32) ([]Complaint, error)
	EscalateOverdue(ctx context.Context, now, escalateBefore time.Time, limit int32) ([]Complaint, error)
	ResolveExpiredBuyerReviews(ctx context.Context, now, reopenAllowedAfter time.Time, resolutionCode string, limit int32) ([]Complaint, error)
}

// ⚠️ 這個型別住在 domain 而不是 internal/mq：usecase 與 client 都要用它，
// 放 mq 會讓 usecase → mq → usecase 形成 import cycle（實測擋下）。
// ReviseSnapshotInput 是 R03（開立前直接改）要送去 order-service 的內容軸欄位。
//
// 🔴 **刻意沒有 NotifyEmail**（58 號 §6.2.5 定版 7）：snapshot_version 是內容軸，
// 只承載發票上會印出來的東西；通知信箱只決定通知寄到哪，由本服務就地改
// invoice_documents.recipient_email。放進來會讓它遞增內容軸，使「同內容重開一張票」
// 與「內容真的變了」在冪等鍵上無法區分（那正是 issue_attempt_seq 實體軸的存在理由）。
type ReviseSnapshotInput struct {
	OrderNo string
	// ExpectedSnapshotVersion 是呼叫端讀到的版本。對方比對不符即回 9308、不寫入——
	// 沒有它，兩筆並行更正是後寫的靜默覆蓋前者，而兩筆都會顯示成功。
	ExpectedSnapshotVersion int64

	InvoiceKind              string
	CarrierSelectionMode     string
	BuyerInvoicePreferenceID int64 // 0 ＝ 未提供
	BuyerTaxID               string
	BuyerTitle               string
	DonationCode             string

	// IdempotencyKey ＝ correction_id。同鍵重送必須回同一個版本、不新增列；
	// 空字串會讓每次重試都變成一次新的更正，client 端就擋掉。
	IdempotencyKey string
}

// ReviseDocumentSnapshot 是 R03 對 invoice_documents 的就地更新輸入。
//
// 欄位對應 InsertInvoiceDocument 的同名參數。**明文進、密文出**：加密與雜湊
// 由 repository 內部用金鑰環做，與 NewDocument 同一條線（§5.1.1）。
type ReviseDocumentSnapshot struct {
	ID int64

	// DocumentKind 與金額四欄由呼叫端**依新的買受人重算**後傳入（#3085／#3089 A-3）。
	//
	// 🔴 不得沿用原文件的值：買家補打統編會讓 B2C 變 B2B，而兩者的
	// chk_invdoc_b2c／chk_invdoc_b2b／chk_invdoc_total 三條約束互相排斥
	// ⇒ 原樣複製的症狀是就地 UPDATE 當場 23514，買家看到 500。
	// 算式走 domain.BuildHeader（那是唯一一份），不要自己拆稅。
	DocumentKind       string
	SalesAmount        int64
	FreeTaxSalesAmount int64
	ZeroTaxSalesAmount int64
	TaxAmount          int64
	TotalAmount        int64
	// StatutoryDeadline 跟著 DocumentKind 走（B2C 2 天／B2B 7 天）。
	// nil ⇒ 不動原值（原文件沒有 paid_at 時算不出基準日）。
	StatutoryDeadline *time.Time
	// ExpectedVersion 是樂觀鎖：與 DB 現況不符即 rows=0。少了它，兩筆並行更正
	// 是後寫的靜默覆蓋前者。
	ExpectedVersion int64
	// SnapshotVersion 只能來自 order-service 的回傳值（I-50）。
	SnapshotVersion int32

	RecipientType  string
	RecipientRefID *int64
	RecipientName  string
	RecipientTaxID string

	// 🔴 以下是**明文**，加密與雜湊由 repository 內部做（與 NewDocument 一致）。
	// 一度設計成收密文，那是錯的：加密金鑰環住在 repository，呼叫端拿不到它
	// ⇒ 只會逼出「usecase 自己接一份 ring」這種第二把鑰匙。
	RecipientEmail string
	CarrierID1     string
	CarrierID2     string

	CarrierType  *string
	DonationCode *string

	Now time.Time
}

// CorrectionRequestRepository 讀 invoice_correction_requests。
type CorrectionRequestRepository interface {
	GetByID(ctx context.Context, id int64) (CorrectionRequest, error)
	// GetOpenByDocument 回 ErrNotFound 代表這張憑證目前沒有進行中的更正請求。
	//
	// ⚠️ 它的述詞**只有** REQUESTED／VOID_REISSUE_IN_PROGRESS（與序列閘門的
	// partial unique index 逐字相同）⇒ 停在 REISSUE_FAILED 的那些查不到。
	// 要找那一類走 GetReissueFailedByDocument，不要放寬本支（理由見該支）。
	GetOpenByDocument(ctx context.Context, documentID int64) (CorrectionRequest, error)

	// GetLatestByOrder 回這張**訂單**上最近一次修改請求，不分狀態。
	//
	// 🔴 B04（端點 8）用它產出 latest_correction_id，讓買家在**重新載入之後**
	// 仍看得到申請進度與婉拒理由。GetOpenByDocument 做不到這件事：它的述詞不含
	// REJECTED，而那正是買家最需要看到的一筆。
	// 2026-09-05 第 6 輪稽核：網頁的 `fetchInvoiceCorrection`（端點 12 的 client）
	// 零呼叫端，因為端點 8 從來沒有給出任何 correction_id。
	//
	// 🔴 **範圍是訂單不是單一 document**（2026-09-05 修）：更正請求掛在被作廢的那張
	// document 上，作廢重開完成後訂單的最新 document 是重開的新那張、身上沒有 correction
	// 列 ⇒ 以「最新 document」查會讓每一次成功的作廢重開把 latest_correction_id 打回空。
	GetLatestByOrder(ctx context.Context, orderNo string, buyerID int64) (CorrectionRequest, error)

	// GetReissueFailedByDocument 找停在 REISSUE_FAILED 的修改請求（R09／R10 的入口）。
	//
	// 🔴 它與 GetOpenByDocument 分成兩支是刻意的：REISSUE_FAILED 不在
	// uq_invcorr_open_per_document 裡 ⇒「一筆 REISSUE_FAILED ＋ 一筆新 REQUESTED」
	// 在 DB 層合法，把它併進那一支會讓「最多一列」變成「可能多列」，而 sqlc 的 :one
	// 對多列是取任意一列、零錯誤零 log。
	GetReissueFailedByDocument(ctx context.Context, documentID int64) (CorrectionRequest, error)

	// RetryReissue 是 §7.3.5 **R09**（REISSUE_FAILED → VOID_REISSUE_IN_PROGRESS）。
	//
	// 🔴 它在 2026-09-04 之前**零呼叫端、連 adapter 都不存在**（#3089 A 組第 1 項）：
	// SQL 與 sqlc 生成物一直都在，SQL 檔頭還寫著「沒有第二個觸發者」——而那個
	// 唯一的觸發者從來沒有被接上。後果是 REISSUE_FAILED（設計書自稱「最嚴重的
	// 單一狀態」：買家的合法發票已作廢、新發票不存在）**沒有任何出口**。
	//
	// 由端點 52（ADM 安全重試）在 operation 回 READY 的同一事務內連動。
	// 回 false ＝ CAS rows=0，**不是錯誤**：那代表這張 document 上沒有停在
	// REISSUE_FAILED 的請求（絕大多數重試都是這樣），呼叫端不動它。
	RetryReissue(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error)

	// GetForBuyer 是端點 12 的授權讀取。**三段述詞缺一不可**：申請人 ＝ 當前買家、
	// 該 document 的買受人 ＝ 當前買家、該 document 屬於 request 帶進來的那張訂單。
	// 只驗其一會讓「訂單是我的但發票的買受人不是我」變成可讀（§8.1.4(a)）。
	GetForBuyer(ctx context.Context, id, buyerID int64, sourceRef string) (CorrectionRequest, error)

	// Insert 是 §7.3.5 **R01**（買家於 B06 送出）。
	//
	// 🔴 序列閘門（I5）的 DB 落點就是本句的 ON CONFLICT：
	// uq_invcorr_open_per_document（REQUESTED／VOID_REISSUE_IN_PROGRESS 兩值）
	// ⇒ created=false 代表這張發票**已有進行中的修改請求**，呼叫端應回 9351
	// 並改讀 GetOpenByDocument，不是重試。
	//
	// before/after payload **必須已遮罩**：密文欄位存 hash 與遮罩值，不存明文。
	Insert(ctx context.Context, in NewCorrectionRequest) (CorrectionRequest, bool, error)

	// Reject 是 §7.3.5 的 R02（預檢不過）與 R05（f0501 作廢被 provider 拒絕）—— 兩條邊
	// 共用一句 SQL，差別只在**發現的時點**，買家看到的答案與 reason code 值域都一樣。
	//
	// reasonCode 必須是 chk_invcorr_reject_reason_code 的 7 值之一；chk_invcorr_reject_reason
	// 要求 REJECTED ⇒ reason code 非空，所以兩欄同一句寫。
	//
	// 回 false ＝ CAS rows=0（狀態已不在 REQUESTED／VOID_REISSUE_IN_PROGRESS）。**不是錯誤**，
	// 但呼叫端不得當成成功——那代表有別人先動了這一列。
	Reject(ctx context.Context, in RejectCorrection) (bool, error)

	// MarkUpdatedBeforeIssue 是 R03 的落地（REQUESTED → UPDATED_BEFORE_ISSUE，終態）。
	//
	// 🔴 本句**不是** R03 的守衛：守衛是 operation 的 `HELD` CAS rows=1，搶不到就代表
	// 那筆已經在打 AMEGO、R03 不成立，correction 要改走 R04 作廢重開。呼叫端必須先搶到
	// HELD 才呼叫本支，順序反了會落一個「快照改了但發票已經用舊內容送出去」的狀態。
	MarkUpdatedBeforeIssue(ctx context.Context, id int64, now time.Time) (bool, error)

	// StartVoidReissue 是 §7.3.5 **R04**（預檢通過且 document 已 ISSUED）。
	//
	// voidAdjustmentID 指向同一事務內建立的 VOID adjustment（fk_invcorr_void）。
	//
	// 🔴 呼叫順序是「**先確保可開再作廢**」：四項可預先判定的失敗（字軌不足／載具失效／
	// 已過期限／已存在折讓）必須在建 VOID operation **之前**擋掉。作廢不可逆，
	// 且明令不可用 DB restore 把 ISSUED 回復為未開立——順序反了，買家手上一張都沒有。
	StartVoidReissue(ctx context.Context, id, voidAdjustmentID int64, now time.Time) (bool, error)
	// MarkReissueFailed 是 R08（作廢成功、重開 operation 進入任一 FAILED_*）。
	// 🔴 這是本設計書中最嚴重的單一狀態：買家原本持有的合法發票已經作廢、新發票不存在，
	// 而這是平台的操作造成的。呼叫端必須在同一批做完 A06 佇列／15 分鐘 SLA 告警／
	// 買家通知（不得沿用「處理中」通用文案）三件。
	MarkReissueFailed(ctx context.Context, id int64, now time.Time) (bool, error)
	// Complete 是 R10（重開的新 document 轉 ISSUED）。守衛「原 document 已轉 SUPERSEDED」
	// 跨表，由呼叫端在同一交易內先做 MarkSuperseded。
	Complete(ctx context.Context, id, reissuedDocumentID int64, now time.Time) (bool, error)
}

// NewCorrectionRequest 是 R01 的落庫輸入。
//
// 🔴 BeforePayload／AfterPayload **必須已遮罩**（密文欄位存 hash 與遮罩值）：
// 這兩欄是爭議時的憑據，會被 ADM 讀到——存明文等於把載具與 email 複製了一份到
// 一張沒有加密欄的表上。
type NewCorrectionRequest struct {
	ID         int64
	DocumentID int64
	// RequestedByBuyerID 是申請人，也是端點 12 授權讀取的述詞之一。
	RequestedByBuyerID int64
	// ExpectedSnapshotVersion 是買家送出當下看到的內容版本。與 document 的比對由
	// usecase 做（本表只存快照，不做跨表比較）。
	ExpectedSnapshotVersion int32

	BeforePayload []byte
	AfterPayload  []byte

	Now time.Time
}

// RejectCorrection 是 R02／R05 的輸入。
type RejectCorrection struct {
	ID         int64
	ReasonCode string
	// ReasonDetail 可空。它是給買家看的補充說明，不是碼值——B06 顯示的是後端給的
	// 具體理由，不由前端拼字串（§7.3.5 R02）。
	ReasonDetail *string
	Now          time.Time
}

// BuyerPreferenceRepository 讀 buyer_invoice_preferences。
type BuyerPreferenceRepository interface {
	// GetByID 帶 buyerID 是授權述詞的一部分，不是多餘參數。
	GetByID(ctx context.Context, id, buyerID int64) (BuyerPreference, error)

	// ListByBuyer 回這位買家存活的常用發票資料（created_at DESC，最多 limit 筆）。
	//
	// buyerID 是授權述詞的一部分（寫在 SQL 的 WHERE，§8.1.4(b)），limit 是 SQL 的
	// LIMIT 而不是 Go 端截斷。
	//
	// 🔴 **不得拿它代替 GetDefault**：它帶 LIMIT，預設那一列排在 limit 之外時會靜默
	// 回「沒有預設」（db/queries/buyer_invoice_preferences.sql 的 GetDefault 註解逐字
	// 寫了這條）。兩支的用途不同：本支給 B08 列表，GetDefault 給結帳缺省鏈。
	ListByBuyer(ctx context.Context, buyerID int64, limit int32) ([]BuyerPreference, error)

	// GetDefault 取這個買家標成預設的那一筆（USE_BUYER_DEFAULT 的第一步）。
	// found=false 是**正常分支**（多數買家沒設過），不是錯誤——把它表現成 error
	// 會讓 order.paid consumer 整批進 DLQ。
	GetDefault(ctx context.Context, buyerID int64) (pref BuyerPreference, found bool, err error)

	// WithDecryptedCarrierValue 是載具顯碼明文的唯一出口：明文只活在 fn 的作用域內，
	// 不進 domain struct、不進 log。buyerID 是授權述詞的一部分（I-47）。
	WithDecryptedCarrierValue(ctx context.Context, id, buyerID int64, fn func(carrierType, carrierValue string) error) error

	// Create 新增一列（B08 端點 2；端點 3 的「編輯」也走它，見 Supersede）。
	//
	// inserted=false 是 ON CONFLICT DO NOTHING 的 rows=0：這位買家已經有一列同載具
	// 或同統編的存活資料。它是**業務結果不是錯誤**——回 error 會讓呼叫端分不出
	// 「重複」與「DB 掛了」，而兩者的處置相反。
	Create(ctx context.Context, in NewBuyerPreference, now time.Time) (pref BuyerPreference, inserted bool, err error)

	// ClearDefault 把這位買家所有存活列的 is_default 清為 false，回被改到的列數。
	//
	// 🔴 它與 SetDefault 必須在**同一個交易**內：uq_bip_default 是 partial
	// UNIQUE(buyer_id) ⇒ 不先清就設會撞唯一鍵；清了不設（或中間失敗未 rollback）
	// 則買家會落在「零個預設」而畫面上什麼都沒發生。
	ClearDefault(ctx context.Context, buyerID int64, now time.Time) (int64, error)

	// SetDefault 把一列標成預設。ok=false ＝ 述詞不成立（不存在／不屬於你／已刪／已取代），
	// 四者刻意不可分辨（Tier A，§8.1.4(b)）。
	SetDefault(ctx context.Context, id, buyerID int64, now time.Time) (ok bool, err error)

	// Supersede 把舊列指向新列（端點 3「編輯」的後半）。
	//
	// 🔴 supersededByID 那一列必須**已經存在**：fk_bip_superseded 不是 DEFERRABLE ⇒
	// 順序只能是 Create 先、Supersede 後。SQL 註解說的「被取代的列離開三個 partial
	// unique」講的是**之後**再新增同值時不會撞去重鍵，不是本次的順序。
	Supersede(ctx context.Context, id, buyerID, supersededByID int64, now time.Time) (ok bool, err error)

	// SoftDelete 軟刪一列（端點 4）。ok 的語意同 SetDefault。
	SoftDelete(ctx context.Context, id, buyerID int64, now time.Time) (ok bool, err error)
}

// NewBuyerPreference 是新增一列常用發票資料的輸入。
//
// 🔴 CarrierValue 是**明碼**，它在本 struct 的生命週期只到 repository 的 Create：
// 加密（AES-256-GCM）與 lookup hash（HMAC-SHA256）都在那一層算，與
// ProviderAccountRepository 的 deriveCredential 同一個收斂點。上層不得把它寫進
// log、不得放進任何回應（§8.1.3）。
//
// 空字串一律代表「這一型沒有這個欄位」⇒ repository 寫 NULL。chk_bip_shape 的四個
// 分支就是靠 NULL 與否判定的，寫進空字串會讓那條 CHECK 認為欄位存在。
type NewBuyerPreference struct {
	ID      int64
	BuyerID int64
	// PreferenceKind 是 CodeMeta InvoiceCarrierType 的 4 值之一（對外 JSON 名 carrierType）。
	PreferenceKind string
	// CarrierType 是 AMEGO 原值（3J0002／CQ0001）；統編型與捐贈型為空。**不出 API**。
	CarrierType  string
	CarrierValue string
	// CarrierMask 是落庫的遮罩顯示值（carrier_value_mask）。由組回應的那一層算，
	// 不在 repository 算：遮罩形狀是顯示規則，而 repository 不該有顯示規則。
	CarrierMask  string
	TaxID        string
	TaxTitle     string
	DonationCode string
	Label        string
	IsDefault    bool
}

// NewStatusHistoryEntry 是狀態歷程的落庫輸入（append-only）。
type NewStatusHistoryEntry struct {
	SubjectType    string
	SubjectID      int64
	FromStatus     *string
	ToStatus       string
	ReasonCode     *string
	ReasonDetail   *string
	ActorType      string
	ActorAccountID *int64
	CorrelationID  *string
	Now            time.Time
}

// NewProviderAuditLogEntry 是 provider 往返稽核的落庫輸入。
//
// ⚠️ RequestMasked／ResponseMasked 必須是**已遮罩**的內容：本層不做遮罩（它不知道
// 哪些欄位敏感），但也不會替呼叫端補救。AppKey 本來就不在 AMEGO 的四欄 form 裡
// （§8.1.1(h) 第 5 段），所以存 request 原文不會存到 AppKey。
type NewProviderAuditLogEntry struct {
	OperationID     *int64
	IssuerID        int64
	Provider        string
	Endpoint        string
	AttemptSeq      int32
	RequestMasked   json.RawMessage
	ResponseMasked  json.RawMessage
	RawBodyHash     string
	HTTPStatus      *int32
	ProviderCode    *string
	ProviderMessage *string
	LatencyMs       *int32
	ErrorKind       *string
	CorrelationID   *string
	Now             time.Time
}

// NewAdminAuditLogEntry 是 ADM 操作稽核的落庫輸入。
type NewAdminAuditLogEntry struct {
	ActorAccountID         int64
	Action                 string
	ResourceType           string
	ResourceID             int64
	BeforeState            json.RawMessage
	AfterState             json.RawMessage
	ReasonCode             string
	ReasonDetail           string
	ApprovalActorAccountID *int64
	TraceID                *string
	RequestID              *string
	CorrelationID          *string
	ClientIP               *string
	UserAgent              *string
	Now                    time.Time
}

// AuditRepository 蓋三張 append-only 的稽核／歷程表。
type AuditRepository interface {
	AppendStatusHistory(ctx context.Context, in NewStatusHistoryEntry) error
	ListStatusHistory(ctx context.Context, subjectType string, subjectID int64) ([]StatusHistoryEntry, error)
	AppendProviderAuditLog(ctx context.Context, in NewProviderAuditLogEntry) error
	ListProviderAuditLogsByOperation(ctx context.Context, operationID int64) ([]ProviderAuditLogEntry, error)
	// LatestProviderCorrelationID 回這位 issuer 最近一次有 correlation_id 的往返（#3031）。
	// 查無回空字串——S05 的「最近一次請求編號」在還沒打過 AMEGO 時本來就沒有值。
	LatestProviderCorrelationID(ctx context.Context, issuerID int64) (string, error)
	AppendAdminAuditLog(ctx context.Context, in NewAdminAuditLogEntry) (int64, error)
	ListAdminAuditLogsByResource(ctx context.Context, resourceType string, resourceID int64, limit int32) ([]AdminAuditLogEntry, error)

	// ListAdminAuditLogsForAdmin 是端點 61（A10），九個 filter。
	// store_id／order_no／operation_type 三個 filter 沒有對應欄位，只能把
	// (resource_type, resource_id) 這對多型指標解析回實體再 join——五條 LEFT JOIN
	// 各自被 resource_type 夾住，否則 bigserial 與 TSID 混用之下幾乎必然誤配。
	ListAdminAuditLogsForAdmin(ctx context.Context, in AdminAuditLogQuery) ([]AdminAuditLogItem, error)
	CountAdminAuditLogsForAdmin(ctx context.Context, in AdminAuditLogQuery) (int64, error)
}

// BusinessDayRepository 是行事曆鏡像（§5.1.14）。整段 DELETE+INSERT 由呼叫端包在
// 一個交易裡跑——本層不自己 Begin/Commit。
type BusinessDayRepository interface {
	DeleteAll(ctx context.Context) (int64, error)
	Insert(ctx context.Context, holidayDate, syncedAt time.Time) error
	ListHolidays(ctx context.Context, from, to time.Time) ([]time.Time, error)
	// SyncedAt 回 nil 代表鏡像是空的：呼叫端必須把它當「今天算不出工作天」處理，
	// 不得當成「沒有假日」（invoice_complaints.sla_due_at 是 NOT NULL）。
	SyncedAt(ctx context.Context) (*time.Time, error)
}

// ============================================================================
// 讀取視圖（分頁列表 ＋ owner-scoped 明細）
//
// 本區塊只放**唯讀**的查詢入參與投影型別。三條規則：
//
//	① 擁有者（BuyerID／StoreID）是**述詞的一部分**，不是「查回來之後才比對」的欄位。
//	   每一支帶擁有者的方法，其 SQL 的 WHERE 內就有它（§8.1.4(b)）——所以查不到與
//	   越權在這一層長得一樣，**沒有第二個分支可以回不同的東西**。
//	   🔴 不得用「不帶擁有者的 GetByID ＋ Go 端 if」代替任何一支：那正是 Tier A
//	   防枚舉失效而所有測試照樣綠的形狀。
//
//	② 狀態集合一律由呼叫端以 []string 傳入（禁硬編碼）。UI 分頁名（AWAITING_SELLER／
//	   NEEDS_MANUAL／NEW…）與 DB 狀態值是兩張碼表，展開表的 SSOT 在 CodeMeta ＋
//	   設計書 §6.1.11／§6.1.12／§6.1.14／§6.1.16，不在 SQL 也不在本檔。
//
//	③ 每一支 List 都有一支述詞逐字相同的 Count。改任一支必須同批改另一支，
//	   否則 total 與 records 分屬兩個母體（症狀：總數說有 30 筆、翻到第二頁是空的，
//	   而沒有任何東西會報錯）。
// ============================================================================

// Page 是 OFFSET 分頁的入參。
//
// 用 OFFSET 而非 keyset 是被對外契約決定的：每個 List*Response 的 data 都要回
// total／page／pageSize，而 keyset 游標既表達不出「第 N 頁」也算不出 total。
type Page struct {
	Limit  int32
	Offset int32
}

// DateRange 是「開立日期軸」的區間，nil ＝ 該側不篩。
//
// 軸的定義（COALESCE(provider_issued_at, created_at)）在
// db/queries/invoice_documents.sql 的區塊註解，本層不重複第二份。
// `YYYY-MM-DD` → UTC 區間的換算與格式驗證都在呼叫端（它才知道 UTC+8 日界）。
type DateRange struct {
	From *time.Time
	To   *time.Time
}

// BuyerDocumentQuery 是端點 68（B09 買家發票中心）的入參。
type BuyerDocumentQuery struct {
	BuyerID      int64
	Dates        DateRange
	Status       string // 空字串 ＝ 不篩
	DocumentKind string // 空字串 ＝ 不篩
	Page         Page
}

// SellerDocumentQuery 是端點 30（S06 賣家銷項）的入參。
//
// AbnormalStatuses 是 AbnormalOnly 為 true 時要匹配的 DB 狀態集合（58A S06 ⑥：
// ISSUE_FAILED／AWAITING_MANUAL）。AbnormalOnly=true 而集合為空 ⇒ **查不到任何列**，
// 那是呼叫端傳錯而不是「沒有異常」，呼叫端必須自己保證兩者同時給。
type SellerDocumentQuery struct {
	StoreID          int64
	Dates            DateRange
	Mode             string
	Status           string
	OrderNo          string
	AbnormalOnly     bool
	AbnormalStatuses []string
	Page             Page
}

// SellerDocumentItem 是 S06 列表列：文件本身 ＋ 一個推導欄。
type SellerDocumentItem struct {
	Document Document
	// LatestOperationID ＝ 0 表示這份文件還沒有任何 operation（TSID 恆 > 0）。
	// 呼叫端據此把 latestJobId 填成空字串——前端靠「有 key 但值是空字串」與
	// 「後端還沒部署這一欄」區分（58A T06），所以不能省略該欄。
	LatestOperationID int64
}

// ReceivedDocumentQuery 是端點 72（S12 賣家進項）的入參。
type ReceivedDocumentQuery struct {
	StoreID int64
	Dates   DateRange
	Status  string
	Page    Page
}

// ReceivedDocumentItem 是 S12 列表列。
//
// recipientNote（用途說明）由這兩個欄位推導：line_seq 最小那一行的品名，行數 > 1
// 時後綴「等 N 項」。字串組裝在呼叫端（那是呈現層的措辭）。
// ⚠️ FirstLineDescription 為空字串 ⇒ 資料有問題（Phase 1 方向二／三／四只由端點 57
// 產生，走 MANUAL_LINES ⇒ 明細行必有 description）。呼叫端**不得靜默回空白**。
type ReceivedDocumentItem struct {
	Document             Document
	FirstLineDescription string
	LineCount            int64
}

// AdminDocumentQuery 是端點 47（A12）的入參。
//
// 🔴 StoreID 是 *int64 而不是 int64：nil ＝ 全平台。它在 ADM 面是**篩選**不是授權，
// 這正是它不能與 SellerDocumentQuery 共用一支查詢的理由——後者的 StoreID 是必填的
// 授權述詞，讓它變成可選會把賣家面的越權防線一起拆掉。
type AdminDocumentQuery struct {
	StoreID *int64
	OrderNo string
	Status  string
	Mode    string
	Dates   DateRange
	Page    Page
}

// AdminDocumentItem 是端點 47 的列表列。
type AdminDocumentItem struct {
	Document          Document
	IssuerType        string
	IssuerRefID       int64
	LatestOperationID int64
}

// OperationView 是 InvoiceJobInfo 的資料面：operation 本身 ＋ 四個只存在於
// invoice_documents 的欄位。
//
// 🔴 invoice_operations **沒有 store_id 也沒有 order_no**（§5.1.8），兩者都掛在
// document 上 ⇒ 這四個欄位只能由 join 帶出來，不得在 operation 上反正規化一份
// （那會製造兩個真相且沒有任何約束在維護一致性，§8.1.4(a)）。
type OperationView struct {
	Operation Operation
	// IssuerRefID 在 issuer_type='SELLER' 時 ＝ store_id。
	IssuerRefID int64
	// DocumentStatus 用來判 NEEDS_MANUAL：AWAITING_MANUAL 屬於文件不屬於 job。
	DocumentStatus           string
	DocumentSourceRef        string // source_type='ORDER' 時 ＝ orders.order_no；否則空字串
	DocumentManualReasonCode string
	// DocumentKind 是 B2B／B2C／RECEIPT。賣家中心靠它分辨這是發票還是收據
	// （#3066 裁決 B：兩者是並列的區塊，文案與操作各自為政）。
	// 🔴 免用統一發票的賣家（PLATFORM_RECEIPT）永遠不會有發票 ⇒ 對他顯示任何
	// 帶「發票」字樣的框架都是錯的，而這一欄是前端唯一分得出來的依據。
	DocumentKind        string
	ExternalInvoiceNo   string
	ExternalInvoiceDate *time.Time
	ExternalReportedAt  *time.Time
}

// AdminOperationQuery 是端點 50（A05 工作與異常佇列）的入參。
//
// StatusFilters 為空 ＝ 不篩 tab。IncludeManualDocuments ＋ ManualDocumentStatuses
// 表達 NEEDS_MANUAL 分頁的**聯集**那一半（job 狀態 OR document.status=AWAITING_MANUAL）。
//
// UploadStatus 三欄一組：HasUploadStatus 關總開關，Codes 是 AMEGO 原始整數集合，
// IncludeNull 單獨表達「還沒查過」——PENDING 投影同時涵蓋 NULL 與在途碼，
// 併進整數陣列表達不出來。
type AdminOperationQuery struct {
	StatusFilters          []string
	IncludeManualDocuments bool
	ManualDocumentStatuses []string
	StoreID                *int64
	OrderNo                string
	OperationType          string

	HasUploadStatus         bool
	UploadStatusCodes       []int16
	UploadStatusIncludeNull bool

	ManualReason string
	Page         Page

	// ExcludeResolvedDocuments 只在 NEEDS_MANUAL 分頁為 true（#3045）：
	// 那一頁的口徑必須與 A01 的 needs_manual KPI 一致，而其餘分頁不得排除
	// （SUCCEEDED 分頁本來就該看得到已作廢的票）。
	ExcludeResolvedDocuments bool
	ResolvedDocumentStatuses []string
}

// PriorityOperationQuery 是端點 37（A01）priorityJobs 的入參。
// 排序鍵是 statutory_deadline_at（NULLS LAST），不是 created_at。
type PriorityOperationQuery struct {
	OpenStatuses           []string
	ManualDocumentStatuses []string
	// ResolvedDocumentStatuses 是「已經沒有人要處理」的否決條件，與 A01 的
	// needs_manual KPI 同一份定義（#3014／#3045）。缺它 ⇒ KPI 與它正下方的清單
	// 互相矛盾。
	ResolvedDocumentStatuses []string
	Limit                    int32
}

// BuyerComplaintQuery 是端點 13（B07 該訂單的申訴案）的入參。
type BuyerComplaintQuery struct {
	BuyerID int64
	OrderNo string
	Page    Page
}

// SellerComplaintQuery 是端點 34（S09）的入參。
//
// SortByDue=true ⇒ due_at_asc（**預設**，最接近逾期者優先）；false ⇒ created_at_desc。
// 用布林而不是把欄名字串傳進來：欄名進 SQL 只能靠字串拼接，那是注入面。
type SellerComplaintQuery struct {
	StoreID       int64
	StatusFilters []string
	SortByDue     bool
	Page          Page
}

// AdminComplaintQuery 是端點 58（A07）的入參。
//
// 🔴 StatusFilters 與 DueOverdue 三欄是 **OR** 關係（`OVERDUE` 是橫切視圖）。
// 這與端點 34 的三個互斥視圖規則相反，兩處不可互相套用（§6.1.16）。
type AdminComplaintQuery struct {
	StatusFilters      []string
	IncludeDueOverdue  bool
	DueOverdueStatuses []string
	Now                time.Time

	StoreID   *int64
	OrderNo   string
	SortByDue bool

	// 違規計數的口徑（滑動窗即時算，不是計數器欄）。與
	// CountConfirmedViolationOrders 同一組參數，兩處必須給同樣的值。
	ViolationStatus      string
	ViolationWindowStart time.Time

	Page Page
}

// AdminComplaintItem 是端點 58 的列表列。
//
// ViolationCountInWindow 必須在列表就給（§6.1.16）：A07 的判斷是「這家店是不是慣犯」，
// 逼 ADM 逐筆點進詳情等於要他用眼睛做 group by。
type AdminComplaintItem struct {
	Complaint              Complaint
	IssuerRefID            int64
	ViolationCountInWindow int64
}

// AdminTaxReviewQuery 是端點 38（A02）的入參。
//
// 🔴 三個條件是 **AND**：NEAR_DEADLINE 與 EXTENDED 的定義都是「前四狀態之一 AND
// 額外條件」（§6.1.12）⇒ 呼叫端送 StatusFilters=前四狀態 ＋ 對應旗標。
// 六個 filter 的筆數加總 ≠ 總筆數，UI 不得顯示「合計」。
//
// Keyword 比對 legal_name／tax_id。**store_name 不在本服務**（seller-service 才有），
// 依店名搜尋要由呼叫端先解析成 store_id。
type AdminTaxReviewQuery struct {
	StatusFilters      []string
	NearDeadlineOnly   bool
	NearDeadlineBefore time.Time
	ExtendedOnly       bool
	Keyword            string
	// OpenComplaintStatuses 供 riskFlags 的 OPEN_COMPLAINTS 判定。
	OpenComplaintStatuses []string
	Page                  Page
}

// AdminTaxReviewItem 是端點 38 的列表列。
type AdminTaxReviewItem struct {
	Profile SellerTaxProfile
	StoreID int64
	// OpenComplaintCount > 0 ⇒ riskFlags 帶 OPEN_COMPLAINTS。
	// 另兩個 flag（TAX_ID_MISMATCH_KYC／REPEAT_APPLICATION）的資料源不在本服務。
	OpenComplaintCount int64
}

// AdminProviderAccountQuery 是端點 42（A04）的入參。
//
// MatchNotStarted ＝ 把「查無 provider account 列」也算進 status 篩選的命中集合
// （對外投影 NOT_STARTED）。NOT_STARTED／CONFIG_ERROR 不落 DB，是投影值。
type AdminProviderAccountQuery struct {
	Provider        string
	StatusFilter    string
	MatchNotStarted bool
	Keyword         string
	Page            Page
}

// ProviderAccountSummary 是 A04 列表所需的 provider account 欄位子集。
//
// 刻意**不重用 ProviderAccount**：那個型別是完整投影，用它承載只 SELECT 了一半的
// 資料，等於交出一個「欄位存在但恆為零值」的實體，而零值與真實值無法分辨。
type ProviderAccountSummary struct {
	ID                  int64
	Provider            string
	ProviderTaxID       string
	Status              string
	StatusReason        *string
	CredentialLast4     string
	VerifiedAt          *time.Time
	AutomationCutoverAt *time.Time
	LastHealthCheckedAt *time.Time
	Version             int64
	UpdatedAt           time.Time
}

// AdminProviderAccountItem 是端點 42 的列表列。
//
// Account 為 nil ⇒ 這個賣家還沒建 provider account 列，對外投影 NOT_STARTED
// （§6.1.13 三態表）。把它當成錯誤或跳過該列，上線當天會少掉全體賣家。
type AdminProviderAccountItem struct {
	Issuer  Issuer
	Account *ProviderAccountSummary
}

// AuditResourceTypes 是 invoice_admin_audit_logs.resource_type 的五個字面值
// （proto target_kind：TAX_PROFILE／PROVIDER_ACCOUNT／JOB／DOCUMENT／COMPLAINT）。
//
// 它們由呼叫端從常數傳入而不是寫死在 SQL：本表是多型指標（resource_type ＋
// resource_id），五條 LEFT JOIN 各自要被對應的字面值夾住，否則 bigserial 與 TSID
// 混用之下幾乎必然誤配到別張表的同號 id。
type AuditResourceTypes struct {
	TaxProfile      string
	ProviderAccount string
	Job             string
	Document        string
	Complaint       string
}

// AdminAuditLogQuery 是端點 61（A10）的入參，九個 filter。
type AdminAuditLogQuery struct {
	ResourceTypes        AuditResourceTypes
	TargetActorAccountID *int64
	StoreID              *int64
	OrderNo              string
	DocumentID           *int64
	JobID                *int64
	OperationType        string
	Dates                DateRange
	Page                 Page
}

// AdminAuditLogItem 是端點 61 的列表列：稽核列 ＋ 由 resource_id 解析出來的關聯。
// 每個解析欄都可能是 nil／空字串（該 resource_type 解析不到就是解析不到）。
type AdminAuditLogItem struct {
	Entry                 AdminAuditLogEntry
	StoreID               *int64
	OrderNo               string
	ResolvedDocumentID    *int64
	ResolvedOperationID   *int64
	ResolvedOperationType string
}

// SellerOverviewCountsQuery 是端點 16（S01）三個計數的入參。
type SellerOverviewCountsQuery struct {
	IssuerID                int64
	PendingDocumentStatuses []string
	OpenComplaintStatuses   []string
	// OverdueComplaintStatuses 與 OpenComplaintStatuses 是 **OR** 關係（見下方
	// SellerOverviewCounts.OverdueComplaintCount 的口徑）。
	OverdueComplaintStatuses []string
	Now                      time.Time
}

// SellerOverviewCounts 是端點 16 的三個計數。
//
// OverdueComplaintCount 的口徑 ＝ `status='OVERDUE'` **OR** 「未結案且 sla_due_at 已過」：
// OVERDUE 由 INV.dl 掃描寫入，掃描到期之間有一段窗口，此時期限已過但狀態還是 OPEN。
// 只取前者會讓賣家在最該行動的那幾小時看到 0（§6.1.11a 同名 KPI）。
type SellerOverviewCounts struct {
	PendingDocumentCount  int64
	OpenComplaintCount    int64
	OverdueComplaintCount int64
}

// AdminOverviewCountsQuery 是端點 37（A01）七個 KPI 的入參。
type AdminOverviewCountsQuery struct {
	TaxReviewQueueStatuses []string
	NearDeadlineBefore     time.Time
	ProviderErrorStatuses  []string
	PendingJobStatuses     []string
	NeedsManualJobStatuses []string
	ManualDocumentStatuses []string
	// ResolvedDocumentStatuses 是 needs_manual 的**否決**集合：document 已收束
	// （作廢／被重開票取代／取消／不適用／賣家自行開立）⇒ 不論它的 operation
	// 停在什麼狀態，都沒有人要處理它了（#3014）。
	ResolvedDocumentStatuses []string
	OpenComplaintStatuses    []string
	OverdueComplaintStatuses []string
	Now                      time.Time
}

// AdminOverviewCounts 是端點 37 的七個 KPI。
//
// ⚠️ **第 8 個 KPI `suspended_sellers` 不在這裡**：停權狀態的權威是 sellers.status
// （seller-service），發票模組只提建議、不執行、也不持有那個狀態（§6.1.16）。
// 在本服務湊一個本地計數等於造第二個真相；呼叫端要嘛從 seller-service 取，
// 要嘛明說取不到，**不得回 0 充數**（0 看起來就像「今天沒有人被停權」）。
//
// NearDeadlineCount 是 PendingTaxReviewCount 的子集、NeedsManualCount 與
// PendingJobCount 不相交但都不是全集 ⇒ **A01 不得顯示任何合計**。
type AdminOverviewCounts struct {
	PendingTaxReviewCount int64
	NearDeadlineCount     int64
	ProviderErrorCount    int64
	PendingJobCount       int64
	NeedsManualCount      int64
	OpenComplaintCount    int64
	OverdueComplaintCount int64
}

// OverviewRepository 供兩支總覽端點取計數。
//
// 兩支各自橫跨 3–4 張表 ⇒ 不屬於任何單一聚合的 repository，獨立成介面。
// 每支都用**單一 SELECT** 一次取回全部計數：拆成 N 次往返會讓每個計數落在不同的
// 時間點，總覽頁出現「待處理 3 筆、但清單只有 2 筆」這種間歇性自相矛盾。
type OverviewRepository interface {
	SellerCounts(ctx context.Context, in SellerOverviewCountsQuery) (SellerOverviewCounts, error)
	AdminCounts(ctx context.Context, in AdminOverviewCountsQuery) (AdminOverviewCounts, error)
}

// ---------------------------------------------------------------------------
// §5.1.16 invoice_manual_unlocks｜A4 人工出口的雙人解鎖（端點 54／55）
// ---------------------------------------------------------------------------

// ManualUnlock 是一列 invoice_manual_unlocks。
//
// Status 是**DB 上的值**，不是對外的值：仍掛著但已過期的那一列在 DB 上仍是
// AWAITING_SECOND_APPROVAL，對外要經 ProjectManualUnlockStatus 投影成 EXPIRED。
// 兩者刻意不合一——寫入路徑的守衛必須是 SQL 的 `expires_at > now`，把投影後的值
// 拿去判寫入就是把守衛搬回應用層。
type ManualUnlock struct {
	ID                    int64
	OperationID           int64
	Status                string
	RequestedByAccountID  int64
	RequestReason         string
	ProviderCheckEvidence string
	ProviderCheckedAt     time.Time
	RequestedAt           time.Time
	ExpiresAt             time.Time
	// CancelEvidence* 是 J28 的證據（#3089 A 組第 2 項）。只有從
	// CANCEL_BLOCKED_UNKNOWN 發起的解鎖會有值；覆核者（第二人）要看得到它，
	// 否則他是在對一個看不見證據的請求按同意。
	CancelEvidenceKind      *string
	CancelEvidenceObjectKey *string
	ApprovedByAccountID     *int64
	DecisionReason          *string
	DecidedAt               *time.Time
	Version                 int64
}

// NewManualUnlock 是端點 54 建立一列解鎖請求的輸入。
//
// ExpiresAt 由呼叫端算（RequestedAt + ManualUnlockExpiry）而不是寫在 SQL：時間來源
// 必須是可注入的 clock，寫成 `now() + interval '24 hours'` 會讓「逾期」那一格沒有任何
// 測試造得出來。
type NewManualUnlock struct {
	ID                    int64
	OperationID           int64
	RequestedByAccountID  int64
	RequestReason         string
	ProviderCheckEvidence string
	ProviderCheckedAt     time.Time
	ExpiresAt             time.Time
	// 見 ManualUnlock 的同名欄位。CANCELLED_BY_ADMIN 那條路徑傳 nil。
	CancelEvidenceKind      *string
	CancelEvidenceObjectKey *string
	Now                     time.Time
}

// DecideManualUnlock 是端點 55 的輸入。
//
// 🔴 NextStatus 只接受 APPROVED／REJECTED；ApprovedByAccountID 同時是 CAS 述詞
// `requested_by_account_id <> $` 的右運算元 ——「不得同一人」這條規則在 SQL 裡，
// 本 struct 只是把值送過去。
type DecideManualUnlock struct {
	ID                  int64
	OperationID         int64
	NextStatus          string
	ApprovedByAccountID int64
	DecisionReason      string
	ExpectedVersion     int64
	Now                 time.Time
}

// ManualUnlockRepository 是 invoice_manual_unlocks 的唯一存取面。
type ManualUnlockRepository interface {
	// ExpireStale 把該 job 底下已過期而仍掛著的那一列就地轉 EXPIRED，回被改到的列數。
	//
	// 🔴 它不是清理工作而是**名額釋放**：uq_invmu_open_per_operation 是 partial unique
	// （WHERE status = 'AWAITING_SECOND_APPROVAL'），而 partial 述詞不能寫 now()。少了
	// 這一句，一個 job 逾期一次之後就永遠再也發起不了第二次解鎖，症狀是 23505 而不是
	// 任何業務碼。呼叫端必須在 Create 之前、**同一交易內**呼叫它。
	ExpireStale(ctx context.Context, operationID int64, now time.Time) (int64, error)

	// Create 建立一列解鎖請求。撞 uq_invmu_open_per_operation 時回 taken=true，
	// 呼叫端**不得再下任何一句 SQL**（PostgreSQL 此刻交易已 aborted，25P02）。
	Create(ctx context.Context, in NewManualUnlock) (ManualUnlock, bool, error)

	GetByID(ctx context.Context, id int64) (ManualUnlock, error)
	// GetOpen 回「尚未決策且尚未逾期」的那一列。
	//
	// 🔴 查無走 found=false **不是 error**：呼叫端要用它分辨「有沒有開著的請求」，
	// 把查無折成錯誤會讓「這個 job 目前沒有待覆核的解鎖」與「這個 job 不存在」共用
	// 同一條路徑，而兩者的下一步相反（一個是可以發起、一個是回 404）。
	GetOpen(ctx context.Context, operationID int64, now time.Time) (ManualUnlock, bool, error)
	// GetLatest 回最近一列（不論狀態），供端點 51 的 manualUnlock 卡使用。
	// found=false ＝ 從未發起過解鎖（proto 的 manual_unlock 回 nil），不是錯誤。
	GetLatest(ctx context.Context, operationID int64) (ManualUnlock, bool, error)

	// Decide 是端點 55 的 CAS。回 false ＝ 守衛不成立（已決策／已逾期／同一人／版本不符），
	// 呼叫端必須先讀一次以回出可分辨的碼（9345 vs 9343），**不得把 false 當成功往下走**。
	Decide(ctx context.Context, in DecideManualUnlock) (bool, error)
}

// ---------------------------------------------------------------------------
// §5.1.15 invoice_provider_cutovers｜自動開立切點歷史（端點 46）
// ---------------------------------------------------------------------------

// ProviderCutover 是一列 invoice_provider_cutovers。
//
// DeactivatedAt 為 nil ＝ 現行有效的那一次切點。同一個帳戶最多只有一列如此
// （uq_invpc_open_per_account）。
type ProviderCutover struct {
	ID                 int64
	ProviderAccountID  int64
	IssuerID           int64
	AccountVersion     int64
	CutoverAt          time.Time
	DeactivatedAt      *time.Time
	Mode               string
	CreatedByAccountID int64
	Reason             *string
	CreatedAt          time.Time
}

// NewProviderCutover 是新增一列切點歷史的輸入。
//
// AccountVersion 是**設定之後**的版本（SetAutomationCutover 已 +1）：端點 46 的
// `version` 要能讓 ADM 對回 invoice_provider_accounts 當時那一版。
type NewProviderCutover struct {
	ProviderAccountID  int64
	IssuerID           int64
	AccountVersion     int64
	CutoverAt          time.Time
	CreatedByAccountID int64
	Reason             string
	Now                time.Time
}

// ProviderCutoverRepository 是 invoice_provider_cutovers 的唯一存取面。
//
// 🔴 CloseOpen ＋ Create 必須與 invoice_provider_accounts 的那一句 CAS **同一交易**：
// 分兩個交易寫，歷史與現值會不一致，而端點 46 是唯一會讀到它的地方——沒有任何東西
// 會發現。
type ProviderCutoverRepository interface {
	// CloseOpen 關掉目前有效的那一列，回被改到的列數。
	// **rows=0 不是錯誤**：第一次設定切點時本來就沒有前一列。
	CloseOpen(ctx context.Context, providerAccountID int64, now time.Time) (int64, error)
	Create(ctx context.Context, in NewProviderCutover) (ProviderCutover, error)
	ListForAdmin(ctx context.Context, providerAccountID int64, page Page) ([]ProviderCutover, error)
	CountForAdmin(ctx context.Context, providerAccountID int64) (int64, error)
}

// ConfirmReceiptAuthorization 是端點 29 的輸入。
//
// AuthorizedAt 與 ConsentVersion **同時為 nil 或同時非 nil**：前者是取消授權、
// 後者是授權。只給其中一個會當場 23514（chk_stp_receipt_auth_pair）。
type ConfirmReceiptAuthorization struct {
	ID              int64
	AuthorizedAt    *time.Time
	ConsentVersion  *string
	ExpectedVersion int64
	Now             time.Time
}

// ApproveSellerTaxProfile 是 T07／T08 的輸入。
//
// ApprovalBasisKey／ApprovalBasisAssetID **同時為 nil 或同時非 nil**
// （chk_stp_receipt_basis_pair）：只給其中一個當場 23514。皆 nil ＝ 不指名依據文件，
// 端點 28 於是回 approvalBasisAvailable=false ＋ NOT_DESIGNATED。
type ApproveSellerTaxProfile struct {
	ID                   int64
	ExpectedVersion      int64
	NextStatus           string
	ApprovalValidUntil   time.Time
	ApprovalBasisKey     *string
	ApprovalBasisAssetID *int64
	Now                  time.Time
}

// ---------------------------------------------------------------------------
// 冪等鍵（#3015 裁決 A）
// ---------------------------------------------------------------------------

// ClaimIdempotency 是認領一把冪等鍵所需的全部輸入。
type ClaimIdempotency struct {
	// Scope 是端點語意（例 BUYER_INVOICE_PROFILE_CREATE）。
	// 🔴 少了它，同一把鍵在不同端點之間會互撞——前端常對整個頁面用同一把鍵。
	Scope string
	// ActorID 依 Scope 是 buyer_id／store_id／actor_account_id。
	// 🔴 少了它，A 買家的鍵會擋住 B 買家，而鍵是前端產生的、不保證全域唯一。
	ActorID int64
	Key     string
	// RequestHash 是「同鍵不同內容」的判別依據。**不存請求原文**：
	// 那裡面有載具明碼與買受人資料，而本表沒有加密欄。
	RequestHash string
	Now         time.Time
}

// IdempotencyRecord 是表上那一列。
type IdempotencyRecord struct {
	Scope       string
	ActorID     int64
	Key         string
	RequestHash string
	// ResultRef 為 nil ＝ 已認領但業務寫入尚未完成。那是同一個交易內的中間狀態，
	// 交易失敗會一起 rollback ⇒ 正常情況下呼叫端讀不到 nil。
	ResultRef *int64
}

// IdempotencyRepository 是冪等鍵的讀寫面。
//
// 🔴 兩支都**必須**在業務寫入的同一個交易內呼叫。那是本機制唯一的正確性來源：
// 分開寫等於在兩次寫入之間留一個窗口，而那正是它要消滅的東西。
type IdempotencyRepository interface {
	Claim(ctx context.Context, in ClaimIdempotency) (IdempotencyRecord, bool, error)
	SetResult(ctx context.Context, scope string, actorID int64, key string, resultRef int64) error
}

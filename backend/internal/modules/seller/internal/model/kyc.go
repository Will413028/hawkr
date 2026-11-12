package model

import (
	"errors"
	"regexp"
	"strings"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/phone"
	"github.com/Will413028/hawkr/backend/internal/platform/twaddr"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// KYCStatus 是 KYC 申請的狀態：只記人工決定（ADR 2026-10-19-seller-kyc-payuni-consolidated）。
type KYCStatus string

const (
	// KYCStatusDraft 賣家填寫中（被退回的申請重新編輯時也回到這裡）。
	KYCStatusDraft KYCStatus = "DRAFT"
	// KYCStatusSubmitted 已送出，等待管理員審核。
	KYCStatusSubmitted KYCStatus = "SUBMITTED"
	// KYCStatusApproved 管理員核准。
	KYCStatusApproved KYCStatus = "APPROVED"
	// KYCStatusRejected 管理員退回，賣家可以修改後重送。
	KYCStatusRejected KYCStatus = "REJECTED"
	// KYCStatusCancelled 賣家取消；之後可以重新建立。
	KYCStatusCancelled KYCStatus = "CANCELLED"
)

// KYCStatusValues 回傳全部儲存的狀態，與 DB 的 kyc_applications_status_check 一致。
func KYCStatusValues() []KYCStatus {
	return []KYCStatus{KYCStatusDraft, KYCStatusSubmitted, KYCStatusApproved, KYCStatusRejected, KYCStatusCancelled}
}

// KYCSellerType 是賣家的型態。
type KYCSellerType string

const (
	// KYCIndividual 個人賣家。
	KYCIndividual KYCSellerType = "INDIVIDUAL"
	// KYCCompany 企業賣家：有統編與公司資料。
	KYCCompany KYCSellerType = "COMPANY"
)

// KYCSellerTypeValues 回傳全部賣家型態，與 DB 的 kyc_applications_seller_type_check 一致。
func KYCSellerTypeValues() []KYCSellerType { return []KYCSellerType{KYCIndividual, KYCCompany} }

// MaxCompanyDocuments 是公司補充文件的張數上限（kyc_documents_position_check）。
const MaxCompanyDocuments = 10

// ReviewAction 是審核紀錄的動作。
type ReviewAction string

const (
	// ReviewSubmit 賣家送出。
	ReviewSubmit ReviewAction = "SUBMIT"
	// ReviewApprove 管理員核准。
	ReviewApprove ReviewAction = "APPROVE"
	// ReviewReject 管理員退回。
	ReviewReject ReviewAction = "REJECT"
	// ReviewCancel 賣家取消。
	ReviewCancel ReviewAction = "CANCEL"
)

// ReviewActionValues 回傳全部動作，與 DB 的 kyc_reviews_action_check 一致。
func ReviewActionValues() []ReviewAction {
	return []ReviewAction{ReviewSubmit, ReviewApprove, ReviewReject, ReviewCancel}
}

// SanctionIdentityKind 是措施身分快照的比對種類，與 DB 的 measure_identities_kind_check 一致。
type SanctionIdentityKind string

const (
	// IdentityTaxID 統編。
	IdentityTaxID SanctionIdentityKind = "TAX_ID"
	// IdentityBankAccount 金流商回報的銀行帳號 fingerprint（Stripe 計畫 D38：平台不再收身分證號與銀行帳號）。
	IdentityBankAccount SanctionIdentityKind = "BANK_ACCOUNT"
)

// SanctionIdentityKindValues 回傳全部比對種類。
func SanctionIdentityKindValues() []SanctionIdentityKind {
	return []SanctionIdentityKind{IdentityTaxID, IdentityBankAccount}
}

// KYCDocument 是申請上的一張公司補充文件圖片；Position 是它在清單的順序（從 0 起）。
type KYCDocument struct {
	Position  int
	AssetID   string
	ObjectKey string
}

// KYCDocumentRef 是草稿裡一張公司文件的指定：AssetID 是這份申請目前的文件（保留），StagedToken 是新上傳的圖片（轉正
// 後換上）；兩者只能給一個。
type KYCDocumentRef struct {
	AssetID     string
	StagedToken string
}

// KYCIssueCode 是退回時一個問題的原因類別，與 DB 的 kyc_review_issues_code_check 一致；具體的說明在 KYCIssue.Message。
type KYCIssueCode string

const (
	// IssueUnreadable 看不清楚（圖片模糊、反光、被遮住）。
	IssueUnreadable KYCIssueCode = "UNREADABLE"
	// IssueMismatch 與其他資料或登記不一致。
	IssueMismatch KYCIssueCode = "MISMATCH"
	// IssueExpired 已過期。
	IssueExpired KYCIssueCode = "EXPIRED"
	// IssueMissing 缺少。
	IssueMissing KYCIssueCode = "MISSING"
	// IssueInvalid 格式或內容不符規定。
	IssueInvalid KYCIssueCode = "INVALID"
	// IssueOther 其他，靠說明。
	IssueOther KYCIssueCode = "OTHER"
)

// KYCIssueCodeValues 回傳全部原因碼，與 DB 的 kyc_review_issues_code_check 一致。
func KYCIssueCodeValues() []KYCIssueCode {
	return []KYCIssueCode{IssueUnreadable, IssueMismatch, IssueExpired, IssueMissing, IssueInvalid, IssueOther}
}

// MaxKYCIssueMessage 是問題說明的字數上限（kyc_review_issues_message_check）。
const MaxKYCIssueMessage = 500

// KYCIssue 是退回時指出的一個問題：欄位（KYCFieldNames 的路徑）、文件類的欄位可以再指到哪一張（asset ID，空字串是不指定特定
// 一張）、原因碼與給賣家看的說明。不含欄位的值。
type KYCIssue struct {
	Field           string
	DocumentAssetID string
	Code            KYCIssueCode
	Message         string
}

// CompanyDocumentsField 是文件類的欄位路徑：申請上唯一的文件類欄位；DB 的 kyc_review_issues_asset_check 綁的是同一個值。
const CompanyDocumentsField = "business.company_documents"

// IsDocumentField 回報欄位路徑是不是文件類的欄位。
func IsDocumentField(field string) bool { return field == CompanyDocumentsField }

// IssueAssetRequired 回報原因碼在文件類欄位上是否必須指定是哪一張文件：UNREADABLE、EXPIRED、MISMATCH、INVALID 的問題出在某一張
// 文件上，不指定就不知道是哪一張。
func IssueAssetRequired(code KYCIssueCode) bool {
	switch code {
	case IssueUnreadable, IssueExpired, IssueMismatch, IssueInvalid:
		return true
	case IssueMissing, IssueOther:
		return false
	}
	return false
}

// IssueAssetForbidden 回報原因碼在文件類欄位上是否不可以指定文件：MISSING 缺的是整個位置，沒有那一張可以指。OTHER 兩者皆可。
func IssueAssetForbidden(code KYCIssueCode) bool { return code == IssueMissing }

// IssuableFields 回傳申請可以在退回時指出問題的欄位路徑（依 KYCFieldNames 的順序）：RejectKYC 驗證 KYCIssue.field 與後台
// 退回表單的欄位選項都用它，「哪個型態有哪些欄位」只有這一份。
func IssuableFields(t KYCSellerType) []string {
	var out []string
	for _, f := range KYCFieldNames() {
		if FieldAppliesTo(f, t) {
			out = append(out, f)
		}
	}
	return out
}

// KYCDocumentFieldNames 回傳文件類的欄位路徑。
func KYCDocumentFieldNames() []string { return []string{CompanyDocumentsField} }

// KYCRejection 是最近一次退回的內容（給賣家改完重送）：總理由與逐項問題（不含只給後台看的內部備註）。
type KYCRejection struct {
	Reason    string
	Issues    []KYCIssue
	DecidedAt time.Time
}

// KYCApplication 是申請的檢視（賣家自查與後台各自的回應都是它）：平台不收身分證號、銀行帳號、證件影像與個人姓名
// （Stripe 計畫 D22、D37），這個型別沒有欄位能放它們。日期是 ISO 8601 的日曆日（YYYY-MM-DD）。
type KYCApplication struct {
	ID                 uuid.UUID
	StoreID, AccountID int64
	Status             KYCStatus
	Version            int64
	SellerType         KYCSellerType
	// StoreName 只在後台的審核佇列帶（列名，D37）。
	StoreName                                           *string
	ProductType                                         *string
	WebsiteURL                                          *string
	CompanyName, TaxID                                  *string
	CompanyRegisterDate                                 *string
	CompanyTownCode, CompanyAddressDetail, CompanyPhone *string
	// MissingFields 是送出之前還缺的欄位（KYCFieldNames 的路徑，依它的順序），與送出檢查的是同一份規則。
	MissingFields []string
	// MissingStoreFields 是送出之前還缺的店家欄位（StoreFieldNames 的路徑，依它的順序），與送出檢查的是同一份規則。
	MissingStoreFields []string
	// Documents 只在讀單筆時帶（清單不帶）。
	Documents []KYCDocument
	// LastRejection 是賣家要處理的那次退回（讀單筆時才帶）：最近一次審核紀錄是退回、之後還沒重新送出、核准或取消；賣家把每一項
	// 問題都處理完、還沒重送時仍在。其他時候是 nil。
	LastRejection *KYCRejection
	// OutstandingIssues 是最近一次退回的問題裡賣家還沒處理的（同讀單筆時才帶）：之後有別的審核紀錄（重新送出、核准、取消）時是空的；
	// 「已處理」在寫入申請時判定（欄位的值被改過，或指到的文件被移除或替換）。
	OutstandingIssues      []KYCIssue
	SubmittedAt, DecidedAt *time.Time
	CreatedAt, UpdatedAt   time.Time
	// DecideDeadline 是審核的期限（seller.review_kyc_application）：審核中（SUBMITTED）才有；ForStore 的讀取填入。
	DecideDeadline *Deadline
}

// KYCReview 是一筆審核紀錄：誰、何時、什麼決定與理由，不含任何欄位值。管理員只記 ID（顯示名稱由 client 經 identity 查）。
type KYCReview struct {
	ID              uuid.UUID
	Action          ReviewAction
	ReviewerAdminID *int64
	Reason          *string
	// Issues 是退回時逐項指出的問題，InternalNote 是退回時的內部備註（只給後台，賣家端的 KYCRejection 沒有）。
	Issues       []KYCIssue
	InternalNote *string
	DecidedAt    time.Time
	// RegistrationCheckSeq 與 RegistrationCheckQueriedAt 是核准或退回當下後台顯示的登記資料查詢（審核的依據）：序號與查詢完成的
	// 時間；沒有引用時序號是 0、時間是 nil（內容用 KYCAdminApplication.RegistrationCheck，只顯示申請目前這一輪送件的最新一筆已完成查詢）。
	RegistrationCheckSeq       int64
	RegistrationCheckQueriedAt *time.Time
}

// KYCDocumentAccess 是一筆補充文件的讀取紀錄（形狀同 evidence_accesses）：哪位管理員、讀哪一張（asset 與當時的位置）、何時。不含圖片的內容。
type KYCDocumentAccess struct {
	ID         uuid.UUID
	AdminID    int64
	Position   int
	AssetID    string
	AccessedAt time.Time
}

// KYCAuditCursor 是補充文件讀取紀錄的 keyset 位置（新到舊）：最後一筆的時間與 ID。
type KYCAuditCursor struct {
	At time.Time
	ID uuid.UUID
}

// Valid 回報位置是不是完整的。
func (c KYCAuditCursor) Valid() bool { return c.ID != uuid.Nil() && !c.At.IsZero() }

// KYCAuditListInput 是一筆申請的讀取紀錄的分頁輸入；PageSize 為 0 用預設值，After 為 nil 從第一頁開始。
type KYCAuditListInput struct {
	ApplicationID uuid.UUID
	PageSize      int32
	After         *KYCAuditCursor
}

// KYCDocumentAccessPage 是一頁補充文件讀取紀錄（新到舊）。
type KYCDocumentAccessPage struct {
	Items []KYCDocumentAccess
	Next  *KYCAuditCursor
}

// KYCAdminApplication 是後台的申請檢視：申請加上審核紀錄。補充文件的讀取紀錄會隨使用增長，分頁（KYCDocumentAccessPage），不放在這裡；
// 審核紀錄的筆數由人工動作限制。
type KYCAdminApplication struct {
	KYCApplication
	Reviews []KYCReview
	// IssuableFields 是這份申請可以在退回時指出問題的欄位路徑（model.IssuableFields，與 RejectKYC 的驗證同一份規則）。
	IssuableFields []string
	// RegistrationCheck 是申請目前這一輪送件（submission_seq；退回後重送是新的一輪）seq 最大的已完成登記資料查詢（只有公司型申請會有；這一輪還沒有已完成的查詢時是 nil，上一輪的快照不顯示）；PendingRegistrationCheck 是申請審核中（SUBMITTED）而且這一輪有比
	// 它更新的查詢還在進行中的那一筆（帶期限：伺服器保證在期限前到終態），沒有時是 nil。後台 etag 由 Version 與 RegistrationCheck 的序號（沒有時是 0）組成。
	RegistrationCheck        *RegistrationCheck
	PendingRegistrationCheck *RegistrationCheck
	// ApproveUnavailable、RejectUnavailable、RequeryUnavailable 是此刻不能核准、退回、重新查詢的原因（KYCActionUnavailableReasons，與三個方法的前置檢查同一個函式，
	// 空表示可以）。身分命中別的帳號生效中的處分時核准仍被擋，那不列在原因裡（不透露）。
	ApproveUnavailable, RejectUnavailable, RequeryUnavailable []StoreActionUnavailableReason
}

// RegistrationCheckSeq 回傳顯示的登記資料查詢的序號（沒有查詢是 0）：後台 etag 的第二個部分。
func (a KYCAdminApplication) RegistrationCheckSeq() int64 {
	if a.RegistrationCheck == nil {
		return 0
	}
	return a.RegistrationCheck.Seq
}

// RequeryKYCRegistrationInput 是後台對一筆審核中的公司型申請重新查詢登記資料。
type RequeryKYCRegistrationInput struct {
	ApplicationID uuid.UUID
	AdminID       int64
}

// ReadKYCDocumentInput 是讀一張補充文件：以 (申請, 位置) 定址；AdminID 是讀的管理員（授權是後台的 KYC 讀取權限，由路由檢查）。
type ReadKYCDocumentInput struct {
	AdminID       int64
	ApplicationID uuid.UUID
	Position      int
}

// FileContent 是一份機敏檔的內容（證件圖片、治理佐證），由擁有者的端點讀出。
type FileContent struct {
	Data        []byte
	ContentType string
}

// ErrFileNotSynced 表示機敏檔在我們這邊有、media 還讀不到，而且它的 media 引用同步還沒結算：稍後重試會好。
var ErrFileNotSynced = errors.New("seller：機敏檔的 media 引用還沒同步完")

// ErrFileMissing 表示我們的資料庫有這份機敏檔、引用也同步過了，media 卻說沒有：資料不一致，要人處理（呼叫方記 error）。
var ErrFileMissing = errors.New("seller：機敏檔在 media 找不到")

// KYCDraftInput 是草稿的部分更新：nil 保留原值，非 nil 的空字串清成沒有值。ExpectedVersion 是讀取時的版本，
// 建立草稿（店家還沒有申請）時必須是 0，並且要給 SellerType。日期是 YYYY-MM-DD，解析不了就拒絕、整筆不寫入。
// 切換 SellerType 時，另一種型態專屬的欄位一併清掉。
type KYCDraftInput struct {
	AccountID, ExpectedVersion                          int64
	SellerType                                          *KYCSellerType
	ProductType                                         *string
	WebsiteURL                                          *string
	CompanyName, TaxID, CompanyRegisterDate             *string
	CompanyTownCode, CompanyAddressDetail, CompanyPhone *string
	// CompanyDocuments 是公司補充文件：nil 保留，非 nil 整個換成清單（順序就是位置，空清單清掉全部）。
	CompanyDocuments *[]KYCDocumentRef
}

// SubmitKYCInput 是送出申請。
type SubmitKYCInput struct {
	AccountID, ExpectedVersion int64
	// AcceptedAgreements 是賣家同意的文件，剛好是 SubmitKYCAgreements 的種類、版本是現行版本。ClientIP 與 UserAgent 是同意
	// 當下的來源，取不到是空字串。
	AcceptedAgreements  []AgreementAcceptance
	ClientIP, UserAgent string
}

// CancelKYCInput 是取消申請；Reason 可以空。
type CancelKYCInput struct {
	AccountID, ExpectedVersion int64
	Reason                     string
}

// ApproveKYCInput 是核准；Comment 可以空，有的話與理由同一條規則（去掉前後空白，至多 1000 字，platform/auditreason）。
//
// ExpectedVersion 與 ExpectedCheckSeq 是讀到的後台 etag 的兩個部分：申請的版本，與當時顯示的登記資料查詢的序號（沒有查詢是 0）。
// 兩者任一與現在不符都是版本衝突：審核員看到的登記資料與現在的不同，不能核准一份他沒看過的比對結果。
type ApproveKYCInput struct {
	ApplicationID                              uuid.UUID
	ExpectedVersion, ExpectedCheckSeq, AdminID int64
	Comment                                    string
}

// RejectKYCInput 是退回；Reason 必填（platform/auditreason），Issues 至少一項，InternalNote 選填、只給後台看。
// ExpectedVersion 與 ExpectedCheckSeq 同 ApproveKYCInput。
type RejectKYCInput struct {
	ApplicationID                              uuid.UUID
	ExpectedVersion, ExpectedCheckSeq, AdminID int64
	Reason                                     string
	Issues                                     []KYCIssue
	InternalNote                               string
}

// KYCCursor 是 admin 審核佇列的 keyset 位置：最後一筆的送出時間與申請 ID（方向由篩選的狀態決定，見 KYCListInput）。
type KYCCursor struct {
	SubmittedAt   time.Time
	ApplicationID uuid.UUID
}

// Valid 回報位置是不是完整的。
func (c KYCCursor) Valid() bool { return c.ApplicationID != uuid.Nil() && !c.SubmittedAt.IsZero() }

// KYCListInput 是 admin 的審核佇列：只列送出過的申請（不含草稿，也不含沒送出就取消的）。排序鍵是送出時間：Status 是
// SUBMITTED（待審核）時舊到新（等最久的在最前面），其他（含 nil，全部）新到舊；兩個方向都以申請 ID 定序。Status 為 nil
// 是全部，DRAFT 與不認得的狀態回 InvalidInput。Query 是店名、公司名稱或統編的子字串（ILIKE，空白不篩選）；PageSize 為 0 用預設值。
type KYCListInput struct {
	Status   *KYCStatus
	Query    string
	PageSize int32
	After    *KYCCursor
}

// OldestFirst 回報這份輸入的佇列是舊到新（只有篩選審核中）。排序方向只有這一處定義。
func (in KYCListInput) OldestFirst() bool {
	return in.Status != nil && *in.Status == KYCStatusSubmitted
}

// KYCPage 是一頁申請；Next 為 nil 表示沒有下一頁。清單的項目不帶 Documents。
type KYCPage struct {
	Items []KYCApplication
	Next  *KYCCursor
}

// KYCSubmitted 在賣家送出申請的 transaction 內發出。沒有對應的舊 routing key（舊系統不通知送出）。
type KYCSubmitted struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
}

// KYCApproved 在管理員核准的 transaction 內發出：這是人工決定；收款帳戶的建立由同一個 transaction 內 payment 模組寫入。
type KYCApproved struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
}

// KYCRejected 在管理員退回的 transaction 內發出；Reason 是退回理由，EventID 是穩定的 UUIDv7，訂閱者重送時不變。目前沒有訂閱者。
type KYCRejected struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
	Reason             string
}

// KYCCancelled 在賣家取消申請的 transaction 內發出。沒有對應的舊 routing key。
type KYCCancelled struct {
	OccurredAt         time.Time
	EventID            uuid.UUID
	ApplicationID      uuid.UUID
	StoreID, AccountID int64
}

// 以下是 KYC 欄位的正規化與驗證：寫入端先檢查，DB 的 CHECK 是保底（同一組規則）。

var (
	taxIDRe  = regexp.MustCompile(`^[0-9]{8}$`)
	mobileRe = regexp.MustCompile(`^09[0-9]{8}$`)
)

// NormalizeTaxID 去掉空白並檢查統編（8 碼數字加檢查碼）：統編的規則只有這一處，申請的欄位（草稿）與登記資料查詢用同一個。
// 檢查碼依財政部 2023 年的規則：各位數依序乘 1、2、1、2、1、2、4、1，乘積的十位數與個位數相加，總和是 5 的倍數（之前是
// 10 的倍數，所以舊的統編都仍然通過）；第 7 位是 7 時，總和或總和加 1 是 5 的倍數。
func NormalizeTaxID(raw string) (string, bool) {
	id := strings.Join(strings.Fields(raw), "")
	if !taxIDRe.MatchString(id) {
		return id, false
	}
	weights := [8]int{1, 2, 1, 2, 1, 2, 4, 1}
	sum := 0
	for i, c := range id {
		p := int(c-'0') * weights[i]
		sum += p/10 + p%10
	}
	return id, sum%5 == 0 || (id[6] == '7' && (sum+1)%5 == 0)
}

// IdentityInput 是統編 blind index 的輸入：標籤做 domain separation，與銀行帳號 fingerprint 的標籤（BankFingerprintInput）是不同的 hash 空間。
// 平台不再收身分證號與銀行帳號（Stripe 計畫 D22、D38），只有統編一種申請持有的識別值；其他種類 panic。
func IdentityInput(kind SanctionIdentityKind, value string) []byte {
	if kind == IdentityTaxID {
		return []byte("kyc:ubn:v1|" + value)
	}
	panic("seller: unknown identity kind " + string(kind))
}

// BankFingerprintInput 是 provider 回報的外部銀行帳號 fingerprint 的 blind index 輸入：標籤與 IdentityInput 的任何一種都不同（兩者是不同的 hash 空間，
// 不會互相命中），值帶 provider 代碼，不同 provider 的 fingerprint 不會撞。
func BankFingerprintInput(provider, fingerprint string) []byte {
	return []byte("provider:bankfp:v1|" + provider + ":" + fingerprint)
}

// ParseDate 解析 YYYY-MM-DD 的日曆日（UTC 午夜）；格式不合、不存在的日期（2 月 30 日）、早於 1900 年或晚於 now 在
// 台北的今天都拒絕。
func ParseDate(raw string, now time.Time) (time.Time, bool) {
	d, err := time.Parse(time.DateOnly, strings.TrimSpace(raw))
	if err != nil || d.Year() < 1900 {
		return time.Time{}, false
	}
	return d, !d.After(twtime.Day(now))
}

// FormatDate 把日曆日輸出成 YYYY-MM-DD。
func FormatDate(d time.Time) string { return d.Format(time.DateOnly) }

// NormalizeMobile 正規化台灣手機（09 開頭 10 碼）。
func NormalizeMobile(raw string) (string, bool) {
	p := phone.NormalizeToLocal(raw)
	return p, mobileRe.MatchString(p)
}

// KYCFieldNames 回傳申請上可以指出「有問題」的欄位路徑（與對外 KYCApplication 的欄位路徑一對一，個人與企業專屬的欄位在
// individual.、business. 底下）：退回要補的欄位與還缺的欄位都用這一套。
func KYCFieldNames() []string {
	return []string{
		"product_type", "website_url",
		"business.company_name", "business.tax_id", "business.company_register_date",
		"business.company_address", "business.company_phone",
		CompanyDocumentsField,
	}
}

// FieldAppliesTo 回報欄位路徑對賣家型態是否適用：individual. 底下只給個人賣家，business. 底下只給企業賣家，其餘兩者共用。
// 「哪個型態有哪些欄位」只有這個前綴規則一份。
func FieldAppliesTo(field string, t KYCSellerType) bool {
	switch {
	case strings.HasPrefix(field, "individual."):
		return t == KYCIndividual
	case strings.HasPrefix(field, "business."):
		return t == KYCCompany
	}
	return true
}

// ValidCompanyTownCode 檢查是現行的鄉鎮市區代碼。
func ValidCompanyTownCode(code string) bool {
	_, ok := twaddr.TownByCode(code)
	return ok
}

// ErrIdentityNotEligible 是身分比對命中的泛用錯誤：不透露命中的欄位或帳號（避免成為「某身分是否被停權」的探測器）。
func ErrIdentityNotEligible() error {
	return apperr.FailedPrecondition("無法通過認證，請聯絡客服")
}

// NormalizeWebsiteURL 只接受有主機的 http、https 網址，scheme 與主機轉成小寫（與店家的網站網址同一組規則）。
func NormalizeWebsiteURL(raw string) (string, bool) { return websiteURL(strings.TrimSpace(raw)) }

// NormalizeLandline 正規化公司電話：台灣本地格式、9 到 10 碼（市話）。
func NormalizeLandline(raw string) (string, bool) {
	p := phone.NormalizeToLocal(raw)
	return p, phone.Valid(p)
}

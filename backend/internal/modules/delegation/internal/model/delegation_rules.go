package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)


// 代行的參數：常數，不是環境變數，沒有任何部署需要不同的值。TTL 的上限同時是 DB 的 CHECK
// （delegated_operation_sessions_lifetime_check），閒置逾時必須小於 TTL 的上限。
const (
	// DelegationTTL 是 preflight 下發的 TTL，也是 TTL 的上限（DELEGATION_SESSION_TTL_DEFAULT_SECONDS、
	// DELEGATION_SESSION_TTL_MAX_SECONDS）。
	DelegationTTL = 1800 * time.Second
	// DelegationIdleTimeout 是閒置逾時（DELEGATION_SESSION_IDLE_SECONDS；mockup 的「閒置 15 分鐘後自動結束」）。
	DelegationIdleTimeout = 900 * time.Second
	// DelegationHandoffTTL 是 handoff code 的有效期（DELEGATION_HANDOFF_TTL_SECONDS）：只涵蓋一次 303 導向。
	DelegationHandoffTTL = 60 * time.Second
	// DelegationAuditRetentionDays 是畫面顯示的稽核保留天數（DELEGATION_AUDIT_RETENTION_DAYS）。只下發給畫面，
	// 沒有清理：設計書的清理是維運 SQL，系統沒有上線、也沒有會過期的資料（見沿用盤點）。
	DelegationAuditRetentionDays = 730
	delegationMaxBatch = 50
	delegationDefaultPageSize = 20
	delegationMaxPageSize     = 50
)

// Capability 是代行的能力：封閉的十個值，沒有任何履約或金流的能力。值等於 DB 的 CHECK
// （TestCodeValueChecksMatchTheGoTypes）。比對區分大小寫：它是前端 allow-list 的成員。
type Capability string

const (
	CapabilityStorefrontRead  Capability = "storefront.read"
	CapabilityStorefrontWrite Capability = "storefront.write"
	CapabilityProductsRead    Capability = "products.read"
	CapabilityProductsWrite   Capability = "products.write"
	CapabilityProductsPublish Capability = "products.publish"
	CapabilityPostsRead       Capability = "posts.read"
	CapabilityPostsWrite      Capability = "posts.write"
	CapabilityPostsPublish    Capability = "posts.publish"
	CapabilityOrdersRead      Capability = "orders.read"
	// CapabilityMediaUpload 是代行上傳檔案（B1-D16）：上傳只產生會過期的暫存，不讓檔案生效；掛上時仍由擁有者的寫入能力把關。
	CapabilityMediaUpload Capability = "media.upload"
)

// Capabilities 回傳十個能力，前九個順序同設計書 56 §6.1.1，media.upload 附在最後（B1-D16）。
func Capabilities() []Capability {
	return []Capability{
		CapabilityStorefrontRead, CapabilityStorefrontWrite,
		CapabilityProductsRead, CapabilityProductsWrite, CapabilityProductsPublish,
		CapabilityPostsRead, CapabilityPostsWrite, CapabilityPostsPublish,
		CapabilityOrdersRead, CapabilityMediaUpload,
	}
}

func (c Capability) Valid() bool { return slices.Contains(Capabilities(), c) }

// DelegationReason 是發起代行的理由。值是小寫 snake（前端當作不透明的 ID 原樣比對），等於 DB 的 CHECK。
type DelegationReason string

const (
	ReasonSellerRequest       DelegationReason = "SELLER_REQUEST"
	ReasonPlatformMaintenance DelegationReason = "PLATFORM_MAINTENANCE"
	ReasonContentGovernance   DelegationReason = "CONTENT_GOVERNANCE"
)

// DelegationReasonOption 是 preflight 給建立畫面的理由選項。
type DelegationReasonOption struct {
	Code                  DelegationReason
	Label                 string
	CaseReferenceRequired bool
}

var delegationReasons = []DelegationReasonOption{
	{Code: ReasonSellerRequest, Label: "賣家請求（管理員記錄）", CaseReferenceRequired: true},
	{Code: ReasonPlatformMaintenance, Label: "平台維護", CaseReferenceRequired: true},
	{Code: ReasonContentGovernance, Label: "內容治理", CaseReferenceRequired: true},
}

// DelegationReasonOptions 回傳所有理由（複本）。preflight 不允許時也要非空：前端在看 allowed 之前就檢查它。
func DelegationReasonOptions() []DelegationReasonOption { return slices.Clone(delegationReasons) }

func (r DelegationReason) option() (DelegationReasonOption, bool) {
	i := slices.IndexFunc(delegationReasons, func(o DelegationReasonOption) bool { return o.Code == r })
	if i < 0 {
		return DelegationReasonOption{}, false
	}
	return delegationReasons[i], true
}

// Label 是理由的顯示名；不認得的理由是空字串（不回傳原值，免得被當成標籤存進 session）。
func (r DelegationReason) Label() string {
	o, _ := r.option()
	return o.Label
}

// CaseReferenceRequired 回傳這個理由要不要案件編號。不認得的理由要（漏登錄的代價是多一個必填，不是靜默變成選填）。
func (r DelegationReason) CaseReferenceRequired() bool {
	o, ok := r.option()
	return !ok || o.CaseReferenceRequired
}

// AnyCaseReferenceRequired 是 preflight 的 caseReferenceRequired：有沒有任何理由需要案件編號（決定欄位要不要顯示）。
func AnyCaseReferenceRequired() bool {
	return slices.ContainsFunc(delegationReasons, func(o DelegationReasonOption) bool { return o.CaseReferenceRequired })
}

// DelegationStatus 是 session 對外的狀態：三個值的封閉列舉。DB 只有 active、revoked，expired 在讀取時推導。
type DelegationStatus string

const (
	DelegationActive  DelegationStatus = "ACTIVE"
	DelegationExpired DelegationStatus = "EXPIRED"
	DelegationRevoked DelegationStatus = "REVOKED"
)

// 撤銷的理由裡由系統寫入的固定值（設計書 56 §12.4 ①、backend/docs/modules/delegation.md §3）：稽核以這一欄機械地區分撤銷的來源，不是自由文字。
const (
	RevokeReasonDelegationDisabled = "delegation_disabled"
	// RevokeReasonAuthorizationRevoked：撤回店家的授權時連帶撤銷，revoked_by_admin_id 是撤回授權的管理員。
	RevokeReasonAuthorizationRevoked = "authorization_revoked"
	// RevokeReasonAdminRoleRevoked：撤銷管理員的角色（identity 的 AdminRoleRevoked）時連帶撤銷，revoked_by_admin_id 是 NULL：
	// 撤銷角色的 CLI 沒有經過驗證的操作者，誰撤的在 identity 的 admin_role_events。
	RevokeReasonAdminRoleRevoked = "admin_role_revoked"
)

// DelegationDenial 是代行的業務拒絕，對應設計書 56 §6.5.1 的業務碼。模組只說明是哪一種拒絕與給操作者的訊息，
// 對外的碼由傳輸層決定（過渡期是 internal/legacy/delegationservice，以舊的業務碼回應 gateway）。
type DelegationDenial int

const (
	_ DelegationDenial = iota
	// DenialInvalidCommand：參數缺少、格式或值域不對（9260）。
	DenialInvalidCommand
	// DenialAdminRequired：沒有管理員的身分（9261）。
	DenialAdminRequired
	// DenialCapabilityDenied：要求的能力不在授權或 session 的範圍內（9262）。
	DenialCapabilityDenied
	// DenialAuthorizationRevoked：店家的授權存在但已撤回或已到期（9264）。
	DenialAuthorizationRevoked
	// DenialStoreNotDelegable：店家不存在（9265）。
	DenialStoreNotDelegable
	// DenialIdempotencyKeyReused：同一把冪等鍵、不同的內容，或 credential 已無法重建（9267）。
	DenialIdempotencyKeyReused
	// DenialActorBusy：這位管理員已經在代行另一家店（9268）。
	DenialActorBusy
	// DenialStoreBusy：這家店已經有其他管理員在代行（9269）。
	DenialStoreBusy
	// DenialSessionExpired：session 已到期，hard 與閒置共用（9270）。
	DenialSessionExpired
	// DenialSessionRevoked：session 已撤銷（9271）。
	DenialSessionRevoked
	// DenialStoreNotOperable：店家等待刪除或帳號停權（9272）；已關閉是 DenialStoreClosed。
	DenialStoreNotOperable
	// DenialCredentialInvalid：credential 缺少或對不到 session（9274）。
	DenialCredentialInvalid
	// DenialSessionNotFound：session 不存在（9277）。
	DenialSessionNotFound
	// DenialAuditIntentFailed：稽核列不存在，或排程找不到可繼承的理由（9279）。
	DenialAuditIntentFailed
	// DenialHandoffInvalid：handoff code 無效，四種成因共用（9282）。
	DenialHandoffInvalid
	// DenialAuthorizationAlreadyActive：店家已經有有效的授權（9285）。
	DenialAuthorizationAlreadyActive
	// DenialAuthorizationNotFound：店家從來沒有開通（9286）。
	DenialAuthorizationNotFound
	// DenialAdminNotAllowlisted：管理員沒有開通代行（9288）。
	DenialAdminNotAllowlisted
	// DenialStoreClosed：店家已關閉（舊碼對應 9265，只在 legacy 轉接層）。店家存在，只是已關閉；句子由 seller 的 StoreActionDelegate 原因給。
	DenialStoreClosed
)

func (d DelegationDenial) newError(format string, args ...any) *apperr.Error {
	switch d {
	case DenialInvalidCommand:
		return apperr.InvalidInput(format, args...)
	case DenialAdminRequired, DenialCredentialInvalid, DenialHandoffInvalid:
		return apperr.Unauthenticated(format, args...)
	case DenialCapabilityDenied, DenialAdminNotAllowlisted:
		return apperr.PermissionDenied(format, args...)
	case DenialSessionNotFound, DenialAuthorizationNotFound:
		return apperr.NotFound(format, args...)
	case DenialAuthorizationAlreadyActive:
		return apperr.AlreadyExists(format, args...)
	case DenialAuthorizationRevoked, DenialStoreNotDelegable, DenialStoreClosed, DenialIdempotencyKeyReused,
		DenialActorBusy, DenialStoreBusy, DenialSessionExpired, DenialSessionRevoked,
		DenialStoreNotOperable, DenialAuditIntentFailed:
		return apperr.FailedPrecondition(format, args...)
	default:
		return apperr.FailedPrecondition(format, args...)
	}
}

// DelegationError 是代行的業務拒絕。錯誤鏈上是 apperr.Error（分類與給操作者的訊息），apperr.KindOf 照常可用。
type DelegationError struct {
	Denial DelegationDenial
	err    *apperr.Error
}

func (e *DelegationError) Error() string { return e.err.Error() }
func (e *DelegationError) Unwrap() error { return e.err }

// Message 是給操作者看的訊息。
func (e *DelegationError) Message() string { return e.err.Message }

// DelegationDenialOf 回傳 err 鏈上的代行拒絕；沒有時 ok 是 false（其他錯誤都是內部錯誤）。
func DelegationDenialOf(err error) (*DelegationError, bool) {
	var e *DelegationError
	ok := errors.As(err, &e)
	return e, ok
}

func Deny(d DelegationDenial, format string, args ...any) error {
	return &DelegationError{Denial: d, err: d.newError(format, args...)}
}

// DeniedLabel 是 preflight 拒絕時給操作者的那一句（每一種拒絕只有一個來源）。
func DeniedLabel(d DelegationDenial) string {
	switch d {
	case DenialStoreNotDelegable:
		return "此店家目前不可代行"
	case DenialAdminNotAllowlisted:
		return "此管理員尚未開通代行營運，請洽有治理權限的管理員"
	case DenialAuthorizationNotFound:
		return "此店家尚未開通代行營運"
	case DenialAuthorizationRevoked:
		return "此店家的代行營運授權已撤回或已到期，請重新開通"
	case DenialActorBusy:
		return "您已有進行中的代行工作階段，請先結束"
	case DenialStoreBusy:
		return "此店家已有其他管理員正在代行"
	case DenialInvalidCommand, DenialAdminRequired, DenialCapabilityDenied,
		DenialIdempotencyKeyReused, DenialSessionExpired, DenialSessionRevoked,
		DenialCredentialInvalid, DenialSessionNotFound, DenialAuditIntentFailed,
		DenialHandoffInvalid, DenialAuthorizationAlreadyActive,
		DenialStoreNotOperable, DenialStoreClosed:
		return "目前不可代行"
	default:
		return "目前不可代行"
	}
}

// StoreState 是店家此刻的狀態，由模組根套件向 seller 的公開 API 讀取（依賴表 internal/arch/modules.go 允許 delegation 依賴 seller）。
type StoreState struct {
	// Found 為 false 表示店家不存在；已關閉的店家存在（Found 為 true、Closed 為 true、Operable 為 false）。
	Found bool
	// StoreID 是店家的 ID（seller 回傳的值）。
	StoreID         int64
	SellerAccountID int64
	Name            string
	// Operable 表示店家此刻可以營運：seller 授予店面寫入能力（Store.Can(CapStorefrontWrite)）；停權與等待刪除都是 false（Deny-by-default）。
	Operable bool
	// Closed 表示店家已關閉（Operable 為 false 時用來選 DenialStoreClosed 或 DenialStoreNotOperable）。
	Closed bool
	// UnavailableText 是不可營運時給操作者的那一句：seller 的 StoreActionDelegate 第一個原因的句子（seller.StoreActionUnavailableText）。Operable 為 true 時是空的。
	UnavailableText string
}

// DelegationPaging 是分頁的輸入：Page 從 1 開始，零值是第 1 頁、每頁 20 筆。
type DelegationPaging struct {
	Page     int32
	PageSize int32
}

// Normalize 套用預設值並檢查範圍；超出範圍回拒絕而不是夾住（前端以回應的 pageSize 算頁數）。
func (p DelegationPaging) Normalize() (DelegationPaging, error) {
	if p.Page == 0 {
		p.Page = 1
	}
	if p.PageSize == 0 {
		p.PageSize = delegationDefaultPageSize
	}
	if p.Page < 1 || p.PageSize < 1 || p.PageSize > delegationMaxPageSize {
		return p, Deny(DenialInvalidCommand, "分頁參數超出範圍")
	}
	return p, nil
}

func (p DelegationPaging) Offset() int32 { return (p.Page - 1) * p.PageSize }

// ValidateCaseReference 是案件編號的規則（設計書 56 §6.1.3）：沒有是合法的；有的話 4–200 字、沒有控制字元、至少
// 一個數字。刻意不做格式驗證（格式只證明長得像，證明不了它指向任何東西）。DB 的 CHECK 守值域，這裡給訊息。
func ValidateCaseReference(ref *string) error {
	if ref == nil {
		return nil
	}
	if n := utf8.RuneCountInString(*ref); n < 4 || n > 200 {
		return Deny(DenialInvalidCommand, "caseReference 長度必須介於 4 與 200 之間")
	}
	hasDigit := false
	for _, r := range *ref {
		if r < 0x20 || r == 0x7f {
			return Deny(DenialInvalidCommand, "caseReference 不得含控制字元")
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	if !hasDigit {
		return Deny(DenialInvalidCommand, "caseReference 至少需含一個數字")
	}
	return nil
}

// ValidateCapabilities 檢查能力的值域：非空、認得、不重複，最多 max 個。
func ValidateCapabilities(field string, caps []Capability) error {
	if len(caps) == 0 {
		return Deny(DenialInvalidCommand, "%s 不得為空", field)
	}
	if len(caps) > len(Capabilities()) {
		return Deny(DenialInvalidCommand, "%s 超出可授權的能力數量", field)
	}
	seen := make(map[Capability]bool, len(caps))
	for _, c := range caps {
		if !c.Valid() {
			return Deny(DenialInvalidCommand, "%s 含未知的能力值", field)
		}
		if seen[c] {
			return Deny(DenialInvalidCommand, "%s 不得重複", field)
		}
		seen[c] = true
	}
	return nil
}

func ToCapabilities(s []string) []Capability {
	out := make([]Capability, len(s))
	for i, c := range s {
		out[i] = Capability(c)
	}
	return out
}

func FromCapabilities(c []Capability) []string {
	out := make([]string, len(c))
	for i, v := range c {
		out[i] = string(v)
	}
	return out
}

// SameCapabilitySet 比較兩組能力（集合相等，順序不是契約的一部分）。
func SameCapabilitySet(a, b []Capability) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}


const (
	HandoffPrefix    = "dhof_"
	CredentialPrefix = "dops_"
	tokenLength = 48
)

// NewHandoffCode 產生一次性的 handoff code。
func NewHandoffCode() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return HandoffPrefix + base64.RawURLEncoding.EncodeToString(b)
}

// NewOperationCredential 產生 256-bit 的一次性隨機憑證。
func NewOperationCredential() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return CredentialPrefix + base64.RawURLEncoding.EncodeToString(b)
}

// TokenHash 是 credential 與 handoff code 唯一能進 DB 的形式。
func TokenHash(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// WellFormedToken 回傳 s 是不是帶 prefix 的 48 字 token：只是便宜的預先過濾（兌換的端點不必為每個亂試的值查表），
// 不是授權。
func WellFormedToken(s, prefix string) bool {
	if len(s) != tokenLength || s[:len(prefix)] != prefix {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(s[len(prefix):])
	return err == nil
}

const displayCodeAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func NewDisplayCode() string {
	out := make([]byte, 0, 17)
	out = append(out, "OPS-"...)
	var buf [32]byte
	for len(out) < cap(out) {
		_, _ = rand.Read(buf[:])
		for _, b := range buf {
			if b < 252 && len(out) < cap(out) {
				out = append(out, displayCodeAlphabet[b%36])
			}
		}
	}
	return string(out)
}

var denialBizCodes = map[DelegationDenial]int32{
	DenialInvalidCommand:             9260,
	DenialAdminRequired:              9261,
	DenialCapabilityDenied:           9262,
	DenialAuthorizationRevoked:       9264,
	DenialStoreNotDelegable:          9265,
	DenialStoreClosed:                9265,
	DenialIdempotencyKeyReused:       9267,
	DenialActorBusy:                  9268,
	DenialStoreBusy:                  9269,
	DenialSessionExpired:             9270,
	DenialSessionRevoked:             9271,
	DenialStoreNotOperable:           9272,
	DenialCredentialInvalid:          9274,
	DenialSessionNotFound:            9277,
	DenialAuditIntentFailed:          9279,
	DenialHandoffInvalid:             9282,
	DenialAuthorizationAlreadyActive: 9285,
	DenialAuthorizationNotFound:      9286,
	DenialAdminNotAllowlisted:        9288,
}

// DenialBizCodes 回傳拒絕對應的業務碼表的副本。
func DenialBizCodes() map[DelegationDenial]int32 { return maps.Clone(denialBizCodes) }

var denialCodes = map[DelegationDenial]string{ //nolint:gosec // G101 false positive: CREDENTIAL_INVALID is a denial code name, not a secret
	DenialInvalidCommand:             "INVALID_COMMAND",
	DenialAdminRequired:              "ADMIN_REQUIRED",
	DenialCapabilityDenied:           "CAPABILITY_DENIED",
	DenialAuthorizationRevoked:       "AUTHORIZATION_REVOKED",
	DenialStoreNotDelegable:          "STORE_NOT_DELEGABLE",
	DenialStoreClosed:                "STORE_CLOSED",
	DenialIdempotencyKeyReused:       "IDEMPOTENCY_KEY_REUSED",
	DenialActorBusy:                  "ACTOR_BUSY",
	DenialStoreBusy:                  "STORE_BUSY",
	DenialSessionExpired:             "SESSION_EXPIRED",
	DenialSessionRevoked:             "SESSION_REVOKED",
	DenialStoreNotOperable:           "STORE_NOT_OPERABLE",
	DenialCredentialInvalid:          "CREDENTIAL_INVALID",
	DenialSessionNotFound:            "SESSION_NOT_FOUND",
	DenialAuditIntentFailed:          "AUDIT_INTENT_FAILED",
	DenialHandoffInvalid:             "HANDOFF_INVALID",
	DenialAuthorizationAlreadyActive: "AUTHORIZATION_ALREADY_ACTIVE",
	DenialAuthorizationNotFound:      "AUTHORIZATION_NOT_FOUND",
	DenialAdminNotAllowlisted:        "ADMIN_NOT_ALLOWLISTED",
}

// FailureReasonInternal 是沒有 ErrorReason 也不是代行拒絕的失敗（內部錯誤、沒有業務原因的分類錯誤）寫進 failure_reason 的明確值。
const FailureReasonInternal = "INTERNAL"

// Code 是拒絕寫進稽核列的代碼；沒有登錄的值回空字串。
func (d DelegationDenial) Code() string { return denialCodes[d] }

// DenialCodes 回傳拒絕對應的代碼表的副本。
func DenialCodes() map[DelegationDenial]string { return maps.Clone(denialCodes) }

// DenialOfCode 由 failure_reason 的代碼找回拒絕；不是拒絕的代碼（ErrorReason、INTERNAL）回 false。
func DenialOfCode(code string) (DelegationDenial, bool) {
	for d, c := range denialCodes {
		if c == code {
			return d, true
		}
	}
	return 0, false
}

var failureReasonFormat = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)

// LegalFailureReason 回傳 s 是否是合法的 failure_reason：與 DB CHECK（admin_operation_audit_logs_failure_reason_check）同一條規則，UPPER_SNAKE 且不是 UNSPECIFIED。
func LegalFailureReason(s string) bool {
	return failureReasonFormat.MatchString(s) && s != "UNSPECIFIED"
}

// FailureReasonOf 是 err 鏈上的失敗原因：代行拒絕是拒絕的代碼，其次是 apperr.Error 的 ErrorReason，沒有 ErrorReason 的分類錯誤取分類的大寫名稱
// （NOT_FOUND、INVALID_INPUT、FAILED_PRECONDITION…），其餘（內部錯誤、沒有分類的錯誤）是 FailureReasonInternal。
func FailureReasonOf(err error) string {
	if d, ok := DelegationDenialOf(err); ok {
		if c := d.Denial.Code(); c != "" {
			return c
		}
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		if r := ae.Reason.String(); r != "" {
			return r
		}
		if k := apperr.KindOf(err); k != apperr.KindInternal {
			return strings.ToUpper(k.String())
		}
	}
	return FailureReasonInternal
}

// CheckSession 檢查代行 session 此刻還能用（ACTIVE；到期與已撤銷分別是 DenialSessionExpired、DenialSessionRevoked）並帶有能力 c（DenialCapabilityDenied）。
// 傳輸層的 interceptor 與同 transaction 的稽核寫入都呼叫它（兩處對的是不同時間點讀到的 session）。
func CheckSession(s DelegationSession, c Capability) error {
	switch s.Status {
	case DelegationRevoked:
		return Deny(DenialSessionRevoked, "代行工作階段已被撤銷")
	case DelegationExpired:
		return Deny(DenialSessionExpired, "代行工作階段已逾時")
	case DelegationActive:
	}
	if !slices.Contains(s.Capabilities, c) {
		return Deny(DenialCapabilityDenied, "此工作階段未取得執行該操作的能力")
	}
	return nil
}

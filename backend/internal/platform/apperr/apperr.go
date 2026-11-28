// Package apperr 是模組回報給呼叫方的錯誤分類。
//
// 模組只說明「是哪一類錯誤」與「給使用者看的訊息」，不知道傳輸層：對外的狀態碼
// （HTTP status、Connect code）由 httpapi 依 Kind 決定。沒有分類的錯誤一律視為
// 內部錯誤，訊息不回給使用者。
//
// 分類只在第一個需要它的模組搬進來時新增，不預先列出用不到的種類。
package apperr

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Kind 是錯誤的分類。
type Kind int

const (
	// KindInternal 是零值：沒有分類的錯誤都屬於這一類。
	KindInternal Kind = iota
	// KindInvalidInput 表示呼叫方給的值不合法，改了輸入就能成功。
	KindInvalidInput
	// KindNotFound 表示要找的東西不存在。
	KindNotFound
	// KindVersionConflict 表示呼叫方依據的版本已經過時：修改時帶的版本與目前的不符
	// （ADR 2026-09-25-optimistic-concurrency-etag D2）。重新讀取後才能再送。
	KindVersionConflict
	// KindAlreadyExists 表示要建立的東西已經存在，例如以 key 建立時那個 key 已經有人建立（ADR 的 D3）。
	KindAlreadyExists
	// KindFailedPrecondition 表示目前的狀態不允許這個動作，輸入本身沒有錯，改變狀態之後才能成功（例如數量已達
	// 上限，要先移除其他的）。與 KindVersionConflict 不同：重讀最新版本也解決不了。
	KindFailedPrecondition
	// KindUnauthenticated 表示呼叫方沒有有效的憑證：沒有登入、session 已失效，或登入用的 token 無效。
	// 重新登入才能成功。
	KindUnauthenticated
	// KindPermissionDenied 表示呼叫方的身分不允許這個動作，例如帳號已停權。
	KindPermissionDenied
	// KindUnavailable 表示依賴的外部服務或功能暫時不能用（例如沒有設定、或 provider 沒有回應），稍後可能恢復。
	KindUnavailable
	// KindResourceExhausted 表示超過次數或頻率的上限；RetryAfter 大於零時，是多久之後可以再試。
	KindResourceExhausted
)

func (k Kind) String() string {
	switch k {
	case KindInvalidInput:
		return "invalid_input"
	case KindNotFound:
		return "not_found"
	case KindVersionConflict:
		return "version_conflict"
	case KindAlreadyExists:
		return "already_exists"
	case KindFailedPrecondition:
		return "failed_precondition"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindPermissionDenied:
		return "permission_denied"
	case KindUnavailable:
		return "unavailable"
	case KindResourceExhausted:
		return "resource_exhausted"
	case KindInternal:
		return "internal"
	default:
		return "internal"
	}
}

// Reason 是同一個分類底下的業務原因，對應對外契約的 hawkr.common.v1alpha1.ErrorReason
// （ADR 2026-09-25-api-error-model）。只為 client 會分支的情境定義。wire 上只有一個列舉，這裡也只有
// 一份：Reason 只能在本套件以 newReason 宣告，Reasons 列出全部，internal/httpapi/rpc 的測試與 proto
// 雙向比對。零值表示沒有業務原因。
type Reason struct{ name string }

// String 回傳 wire 上的 reason，也就是 proto 的值名。
func (r Reason) String() string { return r.name }

var reasons []Reason

func newReason(name string) Reason {
	r := Reason{name}
	reasons = append(reasons, r)
	return r
}

// Reasons 回傳本套件宣告的所有 Reason。
func Reasons() []Reason { return append([]Reason(nil), reasons...) }

// ReasonVersionConflict 表示依據的版本已經過時：修改時帶的版本與目前的不符，或以 key 建立時
// 那筆已經存在（ADR 2026-09-25-optimistic-concurrency-etag D2、D3）。
var ReasonVersionConflict = newReason("VERSION_CONFLICT")

// ReasonSellerSuspended 表示賣家帳號停權中，要求的操作不在停權時仍可做的範圍；搭配 PermissionDenied。
var ReasonSellerSuspended = newReason("SELLER_SUSPENDED")

// ReasonProspectMatchFound 表示商家名單的聯絡方式與其他未封存（進行中或已入駐）的業者相同、而且沒有標記為不是同一家；搭配 FailedPrecondition。
// 相符的業者以 metadata 的 `prospectIds` 傳給 client（十進位 ID，由小到大以逗號連接）。
var ReasonProspectMatchFound = newReason("PROSPECT_MATCH_FOUND")

// ReasonProspectTaxIDHeld 表示商家名單的統一編號已由另一筆未封存（進行中或已入駐）的業者持有；建立搭配 AlreadyExists，修改與還原搭配
// FailedPrecondition。持有者以 metadata 的 `prospectIds` 傳給 client（十進位 ID，只有一筆）。
var ReasonProspectTaxIDHeld = newReason("PROSPECT_TAX_ID_HELD")

// ReasonAgreementOutdated 表示賣家同意的入駐文件版本不是現行版本；搭配 FailedPrecondition。client 重讀 GetSellerAgreements，
// 請賣家重新勾選。
var ReasonAgreementOutdated = newReason("AGREEMENT_OUTDATED")

// ReasonSellerRequirementMissing 表示賣家做這個操作需要的前置條件沒有滿足（身分認證未核准、店家正在隨帳號刪除）；搭配
// FailedPrecondition。停權不用它（SELLER_SUSPENDED）。缺的條件以 metadata 的 `missingRequirements` 傳給 client
// （SellerRequirement 的值名，不加前綴，依列舉的數字由小到大以逗號連接）。
var ReasonSellerRequirementMissing = newReason("SELLER_REQUIREMENT_MISSING")

// ReasonStoreOpeningSuspended 表示帳號停權中，不能申請開店；搭配 FailedPrecondition。帳號同時有生效中的終止合作時用 ReasonStoreOpeningTerminated。
var ReasonStoreOpeningSuspended = newReason("STORE_OPENING_SUSPENDED")

// ReasonStoreOpeningTerminated 表示帳號有生效中的終止合作（條款第 21.3 條），終止撤銷之前不能申請開店；搭配 FailedPrecondition。
var ReasonStoreOpeningTerminated = newReason("STORE_OPENING_TERMINATED")

// ReasonAccountHasStore 表示帳號擁有商店，不能被停權；搭配 FailedPrecondition。
var ReasonAccountHasStore = newReason("ACCOUNT_HAS_STORE")

// ReasonAccountSuspended 表示帳號停權中，這個操作是寫入，不在停權時仍可做的範圍；搭配 PermissionDenied。只由入口依動作的性質判斷。
var ReasonAccountSuspended = newReason("ACCOUNT_SUSPENDED")

// ReasonIdentityAlreadyLinked 表示要連結的登入方式（provider 的身分）已經連結到另一個帳號，包含已刪除帳號留下的身分；搭配 AlreadyExists。
var ReasonIdentityAlreadyLinked = newReason("IDENTITY_ALREADY_LINKED")

// ReasonReauthenticationRequired 表示這個操作要求最近重新驗證過，目前的 session 沒有；搭配 FailedPrecondition。
var ReasonReauthenticationRequired = newReason("REAUTHENTICATION_REQUIRED")

// ReasonContactEmailRequired 表示帳號沒有聯絡信箱（沒有已驗證的 email，也沒有自填的），不能送出 KYC 申請；搭配 FailedPrecondition。
var ReasonContactEmailRequired = newReason("CONTACT_EMAIL_REQUIRED")

// ReasonContentTakedownActive 表示要下架的內容已經有生效中的內容下架措施；搭配 FailedPrecondition。
var ReasonContentTakedownActive = newReason("CONTENT_TAKEDOWN_ACTIVE")

// ReasonReviewAlreadyExists 表示這個訂單項目已經有評價；搭配 AlreadyExists。
var ReasonReviewAlreadyExists = newReason("REVIEW_ALREADY_EXISTS")

// ReasonOrderNotReviewable 表示訂單目前不能評價（只有已完成的訂單能評價）；搭配 FailedPrecondition。
var ReasonOrderNotReviewable = newReason("ORDER_NOT_REVIEWABLE")

// ReasonReviewWindowExpired 表示訂單已超過評價期限；搭配 FailedPrecondition。
var ReasonReviewWindowExpired = newReason("REVIEW_WINDOW_EXPIRED")

// ReasonReviewNotReplyable 表示這則評價目前不能回覆（被隱藏、作者已離開、回覆被下架）；搭配 FailedPrecondition。
var ReasonReviewNotReplyable = newReason("REVIEW_NOT_REPLYABLE")

// ReasonSelfMessageNotAllowed 表示買家帳號與店家帳號是同一個帳號，不能傳訊息給自己；搭配 InvalidArgument。
var ReasonSelfMessageNotAllowed = newReason("SELF_MESSAGE_NOT_ALLOWED")

// ReasonStoreClosed 表示店家已停業，無法傳送訊息；搭配 FailedPrecondition。
var ReasonStoreClosed = newReason("STORE_CLOSED")

// ReasonChatBlocked 表示買家與店家之間任一方向有封鎖且沒有進行中的訂單或售後，無法傳送訊息；搭配 FailedPrecondition。
var ReasonChatBlocked = newReason("CHAT_BLOCKED")

// ReasonReceiverDeleted 表示收件人（買家或店家）的帳號已刪除，無法傳送訊息；搭配 FailedPrecondition。
var ReasonReceiverDeleted = newReason("RECEIVER_DELETED")

// ReasonCrmTagNameTaken 表示同店已有同名標籤；搭配 AlreadyExists。
var ReasonCrmTagNameTaken = newReason("CRM_TAG_NAME_TAKEN")

// ReasonCrmTagLimit 表示店家或客戶的標籤數已達上限；搭配 FailedPrecondition。
var ReasonCrmTagLimit = newReason("CRM_TAG_LIMIT")

// ReasonReplayCursorExpired 表示串流（助理與採購的 server streaming）的 cursor 早於保留的事件範圍；搭配 FailedPrecondition。
var ReasonReplayCursorExpired = newReason("REPLAY_CURSOR_EXPIRED")

// ReasonReplayCursorAhead 表示串流的 cursor 超前於最新的事件；搭配 FailedPrecondition。
var ReasonReplayCursorAhead = newReason("REPLAY_CURSOR_AHEAD")

// ReasonRequirementNotReady 表示採購需求的必填欄位還沒填滿，不能搜尋候選；搭配 FailedPrecondition。
var ReasonRequirementNotReady = newReason("REQUIREMENT_NOT_READY")

// ReasonApprovalPending 表示採購需求有待回覆的中斷，回答與倒回在這期間不能寫入；搭配 FailedPrecondition。
var ReasonApprovalPending = newReason("APPROVAL_PENDING")

// ReasonApprovalAlreadyResolved 表示要回覆的中斷已經被回覆過；搭配 FailedPrecondition。
var ReasonApprovalAlreadyResolved = newReason("APPROVAL_ALREADY_RESOLVED")

// ReasonRunNotActive 表示 run 已不在進行中，不能對它下需要進行中的命令；搭配 FailedPrecondition。
var ReasonRunNotActive = newReason("RUN_NOT_ACTIVE")

// ReasonRfqResponseExpired 表示被接受的賣家回價版本已過有效期；搭配 FailedPrecondition。
var ReasonRfqResponseExpired = newReason("RFQ_RESPONSE_EXPIRED")

// ReasonRfqRecipientClosed 表示賣家對這筆詢價已經不能再寫入（已婉拒、這張需求有進行中的成交、買家已刪除，或回價期限已過）；搭配 FailedPrecondition。
var ReasonRfqRecipientClosed = newReason("RFQ_RECIPIENT_CLOSED")

// ReasonRfqCheckoutAlreadyActive 表示同一張採購需求已有進行中的成交；搭配 FailedPrecondition，Metadata 的 orderNo 是既有訂單。
var ReasonRfqCheckoutAlreadyActive = newReason("RFQ_CHECKOUT_ALREADY_ACTIVE")

// ReasonAgentMonthlyQuotaExceeded 表示本期的採購額度已用完；搭配 ResourceExhausted，Metadata 的 remaining、resetTime 說明剩餘與重置時間。
var ReasonAgentMonthlyQuotaExceeded = newReason("AGENT_MONTHLY_QUOTA_EXCEEDED")

// ReasonImageQuotaExceeded 表示本週的 AI 修圖額度已用完；搭配 ResourceExhausted，Metadata 的 remaining、resetTime 說明剩餘與重置時間。
var ReasonImageQuotaExceeded = newReason("IMAGE_QUOTA_EXCEEDED")

// ReasonImageSafetyBlocked 表示圖片或提示詞被安全性檢查擋下，重送結果相同；搭配 InvalidArgument。
var ReasonImageSafetyBlocked = newReason("IMAGE_SAFETY_BLOCKED")

// ReasonImageRecitationBlocked 表示圖片涉及著作權複誦而被擋下，重送結果相同；搭配 InvalidArgument。
var ReasonImageRecitationBlocked = newReason("IMAGE_RECITATION_BLOCKED")

// ReasonImageNotProduced 表示模型沒有產出圖片，可以重試；搭配 Unavailable。
var ReasonImageNotProduced = newReason("IMAGE_NOT_PRODUCED")

// ReasonProcurementRunInProgress 表示這個採購需求已有進行中的 run，同一個需求同時只允許一個；搭配 FailedPrecondition。
var ReasonProcurementRunInProgress = newReason("PROCUREMENT_RUN_IN_PROGRESS")

// ReasonAssistantTurnInProgress 表示這個助理對話已有一輪進行中，同一個對話同時只允許一輪；搭配 FailedPrecondition。
var ReasonAssistantTurnInProgress = newReason("ASSISTANT_TURN_IN_PROGRESS")

// ReasonAssistantTurnFailed 表示助理這一輪失敗，用於串流 error 事件的原因（事件在串流內，沒有搭配的 Connect code）。
var ReasonAssistantTurnFailed = newReason("ASSISTANT_TURN_FAILED")

// ReasonCouponCampaignNotActive 表示優惠券活動不是進行中（尚未發布、已被結束）；搭配 FailedPrecondition。
var ReasonCouponCampaignNotActive = newReason("COUPON_CAMPAIGN_NOT_ACTIVE")

// ReasonCouponOutsideWindow 表示現在不在優惠券活動的時間窗內；搭配 FailedPrecondition。
var ReasonCouponOutsideWindow = newReason("COUPON_OUTSIDE_WINDOW")

// ReasonCouponNotEligible 表示買家不符合優惠券活動的資格；搭配 FailedPrecondition。
var ReasonCouponNotEligible = newReason("COUPON_NOT_ELIGIBLE")

// ReasonCouponStoreSuspended 表示發行優惠券活動的店家停權中；搭配 FailedPrecondition。
var ReasonCouponStoreSuspended = newReason("COUPON_STORE_SUSPENDED")

// ReasonCouponIssueLimitReached 表示優惠券活動已經領完；搭配 FailedPrecondition。
var ReasonCouponIssueLimitReached = newReason("COUPON_ISSUE_LIMIT_REACHED")

// ReasonCouponQuotaExceeded 表示店家同時進行的優惠券活動數已達上限；搭配 FailedPrecondition。
var ReasonCouponQuotaExceeded = newReason("COUPON_QUOTA_EXCEEDED")

// ReasonRestockSKUInStock 表示規格（SKU）目前有貨，不接受補貨訂閱；搭配 FailedPrecondition。
var ReasonRestockSKUInStock = newReason("RESTOCK_SKU_IN_STOCK")

var ReasonProductNotPublished = newReason("PRODUCT_NOT_PUBLISHED")

var ReasonProductNotPurchasable = newReason("PRODUCT_NOT_PURCHASABLE")

// FieldViolation 是一個輸入欄位的錯誤。Field 是模組輸入的欄位名（Go 的欄位名，巢狀以 . 連接、
// 索引寫成 [n]），例如 Title、Action.Target、Ingredients[2].Name；對應到請求的路徑是傳輸層的事
// （ADR 2026-09-25-api-error-model D4）。Message 給使用者看。
type FieldViolation struct {
	Field   string
	Message string
}

// IndexedField 是清單欄位 field 的第 i 個元素的欄位名（"Docs[2]"）：單一元素的問題指到那一個，整個清單的問題（張數上限）用 field。
// 模組裡組元素欄位名只用這一個函式。
func IndexedField(field string, i int) string { return fmt.Sprintf("%s[%d]", field, i) }

// Error 是帶分類的錯誤。Message 會回給使用者，不得包含內部細節。Reason 是同一個分類底下的業務原因，
// client 需要區分業務情境時才設。欄位錯誤（FieldViolations）只能由 InvalidInputOn、InvalidInputFields 在建立時帶，傳輸層放進錯誤的 detail。RetryAfter 只用於
// KindResourceExhausted：多久之後可以再試，傳輸層放進 RetryInfo。Metadata 是 client 依 Reason 分支時需要的動態值，傳輸層放進
// ErrorInfo.metadata；沒有 Reason 時不會送出（metadata 屬於 ErrorInfo，而 ErrorInfo 只在有 Reason 時帶）。
type Error struct {
	kind       Kind
	Message    string
	Reason     Reason
	violations []FieldViolation
	RetryAfter time.Duration
	Metadata   map[string]string
	locale     uitext.Locale
}

// Locale 是 Message 的語系：建構時有格式參數是 uitext.Message（模組句子，語系是實際使用的語系）就取它的 Locale，
// 否則是 zh-TW（字面值都是 zh-TW）。
func (e *Error) Locale() uitext.Locale {
	if e.locale == "" {
		return uitext.ZhTW
	}
	return e.locale
}

func newError(kind Kind, format string, args []any) *Error {
	e := &Error{kind: kind, Message: fmt.Sprintf(format, args...)}
	for _, a := range args {
		if m, ok := a.(uitext.Message); ok {
			e.locale = m.Locale
			break
		}
	}
	return e
}

// WithReason 設定業務原因並回傳同一個錯誤。
func (e *Error) WithReason(r Reason) *Error {
	e.Reason = r
	return e
}

// WithMetadata 加上一個 ErrorInfo.metadata 的值並回傳同一個錯誤；key 依 AIP-193 用 lowerCamelCase。
func (e *Error) WithMetadata(key, value string) *Error {
	if e.Metadata == nil {
		e.Metadata = map[string]string{}
	}
	e.Metadata[key] = value
	return e
}

// FieldViolations 回傳錯誤指出的欄位（複本）；只有 InvalidInputOn、InvalidInputFields 建立的錯誤會有。
func (e *Error) FieldViolations() []FieldViolation { return slices.Clone(e.violations) }

// Kind 回傳錯誤的分類。分類在建立時決定，之後不能改（欄位未匯出），所以「只有 invalid_argument 帶欄位」由建構保證。
func (e *Error) Kind() Kind { return e.kind }

func (e *Error) Error() string { return e.kind.String() + ": " + e.Message }

// InvalidInput 回傳 KindInvalidInput 的錯誤，不指出欄位。
func InvalidInput(format string, args ...any) *Error {
	return newError(KindInvalidInput, format, args)
}

// InvalidInputOn 回傳 KindInvalidInput 的錯誤，指出出錯的欄位，每個欄位的訊息都是整體的 Message。欄位只屬於
// invalid_argument（API 設計規範「八、錯誤」的 BadRequest），而且只能在建立時給：*Error 沒有事後加欄位的方法，
// 其他分類因此不可能帶欄位。
func InvalidInputOn(fields []string, format string, args ...any) *Error {
	e := InvalidInput(format, args...)
	for _, f := range fields {
		e.violations = append(e.violations, FieldViolation{Field: f, Message: e.Message})
	}
	return e
}

// InvalidInputFields 回傳 KindInvalidInput 的錯誤，每個欄位有自己的訊息（與整體的 Message 不同時用）。
// 限制同 InvalidInputOn。
func InvalidInputFields(violations []FieldViolation, format string, args ...any) *Error {
	e := InvalidInput(format, args...)
	e.violations = slices.Clone(violations)
	return e
}

// NotFound 回傳 KindNotFound 的錯誤。
func NotFound(format string, args ...any) *Error {
	return newError(KindNotFound, format, args)
}

// VersionConflict 回傳 KindVersionConflict 的錯誤，業務原因是 VERSION_CONFLICT。
func VersionConflict(format string, args ...any) *Error {
	return newError(KindVersionConflict, format, args).WithReason(ReasonVersionConflict)
}

// AlreadyExists 回傳 KindAlreadyExists 的錯誤。
func AlreadyExists(format string, args ...any) *Error {
	return newError(KindAlreadyExists, format, args)
}

// FailedPrecondition 回傳 KindFailedPrecondition 的錯誤。
func FailedPrecondition(format string, args ...any) *Error {
	return newError(KindFailedPrecondition, format, args)
}

// Unauthenticated 回傳 KindUnauthenticated 的錯誤。
func Unauthenticated(format string, args ...any) *Error {
	return newError(KindUnauthenticated, format, args)
}

// PermissionDenied 回傳 KindPermissionDenied 的錯誤。
func PermissionDenied(format string, args ...any) *Error {
	return newError(KindPermissionDenied, format, args)
}

// Unavailable 回傳 KindUnavailable 的錯誤。
func Unavailable(format string, args ...any) *Error {
	return newError(KindUnavailable, format, args)
}

// ResourceExhausted 回傳 KindResourceExhausted 的錯誤；retryAfter 是多久之後可以再試，不知道時給 0。
func ResourceExhausted(retryAfter time.Duration, format string, args ...any) *Error {
	e := newError(KindResourceExhausted, format, args)
	e.RetryAfter = retryAfter
	return e
}

// KindOf 回傳 err 鏈上第一個 *Error 的分類；沒有時回傳 KindInternal。
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.kind
	}
	return KindInternal
}

// WithCause 保留公開錯誤的分類與訊息，將內部原因留在診斷文字中。
// errors.Is/As 只沿公開錯誤查找，避免內部原因覆蓋對外契約。
func WithCause(public, cause error) error {
	if cause == nil {
		return public
	}
	return &causedError{public: public, cause: cause}
}

type causedError struct{ public, cause error }

func (e *causedError) Error() string { return e.public.Error() + "：" + e.cause.Error() }
func (e *causedError) Unwrap() error { return e.public }

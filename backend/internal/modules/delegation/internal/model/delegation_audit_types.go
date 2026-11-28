package model

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"uuid"
)

// DelegationMaxBatch 是稽核批次的筆數上限（設計書 56 §7.2.8）。
const DelegationMaxBatch = delegationMaxBatch

// AuditAction 是稽核的動作（原本是 CodeMeta 的 DelegationAuditAction）。DB 以 CHECK 限定這 20 個值（設計書 56
// §6.5.3），所以 Label、RiskLevel 的退路走不到，只是 fail closed 的保險；結束摘要照 §6.1.11 以子字串數履約、金流類的
// 動作，結構上恆為 0。
type AuditAction string

const (
	AuditStorefrontUpdate      AuditAction = "STOREFRONT_UPDATE"
	AuditStorefrontLogoUpdate  AuditAction = "STOREFRONT_LOGO_UPDATE"
	AuditStorefrontCoverUpdate AuditAction = "STOREFRONT_COVER_UPDATE"
	AuditProductCreate         AuditAction = "PRODUCT_CREATE"
	AuditProductUpdate         AuditAction = "PRODUCT_UPDATE"
	AuditProductPublish        AuditAction = "PRODUCT_PUBLISH"
	AuditProductUnpublish      AuditAction = "PRODUCT_UNPUBLISH"
	AuditProductBatchUnpublish AuditAction = "PRODUCT_BATCH_UNPUBLISH"
	AuditPostCreate            AuditAction = "POST_CREATE"
	AuditPostUpdate            AuditAction = "POST_UPDATE"
	AuditPostPublish           AuditAction = "POST_PUBLISH"
	AuditPostUnpublish         AuditAction = "POST_UNPUBLISH"
	AuditPostSchedule          AuditAction = "POST_SCHEDULE"
	AuditOrderPIIReveal        AuditAction = "ORDER_PII_REVEAL"
	AuditStorefrontRead        AuditAction = "STOREFRONT_READ"
	AuditProductRead           AuditAction = "PRODUCT_READ"
	AuditPostRead              AuditAction = "POST_READ"
	AuditOrderRead             AuditAction = "ORDER_READ"
	AuditCorrection            AuditAction = "AUDIT_CORRECTION"
	AuditMediaUpload           AuditAction = "MEDIA_UPLOAD"
)

// RiskLevel 是稽核列的風險等級，由動作決定（同一個 orders.read 底下，看訂單清單與揭露收件人個資風險不同）。
type RiskLevel string

const (
	RiskLow    RiskLevel = "LOW"
	RiskMedium RiskLevel = "MEDIUM"
	RiskHigh   RiskLevel = "HIGH"
)

type auditActionRule struct {
	label string
	risk  RiskLevel
}

var auditActions = map[AuditAction]auditActionRule{
	AuditStorefrontUpdate:      {"更新店面資料", RiskMedium},
	AuditStorefrontLogoUpdate:  {"更新商店 Logo", RiskMedium},
	AuditStorefrontCoverUpdate: {"更新商店封面", RiskMedium},
	AuditProductCreate:         {"建立商品", RiskMedium},
	AuditProductUpdate:         {"編輯商品", RiskMedium},
	AuditProductPublish:        {"上架商品", RiskHigh},
	AuditProductUnpublish:      {"下架商品", RiskHigh},
	AuditProductBatchUnpublish: {"批次下架商品", RiskHigh},
	AuditPostCreate:            {"建立貼文", RiskMedium},
	AuditPostUpdate:            {"編輯貼文", RiskMedium},
	AuditPostPublish:           {"發布貼文", RiskHigh},
	AuditPostUnpublish:         {"下架貼文", RiskHigh},
	AuditPostSchedule:          {"排程發布貼文", RiskHigh},
	AuditOrderPIIReveal:        {"查看訂單完整收件資訊", RiskHigh},
	AuditStorefrontRead:        {"查看店面內容（被拒絕）", RiskLow},
	AuditProductRead:           {"查看商品（被拒絕）", RiskLow},
	AuditPostRead:              {"查看貼文（被拒絕）", RiskLow},
	AuditOrderRead:             {"查看訂單（被拒絕）", RiskLow},
	AuditCorrection:            {"稽核補正（維運）", RiskLow},
	AuditMediaUpload:           {"上傳檔案（被拒絕）", RiskLow},
}

// AuditActions 回傳 20 個登錄的動作。
func AuditActions() []AuditAction {
	out := make([]AuditAction, 0, len(auditActions))
	for a := range auditActions {
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}

func (a AuditAction) Known() bool {
	_, ok := auditActions[a]
	return ok
}

// Label 是動作的顯示名；沒有登錄的動作回傳原值（看得出發生了什麼）。
func (a AuditAction) Label() string {
	if r, ok := auditActions[a]; ok {
		return r.label
	}
	return string(a)
}

// RiskLevel 是動作的風險等級；沒有登錄的動作是 high（fail closed：多數一列高風險，不會藏起一列）。
func (a AuditAction) RiskLevel() RiskLevel {
	if r, ok := auditActions[a]; ok {
		return r.risk
	}
	return RiskHigh
}

// ReadDenial 是四個只為了讓「被拒絕的讀取」有合法動作值的讀取動作；它們不會成功，也不通知任何人。
func (a AuditAction) ReadDenial() bool {
	return a == AuditStorefrontRead || a == AuditProductRead || a == AuditPostRead || a == AuditOrderRead
}

// DenialOnly 是只會出現在被拒絕（FAILED）列的動作：ReadDenial 的四個讀取，加上上傳（B1-D16：上傳不寫成功的稽核，稽核留在擁有者確認時）。
// 它們不是寫入動作、不需要操作原因、不通知任何人。
func (a AuditAction) DenialOnly() bool {
	return a.ReadDenial() || a == AuditMediaUpload
}

// MayNotifySeller 回傳這個動作的稽核列能不能通知賣家：被拒絕的讀取與上傳、PII 揭露與稽核補正恆為 false（機械保證，
// 不靠 gateway 記得）。寫入類不強制任何方向：強制成 true 會藏起 gateway 漏填的情況。
func (a AuditAction) MayNotifySeller() bool {
	return !a.DenialOnly() && a != AuditOrderPIIReveal && a != AuditCorrection
}

// AuditResult 是稽核列的結果。PENDING 是合法的終態：嘗試過、結果沒有確認。
type AuditResult string

const (
	AuditPending   AuditResult = "PENDING"
	AuditSucceeded AuditResult = "SUCCEEDED"
	AuditFailed    AuditResult = "FAILED"
)

// AuditOrigin 是發起的形態：操作者當下的互動，或代行期間設定、稍後自動執行的排程。
type AuditOrigin string

const (
	AuditInteractive AuditOrigin = "INTERACTIVE"
	AuditScheduled   AuditOrigin = "SCHEDULED"
)

// AuditResourceType 是稽核對象的種類。
type AuditResourceType string

const (
	ResourceStorefront AuditResourceType = "STOREFRONT"
	ResourceProduct    AuditResourceType = "PRODUCT"
	ResourcePost       AuditResourceType = "POST"
	ResourceOrder      AuditResourceType = "ORDER"
	ResourceMedia      AuditResourceType = "MEDIA"
)

func (t AuditResourceType) Valid() bool {
	return t == ResourceStorefront || t == ResourceProduct || t == ResourcePost || t == ResourceOrder || t == ResourceMedia
}

// ChangedField 是一個欄位的變更（已遮罩、已經過 allow-list 的顯示值）。
type ChangedField struct {
	Field  string
	Label  string
	Before string
	After  string
}

// AuditLog 是一列代行稽核。actor 與 target 是寫入當下的快照，不由 session 推導（中途改名時兩者可能不同）。
type AuditLog struct {
	Items        []AuditItem
	ID           uuid.UUID
	SessionID    int64
	Actor        DelegationActor
	Target       DelegationTarget
	Origin       AuditOrigin
	Capability   Capability
	Action       AuditAction
	ResourceType AuditResourceType
	ResourceID   *string
	ResourceName *string
	Result       AuditResult
	// BizCode 只有舊 gateway 的兩段式路徑寫入（過渡邊界）；FailureReason 是 FAILED 列的失敗原因（拒絕的代碼或 ErrorReason 的值名）。
	BizCode         *int32
	FailureReason   *string
	OperationReason *string
	CaseReference   *string
	RequestID       string
	CorrelationID   string
	IdempotencyKey  *string
	// ResourceVersionBefore、ResourceVersionAfter 是資源的版本（樂觀鎖）。
	ResourceVersionBefore *int64
	ResourceVersionAfter  *int64
	ChangedFields         []ChangedField
	UserAgent             *string
	RiskLevel             RiskLevel
	NotifySeller          bool
	// NotifiedAt 是完成事件在確認的 transaction 內排入送出的時刻；只有成功而且要通知賣家的列有。
	NotifiedAt  *time.Time
	CreatedAt   time.Time
	ConfirmedAt *time.Time
}

// AuditIntentInput 是稽核 intent 的輸入。
type AuditIntentInput struct {
	SessionID    int64
	Capability   Capability
	Action       AuditAction
	ResourceType AuditResourceType
	// ResourceID 是資源自己的 ID（商品、貼文、店家是數字，訂單是訂單號）；建立類是空字串（confirm 時補上）。
	ResourceID   string
	ResourceName string
	// OperationReason 在寫入類與 PII 揭露必填；只有四個被拒絕的讀取可以是空的。排程執行（Origin 是 scheduled）一律
	// 繼承建立排程時的理由，呼叫方給的值不採用。
	OperationReason       string
	ResourceVersionBefore *int64
	// BeforeSnapshot 是 allow-list 後的 JSON（不含 token 與個資）；nil 表示沒有。
	BeforeSnapshot json.RawMessage
	// IdempotencyKey 以 (session, key) 去重；PII 揭露刻意不去重，留空。
	IdempotencyKey string
	RequestID      string
	CorrelationID  string
	UserAgent      string
	PayloadHash    string
	NotifySeller   bool
	Origin         AuditOrigin
}

// AuditIntent 是 intent 的結果。Replayed 表示同一把冪等鍵已經有列，Result 是那一列目前的結果：呼叫方據此決定要不要
// 重做寫入。
type AuditIntent struct {
	AuditLogID uuid.UUID
	Replayed   bool
	Result     AuditResult
}

// ResourceNameMaxRunes 是資源名稱的上限（舊契約的 1–160 字；商品名稱最長 255 字，截斷而不是讓 intent 失敗）。
const ResourceNameMaxRunes = 160

// AuditIntentOutcome 是批次 intent 的一項：Err 是這一項的錯誤（其他項照跑）。
type AuditIntentOutcome struct {
	Intent AuditIntent
	Err    error
}

// AuditConfirmInput 是 confirm 的輸入。
type AuditConfirmInput struct {
	AuditLogID uuid.UUID
	// Result 是 SUCCEEDED 或 FAILED。
	Result AuditResult
	// BizCode 只記在 FAILED 的列；零值表示沒有。只有舊 gateway 的轉接層帶（過渡邊界）。
	BizCode int32
	// FailureReason 是 FAILED 列的失敗原因（拒絕的代碼或 ErrorReason 的值名）；空值寫 FailureReasonInternal。
	FailureReason string
	// ResourceID 是建立類在這裡補上的 ID；nil 保留 intent 寫入的值。
	ResourceID           *string
	ResourceVersionAfter *int64
	AfterSnapshot        json.RawMessage
	ChangedFields        []ChangedField
}

// ScheduledPublishAuditInput 是排程到點發布成功的單段稽核（沒有 intent 與 confirm）：SessionID 是發起排程的代行 session，ScheduleID 是冪等鍵的來源。
type ScheduledPublishAuditInput struct {
	SessionID  int64
	ScheduleID uuid.UUID
	PostID     uuid.UUID
	// Preview 是貼文內文的預覽，寫成資源名稱（截到 ResourceNameMaxRunes）。
	Preview string
}

// ScheduledPublishIdempotencyKey 是單段稽核的冪等鍵：一次排程至多一列。
func ScheduledPublishIdempotencyKey(scheduleID uuid.UUID) string {
	return "schedule:" + scheduleID.String()
}

// OperationAuditInput 是代行互動寫入的單段稽核：與被稽核的寫入同一個 transaction，寫入成功才留下（失敗時一起 rollback）。
type OperationAuditInput struct {
	SessionID    int64
	Capability   Capability
	Action       AuditAction
	ResourceType AuditResourceType
	// ResourceID 是資源自己的 ID；建立類在寫入前沒有，留空，寫入的事件到了由 OperationOutcome.ResourceID 補上。
	ResourceID            string
	ResourceName          string
	ResourceVersionBefore *int64
	// OperationReason 去除首尾空白後 1–100 字，不得含控制字元。
	OperationReason string
	NotifySeller    bool
	// RequestID 是呼叫方的 request_id（AIP-155）：同時是稽核列的 request_id 與 (session_id, idempotency_key) 的冪等鍵。
	RequestID     string
	CorrelationID string
	UserAgent     string
	// PayloadHash 綁定冪等鍵與請求內容：同一把鍵、不同內容不是重送。
	PayloadHash string
}

// OperationOutcome 是被稽核的寫入完成後補進稽核列的事實。
type OperationOutcome struct {
	ResourceID   string
	ResourceName string
	// VersionAfter 是資源寫入後的版本；0 表示沒有版本。
	VersionAfter int64
	Items        []AuditItem
}

type AuditItem struct {
	ResourceID    string
	ResourceName  string
	VersionBefore int64
	VersionAfter  int64
}

// FailureAuditInput 是一次失敗的代行請求的稽核：被拒絕（能力不足、session 已失效、店家不可代行，含讀取）或寫入的業務失敗。Session 是請求當時解析到的
// session（可能已結束、到期或撤銷：失敗列正是要記下這一種嘗試）。
type FailureAuditInput struct {
	Session      DelegationSession
	Capability   Capability
	Action       AuditAction
	ResourceType AuditResourceType
	// ResourceID 與 ResourceVersionBefore 是請求指向的資源與它的版本；不知道（建立類、尚未解析）留空。
	ResourceID string
	// ResourceName 是請求已知的資源名稱（建立類的預覽）；不知道留空。
	ResourceName          string
	ResourceVersionBefore *int64
	// OperationReason 只在值域合法時寫入（1–100 字）；寫入類的請求在驗證前就被拒絕時，理由可能是不合法的值，這時不寫。
	OperationReason string
	// RequestID 是呼叫方的 request_id；被拒絕得比解析請求還早（讀取、interceptor 的拒絕）時用請求的 trace ID。
	RequestID     string
	CorrelationID string
	UserAgent     string
	// Cause 是失敗的原因：failure_reason 取自 FailureReasonOf(Cause)：代行拒絕的代碼、ErrorReason，都沒有則是 INTERNAL。
	Cause error
}

// OperationReasonMaxRunes 是操作原因的上限。
const OperationReasonMaxRunes = 100

// LegalOperationReason 回傳去除首尾空白後的操作原因，與它是否合法（1 到 OperationReasonMaxRunes 字、不含控制字元）。不合法的值不寫進稽核列。
func LegalOperationReason(raw string) (string, bool) {
	reason := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(reason); n == 0 || n > OperationReasonMaxRunes {
		return "", false
	}
	if strings.ContainsFunc(reason, unicode.IsControl) {
		return "", false
	}
	return reason, true
}

// ErrOperationKeyTaken 表示 (session, request_id) 的 SUCCEEDED 稽核列已被另一個並行的請求寫下：RecordOperation 回它時沒有寫任何列，
// 呼叫方要回傳它讓整個 transaction rollback（資源模組的寫入跟著消失），再於 transaction 外重讀那一列收斂為重送。
var ErrOperationKeyTaken = errors.New("delegation：request_id 已有稽核列")

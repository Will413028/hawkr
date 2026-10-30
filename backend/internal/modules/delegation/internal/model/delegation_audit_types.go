package model

import (
	"encoding/json"
	"slices"
	"time"
	"uuid"
)

// DelegationMaxBatch 是稽核批次的筆數上限（設計書 56 §7.2.8）。
const DelegationMaxBatch = delegationMaxBatch

// AuditAction 是稽核的動作（原本是 CodeMeta 的 DelegationAuditAction）。DB 以 CHECK 限定這 19 個值（設計書 56
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

// auditActions 是 19 個登錄的動作：中文標籤（原本在 CodeMeta）與風險等級（CodeMeta 的 DelegationAuditAction）。
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
}

// AuditActions 回傳 19 個登錄的動作。
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

// ReadDenial 是四個只為了讓「被拒絕的讀取」有合法動作值的動作：它們不會成功，也不通知任何人。
func (a AuditAction) ReadDenial() bool {
	return a == AuditStorefrontRead || a == AuditProductRead || a == AuditPostRead || a == AuditOrderRead
}

// MayNotifySeller 回傳這個動作的稽核列能不能通知賣家：被拒絕的讀取、PII 揭露與稽核補正恆為 false（機械保證，
// 不靠 gateway 記得）。寫入類不強制任何方向：強制成 true 會藏起 gateway 漏填的情況。
func (a AuditAction) MayNotifySeller() bool {
	return !a.ReadDenial() && a != AuditOrderPIIReveal && a != AuditCorrection
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
)

func (t AuditResourceType) Valid() bool {
	return t == ResourceStorefront || t == ResourceProduct || t == ResourcePost || t == ResourceOrder
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
	ID              uuid.UUID
	SessionID       int64
	Actor           DelegationActor
	Target          DelegationTarget
	Origin          AuditOrigin
	Capability      Capability
	Action          AuditAction
	ResourceType    AuditResourceType
	ResourceID      *string
	ResourceName    *string
	Result          AuditResult
	BizCode         *int32
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
	// BizCode 只記在 FAILED 的列；零值表示沒有。
	BizCode int32
	// ResourceID 是建立類在這裡補上的 ID；nil 保留 intent 寫入的值。
	ResourceID           *string
	ResourceVersionAfter *int64
	AfterSnapshot        json.RawMessage
	ChangedFields        []ChangedField
}

package model

import (
	"time"
	"uuid"
)

// DelegationOperationCompleted 在代行的寫入確認成功、而且 intent 要求通知賣家時，於確認的 transaction 內發出（原本是
// account-service 的 admin_operation_outbox）。過渡期由 internal/legacy 排入 job，送到 notification-service 訂閱的
// RabbitMQ（delegation.operation.completed）。欄位是寫入當下的快照；消費者以 AuditLogID 去重。
type DelegationOperationCompleted struct {
	AuditLogID            uuid.UUID
	SessionID             int64
	SessionDisplayCode    string
	ActorAdminID          int64
	ActorDisplayName      string
	TargetStoreID         int64
	TargetSellerAccountID int64
	Capability            Capability
	Action                AuditAction
	// ActionLabel 是寫入當下的標籤（賣家收到的通知渲染它）：事後改了標籤，已經發生的事不該改寫意思。
	ActionLabel     string
	RiskLevel       RiskLevel
	ResourceType    AuditResourceType
	ResourceID      *string
	ResourceName    *string
	OperationReason *string
	CaseReference   *string
	// CorrelationID 是發生這件事的請求脈絡（稽核列的 correlation_id）。
	CorrelationID string
	// OccurredAt 是操作發生（intent）的時刻；NotifiedAt 是確認、排入送出的時刻。
	OccurredAt time.Time
	NotifiedAt time.Time
}

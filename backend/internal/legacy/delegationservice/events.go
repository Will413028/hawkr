package delegationservice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
)

// delegationCompleted 是舊的 delegation.operation.completed（go-pkg/mq 的 DelegationOperationCompletedEvent：BaseEvent
// 加業務欄位）：notification-service 還在 RabbitMQ 上訂閱它，渲染給賣家的通知。每個 ID 都是字串（泛型 map 的消費者會把
// int64 經 float64 截斷）；resource_name 與 case_reference 沒有時省略，resource_id 與 operation_reason 沒有時是空字串。
// 消費者以 audit_log_id 去重（event_id 在排入時決定，重送時不變）。
type delegationCompleted struct {
	EventID               string    `json:"event_id"`
	EventType             string    `json:"event_type"`
	Version               string    `json:"version"`
	Timestamp             time.Time `json:"timestamp"`
	Source                string    `json:"source"`
	CorrelationID         string    `json:"correlation_id,omitempty"`
	AuditLogID            string    `json:"audit_log_id"`
	SessionID             string    `json:"session_id"`
	SessionDisplayCode    string    `json:"session_display_code"`
	ActorAdminID          string    `json:"actor_admin_id"`
	ActorDisplayName      string    `json:"actor_display_name"`
	TargetStoreID         string    `json:"target_store_id"`
	TargetSellerAccountID string    `json:"target_seller_account_id"`
	Capability            string    `json:"capability"`
	Action                string    `json:"action"`
	ActionLabel           string    `json:"action_label"`
	RiskLevel             string    `json:"risk_level"`
	ResourceType          string    `json:"resource_type"`
	ResourceID            string    `json:"resource_id"`
	ResourceName          *string   `json:"resource_name,omitempty"`
	OperationReason       string    `json:"operation_reason"`
	CaseReference         *string   `json:"case_reference,omitempty"`
	OccurredAt            string    `json:"occurred_at"`
}

// OnDelegationOperationCompleted 在確認代行稽核的 transaction 內排入送出 delegation.operation.completed 的 job（delegation
// 的 DelegationOperationCompleted；在組裝點以 events.Subscribe 訂閱），取代 account-service 的 admin_operation_outbox
// 與它的 dispatcher。
func OnDelegationOperationCompleted(enqueue func(context.Context, rabbitmq.PublishArgs) error) func(context.Context, delegation.DelegationOperationCompleted) error {
	return func(ctx context.Context, e delegation.DelegationOperationCompleted) error {
		eventID := uuid.NewV7().String()
		body := delegationCompleted{
			EventID: eventID, EventType: "DELEGATION_OPERATION_COMPLETED", Version: "1.0",
			Timestamp: e.NotifiedAt.UTC(), Source: "account-service", CorrelationID: e.CorrelationID,
			AuditLogID:            e.AuditLogID.String(),
			SessionID:             formatID(e.SessionID),
			SessionDisplayCode:    e.SessionDisplayCode,
			ActorAdminID:          formatID(e.ActorAdminID),
			ActorDisplayName:      e.ActorDisplayName,
			TargetStoreID:         formatID(e.TargetStoreID),
			TargetSellerAccountID: formatID(e.TargetSellerAccountID),
			Capability:            string(e.Capability),
			Action:                string(e.Action),
			ActionLabel:           e.ActionLabel,
			RiskLevel:             string(e.RiskLevel),
			ResourceType:          string(e.ResourceType),
			ResourceID:            deref(e.ResourceID),
			ResourceName:          e.ResourceName,
			OperationReason:       deref(e.OperationReason),
			CaseReference:         e.CaseReference,
			OccurredAt:            formatTime(e.OccurredAt),
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("delegationservice：delegation.operation.completed：%w", err)
		}
		return enqueue(ctx, rabbitmq.PublishArgs{
			EventID: eventID, Exchange: "hawkr.delegation", RoutingKey: "delegation.operation.completed", Body: raw,
		})
	}
}

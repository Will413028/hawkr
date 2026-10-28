package accountservice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
)

// accountDeleted 是舊的 account.deleted 事件（go-pkg/mq 的 AccountDeletedEvent）：im 與 notification 還在 RabbitMQ 上訂閱它
// （遷移計畫的 events.csv）；buyer 與 seller 不訂閱，由 accountdeletion 在刪帳的 transaction 內直接關閉。
type accountDeleted struct {
	EventID   string            `json:"event_id"`
	EventType string            `json:"event_type"`
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
	Metadata  map[string]string `json:"metadata"`
	AccountID int64             `json:"accountId"`
}

// OnAccountDeleted 在刪除帳號的 transaction 內排入送出 account.deleted 的 job（identity 的 AccountDeleted 事件；在組裝點
// 以 events.Subscribe 訂閱）。
func OnAccountDeleted(enqueue func(context.Context, rabbitmq.PublishArgs) error) func(context.Context, identity.AccountDeleted) error {
	return func(ctx context.Context, e identity.AccountDeleted) error {
		eventID := uuid.NewV7().String()
		body, err := json.Marshal(accountDeleted{
			EventID: eventID, EventType: "ACCOUNT_DELETED", Version: "1.0",
			Timestamp: e.DeletedAt.UTC(), Source: "account-service", Metadata: map[string]string{}, AccountID: e.AccountID,
		})
		if err != nil {
			return fmt.Errorf("accountservice：account.deleted：%w", err)
		}
		return enqueue(ctx, rabbitmq.PublishArgs{EventID: eventID, Exchange: "hawkr.account", RoutingKey: "account.deleted", Body: body})
	}
}

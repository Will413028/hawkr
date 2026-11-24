// Package suppress 是 email suppression 名單的寫入端：Resend 的退信與投訴 webhook 驗簽後先寫 platform/inbox（commit 後才回 2xx），
// 派送 job 再呼叫 Handle 把永久退信與投訴的位址寫進 notification.email_suppressions。email 渠道 job 送出前讀這張表（deliver 套件）。
package suppress

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/emailaddr"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/resend"
)

// Provider 是 inbox 裡 Resend 事件的 provider 名稱。
const Provider = "resend"

// EventRetention 是已處理事件原始 payload（含收件位址）在 inbox 的保存期。
const EventRetention = 14 * 24 * time.Hour

const routeKey = "email"

// Deps 是 Service 的依賴。WebhookSecret 為空表示沒有設定 webhook：Receive 回 FailedPrecondition。
type Deps struct {
	DB            *db.DB
	Inbox         *inbox.Inbox
	WebhookSecret string
	Now           func() time.Time
}

// Service 接收退信與投訴事件。
type Service struct{ d Deps }

// New 建立 Service；有 Inbox 時登記 Resend 的 provider（同一個 inbox 登記兩次會 panic，每個行程只建一個模組）。
func New(d Deps) *Service {
	s := &Service{d: d}
	if d.Inbox != nil {
		d.Inbox.Register(inbox.Source{Provider: Provider, Retention: EventRetention, Handle: s.Handle})
	}
	return s
}

// Receive 是 webhook 入口：驗簽、只收會讓位址進名單的事件（永久退信、投訴），在一個 transaction 內寫 inbox 並排入派送 job。
// 沒有設定密鑰或 inbox 回 FailedPrecondition；簽章或格式不合回 InvalidInput。其他事件型別（送達、延遲、暫時退信）直接接受、不寫入。
func (s *Service) Receive(ctx context.Context, header http.Header, body []byte) error {
	if s.d.WebhookSecret == "" || s.d.Inbox == nil {
		return apperr.FailedPrecondition("Resend webhook 沒有設定")
	}
	ev, err := resend.Verify(s.d.WebhookSecret, header, body, s.d.Now())
	if errors.Is(err, resend.ErrSignature) || errors.Is(err, resend.ErrMalformed) {
		return apperr.InvalidInput("Resend webhook 簽章或格式不合")
	}
	if err != nil {
		return err
	}
	if !ev.Suppresses() {
		return nil
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := s.d.Inbox.Record(ctx, inbox.Event{Provider: Provider, EventID: ev.ID, RouteKey: routeKey, Payload: body})
		return err
	})
}

// Handle 是 inbox 的 handler（派送 job 的 transaction 內）：把事件的收件位址寫進 suppression 名單並標記已處理。可重複執行。
func (s *Service) Handle(ctx context.Context, e inbox.Event) error {
	ev, err := resend.ParseEvent(e.Payload)
	if err != nil {
		return fmt.Errorf("解析 Resend 事件：%w", err)
	}
	reason := "BOUNCE"
	if ev.Type == resend.EventComplained {
		reason = "COMPLAINT"
	}
	q := store.New(s.d.DB.Conn(ctx))
	if ev.Suppresses() {
		for _, to := range ev.To {
			addr, ok := emailaddr.Normalize(to)
			if !ok {
				continue
			}
			if err := q.InsertEmailSuppression(ctx, store.InsertEmailSuppressionParams{Address: addr, Reason: reason, Now: s.d.Now()}); err != nil {
				return fmt.Errorf("寫入 suppression 名單：%w", err)
			}
		}
	}
	return s.d.Inbox.MarkProcessed(ctx, e.Provider, e.EventID, "notification.suppress")
}

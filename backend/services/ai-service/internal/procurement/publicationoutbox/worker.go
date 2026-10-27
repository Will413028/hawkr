package publicationoutbox

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
)

const (
	// maxAttempts 是重試上限；attempt_count 達到它就轉終態 failed，不再重排。
	// 第一次呼叫是 attempt 1。
	maxAttempts = 5
	// baseBackoff 是 backoff 的單位間隔：延遲 = 2^attempt * baseBackoff。
	baseBackoff = 1 * time.Minute
	// maxBackoff 封頂每次重試的延遲，不論 attempt 多大。
	maxBackoff = 1 * time.Hour
	// fetchBatch 限制每次 RunOnce 撈出的候選列數。
	fetchBatch = 50
	// claimLease 是成功 Claim 之後把 next_attempt_at 推到多遠的未來。這條路徑
	// 一列就是一次 MQ publish（publisher confirm，毫秒級），3 分鐘留了約四個
	// 數量級的餘裕。《多副本排程搶佔規範》§三。
	claimLease = 3 * time.Minute

	// outboxName 是 pkg/outbox.Metrics 的 outbox_name 標籤值。
	outboxName = "rfq_publication"
)

// RawPublisher 是把 body 原樣送上 exchange 的最小 seam；*mq.Publisher 結構上滿足它。
type RawPublisher interface {
	PublishRaw(ctx context.Context, exchange, routingKey string, body []byte) error
}

// PublishFunc 是實際外送動作的 seam，讓 worker 的測試不需要 broker。
type PublishFunc func(ctx context.Context, eventType string, payload []byte) error

// NewPublishFunc 把 outbox 列的 payload **原樣**送到 procurement.events。
//
// ⛔ 刻意不解碼再重新編碼：payload 在 execute 的交易內就已經寫成上線形狀
// （camelCase 的 inviteId／publicationId／storeId，三者皆字串化 TSID），已提交的
// bytes 就是上線的 bytes——transactional outbox 的重點正是這個等式。中間插一層
// struct 只會多一個讓兩者悄悄漂開的地方。
//
// ⚠️ 這條線的 payload 形狀與隔壁 legacy quote-invite publisher 的 snake_case
// store_id **不同**，不可互抄（見 pkg/mq 兩組 routing key 的註解）。
func NewPublishFunc(raw RawPublisher) PublishFunc {
	return func(ctx context.Context, eventType string, payload []byte) error {
		routingKey, ok := routingKeyFor(eventType)
		if !ok {
			return fmt.Errorf("publicationoutbox: unknown event type %q", eventType)
		}
		return raw.PublishRaw(ctx, mq.ExchangeProcurement, routingKey, payload)
	}
}

// routingKeyFor 把 event_type（DB 值域，由 chk_prpo_event_type 強制）映到 routing
// key。兩邊都是封閉值域，新增事件時一起改。
func routingKeyFor(eventType string) (string, bool) {
	switch eventType {
	case repository.PublicationOutboxEventInviteCreated:
		return mq.RoutingKeyProcurementRfqPublicationInviteCreated, true
	case repository.PublicationOutboxEventInviteExternalCreated:
		return mq.RoutingKeyProcurementRfqPublicationInviteExternalCreated, true
	default:
		return "", false
	}
}

// Worker 排空 procurement_rfq_publication_outbox。claim 搶佔、backoff、終態處理
// 全部在 pkg/outbox.Worker[*Row] 裡；本型別只多一件事：MQ 在啟動時就不可用的話
// 整個 tick 跳過。
type Worker struct {
	inner  *outbox.Worker[*Row]
	logger *zap.Logger
	// publisherMissing 在建構時決定一次就不再變，而且只代表**一種**情況：
	// RabbitMQ URL 根本沒設定（config 層的固定條件，見 di.go newRawPublisher）。
	// broker 暫時連不上**不**走這裡——那由 lazyRawPublisher 每次發送時重試連線，
	// 不需要重啟程序。
	//
	// RunOnce 在委派給 inner **之前**檢查，因為 pkg/outbox.Worker 一律先 Claim 再
	// 叫 ProcessFunc——放進 ProcessFunc 裡檢查等於白燒一次 claim lease，更糟的是會
	// 累加 attempt_count，把「這個環境沒接 MQ」變成一批終態 FAILED 的 invite。
	// 比照 buyer-service 的同款前置檢查。
	publisherMissing bool
}

// NewWorker 建構 worker。publish 為 nil（RabbitMQ URL 沒設定）時 RunOnce 是 no-op；
// 期間 outbox 列原封不動留在 pending，backlog gauge 會把它看得見（那正是 di.go
// 一併掛上 Metrics.Refresh 的理由）。broker 暫時不可用不是這種情況：publish 會回
// ErrPublisherUnavailable，那一列照 backoff 重排，下一輪重新嘗試連線。
// metrics 不得為 nil。
func NewWorker(repo *Repository, publish PublishFunc, metrics *outbox.Metrics, logger *zap.Logger) *Worker {
	if metrics == nil {
		panic("publicationoutbox.NewWorker: metrics must not be nil")
	}

	// ⛔ 順序固定：**先發 MQ、後轉 invite 狀態**，不可對調。
	//
	// 先轉 SENT 再發送，一旦發送失敗就留下「說已送出但沒送」——那正是本條線要
	// 消滅的失效模式。這個順序的最壞後果是「invite 停在 QUEUED 但其實已送達」，
	// 方向是低估而非說謊。
	//
	// MarkDone 失敗導致的重發由 consumer 端的 DB 唯一索引吸收（機械保證，不是
	// 口頭約定）：notification-service 的 notifications_inbox_dedup_idx 與
	// push_notifications_dedup_idx 以 (user_id, user_type, event_type,
	// reference_id) 去重，而本事件的 template 把 reference_id 綁到 inviteId。
	process := func(ctx context.Context, row *Row) (any, error) {
		if err := publish(ctx, row.EventType, row.Payload); err != nil {
			return nil, err
		}
		// CAS 影響 0 列不是錯誤：代表前一輪已轉過（MarkDone 失敗後的重試路徑）。
		return nil, repo.MarkInviteSent(ctx, row.InviteID, time.Now())
	}

	hooks := outbox.Hooks[*Row]{
		OnAttemptFailed: func(_ context.Context, row *Row, cause string, err error) {
			logger.Warn("rfq-publication-outbox: attempt failed",
				zap.Int64("id", row.ID), zap.Int64("invite_id", row.InviteID),
				zap.String("event_type", row.EventType),
				zap.String("cause", cause), zap.Error(err))
			metrics.AttemptFailedTotal(cause).Inc()
		},
		OnTerminalFailed: func(_ context.Context, row *Row, newAttempt int, lastErr string, markFailedErr error) {
			if markFailedErr != nil {
				// MarkFailed 沒寫成 ⇒ outbox 列仍是 pending、invite 仍是 QUEUED
				// （兩者同一交易，見 Repository.MarkFailed），下一輪重試整段。
				logger.Warn("rfq-publication-outbox: MarkFailed error (degraded)",
					zap.Int64("id", row.ID), zap.Int64("invite_id", row.InviteID),
					zap.Int("attempt", newAttempt), zap.String("last_error", lastErr),
					zap.Error(markFailedErr))
				return
			}
			logger.Warn("rfq-publication-outbox: row exhausted all retries → failed",
				zap.Int64("id", row.ID), zap.Int64("invite_id", row.InviteID),
				zap.Int("attempt", newAttempt), zap.String("last_error", lastErr))
			metrics.FailedTotal().Inc()
		},
	}

	cfg := outbox.Config{
		MaxAttempts: maxAttempts,
		BaseBackoff: baseBackoff,
		MaxBackoff:  maxBackoff,
		FetchBatch:  fetchBatch,
		ClaimLease:  claimLease,
	}

	return &Worker{
		inner:            outbox.New[*Row](repo, process, cfg, hooks),
		logger:           logger,
		publisherMissing: publish == nil,
	}
}

// RunOnce 處理一批 pending+due 的列後返回，由 scheduler 每個 tick 呼叫一次。
func (w *Worker) RunOnce(ctx context.Context) error {
	if w.publisherMissing {
		w.logger.Warn("rfq-publication-outbox: RabbitMQ URL not configured, skipping tick")
		return nil
	}
	return w.inner.RunOnce(ctx)
}

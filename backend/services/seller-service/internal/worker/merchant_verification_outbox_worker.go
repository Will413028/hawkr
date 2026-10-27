// merchant_verification_outbox_worker.go: seller-service's certification/PayUni
// verification-notification outbox worker, built on
// github.com/Will413028/hawkr/go-pkg/outbox. Unlike seller_deleted_outbox_worker.go
// this is a single-phase worker — the row's payload IS the MQ event; there is
// no resume checkpoint because there is only one side effect (publish).
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/zap"
)

const (
	mvoMaxAttempts = 5
	mvoBaseBackoff = 1 * time.Minute
	mvoMaxBackoff  = 1 * time.Hour
	mvoFetchBatch  = 50
	// mvoClaimLease: pure MQ publish, no downstream RPC — a short lease is
	// enough (spec §3; contrast seller_deleted_outbox's 3-minute lease, which
	// also waits on an account-service RemoveSellerRole call).
	mvoClaimLease = 1 * time.Minute
)

// merchantVerificationPublisher is the seam for publishing a verification
// event. *mq.MerchantEventPublisher satisfies this.
type merchantVerificationPublisher interface {
	PublishVerificationCompleted(ctx context.Context, event domain.MerchantVerificationCompletedEvent) error
}

// MerchantVerificationOutboxWorker drains pending merchant_verification_outbox
// rows and publishes them. RunOnce is the single entry point called by the
// scheduler on every tick.
type MerchantVerificationOutboxWorker struct {
	inner            *outbox.Worker[*domain.MerchantVerificationOutbox]
	publisherMissing bool
}

// NewMerchantVerificationOutboxWorker constructs a worker. If pub is nil (MQ
// unavailable at startup), RunOnce is a no-op until the process restarts with
// a healthy MQ connection. metrics must not be nil.
func NewMerchantVerificationOutboxWorker(
	repo domain.MerchantVerificationOutboxRepository,
	pub merchantVerificationPublisher,
	metrics *outbox.Metrics,
) *MerchantVerificationOutboxWorker {
	if metrics == nil {
		panic("worker.NewMerchantVerificationOutboxWorker: metrics must not be nil")
	}

	process := func(ctx context.Context, row *domain.MerchantVerificationOutbox) (any, error) {
		var event domain.MerchantVerificationCompletedEvent
		if err := json.Unmarshal(row.Payload, &event); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
		if err := pub.PublishVerificationCompleted(ctx, event); err != nil {
			return nil, fmt.Errorf("publish: %w", err)
		}
		return nil, nil
	}

	hooks := outbox.Hooks[*domain.MerchantVerificationOutbox]{
		OnAttemptFailed: func(ctx context.Context, row *domain.MerchantVerificationOutbox, cause string, err error) {
			logger.Warn(ctx, "merchant-verification-outbox: attempt failed",
				zap.Int64("id", row.ID), zap.Int64("accountID", row.AccountID),
				zap.String("cause", cause), zap.Error(err))
			metrics.AttemptFailedTotal(cause).Inc()
		},
		OnTerminalFailed: func(ctx context.Context, row *domain.MerchantVerificationOutbox, newAttempt int, lastErr string, markFailedErr error) {
			if markFailedErr != nil {
				logger.Warn(ctx, "merchant-verification-outbox: MarkFailed error (degraded)",
					zap.Int64("id", row.ID), zap.Int64("accountID", row.AccountID),
					zap.Int("attempt", newAttempt), zap.String("lastErr", lastErr), zap.Error(markFailedErr))
				return
			}
			logger.Warn(ctx, "merchant-verification-outbox: row exhausted all retries → failed",
				zap.Int64("id", row.ID), zap.Int64("accountID", row.AccountID),
				zap.Int("attempt", newAttempt), zap.String("lastErr", lastErr))
			metrics.FailedTotal().Inc()
		},
	}

	cfg := outbox.Config{
		MaxAttempts: mvoMaxAttempts,
		BaseBackoff: mvoBaseBackoff,
		MaxBackoff:  mvoMaxBackoff,
		FetchBatch:  mvoFetchBatch,
		ClaimLease:  mvoClaimLease,
	}

	return &MerchantVerificationOutboxWorker{
		inner:            outbox.New[*domain.MerchantVerificationOutbox](repo, process, cfg, hooks),
		publisherMissing: pub == nil,
	}
}

// RunOnce processes one batch of pending+due outbox rows and returns.
func (w *MerchantVerificationOutboxWorker) RunOnce(ctx context.Context) error {
	if w.publisherMissing {
		logger.Warn(ctx, "merchant-verification-outbox: publisher unavailable, skipping tick")
		return nil
	}
	return w.inner.RunOnce(ctx)
}

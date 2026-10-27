package mq

import (
	"context"
	"fmt"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

const (
	// 按審核結果拆分 routing key，讓 notification-service 模板引擎能推送不同文案
	routingKeyVerificationApproved    = "merchant.verification.approved"
	routingKeyVerificationRejected    = "merchant.verification.rejected"
	routingKeyVerificationPendingInfo = "merchant.verification.pending_info"
	routingKeyVerificationForwarded   = "merchant.verification.forwarded"
	routingKeyVerificationUnderReview = "merchant.verification.under_review"
)

// MerchantEventPublisher publishes merchant-related events to RabbitMQ.
//
// It wraps pkgmq.Publisher rather than dialing its own amqp.Connection/
// amqp.Channel, so it inherits publisher confirms (PublishWithDeferredConfirm
// + confirm.WaitContext on every publish) and automatic reconnect (a
// background goroutine redials on connection/channel loss) — required
// because this publisher backs the merchant_verification_outbox worker's
// eventual-delivery guarantee (issue #2806); a raw channel that silently
// stops accepting publishes after a drop would defeat that guarantee.
type MerchantEventPublisher struct {
	pub    *pkgmq.Publisher
	logger *zap.Logger
}

// NewMerchantEventPublisher creates a publisher connected to RabbitMQ.
func NewMerchantEventPublisher(url string, logger *zap.Logger) (*MerchantEventPublisher, error) {
	pub, err := pkgmq.NewPublisher(url)
	if err != nil {
		return nil, err
	}
	return &MerchantEventPublisher{pub: pub, logger: logger}, nil
}

// PublishVerificationCompleted publishes a merchant verification result event
// built from a merchant_verification_outbox row (issue #2806). Returns an
// error so the outbox worker can retry a failed publish with backoff instead
// of losing it to a log line.
func (p *MerchantEventPublisher) PublishVerificationCompleted(ctx context.Context, event domain.MerchantVerificationCompletedEvent) error {
	routingKey := routingKeyForStatus(event.Status)
	if routingKey == "" {
		return fmt.Errorf("unknown verification status %q, not publishing", event.Status)
	}
	if err := p.pub.Publish(ctx, pkgmq.ExchangeMerchant, routingKey, event); err != nil {
		return fmt.Errorf("publish verification event: %w", err)
	}
	p.logger.Info("[MQ] verification event published",
		zap.Int64("merchant_id", event.MerchantID), zap.String("status", event.Status), zap.String("routing_key", routingKey))
	return nil
}

func routingKeyForStatus(status string) string {
	switch status {
	case "approved":
		return routingKeyVerificationApproved
	case "rejected":
		return routingKeyVerificationRejected
	case "pending_info":
		return routingKeyVerificationPendingInfo
	case "forwarded_to_payuni":
		return routingKeyVerificationForwarded
	case "under_review":
		return routingKeyVerificationUnderReview
	default:
		// 未知 status 不得 fallback 到任何真實 routing key（尤其 approved 會推「審核
		// 已全部完成」文案）；回空字串由 caller 跳過 publish。
		return ""
	}
}

// Close closes the underlying pkgmq.Publisher (which stops its reconnect
// watcher and closes the RabbitMQ channel/connection). Kept void-returning
// to match the existing fx.Lifecycle OnStop call site in di.go; a close
// failure is logged rather than silently swallowed.
func (p *MerchantEventPublisher) Close() {
	if p.pub == nil {
		return
	}
	if err := p.pub.Close(); err != nil {
		p.logger.Warn("[MQ] failed to close MerchantEventPublisher", zap.Error(err))
	}
}

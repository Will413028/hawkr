// Package mq publishes im-service domain events to RabbitMQ.
package mq

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"go.uber.org/zap"
)

const (
	// exchangeIM is the topic exchange for im-service domain events.
	// It is declared durable by consumers (notification-service worker), so the
	// publisher does not declare it — matching the order/payment/review convention.
	exchangeIM = "hawkr.im"

	// routingKeyChatMessageSent is published for every persisted chat message and
	// drives offline FCM push. notification-service matches it to DB templates.
	routingKeyChatMessageSent = "chat.message.sent"
)

// chatEventPublisher adapts the shared MQ publisher to usecase.ChatEventPublisher.
type chatEventPublisher struct {
	pub    *pkgmq.Publisher
	logger *zap.Logger
}

// PublishChatMessageSent publishes a chat.message.sent event. Best-effort: the
// message is already persisted and delivered over WebSocket, so a publish failure
// only loses the offline push — it is logged, never propagated to the caller.
func (p *chatEventPublisher) PublishChatMessageSent(ctx context.Context, e usecase.ChatMessageSentEvent) {
	if err := p.pub.Publish(ctx, exchangeIM, routingKeyChatMessageSent, e); err != nil {
		p.logger.Warn("publish chat.message.sent failed",
			zap.String("messageId", e.MessageID),
			zap.String("receiverId", e.ReceiverID),
			zap.Error(err))
	}
}

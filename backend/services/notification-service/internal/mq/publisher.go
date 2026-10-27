// Package mq provides MQ publisher for notification-service.
//
// 2026-06 新增：notification-service 首次成為 MQ producer，配合 41 號 SSE 推播平台 §4.3，
// publish 兩個事件給 gateway SSE consumer（僅推 web）：
//   - notification.inbox.new       — 寫完 inbox 後
//   - notification.unread_changed  — mark-read / mark-all-read / delete 後
//
// Mobile 端維持既有 FCM，不訂閱本 channel（見 41 §4.0）。
package mq

import (
	"context"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"go.uber.org/zap"
)

// EventPublisher defines the interface for publishing notification dispatch events.
type EventPublisher interface {
	PublishNotificationInboxNew(ctx context.Context, userID int64, userType string, item map[string]interface{}, unreadCount int64) error
	PublishNotificationUnreadChanged(ctx context.Context, userID int64, userType string, unreadCount int64) error
}

// Publisher wraps pkgmq.Publisher to implement EventPublisher.
type Publisher struct {
	pub *pkgmq.Publisher
}

// NewPublisher creates a new MQ publisher. Returns nil + error if RabbitMQ URL invalid;
// caller may choose to proceed with nil publisher (publish becomes no-op) on best-effort principle.
func NewPublisher(url string) (*Publisher, error) {
	pub, err := pkgmq.NewPublisher(url)
	if err != nil {
		return nil, err
	}
	return &Publisher{pub: pub}, nil
}

// PublishNotificationInboxNew publishes after a new inbox item is written.
func (p *Publisher) PublishNotificationInboxNew(ctx context.Context, userID int64, userType string, item map[string]interface{}, unreadCount int64) error {
	event := pkgmq.NewNotificationInboxNewEvent(userID, userType, item, unreadCount)
	if err := p.pub.PublishNotificationInboxNew(ctx, event); err != nil {
		zap.L().Error("failed to publish notification.inbox.new", zap.Error(err), zap.Int64("userId", userID))
		return err
	}
	return nil
}

// PublishNotificationUnreadChanged publishes after unread count changes (mark-read / delete).
func (p *Publisher) PublishNotificationUnreadChanged(ctx context.Context, userID int64, userType string, unreadCount int64) error {
	event := pkgmq.NewNotificationUnreadChangedEvent(userID, userType, unreadCount)
	if err := p.pub.PublishNotificationUnreadChanged(ctx, event); err != nil {
		zap.L().Error("failed to publish notification.unread_changed", zap.Error(err), zap.Int64("userId", userID))
		return err
	}
	return nil
}

// Close releases the underlying connection.
func (p *Publisher) Close() {
	if p.pub != nil {
		_ = p.pub.Close()
	}
}

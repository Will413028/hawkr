package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher handles publishing messages to RabbitMQ
type Publisher struct {
	url            string
	conn           *amqp.Connection
	channel        *amqp.Channel
	confirmTimeout time.Duration
	mu             sync.RWMutex
	closed         chan struct{}
}

var (
	defaultPublisher *Publisher
	publisherOnce    sync.Once
)

// GetPublisher returns the singleton publisher instance
func GetPublisher() (*Publisher, error) {
	var initErr error
	publisherOnce.Do(func() {
		url := os.Getenv("RABBITMQ_URL")
		if url == "" {
			url = "amqp://guest:guest@localhost:5672/"
		}
		defaultPublisher, initErr = NewPublisher(url)
	})
	return defaultPublisher, initErr
}

// publisherOpts collects optional NewPublisher configuration.
type publisherOpts struct {
	confirmTimeout time.Duration
}

// Option mutates publisherOpts at NewPublisher time.
type Option func(*publisherOpts)

// WithConfirmTimeout overrides the default 30s ceiling that
// publisher.PublishRaw / PublishDelayed use to bound
// confirm.WaitContext. Fan-out / latency-sensitive callers should
// shorten this (e.g. 5s) so a stuck broker cannot hang their tx.
func WithConfirmTimeout(d time.Duration) Option {
	return func(o *publisherOpts) {
		o.confirmTimeout = d
	}
}

// defaultConfirmTimeout bounds confirm.WaitContext when the caller did
// not pass WithConfirmTimeout, so a stuck broker cannot hang publishers
// indefinitely.
const defaultConfirmTimeout = 30 * time.Second

// NewPublisher creates a new publisher. Options default to a 30s
// confirm-wait timeout; pass WithConfirmTimeout(d) to override per caller.
//
// The returned Publisher auto-reconnects on connection or channel loss (e.g. its
// current RabbitMQ node restarting in a clustered deployment): a
// background goroutine watches both the connection's and channel's NotifyClose
// and redials url with backoff until it reconnects or Close is called.
func NewPublisher(url string, opts ...Option) (*Publisher, error) {
	cfg := publisherOpts{confirmTimeout: defaultConfirmTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}

	conn, ch, err := dialPublisherChannel(url)
	if err != nil {
		return nil, err
	}

	p := &Publisher{
		url:            url,
		conn:           conn,
		channel:        ch,
		confirmTimeout: cfg.confirmTimeout,
		closed:         make(chan struct{}),
	}
	go p.watchReconnect()
	return p, nil
}

// dialPublisherChannel dials url, opens a channel, and enables publisher
// confirms. Shared by NewPublisher and watchReconnect so both the initial
// connect and every reconnect attempt produce an identically-configured
// channel.
func dialPublisherChannel(url string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("failed to open channel: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, nil, fmt.Errorf("failed to enable confirms: %w", err)
	}

	return conn, ch, nil
}

// watchReconnect blocks until the publisher's current connection or channel closes,
// then redials with backoff (reconnectBackoff, shared with
// ResilientConsumer) until it reconnects or Close is called. Any node in a
// clustered broker can accept a publish, so a fresh dial against the same
// url — the k8s Service name, whose Endpoints fan out across every cluster
// node — transparently lands on a healthy node without this code needing
// to know which nodes exist.
func (p *Publisher) watchReconnect() {
	for {
		p.mu.RLock()
		conn := p.conn
		ch := p.channel
		p.mu.RUnlock()

		connErr := conn.NotifyClose(make(chan *amqp.Error, 1))
		chErr := ch.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case <-p.closed:
			return
		case err := <-connErr:
			log.Printf("[MQ] Publisher connection lost (%v), reconnecting", err)
		case err := <-chErr:
			log.Printf("[MQ] Publisher channel lost (%v), reconnecting", err)
		}

		for attempt := 1; ; attempt++ {
			select {
			case <-p.closed:
				return
			default:
			}

			newConn, newCh, dialErr := dialPublisherChannel(p.url)
			if dialErr != nil {
				delay := reconnectBackoff(attempt)
				log.Printf("[MQ] Publisher reconnect failed (%v), retrying in %v", dialErr, delay)
				select {
				case <-p.closed:
					return
				case <-time.After(delay):
					continue
				}
			}

			// Check if Close() was called while this dial was in progress,
			// making the check atomic with the swap by holding the lock.
			// If p.closed is already closed, close the newly-dialed connection
			// (and its channels cascade-close internally) to avoid leaking a live connection.
			p.mu.Lock()
			select {
			case <-p.closed:
				p.mu.Unlock()
				_ = newConn.Close()
				return
			default:
			}
			p.conn = newConn
			p.channel = newCh
			p.mu.Unlock()

			// conn (captured before this reconnect attempt) is no longer
			// referenced by p.conn/p.channel once the swap above lands. On a
			// channel-only close (protocol exception, connection stays healthy)
			// conn would otherwise leak its socket + amqp091-go's internal
			// reader/heartbeat goroutines. On a connection-level close, conn is
			// already dead and Close() on it is a safe no-op — so this is safe
			// to call unconditionally regardless of which kind of close
			// triggered this reconnect.
			_ = conn.Close()
			log.Printf("[MQ] Publisher reconnected")
			break
		}
	}
}

// Publish publishes a message to an exchange
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, event interface{}) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return p.PublishRaw(ctx, exchange, routingKey, body)
}

// PublishRaw publishes raw bytes to an exchange
func (p *Publisher) PublishRaw(ctx context.Context, exchange, routingKey string, body []byte) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	confirm, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Wait for confirmation with a bounded background deadline
	// （publisher confirm 規則見 docs/開發規範/MQ 與事件管理規範.md）：
	// 故意 NOT 傳 caller ctx — 訊息已進 broker channel，caller 取消（如 HTTP
	// request cancellation）不應導致「明明 broker 即將 ack」的誤判失敗。bound 透過
	// publisherConfirmTimeout 限制最大等待時間，避免 broker 卡死 hang。
	confirmCtx, cancel := context.WithTimeout(context.Background(), p.confirmTimeout)
	defer cancel()
	acked, err := confirm.WaitContext(confirmCtx)
	if err != nil {
		return fmt.Errorf("publisher confirm wait: %w", err)
	}
	if !acked {
		return fmt.Errorf("message was not confirmed")
	}

	return nil
}

// PublishDelayed publishes a delayed message
func (p *Publisher) PublishDelayed(ctx context.Context, exchange, routingKey string, event interface{}, delay time.Duration) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	confirm, err := p.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
			Headers: amqp.Table{
				"x-delay": int64(delay.Milliseconds()),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish delayed message: %w", err)
	}

	// 同 PublishRaw 的鎖定理由（docs/開發規範/MQ 與事件管理規範.md）：訊息已上 channel，caller ctx 取消
	// 不應導致誤判失敗；改用 bounded background ctx 防 broker 卡死。
	confirmCtx, cancel := context.WithTimeout(context.Background(), p.confirmTimeout)
	defer cancel()
	acked, err := confirm.WaitContext(confirmCtx)
	if err != nil {
		return fmt.Errorf("publisher confirm wait: %w", err)
	}
	if !acked {
		return fmt.Errorf("delayed message was not confirmed")
	}

	return nil
}

// ==================== Shipping/Fulfillment Events ====================

// ==================== Review Events ====================

// PublishReviewCreated publishes a REVIEW_CREATED event
func (p *Publisher) PublishReviewCreated(ctx context.Context, event *ReviewCreatedEvent) error {
	return p.Publish(ctx, ExchangeReview, RoutingKeyReviewCreated, event)
}

// PublishReviewHidden publishes a REVIEW_HIDDEN event (#1156)
func (p *Publisher) PublishReviewHidden(ctx context.Context, event *ReviewHiddenEvent) error {
	return p.Publish(ctx, ExchangeReview, RoutingKeyReviewHidden, event)
}

// ==================== Content Events (Social) ====================

// PublishPostCreated publishes a POST_CREATED event
func (p *Publisher) PublishPostCreated(ctx context.Context, event *PostCreatedEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyPostCreated, event)
}

// PublishCommentCreated publishes a COMMENT_CREATED event
func (p *Publisher) PublishCommentCreated(ctx context.Context, event *CommentCreatedEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyCommentCreated, event)
}

// ==================== Marketing Events (Coupon) ====================

// ==================== Post/Comment Interaction Events ====================

func (p *Publisher) PublishPostLiked(ctx context.Context, event *PostLikedEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyPostLiked, event)
}

func (p *Publisher) PublishPostHiddenByPlatform(ctx context.Context, event *PostHiddenByPlatformEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyPostHiddenByPlatform, event)
}

func (p *Publisher) PublishCommentReplied(ctx context.Context, event *CommentRepliedEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyCommentReplied, event)
}

// 41 號設計書 §6.1：2026-06 新增 3 個 comment realtime event；CommentLikedEvent 直接拔除
func (p *Publisher) PublishCommentUpdated(ctx context.Context, event *CommentUpdatedEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyCommentUpdated, event)
}

func (p *Publisher) PublishCommentDeleted(ctx context.Context, event *CommentDeletedEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyCommentDeleted, event)
}

func (p *Publisher) PublishCommentLikeChanged(ctx context.Context, event *CommentLikeChangedEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyCommentLikeChanged, event)
}

func (p *Publisher) PublishCounterDelta(ctx context.Context, event *CounterDeltaEvent) error {
	return p.Publish(ctx, ExchangeContent, RoutingKeyCounterDelta, event)
}

// 41 號設計書 §6.2：notification-service 首次成為 producer
func (p *Publisher) PublishNotificationInboxNew(ctx context.Context, event *NotificationInboxNewEvent) error {
	return p.Publish(ctx, ExchangeNotification, RoutingKeyNotificationInboxNew, event)
}

func (p *Publisher) PublishNotificationUnreadChanged(ctx context.Context, event *NotificationUnreadChangedEvent) error {
	return p.Publish(ctx, ExchangeNotification, RoutingKeyNotificationUnreadChanged, event)
}

// ==================== Account Events ====================

// PublishAccountDeleted publishes an ACCOUNT_DELETED event
func (p *Publisher) PublishAccountDeleted(ctx context.Context, event *AccountDeletedEvent) error {
	return p.Publish(ctx, ExchangeAccount, RoutingKeyAccountDeleted, event)
}

// ==================== Seller Events ====================

// PublishSellerDeleted publishes a SELLER_DELETED event
func (p *Publisher) PublishSellerDeleted(ctx context.Context, event *SellerDeletedEvent) error {
	return p.Publish(ctx, ExchangeSeller, RoutingKeySellerDeleted, event)
}

// PublishSellerProfileChanged publishes a SELLER_PROFILE_CHANGED event (#1426):
// seller name/logo/cover changed → cart-service evicts seller:meta:{id}.
func (p *Publisher) PublishSellerProfileChanged(ctx context.Context, event *SellerProfileChangedEvent) error {
	return p.Publish(ctx, ExchangeSeller, RoutingKeySellerProfileChanged, event)
}

// PublishSellerOnboarded publishes a SELLER_ONBOARDED event (first-time onboard
// AND re-onboard; consumed by the gateway resolver cache).
func (p *Publisher) PublishSellerOnboarded(ctx context.Context, event *SellerOnboardedEvent) error {
	return p.Publish(ctx, ExchangeSeller, RoutingKeySellerOnboarded, event)
}

// PublishSellerSuspended publishes a SELLER_SUSPENDED event (32 §13 / 44 §13.2 /
// 36 §3.6f admin manual suspension). Multi-consumer: product/order/coupon/
// notification/im.
func (p *Publisher) PublishSellerSuspended(ctx context.Context, event *SellerSuspendedEvent) error {
	return p.Publish(ctx, ExchangeSeller, RoutingKeySellerSuspended, event)
}

// PublishSellerUnsuspended publishes a SELLER_UNSUSPENDED event (32 §13 / 44 §13.2 /
// 36 §3.6f admin manual lift).
func (p *Publisher) PublishSellerUnsuspended(ctx context.Context, event *SellerUnsuspendedEvent) error {
	return p.Publish(ctx, ExchangeSeller, RoutingKeySellerUnsuspended, event)
}

func (p *Publisher) PublishSellerMembershipChanged(ctx context.Context, event *SellerMembershipChangedEvent) error {
	return p.Publish(ctx, ExchangeSeller, RoutingKeySellerMembershipChanged, event)
}

// Close closes the publisher and stops its reconnect watcher.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.closed:
	default:
		close(p.closed)
	}

	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

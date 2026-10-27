package worker

import (
	"sort"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"go.uber.org/zap"
)

// notificationQueueByExchange maps each source exchange to the single queue this
// service consumes it from.
//
// One queue per source exchange — NOT one per notification type. The previous
// design derived the queue name from notification_templates.queue_name, which made
// "add a row to a data table" silently imply "provision a queue": that is the bug
// class behind #1795, and the only reason the infra/mq codegen existed. The queue
// set now lives here in code, so the template table is pure content — adding,
// editing or removing a template can never require an infra change.
//
// Queues are bound with "#" (see buildDynamicSubscriptions), so a template using a
// routing key nobody bound before is still delivered. The cost is receiving events
// this service has no template for; those are dropped after a map lookup, before
// payload decode, so they are cheap. If one exchange ever proves too noisy, tighten
// just that binding to literal routing keys in infra/mq — a per-exchange,
// reversible decision that does not affect the table design.
//
// Keys must stay in sync with infra/mq/topology/topology.yaml: adding an exchange
// here means adding a queue entry there in the same PR, with a
// `retry: {max_attempts: 3}` policy and a `{exchange: <exchange>, key: "#"}`
// binding — do not hand-write the derived .dlq queue, .retry exchange, or their
// bindings, the policy expands into all of those (see infra/mq/README.md).
//
// `hawkr.notification` is deliberately absent: it is this service's own
// publish target (notification.inbox.new / notification.unread_changed, consumed
// by gateway's SSE fanout), so consuming it would be self-consumption. Its only
// former queue `notification.system` carried three ghost templates that no
// publisher ever fed; both the templates and the queue were removed together
// (20260822130000). Platform announcements now have exactly one writer, the
// BroadcastAnnouncement RPC, which never touches MQ.
var notificationQueueByExchange = map[string]string{
	"aftersales.events":    "notification.aftersales",
	"hawkr.buyer":          "notification.buyer",
	"hawkr.content":        "notification.content",
	"hawkr.delegation":     "notification.delegation",
	"hawkr.im":             "notification.im",
	"hawkr.invoice":        "notification.invoice",
	"hawkr.marketing":      "notification.marketing",
	"hawkr.merchant":       "notification.merchant",
	"hawkr.order":          "notification.order",
	"hawkr.product":        "notification.product",
	"hawkr.review":         "notification.review",
	"hawkr.seller":         "notification.seller",
	"hawkr.shipping":       "notification.shipping",
	"payment.events":       "notification.payment",
	mq.ExchangeProcurement: "notification.procurement",
}

// buildDynamicSubscriptions returns one mq.Subscription per entry in
// notificationQueueByExchange. The set is derived from that map rather than from
// the template cache, so it is identical on every boot regardless of which
// templates happen to be enabled — and a template added later for an
// already-mapped exchange starts working with no topology change at all.
//
// Each handler dispatches by (exchange, routing key); see newTemplateHandler.
func buildDynamicSubscriptions(
	cache *TemplateCache,
	sendPush *usecase.SendPushUseCase,
	inboxUC *usecase.NotificationInboxUseCase,
	mediaBase string,
	logger *zap.Logger,
	vendorInvite vendorInviteDeps,
) []mq.Subscription {
	// Loud, not silent: a template on an exchange with no queue mapped here can
	// never be delivered. Under passive declare the broker raises nothing — the
	// subscription simply does not exist — so this log is the only place that
	// failure surfaces. The fix is to add the exchange here AND its queue entry
	// (with a retry: policy and a "#" binding) to infra/mq/topology/topology.yaml.
	for _, tmpl := range cache.All() {
		if _, ok := notificationQueueByExchange[tmpl.Exchange]; !ok {
			logger.Error("[template] exchange 未登記於 notificationQueueByExchange，該 template 永遠不會被消費",
				zap.String("exchange", tmpl.Exchange),
				zap.String("routing_key", tmpl.RoutingKey),
				zap.String("event_type", tmpl.EventType))
		}
	}

	exchanges := make([]string, 0, len(notificationQueueByExchange))
	for ex := range notificationQueueByExchange {
		exchanges = append(exchanges, ex)
	}
	sort.Strings(exchanges) // deterministic subscription order

	subs := make([]mq.Subscription, 0, len(exchanges))
	for _, ex := range exchanges {
		handler := newTemplateHandler(cache, sendPush, inboxUC, ex, mediaBase, logger)
		if ex == mq.ExchangeProcurement {
			handler = withVendorInvite(handler, newVendorInviteHandler(vendorInvite))
		}
		subs = append(subs, mq.Subscription{
			Exchange:    ex,
			Queue:       notificationQueueByExchange[ex],
			RoutingKeys: []string{"#"},
			Handler:     handler,
			MaxRetries:  3,
		})
	}
	return subs
}

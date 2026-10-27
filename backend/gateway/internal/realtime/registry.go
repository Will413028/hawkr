// Package realtime houses the Channel registry that bridges MQ events to the
// generic SSE Hub. Each Channel defines:
//   - which routing keys it subscribes to
//   - how to resolve a topic from a routing key + payload
//   - how to build the outgoing SSE event (name + data) from the payload
//   - subscribe-time authorization
//
// New channels (e.g. inventory:seller:{id}) plug in here without touching
// the Hub or MQ consumer. 41 號設計書 §二.4 / §十一。
package realtime

import (
	"context"
	"errors"
	"sync"

	"github.com/Will413028/hawkr/gateway/internal/sse"
	"go.uber.org/zap"
)

// Channel is the contract a realtime channel must satisfy。
type Channel interface {
	Name() string
	RoutingKeys() []string
	Exchange() string

	// ResolveTopic returns the SSE topic for an MQ message, or ("", false)
	// if the channel cannot map this message (skip)。
	ResolveTopic(routingKey string, payload []byte) (string, bool)

	// BuildSSEEvent returns the outgoing SSE event name and JSON data bytes。
	// Returning ("", nil, nil) means "skip — do not publish"（例如 channel
	// 想丟掉某些 routing key 的特定 payload）。
	BuildSSEEvent(routingKey string, payload []byte) (eventName string, data []byte, err error)

	// BatchEventName is the wrapper event name when N>1 events accumulate in
	// the batch window. Return "" to disable batching for this channel。
	BatchEventName() string

	// AuthorizeSubscribe is called before Hub.Subscribe to check whether a
	// caller may subscribe to the given topic. Auth context comes from gin
	// (already extracted into typed args by the handler) — Channel doesn't
	// inspect gin directly。
	AuthorizeSubscribe(ctx context.Context, topic string, accountID int64) error
}

// Registry maps routing keys → channels and exposes Dispatch / Channels APIs
// consumed by the MQ consumer and bootstrap wiring。
type Registry struct {
	mu       sync.RWMutex
	channels []Channel
	byRK     map[string]Channel // exact-match routing key (no wildcard expansion)
	log      *zap.Logger
	hub      *sse.Hub
}

// NewRegistry constructs an empty Registry bound to the given Hub。
func NewRegistry(hub *sse.Hub, log *zap.Logger) *Registry {
	if log == nil {
		log = zap.NewNop()
	}
	return &Registry{
		channels: nil,
		byRK:     make(map[string]Channel),
		log:      log,
		hub:      hub,
	}
}

// Register adds a channel. Routing keys are matched literally when the MQ
// consumer dispatches — wildcards in RoutingKeys() (e.g. "comment.#") are
// for QueueBind, not Dispatch. Resolution of "any comment.*" routing key
// to the comment channel relies on the consumer giving us the full key and
// the channel's ResolveTopic / BuildSSEEvent handling it.
//
// Conflict (two channels registering the same exact routing key) is logged
// and the later registration wins。
func (r *Registry) Register(ch Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels = append(r.channels, ch)
	for _, rk := range ch.RoutingKeys() {
		if _, exists := r.byRK[rk]; exists {
			r.log.Warn("realtime.registry.routing_key_conflict",
				zap.String("routingKey", rk), zap.String("channel", ch.Name()))
		}
		r.byRK[rk] = ch
	}
}

// Channels returns the registered channels (read-only snapshot)。
func (r *Registry) Channels() []Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Channel, len(r.channels))
	copy(out, r.channels)
	return out
}

// Dispatch is invoked by the MQ consumer for every received message。
// Looks up the channel by routing key, resolves topic, builds SSE event,
// and publishes via Hub. Failures are logged best-effort — never returns
// error to the MQ consumer (we ack regardless)。
//
// Routing key matching is exact. If wildcard binding (e.g. "comment.#")
// causes the consumer to receive "comment.created", we look up the literal
// "comment.created" in byRK. Channels must register every concrete routing
// key they want to dispatch — see comment_channel.go for the canonical list.
func (r *Registry) Dispatch(routingKey string, payload []byte) {
	r.mu.RLock()
	ch, ok := r.byRK[routingKey]
	r.mu.RUnlock()
	if !ok {
		r.log.Debug("realtime.dispatch.no_channel", zap.String("routingKey", routingKey))
		return
	}

	topic, ok := ch.ResolveTopic(routingKey, payload)
	if !ok {
		r.log.Debug("realtime.dispatch.resolve_topic_skip",
			zap.String("routingKey", routingKey), zap.String("channel", ch.Name()))
		return
	}

	eventName, data, err := ch.BuildSSEEvent(routingKey, payload)
	if err != nil {
		r.log.Warn("realtime.dispatch.build_event_failed",
			zap.String("routingKey", routingKey),
			zap.String("channel", ch.Name()),
			zap.Error(err))
		return
	}
	if eventName == "" {
		return
	}

	r.hub.Publish(topic, eventName, data, ch.BatchEventName())
}

// AllRoutingKeys returns the union of routing keys across all channels,
// used by the MQ consumer to bind the broadcast queue。
func (r *Registry) AllRoutingKeys() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]struct{})
	out := []string{}
	for _, ch := range r.channels {
		for _, rk := range ch.RoutingKeys() {
			if _, dup := seen[rk]; dup {
				continue
			}
			seen[rk] = struct{}{}
			out = append(out, rk)
		}
	}
	return out
}

// AllExchanges returns the set of distinct exchanges to bind from。
func (r *Registry) AllExchanges() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	seen := make(map[string]struct{})
	out := []string{}
	for _, ch := range r.channels {
		ex := ch.Exchange()
		if _, dup := seen[ex]; dup {
			continue
		}
		seen[ex] = struct{}{}
		out = append(out, ex)
	}
	return out
}

// BindingsByExchange returns exchange → routing keys map, for QueueBind iteration。
func (r *Registry) BindingsByExchange() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string][]string)
	for _, ch := range r.channels {
		ex := ch.Exchange()
		out[ex] = append(out[ex], ch.RoutingKeys()...)
	}
	return out
}

// ErrForbidden is returned by AuthorizeSubscribe when the caller is not allowed
// to subscribe to the requested topic (e.g. JWT accountId != topic accountId)。
var ErrForbidden = errors.New("realtime: forbidden")

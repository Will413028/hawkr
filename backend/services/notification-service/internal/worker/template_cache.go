package worker

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// cacheData is the immutable snapshot stored atomically in TemplateCache.
type cacheData struct {
	// byKey indexes templates by "exchange:routing_key".
	byKey map[string][]*domain.NotificationTemplate
	// all is the flat list of all enabled templates (used to build MQ subscriptions at startup).
	all []*domain.NotificationTemplate
}

// TemplateCache holds all enabled notification templates in memory.
// The internal snapshot is replaced atomically on each reload to avoid locking.
type TemplateCache struct {
	ptr  atomic.Pointer[cacheData]
	repo domain.NotificationTemplateRepository
}

// NewTemplateCache creates a TemplateCache backed by the given repository.
func NewTemplateCache(repo domain.NotificationTemplateRepository) *TemplateCache {
	tc := &TemplateCache{repo: repo}
	empty := &cacheData{
		byKey: make(map[string][]*domain.NotificationTemplate),
	}
	tc.ptr.Store(empty)
	return tc
}

// cacheKey returns the composite key used to index templates.
func cacheKey(exchange, routingKey string) string {
	return fmt.Sprintf("%s:%s", exchange, routingKey)
}

// FindByExchangeAndRoutingKey returns the templates for the given (exchange,
// routing key) pair. Returns nil if no templates are found.
//
// Keyed on exchange rather than queue since the queue-per-notification-type
// split was collapsed to one queue per source exchange (see
// notificationQueueByExchange). The buyer/seller variants of the same event —
// previously two queues receiving two copies of one message — now resolve to two
// templates behind a single lookup, which the caller already iterates.
func (tc *TemplateCache) FindByExchangeAndRoutingKey(exchange, routingKey string) []*domain.NotificationTemplate {
	data := tc.ptr.Load()
	if data == nil {
		return nil
	}
	return data.byKey[cacheKey(exchange, routingKey)]
}

// All returns a snapshot of all enabled templates currently in cache.
// Used at startup to build MQ subscriptions.
func (tc *TemplateCache) All() []*domain.NotificationTemplate {
	data := tc.ptr.Load()
	if data == nil {
		return nil
	}
	return data.all
}

// LoadFromDB reloads all enabled templates from the database and atomically swaps the cache.
// Returns the number of templates loaded.
func (tc *TemplateCache) LoadFromDB(ctx context.Context) (int, error) {
	templates, err := tc.repo.FindEnabled(ctx)
	if err != nil {
		return 0, fmt.Errorf("template cache reload: %w", err)
	}

	byKey := make(map[string][]*domain.NotificationTemplate, len(templates))
	for _, t := range templates {
		key := cacheKey(t.Exchange, t.RoutingKey)
		byKey[key] = append(byKey[key], t)
	}

	tc.ptr.Store(&cacheData{byKey: byKey, all: templates})
	return len(templates), nil
}

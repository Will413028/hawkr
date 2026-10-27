package mq

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// BindTypedHandler adapts a typed event handler into the HandlerFunc shape
// Subscription.Handler expects (see resilient.go), doing the
// json.Unmarshal(body, &event) + error-wrap boilerplate every hand-rolled
// consumer in this repo repeats today: var payload T; json.Unmarshal(body,
// &payload); if err { log + wrap }.
//
// fn receives a non-nil *T only after a successful unmarshal. Field-level
// validation (e.g. rejecting a zero ID) stays in fn — this helper only owns
// decode, not business validation.
func BindTypedHandler[T any](logger *zap.Logger, fn func(ctx context.Context, routingKey string, event *T) error) HandlerFunc {
	return func(ctx context.Context, routingKey string, body []byte) error {
		var event T
		if err := json.Unmarshal(body, &event); err != nil {
			logger.Error("failed to unmarshal event", zap.String("routing_key", routingKey), zap.Error(err))
			return fmt.Errorf("unmarshal event (routing key %q): %w", routingKey, err)
		}
		return fn(ctx, routingKey, &event)
	}
}

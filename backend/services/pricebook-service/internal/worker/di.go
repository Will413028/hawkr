package worker

import (
	"context"
	"sync"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("worker", fx.Invoke(startConsumer))

func sellerDeletedSubscription(repo domain.MarketWatchCleanupRepository, logger *zap.Logger) mq.Subscription {
	return mq.Subscription{
		Exchange:    mq.ExchangeSeller,
		Queue:       mq.QueuePricebookSellerDeleted,
		RoutingKeys: []string{mq.RoutingKeySellerDeleted},
		Handler:     newSellerDeletedHandler(repo, logger),
		MaxRetries:  3,
	}
}

// Dependency construction registers pool and telemetry hooks before this hook;
// Fx stops in reverse order, so the consumer drains before either dependency.
func startConsumer(lc fx.Lifecycle, cfg *config.Config, repo domain.MarketWatchCleanupRepository, logger *zap.Logger, meter *sdkmetric.MeterProvider) {
	if cfg.RabbitMQ.URL == "" {
		logger.Warn("RABBITMQ_URL not configured; skipping pricebook seller.deleted consumer")
		return
	}
	drain := &handlerDrain{}
	subscription := sellerDeletedSubscription(repo, logger)
	subscription.Handler = drain.wrap(subscription.Handler)
	consumer := mq.NewResilientConsumer(cfg.RabbitMQ.URL,
		[]mq.Subscription{subscription}, mq.WithOTelMetrics(meter))
	registerConsumer(lc, consumer, drain)
}

type runner interface{ Run(context.Context) }

func registerConsumer(lc fx.Lifecycle, consumer runner, drain *handlerDrain) {
	var cancel context.CancelFunc
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			ctx, stop := context.WithCancel(context.Background())
			cancel = stop
			go func() {
				defer close(done)
				consumer.Run(ctx)
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			drain.stopAdmission()
			cancel()
			stopped := make(chan struct{})
			go func() {
				<-done
				drain.active.Wait()
				close(stopped)
			}()
			select {
			case <-stopped:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
}

// The shared consumer closes its connection when Run exits but does not join
// delivery goroutines. Guard admission and join our active DB handlers locally.
type handlerDrain struct {
	mu       sync.Mutex
	stopping bool
	active   sync.WaitGroup
}

func (d *handlerDrain) stopAdmission() {
	d.mu.Lock()
	d.stopping = true
	d.mu.Unlock()
}

func (d *handlerDrain) wrap(handler mq.HandlerFunc) mq.HandlerFunc {
	return func(ctx context.Context, key string, body []byte) error {
		d.mu.Lock()
		if d.stopping {
			d.mu.Unlock()
			return context.Canceled
		}
		d.active.Add(1)
		d.mu.Unlock()
		defer d.active.Done()
		return handler(ctx, key, body)
	}
}

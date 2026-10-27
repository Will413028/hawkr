package worker

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type runnerFunc func(context.Context)

func (f runnerFunc) Run(ctx context.Context) { f(ctx) }

func TestSellerDeletedSubscription(t *testing.T) {
	sub := sellerDeletedSubscription(&cleanupStub{}, zap.NewNop())
	require.Equal(t, mq.ExchangeSeller, sub.Exchange)
	require.Equal(t, "pricebook.seller.deleted", sub.Queue)
	require.Equal(t, []string{mq.RoutingKeySellerDeleted}, sub.RoutingKeys)
	require.Equal(t, 3, sub.MaxRetries)
	require.NotNil(t, sub.Handler)
}

func TestConsumerLifecycleSurvivesStartupAndDrainsBeforePool(t *testing.T) {
	runCtx := make(chan context.Context, 1)
	runExit := make(chan struct{})
	allowRunExit := make(chan struct{})
	handlerEntered := make(chan struct{})
	allowHandlerExit := make(chan struct{})
	handlerDone := make(chan error, 1)
	poolClosed := make(chan struct{})
	drain := &handlerDrain{}
	handler := drain.wrap(func(context.Context, string, []byte) error {
		close(handlerEntered)
		<-allowHandlerExit
		return nil
	})
	consumer := runnerFunc(func(ctx context.Context) {
		runCtx <- ctx
		<-ctx.Done()
		<-allowRunExit
		close(runExit)
	})
	app := fx.New(fx.NopLogger, fx.Invoke(func(lc fx.Lifecycle) {
		lc.Append(fx.Hook{OnStop: func(context.Context) error { close(poolClosed); return nil }})
		registerConsumer(lc, consumer, drain)
	}))
	startup, cancelStart := context.WithCancel(context.Background())
	require.NoError(t, app.Start(startup))
	ctx := <-runCtx
	cancelStart()
	require.NoError(t, ctx.Err(), "startup context must not own the running consumer")
	go func() { handlerDone <- handler(ctx, mq.RoutingKeySellerDeleted, nil) }()
	<-handlerEntered
	stopDone := make(chan error, 1)
	stopCtx, cancelStop := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelStop()
	go func() { stopDone <- app.Stop(stopCtx) }()
	<-ctx.Done()
	select {
	case <-poolClosed:
		t.Fatal("pool closed before consumer Run exited")
	default:
	}
	close(allowRunExit)
	<-runExit
	select {
	case <-poolClosed:
		t.Fatal("pool closed before active handler exited")
	default:
	}
	require.ErrorIs(t, handler(context.Background(), "", nil), context.Canceled, "shutdown must reject new DB work")
	close(allowHandlerExit)
	require.NoError(t, <-handlerDone)
	require.NoError(t, <-stopDone)
	<-poolClosed
}

type captureLifecycle struct{ hooks []fx.Hook }

func (c *captureLifecycle) Append(h fx.Hook) { c.hooks = append(c.hooks, h) }

func TestUnconfiguredLocalConsumerDoesNotRegisterLifecycle(t *testing.T) {
	lc := &captureLifecycle{}
	startConsumer(lc, &config.Config{}, &cleanupStub{}, zap.NewNop(), nil)
	require.Empty(t, lc.hooks)
}

func TestConsumerStopHonorsDeadline(t *testing.T) {
	lc := &captureLifecycle{}
	release := make(chan struct{})
	registerConsumer(lc, runnerFunc(func(context.Context) { <-release }), &handlerDrain{})
	require.NoError(t, lc.hooks[0].OnStart(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, lc.hooks[0].OnStop(ctx), context.Canceled)
	close(release)
}

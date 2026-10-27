package interceptor

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"google.golang.org/grpc/stats"
)

func TestOTelServerStatsHandler_DefaultDoesNotEmitDirectMetrics(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetMeterProvider(previous)
	})

	recordTestRPC(OTelServerStatsHandler())

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect default handler metrics: %v", err)
	}
	if len(metrics.ScopeMetrics) != 0 {
		t.Fatalf("default stats handler emitted direct metrics: %#v", metrics.ScopeMetrics)
	}
}

func TestOTelServerStatsHandlerWithMetrics_EmitsDirectMetrics(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetMeterProvider(previous)
	})

	recordTestRPC(OTelServerStatsHandlerWithMetrics())

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect opt-in handler metrics: %v", err)
	}
	if len(metrics.ScopeMetrics) == 0 {
		t.Fatal("metrics-enabled stats handler emitted no direct metrics")
	}
}

func recordTestRPC(handler stats.Handler) {
	ctx := handler.TagRPC(context.Background(), &stats.RPCTagInfo{
		FullMethodName: "/hawkr.test.TestService/Check",
	})
	now := time.Now()
	handler.HandleRPC(ctx, &stats.End{
		BeginTime: now.Add(-time.Millisecond),
		EndTime:   now,
	})
}

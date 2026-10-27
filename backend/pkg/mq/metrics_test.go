package mq

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestResilientConsumer_OptionalOTelMetricsRecordsAllOutcomes(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	consumer := NewResilientConsumer("amqp://unused", nil, WithOTelMetrics(provider))
	for _, result := range []string{"ack", "retry", "dlq", "requeue"} {
		consumer.metrics.Inc(context.Background(), "notification.test", result)
	}

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &metrics); err != nil {
		t.Fatalf("collect MQ OTel metrics: %v", err)
	}

	seen := map[string]int64{}
	for _, scope := range metrics.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "mq_consume_total" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("mq_consume_total data type = %T, want Sum[int64]", m.Data)
			}
			for _, point := range sum.DataPoints {
				queue, queueOK := point.Attributes.Value("queue")
				result, resultOK := point.Attributes.Value("result")
				if queueOK && resultOK && queue.AsString() == "notification.test" {
					seen[result.AsString()] = point.Value
				}
			}
		}
	}
	for _, result := range []string{"ack", "retry", "dlq", "requeue"} {
		if seen[result] != 1 {
			t.Errorf("mq_consume_total result %q = %d, want 1", result, seen[result])
		}
	}
}

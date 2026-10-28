package otel

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	metricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricdatapb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func TestConfiguredOTLPEndpoint_PrefersSignalSpecificValue(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "common:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "metrics:4317")

	if got, want := configuredOTLPEndpoint(), "metrics:4317"; got != want {
		t.Fatalf("configured endpoint = %q, want signal-specific %q", got, want)
	}
}

func TestConfiguredOTLPEndpoint_FallsBackToCommonValue(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "common:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	if got, want := configuredOTLPEndpoint(), "common:4317"; got != want {
		t.Fatalf("configured endpoint = %q, want common %q", got, want)
	}
}

func TestNormalizeOTLPEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "host and port", endpoint: "otel-collector:4317", want: "http://otel-collector:4317"},
		{name: "http URL", endpoint: "http://otel-collector:4317", want: "http://otel-collector:4317"},
		{name: "https URL", endpoint: "https://otel-collector:4317", want: "https://otel-collector:4317"},
		{name: "unix URL", endpoint: "unix:///var/run/otel.sock", want: "unix:///var/run/otel.sock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeOTLPEndpoint(tc.endpoint); got != tc.want {
				t.Fatalf("normalizeOTLPEndpoint(%q) = %q, want %q", tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestNewMeterProvider_WithoutEndpointDoesNotCreateExporter(t *testing.T) {
	called := false
	provider := newMeterProvider(
		context.Background(),
		"test-service",
		"",
		zap.NewNop(),
		func(context.Context, string) (metric.Exporter, error) {
			called = true
			return nil, errors.New("exporter must not be created without an endpoint")
		},
	)

	if called {
		t.Fatal("exporter factory was called without a configured endpoint")
	}
	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush without a reader: %v", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown without a reader: %v", err)
	}
}

func TestNewMeterProvider_ExporterConstructionFailureIsFailSoft(t *testing.T) {
	provider := newMeterProvider(
		context.Background(),
		"test-service",
		"http://collector:4317",
		zap.NewNop(),
		func(context.Context, string) (metric.Exporter, error) {
			return nil, errors.New("collector configuration is invalid")
		},
	)

	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush after exporter construction failure: %v", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown after exporter construction failure: %v", err)
	}
}

// TestNewMeterProvider_ExportsServiceInstanceID 證明 service.instance.id 真的隨 OTLP
// 送到 collector，而不只是掛在 resource 物件上。
//
// 這是 `instance` label 的唯一來源：prometheusremotewrite 對 OTLP resource 只特別處理
// service.name（→ job）與 service.instance.id（→ instance），其餘走 target_info，而本
// 叢集的 collector 設定是 target_info.enabled=false。少了它，同一個服務的每個 pod 在
// Prometheus 裡是同一條 series，rate()/increase() 靜默灌水。
//
// 既有的 TestNewMeterProvider_ExportsOTLPMetricWithResourceIdentity 只斷言 service.name
// 與 deployment.environment，對這個屬性完全無感。
func TestNewMeterProvider_ExportsServiceInstanceID(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("POD_NAME", "review-service-5d7c9f8b64-x2q7z")
	collector := newMetricsCollector(t)
	provider := newMeterProvider(
		context.Background(),
		"review-service",
		"http://"+collector.address,
		zap.NewNop(),
		newOTLPMetricExporter,
	)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	meter := provider.Meter("github.com/Will413028/hawkr/go-pkg/otel/test")
	counter, err := meter.Int64Counter("review_service_instance_probe_total")
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	counter.Add(context.Background(), 1)

	flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := provider.ForceFlush(flushCtx); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}

	select {
	case request := <-collector.requests:
		assertResourceAttribute(t, request.GetResourceMetrics(), "service.instance.id", "review-service-5d7c9f8b64-x2q7z")
	case <-flushCtx.Done():
		t.Fatal("timed out waiting for fake Collector to receive OTLP metrics")
	}
}

func TestNewMeterProvider_ExportsOTLPMetricWithResourceIdentity(t *testing.T) {
	t.Setenv("ENV", "")
	collector := newMetricsCollector(t)
	provider := newMeterProvider(
		context.Background(),
		"review-service",
		"http://"+collector.address,
		zap.NewNop(),
		newOTLPMetricExporter,
	)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	meter := provider.Meter("github.com/Will413028/hawkr/go-pkg/otel/test")
	counter, err := meter.Int64Counter("review_service_bootstrap_total")
	if err != nil {
		t.Fatalf("create counter: %v", err)
	}
	counter.Add(context.Background(), 1)

	flushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := provider.ForceFlush(flushCtx); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}

	select {
	case request := <-collector.requests:
		assertResourceAttribute(t, request.GetResourceMetrics(), "service.name", "review-service")
		assertResourceAttribute(t, request.GetResourceMetrics(), "deployment.environment", "development")
		if !containsMetric(request.GetResourceMetrics(), "review_service_bootstrap_total") {
			t.Fatalf("exported metrics do not contain review_service_bootstrap_total: %v", request.GetResourceMetrics())
		}
	case <-flushCtx.Done():
		t.Fatal("timed out waiting for fake Collector to receive OTLP metrics")
	}
}

func TestInitMeterProvider_RegistersGlobalProvider(t *testing.T) {
	previous := otel.GetMeterProvider()
	t.Cleanup(func() { otel.SetMeterProvider(previous) })
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	provider, shutdown := InitMeterProvider("test-service", zap.NewNop())
	if provider == nil {
		t.Fatal("InitMeterProvider returned nil provider")
	}
	if got := otel.GetMeterProvider(); got != provider {
		t.Fatalf("global meter provider = %T, want the initialized provider %T", got, provider)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestValidateMetricsConfiguration_RequiresValidEndpointInProduction(t *testing.T) {
	for _, tc := range []struct {
		name     string
		env      string
		endpoint string
		wantErr  bool
	}{
		{name: "production requires endpoint", env: "production", wantErr: true},
		{name: "prod requires endpoint", env: "prod", wantErr: true},
		{name: "development allows no endpoint", env: "development", wantErr: false},
		{name: "production accepts host port", env: "production", endpoint: "otel-collector:4317", wantErr: false},
		{name: "production accepts https", env: "production", endpoint: "https://collector.example:4317", wantErr: false},
		{name: "production rejects malformed URL", env: "production", endpoint: "http://", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", tc.endpoint)
			t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
			if err := ValidateMetricsConfiguration(tc.env); (err != nil) != tc.wantErr {
				t.Fatalf("ValidateMetricsConfiguration(%q) error = %v, wantErr %v", tc.env, err, tc.wantErr)
			}
		})
	}
}

type metricsCollector struct {
	metricpb.UnimplementedMetricsServiceServer
	address  string
	requests chan *metricpb.ExportMetricsServiceRequest
}

func newMetricsCollector(t *testing.T) *metricsCollector {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for fake Collector: %v", err)
	}

	collector := &metricsCollector{
		address:  listener.Addr().String(),
		requests: make(chan *metricpb.ExportMetricsServiceRequest, 1),
	}
	server := grpc.NewServer()
	metricpb.RegisterMetricsServiceServer(server, collector)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return collector
}

func (c *metricsCollector) Export(_ context.Context, request *metricpb.ExportMetricsServiceRequest) (*metricpb.ExportMetricsServiceResponse, error) {
	c.requests <- request
	return &metricpb.ExportMetricsServiceResponse{}, nil
}

func assertResourceAttribute(t *testing.T, resources []*metricdatapb.ResourceMetrics, key, want string) {
	t.Helper()
	for _, resourceMetrics := range resources {
		for _, attribute := range resourceMetrics.GetResource().GetAttributes() {
			if attribute.GetKey() == key && attribute.GetValue().GetStringValue() == want {
				return
			}
		}
	}
	t.Fatalf("resource attribute %s=%q not found in %v", key, want, resources)
}

func containsMetric(resources []*metricdatapb.ResourceMetrics, want string) bool {
	for _, resourceMetrics := range resources {
		for _, scopeMetrics := range resourceMetrics.GetScopeMetrics() {
			for _, metric := range scopeMetrics.GetMetrics() {
				if metric.GetName() == want {
					return true
				}
			}
		}
	}
	return false
}

// TestNewMeterProvider_ExportsPrometheusDefaultRegistry 釘住「prometheus 預設
// registry 的指標會經由同一條 OTLP 管線送出」。
//
// 這條的存在理由是一次真實的失聯：gRPC-only 遷移把 9940 的 HTTP bootstrap 連同
// /metrics 一起移除之後，promauto 註冊的指標就再也沒有出口——OTel 的
// MeterProvider 只讀自己的 instrument，不會去看 prometheus 的 registry。
// 2026-09-07 在 dev 上實測：identity 模組的 auth_refresh_total 在剛被大量
// 增量之後，Prometheus 裡 30 天內 0 個 series，而同一個 process 的
// server_start_total（OTel instrument）一直都在。
//
// 鑑別力：這支測試用的是 prometheus.DefaultRegisterer，也就是 promauto 在
// production 用的那一個。拿掉 newMeterProvider 裡的 WithProducer 就會轉紅；
// 既有的 TestNewMeterProvider_ExportsOTLPMetricWithResourceIdentity 用的是 OTel
// 自己的 instrument，兩種寫法它都綠——所以它證不到這件事。
func TestNewMeterProvider_ExportsPrometheusDefaultRegistry(t *testing.T) {
	t.Setenv("ENV", "")

	const name = "go_pkg_otel_bridge_probe_total"
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: "probe counter proving the prometheus default registry is bridged into OTLP",
	})
	if err := prometheus.DefaultRegisterer.Register(counter); err != nil {
		t.Fatalf("register probe counter: %v", err)
	}
	t.Cleanup(func() { prometheus.DefaultRegisterer.Unregister(counter) })
	counter.Inc()

	collector := newMetricsCollector(t)
	provider := newMeterProvider(
		context.Background(),
		"account-service",
		"http://"+collector.address,
		zap.NewNop(),
		newOTLPMetricExporter,
	)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := provider.ForceFlush(flushCtx); err != nil {
		t.Fatalf("ForceFlush: %v", err)
	}

	for {
		select {
		case request := <-collector.requests:
			if containsMetric(request.GetResourceMetrics(), name) {
				return
			}
		case <-flushCtx.Done():
			t.Fatalf("prometheus 預設 registry 的 %s 沒有出現在 OTLP payload 裡——"+
				"promauto 指標離不開 process，維運端全黑", name)
		}
	}
}

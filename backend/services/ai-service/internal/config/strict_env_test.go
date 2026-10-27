package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func captureWarnings(t *testing.T, fn func()) string {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	defer slog.SetDefault(prev)

	fn()

	return buf.String()
}

func TestWarnUnknownEnv_KnownExactNameIsSilent(t *testing.T) {
	t.Setenv("DB_HOST", "postgres")
	t.Setenv("GRPC_PORT", "9939")

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if strings.Contains(out, "DB_HOST") || strings.Contains(out, "GRPC_PORT") {
		t.Fatalf("known env vars must not be warned about, got: %q", out)
	}
}

func TestWarnUnknownEnv_FallbackChainBothNamesAreSilent(t *testing.T) {

	t.Setenv("DB_POOL_PORT", "6432")
	t.Setenv("DB_PORT", "5432")

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if strings.Contains(out, "DB_POOL_PORT") || strings.Contains(out, "DB_PORT") {
		t.Fatalf("both names of a fallback chain must be silent, got: %q", out)
	}
}

func TestWarnUnknownEnv_InFamilyUnknownNameWarns(t *testing.T) {

	t.Setenv("DB_POOL_PROT", "6432")

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if !strings.Contains(out, "DB_POOL_PROT") {
		t.Fatalf("expected a warning naming DB_POOL_PROT, got: %q", out)
	}
}

func TestWarnUnknownEnv_OutOfFamilyVariableIsSilent(t *testing.T) {

	t.Setenv("REDIS_HOST", "redis")

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if strings.Contains(out, "REDIS_HOST") {
		t.Fatalf("out-of-family env vars must not be warned about, got: %q", out)
	}
}

func TestWarnUnknownEnv_TypoInFamilyWarns(t *testing.T) {

	t.Setenv("GEMINI_APIKEY", "leaked-would-be-ignored-silently")

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if !strings.Contains(out, "GEMINI_APIKEY") {
		t.Fatalf("expected a warning naming the typo'd GEMINI_APIKEY, got: %q", out)
	}
}

func TestWarnUnknownEnv_K8sServiceLinkVarsForOwnedFamilyAreSilent(t *testing.T) {
	k8sInjected := []string{
		"ORDER_SERVICE_SERVICE_HOST",
		"ORDER_SERVICE_SERVICE_PORT",
		"ORDER_SERVICE_SERVICE_PORT_GRPC",
		"ORDER_SERVICE_PORT",
		"ORDER_SERVICE_PORT_9915_TCP",
		"ORDER_SERVICE_PORT_9915_TCP_PROTO",
		"ORDER_SERVICE_PORT_9915_TCP_PORT",
		"ORDER_SERVICE_PORT_9915_TCP_ADDR",
	}
	for _, name := range k8sInjected {
		t.Setenv(name, "x")
	}

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	for _, name := range k8sInjected {
		if strings.Contains(out, name) {
			t.Errorf("k8s-service-link-shaped variable %s must not be warned about, got: %q", name, out)
		}
	}
}

func TestWarnUnknownEnv_RealTypoInSixServiceFamilyStillWarns(t *testing.T) {
	t.Setenv("ORDER_SERVICE_GRPCPORT", "9915")

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if !strings.Contains(out, "ORDER_SERVICE_GRPCPORT") {
		t.Fatalf("expected a warning naming the typo'd ORDER_SERVICE_GRPCPORT, got: %q", out)
	}
}

var devRenderedEnvNames = []string{

	"BUYER_SERVICE_GRPC_PORT", "BUYER_SERVICE_HOST", "DB_HOST", "DB_NAME", "DB_PORT",
	"ENV", "EXTRACTOR_MODEL", "GONOSUMDB", "GOPROXY", "GRPC_PORT", "INTERNAL_AUTH_MODE",
	"MEDIA_SERVICE_GRPC_PORT", "MEDIA_SERVICE_HOST", "MQ_PASSIVE_DECLARE",
	"ORDER_SERVICE_GRPC_PORT", "ORDER_SERVICE_HOST", "OTEL_EXPORTER_OTLP_ENDPOINT",
	"OTEL_RESOURCE_ATTRIBUTES", "PRICEBOOK_SERVICE_GRPC_PORT", "PRICEBOOK_SERVICE_HOST",
	"PRODUCT_SERVICE_GRPC_PORT", "PRODUCT_SERVICE_HOST", "RABBITMQ_HOST", "RABBITMQ_PORT",
	"RABBITMQ_VHOST", "REDIS_HOST", "REDIS_PORT", "SELLER_SERVICE_GRPC_PORT",
	"SELLER_SERVICE_HOST",

	"INTERNAL_SERVICE_HMAC_SECRET", "INTERNAL_SERVICE_HMAC_SECRET_PREV", "DB_USER",
	"DB_PASSWORD", "REDIS_PASSWORD", "RABBITMQ_USERNAME", "RABBITMQ_PASSWORD",
	"RABBITMQ_URL", "GEMINI_API_KEY", "DEEPSEEK_API_KEY", "OPENROUTER_API_KEY",
}

var prodRenderedEnvNames = append(append([]string{}, devRenderedEnvNames...),
	"DB_POOL_MAX_CONNS", "DB_POOL_PORT", "DB_SSLMODE", "MEDIA_BASE_URL", "TRACING_SAMPLING",
)

func TestWarnUnknownEnv_DevRenderedSetWarnsZero(t *testing.T) {
	for _, name := range devRenderedEnvNames {
		t.Setenv(name, "x")
	}

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if out != "" {
		t.Fatalf("dev's real rendered env set must warn zero times, got: %q", out)
	}
}

func TestWarnUnknownEnv_ProdRenderedSetWarnsZero(t *testing.T) {
	for _, name := range prodRenderedEnvNames {
		t.Setenv(name, "x")
	}

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if out != "" {
		t.Fatalf("prod's real rendered env set must warn zero times, got: %q", out)
	}
}

func TestWarnUnknownEnv_TypoAmongProdRenderedSetStillWarns(t *testing.T) {
	for _, name := range prodRenderedEnvNames {
		t.Setenv(name, "x")
	}
	t.Setenv("GRPC_PROT", "9939")

	out := captureWarnings(t, func() { WarnUnknownEnv(mustLoad(t)) })

	if !strings.Contains(out, "GRPC_PROT") {
		t.Fatalf("expected a warning naming the typo'd GRPC_PROT among the real prod set, got: %q", out)
	}
}

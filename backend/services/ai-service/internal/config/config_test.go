package config

import "testing"

func mustLoad(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() 非預期 error: %v", err)
	}
	return cfg
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("GRPC_PORT", "")

	cfg := mustLoad(t)
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}

	if cfg.Env != "development" {
		t.Errorf("Env = %q, want %q", cfg.Env, "development")
	}
	if cfg.Server.GRPCPort != "9939" {
		t.Errorf("Server.GRPCPort = %q, want %q", cfg.Server.GRPCPort, "9939")
	}
	if cfg.LLMApiURL != "https://api.openai.com/v1" {
		t.Errorf("LLMApiURL = %q, want %q", cfg.LLMApiURL, "https://api.openai.com/v1")
	}
	if cfg.LLMModel != "gpt-3.5-turbo" {
		t.Errorf("LLMModel = %q, want %q", cfg.LLMModel, "gpt-3.5-turbo")
	}
	if cfg.GeminiModel != "gemini-2.5-flash" {
		t.Errorf("GeminiModel = %q, want %q", cfg.GeminiModel, "gemini-2.5-flash")
	}

	if cfg.ExtractorModel != "" {
		t.Errorf("ExtractorModel = %q, want empty（未設定時交給 llmprovider 套預設）", cfg.ExtractorModel)
	}
	if cfg.OrderServiceHost != "127.0.0.1" {
		t.Errorf("OrderServiceHost = %q, want %q", cfg.OrderServiceHost, "127.0.0.1")
	}
	if cfg.OrderServicePort != "9915" {
		t.Errorf("OrderServicePort = %q, want %q", cfg.OrderServicePort, "9915")
	}
	if cfg.ProductServiceHost != "127.0.0.1" {
		t.Errorf("ProductServiceHost = %q, want %q", cfg.ProductServiceHost, "127.0.0.1")
	}
	if cfg.ProductServicePort != "9913" {
		t.Errorf("ProductServicePort = %q, want %q", cfg.ProductServicePort, "9913")
	}
	if cfg.SellerServiceHost != "127.0.0.1" {
		t.Errorf("SellerServiceHost = %q, want %q", cfg.SellerServiceHost, "127.0.0.1")
	}
	if cfg.SellerServicePort != "9941" {
		t.Errorf("SellerServicePort = %q, want %q", cfg.SellerServicePort, "9941")
	}
	if cfg.BuyerServiceHost != "127.0.0.1" {
		t.Errorf("BuyerServiceHost = %q, want %q", cfg.BuyerServiceHost, "127.0.0.1")
	}
	if cfg.BuyerServicePort != "9941" {
		t.Errorf("BuyerServicePort = %q, want %q", cfg.BuyerServicePort, "9941")
	}
}

func TestLoad_UsesFinalRuntimeEnvironmentAndIgnoresLegacyPortInputs(t *testing.T) {
	t.Setenv("ENV", "staging")
	t.Setenv("APP_ENV", "legacy")
	t.Setenv("PORT", "19938")
	t.Setenv("GRPC_PORT", "19939")

	cfg := mustLoad(t)
	if cfg.Env != "staging" {
		t.Fatalf("Env = %q, want ENV=staging", cfg.Env)
	}
	if cfg.Server.GRPCPort != "19939" {
		t.Fatalf("Server.GRPCPort = %q, want 19939", cfg.Server.GRPCPort)
	}
}

func TestLoad_ExtractorModelOverride(t *testing.T) {
	t.Setenv("EXTRACTOR_MODEL", "deepseek-flash")
	cfg := mustLoad(t)
	if cfg.ExtractorModel != "deepseek-flash" {
		t.Errorf("ExtractorModel = %q, want %q", cfg.ExtractorModel, "deepseek-flash")
	}
}

func TestLoadValidated_RequiresOTLPMetricsInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	if _, err := LoadValidated(); err == nil {
		t.Fatal("LoadValidated should reject production without an OTLP metrics endpoint")
	}
}

func TestLoadValidated_AllowsDevelopmentWithoutOTLPMetrics(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	if _, err := LoadValidated(); err != nil {
		t.Fatalf("LoadValidated should allow development without an OTLP metrics endpoint: %v", err)
	}
}

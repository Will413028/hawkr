package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// MEDIA_BASE_URL 無預設域名且缺值即 fail-fast（backend#2283），測試須自行注入。
	t.Setenv("MEDIA_BASE_URL", "https://test-media.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Env != "development" {
		t.Errorf("Env = %q, want %q", cfg.Env, "development")
	}
	if cfg.GRPCPort != "9933" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9933")
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL should not be empty with defaults")
	}
	if !strings.Contains(cfg.DatabaseURL, "localhost") {
		t.Errorf("DatabaseURL should contain localhost: %s", cfg.DatabaseURL)
	}
	if !strings.Contains(cfg.DatabaseURL, "hawkr_im") {
		t.Errorf("DatabaseURL should contain hawkr_im: %s", cfg.DatabaseURL)
	}
	if cfg.BuyerServiceAddr != "127.0.0.1:9941" {
		t.Errorf("BuyerServiceAddr = %q, want %q", cfg.BuyerServiceAddr, "127.0.0.1:9941")
	}
	if cfg.SellerServiceAddr != "127.0.0.1:9941" {
		t.Errorf("SellerServiceAddr = %q, want %q", cfg.SellerServiceAddr, "127.0.0.1:9941")
	}
	if cfg.NotificationServiceAddr != "notification-service:10037" {
		t.Errorf("NotificationServiceAddr = %q, want %q", cfg.NotificationServiceAddr, "notification-service:10037")
	}
	if cfg.MediaServiceAddr != "media-service:9937" {
		t.Errorf("MediaServiceAddr = %q, want %q", cfg.MediaServiceAddr, "media-service:9937")
	}
	if cfg.MediaBaseURL != "https://test-media.example.com" {
		t.Errorf("MediaBaseURL = %q, want the injected value", cfg.MediaBaseURL)
	}
}

func TestLoad_FinalConfigOmitsLegacyListenerFields(t *testing.T) {
	t.Setenv("MEDIA_BASE_URL", "https://test-media.example.com")
	t.Setenv("GRPC_PORT", "19933")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	typ := reflect.TypeOf(*cfg)
	for _, name := range []string{"HTTPPort", "WSPort"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Errorf("final IM config must not expose legacy %s", name)
		}
	}
	if cfg.GRPCPort != "19933" {
		t.Fatalf("GRPCPort = %q, want environment value 19933", cfg.GRPCPort)
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			DatabaseURL:  "postgres://user:pass@localhost:5432/hawkr_im?sslmode=disable",
			MediaBaseURL: "https://test-media.example.com",
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	// 媒體域名不做新舊相容，缺 MEDIA_BASE_URL 即啟動失敗（backend#2283）。
	t.Run("missing media base url", func(t *testing.T) {
		cfg := &Config{
			DatabaseURL: "postgres://user:pass@localhost:5432/hawkr_im?sslmode=disable",
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing MEDIA_BASE_URL")
		}
		if !strings.Contains(err.Error(), "MEDIA_BASE_URL") {
			t.Errorf("error should mention MEDIA_BASE_URL: %v", err)
		}
	})

	t.Run("missing database url", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing DatabaseURL")
		}
		if !strings.Contains(err.Error(), "DATABASE_URL") {
			t.Errorf("error should mention DATABASE_URL: %v", err)
		}
	})
}

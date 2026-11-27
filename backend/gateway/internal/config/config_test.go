package config

import (
	"strings"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{MediaBaseURL: "https://test-media.example.com"}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("missing MediaBaseURL", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing MEDIA_BASE_URL")
		}
		if !strings.Contains(err.Error(), "MEDIA_BASE_URL") {
			t.Errorf("error should mention MEDIA_BASE_URL: %v", err)
		}
	})
}

func TestConfig_LoadDefaults(t *testing.T) {
	cfg := &Config{
		Env:         "development",
		Port:        "9900",
		DebugPort:   "9901",
		MetricsPort: "9902",
	}

	if cfg.Env != "development" {
		t.Errorf("expected Env=development, got %s", cfg.Env)
	}
	if cfg.Port != "9900" {
		t.Errorf("expected Port=9900, got %s", cfg.Port)
	}
	if cfg.DebugPort != "9901" {
		t.Errorf("expected DebugPort=9901, got %s", cfg.DebugPort)
	}
	if cfg.MetricsPort != "9902" {
		t.Errorf("expected MetricsPort=9902, got %s", cfg.MetricsPort)
	}
}

// 未設 MEDIA_BASE_URL 時**啟動失敗**（backend#2283）：媒體域名不做新舊相容，dev 一律
// test-media.example.com、prod 一律 media.example.com。舊行為是回退
// 一個已退役的舊 OSS 網域，DNS 一撤就靜默吐死連結；回退任何一個
// 域名都是猜，所以直接讓它起不來。
func TestConfig_LoadMediaBaseURLRequired(t *testing.T) {
	t.Setenv("MEDIA_BASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected Load() to fail when MEDIA_BASE_URL is unset, got nil error")
	} else if !strings.Contains(err.Error(), "MEDIA_BASE_URL") {
		t.Fatalf("error should name MEDIA_BASE_URL, got %v", err)
	}
}

func TestConfig_LoadMediaBaseURLOverride(t *testing.T) {
	t.Setenv("MEDIA_BASE_URL", "https://media.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.MediaBaseURL != "https://media.example.com" {
		t.Errorf("expected MediaBaseURL override, got %s", cfg.MediaBaseURL)
	}
}

func TestConfig_TLSEnabled(t *testing.T) {
	t.Run("both cert and key set", func(t *testing.T) {
		cfg := &Config{TLSCert: "/path/to/cert.pem", TLSKey: "/path/to/key.pem"}
		if !cfg.TLSEnabled() {
			t.Error("expected TLSEnabled=true when both cert and key are set")
		}
	})

	t.Run("only cert set", func(t *testing.T) {
		cfg := &Config{TLSCert: "/path/to/cert.pem"}
		if cfg.TLSEnabled() {
			t.Error("expected TLSEnabled=false when only cert is set")
		}
	})

	t.Run("only key set", func(t *testing.T) {
		cfg := &Config{TLSKey: "/path/to/key.pem"}
		if cfg.TLSEnabled() {
			t.Error("expected TLSEnabled=false when only key is set")
		}
	})

	t.Run("neither set", func(t *testing.T) {
		cfg := &Config{}
		if cfg.TLSEnabled() {
			t.Error("expected TLSEnabled=false when neither cert nor key is set")
		}
	})
}

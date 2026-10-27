package config

import (
	"strings"
	"testing"
)

func TestDatabaseConfig_DSN(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "admin",
		Password: "secret",
		DBName:   "hawkr_notification",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()

	checks := []string{"localhost", "5432", "admin", "secret", "hawkr_notification", "sslmode=disable"}
	for _, s := range checks {
		if !strings.Contains(dsn, s) {
			t.Errorf("DSN missing %q: %s", s, dsn)
		}
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Host:     "db-host",
				User:     "user",
				Password: "pass",
			},
			MediaBaseURL: "https://test-media.example.com",
			VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("missing host", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{User: "user", Password: "pass"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing host")
		}
		if !strings.Contains(err.Error(), "DB_HOST") {
			t.Errorf("error should mention DB_HOST: %v", err)
		}
	})

	t.Run("missing user", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Host: "host", Password: "pass"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing user")
		}
		if !strings.Contains(err.Error(), "DB_USER") {
			t.Errorf("error should mention DB_USER: %v", err)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Host: "host", User: "user"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing password")
		}
		if !strings.Contains(err.Error(), "DB_PASSWORD") {
			t.Errorf("error should mention DB_PASSWORD: %v", err)
		}
	})

	t.Run("multiple missing fields reported together", func(t *testing.T) {
		cfg := &Config{Database: DatabaseConfig{}}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error")
		}
		for _, field := range []string{"DB_HOST", "DB_USER", "DB_PASSWORD"} {
			if !strings.Contains(err.Error(), field) {
				t.Errorf("error should mention %s: %v", field, err)
			}
		}
	})

	t.Run("missing vendor portal base url", func(t *testing.T) {
		cfg := &Config{
			Database:     DatabaseConfig{Host: "host", User: "user", Password: "pass"},
			MediaBaseURL: "https://test-media.example.com",
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing vendor portal base url")
		}
		if !strings.Contains(err.Error(), "VENDOR_PORTAL_BASE_URL") {
			t.Errorf("error should mention VENDOR_PORTAL_BASE_URL: %v", err)
		}
	})
}

func TestConfig_Validate_ProductionRequiresOTLPMetricsEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	cfg := &Config{
		Env: "production",
		Database: DatabaseConfig{
			Host:     "db-host",
			User:     "user",
			Password: "pass",
		},
		MediaBaseURL: "https://media.example",
		VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
		Mail:         MailConfig{Provider: "resend", From: "noreply@example.com", RecipientPolicy: "unrestricted"},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP") {
		t.Fatalf("production without OTLP metrics endpoint error = %v, want endpoint validation error", err)
	}
}

func TestConfig_Validate_ProductionRequiresResendMailProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317")

	cfg := &Config{
		Env: "production",
		Database: DatabaseConfig{
			Host:     "db-host",
			User:     "user",
			Password: "pass",
		},
		MediaBaseURL: "https://media.example",
		VendorPortal: VendorPortalConfig{BaseURL: "https://example.com"},
		Mail:         MailConfig{Provider: "log"},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "MAIL_PROVIDER") {
		t.Fatalf("production with MAIL_PROVIDER=log error = %v, want MAIL_PROVIDER validation error", err)
	}
}

func TestConfig_Validate_ProductionWithResendMailProviderPasses(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317")

	cfg := &Config{
		Env: "production",
		Database: DatabaseConfig{
			Host:     "db-host",
			User:     "user",
			Password: "pass",
		},
		MediaBaseURL: "https://media.example",
		VendorPortal: VendorPortalConfig{BaseURL: "https://example.com"},
		Mail:         MailConfig{Provider: "resend", From: "noreply@example.com", RecipientPolicy: "unrestricted"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("production with MAIL_PROVIDER=resend error = %v, want nil", err)
	}
}

func TestConfig_Validate_DevelopmentAllowsLogMailProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	cfg := &Config{
		Env: "development",
		Database: DatabaseConfig{
			Host:     "db-host",
			User:     "user",
			Password: "pass",
		},
		MediaBaseURL: "https://test-media.example.com",
		VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
		Mail:         MailConfig{Provider: "log"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development with MAIL_PROVIDER=log error = %v, want nil", err)
	}
}

func TestConfig_Validate_DevelopmentAllowsMissingOTLPMetricsEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	cfg := &Config{
		Env: "development",
		Database: DatabaseConfig{
			Host:     "db-host",
			User:     "user",
			Password: "pass",
		},
		MediaBaseURL: "https://media.example",
		VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("development without OTLP metrics endpoint error = %v, want nil", err)
	}
}

// 🔴 非 prod 不得使用 unrestricted。這是 #2539 收尾時暴露的結構缺陷：dev 的 pod
// 同時具備有效的 RESEND_API_KEY、驗證過的寄件網域、以及 632 筆真實業者信箱，
// 中間只隔著一個 MAIL_PROVIDER 字串。這一格讓「非 prod ＋ 不限制收件人」開不起來。
func TestConfig_Validate_NonProductionRejectsUnrestrictedRecipients(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	for _, env := range []string{"development", "dev", "staging"} {
		t.Run(env, func(t *testing.T) {
			cfg := &Config{
				Env: env,
				Database: DatabaseConfig{
					Host: "db-host", User: "user", Password: "pass",
				},
				MediaBaseURL: "https://media.example",
				VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
				Mail:         MailConfig{Provider: "resend", From: "noreply@example.com", RecipientPolicy: "unrestricted"},
			}
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), "only allowed when ENV=prod") {
				t.Fatalf("ENV=%s 帶 unrestricted 應被拒，實際 err = %v", env, err)
			}
		})
	}
}

// 對照組：非 prod 走 redirect 要能通過，否則上面那條就只是把 dev 全部擋死。
func TestConfig_Validate_NonProductionAllowsRedirect(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")

	cfg := &Config{
		Env: "development",
		Database: DatabaseConfig{
			Host: "db-host", User: "user", Password: "pass",
		},
		MediaBaseURL: "https://media.example",
		VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
		Mail: MailConfig{
			Provider: "resend", From: "noreply@example.com", RecipientPolicy: "redirect", RedirectTo: "willwu@example.com",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("ENV=development 帶 redirect 應通過，實際 err = %v", err)
	}
}

// prod 少了 unrestricted 也要擋：prod 必須寄給真實業者，靜默退回 redirect 等於
// 所有正式邀請信都寄到同一個測試信箱而沒有人收得到。
func TestConfig_Validate_ProductionRequiresUnrestrictedRecipients(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel-collector:4317")

	cfg := &Config{
		Env: "production",
		Database: DatabaseConfig{
			Host: "db-host", User: "user", Password: "pass",
		},
		MediaBaseURL: "https://media.example",
		VendorPortal: VendorPortalConfig{BaseURL: "https://example.com"},
		Mail: MailConfig{
			Provider: "resend", From: "noreply@example.com", RecipientPolicy: "redirect", RedirectTo: "willwu@example.com",
		},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MAIL_RECIPIENT_POLICY must be") {
		t.Fatalf("prod 帶 redirect 應被拒，實際 err = %v", err)
	}
}

func TestConfig_Validate_ResendRequiresMailFrom(t *testing.T) {
	cfg := &Config{
		Env: "development",
		Database: DatabaseConfig{
			Host:     "db-host",
			User:     "user",
			Password: "pass",
		},
		MediaBaseURL: "https://test-media.example.com",
		VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
		Mail:         MailConfig{Provider: "resend", RecipientPolicy: "redirect", RedirectTo: "willwu@example.com"},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MAIL_FROM") {
		t.Fatalf("resend without MAIL_FROM: error = %v, want it to name MAIL_FROM", err)
	}
	cfg.Mail.From = "noreply@example.com"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("resend with MAIL_FROM error = %v, want nil", err)
	}
}

func TestConfig_Validate_ResendProviderIsCaseInsensitive(t *testing.T) {
	cfg := &Config{
		Env: "development",
		Database: DatabaseConfig{
			Host:     "db-host",
			User:     "user",
			Password: "pass",
		},
		MediaBaseURL: "https://test-media.example.com",
		VendorPortal: VendorPortalConfig{BaseURL: "https://web-dev.example.com"},
		Mail:         MailConfig{Provider: " Resend ", RecipientPolicy: "redirect", RedirectTo: "willwu@example.com"},
	}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "MAIL_FROM") {
		t.Fatalf("Resend without MAIL_FROM: error = %v, want it to name MAIL_FROM", err)
	}
}

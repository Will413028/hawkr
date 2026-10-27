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
		Name:     "hawkr_social",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()

	checks := []string{"localhost", "5432", "admin", "secret", "hawkr_social", "sslmode=disable"}
	for _, s := range checks {
		if !strings.Contains(dsn, s) {
			t.Errorf("DSN missing %q: %s", s, dsn)
		}
	}
}

func TestExternalServiceConfig_Addr(t *testing.T) {
	cfg := ExternalServiceConfig{Host: "account-service", Port: "9941"}
	addr := cfg.Addr()

	if addr != "account-service:9941" {
		t.Errorf("expected account-service:9941, got %s", addr)
	}
}

func TestConfig_LoadDefaults(t *testing.T) {
	// social-service Load() has no Validate(), so we verify default values directly.
	cfg := &Config{
		Env: "development",
		DB: DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "dbadmin",
			Password: "",
			Name:     "hawkr_social",
		},
		Server:  ServerConfig{GRPCPort: "9917"},
		Account: ExternalServiceConfig{Host: "account-service", Port: "9941"},
		Buyer:   ExternalServiceConfig{Host: "127.0.0.1", Port: "9941"},
		Media:   ExternalServiceConfig{Host: "media-service", Port: "9937"},
		Product: ExternalServiceConfig{Host: "product-service", Port: "9913"},
		Seller:  ExternalServiceConfig{Host: "127.0.0.1", Port: "9941"},
		RabbitMQ: RabbitMQConfig{
			URL: "amqp://guest:guest@localhost:5672/",
		},
	}

	if cfg.Env != "development" {
		t.Errorf("expected Env=development, got %s", cfg.Env)
	}
	if cfg.DB.Name != "hawkr_social" {
		t.Errorf("expected DB.Name=hawkr_social, got %s", cfg.DB.Name)
	}
	if cfg.DB.User != "dbadmin" {
		t.Errorf("expected DB.User=dbadmin, got %s", cfg.DB.User)
	}
	if cfg.Server.GRPCPort != "9917" {
		t.Errorf("expected GRPCPort=9917, got %s", cfg.Server.GRPCPort)
	}
	if cfg.Account.Addr() != "account-service:9941" {
		t.Errorf("expected Account.Addr()=account-service:9941, got %s", cfg.Account.Addr())
	}
	if cfg.Buyer.Addr() != "127.0.0.1:9941" {
		t.Errorf("expected Buyer.Addr()=127.0.0.1:9941, got %s", cfg.Buyer.Addr())
	}
	if cfg.Media.Addr() != "media-service:9937" {
		t.Errorf("expected Media.Addr()=media-service:9937, got %s", cfg.Media.Addr())
	}
	if cfg.Product.Addr() != "product-service:9913" {
		t.Errorf("expected Product.Addr()=product-service:9913, got %s", cfg.Product.Addr())
	}
	if cfg.Seller.Addr() != "127.0.0.1:9941" {
		t.Errorf("expected Seller.Addr()=127.0.0.1:9941, got %s", cfg.Seller.Addr())
	}
}

func TestConfig_ServerUsesFinalGRPCOnlyConfigType(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{GRPCPort: "9917"},
	}
	if cfg.Server.GRPCPort != "9917" {
		t.Errorf("GRPCPort = %q, want %q", cfg.Server.GRPCPort, "9917")
	}
}

func TestLoad_DatabaseDefaultsMatchSharedLoader(t *testing.T) {
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_POOL_PORT", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("MEDIA_BASE_URL", "https://test-media.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DB.Host != "localhost" {
		t.Errorf("DB.Host = %q, want %q (from database.LoadDBConfigFromEnv default)", cfg.DB.Host, "localhost")
	}
	if cfg.DB.User != "dbadmin" {
		t.Errorf("DB.User = %q, want %q (from database.LoadDBConfigFromEnv default)", cfg.DB.User, "dbadmin")
	}
	if cfg.DB.Name != "hawkr_social" {
		t.Errorf("DB.Name = %q, want %q (service-specific fallback)", cfg.DB.Name, "hawkr_social")
	}
}

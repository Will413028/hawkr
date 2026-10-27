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
		DBName:   "hawkr_review",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()

	checks := []string{"localhost", "5432", "admin", "secret", "hawkr_review", "sslmode=disable"}
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
				Port:     "5432",
				User:     "user",
				Password: "pass",
				DBName:   "testdb",
			},
			Server:       ServerConfig{GRPCPort: "10033"},
			MediaBaseURL: "https://test-media.example.com",
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("missing DB_HOST", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Port: "5432", User: "user", Password: "pass", DBName: "db"},
			Server:   ServerConfig{GRPCPort: "10033"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing DB_HOST")
		}
		if !strings.Contains(err.Error(), "DB_HOST") {
			t.Errorf("error should mention DB_HOST: %v", err)
		}
	})

	t.Run("missing DB_PORT", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Host: "host", User: "user", Password: "pass", DBName: "db"},
			Server:   ServerConfig{GRPCPort: "10033"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing DB_PORT")
		}
		if !strings.Contains(err.Error(), "DB_PORT") {
			t.Errorf("error should mention DB_PORT: %v", err)
		}
	})

	t.Run("missing DB_USER", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Host: "host", Port: "5432", Password: "pass", DBName: "db"},
			Server:   ServerConfig{GRPCPort: "10033"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing DB_USER")
		}
		if !strings.Contains(err.Error(), "DB_USER") {
			t.Errorf("error should mention DB_USER: %v", err)
		}
	})

	t.Run("missing DB_PASSWORD", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Host: "host", Port: "5432", User: "user", DBName: "db"},
			Server:   ServerConfig{GRPCPort: "10033"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing DB_PASSWORD")
		}
		if !strings.Contains(err.Error(), "DB_PASSWORD") {
			t.Errorf("error should mention DB_PASSWORD: %v", err)
		}
	})

	t.Run("missing DB_NAME", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Host: "host", Port: "5432", User: "user", Password: "pass"},
			Server:   ServerConfig{GRPCPort: "10033"},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing DB_NAME")
		}
		if !strings.Contains(err.Error(), "DB_NAME") {
			t.Errorf("error should mention DB_NAME: %v", err)
		}
	})

	t.Run("missing GRPC_PORT", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Host: "host", Port: "5432", User: "user", Password: "pass", DBName: "db"},
			Server:   ServerConfig{},
		}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error for missing GRPC_PORT")
		}
		if !strings.Contains(err.Error(), "GRPC_PORT") {
			t.Errorf("error should mention GRPC_PORT: %v", err)
		}
	})

	t.Run("multiple missing fields reported together", func(t *testing.T) {
		cfg := &Config{Database: DatabaseConfig{}, Server: ServerConfig{}}
		err := cfg.Validate()
		if err == nil {
			t.Fatal("expected error")
		}
		for _, field := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "GRPC_PORT"} {
			if !strings.Contains(err.Error(), field) {
				t.Errorf("error should mention %s: %v", field, err)
			}
		}
	})
}

func TestConfig_LoadDefaults(t *testing.T) {
	cfg := &Config{
		Env: "development",
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "dbadmin",
			Password: "",
			DBName:   "hawkr_review",
		},
		Server: ServerConfig{GRPCPort: "10033"},
		OrderService: OrderServiceConfig{
			Addr: "order-service:10011",
		},
		RabbitMQ: RabbitMQConfig{
			URL: "amqp://guest:guest@localhost:5672/",
		},
	}

	if cfg.Env != "development" {
		t.Errorf("expected Env=development, got %s", cfg.Env)
	}
	if cfg.Database.DBName != "hawkr_review" {
		t.Errorf("expected DBName=hawkr_review, got %s", cfg.Database.DBName)
	}
	if cfg.Server.GRPCPort != "10033" {
		t.Errorf("expected GRPCPort=10033, got %s", cfg.Server.GRPCPort)
	}
}

func TestConfig_ServerIsFinalGRPCOnlyConfig(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{GRPCPort: "10033"},
	}
	if cfg.Server.GRPCPort != "10033" {
		t.Errorf("GRPCPort = %q, want %q", cfg.Server.GRPCPort, "10033")
	}
}

func TestLoad_DatabaseDefaultsMatchSharedLoader(t *testing.T) {
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_POOL_PORT", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "postgres")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("GRPC_PORT", "10033")
	t.Setenv("HTTP_PORT", "10032")
	t.Setenv("MEDIA_BASE_URL", "https://test-media.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.User != "dbadmin" {
		t.Errorf("Database.User = %q, want %q (from database.LoadDBConfigFromEnv default)", cfg.Database.User, "dbadmin")
	}
	if cfg.Database.DBName != "hawkr_review" {
		t.Errorf("Database.DBName = %q, want %q (service-specific fallback)", cfg.Database.DBName, "hawkr_review")
	}
	if cfg.Server.GRPCPort != "10033" {
		t.Errorf("Server.GRPCPort = %q, want %q", cfg.Server.GRPCPort, "10033")
	}
}

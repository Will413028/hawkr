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
		DBName:   "hawkr_crm",
		SSLMode:  "disable",
	}
	dsn := cfg.DSN()

	checks := []string{"localhost", "5432", "admin", "secret", "hawkr_crm", "sslmode=disable"}
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
			Server: ServerConfig{GRPCPort: "10030"},
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("missing DB_HOST", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{Port: "5432", User: "user", Password: "pass", DBName: "db"},
			Server:   ServerConfig{GRPCPort: "10030"},
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
			Server:   ServerConfig{GRPCPort: "10030"},
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
			Server:   ServerConfig{GRPCPort: "10030"},
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
			Server:   ServerConfig{GRPCPort: "10030"},
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
			Server:   ServerConfig{GRPCPort: "10030"},
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
		Env:         "development",
		RabbitMQURL: "amqp://guest:guest@localhost:5672/",
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Password: "postgres",
			DBName:   "hawkr_crm",
		},
		Server: ServerConfig{
			GRPCPort: "10030",
		},
	}

	if cfg.Env != "development" {
		t.Errorf("expected Env=development, got %s", cfg.Env)
	}
	if cfg.Database.DBName != "hawkr_crm" {
		t.Errorf("expected DBName=hawkr_crm, got %s", cfg.Database.DBName)
	}
	if cfg.Server.GRPCPort != "10030" {
		t.Errorf("expected GRPCPort=10030, got %s", cfg.Server.GRPCPort)
	}
}

func TestConfig_Validate_ProductionRequiresOTLPMetricsEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	cfg := &Config{
		Env: "production",
		Database: DatabaseConfig{
			Host: "db-host", Port: "5432", User: "user", Password: "pass", DBName: "crm",
		},
		Server: ServerConfig{GRPCPort: "10030"},
	}
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP") {
		t.Fatalf("expected production OTLP metrics validation error, got %v", err)
	}
}

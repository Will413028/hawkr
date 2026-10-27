package config

import (
	"fmt"
	"strings"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
)

// Config holds all configuration for the CRM service.
type Config struct {
	Env            string
	RabbitMQURL    string
	Database       DatabaseConfig
	Server         ServerConfig
	AccountService ExternalServiceConfig
	BuyerService   ExternalServiceConfig
	CouponService  ExternalServiceConfig
	OrderService   ExternalServiceConfig
}

type ExternalServiceConfig struct {
	Addr string
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	GRPCPort string
}

// DSN returns the PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return database.BuildDSNKeyword(database.DBConfig{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Name:     c.DBName,
		SSLMode:  c.SSLMode,
	})
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Env:         pkgconfig.GetEnvOrDefault("ENV", "development"),
		RabbitMQURL: pkgconfig.GetEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Database: DatabaseConfig{
			Host:     pkgconfig.GetEnvOrDefault("DB_HOST", "localhost"),
			Port:     database.ResolvePoolPort(),
			User:     pkgconfig.GetEnvOrDefault("DB_USER", "postgres"),
			Password: pkgconfig.GetEnvOrDefault("DB_PASSWORD", "postgres"),
			DBName:   pkgconfig.GetEnvOrDefault("DB_NAME", "hawkr_crm"),
			SSLMode:  pkgconfig.GetEnvOrDefault("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			GRPCPort: pkgconfig.GetEnvOrDefault("GRPC_PORT", "10030"),
		},
		AccountService: ExternalServiceConfig{
			Addr: fmt.Sprintf("%s:%s",
				pkgconfig.GetEnvOrDefault("ACCOUNT_SERVICE_HOST", "127.0.0.1"),
				pkgconfig.GetEnvOrDefault("ACCOUNT_SERVICE_GRPC_PORT", "9941"),
			),
		},
		BuyerService: ExternalServiceConfig{
			Addr: fmt.Sprintf("%s:%s",
				pkgconfig.GetEnvOrDefault("BUYER_SERVICE_HOST", "127.0.0.1"),
				pkgconfig.GetEnvOrDefault("BUYER_SERVICE_GRPC_PORT", "9941"),
			),
		},
		CouponService: ExternalServiceConfig{
			Addr: fmt.Sprintf("%s:%s",
				pkgconfig.GetEnvOrDefault("COUPON_SERVICE_HOST", "127.0.0.1"),
				pkgconfig.GetEnvOrDefault("COUPON_SERVICE_GRPC_PORT", "9951"),
			),
		},
		OrderService: ExternalServiceConfig{
			Addr: fmt.Sprintf("%s:%s",
				pkgconfig.GetEnvOrDefault("ORDER_SERVICE_HOST", "127.0.0.1"),
				pkgconfig.GetEnvOrDefault("ORDER_SERVICE_GRPC_PORT", "9915"),
			),
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks required configuration fields.
func (c *Config) Validate() error {
	var missing []string

	if c.Database.Host == "" {
		missing = append(missing, "DB_HOST")
	}
	if c.Database.Port == "" {
		missing = append(missing, "DB_PORT")
	}
	if c.Database.User == "" {
		missing = append(missing, "DB_USER")
	}
	if c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if c.Database.DBName == "" {
		missing = append(missing, "DB_NAME")
	}
	if c.Server.GRPCPort == "" {
		missing = append(missing, "GRPC_PORT")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if err := pkgotel.ValidateMetricsConfiguration(c.Env); err != nil {
		return err
	}
	return nil
}

package config

import (
	"fmt"
	"strings"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
)

type Config struct {
	Database      DatabaseConfig
	Server        ServerConfig
	Env           string
	OrderService  OrderServiceConfig
	MediaService  MediaServiceConfig
	BuyerService  BuyerServiceConfig
	SellerService SellerServiceConfig
	MediaBaseURL  string
	WebRevalidate WebRevalidateConfig
	RabbitMQ      RabbitMQConfig
	Redis         RedisConfig
}

// ServerConfig is the final review-service process boundary: one business
// gRPC listener. Management HTTP (health/metrics/pprof) is intentionally not
// part of the service configuration; observability uses OTLP push and health
// uses the standard gRPC health service.
type ServerConfig struct {
	GRPCPort string
}

// WebRevalidateConfig holds hawkr-website /api/revalidate caller config (後端服務責任邊界規範 §9.7).
type WebRevalidateConfig struct {
	BaseURL string
	Secret  string
}

type RedisConfig struct {
	Host, Port, Password string
}

type BuyerServiceConfig struct {
	Addr string
}

type SellerServiceConfig struct {
	Addr string
}

type MediaServiceConfig struct {
	Host string
	Port string
}

func (m *MediaServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%s", m.Host, m.Port)
}

type RabbitMQConfig struct {
	URL string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type OrderServiceConfig struct {
	Addr string
}

func (c *DatabaseConfig) DSN() string {
	return database.BuildDSNKeyword(database.DBConfig{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Name:     c.DBName,
		SSLMode:  c.SSLMode,
		TimeZone: "UTC",
	})
}

func Load() (*Config, error) {
	dbCfg := database.LoadDBConfigFromEnv()
	if dbCfg.Name == "" {
		dbCfg.Name = "hawkr_review"
	}
	cfg := &Config{
		Env: pkgconfig.GetEnvOrDefault("ENV", "development"),
		Database: DatabaseConfig{
			Host:     dbCfg.Host,
			Port:     dbCfg.Port,
			User:     dbCfg.User,
			Password: dbCfg.Password,
			DBName:   dbCfg.Name,
			SSLMode:  dbCfg.SSLMode,
		},
		Server: ServerConfig{
			GRPCPort: pkgconfig.GetEnvOrDefault("GRPC_PORT", "10033"),
		},
		OrderService: OrderServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("ORDER_SERVICE_ADDR", "order-service:9915"),
		},
		MediaService: MediaServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("MEDIA_SERVICE_HOST", "media-service"),
			Port: pkgconfig.GetEnvOrDefault("MEDIA_SERVICE_GRPC_PORT", "9937"),
		},
		BuyerService: BuyerServiceConfig{
			Addr: fmt.Sprintf("%s:%s",
				pkgconfig.GetEnvOrDefault("BUYER_SERVICE_HOST", "127.0.0.1"),
				pkgconfig.GetEnvOrDefault("BUYER_SERVICE_GRPC_PORT", "9941")),
		},
		SellerService: SellerServiceConfig{
			Addr: fmt.Sprintf("%s:%s",
				pkgconfig.GetEnvOrDefault("SELLER_SERVICE_HOST", "127.0.0.1"),
				pkgconfig.GetEnvOrDefault("SELLER_SERVICE_GRPC_PORT", "9941")),
		},
		// 無預設值，缺值由 Validate() fail-fast（backend#2283）：媒體域名不做新舊相容，
		// dev 一律 test-media.example.com、prod 一律 media.example.com。
		MediaBaseURL: pkgconfig.GetEnvOrDefault(pkgconfig.MediaBaseURLEnvKey, ""),
		WebRevalidate: WebRevalidateConfig{
			BaseURL: pkgconfig.GetEnvOrDefault("WEB_REVALIDATE_BASE_URL", ""),
			Secret:  pkgconfig.GetEnvOrDefault("WEB_REVALIDATE_SECRET", ""),
		},
		RabbitMQ: RabbitMQConfig{
			URL: pkgconfig.GetEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		},
		Redis: RedisConfig{
			Host:     pkgconfig.GetEnvOrDefault("REDIS_HOST", "redis"),
			Port:     pkgconfig.GetEnvOrDefault("REDIS_PORT", "6379"),
			Password: pkgconfig.GetEnvOrDefault("REDIS_PASSWORD", ""),
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

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
	if pkgconfig.MediaBaseURLMissing(c.MediaBaseURL) {
		missing = append(missing, pkgconfig.MediaBaseURLEnvKey)
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	return nil
}

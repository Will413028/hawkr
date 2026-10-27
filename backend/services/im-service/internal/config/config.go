package config

import (
	"fmt"
	"os"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
)

// Config holds the service configuration.
type Config struct {
	Env                     string
	GRPCPort                string
	RabbitMQURL             string
	DatabaseURL             string
	BuyerServiceAddr        string
	SellerServiceAddr       string
	OrderServiceAddr        string
	NotificationServiceAddr string
	MediaServiceAddr        string
	MediaBaseURL            string
	RedisHost               string
	RedisPort               string
	RedisPassword           string
	// ChatStreamJWTSecret authenticates the Gateway-to-IM long-lived stream.
	// It is separate from both the external user JWT and the existing internal
	// HMAC purpose; the previous value is retained only during key rotation.
	ChatStreamJWTSecret     string
	ChatStreamJWTSecretPrev string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		// 部署注入的是 ENV（k8s ConfigMap，prod=prod）；APP_ENV 只有 gateway 與
		// payuni-webhook 兩個 overlay 會設，其餘服務讀它永遠拿到預設值 —— 這正是
		// prod 的 pprof 閘門失效的來源之一（issue #1418）。
		Env:                     getEnv("ENV", "development"),
		GRPCPort:                getEnv("GRPC_PORT", "9933"),
		BuyerServiceAddr:        getEnv("BUYER_SERVICE_ADDR", "127.0.0.1:9941"),
		SellerServiceAddr:       getEnv("SELLER_SERVICE_ADDR", "127.0.0.1:9941"),
		OrderServiceAddr:        getEnv("ORDER_SERVICE_ADDR", "order-service:9915"),
		NotificationServiceAddr: getEnv("NOTIFICATION_SERVICE_ADDR", "notification-service:10037"),
		MediaServiceAddr:        getEnv("MEDIA_SERVICE_ADDR", "media-service:9937"),
		// 無預設值，缺值由 Validate() fail-fast（backend#2283）：媒體域名不做新舊相容，
		// dev 一律 test-media.example.com、prod 一律 media.example.com。
		MediaBaseURL:            getEnv("MEDIA_BASE_URL", ""),
		RedisHost:               getEnv("REDIS_HOST", "redis"),
		RedisPort:               getEnv("REDIS_PORT", "6379"),
		RedisPassword:           getEnv("REDIS_PASSWORD", ""),
		ChatStreamJWTSecret:     os.Getenv("IM_CHAT_STREAM_JWT_SECRET"),
		ChatStreamJWTSecretPrev: os.Getenv("IM_CHAT_STREAM_JWT_SECRET_PREV"),
	}

	cfg.RabbitMQURL = os.Getenv("RABBITMQ_URL")
	if cfg.RabbitMQURL == "" {
		host := getEnv("RABBITMQ_HOST", "localhost")
		port := getEnv("RABBITMQ_PORT", "5672")
		user := getEnv("RABBITMQ_USERNAME", "guest")
		pass := getEnv("RABBITMQ_PASSWORD", "guest")
		vhost := getEnv("RABBITMQ_VHOST", "/")
		cfg.RabbitMQURL = fmt.Sprintf("amqp://%s:%s@%s:%s/%s", user, pass, host, port, vhost)
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		host := getEnv("DB_HOST", "localhost")
		port := getEnv("DB_POOL_PORT", getEnv("DB_PORT", "5432"))
		user := getEnv("DB_USER", "postgres")
		pass := getEnv("DB_PASSWORD", "postgres")
		name := getEnv("DB_NAME", "hawkr_im")
		cfg.DatabaseURL = database.BuildDSN(database.DBConfig{
			Host:     host,
			Port:     port,
			User:     user,
			Password: pass,
			Name:     name,
			SSLMode:  pkgconfig.GetEnvOrDefault("DB_SSLMODE", "disable"),
		})
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks required configuration fields.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("missing required config: DATABASE_URL or DB_* env vars")
	}
	// MEDIA_BASE_URL 無預設域名，缺值即 fail-fast（backend#2283）：dev 一律 test-media、
	// prod 一律 media，不做新舊相容也不猜域名——漏設就啟動失敗，不允許靜默吐出死連結。
	if c.MediaBaseURL == "" {
		return fmt.Errorf("missing required config: MEDIA_BASE_URL")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

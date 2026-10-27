package config

import (
	"fmt"
	"strings"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/mail/guard"
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
)

// Config holds all configuration for the service.
type Config struct {
	Env          string
	Database     DatabaseConfig
	Server       ServerConfig
	RabbitMQ     RabbitMQConfig
	Firebase     FirebaseConfig
	MediaBaseURL string // 媒體域名基底（49 §3/§4.3）；渲染 push imageUrl 時把 objectKey 組顯示 URL

	// BuyerServiceGRPCAddr/SellerServiceGRPCAddr 供 chat-notification PII 清除
	// 流程列舉帳號歷史 buyer_id/store_id surrogate（見
	// internal/client/buyer、internal/client/seller）。
	BuyerServiceGRPCAddr  string
	SellerServiceGRPCAddr string

	Mail         MailConfig
	VendorPortal VendorPortalConfig
}

// MailConfig holds the pkg/mail provider configuration for the vendor portal
// invite email consumer (#2539 E1).
type MailConfig struct {
	Provider     string
	From         string
	ResendAPIKey string

	// RecipientPolicy／AllowedRecipients／RedirectTo 交給 pkg/mail/guard。
	// 語意與允許值見該 package；此處只負責把 env 原樣帶過去，不在這裡解釋或
	// 補預設——補了就等於多一個「忘了設也能寄」的入口。
	RecipientPolicy   string
	AllowedRecipients []string
	RedirectTo        string
}

// VendorPortalConfig holds the base URL used to build vendor portal invite
// links (see worker/vendor_invite.go).
type VendorPortalConfig struct {
	BaseURL string
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

// RabbitMQConfig holds RabbitMQ configuration.
type RabbitMQConfig struct {
	URL string
}

// FirebaseConfig holds Firebase credentials.
type FirebaseConfig struct {
	CredentialsJSON string
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
		Env: pkgconfig.GetEnvOrDefault("ENV", "development"),
		Database: DatabaseConfig{
			Host:     pkgconfig.GetEnvOrDefault("DB_HOST", "localhost"),
			Port:     database.ResolvePoolPort(),
			User:     pkgconfig.GetEnvOrDefault("DB_USER", "postgres"),
			Password: pkgconfig.GetEnvOrDefault("DB_PASSWORD", "postgres"),
			DBName:   pkgconfig.GetEnvOrDefault("DB_NAME", "hawkr_notification"),
			SSLMode:  pkgconfig.GetEnvOrDefault("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			GRPCPort: pkgconfig.GetEnvOrDefault("GRPC_PORT", "10037"),
		},
		RabbitMQ: RabbitMQConfig{
			URL: pkgconfig.GetEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		},
		Firebase: FirebaseConfig{
			CredentialsJSON: pkgconfig.GetEnvOrDefault("FIREBASE_CREDENTIALS_JSON", ""),
		},
		// 無預設值，缺值由 Validate() fail-fast（backend#2283）：媒體域名不做新舊相容，
		// dev 一律 test-media.example.com、prod 一律 media.example.com。
		MediaBaseURL: pkgconfig.GetEnvOrDefault("MEDIA_BASE_URL", ""),

		BuyerServiceGRPCAddr:  pkgconfig.GetEnvOrDefault("BUYER_SERVICE_GRPC_ADDR", "127.0.0.1:9941"),
		SellerServiceGRPCAddr: pkgconfig.GetEnvOrDefault("SELLER_SERVICE_GRPC_ADDR", "127.0.0.1:9941"),

		Mail: MailConfig{
			Provider:          pkgconfig.GetEnvOrDefault("MAIL_PROVIDER", "log"),
			From:              pkgconfig.GetEnvOrDefault("MAIL_FROM", ""),
			ResendAPIKey:      pkgconfig.GetEnvOrDefault("RESEND_API_KEY", ""),
			RecipientPolicy:   strings.TrimSpace(pkgconfig.GetEnvOrDefault("MAIL_RECIPIENT_POLICY", "")),
			AllowedRecipients: guard.ParseAllowed(pkgconfig.GetEnvOrDefault("MAIL_ALLOWED_RECIPIENTS", "")),
			RedirectTo:        strings.TrimSpace(pkgconfig.GetEnvOrDefault("MAIL_REDIRECT_TO", "")),
		},
		// 無預設值，缺值由 Validate() fail-fast：組出的邀請連結指到 dev 或 prod 站是逐環境
		// 決定，猜錯值等於寄出一封連到錯環境的信。
		VendorPortal: VendorPortalConfig{
			BaseURL: pkgconfig.GetEnvOrDefault("VENDOR_PORTAL_BASE_URL", ""),
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
	if c.Database.User == "" {
		missing = append(missing, "DB_USER")
	}
	if c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}

	// MEDIA_BASE_URL 無預設域名，缺值即 fail-fast（backend#2283）：dev 一律 test-media、
	// prod 一律 media，不做新舊相容也不猜域名——漏設就啟動失敗，不允許靜默吐出死連結。
	if c.MediaBaseURL == "" {
		missing = append(missing, "MEDIA_BASE_URL")
	}
	if c.VendorPortal.BaseURL == "" {
		missing = append(missing, "VENDOR_PORTAL_BASE_URL")
	}
	provider := strings.ToLower(strings.TrimSpace(c.Mail.Provider))
	if provider == "resend" && strings.TrimSpace(c.Mail.From) == "" {
		missing = append(missing, "MAIL_FROM")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if env := strings.ToLower(strings.TrimSpace(c.Env)); env == "prod" || env == "production" {
		if provider != "resend" {
			return fmt.Errorf("MAIL_PROVIDER must be resend when ENV=%s, got %q", c.Env, c.Mail.Provider)
		}
		// prod 必須寄給呼叫端指定的真實業者，所以只有 prod 能是 unrestricted。
		if guard.ParseMode(c.Mail.RecipientPolicy) != guard.ModeUnrestricted {
			return fmt.Errorf("MAIL_RECIPIENT_POLICY must be %q when ENV=%s, got %q",
				guard.ModeUnrestricted, c.Env, c.Mail.RecipientPolicy)
		}
	} else if guard.ParseMode(c.Mail.RecipientPolicy) == guard.ModeUnrestricted {
		// ⛔ 反向那半邊才是真正防的東西：非 prod 帶著有效憑證與真實 prospect
		// 名單，只要收件人不設限就會把測試信寄給真的業者。這條讓那個組合開不起來。
		return fmt.Errorf("MAIL_RECIPIENT_POLICY=%q is only allowed when ENV=prod, got ENV=%q",
			guard.ModeUnrestricted, c.Env)
	}
	if err := pkgotel.ValidateMetricsConfiguration(c.Env); err != nil {
		return err
	}
	return nil
}

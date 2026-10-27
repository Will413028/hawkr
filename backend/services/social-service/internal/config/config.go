package config

import (
	"fmt"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
)

type Config struct {
	Env     string
	DB      DatabaseConfig
	Server  ServerConfig
	Account ExternalServiceConfig
	Buyer   ExternalServiceConfig
	Media   ExternalServiceConfig
	Product ExternalServiceConfig
	Seller  ExternalServiceConfig
	Review  ExternalServiceConfig
	IM      ExternalServiceConfig
	// MediaBaseURL 為媒體 URL 組裝域名（MEDIA_BASE_URL，49 號設計書 §8.5）。
	MediaBaseURL  string
	RabbitMQ      RabbitMQConfig
	Redis         RedisConfig
	WebRevalidate WebRevalidateConfig
	// ScheduledPostPublish 為排程貼文發布 worker 的參數（56 號設計書 §8.5.1 (C)）。
	ScheduledPostPublish ScheduledPostPublishConfig
}

// ServerConfig is the final social-service process boundary: one business
// gRPC listener. Management HTTP (health/metrics/pprof) is intentionally not
// part of the service configuration; observability uses OTLP push and health
// uses the standard gRPC health service.
type ServerConfig struct {
	GRPCPort string
}

// ScheduledPostPublishConfig holds the scheduled-post publish worker's tunables
// (56 號設計書 §8.5.1 (C)). 這組必須是本服務自己的參數而不是沿用 Gateway 的 (B) 組：
// (B) 落在 gateway 的 config，social-service 讀不到它；而 deadline 是呼叫端的決定，
// worker 是 RecordAuditIntent 的第三個呼叫端。
//
// ⚠️ 這組的三條 Validate() 斷言比的是**這組參數彼此之間**的不等式，全部吃程式碼預設值
// 時三條照樣成立、boot 照樣綠——斷言擋得住「調錯」，擋不住「沒設」。位置 3（k8s values）
// 漏掉本服務不會被任何檢查抓到，只能靠部署後 kubectl get deploy 逐項對帳。
type ScheduledPostPublishConfig struct {
	// PollIntervalSeconds 同時是「排程時點到實際發布」的下限誤差。
	PollIntervalSeconds int
	// BatchSize 不影響租約不變式（租約基準逐筆取，§29）。
	BatchSize int
	// LeaseSeconds 是 cms_post.schedule_lease_expires_at 的租約長度。
	LeaseSeconds int
	MaxRetry     int
	// BackoffBaseSeconds 應 ≥ 一個 tick——小於 tick 的退避不會讓重試更早發生。
	BackoffBaseSeconds int
	BackoffMaxSeconds  int

	AuditIntentDeadlineMs     int
	AuditConfirmDeadlineMs    int
	AuditConfirmMaxAttempts   int
	AuditConfirmTotalBudgetMs int
	// TxBudgetMs 是單則貼文發布交易的預算。它必須是一個可調參的具名值：租約下界是
	// 不等式，右邊每一項都要有出處，否則那條斷言只能靠散文承諾。
	TxBudgetMs int
	// WindowHours 是候選窗口長度；scheduled_publish_at 早於 now()-本值的列由每個
	// tick 的收斂語句轉 expired。改小它必須同時複核租約長度仍遠小於它。
	WindowHours int
}

// WebRevalidateConfig holds hawkr-website /api/revalidate caller config (後端服務責任邊界規範 §9.7).
type WebRevalidateConfig struct {
	BaseURL string
	Secret  string
}

type RabbitMQConfig struct {
	URL string
}

type RedisConfig struct {
	Host, Port, Password string
}

type DatabaseConfig struct {
	Host, Port, User, Password, Name, SSLMode string
}

func (d *DatabaseConfig) DSN() string {
	return database.BuildDSNKeyword(database.DBConfig{
		Host:     d.Host,
		Port:     d.Port,
		User:     d.User,
		Password: d.Password,
		Name:     d.Name,
		SSLMode:  d.SSLMode,
		TimeZone: "UTC",
	})
}

type ExternalServiceConfig struct {
	Host, Port string
}

func (e *ExternalServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%s", e.Host, e.Port)
}

func Load() (*Config, error) {
	dbCfg := database.LoadDBConfigFromEnv()
	if dbCfg.Name == "" {
		dbCfg.Name = "hawkr_social"
	}
	cfg := &Config{
		// 部署注入的是 ENV（k8s ConfigMap，prod=prod）；APP_ENV 只有 gateway 與
		// payuni-webhook 兩個 overlay 會設，其餘服務讀它永遠拿到預設值 —— 這正是
		// prod 的 pprof 閘門失效的來源之一（issue #1418）。
		Env: pkgconfig.GetEnvOrDefault("ENV", "development"),
		DB: DatabaseConfig{
			Host:     dbCfg.Host,
			Port:     dbCfg.Port,
			User:     dbCfg.User,
			Password: dbCfg.Password,
			Name:     dbCfg.Name,
			SSLMode:  dbCfg.SSLMode,
		},
		Server: ServerConfig{
			GRPCPort: pkgconfig.GetEnvOrDefault("GRPC_PORT", "9917"),
		},
		Account: ExternalServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("ACCOUNT_SERVICE_HOST", "account-service"),
			Port: pkgconfig.GetEnvOrDefault("ACCOUNT_SERVICE_GRPC_PORT", "9941"),
		},
		Buyer: ExternalServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("BUYER_SERVICE_HOST", "127.0.0.1"),
			Port: pkgconfig.GetEnvOrDefault("BUYER_SERVICE_GRPC_PORT", "9941"),
		},
		Media: ExternalServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("MEDIA_SERVICE_HOST", "media-service"),
			Port: pkgconfig.GetEnvOrDefault("MEDIA_SERVICE_GRPC_PORT", "9937"),
		},
		Product: ExternalServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("PRODUCT_SERVICE_HOST", "product-service"),
			Port: pkgconfig.GetEnvOrDefault("PRODUCT_SERVICE_GRPC_PORT", "9913"),
		},
		Seller: ExternalServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("SELLER_SERVICE_HOST", "127.0.0.1"),
			Port: pkgconfig.GetEnvOrDefault("SELLER_SERVICE_GRPC_PORT", "9941"),
		},
		Review: ExternalServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("REVIEW_SERVICE_HOST", "review-service"),
			Port: pkgconfig.GetEnvOrDefault("REVIEW_SERVICE_GRPC_PORT", "10033"),
		},
		IM: ExternalServiceConfig{
			Host: pkgconfig.GetEnvOrDefault("IM_SERVICE_HOST", "im-service"),
			Port: pkgconfig.GetEnvOrDefault("IM_SERVICE_GRPC_PORT", "9933"),
		},
		MediaBaseURL: pkgconfig.GetEnvOrDefault(pkgconfig.MediaBaseURLEnvKey, ""),
		RabbitMQ: RabbitMQConfig{
			URL: pkgconfig.GetEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		},
		Redis: RedisConfig{
			Host:     pkgconfig.GetEnvOrDefault("REDIS_HOST", "redis"),
			Port:     pkgconfig.GetEnvOrDefault("REDIS_PORT", "6379"),
			Password: pkgconfig.GetEnvOrDefault("REDIS_PASSWORD", ""),
		},
		WebRevalidate: WebRevalidateConfig{
			BaseURL: pkgconfig.GetEnvOrDefault("WEB_REVALIDATE_BASE_URL", ""),
			Secret:  pkgconfig.GetEnvOrDefault("WEB_REVALIDATE_SECRET", ""),
		},
		ScheduledPostPublish: ScheduledPostPublishConfig{
			PollIntervalSeconds: pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_POLL_INTERVAL_SECONDS", 30),
			BatchSize:           pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_BATCH_SIZE", 50),
			LeaseSeconds:        pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_LEASE_SECONDS", 120),
			MaxRetry:            pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_MAX_RETRY", 5),
			BackoffBaseSeconds:  pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_BACKOFF_BASE_SECONDS", 60),
			BackoffMaxSeconds:   pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_BACKOFF_MAX_SECONDS", 1800),

			AuditIntentDeadlineMs:     pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_AUDIT_INTENT_DEADLINE_MS", 800),
			AuditConfirmDeadlineMs:    pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_AUDIT_CONFIRM_DEADLINE_MS", 400),
			AuditConfirmMaxAttempts:   pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_AUDIT_CONFIRM_MAX_ATTEMPTS", 3),
			AuditConfirmTotalBudgetMs: pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_AUDIT_CONFIRM_TOTAL_BUDGET_MS", 2000),
			TxBudgetMs:                pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_TX_BUDGET_MS", 5000),
			WindowHours:               pkgconfig.GetEnvAsIntOrDefault("SCHEDULED_POST_PUBLISH_WINDOW_HOURS", 24),
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks required configuration fields (fail-fast, never a warning).
func (c *Config) Validate() error {
	// MEDIA_BASE_URL 無預設域名，缺值即 fail-fast（backend#2283）：dev 一律 test-media、
	// prod 一律 media，不做新舊相容也不猜域名——漏設就啟動失敗，不允許靜默吐出死連結。
	if pkgconfig.MediaBaseURLMissing(c.MediaBaseURL) {
		return fmt.Errorf("missing required config: %s", pkgconfig.MediaBaseURLEnvKey)
	}
	return c.validateScheduledPostPublish()
}

// validateScheduledPostPublish enforces the three lease/backoff assertions of
// §8.5.1 (C). 值域逐項成立不保證不等式成立（例：lease 取 30 而 TX_BUDGET_MS 一起調到
// 上界 15000），所以真正的守門人是這三條。
func (c *Config) validateScheduledPostPublish() error {
	s := &c.ScheduledPostPublish

	// ① 租約下界：租約必須大於單筆最壞耗時，否則持有者還在發布租約就被合法搶走
	//    ⇒ 同一則貼文重複發布（#2294 C1 同型）。
	worstCaseMs := s.AuditIntentDeadlineMs + s.TxBudgetMs + s.AuditConfirmTotalBudgetMs
	if s.LeaseSeconds*1000 <= worstCaseMs {
		return fmt.Errorf("SCHEDULED_POST_PUBLISH_LEASE_SECONDS (%ds) must exceed worst-case per-row cost %dms (intent+tx+confirm); otherwise the same post can be published twice",
			s.LeaseSeconds, worstCaseMs)
	}

	// ② 租約上界：租約長度逼近候選窗口時 lease 會退化成永久旗標（§29 的 L4 形狀：
	//    24h 租約配 24h 窗口，實際語意變成 at-most-once）。
	if s.LeaseSeconds*10 >= s.WindowHours*3600 {
		return fmt.Errorf("SCHEDULED_POST_PUBLISH_LEASE_SECONDS (%ds) is too close to the candidate window (%dh): the lease degrades into a permanent flag",
			s.LeaseSeconds, s.WindowHours)
	}

	// ③ 退避序列走得完候選窗口：否則重試還沒用完該列就逾窗轉 expired，OBS8 的 reason
	//    全記成 window_exceeded，真正的 retry_exhausted 被蓋掉。
	windowSeconds := s.WindowHours * 3600
	if total := backoffSeriesTotalSeconds(s.BackoffBaseSeconds, s.BackoffMaxSeconds, s.MaxRetry); total >= windowSeconds {
		return fmt.Errorf("scheduled-post backoff series totals %ds, must be < candidate window %ds; rows would expire as window_exceeded before retries run out",
			total, windowSeconds)
	}

	return nil
}

// backoffSeriesTotalSeconds sums LEAST(base×2^(k-1), cap) for k = 1 … maxRetry-1.
// 達 MAX_RETRY 即轉 expired、不再排下一次，所以只加到 maxRetry-1 項。倍增在迴圈內
// 封頂，大的 maxRetry 不會讓位移溢位。
func backoffSeriesTotalSeconds(base, capSeconds, maxRetry int) int {
	total, delay := 0, base
	for k := 1; k <= maxRetry-1; k++ {
		if delay > capSeconds {
			delay = capSeconds
		}
		total += delay
		if delay < capSeconds {
			delay *= 2
		}
	}
	return total
}

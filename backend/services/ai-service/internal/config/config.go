package config

import (
	pkgotel "github.com/Will413028/hawkr/go-pkg/otel"
)

type Config struct {
	Env            string `env:"ENV" default:"development"`
	Server         ServerConfig
	LLMApiURL      string `env:"LLM_API_URL" default:"https://api.openai.com/v1"`
	LLMApiKey      string `env:"LLM_API_KEY"`
	LLMModel       string `env:"LLM_MODEL" default:"gpt-3.5-turbo"`
	GeminiAPIKey   string `env:"GEMINI_API_KEY"`
	GeminiModel    string `env:"GEMINI_MODEL" default:"gemini-2.5-flash"`
	ExtractorModel string `env:"EXTRACTOR_MODEL"`

	LLMTokenBudgetPerMinute string `env:"LLM_TOKEN_BUDGET_PER_MINUTE"`

	StaleExtractorProvider string `env:"EXTRACTOR_PROVIDER"`
	DeepSeekAPIKey         string `env:"DEEPSEEK_API_KEY"`
	OpenRouterAPIKey       string `env:"OPENROUTER_API_KEY"`
	OrderServiceHost       string `env:"ORDER_SERVICE_HOST" default:"127.0.0.1"`
	OrderServicePort       string `env:"ORDER_SERVICE_GRPC_PORT" default:"9915"`
	ProductServiceHost     string `env:"PRODUCT_SERVICE_HOST" default:"127.0.0.1"`
	ProductServicePort     string `env:"PRODUCT_SERVICE_GRPC_PORT" default:"9913"`
	SellerServiceHost      string `env:"SELLER_SERVICE_HOST" default:"127.0.0.1"`
	SellerServicePort      string `env:"SELLER_SERVICE_GRPC_PORT" default:"9941"`
	BuyerServiceHost       string `env:"BUYER_SERVICE_HOST" default:"127.0.0.1"`
	BuyerServicePort       string `env:"BUYER_SERVICE_GRPC_PORT" default:"9941"`
	MediaServiceHost       string `env:"MEDIA_SERVICE_HOST" default:"127.0.0.1"`
	MediaServicePort       string `env:"MEDIA_SERVICE_GRPC_PORT" default:"9937"`

	// ProcurementSettlementStoreID 是 EXTERNAL lane 的訂單賣方——站外業者不是平台
	// 賣家，RFQ 訂單的賣方記為平台結算 store（doc 64 §6.4、§6.4.8）。
	// ⚠️ 非敏感，放 config 不放 secretRef。未設定（0）不會讓 worker 停擺，只有
	// EXTERNAL lane 的列會終態失敗並記下 CONFIGURATION_MISSING。
	// 🔴 §6.4.8：它必須是 seller-service 真的查得到的 store，不只是一個設定值——
	// gateway 的 batchGetSellerMeta 對查不到的 store 是 fail-soft，後果是買家在訂單上
	// 看到沒有賣方。
	ProcurementSettlementStoreID int64 `env:"PROCUREMENT_SETTLEMENT_STORE_ID" default:"0"`

	DBHost string `env:"DB_HOST" default:"127.0.0.1"`

	DBPort     string `env:"DB_POOL_PORT,DB_PORT" default:"5432"`
	DBUser     string `env:"DB_USER" default:"dbadmin"`
	DBPassword string `env:"DB_PASSWORD"`
	DBName     string `env:"DB_NAME" default:"hawkr_ai"`
	DBSSLMode  string `env:"DB_SSLMODE" default:"disable"`

	RabbitMQ RabbitMQConfig
}

type ServerConfig struct {
	GRPCPort string `env:"GRPC_PORT" default:"9939"`
}

type RabbitMQConfig struct {
	URL string `env:"RABBITMQ_URL" default:"amqp://guest:guest@localhost:5672/"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := fillFromEnv(cfg); err != nil {
		return nil, err
	}
	applyDerivedFields(cfg)
	return cfg, nil
}

func applyDerivedFields(cfg *Config) {

	if cfg.ExtractorModel != "" {
		cfg.StaleExtractorProvider = ""
	}
}

func LoadValidated() (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	WarnUnknownEnv(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	return pkgotel.ValidateMetricsConfiguration(c.Env)
}

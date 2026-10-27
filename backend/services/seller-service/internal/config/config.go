package config

import (
	"fmt"
	"strings"
	"unicode"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
	"go.uber.org/zap"
)

// Config holds all configuration for the service.
type Config struct {
	Database               DatabaseConfig
	Server                 ServerConfig
	Env                    string
	MediaBaseURL           string
	WebBaseUrl             string
	WebRevalidate          WebRevalidateConfig
	AccountService         AccountServiceConfig
	MediaService           MediaServiceConfig
	OrderService           OrderServiceConfig
	BuyerService           BuyerServiceConfig
	ReviewService          ReviewServiceConfig
	SocialService          SocialServiceConfig
	ProductService         ProductServiceConfig
	RabbitMQ               RabbitMQConfig
	Redis                  RedisConfig
	Payuni                 PayuniConfig
	TGOSBaseURL            string
	TGOSQueryAddrAppID     string
	TGOSQueryAddrAPIKey    string
	TGOSProxyCallerKeyFile string

	// EncryptionMasterKey is the AES-256-GCM master secret for at-rest PII
	// encryption (certification id_card_no, issue #608). Fail-closed: required
	// non-empty by Validate() so the service refuses to start without a key and
	// never downgrades to storing plaintext.
	EncryptionMasterKey string

	// FeatureFlags B-169 (D18) — D9 不上 Discord 後僅留 endpoint flag。
	FeatureFlags FeatureFlagsConfig
}

// RabbitMQConfig holds RabbitMQ connection configuration.
type RabbitMQConfig struct {
	URL string
}

// RedisConfig B-169 (D8) public violation cache 用；fail-open。
type RedisConfig struct {
	Host, Port, Password string
}

// FeatureFlagsConfig B-169 (D18) emergency kill switch.
type FeatureFlagsConfig struct {
	FoodSafetyEndpointEnabled bool
}

// MediaServiceConfig holds media service gRPC configuration.
type MediaServiceConfig struct {
	Addr string
}

// OrderServiceConfig holds order service gRPC configuration.
type OrderServiceConfig struct {
	Addr string
}

// BuyerServiceConfig holds buyer service gRPC configuration.
type BuyerServiceConfig struct {
	Addr string
}

// ReviewServiceConfig holds review service gRPC configuration.
type ReviewServiceConfig struct {
	Addr string
}

// SocialServiceConfig holds social service gRPC configuration.
type SocialServiceConfig struct {
	Addr string
}

// ProductServiceConfig holds product service gRPC configuration.
type ProductServiceConfig struct {
	Addr string
}

// AccountServiceConfig holds account service gRPC configuration.
type AccountServiceConfig struct {
	Addr string
}

// WebRevalidateConfig holds hawkr-website /api/revalidate caller config.
// 後端服務責任邊界規範 §9.7。BaseURL+Secret 缺值時 client noop（local dev 不對接 web）。
type WebRevalidateConfig struct {
	BaseURL string
	Secret  string
}

// PayuniConfig holds PAYUNi payment gateway configuration.
type PayuniConfig struct {
	AgentID   string
	HashKey   string
	HashIV    string
	IsSandbox bool
	// SandboxMerIDOverride 是「本地開發用固定商店代號」的覆寫值，**只在 sandbox 模式下非空**。
	//
	// 🔴 這個欄位存在的理由是它原本不存在：覆寫原本寫在 usecase 裡直接
	// os.Getenv("PAYUNI_SANDBOX_MER_ID")，**無條件生效、不看 IsSandbox**。而 prod 的
	// hawkr-payuni secret 實際帶著 PAYUNI_SANDBOX_MER_ID=HAWK9（docker-compose-local
	// 的本地預設值），所以只要有人開始消費那個回傳欄位，prod 就會拿到測試商店代號（#3333）。
	// 把判定收在這裡，usecase 只讀值——非 sandbox 時它恆為空字串，覆寫路徑走不到。
	SandboxMerIDOverride string
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
		TimeZone: "UTC",
	})
}

// Load loads configuration from environment variables.
// Returns error if required fields are missing (fail-fast).
// resolveSandboxMerIDOverride 決定 PAYUNI_SANDBOX_MER_ID 要不要帶進設定。
//
// 抽成純函式是為了讓它**可以被單獨測**：Load() 內含 Validate()，而 Validate 的必填清單
// 會隨時間變（目前含 TGOS 四項），測試若走 Load 就綁在那份清單上，改動無關的必填項
// 會讓這支測試一起紅。
//
// 判準只有一條：**非 sandbox 模式一律回空字串**，即使 env 有值。
// 理由見 PayuniConfig.SandboxMerIDOverride 的註解（#3333）。
func resolveSandboxMerIDOverride(sandboxFlag, rawMerID string) string {
	if sandboxFlag != "true" {
		return ""
	}
	return strings.TrimSpace(rawMerID)
}

func Load() (*Config, error) {
	cfg := &Config{
		Env: pkgconfig.GetEnvOrDefault("ENV", "development"),
		// 無預設值，缺值由 Validate() fail-fast（backend#2283）：媒體域名不做新舊相容，
		// dev 一律 test-media.example.com、prod 一律 media.example.com。
		MediaBaseURL: pkgconfig.GetEnvOrDefault("MEDIA_BASE_URL", ""),
		WebBaseUrl:   pkgconfig.GetEnvOrDefault("WEB_BASE_URL", "http://localhost:3000"),
		WebRevalidate: WebRevalidateConfig{
			BaseURL: pkgconfig.GetEnvOrDefault("WEB_REVALIDATE_BASE_URL", ""),
			Secret:  pkgconfig.GetEnvOrDefault("WEB_REVALIDATE_SECRET", ""),
		},
		Database: DatabaseConfig{
			Host:     pkgconfig.GetEnvOrDefault("DB_HOST", "postgres"),
			Port:     database.ResolvePoolPort(),
			User:     pkgconfig.GetEnvOrDefault("DB_USER", "postgres"),
			Password: pkgconfig.GetEnvOrDefault("DB_PASSWORD", "postgres"),
			DBName:   pkgconfig.GetEnvOrDefault("DB_NAME", "hawkr_seller"),
			SSLMode:  pkgconfig.GetEnvOrDefault("DB_SSLMODE", "disable"),
		},
		Server: ServerConfig{
			GRPCPort: pkgconfig.GetEnvOrDefault("GRPC_PORT", "10020"),
		},
		Payuni: PayuniConfig{
			AgentID:   strings.TrimSpace(pkgconfig.GetEnvOrDefault("PAYUNI_AGENT_ID", "")),
			HashKey:   strings.TrimSpace(pkgconfig.GetEnvOrDefault("PAYUNI_AGENT_HASH_KEY", "")),
			HashIV:    strings.TrimSpace(pkgconfig.GetEnvOrDefault("PAYUNI_AGENT_HASH_IV", "")),
			IsSandbox: pkgconfig.GetEnvOrDefault("PAYUNI_SANDBOX", "true") == "true",
			// 非 sandbox 一律空字串——即使 env 有值也不帶進來（見 SandboxMerIDOverride 註解）。
			SandboxMerIDOverride: resolveSandboxMerIDOverride(
				pkgconfig.GetEnvOrDefault("PAYUNI_SANDBOX", "true"),
				pkgconfig.GetEnvOrDefault("PAYUNI_SANDBOX_MER_ID", ""),
			),
		},
		TGOSBaseURL:            pkgconfig.GetEnvOrDefault("TGOS_BASE_URL", ""),
		TGOSQueryAddrAppID:     pkgconfig.GetEnvOrDefault("TGOS_QUERYADDR_APPID", ""),
		TGOSQueryAddrAPIKey:    pkgconfig.GetEnvOrDefault("TGOS_QUERYADDR_APIKEY", ""),
		TGOSProxyCallerKeyFile: pkgconfig.GetEnvOrDefault("TGOS_PROXY_CALLER_KEY_FILE", ""),
		EncryptionMasterKey:    strings.TrimSpace(pkgconfig.GetEnvOrDefault("ENCRYPTION_MASTER_KEY", "")),
		AccountService: AccountServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("ACCOUNT_SERVICE_ADDR", "account-service:9941"),
		},
		MediaService: MediaServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("MEDIA_SERVICE_ADDR", "media-service:9937"),
		},
		OrderService: OrderServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("ORDER_SERVICE_ADDR", "order-service:9915"),
		},
		BuyerService: BuyerServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("BUYER_SERVICE_ADDR", "127.0.0.1:9941"),
		},
		ReviewService: ReviewServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("REVIEW_SERVICE_ADDR", "review-service:10033"),
		},
		SocialService: SocialServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("SOCIAL_SERVICE_ADDR", "social-service:9918"),
		},
		ProductService: ProductServiceConfig{
			Addr: pkgconfig.GetEnvOrDefault("PRODUCT_SERVICE_ADDR", "product-service:9913"),
		},
		RabbitMQ: RabbitMQConfig{
			URL: pkgconfig.GetEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		},
		Redis: RedisConfig{
			Host:     pkgconfig.GetEnvOrDefault("REDIS_HOST", "redis"),
			Port:     pkgconfig.GetEnvOrDefault("REDIS_PORT", "6379"),
			Password: pkgconfig.GetEnvOrDefault("REDIS_PASSWORD", ""),
		},
		FeatureFlags: FeatureFlagsConfig{
			FoodSafetyEndpointEnabled: pkgconfig.GetEnvOrDefault("FOOD_SAFETY_ENDPOINT_ENABLED", "false") == "true",
		},
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks required configuration fields and returns an error
// listing all missing or invalid values (fail-fast on startup).
func (c *Config) Validate() error {
	var missing []string
	var conflicts []string

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
	// 🔴 PAYUNI_AGENT_HASH_KEY／IV 刻意**不在**必填清單（#3329）。兩者皆空 ＝ 金流商家
	// 建檔尚未開通，是一個**合法的部署狀態**：seller-service 的 91 支 RPC 裡只有建檔／
	// 狀態同步／商家資料更新那幾支用得到，店家 CRUD、商品、違規、會員全部不碰。
	// fail-closed 移到使用點：client/payuni 的 disabledClient 一律回 9704。
	// Fail-closed: at-rest PII encryption (id_card_no, #608) must have a key.
	if c.EncryptionMasterKey == "" {
		missing = append(missing, "ENCRYPTION_MASTER_KEY")
	}
	// 🔴 TGOS 四項同樣不在必填清單（#3329），理由與上面同型：91 支 RPC 裡只有
	// SuggestAddress／ValidateAddress 兩支用得到。把單一功能的憑證列成啟動條件，
	// 會讓維運在「未開通」時只剩 replicas: 0 這條路，於是 89 支無關的 RPC 一起靜默消失
	// （#3230 的形狀）。fail-closed 見 usecase.ErrAddressLookupDisabled（9700）。

	// MEDIA_BASE_URL 無預設域名，缺值即 fail-fast（backend#2283）：dev 一律 test-media、
	// prod 一律 media，不做新舊相容也不猜域名——漏設就啟動失敗，不允許靜默吐出死連結。
	if c.MediaBaseURL == "" {
		missing = append(missing, "MEDIA_BASE_URL")
	}
	if len(missing) > 0 || len(conflicts) > 0 {
		var msgs []string
		if len(missing) > 0 {
			msgs = append(msgs, "missing required config: "+strings.Join(missing, ", "))
		}
		if len(conflicts) > 0 {
			msgs = append(msgs, "invalid config: "+strings.Join(conflicts, "; "))
		}
		return fmt.Errorf("%s", strings.Join(msgs, " | "))
	}

	zap.L().Info("PAYUNi config loaded",
		zap.String("agentID", c.Payuni.AgentID),
		zap.Int("keyLen", len(c.Payuni.HashKey)),
		zap.Int("ivLen", len(c.Payuni.HashIV)),
		zap.Bool("sandbox", c.Payuni.IsSandbox))
	// 兩者皆空 ＝ 尚未開通，是合法狀態，不該噴「長度不符預期」的警告——那會讓每個
	// 未開通環境的啟動 log 都帶著兩行看起來像故障的 WARN，稀釋掉真正的配置錯誤。
	if !c.PayuniMerchantConfigured() && !c.PayuniMerchantPartiallyConfigured() {
		return nil
	}
	if len(c.Payuni.HashKey) != 32 {
		zap.L().Warn("PAYUNI_AGENT_HASH_KEY 長度不符預期", zap.Int("actual", len(c.Payuni.HashKey)), zap.Int("expected", 32))
	}
	if len(c.Payuni.HashIV) != 16 {
		zap.L().Warn("PAYUNI_AGENT_HASH_IV 長度不符預期", zap.Int("actual", len(c.Payuni.HashIV)), zap.Int("expected", 16))
	}
	if containsNonASCII(c.Payuni.HashKey) {
		zap.L().Warn("PAYUNI_AGENT_HASH_KEY 含非 ASCII 字元，可能有 BOM 或編碼問題")
	}
	if containsNonASCII(c.Payuni.HashIV) {
		zap.L().Warn("PAYUNI_AGENT_HASH_IV 含非 ASCII 字元，可能有 BOM 或編碼問題")
	}

	return nil
}

// PayuniMerchantConfigured 回報金流商家建檔（PayUni 代理商憑證）是否已開通。
func (c *Config) PayuniMerchantConfigured() bool {
	return c.Payuni.HashKey != "" && c.Payuni.HashIV != ""
}

// PayuniMerchantPartiallyConfigured 回報「只設了一半」——那是配置錯誤，不是未開通。
//
// 🔴 與 Configured 分成兩支而不是一個布林：全有與全無都合法，只有一半才要有人去修。
// 同 invoice-service platformCredentialIncomplete 的三分支結構。
func (c *Config) PayuniMerchantPartiallyConfigured() bool {
	return (c.Payuni.HashKey == "") != (c.Payuni.HashIV == "")
}

// GeocodeConfigured 回報地址查詢（TGOS）是否已開通：四個設定值**全部**非空才算。
//
// 「全部」是刻意的：少任何一個都組不出一次成功的 TGOS 呼叫，回 true 只會把失敗從
// 開機延後到第一個打進來的使用者身上。
func (c *Config) GeocodeConfigured() bool {
	return c.TGOSBaseURL != "" && c.TGOSQueryAddrAppID != "" &&
		c.TGOSQueryAddrAPIKey != "" && c.TGOSProxyCallerKeyFile != ""
}

// GeocodePartiallyConfigured 回報設定「只有一半」——四項裡有非空的、但不是全部。
func (c *Config) GeocodePartiallyConfigured() bool {
	set := 0
	for _, v := range []string{c.TGOSBaseURL, c.TGOSQueryAddrAppID, c.TGOSQueryAddrAPIKey, c.TGOSProxyCallerKeyFile} {
		if v != "" {
			set++
		}
	}
	return set > 0 && set < 4
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

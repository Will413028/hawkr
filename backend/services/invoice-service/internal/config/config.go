// Package config 載入並驗證 invoice-service 的全部環境變數。
//
// 環境變數清單的 SSOT 是 58 號設計書 §12.1.2（A~F 六組）。這一層是「設定了但沒人讀」
// 與「讀了但名字打錯」唯一會被抓到的地方（§12.1.1 第 5 列），所以每個變數都必須在
// Load 內出現一次。
package config

import (
	"fmt"
	"strconv"
	"strings"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/database"
)

// Config holds all configuration for invoice-service.
type Config struct {
	Env        string
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	MQ         MQConfig
	Encryption EncryptionConfig
	Amego      AmegoConfig
	Schedule   ScheduleConfig
	Governance GovernanceConfig
	Peers      PeerConfig
	MediaBase  string
	Receipt    ReceiptConfig
}

// ReceiptConfig 是平台代製收據（PLATFORM_RECEIPT，#3026）的產製設定。
//
// 🔴 FontPath 是 **fail-closed 的硬依賴**，不是選配：收據全篇中文，字型缺席時
// go-pdf/fpdf 產出的是**一份合法但整頁空白的 PDF**——`%PDF-` magic bytes 對、
// HTTP 200、下載得到，只有內容不見了。那種失敗沒有任何一道下游 gate 攔得住，
// 所以要在 boot 期擋（Validate 會實際把檔案讀起來 parse 一次，不只檢查路徑存在）。
//
// ⚠️ **必須是 TTF，不能是 TTC**：alpine 的 CJK 套件全是 TTC，而 fpdf 對 TTC 回
// "get metrics Error: not supported"。Dockerfile 的 font stage 抓的是釘住 SHA256 的
// Noto Sans TC TTF，兩個 stage（dev／prod）都 COPY 一份。
type ReceiptConfig struct {
	FontPath string
}

// ServerConfig holds the three independent listening surfaces (§4.3.1).
//
// FilePort 是 PDF 授權下載通道，必須是獨立的 http.Server —— 不得掛在 HTTPPort 的
// 共用運維 router 上，也不能併進 gRPC 面（gRPC 不承載 bytes）。
type ServerConfig struct {
	HTTPPort string
	GRPCPort string
	FilePort string
}

// DatabaseConfig holds PostgreSQL configuration.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
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

// RedisConfig holds Redis connection settings.
//
// 冪等記錄不放 Redis（§12.1.2 E）；這裡只給快取類用途。
type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

// MQConfig holds RabbitMQ settings.
type MQConfig struct {
	URL                  string
	PassiveDeclare       bool
	PublishTimeoutSecond int
}

// EncryptionConfig holds AppKey at-rest encryption material (§12.1.2 A).
//
// MasterKeyVersion 是整數不是字串標籤（credential_key_version 是 smallint）。
// 沒有 INVOICE_ENCRYPTION_MASTER_KEY_ID 這個變數。
type EncryptionConfig struct {
	MasterKey          string
	MasterKeyVersion   int
	PrevMasterKey      string
	PrevMasterKeyVer   int
	LookupHMACKey      string
	prevVersionPresent bool
}

// AmegoConfig holds AMEGO (光貿) provider settings (§12.1.2 B).
type AmegoConfig struct {
	BaseURL            string
	AllowRealInvoice   bool
	TestInvoice        string
	PlatformInvoice    string
	PlatformAppKey     string
	ProxyCallerKeyFile string
	TimeoutIssueMS     int
	TimeoutQueryMS     int
	RateLimitQPS       int
	CBFailureThreshold int
	CBTimeoutSeconds   int
	CBMaxRequests      int
}

// ScheduleConfig holds scheduler / lease settings (§12.1.2 C).
type ScheduleConfig struct {
	IssuePollIntervalSeconds int
	IssueBatchSize           int
	IssueLeaseSeconds        int
	IssueMaxRetry            int
	BackoffBaseSeconds       int
	BackoffMaxSeconds        int
	RecheckLeaseSeconds      int
	RecheckMaxAttempts       int
	RotationTimeoutMinutes   int
	PDFMaxBytes              int64
	TrackBalanceCheckHourUTC int
	TrackBalanceWarnThresh   int
}

// GovernanceConfig holds business governance parameters (§12.1.2 D).
type GovernanceConfig struct {
	TaxReviewGraceDays        int
	TaxReviewPendingMaxOrders int
	TaxReviewPendingMaxAmount int64
	ReapplyCooldownDays       int
	ComplaintSLAWorkingDays   int
	ViolationWindowDays       int
	ViolationSuspendThreshold int
	SuspensionDays            int
	DocumentRetentionDays     int
	AuditRetentionDays        int
	ManualCapacityPerDay      int
	TrackGrowthHeadroom       float64
	AllowanceB2CSplitTax      bool
}

// PeerConfig holds downstream gRPC addresses (§12.1.2 E).
type PeerConfig struct {
	OrderAddr string
	// PaymentAddr 是 PAY peer client 的位址（§7.1.2 步驟 5 ②）。
	// `PAYMENT_SERVICE_ADDR` 已登記在 k8s/values/invoice-service.yaml 與
	// docker-compose-local.yml 的 invoice 區塊；`.env.template` 依既有慣例
	// 不放任何 `*_SERVICE_ADDR`（全檔零命中），故不需補。
	PaymentAddr  string
	SellerAddr   string
	BuyerAddr    string
	ShippingAddr string
	MediaAddr    string
}

// Load loads configuration from environment variables and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		Env: pkgconfig.GetEnvOrDefault("ENV", "development"),
		Receipt: ReceiptConfig{
			FontPath: pkgconfig.GetEnvOrDefault("RECEIPT_FONT_PATH", "/app/fonts/NotoSansTC.ttf"),
		},
		Server: ServerConfig{
			HTTPPort: pkgconfig.GetEnvOrDefault("HTTP_PORT", "10040"),
			GRPCPort: pkgconfig.GetEnvOrDefault("GRPC_PORT", "10041"),
			FilePort: pkgconfig.GetEnvOrDefault("INVOICE_SERVICE_FILE_PORT", "10042"),
		},
		Database: DatabaseConfig{
			Host:     pkgconfig.GetEnvOrDefault("DB_HOST", "localhost"),
			Port:     database.ResolvePoolPort(),
			User:     pkgconfig.GetEnvOrDefault("DB_USER", "postgres"),
			Password: pkgconfig.GetEnvOrDefault("DB_PASSWORD", "postgres"),
			DBName:   pkgconfig.GetEnvOrDefault("DB_NAME", "hawkr_invoice"),
			SSLMode:  pkgconfig.GetEnvOrDefault("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     pkgconfig.GetEnvOrDefault("REDIS_HOST", "localhost"),
			Port:     pkgconfig.GetEnvOrDefault("REDIS_PORT", "6379"),
			Password: pkgconfig.GetEnvOrDefault("REDIS_PASSWORD", ""),
		},
		MQ: MQConfig{
			URL:                  pkgconfig.GetEnvOrDefault("RABBITMQ_URL", ""),
			PassiveDeclare:       envBool("MQ_PASSIVE_DECLARE", false),
			PublishTimeoutSecond: envInt("INVOICE_MQ_PUBLISH_TIMEOUT_SECONDS", 10),
		},
		Encryption: EncryptionConfig{
			MasterKey:          pkgconfig.GetEnvOrDefault("INVOICE_ENCRYPTION_MASTER_KEY", ""),
			MasterKeyVersion:   envInt("INVOICE_ENCRYPTION_MASTER_KEY_VERSION", 1),
			PrevMasterKey:      pkgconfig.GetEnvOrDefault("INVOICE_ENCRYPTION_MASTER_KEY_PREV", ""),
			PrevMasterKeyVer:   envInt("INVOICE_ENCRYPTION_MASTER_KEY_PREV_VERSION", 0),
			LookupHMACKey:      pkgconfig.GetEnvOrDefault("INVOICE_LOOKUP_HMAC_KEY", ""),
			prevVersionPresent: strings.TrimSpace(pkgconfig.GetEnvOrDefault("INVOICE_ENCRYPTION_MASTER_KEY_PREV_VERSION", "")) != "",
		},
		Amego: AmegoConfig{
			BaseURL:            pkgconfig.GetEnvOrDefault("AMEGO_BASE_URL", ""),
			AllowRealInvoice:   envBool("AMEGO_ALLOW_REAL_INVOICE", false),
			TestInvoice:        pkgconfig.GetEnvOrDefault("AMEGO_TEST_INVOICE", ""),
			PlatformInvoice:    pkgconfig.GetEnvOrDefault("AMEGO_PLATFORM_INVOICE", ""),
			PlatformAppKey:     pkgconfig.GetEnvOrDefault("AMEGO_PLATFORM_APP_KEY", ""),
			ProxyCallerKeyFile: pkgconfig.GetEnvOrDefault("AMEGO_PROXY_CALLER_KEY_FILE", ""),
			TimeoutIssueMS:     envInt("AMEGO_HTTP_TIMEOUT_ISSUE_MS", 30000),
			TimeoutQueryMS:     envInt("AMEGO_HTTP_TIMEOUT_QUERY_MS", 10000),
			RateLimitQPS:       envInt("AMEGO_CLIENT_RATE_LIMIT_QPS", 5),
			CBFailureThreshold: envInt("AMEGO_CB_FAILURE_THRESHOLD", 5),
			CBTimeoutSeconds:   envInt("AMEGO_CB_TIMEOUT_SECONDS", 10),
			CBMaxRequests:      envInt("AMEGO_CB_MAX_REQUESTS", 3),
		},
		Schedule: ScheduleConfig{
			IssuePollIntervalSeconds: envInt("INVOICE_ISSUE_POLL_INTERVAL_SECONDS", 10),
			IssueBatchSize:           envInt("INVOICE_ISSUE_BATCH_SIZE", 50),
			IssueLeaseSeconds:        envInt("INVOICE_ISSUE_LEASE_SECONDS", 120),
			IssueMaxRetry:            envInt("INVOICE_ISSUE_MAX_RETRY", 13),
			BackoffBaseSeconds:       envInt("INVOICE_BACKOFF_BASE_SECONDS", 30),
			BackoffMaxSeconds:        envInt("INVOICE_BACKOFF_MAX_SECONDS", 1800),
			RecheckLeaseSeconds:      envInt("INVOICE_RECHECK_LEASE_SECONDS", 60),
			RecheckMaxAttempts:       envInt("INVOICE_RECHECK_MAX_ATTEMPTS", 12),
			RotationTimeoutMinutes:   envInt("INVOICE_ROTATION_TIMEOUT_MINUTES", 30),
			PDFMaxBytes:              int64(envInt("INVOICE_PDF_MAX_BYTES", 4194304)),
			TrackBalanceCheckHourUTC: envInt("INVOICE_TRACK_BALANCE_CHECK_HOUR_UTC", 1),
			TrackBalanceWarnThresh:   envInt("INVOICE_TRACK_BALANCE_WARN_THRESHOLD", 200),
		},
		Governance: GovernanceConfig{
			TaxReviewGraceDays:        envInt("INVOICE_TAX_REVIEW_GRACE_DAYS", 60),
			TaxReviewPendingMaxOrders: envInt("INVOICE_TAX_REVIEW_PENDING_MAX_ORDERS", 50),
			TaxReviewPendingMaxAmount: int64(envInt("INVOICE_TAX_REVIEW_PENDING_MAX_AMOUNT", 100000)),
			ReapplyCooldownDays:       envInt("INVOICE_REAPPLY_COOLDOWN_DAYS", 7),
			ComplaintSLAWorkingDays:   envInt("INVOICE_COMPLAINT_SLA_WORKING_DAYS", 3),
			ViolationWindowDays:       envInt("INVOICE_VIOLATION_WINDOW_DAYS", 90),
			ViolationSuspendThreshold: envInt("INVOICE_VIOLATION_SUSPEND_THRESHOLD", 3),
			SuspensionDays:            envInt("INVOICE_SUSPENSION_DAYS", 14),
			DocumentRetentionDays:     envInt("INVOICE_DOCUMENT_RETENTION_DAYS", 1826),
			AuditRetentionDays:        envInt("INVOICE_AUDIT_RETENTION_DAYS", 1826),
			ManualCapacityPerDay:      envInt("INVOICE_MANUAL_CAPACITY_PER_DAY", 20),
			TrackGrowthHeadroom:       envFloat("INVOICE_TRACK_GROWTH_HEADROOM", 0.5),
			AllowanceB2CSplitTax:      envBool("INVOICE_ALLOWANCE_B2C_SPLIT_TAX", true),
		},
		Peers: PeerConfig{
			OrderAddr:    pkgconfig.GetEnvOrDefault("ORDER_SERVICE_ADDR", "order-service:9915"),
			PaymentAddr:  pkgconfig.GetEnvOrDefault("PAYMENT_SERVICE_ADDR", "payment-service:9925"),
			SellerAddr:   pkgconfig.GetEnvOrDefault("SELLER_SERVICE_ADDR", "127.0.0.1:9941"),
			BuyerAddr:    pkgconfig.GetEnvOrDefault("BUYER_SERVICE_ADDR", "127.0.0.1:9941"),
			ShippingAddr: pkgconfig.GetEnvOrDefault("SHIPPING_SERVICE_ADDR", "shipping-service:9931"),
			MediaAddr: pkgconfig.GetEnvOrDefault("MEDIA_SERVICE_HOST", "media-service") + ":" +
				pkgconfig.GetEnvOrDefault("MEDIA_SERVICE_GRPC_PORT", "9937"),
		},
		MediaBase: pkgconfig.GetEnvOrDefault("MEDIA_BASE_URL", ""),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate fails closed. 每一條的失敗形狀寫在該條註解裡；不要為了讓本地方便啟動而放寬。
//
// 已實作：設定自相矛盾類 ＋ **§4.5.8 環境 fail-closed 的第一層（啟動時，三條互斥斷言）**。
//
// 第二層不在這裡，但**已經實作**：§4.5.8 的 `guardEnvironment(cred)` 落在
// internal/client/amego/client.go 的 do()，是所有 POST 的唯一入口。啟動時的斷言只涵蓋
// 平台自身那一組憑證，而 per-seller 憑證是 runtime 從 DB 讀出來的、Validate 跑完時還不
// 存在，所以那一層必須在 client。§8.1 的 KeyRing（①–⑫ 的其餘各條）仍待補。
//
// ⚠️ 這段註解曾經寫著「第二層尚未實作」，而它早已落地——**低報現有保護比高報更難發現**：
// 高報會被事故戳破，低報只會讓人重複實作或誤開 issue，沒有任何事故來糾正它。
func (c *Config) Validate() error {
	var missing []string
	var conflicts []string

	if c.Database.Host == "" {
		missing = append(missing, "DB_HOST")
	}
	if c.Database.User == "" {
		missing = append(missing, "DB_USER")
	}
	if c.Database.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	// 三個 port 是三個不同的面，共用一個值會讓其中兩個面靜默搶不到 listener。
	if c.Server.HTTPPort == c.Server.GRPCPort ||
		c.Server.HTTPPort == c.Server.FilePort ||
		c.Server.GRPCPort == c.Server.FilePort {
		conflicts = append(conflicts, fmt.Sprintf(
			"HTTP_PORT/GRPC_PORT/INVOICE_SERVICE_FILE_PORT 必須三個互不相同（現值 %s/%s/%s）",
			c.Server.HTTPPort, c.Server.GRPCPort, c.Server.FilePort))
	}
	// AppKey at-rest 加密：缺任一把就等於「賣家憑證明文落庫」或「解不開」，一律 fail-closed。
	if c.Encryption.MasterKey == "" {
		missing = append(missing, "INVOICE_ENCRYPTION_MASTER_KEY")
	}
	if c.Encryption.LookupHMACKey == "" {
		missing = append(missing, "INVOICE_LOOKUP_HMAC_KEY")
	}
	// 指紋 HMAC 與 at-rest 金鑰共用一把 ⇒ 指紋等於用加密金鑰做的可預測推導，失去分離的意義。
	if c.Encryption.MasterKey != "" && c.Encryption.MasterKey == c.Encryption.LookupHMACKey {
		conflicts = append(conflicts, "INVOICE_LOOKUP_HMAC_KEY 不得等於 INVOICE_ENCRYPTION_MASTER_KEY")
	}
	// credential_key_version 是 smallint，值域 ^[1-9][0-9]{0,4}$。
	// 上界是 32767 不是 99999：credential_key_version 是 smallint（§5.1.3）。
	// 落在 (32767, 99999] 的值在 boot 期會通過，卻在**寫入第一列憑證時**才撞 22003
	// ——那時錯誤出現在一個與設定無關的地方，而 pod 已經 Ready。
	// 設計書 §8.5.1／§12.1.2 標的 `^[1-9][0-9]{0,4}$` 同樣過寬（最大 99999），已一併修正。
	if c.Encryption.MasterKeyVersion < 1 || c.Encryption.MasterKeyVersion > 32767 {
		conflicts = append(conflicts, fmt.Sprintf(
			"INVOICE_ENCRYPTION_MASTER_KEY_VERSION 必須在 1~32767（smallint 上限，現值 %d）", c.Encryption.MasterKeyVersion))
	}
	// 輪替期的兩個值必須成對出現，且版本不得與現行版本相同（否則解不出該用哪一把）。
	prevKeySet := c.Encryption.PrevMasterKey != ""
	if prevKeySet != c.Encryption.prevVersionPresent {
		conflicts = append(conflicts,
			"INVOICE_ENCRYPTION_MASTER_KEY_PREV 與 INVOICE_ENCRYPTION_MASTER_KEY_PREV_VERSION 必須成對設定")
	}
	if prevKeySet && c.Encryption.PrevMasterKeyVer == c.Encryption.MasterKeyVersion {
		conflicts = append(conflicts, "INVOICE_ENCRYPTION_MASTER_KEY_PREV_VERSION 不得等於 _VERSION")
	}
	// AMEGO base URL 沒有預設值：預填會讓「照抄 template 的 prod .env」看起來已填好，
	// 而測試與正式共用同一個網域且回應無環境標記 ⇒ 覆蓋錯了不會連不上，只會靜默開出
	// 不進任何公司帳的發票。
	if c.Amego.BaseURL == "" {
		missing = append(missing, "AMEGO_BASE_URL")
	}
	if c.Amego.TestInvoice == "" {
		missing = append(missing, "AMEGO_TEST_INVOICE")
	}
	// AMEGO_PLATFORM_INVOICE 刻意**不在**必填清單（#3230）。空值 ＝ 平台自身的開票功能
	// 尚未開通，那是一個合法狀態：平台 issuer 要等平台開始向賣家收費才會用到。
	//
	// 🔴 把它列為必填的實際後果不是「有人補上值」，而是 prod 部署者把 replicas 設成 0
	// 繞過 boot 失敗 ⇒ 整個服務連同 76 條發票路由一起消失，而 deploy job 全綠、
	// kubectl 顯示 0/0、沒有任何部署面訊號。#3230 就是這樣發生的：206 次請求全部 503、
	// 成功率 0%、持續三天無人察覺。
	//
	// fail-closed 一個位元都沒有放鬆，只是換了層級：下面兩條 conflict 斷言以
	// 「TestInvoice 與 PlatformInvoice 皆非空」為前提，空值時自動跳過（空統編既不是
	// 「用了正式統編」也不是「用了測試統編」）；真正要擋的「拿空統編去開票」由
	// usecase/admin_issuance.go 的 resolveIssuer PLATFORM 分支在**開票當下**擋下。

	// 環境 fail-closed 的第一層（§4.5.8）。AMEGO 的測試與正式**共用同一個網域，回應
	// 也沒有任何環境標記** ⇒ 設定填錯不會連不上、不會報錯，只會靜默開出一張性質完全
	// 相反的真發票，而且不可回收。所以這三條是雙向的，兩個方向各自都是災難：
	//
	//   方向 A（未開放真發票卻用正式統編）：對真實公司開出無法回收的真發票
	//   方向 B（開放真發票卻用測試統編）：開出的發票不進入任何公司的帳
	//
	// 判定「准不准開真發票」的是 AMEGO_ALLOW_REAL_INVOICE，**不是 Env == "prod"**
	// （§4.6 W-21）。Env 只用在第 3 條的反向斷言。
	if c.Amego.TestInvoice != "" && c.Amego.PlatformInvoice != "" {
		if !c.Amego.AllowRealInvoice && c.Amego.PlatformInvoice != c.Amego.TestInvoice {
			conflicts = append(conflicts, fmt.Sprintf(
				"AMEGO_ALLOW_REAL_INVOICE=false 的環境禁止使用正式統編："+
					"AMEGO_PLATFORM_INVOICE(%s) != AMEGO_TEST_INVOICE(%s)，"+
					"這會對真實公司開出無法回收的真發票",
				c.Amego.PlatformInvoice, c.Amego.TestInvoice))
		}
		if c.Amego.AllowRealInvoice && c.Amego.PlatformInvoice == c.Amego.TestInvoice {
			conflicts = append(conflicts, fmt.Sprintf(
				"AMEGO_ALLOW_REAL_INVOICE=true 卻使用測試統編 %s："+
					"這樣開出的發票不會進入任何公司的帳",
				c.Amego.PlatformInvoice))
		}
	}
	// 反向失效：prod 沒開 ALLOW_REAL 的話一張發票都開不出來，而且沒有任何錯誤訊號
	// ——那正是「靜默」的另一半。與 §8.5.1 (D) 第 ⑨ 條是同一份，改一邊要改兩邊。
	if env := strings.ToLower(strings.TrimSpace(c.Env)); env == "prod" || env == "production" {
		if !c.Amego.AllowRealInvoice {
			conflicts = append(conflicts,
				"ENV="+c.Env+" 但 AMEGO_ALLOW_REAL_INVOICE 不是 true："+
					"這個環境一張發票都開不出來，且不會有任何錯誤訊號")
		}
	}
	// 租約下界：單筆最壞耗時必須小於租約，否則持有者還在打 AMEGO 就被合法搶走（重複開票）。
	if c.Schedule.IssueLeaseSeconds*1000 <= c.Amego.TimeoutIssueMS {
		conflicts = append(conflicts, fmt.Sprintf(
			"INVOICE_ISSUE_LEASE_SECONDS(%ds) 必須大於 AMEGO_HTTP_TIMEOUT_ISSUE_MS(%dms)",
			c.Schedule.IssueLeaseSeconds, c.Amego.TimeoutIssueMS))
	}
	if c.Schedule.RecheckLeaseSeconds*1000 <= c.Amego.TimeoutQueryMS {
		conflicts = append(conflicts, fmt.Sprintf(
			"INVOICE_RECHECK_LEASE_SECONDS(%ds) 必須大於 AMEGO_HTTP_TIMEOUT_QUERY_MS(%dms)",
			c.Schedule.RecheckLeaseSeconds, c.Amego.TimeoutQueryMS))
	}
	// 整條退避序列必須在告警門檻內走完，否則本來會自己成功的列也會先把告警打亮。
	if c.Schedule.BackoffMaxSeconds < c.Schedule.BackoffBaseSeconds {
		conflicts = append(conflicts, "INVOICE_BACKOFF_MAX_SECONDS 不得小於 INVOICE_BACKOFF_BASE_SECONDS")
	}

	var msgs []string
	if len(missing) > 0 {
		msgs = append(msgs, "missing required config: "+strings.Join(missing, ", "))
	}
	if len(conflicts) > 0 {
		msgs = append(msgs, "invalid config: "+strings.Join(conflicts, "; "))
	}
	if len(msgs) > 0 {
		return fmt.Errorf("%s", strings.Join(msgs, " | "))
	}
	return nil
}

func envInt(key string, def int) int {
	raw := strings.TrimSpace(pkgconfig.GetEnvOrDefault(key, ""))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func envFloat(key string, def float64) float64 {
	raw := strings.TrimSpace(pkgconfig.GetEnvOrDefault(key, ""))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return def
	}
	return v
}

func envBool(key string, def bool) bool {
	raw := strings.TrimSpace(pkgconfig.GetEnvOrDefault(key, ""))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return v
}

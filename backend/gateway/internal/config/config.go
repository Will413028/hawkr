package config

import (
	"fmt"
	"os"
	"strings"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
)

// Config holds all configuration for the gateway.
type Config struct {
	Env         string
	Port        string
	DebugPort   string
	MetricsPort string
	// JWTSecret 只用來簽聊天 WebSocket 的 HMAC ticket（middleware.SignChatWSTicket）：認證在單體，gateway 不驗 JWT。
	JWTSecret string
	TLSCert   string
	TLSKey    string
	GinMode   string
	// MediaBaseURL 是媒體資產顯示 URL 的域名唯一來源（49 號設計書 §8.5）。
	// Gateway 用它把微服務回傳的 objectKey 組成對外顯示 URL（pkg/mediaurl.BuildURL）。
	//
	// **無預設值，缺值即 fail-fast**（backend#2283）：媒體域名不做新舊相容——dev 一律
	// `https://test-media.example.com`、prod 一律 `https://media.example.com`。
	// 舊實作預設回退 `https://cdn.example.com`（退役中的舊 OSS 網域），DNS 一撤就會
	// 讓漏設本變數的 pod 靜默吐出死連結；「猜一個域名」無論猜哪個都是錯的，所以直接
	// 讓它啟動失敗。注入來源：dev = k8s/values/*.yaml、prod = overlays/prod/common.yaml、
	// 本機 = docker-compose-local.yml。
	MediaBaseURL string
	// WebBaseURL 是賣家公開頁（買家與賣家網頁）的域名，跟 seller-service 的 WEB_BASE_URL
	// 語意相同、各自獨立設定（非跨服務共享）。目前唯一用途：admin 審核頁算平台
	// 生成頁 fallback URL 顯示用，見 handler.Holder.WebBaseURL 的說明。
	WebBaseURL string

	// ---- Rate limiting（issue #1416）----
	// 分層限流：同一時間窗內、依 client IP 計數，不同敏感度端點套不同上限。經 Redis
	// 跨 pod 共享（避免 per-pod N×放大），Redis 不可用時降級為 bounded 記憶體限流。
	// 各層上限為「每 RateLimitWindowSeconds 秒的請求數」，皆可 env 覆寫。
	RateLimitWindowSeconds     int
	RateLimitModeratePerWindow int // 匿名公開讀取：一個真人（含網頁／admin 代打）
	// 匿名公開讀取的另外兩軸。只有這一層分 kind：網頁 SSR 一個訪客就會扇出上百次
	// 上游讀取，CI 的 E2E 亦然，而它們共用一個出口位址——與真人共用同一個配額值，
	// 不是「誰超量誰被擋」，是「誰先到誰用完」。
	RateLimitModerateSSRPerWindow int // 網頁 server 端渲染（含 ISR / build 預抓）
	RateLimitModerateCIPerWindow  int // CI runner / E2E
	RateLimitDefaultPerWindow     int // 已認證 app + admin
	RateLimitFallbackCapacity     int // 記憶體 fallback 保留的 key 數上限（LRU 淘汰）
	// 同一 IP 每個時間窗容許的 401 次數，超過後該 IP 的所有請求一律 429。與上面
	// 各層是不同軸：壞掉的 client 卡在失敗迴圈時，流量完全在自己那層的配額內，
	// 上面任何一層都不會擋（2026-07-29 dev 實例：4.8 req/s vs 門檻 5.0 req/s）。
	RateLimitAuthFailurePerWindow int

	// ---- 來源簽章 X-HK-Origin-Auth（#2264，01 設計書 §3.2.5）----
	// 自家 server 端呼叫者（網頁 Worker / CI）用它宣告「我代表哪個限流
	// 身分」。刻意不進 Validate()：空值 = 停用該類簽章驗證（請求落回 v:<ClientIP>，
	// 即本設計之前的行為），不是啟動失敗——secret 是人工建立的，把它變成啟動必要
	// 條件等於保證每個環境第一次部署就 CrashLoop。停用狀態由
	// gateway_origin_auth_enabled gauge 的 critical 告警現形。
	OriginAuthSecret     string // 網頁 Worker：只接受 p=visitor / p=ssr
	OriginAuthSecretPrev string // 輪替重疊窗口（同 §5.2 慣例）
	OriginAuthCISecret   string // GitHub Actions：只接受 p=ci。暴露面不同的持有者不共用密鑰

	// TrustedProxyCIDRs 是 Gateway 信任、可代設 X-Forwarded-For 的叢集內反向代理
	// （Traefik）網段。其他來源送的 XFF 一律忽略、ClientIP 退回 socket peer，杜絕
	// origin 直連路徑上的 IP 偽造（Cloudflare 的 CF-Connecting-IP 另由 gin
	// TrustedPlatform 處理，兩者並存）。#1416。
	TrustedProxyCIDRs []string

	// Delegation 是 ADM 代行營運的 per-hop gRPC deadline（56 號設計書 §8.5.1 (B)）。
	Delegation DelegationConfig

	// ProcurementReplaySseEnabled 是 doc 57 §13 的 capability flag
	// `PROCUREMENT_REPLAY_SSE_ENABLED`：true 時開放 GET
	// /buyers/me/procurement/runs/:runId/events（ProxyProcurementRunEvents）；
	// false 時該端點固定回 503 CAPABILITY_UNAVAILABLE。
	// Gateway 沒有既有的通用 feature-flag 框架，這裡沿用 Config 既有的
	// 「一個 bool 欄位 + env 讀取」寫法，不新建一套。預設 false，rollback 就是關掉它。
	ProcurementReplaySseEnabled bool

	// Docs 的環境變數：DOCS_USERNAME、DOCS_PASSWORD、DOCS_SIGN_KEY。
	Docs DocsConfig

	IMChatStreamJWTSecret     string
	IMChatStreamJWTSecretPrev string
}

// DelegationConfig holds the Gateway's per-hop gRPC deadlines for delegated
// operations (56 號設計書 §8.5.1 (B)). Deadlines belong to the caller, so this
// group lives here and not in account-service: account-service has neither the
// standing nor the information to decide how long the Gateway waits before
// giving up. The mirror-imaged DELEGATION_GRPC_DEADLINE_SELLER_STATE_MS is
// account-service's own hop and is deliberately not in this struct — setting
// either one on the wrong service produces no error signal at all.
//
// 預設值取自 §7.3.1 的逾時預算表；值域上界另有各自的線（本 struct 不斷言逐支值域，
// 因為值域逐項成立不保證加總成立——真正的守門人是 Validate() 的兩條總預算斷言）。
type DelegationConfig struct {
	// 單筆路徑（8 支）。EndSession／RevokeSession 沿用 CreateMs，不另立參數。
	GrpcDeadlinePreflightMs    int
	GrpcDeadlineCreateMs       int
	GrpcDeadlineCurrentMs      int
	GrpcDeadlineStoreStateMs   int
	GrpcDeadlineDomainReadMs   int
	GrpcDeadlineAuditIntentMs  int
	GrpcDeadlineAuditConfirmMs int
	GrpcDeadlineDomainWriteMs  int

	// 批次路徑（4 支）。刻意不沿用單筆那組：批次每一跳在服務端的工作量是 N 筆，
	// 沿用單筆 deadline 會讓 N 稍大就必然逾時，而逾時＝結果未知＝稽核記 FAILED。
	GrpcDeadlineBatchDomainReadMs   int
	GrpcDeadlineBatchAuditIntentMs  int
	GrpcDeadlineBatchDomainWriteMs  int
	GrpcDeadlineBatchAuditConfirmMs int

	// AuditConfirmTotalBudgetMs 是單筆 confirm 重試迴圈的總預算（含重試之間的
	// 退避）。account-service 也有一支同名 env（server 端自己的處理上界），兩者
	// 必須同值——confirm 的重試迴圈實際跑在 Gateway 的 orchestrator，總預算的
	// 執行者是這裡；而 Gateway 讀不到別的服務的 env，所以各自宣告一份。
	// **同值沒有任何機械保證**，靠 §12.1 位置 5 的部署後對帳。
	AuditConfirmTotalBudgetMs int
	// AuditConfirmBatchTotalBudgetMs 是批次版，只落在 Gateway。
	AuditConfirmBatchTotalBudgetMs int
}

// DocsConfig 是本機文件檢視器（/docs）的登入憑證與 session 簽章金鑰。
// 三個值都沒有預設；任一為空，檢視器停用、路由不註冊，gateway 照常啟動。
type DocsConfig struct {
	Username string
	Password string
	SignKey  string
}

// TLSEnabled returns true if both TLS cert and key paths are set.
func (c *Config) TLSEnabled() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Env:          pkgconfig.GetEnvOrDefault("APP_ENV", "development"),
		Port:         pkgconfig.GetEnvOrDefault("PORT", "9900"),
		DebugPort:    pkgconfig.GetEnvOrDefault("DEBUG_PORT", "9901"),
		MetricsPort:  pkgconfig.GetEnvOrDefault("METRICS_PORT", "9902"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		TLSCert:      os.Getenv("TLS_CERT_PATH"),
		TLSKey:       os.Getenv("TLS_KEY_PATH"),
		GinMode:      os.Getenv("GIN_MODE"),
		MediaBaseURL: pkgconfig.GetEnvOrDefault("MEDIA_BASE_URL", ""),
		WebBaseURL:   pkgconfig.GetEnvOrDefault("WEB_BASE_URL", "http://localhost:3000"),

		RateLimitWindowSeconds: pkgconfig.GetEnvAsIntOrDefault("RATE_LIMIT_WINDOW_SECONDS", 60),
		// 600 = 實測上界（瀏覽器直連尖峰 25/分 + 網頁 middleware／route handler
		// 與 admin BFF 代打，保守估上界約 200/分）的 3 倍，並對 NAT／CGNAT 後多
		// 真人共用一個位址留餘裕。舊值 120 只放寬不收緊。
		RateLimitModeratePerWindow: pkgconfig.GetEnvAsIntOrDefault("RATE_LIMIT_MODERATE_PER_WINDOW", 600),
		// 12000/60s = 200 req/s。定位是**失控保險絲不是日常節流閥**：目錄長滿且扇出
		// 修復後單訪客 SSR 需求 200–240/分，×10 個並發重度瀏覽訪客 = 2,400/分，
		// 3 倍門檻 7,200 ⇒ 供需差 5.0 倍。40%（4,800/分）發告警。
		RateLimitModerateSSRPerWindow: pkgconfig.GetEnvAsIntOrDefault("RATE_LIMIT_MODERATE_SSR_PER_WINDOW", 12000),
		// 300/60s：7 天實測 CI 單分鐘尖峰 84（該期間零筆被擋，故 84 是真實需求而非
		// 被 120 擋出來的天花板），×3 = 252 ⇒ 取 300。加第二台 website-heavy runner
		// 後推估約 105/分，仍有 2.9 倍——真的加了要重量。
		RateLimitModerateCIPerWindow: pkgconfig.GetEnvAsIntOrDefault("RATE_LIMIT_MODERATE_CI_PER_WINDOW", 300),
		RateLimitDefaultPerWindow:    pkgconfig.GetEnvAsIntOrDefault("RATE_LIMIT_DEFAULT_PER_WINDOW", 300),
		RateLimitFallbackCapacity:    pkgconfig.GetEnvAsIntOrDefault("RATE_LIMIT_FALLBACK_CAPACITY", 10000),
		// 100/分鐘。計數單位是 IP，但一個 IP 背後常是整間辦公室（NAT）、整個共用
		// 出口，或一台 CI runner；門檻要能容納「多人同時 session 過期」的合理尖峰
		// ——單一使用者開 App 撞到過期 session，就可能一次噴出近十個並行 401。
		// 20 太低（原值）：十來個人同分鐘過期就足以連坐整個位址。
		// 100 仍能即時擋下本軸要防的機器迴圈：#106 那種 4.8 req/s 全 401 的 loop
		// 約 21 秒撞頂（20 時約 4 秒），而該事故實際跑了數小時——兩者差距在防護
		// 價值上等價。登入端點在單體，登入的暴力嘗試由單體的限流處理，與此值無關。
		RateLimitAuthFailurePerWindow: pkgconfig.GetEnvAsIntOrDefault("RATE_LIMIT_AUTH_FAILURE_PER_WINDOW", 100),
		// 預設信任 RFC1918 私有網段（叢集內 Traefik 來源）。origin 直連的公網 peer
		// 不在此列，其 XFF 會被忽略。
		TrustedProxyCIDRs: splitCSV(pkgconfig.GetEnvOrDefault("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")),

		OriginAuthSecret:     os.Getenv("ORIGIN_AUTH_SECRET"),
		OriginAuthSecretPrev: os.Getenv("ORIGIN_AUTH_SECRET_PREV"),
		OriginAuthCISecret:   os.Getenv("ORIGIN_AUTH_CI_SECRET"),

		Delegation: DelegationConfig{
			GrpcDeadlinePreflightMs:    pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_PREFLIGHT_MS", 3000),
			GrpcDeadlineCreateMs:       pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_CREATE_MS", 2000),
			GrpcDeadlineCurrentMs:      pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_CURRENT_MS", 800),
			GrpcDeadlineStoreStateMs:   pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_STORE_STATE_MS", 800),
			GrpcDeadlineDomainReadMs:   pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_DOMAIN_READ_MS", 800),
			GrpcDeadlineAuditIntentMs:  pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_AUDIT_INTENT_MS", 800),
			GrpcDeadlineAuditConfirmMs: pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_AUDIT_CONFIRM_MS", 400),
			GrpcDeadlineDomainWriteMs:  pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_DOMAIN_WRITE_MS", 4000),

			GrpcDeadlineBatchDomainReadMs:   pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_BATCH_DOMAIN_READ_MS", 3000),
			GrpcDeadlineBatchAuditIntentMs:  pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_BATCH_AUDIT_INTENT_MS", 4000),
			GrpcDeadlineBatchDomainWriteMs:  pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_BATCH_DOMAIN_WRITE_MS", 20000),
			GrpcDeadlineBatchAuditConfirmMs: pkgconfig.GetEnvAsIntOrDefault("DELEGATION_GRPC_DEADLINE_BATCH_AUDIT_CONFIRM_MS", 3000),

			AuditConfirmTotalBudgetMs:      pkgconfig.GetEnvAsIntOrDefault("DELEGATION_AUDIT_CONFIRM_TOTAL_BUDGET_MS", 2000),
			AuditConfirmBatchTotalBudgetMs: pkgconfig.GetEnvAsIntOrDefault("DELEGATION_AUDIT_CONFIRM_BATCH_TOTAL_BUDGET_MS", 10000),
		},

		ProcurementReplaySseEnabled: pkgconfig.GetEnvAsBoolOrDefault("PROCUREMENT_REPLAY_SSE_ENABLED", false),
		Docs: DocsConfig{
			Username: os.Getenv("DOCS_USERNAME"),
			Password: os.Getenv("DOCS_PASSWORD"),
			SignKey:  os.Getenv("DOCS_SIGN_KEY"),
		},
		IMChatStreamJWTSecret:     os.Getenv("IM_CHAT_STREAM_JWT_SECRET"),
		IMChatStreamJWTSecretPrev: os.Getenv("IM_CHAT_STREAM_JWT_SECRET_PREV"),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// splitCSV parses a comma-separated env value into a trimmed, non-empty slice.
// An empty string yields nil (no trusted proxies configured).
func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Validate checks required configuration fields.
func (c *Config) Validate() error {
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is not set")
	}
	// MEDIA_BASE_URL 無預設域名，缺值即 fail-fast（backend#2283）：dev 一律 test-media、
	// prod 一律 media，不做新舊相容也不猜域名——漏設就啟動失敗，不允許靜默吐出死連結。
	if c.MediaBaseURL == "" {
		return fmt.Errorf("missing required config: MEDIA_BASE_URL")
	}
	return c.validateDelegation()
}

// 代行寫入的 handler 總預算（56 號設計書 §7.3.1）。外層逾時是前端 AbortController
// 的值，其中還要扣掉 BFF、網路與 CF 的開銷，所以這裡用的是 handler 預算而不是外層值
// ——拿外層值當上界＝把餘裕當預算用，而逾時發生在前端時 Gateway 收不到取消訊號，
// 稽核列會停在一個沒有人去 confirm 的狀態。
const (
	delegationSingleWriteBudgetMs = 12000
	delegationBatchWriteBudgetMs  = 50000
)

// validateDelegation asserts the two budget inequalities of §8.5.1 (B).
// Per-variable ranges hold individually but do not add up: every single-item
// deadline at its own upper bound sums to 27s, far past this path's outer
// timeout. Both assertions fail-fast, never log a warning.
//
// confirm 那一項直接取 _TOTAL_BUDGET_MS，不寫成 min(單次 × 次數)：後者漏算重試
// 之間的退避，MAX_ATTEMPTS 調到 5 時退避總和達 1500 ms，會系統性低估。
// STORE_STATE 在兩式各只出現一次——它是 per-request 的判定跳，批次的 target store
// 恰好一個，與 items 筆數無關。
func (c *Config) validateDelegation() error {
	d := &c.Delegation

	single := d.GrpcDeadlineCurrentMs + d.GrpcDeadlineStoreStateMs + d.GrpcDeadlineDomainReadMs +
		d.GrpcDeadlineAuditIntentMs + d.GrpcDeadlineDomainWriteMs + d.AuditConfirmTotalBudgetMs
	if single > delegationSingleWriteBudgetMs {
		return fmt.Errorf("single-item delegated write budget is %dms, must be <= %dms (gateway handler budget, §7.3.1)",
			single, delegationSingleWriteBudgetMs)
	}

	batch := d.GrpcDeadlineCurrentMs + d.GrpcDeadlineStoreStateMs + d.GrpcDeadlineBatchDomainReadMs +
		d.GrpcDeadlineBatchAuditIntentMs + d.GrpcDeadlineBatchDomainWriteMs + d.AuditConfirmBatchTotalBudgetMs
	if batch > delegationBatchWriteBudgetMs {
		return fmt.Errorf("batch delegated write budget is %dms, must be <= %dms (gateway handler budget, §7.3.1)",
			batch, delegationBatchWriteBudgetMs)
	}

	return nil
}

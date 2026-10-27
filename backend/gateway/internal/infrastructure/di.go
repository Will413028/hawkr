package infrastructure

import (
	"context"
	"net/http"
	_ "net/http/pprof" // registers pprof handlers on http.DefaultServeMux
	"runtime"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"github.com/Will413028/hawkr/gateway/internal/cache"
	"github.com/Will413028/hawkr/gateway/internal/config"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/router"
	ginmw "github.com/Will413028/hawkr/go-pkg/middleware/gin"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("infrastructure",
	fx.Invoke(initTracer),
	fx.Invoke(initRedis),
	fx.Invoke(startPprof),
	fx.Invoke(startMetrics),
	fx.Invoke(startHTTPServer),
)

func initTracer(lc fx.Lifecycle, l *zap.Logger) {
	shutdown := middleware.InitTracer(l)
	if shutdown != nil {
		l.Info("Tracing initialized")
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				l.Info("Shutting down tracer provider")
				return shutdown(ctx)
			},
		})
	}
}

func initRedis(lc fx.Lifecycle) {
	cache.InitRedis()
	if cache.RedisClient != nil {
		middleware.SetRedisClient(cache.RedisClient)
	}
	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			if cache.RedisClient != nil {
				return cache.RedisClient.Close()
			}
			return nil
		},
	})
}

func startPprof(cfg *config.Config, l *zap.Logger) {
	if pkgserver.IsProductionEnv(cfg.Env) {
		return
	}
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)
	go func() {
		l.Info("pprof listening", zap.String("addr", "localhost:"+cfg.DebugPort+"/debug/pprof/"))
		if err := http.ListenAndServe("localhost:"+cfg.DebugPort, nil); err != nil {
			l.Info("pprof server error", zap.Error(err))
		}
	}()
}

// startMetrics 把 /metrics 掛在獨立 port，不經公開 Gateway Ingress（B-1420：避免與
// 公開 API 共用同一個 Gin engine/port 導致匿名可存取）。走 pod IP 直接被 in-cluster
// Prometheus Agent scrape（kubernetes_sd role: pod），不需要透過 Service/NodePort，
// 因此得綁 0.0.0.0（跟 startPprof 的 localhost-only 不同）。
func startMetrics(lc fx.Lifecycle, cfg *config.Config, l *zap.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // slowloris protection on headers, matches the main gateway server
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			l.Info("Metrics listening", zap.String("port", cfg.MetricsPort))
			go func() {
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					l.Error("Metrics server error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}

func newEngine(cfg *config.Config, l *zap.Logger, h *handler.Holder) *gin.Engine {
	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Behind Cloudflare: resolve the real client IP from CF-Connecting-IP
	// instead of a client-controllable X-Forwarded-For. Without this,
	// c.ClientIP() (consumed by the access log and OTel; quota keys go through
	// middleware.RateLimitSubject) trusts the left-most XFF.
	r.TrustedPlatform = gin.PlatformCloudflare
	// Explicit trusted-proxy allowlist (#1416): gin's default trusts XFF from
	// ANY peer (trustedProxies = 0.0.0.0/0).
	//
	// ⚠️ Do not read this as "XFF spoofing is therefore impossible off the CF
	// path" — an earlier version of this comment claimed exactly that and it is
	// wrong here. klipper-lb SNATs every request (POSTROUTING -j MASQUERADE), so
	// the peer this allowlist tests is ALWAYS 10.42.0.1, which is inside
	// 10.0.0.0/8 — the check passes for the origin-direct attacker and the
	// Cloudflare edge alike, because at that layer they are indistinguishable
	// (24h of traefik access logs: zero exceptions, dev and prod). What actually
	// keeps a forged XFF out today is traefik overwriting it. Closing the
	// origin-direct path is a network-layer job (loadBalancerSourceRanges),
	// tracked separately from this PR; until it lands, the guarantee that holds
	// is the narrower one in RateLimitSubject: an unsigned request can only ever
	// land in the "v:" namespace, never on the SSR or CI axis.
	if err := r.SetTrustedProxies(cfg.TrustedProxyCIDRs); err != nil {
		l.Warn("invalid TRUSTED_PROXY_CIDRS, disabling XFF trust", zap.Error(err))
		_ = r.SetTrustedProxies(nil)
	}
	// Origin signature keys, before any middleware can be asked to resolve a
	// subject. Empty values disable that key's verification (see
	// middleware.SetOriginAuthSecrets) — never a boot failure.
	middleware.SetOriginAuthSecrets(cfg.OriginAuthSecret, cfg.OriginAuthSecretPrev, cfg.OriginAuthCISecret, l)
	r.Use(gin.Recovery())
	r.Use(middleware.TracingMiddleware())
	// 業務碼 carrier（bizcarrier）：讓「上游回了一個 preserveData 業務碼」跨越
	// handler 的函式邊界活到回應寫出的那一刻，於是 code=0 的假成功寫不出來。
	//
	// 掛全域鏈而不是掛在 invoice／checkout 群組上：Arm 拿不到 carrier 時
	// EnvelopeGuardInterceptor 會撤銷特權（改走一般錯誤映射），群組式掛載會讓群組外
	// 的路由行為從「return nil」變成硬錯誤 —— 那是新的失效模式，不是修復。全域掛載
	// 之後 fail-closed 分支只在真正異常（handler 傳了 detached context）時才走到。
	//
	// 位置在 TracingMiddleware 之後、任何會寫回應的 gate 之前：限流／BodyGuard
	// 的 4xx 都經由 internal/response 寫出，會正確留下 emitted 痕跡。
	r.Use(bizcarrier.Middleware())
	r.Use(ginmw.TraceLogger(l))
	r.Use(middleware.HTTPMetrics())
	// Tiered per-IP rate limiter (#1416): Redis-shared across pods with a bounded
	// in-memory fallback; classifies each route into a sensitivity tier (exempt /
	// moderate / default).
	//
	// CORS 與 CSRF 在單體（對外的入口）：瀏覽器的預檢由單體回應，不會到這裡；這裡的 429 經單體轉回時由單體的
	// CORS 補上 header。
	r.Use(middleware.NewRateLimiter(middleware.RateLimitPolicy{
		ModeratePerWindow:    cfg.RateLimitModeratePerWindow,
		ModerateSSRPerWindow: cfg.RateLimitModerateSSRPerWindow,
		ModerateCIPerWindow:  cfg.RateLimitModerateCIPerWindow,
		DefaultPerWindow:     cfg.RateLimitDefaultPerWindow,
		AuthFailurePerWindow: cfg.RateLimitAuthFailurePerWindow,
		Window:               time.Duration(cfg.RateLimitWindowSeconds) * time.Second,
		FallbackCapacity:     cfg.RateLimitFallbackCapacity,
	}, l))
	r.Use(middleware.BodyGuard()) // NEW — before Logger ever touches the body
	r.Use(middleware.Logger())    // now runs after every cheap gate

	router.SetupRoutes(r, h)
	return r
}

func startHTTPServer(lc fx.Lifecycle, cfg *config.Config, l *zap.Logger, h *handler.Holder) {
	r := newEngine(cfg, l, h)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,  // slowloris protection on headers
		IdleTimeout:       120 * time.Second, // keep-alive idle reclaim
		// No ReadTimeout/WriteTimeout: both would bound the entire connection
		// lifetime including in-flight SSE/AI-stream responses. Body-read
		// bounding is per-request via BodyGuard; streaming write duration is
		// bounded per-write in sse.ServeSSE and the AI stream proxy's
		// deadlineResponseWriter (proxy.ServeAIAssistantStream).
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			if cfg.TLSEnabled() {
				l.Info("Gateway starting with TLS", zap.String("port", cfg.Port))
			} else {
				l.Info("Gateway starting", zap.String("port", cfg.Port))
			}
			go func() {
				var err error
				if cfg.TLSEnabled() {
					err = server.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
				} else {
					err = server.ListenAndServe()
				}
				if err != nil && err != http.ErrServerClosed {
					l.Fatal("Failed to start server", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info("Shutting down gateway")
			return server.Shutdown(ctx)
		},
	})
}

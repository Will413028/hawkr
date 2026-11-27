package infrastructure

import (
	"context"
	"net/http"
	_ "net/http/pprof"
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

func startMetrics(lc fx.Lifecycle, cfg *config.Config, l *zap.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
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
	r.TrustedPlatform = gin.PlatformCloudflare
	if err := r.SetTrustedProxies(cfg.TrustedProxyCIDRs); err != nil {
		l.Warn("invalid TRUSTED_PROXY_CIDRS, disabling XFF trust", zap.Error(err))
		_ = r.SetTrustedProxies(nil)
	}
	middleware.SetOriginAuthSecrets(cfg.OriginAuthSecret, cfg.OriginAuthSecretPrev, cfg.OriginAuthCISecret, l)
	r.Use(gin.Recovery())
	r.Use(middleware.TracingMiddleware())
	r.Use(bizcarrier.Middleware())
	r.Use(ginmw.TraceLogger(l))
	r.Use(middleware.HTTPMetrics())
	r.Use(middleware.NewRateLimiter(middleware.RateLimitPolicy{
		ModeratePerWindow:    cfg.RateLimitModeratePerWindow,
		ModerateSSRPerWindow: cfg.RateLimitModerateSSRPerWindow,
		ModerateCIPerWindow:  cfg.RateLimitModerateCIPerWindow,
		DefaultPerWindow:     cfg.RateLimitDefaultPerWindow,
		AuthFailurePerWindow: cfg.RateLimitAuthFailurePerWindow,
		Window:               time.Duration(cfg.RateLimitWindowSeconds) * time.Second,
		FallbackCapacity:     cfg.RateLimitFallbackCapacity,
	}, l))
	r.Use(middleware.BodyGuard())
	r.Use(middleware.Logger())

	router.SetupRoutes(r, h)
	return r
}

func startHTTPServer(lc fx.Lifecycle, cfg *config.Config, l *zap.Logger, h *handler.Holder) {
	r := newEngine(cfg, l, h)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
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

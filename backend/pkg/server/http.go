package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	httppprof "net/http/pprof"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// HTTPServerParams contains the fx dependencies for creating the internal
// HTTP server (health/metrics/pprof — never exposed via the API gateway).
type HTTPServerParams struct {
	fx.In
	Lifecycle     fx.Lifecycle
	Config        ServerConfig
	ServiceName   string `name:"service_name"`
	Logger        *zap.Logger
	MeterProvider *sdkmetric.MeterProvider `optional:"true"`
}

// NewInternalHTTPServer creates the internal HTTP server for health checks,
// Prometheus metrics, and (outside production) pprof debug endpoints, and
// registers an fx.Hook that starts it on OnStart and shuts it down
// gracefully on OnStop.
func NewInternalHTTPServer(p HTTPServerParams) *http.Server {
	router := newRouter(p.Config, p.ServiceName, p.Logger)
	lifecycleMetrics := newLifecycleMetrics(p.MeterProvider, p.Logger)
	var started atomic.Bool

	srv := &http.Server{
		Addr:    ":" + p.Config.HTTPPort,
		Handler: router,
	}

	var listener net.Listener
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				return err
			}
			listener = lis
			started.Store(true)
			lifecycleMetrics.recordStart(ctx, "http")
			p.Logger.Info("Starting HTTP server", zap.String("port", p.Config.HTTPPort))
			go func() {
				if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					p.Logger.Error("HTTP server error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			p.Logger.Info("Stopping HTTP server")
			err := srv.Shutdown(ctx)
			if err == nil && started.Swap(false) {
				lifecycleMetrics.recordShutdown(ctx, "http")
			}
			return err
		},
	})

	return srv
}

// newRouter builds the gin.Engine backing NewInternalHTTPServer. Split out
// so tests can inspect the registered route surface without binding a port.
func newRouter(cfg ServerConfig, serviceName string, logger *zap.Logger) *gin.Engine {
	if IsProductionEnv(cfg.Env) {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": serviceName})
	})
	router.HEAD("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	if !IsProductionEnv(cfg.Env) {
		runtime.SetMutexProfileFraction(1)
		runtime.SetBlockProfileRate(1)
		dbg := router.Group("/debug/pprof")
		dbg.GET("/", gin.WrapF(httppprof.Index))
		dbg.GET("/cmdline", gin.WrapF(httppprof.Cmdline))
		dbg.GET("/profile", gin.WrapF(httppprof.Profile))
		dbg.GET("/symbol", gin.WrapF(httppprof.Symbol))
		dbg.POST("/symbol", gin.WrapF(httppprof.Symbol))
		dbg.GET("/trace", gin.WrapF(httppprof.Trace))
		dbg.GET("/allocs", gin.WrapH(httppprof.Handler("allocs")))
		dbg.GET("/block", gin.WrapH(httppprof.Handler("block")))
		dbg.GET("/goroutine", gin.WrapH(httppprof.Handler("goroutine")))
		dbg.GET("/heap", gin.WrapH(httppprof.Handler("heap")))
		dbg.GET("/mutex", gin.WrapH(httppprof.Handler("mutex")))
		dbg.GET("/threadcreate", gin.WrapH(httppprof.Handler("threadcreate")))
		logger.Info("pprof enabled", zap.String("path", "/debug/pprof/"))
	}

	return router
}

// IsProductionEnv reports whether env names a production deployment. Deployed
// ConfigMaps set ENV=prod while older code compared against the literal
// "production", so both spellings must count. That mismatch is not hypothetical:
// it left the pprof gate in newRouter OPEN on production (issue #1418, hotfixed
// straight onto main). Exported so callers outside this package (gateway's
// startPprof) share one definition instead of re-deriving it and drifting again.
func IsProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

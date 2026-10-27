// Package server provides the shared gRPC and internal-HTTP bootstrap used
// by this repo's Go services: interceptor chain construction, OTel stats
// handler, reflection registration, and fx.Lifecycle wiring for gRPC; and
// health/metrics/pprof/recovery wiring for the internal HTTP port.
package server

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"

	"github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// ServerConfig holds the port and per-service toggles shared by
// NewGRPCServer and NewInternalHTTPServer. Each service provides its own
// ServerConfig value via fx.Provide, translating its local *config.Config
// into this package-agnostic shape.
//
// It embeds config.ServerConfig for the port fields (GRPCPort, HTTPPort) and
// adds the fx-bootstrap-specific fields below on top.
type ServerConfig struct {
	config.ServerConfig
	// HealthServiceNames registers full protobuf service names with the
	// standard gRPC health server. The empty service name is always registered
	// for aggregate process health; this slice adds service-specific statuses.
	HealthServiceNames []string
	// Env controls Gin release mode and the pprof debug group in
	// NewInternalHTTPServer. Both "prod" and "production" disable pprof;
	// APP_ENV is intentionally not consulted so one process cannot have
	// conflicting service-level environment sources.
	Env string
	// EnableStreamInterceptors registers grpc.ChainStreamInterceptor. Leave
	// false (the zero value) for gRPC services with no streaming RPCs — e.g.
	// pricebook-service's GetObservedPrice is unary-only.
	EnableStreamInterceptors bool
	// LoggingOptions customizes interceptor.LoggingInterceptor per service —
	// e.g. account-service's WithExpectedCodes(accountgrpc.ExpectedStatusCodes)
	// keeps routine per-method negative outcomes (GetWebSession → NotFound for
	// a dead cookie) out of error-level logs (issue #2072). Empty by default:
	// zero behavior change for services that don't need this.
	LoggingOptions []interceptor.LoggingOption
	// MaxRecvMsgSize / MaxSendMsgSize override grpc's default max message size
	// when non-zero (0 = use grpc's default, no option appended). Needed by
	// services with large request/response payloads — e.g. ai-service's
	// procurement candidate sets.
	MaxRecvMsgSize int
	MaxSendMsgSize int
	// ExtraUnaryInterceptors / ExtraStreamInterceptors run after the standard
	// chain (after ErrorInterceptor), for service-specific policy/authorization
	// interceptors that don't belong in the shared chain — e.g. ai-service's
	// UnaryMethodPolicyInterceptor/StreamMethodPolicyInterceptor.
	ExtraUnaryInterceptors  []grpc.UnaryServerInterceptor
	ExtraStreamInterceptors []grpc.StreamServerInterceptor
}

// GRPCServerParams contains the fx dependencies for creating the gRPC server.
type GRPCServerParams struct {
	fx.In
	Lifecycle     fx.Lifecycle
	Config        ServerConfig
	Logger        *zap.Logger
	Health        *HealthController        `optional:"true"`
	MeterProvider *sdkmetric.MeterProvider `optional:"true"`
	Shutdowner    fx.Shutdowner            `optional:"true"`
	Serve         func(net.Listener) error `optional:"true"`
}

// BuildServerOptions builds the interceptor chain and stats handler shared by
// every gRPC server in this repo. Exported (not just used internally by
// NewGRPCServer) for services whose NewGRPCServer can't take an fx.Lifecycle —
// e.g. ai-service's bufconn-based interceptor regression test constructs the
// server without binding a real listener, so it calls BuildServerOptions
// directly and does its own net.Listen/server.Serve wiring.
func BuildServerOptions(cfg ServerConfig, logger *zap.Logger) []grpc.ServerOption {
	var opts []grpc.ServerOption
	if cfg.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize))
	}
	if cfg.MaxSendMsgSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(cfg.MaxSendMsgSize))
	}

	unary := append([]grpc.UnaryServerInterceptor{
		interceptor.CorrelationInterceptor(),
		interceptor.LoggingInterceptor(logger, cfg.LoggingOptions...),
		interceptor.InternalAuthServerInterceptor(logger),
		interceptor.RecoveryInterceptor(logger),
		interceptor.ErrorInterceptor(),
	}, cfg.ExtraUnaryInterceptors...)

	opts = append(opts,
		grpc.StatsHandler(interceptor.OTelServerStatsHandler()),
		grpc.ChainUnaryInterceptor(unary...),
		// EnforcementPolicy.MinTime (10s) MUST stay <= the client's
		// keepalive.ClientParameters.Time (pkg/grpcclient/dial.go Dial,
		// currently 20s) or the server treats every legitimate client
		// keepalive ping as abuse and sends GOAWAY("too_many_pings"),
		// surfacing as random UNAVAILABLE errors on otherwise-healthy
		// connections. grpc-go's own default MinTime is 5 minutes, so
		// leaving this unset while only setting the client's Time would
		// silently reproduce that failure mode. If you change either value,
		// check the other file first.
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: false,
		}),
		// ServerParameters mirrors the client's Time/Timeout so either side
		// can detect a dead peer on an open stream without waiting on an
		// application-layer guess (see pkg/grpcclient/dial.go's Dial for the
		// client half of this pair).
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    20 * time.Second,
			Timeout: 10 * time.Second,
		}),
	)

	if cfg.EnableStreamInterceptors {
		stream := append([]grpc.StreamServerInterceptor{
			interceptor.StreamCorrelationInterceptor(),
			interceptor.StreamLoggingInterceptor(logger),
			interceptor.StreamRecoveryInterceptor(logger),
			interceptor.StreamErrorInterceptor(),
		}, cfg.ExtraStreamInterceptors...)
		opts = append(opts, grpc.ChainStreamInterceptor(stream...))
	}
	return opts
}

// NewGRPCServer creates and configures the gRPC server: interceptor chain,
// OTel stats handler, reflection, standard gRPC health, and an fx.Hook that
// binds cfg.GRPCPort on OnStart and drains it on OnStop.
func NewGRPCServer(p GRPCServerParams) *grpc.Server {
	server := grpc.NewServer(BuildServerOptions(p.Config, p.Logger)...)
	lifecycleMetrics := newLifecycleMetrics(p.MeterProvider, p.Logger)
	var started atomic.Bool
	healthController := p.Health
	if healthController == nil {
		healthController = newLegacyHealthController(p.Config.HealthServiceNames)
	}
	healthController.Register(server)

	reflection.Register(server)
	serve := p.Serve
	if serve == nil {
		serve = server.Serve
	}
	serveDone := make(chan struct{})

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", ":"+p.Config.GRPCPort)
			if err != nil {
				return err
			}
			started.Store(true)
			healthController.MarkBound()
			lifecycleMetrics.recordStart(ctx, "grpc")

			p.Logger.Info("Starting gRPC server",
				zap.String("port", p.Config.GRPCPort))

			go func() {
				defer close(serveDone)
				if err := serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
					p.Logger.Error("gRPC server error", zap.Error(err))
					if p.Shutdowner != nil {
						if shutdownErr := p.Shutdowner.Shutdown(); shutdownErr != nil {
							p.Logger.Error("failed to request FX shutdown after gRPC server error", zap.Error(shutdownErr))
						}
					}
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			p.Logger.Info("Stopping gRPC server")
			// Stop advertising readiness before draining. Long-lived streams are
			// allowed to finish while the fx stop context still has time left, but
			// a wedged peer must not hold process shutdown forever.
			healthController.MarkNotReady()
			gracefulDone := make(chan struct{})
			go func() {
				server.GracefulStop()
				close(gracefulDone)
			}()
			select {
			case <-gracefulDone:
			case <-ctx.Done():
				server.Stop()
			}
			select {
			case <-serveDone:
			case <-ctx.Done():
				return ctx.Err()
			}
			if started.Swap(false) {
				lifecycleMetrics.recordShutdown(ctx, "grpc")
			}
			return nil
		},
	})

	return server
}

func setHealthStatus(healthServer *health.Server, serviceNames []string, status healthpb.HealthCheckResponse_ServingStatus) {
	healthServer.SetServingStatus("", status)
	for _, serviceName := range serviceNames {
		if serviceName == "" {
			continue
		}
		healthServer.SetServingStatus(serviceName, status)
	}
}

package grpc

import (
	"github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"github.com/Will413028/hawkr/go-pkg/serviceauth"
	"google.golang.org/grpc"
)

// getEnvOrDefault delegates to the shared config package.
func getEnvOrDefault(key, defaultVal string) string {
	return config.GetEnvOrDefault(key, defaultVal)
}

// DefaultDialOpts returns the standard gRPC client dial options used by all Gateway clients.
// Includes: OTel tracing and two interceptor chains —
//   - unary: auth metadata forwarding, envelope guard, circuit breaker
//   - stream: auth metadata forwarding, circuit breaker (no envelope guard —
//     a server-stream has no single response envelope to inspect)
//
// Transport credentials are supplied by grpcclient.Dial (every call site dials via
// grpcclient.Dial(target, DefaultDialOpts()...), which prepends insecure creds — same
// final option order as before this migration: insecure creds, then stats handler,
// then chain interceptors).
//
// Interceptor chain order matters: CircuitBreaker is INNERMOST (last) so it observes raw
// transport errors only. EnvelopeGuard runs in an outer layer, translating business errors
// (resp.Code != 0) into gRPC errors — keeping those out of the breaker's failure counts.
func DefaultDialOpts() []grpc.DialOption {
	return dialOpts(nil)
}

// IMDialOpts returns the standard Gateway client options plus the dedicated
// service credential for IM's long-lived ChatStream. The method filter inside
// the interceptor keeps the credential off every other IM RPC.
func IMDialOpts(signer *serviceauth.Signer) []grpc.DialOption {
	if signer == nil {
		return DefaultDialOpts()
	}
	return dialOpts(signer.StreamClientInterceptor(serviceauth.ChatStreamMethod))
}

func dialOpts(streamAuth grpc.StreamClientInterceptor) []grpc.DialOption {
	streamInterceptors := []grpc.StreamClientInterceptor{AuthMetadataStreamInterceptor()}
	if streamAuth != nil {
		streamInterceptors = append(streamInterceptors, streamAuth)
	}
	streamInterceptors = append(streamInterceptors, CircuitBreakerStreamInterceptor())
	return []grpc.DialOption{
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
		grpc.WithChainUnaryInterceptor(
			AuthMetadataInterceptor(),
			EnvelopeGuardInterceptor(),
			CircuitBreakerInterceptor(),
		),
		grpc.WithChainStreamInterceptor(streamInterceptors...),
	}
}

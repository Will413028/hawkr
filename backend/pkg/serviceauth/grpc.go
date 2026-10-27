package serviceauth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// StreamClientInterceptor attaches a service JWT only to targetMethod. User
// identity metadata remains the responsibility of the Gateway's normal
// identity interceptor and is deliberately not encoded into this credential.
func (s *Signer) StreamClientInterceptor(targetMethod string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if method != targetMethod {
			return streamer(ctx, desc, cc, method, opts...)
		}
		raw, err := s.Sign()
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "service credentials unavailable")
		}
		md, _ := metadata.FromOutgoingContext(ctx)
		md = md.Copy()
		md.Set("authorization", "Bearer "+raw)
		return streamer(metadata.NewOutgoingContext(ctx, md), desc, cc, method, opts...)
	}
}

// StreamServerInterceptor verifies a service JWT only for targetMethod. The
// interceptor is method-filtered so shared health and existing unary RPCs do
// not accidentally inherit the stream-only credential policy.
func (v *Verifier) StreamServerInterceptor(targetMethod string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if info.FullMethod != targetMethod {
			return handler(srv, ss)
		}
		raw, ok := bearerToken(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "unauthenticated")
		}
		if _, err := v.Verify(raw); err != nil {
			return status.Error(codes.Unauthenticated, "unauthenticated")
		}
		return handler(srv, ss)
	}
}

func bearerToken(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	values := md.Get("authorization")
	if len(values) != 1 {
		return "", false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

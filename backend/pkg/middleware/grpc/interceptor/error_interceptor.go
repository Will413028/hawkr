package interceptor

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// domainCodeToGRPC maps domain error codes to gRPC status codes.
var domainCodeToGRPC = map[errors.ErrorCode]codes.Code{
	errors.CodeNotFound:           codes.NotFound,
	errors.CodeConflict:           codes.AlreadyExists,
	errors.CodeInvalidInput:       codes.InvalidArgument,
	errors.CodeInvalidTransition:  codes.FailedPrecondition,
	errors.CodePreconditionFailed: codes.FailedPrecondition,
	errors.CodeUnauthorized:       codes.Unauthenticated,
	errors.CodeForbidden:          codes.PermissionDenied,
	errors.CodeUnavailable:        codes.Unavailable,
	errors.CodeUnknown:            codes.Internal,
}

// ErrorInterceptor returns a gRPC unary server interceptor that maps
// DomainError to gRPC status codes.
func ErrorInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		// Check if it implements DomainCoder (covers DomainError and domain entity errors)
		var dc errors.DomainCoder
		if stderrors.As(err, &dc) {
			grpcCode, found := domainCodeToGRPC[errors.ErrorCode(dc.DomainCode())]
			if !found {
				grpcCode = codes.Internal
			}
			return nil, status.Error(grpcCode, dc.DomainMessage())
		}

		// Check if it's already a gRPC status error
		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		// Caller disconnected mid-call (tab closed, app backgrounded,
		// navigated away): ctx is the incoming request context, so ctx.Err()
		// being non-nil confirms this is client cancellation, not an
		// unrelated downstream error that happens to look like one. Same
		// judgment as middleware/auth.go resolveIdentity and gateway
		// response.go (issue #1624) — map to codes.Canceled instead of
		// Internal so this isn't logged/alerted as a server error (#1627).
		if ctx.Err() != nil {
			return nil, status.Error(codes.Canceled, "request canceled by client")
		}

		// Wrap unknown errors as internal errors
		// In production: hide error details for security
		// In non-production: show detailed error for debugging
		if config.IsProduction() {
			return nil, status.Error(codes.Internal, "internal error")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}

// StreamErrorInterceptor returns a gRPC stream server interceptor that maps
// DomainError to gRPC status codes.
func StreamErrorInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := handler(srv, ss)
		if err == nil {
			return nil
		}

		// Check if it implements DomainCoder (covers DomainError and domain entity errors)
		var dc errors.DomainCoder
		if stderrors.As(err, &dc) {
			grpcCode, found := domainCodeToGRPC[errors.ErrorCode(dc.DomainCode())]
			if !found {
				grpcCode = codes.Internal
			}
			return status.Error(grpcCode, dc.DomainMessage())
		}

		// Check if it's already a gRPC status error
		if _, ok := status.FromError(err); ok {
			return err
		}

		// See ErrorInterceptor above for rationale (issue #1627).
		if ss.Context().Err() != nil {
			return status.Error(codes.Canceled, "request canceled by client")
		}

		// Wrap unknown errors as internal errors
		// In production: hide error details for security
		// In non-production: show detailed error for debugging
		if config.IsProduction() {
			return status.Error(codes.Internal, "internal error")
		}
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}

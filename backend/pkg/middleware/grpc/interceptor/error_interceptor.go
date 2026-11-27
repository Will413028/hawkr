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

		var dc errors.DomainCoder
		if stderrors.As(err, &dc) {
			grpcCode, found := domainCodeToGRPC[errors.ErrorCode(dc.DomainCode())]
			if !found {
				grpcCode = codes.Internal
			}
			return nil, status.Error(grpcCode, dc.DomainMessage())
		}

		if _, ok := status.FromError(err); ok {
			return nil, err
		}

		if ctx.Err() != nil {
			return nil, status.Error(codes.Canceled, "request canceled by client")
		}

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

		var dc errors.DomainCoder
		if stderrors.As(err, &dc) {
			grpcCode, found := domainCodeToGRPC[errors.ErrorCode(dc.DomainCode())]
			if !found {
				grpcCode = codes.Internal
			}
			return status.Error(grpcCode, dc.DomainMessage())
		}

		if _, ok := status.FromError(err); ok {
			return err
		}

		if ss.Context().Err() != nil {
			return status.Error(codes.Canceled, "request canceled by client")
		}

		if config.IsProduction() {
			return status.Error(codes.Internal, "internal error")
		}
		return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
	}
}

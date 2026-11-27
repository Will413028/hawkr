package grpc

import (
	"context"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var errNotInternalCall = status.Error(codes.PermissionDenied, "internal-only rpc")

func requireInternalCall(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errNotInternalCall
	}
	for _, v := range md.Get(interceptor.HeaderInternalService) {
		if v != "" {
			return nil
		}
	}
	return errNotInternalCall
}

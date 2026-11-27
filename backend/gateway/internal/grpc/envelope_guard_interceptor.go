package grpc

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type envelopeMapping struct {
	grpcCode codes.Code
	preserveBizCode bool
}

var envelopeCodeToGRPC = map[int32]envelopeMapping{
	400: {grpcCode: codes.InvalidArgument},
	401: {grpcCode: codes.Unauthenticated},
	403: {grpcCode: codes.PermissionDenied},
	404: {grpcCode: codes.NotFound},
	409: {grpcCode: codes.AlreadyExists},
	412: {grpcCode: codes.FailedPrecondition},
	500: {grpcCode: codes.Internal},
	503: {grpcCode: codes.Unavailable},
	3021: {grpcCode: codes.NotFound, preserveBizCode: true},
	3022: {grpcCode: codes.NotFound, preserveBizCode: true},
	3023: {grpcCode: codes.NotFound, preserveBizCode: true},
	4291: {grpcCode: codes.ResourceExhausted, preserveBizCode: true},
	9400: {grpcCode: codes.InvalidArgument, preserveBizCode: true},
	9301: {grpcCode: codes.NotFound, preserveBizCode: true},
	9302: {grpcCode: codes.NotFound, preserveBizCode: true},
	9309: {grpcCode: codes.NotFound, preserveBizCode: true},
	9320: {grpcCode: codes.NotFound, preserveBizCode: true},
	9330: {grpcCode: codes.NotFound, preserveBizCode: true},
	9340: {grpcCode: codes.NotFound, preserveBizCode: true},
	9352: {grpcCode: codes.ResourceExhausted, preserveBizCode: true},
}

var envelopeCodePreserveData = map[int32]bool{
	4001: true,
	4091: true,
	4092: true,
	9310: true,
	9324: true,
	9326: true,
	9331: true,
	9342: true,
	9344: true,
	9347: true,
}

// EnvelopeGuardInterceptor 是 gRPC client interceptor，
// 自動將 envelope error（gRPC OK + resp.Code != 0）轉換為 gRPC error。
//
// 這樣 Gateway handler 只需檢查 err != nil，不需要手動檢查 resp.Code，
// 也不會因為忘記檢查而觸發 resp.Data nil dereference panic。
func EnvelopeGuardInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			return err
		}

		type coder interface{ GetCode() int32 }
		resp, ok := reply.(coder)
		if !ok {
			return nil
		}
		if resp.GetCode() == 0 {
			bizcarrier.Disarm(ctx, method)
			return nil
		}

		code := resp.GetCode()

		msg := fmt.Sprintf("service error (code=%d)", code)
		if m, ok := reply.(interface{ GetMessage() string }); ok && m.GetMessage() != "" {
			msg = m.GetMessage()
		}

		if envelopeCodePreserveData[code] {
			if bizcarrier.Arm(ctx, method, code, msg) {
				return nil
			}
		}

		mapping, found := envelopeCodeToGRPC[code]
		if !found {
			mapping = envelopeMapping{grpcCode: codes.FailedPrecondition, preserveBizCode: true}
		}
		if mapping.preserveBizCode {
			msg = fmt.Sprintf("[%d] %s", code, msg)
		}

		return status.Error(mapping.grpcCode, msg)
	}
}

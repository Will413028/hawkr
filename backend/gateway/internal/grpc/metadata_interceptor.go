package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ==================== Context Keys ====================
// 用於在 Go context 中傳遞身分與追蹤資訊（由 auth/tracing middleware 設定）

type contextKey string

const (
	// CtxKeyAccountID 帳號 ID（int64）
	CtxKeyAccountID contextKey = "auth-account-id"
	// CtxKeyAccountType 帳號類型（string: buyer/seller/admin）
	CtxKeyAccountType contextKey = "auth-account-type"
	// CtxKeyTraceID 追蹤 ID（string）
	CtxKeyTraceID contextKey = "auth-trace-id"
	// CtxKeyRequestID 請求 ID（string）
	CtxKeyRequestID contextKey = "auth-request-id"
)

// ==================== Context 存取工具 ====================

// WithAuthContext 將身分資訊存入 Go context
// 由 auth.go middleware 呼叫
// 對應 API 設計規範 §6.2：僅傳遞 x-account-id, x-account-type
func WithAuthContext(ctx context.Context, accountID int64, accountType string) context.Context {
	if accountID > 0 {
		ctx = context.WithValue(ctx, CtxKeyAccountID, accountID)
	}
	if accountType != "" {
		ctx = context.WithValue(ctx, CtxKeyAccountType, accountType)
	}
	return ctx
}

// WithTraceContext 將追蹤資訊存入 Go context
// 由 tracing.go middleware 呼叫
func WithTraceContext(ctx context.Context, traceID, requestID string) context.Context {
	if traceID != "" {
		ctx = context.WithValue(ctx, CtxKeyTraceID, traceID)
	}
	if requestID != "" {
		ctx = context.WithValue(ctx, CtxKeyRequestID, requestID)
	}
	return ctx
}

// GenerateRequestID 生成 UUID 格式的 Request ID
func GenerateRequestID() string {
	return uuid.New().String()
}

// ==================== gRPC Client Interceptor ====================

// AuthMetadataInterceptor 建立 gRPC Client UnaryInterceptor，
// 自動從 Go context 提取身分與追蹤資訊，注入 gRPC outgoing metadata。
//
// 對應 API 設計規範 §6.2：
// - x-account-id
// - x-account-type
// - x-trace-id
// - x-request-id
func AuthMetadataInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

		pairs := make([]string, 0, 8)

		// 身分資訊
		if accountID, ok := ctx.Value(CtxKeyAccountID).(int64); ok && accountID > 0 {
			pairs = append(pairs, "x-account-id", fmt.Sprintf("%d", accountID))
		}
		if accountType, ok := ctx.Value(CtxKeyAccountType).(string); ok && accountType != "" {
			pairs = append(pairs, "x-account-type", accountType)
		}

		// 追蹤資訊
		if traceID, ok := ctx.Value(CtxKeyTraceID).(string); ok && traceID != "" {
			pairs = append(pairs, "x-trace-id", traceID)
		}
		if requestID, ok := ctx.Value(CtxKeyRequestID).(string); ok && requestID != "" {
			pairs = append(pairs, "x-request-id", requestID)
		}

		// 合併既有 metadata（保留 OTel 等已設定的 metadata）
		if len(pairs) > 0 {
			md := metadata.Pairs(pairs...)
			if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
				md = metadata.Join(existingMD, md)
			}
			ctx = metadata.NewOutgoingContext(ctx, md)
		}

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

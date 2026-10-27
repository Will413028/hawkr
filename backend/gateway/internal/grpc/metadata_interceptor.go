package grpc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/identity"
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
	// CtxKeySellerSuspended 賣家停權原因碼（string；46 §4.4 IM/social 訂單關聯細分）。
	// 只能由 injectSellerSuspension middleware 寫入（外部同名 header 已在該
	// middleware 清除，且外部輸入無法寫 Go context）→ interceptor 轉發即可信。
	CtxKeySellerSuspended contextKey = "auth-seller-suspended"
	// CtxKeyBuyerID 買家 ID（identity.BuyerID）。僅 AIAssistantService 目前使用，
	// 由呼叫端（AI-assistant 買家路由 handler）依路由前綴解析出的 domain id 明確設定，
	// 不是 auth middleware 全域塞入（跟 CtxKeyAccountID 不同，這不是登入 principal）。
	CtxKeyBuyerID contextKey = "auth-buyer-id"
	// CtxKeyStoreID 賣家 store ID（identity.StoreID）。用法同 CtxKeyBuyerID。
	CtxKeyStoreID contextKey = "auth-store-id"
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

// WithSellerSuspension 將停權原因碼存入 Go context，由 injectSellerSuspension
// middleware 呼叫（唯一合法寫入點）。AuthMetadataInterceptor 讀到即轉發
// x-seller-suspended 給下游（im/social usecase 訂單關聯/self-vs-other 細分用）。
func WithSellerSuspension(ctx context.Context, reasonCode string) context.Context {
	if reasonCode == "" {
		return ctx
	}
	return context.WithValue(ctx, CtxKeySellerSuspended, reasonCode)
}

// WithBuyerID 將買家 domain ID 存入 outgoing context，AuthMetadataInterceptor 讀到後
// 轉發 x-buyer-id metadata。呼叫端自行解析 buyer_id 後呼叫本函式，不經過 auth middleware。
func WithBuyerID(ctx context.Context, buyerID identity.BuyerID) context.Context {
	if buyerID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, CtxKeyBuyerID, buyerID)
}

// WithStoreID 將賣家 store domain ID 存入 outgoing context，用法同 WithBuyerID。
func WithStoreID(ctx context.Context, storeID identity.StoreID) context.Context {
	if storeID <= 0 {
		return ctx
	}
	return context.WithValue(ctx, CtxKeyStoreID, storeID)
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
// - x-seller-suspended（僅停權注入 middleware 命中時；46 §4.4）
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

		// 賣家停權原因碼（僅 injectSellerSuspension middleware 會寫入 context；
		// 未注入 = 零成本不帶 key。46 §4.4 IM/social 細分）
		if reason, ok := ctx.Value(CtxKeySellerSuspended).(string); ok && reason != "" {
			pairs = append(pairs, "x-seller-suspended", reason)
		}

		// 買家/賣家 domain ID（僅 AIAssistantService 目前設定，見 WithBuyerID/WithStoreID）
		if buyerID, ok := ctx.Value(CtxKeyBuyerID).(identity.BuyerID); ok && buyerID > 0 {
			pairs = append(pairs, "x-buyer-id", strconv.FormatInt(int64(buyerID), 10))
		}
		if storeID, ok := ctx.Value(CtxKeyStoreID).(identity.StoreID); ok && storeID > 0 {
			pairs = append(pairs, "x-store-id", strconv.FormatInt(int64(storeID), 10))
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

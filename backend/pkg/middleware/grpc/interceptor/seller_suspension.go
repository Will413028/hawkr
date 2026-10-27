package interceptor

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// metadataKeySellerSuspended is the gRPC metadata key carrying the Gateway
// `X-Seller-Suspended` header forward (46 號 §4.4). gRPC lowercases metadata
// keys, so this is the incoming-context lookup key regardless of the HTTP
// header's original casing.
//
// 傳播鏈：gateway `injectSellerSuspension()` 停權命中時以 context value 注入
// 停權原因碼（HTTP header 為輔助載體）→ `AuthMetadataInterceptor` 轉發
// x-seller-suspended 進 outgoing gRPC metadata
// （gateway/internal/grpc/metadata_interceptor.go）。外部偽造 header 在 gateway
// middleware 即被清除，不會透傳至此。
const metadataKeySellerSuspended = "x-seller-suspended"

// SellerSuspendedFromContext reads the forwarded seller-suspension reason
// code from incoming gRPC metadata. Absent/empty metadata = not suspended
// (fail-open at the transport read, matching Gateway's own
// injectSellerSuspension "不 deny" semantics — the actual gating decision is
// the caller usecase's responsibility).
//
// reasonCode is the raw forwarded value (empty when suspended is false).
// Most current callers only need the boolean; reasonCode is exposed for
// future self-vs-other 細分 or 原因碼-specific gating.
func SellerSuspendedFromContext(ctx context.Context) (reasonCode string, suspended bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}
	vals := md.Get(metadataKeySellerSuspended)
	if len(vals) == 0 || vals[0] == "" {
		return "", false
	}
	return vals[0], true
}

package grpc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/go-pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// AuthMetadataStreamInterceptor is the streaming counterpart of
// AuthMetadataInterceptor — forwards the exact same seven context-carried
// identity/trace values into outgoing metadata (account-id, account-type,
// trace-id, request-id, seller-suspended, buyer-id, store-id), but for grpc.NewStream
// instead of grpc.Invoke. See metadata_interceptor.go for the field list
// and WithBuyerID/WithStoreID for why buyer/store id live in a separate
// code path from the auth-middleware-populated fields.
func AuthMetadataStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {

		pairs := make([]string, 0, 8)

		if accountID, ok := ctx.Value(CtxKeyAccountID).(int64); ok && accountID > 0 {
			pairs = append(pairs, "x-account-id", fmt.Sprintf("%d", accountID))
		}
		if accountType, ok := ctx.Value(CtxKeyAccountType).(string); ok && accountType != "" {
			pairs = append(pairs, "x-account-type", accountType)
		}
		if traceID, ok := ctx.Value(CtxKeyTraceID).(string); ok && traceID != "" {
			pairs = append(pairs, "x-trace-id", traceID)
		}
		if requestID, ok := ctx.Value(CtxKeyRequestID).(string); ok && requestID != "" {
			pairs = append(pairs, "x-request-id", requestID)
		}
		if reason, ok := ctx.Value(CtxKeySellerSuspended).(string); ok && reason != "" {
			pairs = append(pairs, "x-seller-suspended", reason)
		}
		if buyerID, ok := ctx.Value(CtxKeyBuyerID).(identity.BuyerID); ok && buyerID > 0 {
			pairs = append(pairs, "x-buyer-id", strconv.FormatInt(int64(buyerID), 10))
		}
		if storeID, ok := ctx.Value(CtxKeyStoreID).(identity.StoreID); ok && storeID > 0 {
			pairs = append(pairs, "x-store-id", strconv.FormatInt(int64(storeID), 10))
		}

		if len(pairs) > 0 {
			md := metadata.Pairs(pairs...)
			if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
				md = metadata.Join(existingMD, md)
			}
			ctx = metadata.NewOutgoingContext(ctx, md)
		}

		return streamer(ctx, desc, cc, method, opts...)
	}
}

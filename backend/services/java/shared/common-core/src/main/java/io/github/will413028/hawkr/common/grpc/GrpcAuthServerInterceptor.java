package io.github.will413028.hawkr.common.grpc;

import io.grpc.Context;
import io.grpc.Contexts;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * gRPC Server 端攔截器：從 metadata 提取身分資訊與內部呼叫標記
 *
 * 提取的 metadata：
 * - x-account-id：帳號 ID（Gateway JWT 注入）
 * - x-account-type：帳號類型 buyer/seller/admin（Gateway JWT 注入）
 * - x-internal-service：內部微服務呼叫標記（由 GrpcAuthClientInterceptor 自動注入）
 *
 * 提取後放入 GrpcAuthContext 的 Context.Key，供 Service 方法透過
 * GrpcAuthContext.getAccountId() / getAccountType() / isInternalCall() 取用。
 *
 * 此攔截器僅做提取，不做強制驗證（允許匿名 / SERVICE_ONLY 流程）。
 * 授權邏輯由各 Service 方法自行呼叫 GrpcAuthContext.requireXxx() 實現，
 * 內部呼叫時 requireXxx() 自動跳過。
 */
public class GrpcAuthServerInterceptor implements ServerInterceptor {

    private static final Logger log = LoggerFactory.getLogger(GrpcAuthServerInterceptor.class);

    private static final Metadata.Key<String> KEY_ACCOUNT_ID =
            Metadata.Key.of(GrpcAuthContext.HEADER_ACCOUNT_ID, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_ACCOUNT_TYPE =
            Metadata.Key.of(GrpcAuthContext.HEADER_ACCOUNT_TYPE, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_INTERNAL_SERVICE =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_SERVICE, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_INTERNAL_TS =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_TS, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_INTERNAL_NONCE =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_NONCE, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_INTERNAL_SIG =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_SIG, Metadata.ASCII_STRING_MARSHALLER);

    @Override
    public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
            ServerCall<ReqT, RespT> call,
            Metadata headers,
            ServerCallHandler<ReqT, RespT> next) {

        Context ctx = Context.current();

        // 提取 x-internal-service（微服務間呼叫標記）並依 mode 驗簽（#1366 HMAC）。
        // 相容關鍵：只對「帶 x-internal-service」的呼叫驗簽；不帶的（gateway→service 使用者呼叫）
        // 走原 requireAccountType 使用者面授權路徑，不受此邏輯影響。
        String internalService = headers.get(KEY_INTERNAL_SERVICE);
        if (internalService != null && !internalService.isBlank()) {
            if (InternalAuthConfig.isOff()) {
                // off：回到明文信任的舊行為。
                ctx = ctx.withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
            } else {
                String sig = headers.get(KEY_INTERNAL_SIG);
                String accountIdRaw = headers.get(KEY_ACCOUNT_ID);
                String accountIdOrEmpty = accountIdRaw == null ? "" : accountIdRaw.trim();

                if (sig == null || sig.isBlank()) {
                    // 無簽章的內部呼叫。
                    if (InternalAuthConfig.isEnforcing()) {
                        return reject(call, "internal call missing signature");
                    }
                    log.warn("internal.auth.unsigned caller={} method={}",
                            internalService, call.getMethodDescriptor().getFullMethodName());
                    ctx = ctx.withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
                } else {
                    boolean valid = InternalAuthVerifier.verify(
                            internalService,
                            headers.get(KEY_INTERNAL_TS),
                            headers.get(KEY_INTERNAL_NONCE),
                            accountIdOrEmpty,
                            sig);
                    if (valid) {
                        ctx = ctx.withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
                    } else if (InternalAuthConfig.isEnforcing()) {
                        return reject(call, "internal call signature invalid");
                    } else {
                        // permissive：簽章無效仍放行（過渡期兜底），記 WARN 供觀察。
                        log.warn("internal.auth.invalid caller={} method={}",
                                internalService, call.getMethodDescriptor().getFullMethodName());
                        ctx = ctx.withValue(GrpcAuthContext.INTERNAL_CALL, Boolean.TRUE);
                    }
                }
            }
        }

        // 提取 x-account-id
        Long accountId = parseLong(headers.get(KEY_ACCOUNT_ID));
        if (accountId != null) {
            ctx = ctx.withValue(GrpcAuthContext.ACCOUNT_ID, accountId);
        }

        // 提取 x-account-type（正規化為小寫）
        String accountType = headers.get(KEY_ACCOUNT_TYPE);
        if (accountType != null && !accountType.isBlank()) {
            ctx = ctx.withValue(GrpcAuthContext.ACCOUNT_TYPE, accountType.trim().toLowerCase());
        }

        return Contexts.interceptCall(ctx, call, headers, next);
    }

    /** enforcing mode 下拒絕無有效簽章的內部呼叫（UNAUTHENTICATED）。 */
    private static <ReqT, RespT> ServerCall.Listener<ReqT> reject(ServerCall<ReqT, RespT> call, String message) {
        call.close(Status.UNAUTHENTICATED.withDescription(message), new Metadata());
        return new ServerCall.Listener<>() {};
    }

    private static Long parseLong(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            long value = Long.parseLong(raw.trim());
            return value > 0 ? value : null;
        } catch (NumberFormatException e) {
            return null;
        }
    }
}

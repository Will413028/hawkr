package io.github.will413028.hawkr.common.grpc;

import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.ForwardingClientCall;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;

/**
 * gRPC Client 端攔截器：自動標記內部呼叫並傳播身分 Context
 *
 * 所有透過 GrpcStubFactory 建立的 gRPC 出站呼叫會自動注入：
 * 1. x-internal-service：標記為內部微服務呼叫，目標服務跳過權限檢查
 * 2. x-account-id：從當前 gRPC Context 傳播（若存在）
 * 3. x-account-type：從當前 gRPC Context 傳播（若存在）
 *
 * 搭配 GrpcAuthServerInterceptor + GrpcAuthContext：
 * - Server 端偵測 x-internal-service → isInternalCall() = true
 * - requireAccountType() 自動跳過
 * - requireAccountId() 回傳傳播的值（或 0）
 *
 * 跨語言支援：
 * 非 Java 微服務（如 Go）只需在 gRPC metadata 中帶上
 * x-internal-service 標頭，即可觸發相同的免驗機制。
 */
public class GrpcAuthClientInterceptor implements ClientInterceptor {

    private static final Metadata.Key<String> KEY_INTERNAL_SERVICE =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_SERVICE, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_INTERNAL_TS =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_TS, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_INTERNAL_NONCE =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_NONCE, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_INTERNAL_SIG =
            Metadata.Key.of(GrpcAuthContext.HEADER_INTERNAL_SIG, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_ACCOUNT_ID =
            Metadata.Key.of(GrpcAuthContext.HEADER_ACCOUNT_ID, Metadata.ASCII_STRING_MARSHALLER);

    private static final Metadata.Key<String> KEY_ACCOUNT_TYPE =
            Metadata.Key.of(GrpcAuthContext.HEADER_ACCOUNT_TYPE, Metadata.ASCII_STRING_MARSHALLER);

    private final String serviceName;

    /**
     * @param serviceName 當前微服務名稱（如 product-service），用於 x-internal-service 標頭
     */
    public GrpcAuthClientInterceptor(String serviceName) {
        this.serviceName = serviceName;
    }

    @Override
    public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
            MethodDescriptor<ReqT, RespT> method,
            CallOptions callOptions,
            Channel next) {

        return new ForwardingClientCall.SimpleForwardingClientCall<>(
                next.newCall(method, callOptions)) {

            @Override
            public void start(Listener<RespT> responseListener, Metadata headers) {
                // 1. 標記內部微服務呼叫（caller 名，資訊用途）
                headers.put(KEY_INTERNAL_SERVICE, serviceName);

                // 2. 傳播 x-account-id（從當前 gRPC Context，若存在）
                Long accountId = GrpcAuthContext.ACCOUNT_ID.get();
                String accountIdOrEmpty = (accountId != null && accountId > 0) ? String.valueOf(accountId) : "";
                if (!accountIdOrEmpty.isEmpty()) {
                    headers.put(KEY_ACCOUNT_ID, accountIdOrEmpty);
                }

                // 3. 傳播 x-account-type（從當前 gRPC Context，若存在）
                String accountType = GrpcAuthContext.ACCOUNT_TYPE.get();
                if (accountType != null && !accountType.isBlank()) {
                    headers.put(KEY_ACCOUNT_TYPE, accountType);
                }

                // 4. 內部身分 HMAC 簽章（#1366）：綁定傳播的 accountId，防窗內身分抽換。
                //    permissive/off 缺 secret 時不簽（過渡期靠 server WARN 觀察）；有 secret 才簽。
                if (InternalAuthConfig.hasSecret()) {
                    String ts = InternalAuthSigner.currentTs();
                    String nonce = InternalAuthSigner.newNonce();
                    String sig = InternalAuthSigner.sign(
                            InternalAuthConfig.secret(),
                            InternalAuthSigner.canonical(serviceName, ts, nonce, accountIdOrEmpty));
                    headers.put(KEY_INTERNAL_TS, ts);
                    headers.put(KEY_INTERNAL_NONCE, nonce);
                    headers.put(KEY_INTERNAL_SIG, sig);
                }

                super.start(responseListener, headers);
            }
        };
    }
}

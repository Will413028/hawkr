package io.github.will413028.hawkr.common.grpc;

import io.grpc.ServerInterceptor;
import jakarta.annotation.PostConstruct;
import net.devh.boot.grpc.server.interceptor.GrpcGlobalServerInterceptor;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnClass;
import org.springframework.context.annotation.Configuration;

/**
 * gRPC 授權攔截器自動配置
 *
 * 當 classpath 中存在 gRPC ServerInterceptor 時自動註冊 GrpcAuthServerInterceptor。
 * 使用 @GrpcGlobalServerInterceptor 確保 net.devh starter 正確套用至所有 gRPC services。
 *
 * 測試環境可透過設定 hawkr.auth.skip-account-type-check=true
 * 或環境變數 SKIP_ACCOUNT_TYPE_CHECK=true 跳過 account-type 檢查，
 * JWT 驗證仍然有效。
 */
@Configuration
@ConditionalOnClass(ServerInterceptor.class)
public class GrpcAuthAutoConfiguration {

    private static final Logger log = LoggerFactory.getLogger(GrpcAuthAutoConfiguration.class);

    @Value("${hawkr.auth.skip-account-type-check:${SKIP_ACCOUNT_TYPE_CHECK:false}}")
    private boolean skipAccountTypeCheck;

    // 內部身分 HMAC 簽章設定（#1366）。預設 permissive；enforcing 由 PR-C 純環境變數切換。
    @Value("${hawkr.auth.internal.mode:${INTERNAL_AUTH_MODE:permissive}}")
    private String internalAuthMode;

    @Value("${hawkr.auth.internal.secret:${INTERNAL_SERVICE_HMAC_SECRET:}}")
    private String internalAuthSecret;

    @Value("${hawkr.auth.internal.secret-prev:${INTERNAL_SERVICE_HMAC_SECRET_PREV:}}")
    private String internalAuthSecretPrev;

    @Value("${hawkr.auth.internal.ts-window-seconds:${INTERNAL_AUTH_TS_WINDOW_SECONDS:300}}")
    private long internalAuthTsWindowSeconds;

    @GrpcGlobalServerInterceptor
    public GrpcAuthServerInterceptor grpcAuthServerInterceptor() {
        return new GrpcAuthServerInterceptor();
    }

    @PostConstruct
    public void initAuthConfig() {
        GrpcAuthContext.setSkipAccountTypeCheck(skipAccountTypeCheck);
        if (skipAccountTypeCheck) {
            log.warn("Account-type 檢查已停用（測試模式）— JWT 驗證仍有效");
        }

        InternalAuthConfig.configure(
                internalAuthMode, internalAuthSecret, internalAuthSecretPrev, internalAuthTsWindowSeconds);

        // enforcing 缺 secret → fail-closed（沿用 #1396 StagedTokenService 模式，不明文 fallback）。
        if (InternalAuthConfig.isEnforcing() && !InternalAuthConfig.hasSecret()) {
            throw new IllegalStateException(
                    "internal-auth mode=enforcing 但缺 INTERNAL_SERVICE_HMAC_SECRET，拒絕啟動（fail-closed）");
        }
        if (!InternalAuthConfig.hasSecret()) {
            log.warn("internal-auth mode={} 缺 INTERNAL_SERVICE_HMAC_SECRET — 出站呼叫不簽章、入站不驗簽（過渡期）",
                    InternalAuthConfig.mode());
        } else {
            log.info("internal-auth mode={} 已載入共享密鑰{}（ts-window={}s）",
                    InternalAuthConfig.mode(),
                    InternalAuthConfig.secretPrev().isEmpty() ? "" : "（含輪替 PREV）",
                    InternalAuthConfig.tsWindowSeconds());
        }
    }
}

package io.github.will413028.hawkr.common.grpc;

/**
 * 內部身分 HMAC 簽章的程序層級設定（issue #1366 PR-B）。
 *
 * 由 {@link GrpcAuthAutoConfiguration} 於啟動時從 {@code hawkr.auth.internal.*}
 * / 環境變數載入一次，供 {@link GrpcAuthClientInterceptor}（簽）與
 * {@link GrpcAuthServerInterceptor}（驗）共同讀取（沿用 {@code setSkipAccountTypeCheck} 的靜態持有模式，
 * 讓 observability-common 的 client interceptor 與 common-core 的 server interceptor 皆可取用）。
 *
 * 密鑰採全體服務共用單一密鑰（+ 輪替期的 PREV），信任邊界＝「是不是合法第一方服務」。
 */
public final class InternalAuthConfig {

    public static final String MODE_OFF = "off";
    public static final String MODE_PERMISSIVE = "permissive";
    public static final String MODE_ENFORCING = "enforcing";

    private static final long DEFAULT_TS_WINDOW_SECONDS = 300L;

    private static volatile String mode = MODE_PERMISSIVE;
    private static volatile String secret = "";
    private static volatile String secretPrev = "";
    private static volatile long tsWindowSeconds = DEFAULT_TS_WINDOW_SECONDS;

    private InternalAuthConfig() {}

    /**
     * 設定內部身分驗證參數。由 GrpcAuthAutoConfiguration 在啟動時呼叫。
     */
    public static void configure(String mode, String secret, String secretPrev, long tsWindowSeconds) {
        InternalAuthConfig.mode = normalizeMode(mode);
        InternalAuthConfig.secret = secret == null ? "" : secret;
        InternalAuthConfig.secretPrev = secretPrev == null ? "" : secretPrev;
        InternalAuthConfig.tsWindowSeconds = tsWindowSeconds > 0 ? tsWindowSeconds : DEFAULT_TS_WINDOW_SECONDS;
    }

    private static String normalizeMode(String m) {
        if (m == null) {
            return MODE_PERMISSIVE;
        }
        String v = m.trim().toLowerCase();
        return switch (v) {
            case MODE_OFF, MODE_PERMISSIVE, MODE_ENFORCING -> v;
            default -> MODE_PERMISSIVE;
        };
    }

    public static String mode() {
        return mode;
    }

    public static String secret() {
        return secret;
    }

    public static String secretPrev() {
        return secretPrev;
    }

    public static long tsWindowSeconds() {
        return tsWindowSeconds;
    }

    public static boolean isOff() {
        return MODE_OFF.equals(mode);
    }

    public static boolean isEnforcing() {
        return MODE_ENFORCING.equals(mode);
    }

    public static boolean hasSecret() {
        return secret != null && !secret.isEmpty();
    }
}

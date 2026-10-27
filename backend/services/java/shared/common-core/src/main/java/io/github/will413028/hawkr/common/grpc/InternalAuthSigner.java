package io.github.will413028.hawkr.common.grpc;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.util.Base64;
import java.util.HexFormat;

/**
 * 內部身分 HMAC-SHA256 簽章器（issue #1366 PR-B）。純函式，用 JDK {@link Mac}，不引 jjwt。
 *
 * canonical / base64 編碼與 Go 端 {@code pkg/middleware/grpc/interceptor/internal_auth.go}
 * 逐位元組一致，兩語言以同一把密鑰互簽互驗：
 *
 * <pre>canonical = serviceName + "\n" + ts + "\n" + nonce + "\n" + accountIdOrEmpty</pre>
 */
public final class InternalAuthSigner {

    private static final SecureRandom RANDOM = new SecureRandom();

    private InternalAuthSigner() {}

    /** 組出待簽字串（UTF-8, LF 分隔）。 */
    public static String canonical(String serviceName, String ts, String nonce, String accountIdOrEmpty) {
        return serviceName + "\n" + ts + "\n" + nonce + "\n" + accountIdOrEmpty;
    }

    /** 回傳 base64( HMAC-SHA256( secret, canonical ) )。 */
    public static String sign(String secret, String canonical) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
            byte[] raw = mac.doFinal(canonical.getBytes(StandardCharsets.UTF_8));
            return Base64.getEncoder().encodeToString(raw);
        } catch (Exception e) {
            throw new IllegalStateException("HMAC-SHA256 簽章失敗", e);
        }
    }

    /** 產生 16 bytes 隨機 nonce 的 hex 字串（forward-compat，v1 不校驗唯一性）。 */
    public static String newNonce() {
        byte[] b = new byte[16];
        RANDOM.nextBytes(b);
        return HexFormat.of().formatHex(b);
    }

    /** 當前 unix 秒（字串）。 */
    public static String currentTs() {
        return Long.toString(System.currentTimeMillis() / 1000L);
    }
}

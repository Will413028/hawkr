package io.github.will413028.hawkr.common.grpc;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;

/**
 * 內部身分 HMAC 簽章驗證器（issue #1366 PR-B）。純函式。
 *
 * 驗證流程：時間窗（|now - ts| ≤ tsWindowSeconds）→ 用 secret（及輪替期 secretPrev）
 * 重算 canonical HMAC，以常數時間比對簽章（{@link MessageDigest#isEqual}）。任一密鑰通過即有效。
 */
public final class InternalAuthVerifier {

    private InternalAuthVerifier() {}

    /**
     * 驗證內部身分簽章。
     *
     * @param serviceName      x-internal-service（caller 名，進 canonical）
     * @param ts               x-internal-ts
     * @param nonce            x-internal-nonce
     * @param accountIdOrEmpty 傳播的 x-account-id 原字串（無則空字串），進 canonical 末段
     * @param sigBase64        x-internal-sig
     * @return true 若簽章在時間窗內且與 secret 或 secretPrev 相符
     */
    public static boolean verify(String serviceName, String ts, String nonce,
                                 String accountIdOrEmpty, String sigBase64) {
        if (sigBase64 == null || sigBase64.isBlank()) {
            return false;
        }
        if (!InternalAuthConfig.hasSecret()) {
            return false;
        }
        if (ts == null || ts.isBlank()) {
            return false;
        }
        long tsVal;
        try {
            tsVal = Long.parseLong(ts.trim());
        } catch (NumberFormatException e) {
            return false;
        }
        long now = System.currentTimeMillis() / 1000L;
        if (Math.abs(now - tsVal) > InternalAuthConfig.tsWindowSeconds()) {
            return false;
        }

        String canonical = InternalAuthSigner.canonical(
                serviceName,
                ts.trim(),
                nonce == null ? "" : nonce,
                accountIdOrEmpty == null ? "" : accountIdOrEmpty);

        if (matches(InternalAuthConfig.secret(), canonical, sigBase64)) {
            return true;
        }
        String prev = InternalAuthConfig.secretPrev();
        return prev != null && !prev.isEmpty() && matches(prev, canonical, sigBase64);
    }

    private static boolean matches(String secret, String canonical, String sigBase64) {
        String expected = InternalAuthSigner.sign(secret, canonical);
        return MessageDigest.isEqual(
                expected.getBytes(StandardCharsets.UTF_8),
                sigBase64.getBytes(StandardCharsets.UTF_8));
    }
}

package io.github.will413028.hawkr.common.grpc;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 內部身分 HMAC 簽章跨語言一致性測試（issue #1366 PR-B）。
 *
 * 黃金值由同一 HMAC-SHA256 演算法對固定輸入產出，Go 端
 * {@code pkg/middleware/grpc/interceptor/internal_auth_test.go} 斷言相同的兩組值，
 * 證明 Java signer 與 Go signer 對同一 (serviceName, ts, nonce, accountId) 逐位元組一致。
 */
class InternalAuthSignerTest {

    private static final String SECRET = "test-shared-secret-please-change";

    // 黃金值一：帶 accountId
    private static final String SVC_1 = "order-service";
    private static final String TS_1 = "1700000000";
    private static final String NONCE_1 = "00112233445566778899aabbccddeeff";
    private static final String ACCT_1 = "123456789";
    private static final String GOLDEN_SIG_1 = "TXm3Uefzoq0Z8ru9n++yFm4DjaEs9XZ2GWMpHsi3BOk=";

    // 黃金值二：空 accountId
    private static final String SVC_2 = "crm-service";
    private static final String TS_2 = "1700000000";
    private static final String NONCE_2 = "abcdef00112233445566778899aabbcc";
    private static final String GOLDEN_SIG_2 = "HTg2eOBanXJvJEs67j5dVl+DQXIjU3rvQAmIySTmcZg=";

    @Test
    void signMatchesGoWithAccount() {
        String canonical = InternalAuthSigner.canonical(SVC_1, TS_1, NONCE_1, ACCT_1);
        assertEquals(GOLDEN_SIG_1, InternalAuthSigner.sign(SECRET, canonical),
                "Java 簽章與 Go 黃金值（帶 accountId）不一致");
    }

    @Test
    void signMatchesGoWithEmptyAccount() {
        String canonical = InternalAuthSigner.canonical(SVC_2, TS_2, NONCE_2, "");
        assertEquals(GOLDEN_SIG_2, InternalAuthSigner.sign(SECRET, canonical),
                "Java 簽章與 Go 黃金值（空 accountId）不一致");
    }

    @Test
    void verifyAcceptsFreshValidSignatureAndRejectsTampered() {
        InternalAuthConfig.configure(InternalAuthConfig.MODE_PERMISSIVE, SECRET, "", 300L);
        try {
            String ts = InternalAuthSigner.currentTs();
            String nonce = InternalAuthSigner.newNonce();
            String sig = InternalAuthSigner.sign(SECRET, InternalAuthSigner.canonical("buyer-service", ts, nonce, "555"));

            assertTrue(InternalAuthVerifier.verify("buyer-service", ts, nonce, "555", sig),
                    "有效簽章應驗過");
            // 抽換 accountId → 簽章不再相符
            assertFalse(InternalAuthVerifier.verify("buyer-service", ts, nonce, "999", sig),
                    "換 accountId 後應驗不過（簽章綁 accountId）");
            // 亂改簽章
            assertFalse(InternalAuthVerifier.verify("buyer-service", ts, nonce, "555", "tampered"),
                    "竄改簽章應驗不過");
        } finally {
            InternalAuthConfig.configure(InternalAuthConfig.MODE_PERMISSIVE, "", "", 300L);
        }
    }

    @Test
    void verifyRejectsOutOfWindowTimestamp() {
        InternalAuthConfig.configure(InternalAuthConfig.MODE_PERMISSIVE, SECRET, "", 300L);
        try {
            long staleTsVal = (System.currentTimeMillis() / 1000L) - 10_000L; // 遠超 300s 窗
            String ts = Long.toString(staleTsVal);
            String nonce = InternalAuthSigner.newNonce();
            String sig = InternalAuthSigner.sign(SECRET, InternalAuthSigner.canonical("buyer-service", ts, nonce, "555"));
            assertFalse(InternalAuthVerifier.verify("buyer-service", ts, nonce, "555", sig),
                    "超出時間窗的簽章應驗不過");
        } finally {
            InternalAuthConfig.configure(InternalAuthConfig.MODE_PERMISSIVE, "", "", 300L);
        }
    }

    @Test
    void verifyAcceptsPrevSecretDuringRotation() {
        // current = 新密鑰，prev = 舊密鑰；用舊密鑰簽的訊息在輪替窗內仍應被接受
        InternalAuthConfig.configure(InternalAuthConfig.MODE_PERMISSIVE, "new-secret", SECRET, 300L);
        try {
            String ts = InternalAuthSigner.currentTs();
            String nonce = InternalAuthSigner.newNonce();
            String sigWithOld = InternalAuthSigner.sign(SECRET, InternalAuthSigner.canonical("buyer-service", ts, nonce, "555"));
            assertTrue(InternalAuthVerifier.verify("buyer-service", ts, nonce, "555", sigWithOld),
                    "輪替期用 PREV 密鑰簽的簽章應被接受");
        } finally {
            InternalAuthConfig.configure(InternalAuthConfig.MODE_PERMISSIVE, "", "", 300L);
        }
    }
}

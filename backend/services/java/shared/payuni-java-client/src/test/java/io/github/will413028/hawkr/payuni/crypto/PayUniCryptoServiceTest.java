package io.github.will413028.hawkr.payuni.crypto;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * PayUni 加解密 Golden Test
 * <p>
 * 對齊 PHP 官方範例與 CryptoTest.java 測試資料，
 * 確保 SDK 搬出後加密行為完全一致。
 */
class PayUniCryptoServiceTest {

    // PHP 官方範例測試金鑰
    private static final String TEST_KEY = "12345678901234567890123456789012";  // 32 字元
    private static final String TEST_IV = "1234567890123456";                   // 16 字元

    // 實際代理商測試金鑰（與 webhook_crypto.sh 一致）
    private static final String AGENT_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String AGENT_IV = "TestOnlyFakeIV00";

    @Nested
    @DisplayName("PHP 官方範例驗證")
    class PhpOfficialTest {

        private final PayUniCryptoService crypto = new PayUniCryptoService(TEST_KEY, TEST_IV);

        // PHP 官方預期輸出
        private static final String PHP_EXPECTED =
                "47396636346f66735853533167396942344f587a3775696b34752b596e70452b3a3a3a4373354a5a5143306b7153467531354c6e6f554a69773d3d";

        @Test
        @DisplayName("加密結果與 PHP 官方範例一致")
        void encryptMatchesPhpOutput() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "AAA");
            params.put("MerTradeNo", "BBB");

            String encrypted = crypto.encrypt(params);

            assertEquals(PHP_EXPECTED, encrypted,
                    "Java 加密結果必須與 PHP 官方範例完全一致");
        }

        @Test
        @DisplayName("解密 PHP 加密結果正確")
        void decryptPhpOutput() {
            Map<String, String> decrypted = crypto.decrypt(PHP_EXPECTED);

            assertEquals("AAA", decrypted.get("MerID"));
            assertEquals("BBB", decrypted.get("MerTradeNo"));
        }

        @Test
        @DisplayName("加密後解密 round-trip 一致")
        void encryptDecryptRoundTrip() {
            Map<String, String> original = new LinkedHashMap<>();
            original.put("MerID", "AAA");
            original.put("MerTradeNo", "BBB");

            String encrypted = crypto.encrypt(original);
            Map<String, String> decrypted = crypto.decrypt(encrypted);

            assertEquals(original.get("MerID"), decrypted.get("MerID"));
            assertEquals(original.get("MerTradeNo"), decrypted.get("MerTradeNo"));
        }

        @Test
        @DisplayName("Hash 計算結果確定性")
        void hashIsDeterministic() {
            String hash1 = crypto.calculateHash(PHP_EXPECTED);
            String hash2 = crypto.calculateHash(PHP_EXPECTED);

            assertNotNull(hash1);
            assertFalse(hash1.isEmpty());
            assertEquals(hash1, hash2, "同一輸入的 hash 必須一致");
            assertEquals(64, hash1.length(), "SHA256 hex 長度為 64");
            assertEquals(hash1, hash1.toUpperCase(), "Hash 結果為大寫");
        }

        @Test
        @DisplayName("Hash 驗證正確")
        void verifyHashWorks() {
            String hash = crypto.calculateHash(PHP_EXPECTED);

            assertTrue(crypto.verifyHash(PHP_EXPECTED, hash));
            assertFalse(crypto.verifyHash(PHP_EXPECTED, "INVALID_HASH"));
        }
    }

    @Nested
    @DisplayName("實際代理商金鑰驗證")
    class AgentKeyTest {

        private final PayUniCryptoService crypto = new PayUniCryptoService(AGENT_KEY, AGENT_IV);

        @Test
        @DisplayName("複合參數加密解密 round-trip")
        void complexParamsRoundTrip() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "S00000001");
            params.put("MerTradeNo", "TEST123");
            params.put("TradeAmt", "100");
            params.put("Timestamp", "1234567890");

            String encrypted = crypto.encrypt(params);
            assertNotNull(encrypted);
            assertFalse(encrypted.isEmpty());

            Map<String, String> decrypted = crypto.decrypt(encrypted);
            assertEquals("S00000001", decrypted.get("MerID"));
            assertEquals("TEST123", decrypted.get("MerTradeNo"));
            assertEquals("100", decrypted.get("TradeAmt"));
            assertEquals("1234567890", decrypted.get("Timestamp"));
        }

        @Test
        @DisplayName("Webhook 付款通知格式 round-trip")
        void webhookPaymentNotifyRoundTrip() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "UTF8290000");
            params.put("MerTradeNo", "WH010_1708067890");
            params.put("TradeNo", "P20260216120000001");
            params.put("TradeStatus", "1");
            params.put("TradeAmt", "300");
            params.put("PayType", "Credit");
            params.put("TradeDate", "2026-02-16 12:00:00");

            String encrypted = crypto.encrypt(params);
            String hash = crypto.calculateHash(encrypted);

            // 驗簽
            assertTrue(crypto.verifyHash(encrypted, hash));

            // 解密
            Map<String, String> decrypted = crypto.decrypt(encrypted);
            assertEquals("UTF8290000", decrypted.get("MerID"));
            assertEquals("WH010_1708067890", decrypted.get("MerTradeNo"));
            assertEquals("1", decrypted.get("TradeStatus"));
            assertEquals("300", decrypted.get("TradeAmt"));
            assertEquals("Credit", decrypted.get("PayType"));
        }

        @Test
        @DisplayName("Webhook 商家狀態通知格式 round-trip")
        void webhookMerStatusRoundTrip() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "UTF8290000");
            params.put("MerStatus", "S");
            params.put("ExecuteTime", "20260216120000");

            String encrypted = crypto.encrypt(params);
            String hash = crypto.calculateHash(encrypted);

            assertTrue(crypto.verifyHash(encrypted, hash));

            Map<String, String> decrypted = crypto.decrypt(encrypted);
            assertEquals("UTF8290000", decrypted.get("MerID"));
            assertEquals("S", decrypted.get("MerStatus"));
            assertEquals("20260216120000", decrypted.get("ExecuteTime"));
        }

        @Test
        @DisplayName("Hash 驗證 — 錯誤的 hash 被拒絕")
        void invalidHashRejected() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("MerID", "TEST");

            String encrypted = crypto.encrypt(params);
            String correctHash = crypto.calculateHash(encrypted);

            // 正確的 hash 通過
            assertTrue(crypto.verifyHash(encrypted, correctHash));

            // 篡改的 hash 被拒絕
            assertFalse(crypto.verifyHash(encrypted, "0000000000000000000000000000000000000000000000000000000000000000"));

            // 空 hash
            assertFalse(crypto.verifyHash(encrypted, ""));
        }
    }

    @Nested
    @DisplayName("邊界條件與錯誤處理")
    class EdgeCaseTest {

        private final PayUniCryptoService crypto = new PayUniCryptoService(TEST_KEY, TEST_IV);

        @Test
        @DisplayName("空參數加密")
        void emptyParamsEncrypt() {
            Map<String, String> params = new LinkedHashMap<>();
            // 空 map 不該拋異常
            String encrypted = crypto.encrypt(params);
            assertNotNull(encrypted);
        }

        @Test
        @DisplayName("包含特殊字元的參數")
        void specialCharactersRoundTrip() {
            Map<String, String> params = new LinkedHashMap<>();
            params.put("Name", "台灣有機蘋果");
            params.put("Desc", "A&B=C D+E");

            String encrypted = crypto.encrypt(params);
            Map<String, String> decrypted = crypto.decrypt(encrypted);

            assertEquals("台灣有機蘋果", decrypted.get("Name"));
            assertEquals("A&B=C D+E", decrypted.get("Desc"));
        }

        @Test
        @DisplayName("無效加密字串拋出 PayUniCryptoException")
        void invalidEncryptedStringThrows() {
            assertThrows(PayUniCryptoException.class, () -> {
                crypto.decrypt("invalid_hex_string");
            });
        }

        @Test
        @DisplayName("TreeMap 排序確保一致性")
        void treeMapSortingConsistency() {
            Map<String, String> params1 = new LinkedHashMap<>();
            params1.put("MerTradeNo", "BBB");
            params1.put("MerID", "AAA");

            Map<String, String> params2 = new LinkedHashMap<>();
            params2.put("MerID", "AAA");
            params2.put("MerTradeNo", "BBB");

            // 不同插入順序的 Map 應產出相同加密結果（因 TreeMap 排序）
            String encrypted1 = crypto.encrypt(params1);
            String encrypted2 = crypto.encrypt(params2);

            assertEquals(encrypted1, encrypted2,
                    "TreeMap 排序後，不同插入順序應產出相同結果");
        }
    }
}

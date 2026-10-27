package io.github.will413028.hawkr.payment;

import javax.crypto.Cipher;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Arrays;
import java.util.Base64;

/**
 * PayUni 加解密測試 - 與 PHP 官方範例比較
 *
 * 執行方式：
 * cd services/payment-service
 * mvn compile exec:java -Dexec.mainClass="io.github.will413028.hawkr.payment.CryptoTest"
 *
 * 或直接在 IDE 中執行 main 方法
 */
public class CryptoTest {

    private static final String TAG_SEPARATOR = ":::";
    private static final int GCM_TAG_LENGTH_BITS = 128;
    private static final int GCM_TAG_LENGTH_BYTES = 16;

    public static void main(String[] args) {
        System.out.println("========================================");
        System.out.println("PayUni 加解密測試 - Java vs PHP 比較");
        System.out.println("========================================\n");

        // PHP 官方範例數據
        String testKey = "12345678901234567890123456789012";  // 32 字元
        String testIv = "1234567890123456";                   // 16 字元
        String plainText = "MerID=AAA&MerTradeNo=BBB";

        // PHP 官方預期輸出
        String phpExpected = "47396636346f66735853533167396942344f587a3775696b34752b596e70452b3a3a3a4373354a5a5143306b7153467531354c6e6f554a69773d3d";

        System.out.println("【測試參數】");
        System.out.println("Key: " + testKey + " (長度: " + testKey.length() + ")");
        System.out.println("IV:  " + testIv + " (長度: " + testIv.length() + ")");
        System.out.println("明文: " + plainText);
        System.out.println("PHP 預期輸出: " + phpExpected);
        System.out.println();

        try {
            // 測試加密
            System.out.println("【Step 1: Java 加密測試】");
            String javaEncrypted = encrypt(plainText, testKey, testIv);
            System.out.println("Java 加密結果: " + javaEncrypted);
            System.out.println("加密結果是否與 PHP 一致: " + javaEncrypted.equals(phpExpected));
            System.out.println();

            // 測試解密 PHP 結果
            System.out.println("【Step 2: 解密 PHP 加密結果】");
            String decrypted = decrypt(phpExpected, testKey, testIv);
            System.out.println("解密結果: " + decrypted);
            System.out.println("解密結果是否正確: " + plainText.equals(decrypted));
            System.out.println();

            // 中間步驟詳細分析
            System.out.println("【Step 3: 詳細分析中間步驟】");
            analyzePhpOutput(phpExpected, testKey, testIv);
            System.out.println();

            // 測試 Hash
            System.out.println("【Step 4: Hash 測試】");
            String hash = calculateHash(phpExpected, testKey, testIv);
            System.out.println("Hash 結果: " + hash);
            System.out.println();

            // 使用你實際的 Key/IV 測試
            System.out.println("========================================");
            System.out.println("【使用實際 Key/IV 測試】");
            System.out.println("========================================\n");

            // TODO: 請替換成你實際的 Key/IV
            String realKey = "TestOnlyFakeHashKey0123456789AB1";  // 你的代理商 Key
            String realIv = "TestOnlyFakeIV00";                   // 你的代理商 IV

            System.out.println("實際 Key: " + realKey + " (長度: " + realKey.length() + ")");
            System.out.println("實際 IV:  " + realIv + " (長度: " + realIv.length() + ")");

            String testData = "MerID=S00000001&MerTradeNo=TEST123&TradeAmt=100&Timestamp=1234567890";
            System.out.println("測試明文: " + testData);

            String realEncrypted = encrypt(testData, realKey, realIv);
            System.out.println("加密結果: " + realEncrypted);

            String realDecrypted = decrypt(realEncrypted, realKey, realIv);
            System.out.println("解密驗證: " + realDecrypted);
            System.out.println("驗證是否一致: " + testData.equals(realDecrypted));

        } catch (Exception e) {
            System.err.println("錯誤: " + e.getMessage());
            e.printStackTrace();
        }
    }

    /**
     * AES-256-GCM 加密
     * 格式: hex(base64(ciphertext) + ":::" + base64(tag))
     */
    public static String encrypt(String plainText, String key, String iv) throws Exception {
        SecretKeySpec secretKeySpec = new SecretKeySpec(key.getBytes(StandardCharsets.UTF_8), "AES");
        GCMParameterSpec gcmParameterSpec = new GCMParameterSpec(GCM_TAG_LENGTH_BITS, iv.getBytes(StandardCharsets.UTF_8));

        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.ENCRYPT_MODE, secretKeySpec, gcmParameterSpec);

        byte[] encryptedBytes = cipher.doFinal(plainText.getBytes(StandardCharsets.UTF_8));

        // 分離 ciphertext 和 tag（tag 是最後 16 bytes）
        byte[] ciphertext = Arrays.copyOfRange(encryptedBytes, 0, encryptedBytes.length - GCM_TAG_LENGTH_BYTES);
        byte[] tag = Arrays.copyOfRange(encryptedBytes, encryptedBytes.length - GCM_TAG_LENGTH_BYTES, encryptedBytes.length);

        // Base64 編碼
        String ciphertextBase64 = Base64.getEncoder().encodeToString(ciphertext);
        String tagBase64 = Base64.getEncoder().encodeToString(tag);

        // 組合並轉 hex
        String combined = ciphertextBase64 + TAG_SEPARATOR + tagBase64;
        return bytesToHex(combined.getBytes(StandardCharsets.UTF_8));
    }

    /**
     * AES-256-GCM 解密
     */
    public static String decrypt(String encryptedStr, String key, String iv) throws Exception {
        // hex 解碼
        byte[] decoded = hexToBytes(encryptedStr);
        String combined = new String(decoded, StandardCharsets.UTF_8);

        // 分離 ciphertext 和 tag
        String[] parts = combined.split(TAG_SEPARATOR);
        if (parts.length != 2) {
            throw new RuntimeException("格式錯誤：找不到 ':::'");
        }

        byte[] ciphertext = Base64.getDecoder().decode(parts[0]);
        byte[] tag = Base64.getDecoder().decode(parts[1]);

        // 合併 ciphertext + tag
        byte[] encryptedData = new byte[ciphertext.length + tag.length];
        System.arraycopy(ciphertext, 0, encryptedData, 0, ciphertext.length);
        System.arraycopy(tag, 0, encryptedData, ciphertext.length, tag.length);

        // 解密
        SecretKeySpec secretKeySpec = new SecretKeySpec(key.getBytes(StandardCharsets.UTF_8), "AES");
        GCMParameterSpec gcmParameterSpec = new GCMParameterSpec(GCM_TAG_LENGTH_BITS, iv.getBytes(StandardCharsets.UTF_8));

        Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
        cipher.init(Cipher.DECRYPT_MODE, secretKeySpec, gcmParameterSpec);

        byte[] decrypted = cipher.doFinal(encryptedData);
        return new String(decrypted, StandardCharsets.UTF_8);
    }

    /**
     * 計算 Hash
     * 格式: SHA256(key + encryptStr + iv).toUpperCase()
     */
    public static String calculateHash(String encryptStr, String key, String iv) throws Exception {
        String rawData = key + encryptStr + iv;
        MessageDigest md = MessageDigest.getInstance("SHA-256");
        byte[] hashBytes = md.digest(rawData.getBytes(StandardCharsets.UTF_8));
        return bytesToHex(hashBytes).toUpperCase();
    }

    /**
     * 分析 PHP 輸出的中間步驟
     */
    public static void analyzePhpOutput(String phpOutput, String key, String iv) throws Exception {
        // Step 1: hex 解碼
        byte[] decoded = hexToBytes(phpOutput);
        String combined = new String(decoded, StandardCharsets.UTF_8);
        System.out.println("1. Hex 解碼後: " + combined);

        // Step 2: 分離 ciphertext 和 tag
        String[] parts = combined.split(TAG_SEPARATOR);
        System.out.println("2. 分離後:");
        System.out.println("   - Ciphertext (Base64): " + parts[0]);
        System.out.println("   - Tag (Base64): " + parts[1]);

        // Step 3: Base64 解碼
        byte[] ciphertext = Base64.getDecoder().decode(parts[0]);
        byte[] tag = Base64.getDecoder().decode(parts[1]);
        System.out.println("3. Base64 解碼後:");
        System.out.println("   - Ciphertext bytes: " + ciphertext.length + " bytes");
        System.out.println("   - Tag bytes: " + tag.length + " bytes");
    }

    private static String bytesToHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder();
        for (byte b : bytes) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }

    private static byte[] hexToBytes(String hex) {
        int len = hex.length();
        byte[] data = new byte[len / 2];
        for (int i = 0; i < len; i += 2) {
            data[i / 2] = (byte) ((Character.digit(hex.charAt(i), 16) << 4)
                    + Character.digit(hex.charAt(i + 1), 16));
        }
        return data;
    }
}

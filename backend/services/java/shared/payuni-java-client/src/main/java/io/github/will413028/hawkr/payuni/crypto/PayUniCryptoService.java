package io.github.will413028.hawkr.payuni.crypto;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.crypto.Cipher;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.*;

/**
 * PayUni 加密服務
 * <p>
 * 根據 PayUni 官方 Java 範例實作
 * <p>
 * 加密規格：
 * - 演算法: AES-256-GCM (使用標準 Java Provider)
 * - Tag 長度: 128 bits
 * - 輸出格式: hex(base64(ciphertext) + ":::" + base64(tag))
 * <p>
 * 簽章規格：
 * - 演算法: SHA256
 * - 格式: SHA256(merKey + encryptStr + merIV).toUpperCase()
 */
public class PayUniCryptoService {

    private static final Logger log = LoggerFactory.getLogger(PayUniCryptoService.class);

    private static final String AES_GCM_TRANSFORMATION = "AES/GCM/NoPadding";
    private static final int GCM_TAG_LENGTH_BITS = 128;
    private static final int GCM_TAG_LENGTH_BYTES = 16;
    private static final String TAG_SEPARATOR = ":::";

    private final String merchantKey;
    private final String merchantIv;

    public PayUniCryptoService(String merchantKey, String merchantIv) {
        this.merchantKey = merchantKey;
        this.merchantIv = merchantIv;

        // 驗證長度是否正確（Key 應為 32 字元，IV 應為 16 字元）
        if (merchantKey == null || merchantKey.length() != 32) {
            log.warn("Key 長度不正確！預期 32 字元，實際 {} 字元",
                    merchantKey != null ? merchantKey.length() : 0);
        }
        if (merchantIv == null || merchantIv.length() != 16) {
            log.warn("IV 長度不正確！預期 16 字元，實際 {} 字元",
                    merchantIv != null ? merchantIv.length() : 0);
        }
    }

    /**
     * 加密請求參數
     *
     * @param params 請求參數 Map
     * @return 加密後的字串（EncryptStr）
     */
    public String encrypt(Map<String, String> params) {
        try {
            // 1. 使用 TreeMap 依 ASCII 順序排序參數
            TreeMap<String, String> sortedParams = new TreeMap<>(params);

            // 2. 組成 URL encoded 格式（與 PHP http_build_query 等效）
            StringBuilder queryBuilder = new StringBuilder();
            for (Map.Entry<String, String> entry : sortedParams.entrySet()) {
                if (queryBuilder.length() > 0) {
                    queryBuilder.append("&");
                }
                queryBuilder.append(urlEncode(entry.getKey()))
                        .append("=")
                        .append(urlEncode(entry.getValue()));
            }
            String plainText = queryBuilder.toString();

            // AES-256-GCM 加密
            return aesGcmEncrypt(plainText);

        } catch (Exception e) {
            log.error("PayUni 加密失敗", e);
            throw new PayUniCryptoException("加密失敗", e);
        }
    }

    /**
     * 解密回應資料
     *
     * @param encryptedStr 加密字串（EncryptStr）
     * @return 解密後的參數 Map
     */
    public Map<String, String> decrypt(String encryptedStr) {
        try {
            // 1. AES-256-GCM 解密
            String decrypted = aesGcmDecrypt(encryptedStr);

            // 2. 解析 URL encoded 參數
            return parseQueryString(decrypted);

        } catch (Exception e) {
            log.error("PayUni 解密失敗", e);
            throw new PayUniCryptoException("解密失敗", e);
        }
    }

    /**
     * 計算簽章
     *
     * @param encryptStr 加密後的字串
     * @return 簽章（大寫 SHA256）
     */
    public String calculateHash(String encryptStr) {
        try {
            // Hash = SHA256(merKey + encryptStr + merIV).toUpperCase()
            String rawData = merchantKey + encryptStr + merchantIv;

            MessageDigest messageDigest = MessageDigest.getInstance("SHA-256");
            messageDigest.update(rawData.getBytes(StandardCharsets.UTF_8));
            byte[] hashBytes = messageDigest.digest();

            // 轉換為大寫 hex 字串
            StringBuilder hexString = new StringBuilder();
            for (byte b : hashBytes) {
                String hex = Integer.toHexString(0xff & b);
                if (hex.length() == 1) {
                    hexString.append('0');
                }
                hexString.append(hex);
            }
            return hexString.toString().toUpperCase();

        } catch (Exception e) {
            log.error("PayUni 簽章計算失敗", e);
            throw new PayUniCryptoException("簽章計算失敗", e);
        }
    }

    /**
     * 驗證簽章
     *
     * @param encryptStr 加密字串
     * @param hash       預期的簽章
     * @return 驗證是否通過
     */
    public boolean verifyHash(String encryptStr, String hash) {
        String calculated = calculateHash(encryptStr);
        if (hash == null || !hash.matches("(?i)[0-9a-f]{64}")) {
            return false;
        }
        return MessageDigest.isEqual(hexToBytes(calculated), hexToBytes(hash));
    }

    /**
     * AES-256-GCM 加密（完全按照 PayUni 官方範例實作）
     * <p>
     * 輸出格式: hex(base64(ciphertext) + ":::" + base64(tag))
     */
    private String aesGcmEncrypt(String plainText) throws Exception {
        SecretKeySpec secretKeySpec = new SecretKeySpec(
                merchantKey.getBytes(StandardCharsets.UTF_8), "AES");
        GCMParameterSpec gcmParameterSpec = new GCMParameterSpec(
                GCM_TAG_LENGTH_BITS, merchantIv.getBytes(StandardCharsets.UTF_8));

        // 使用標準 Java Provider（不指定 provider）
        Cipher cipher = Cipher.getInstance(AES_GCM_TRANSFORMATION);
        cipher.init(Cipher.ENCRYPT_MODE, secretKeySpec, gcmParameterSpec);

        byte[] encryptedBytes = cipher.doFinal(plainText.getBytes(StandardCharsets.UTF_8));

        // 分離 ciphertext 和 tag（tag 是最後 16 bytes）
        byte[] encryptedInfo = Arrays.copyOfRange(encryptedBytes, 0, encryptedBytes.length - GCM_TAG_LENGTH_BYTES);
        byte[] tagInfo = Arrays.copyOfRange(encryptedBytes, encryptedBytes.length - GCM_TAG_LENGTH_BYTES, encryptedBytes.length);

        // Base64 編碼
        String encodeText = Base64.getEncoder().encodeToString(encryptedInfo);
        String encodeTag = Base64.getEncoder().encodeToString(tagInfo);

        // 組合並轉 hex
        String finalString = encodeText + TAG_SEPARATOR + encodeTag;
        return bytesToHex(finalString.getBytes(StandardCharsets.UTF_8));
    }

    /**
     * AES-256-GCM 解密（完全按照 PayUni 官方範例實作）
     * <p>
     * 輸入格式: hex(base64(ciphertext) + ":::" + base64(tag))
     */
    private String aesGcmDecrypt(String encryptedStr) throws Exception {
        // 使用標準 Java Provider（不指定 provider）
        Cipher cipher = Cipher.getInstance(AES_GCM_TRANSFORMATION);
        SecretKeySpec secretKeySpec = new SecretKeySpec(
                merchantKey.getBytes(StandardCharsets.UTF_8), "AES");
        GCMParameterSpec gcmParameterSpec = new GCMParameterSpec(
                GCM_TAG_LENGTH_BITS, merchantIv.getBytes(StandardCharsets.UTF_8));
        cipher.init(Cipher.DECRYPT_MODE, secretKeySpec, gcmParameterSpec);

        // hex 轉 string
        byte[] hexToByte = hexToBytes(encryptedStr);
        String encryptStr = new String(hexToByte, StandardCharsets.UTF_8);

        // 分離 ciphertext 和 tag
        String[] parts = encryptStr.split(TAG_SEPARATOR);
        if (parts.length != 2) {
            throw new PayUniCryptoException("加密資料格式錯誤：找不到分隔符");
        }

        String encryptInfo = parts[0];
        String tagString = parts[1];

        // Base64 解碼
        byte[] encryptInfoBytes = Base64.getDecoder().decode(encryptInfo);
        byte[] tagStringBytes = Base64.getDecoder().decode(tagString);

        // 組合 ciphertext + tag
        byte[] encryptData = new byte[encryptInfoBytes.length + tagStringBytes.length];
        System.arraycopy(encryptInfoBytes, 0, encryptData, 0, encryptInfoBytes.length);
        System.arraycopy(tagStringBytes, 0, encryptData, encryptInfoBytes.length, tagStringBytes.length);

        // 解密
        byte[] decodeInfo = cipher.doFinal(encryptData);
        return new String(decodeInfo, StandardCharsets.UTF_8);
    }

    /**
     * URL Encode（與 PHP rawurlencode 等效）
     */
    private String urlEncode(String value) {
        if (value == null) {
            return "";
        }
        try {
            return java.net.URLEncoder.encode(value, StandardCharsets.UTF_8)
                    .replace("+", "%20")
                    .replace("*", "%2A")
                    .replace("%7E", "~");
        } catch (Exception e) {
            return value;
        }
    }

    /**
     * 解析 URL Query String
     */
    private Map<String, String> parseQueryString(String queryString) {
        Map<String, String> result = new LinkedHashMap<>();
        if (queryString == null || queryString.isEmpty()) {
            return result;
        }

        String[] pairs = queryString.split("&");
        for (String pair : pairs) {
            int idx = pair.indexOf('=');
            if (idx > 0) {
                String key = urlDecode(pair.substring(0, idx));
                String value = idx < pair.length() - 1 ? urlDecode(pair.substring(idx + 1)) : "";
                result.put(key, value);
            }
        }
        return result;
    }

    /**
     * URL Decode
     */
    private String urlDecode(String value) {
        try {
            return java.net.URLDecoder.decode(value, StandardCharsets.UTF_8);
        } catch (Exception e) {
            return value;
        }
    }

    /**
     * Bytes to Hex String
     */
    private String bytesToHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder();
        for (byte b : bytes) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }

    /**
     * Hex String to Bytes
     */
    private byte[] hexToBytes(String hex) {
        int len = hex.length();
        byte[] data = new byte[len / 2];
        for (int i = 0; i < len; i += 2) {
            data[i / 2] = (byte) ((Character.digit(hex.charAt(i), 16) << 4)
                    + Character.digit(hex.charAt(i + 1), 16));
        }
        return data;
    }
}

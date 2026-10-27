package io.github.will413028.hawkr.media.crypto;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import javax.crypto.Cipher;
import javax.crypto.CipherInputStream;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.SecretKeySpec;
import java.io.InputStream;
import java.security.SecureRandom;
import java.util.Base64;

/**
 * AES-256-GCM 流式加解密工具
 * <p>
 * 設計：
 * <ul>
 *   <li>每個機敏檔案生成獨立的 256-bit 隨機金鑰 + 96-bit IV</li>
 *   <li>金鑰以主密鑰（Master Key）加密後存入 DB（encryption_key 欄位）</li>
 *   <li>檔案使用 AES-GCM 加密後存入 OSS，下載時即時解密串流回傳</li>
 * </ul>
 * <p>
 * 密文大小 = 明文大小 + {@link #GCM_TAG_BYTES}（16 bytes auth tag）
 * <p>
 * encryption_key 格式：
 * <pre>Base64( wrap_iv[12] + AES-GCM(master_key, file_key[32] + file_iv[12]) + tag[16] )</pre>
 */
@Component
public class AesStreamCipher {

    private static final ObsLogger log = ObsLogger.forClass(AesStreamCipher.class);

    private static final String ALGORITHM = "AES";
    private static final String TRANSFORMATION = "AES/GCM/NoPadding";
    private static final int KEY_SIZE_BYTES = 32;   // 256 bits
    private static final int IV_SIZE_BYTES = 12;    // 96 bits（GCM 標準推薦）
    private static final int GCM_TAG_BITS = 128;    // 128-bit auth tag

    /** GCM auth tag 佔用的額外位元組數 */
    public static final int GCM_TAG_BYTES = 16;

    private final SecureRandom secureRandom = new SecureRandom();
    private final SecretKey masterKey;

    public AesStreamCipher(
            @Value("${media.encryption.master-key:}") String masterKeyBase64
    ) {
        if (masterKeyBase64 == null || masterKeyBase64.isEmpty()) {
            log.warn("AesStreamCipher.init.noMasterKey",
                    "未配置加密主密鑰，機敏檔案加解密功能將無法使用");
            this.masterKey = null;
        } else {
            byte[] keyBytes = Base64.getDecoder().decode(masterKeyBase64);
            if (keyBytes.length != KEY_SIZE_BYTES) {
                throw new IllegalArgumentException(
                        "加密主密鑰長度必須為 " + KEY_SIZE_BYTES + " bytes (256-bit)，實際: " + keyBytes.length);
            }
            this.masterKey = new SecretKeySpec(keyBytes, ALGORITHM);
            log.info("AesStreamCipher.init.success", "AES 流式加密初始化成功");
        }
    }

    /**
     * 生成檔案加密金鑰，並以主密鑰加密後回傳 Base64 字串
     * <p>
     * 內部隨機產生 file_key (32 bytes) + file_iv (12 bytes)，
     * 以主密鑰 AES-GCM 加密後，回傳 Base64 編碼結果。
     *
     * @return 加密後的金鑰字串（存入 DB encryption_key 欄位）
     */
    public String generateWrappedKey() {
        ensureMasterKeyAvailable();

        byte[] fileKey = new byte[KEY_SIZE_BYTES];
        byte[] fileIv = new byte[IV_SIZE_BYTES];
        secureRandom.nextBytes(fileKey);
        secureRandom.nextBytes(fileIv);

        // 組合 file_key + file_iv 為待加密資料
        byte[] payload = new byte[KEY_SIZE_BYTES + IV_SIZE_BYTES];
        System.arraycopy(fileKey, 0, payload, 0, KEY_SIZE_BYTES);
        System.arraycopy(fileIv, 0, payload, KEY_SIZE_BYTES, IV_SIZE_BYTES);

        try {
            byte[] wrapIv = new byte[IV_SIZE_BYTES];
            secureRandom.nextBytes(wrapIv);

            Cipher cipher = Cipher.getInstance(TRANSFORMATION);
            cipher.init(Cipher.ENCRYPT_MODE, masterKey, new GCMParameterSpec(GCM_TAG_BITS, wrapIv));
            byte[] encrypted = cipher.doFinal(payload);

            // 組合 wrap_iv + encrypted（含 GCM tag）
            byte[] result = new byte[IV_SIZE_BYTES + encrypted.length];
            System.arraycopy(wrapIv, 0, result, 0, IV_SIZE_BYTES);
            System.arraycopy(encrypted, 0, result, IV_SIZE_BYTES, encrypted.length);

            return Base64.getEncoder().encodeToString(result);
        } catch (Exception e) {
            throw MediaServiceException.internal("生成加密金鑰失敗", e);
        }
    }

    /**
     * 建立加密串流（上傳用）
     * <p>
     * 將明文 InputStream 包裝為 AES-GCM 加密的 CipherInputStream。
     * 讀取此串流會得到密文，大小 = 原始大小 + {@link #GCM_TAG_BYTES}。
     *
     * @param plainInput 明文輸入串流
     * @param wrappedKey 已加密的金鑰字串（由 {@link #generateWrappedKey()} 生成）
     * @return 加密的 CipherInputStream
     */
    public CipherInputStream encryptingStream(InputStream plainInput, String wrappedKey) {
        byte[] unwrapped = unwrapKeyBytes(wrappedKey);
        byte[] fileKey = new byte[KEY_SIZE_BYTES];
        byte[] fileIv = new byte[IV_SIZE_BYTES];
        System.arraycopy(unwrapped, 0, fileKey, 0, KEY_SIZE_BYTES);
        System.arraycopy(unwrapped, KEY_SIZE_BYTES, fileIv, 0, IV_SIZE_BYTES);

        try {
            Cipher cipher = Cipher.getInstance(TRANSFORMATION);
            cipher.init(Cipher.ENCRYPT_MODE,
                    new SecretKeySpec(fileKey, ALGORITHM),
                    new GCMParameterSpec(GCM_TAG_BITS, fileIv));
            return new CipherInputStream(plainInput, cipher);
        } catch (Exception e) {
            throw MediaServiceException.internal("建立加密串流失敗", e);
        }
    }

    /**
     * 建立解密串流（下載/預覽用）
     * <p>
     * 將密文 InputStream 包裝為 AES-GCM 解密的 CipherInputStream。
     * 讀取此串流會得到明文，大小 = 密文大小 - {@link #GCM_TAG_BYTES}。
     * <p>
     * 若密文被篡改，GCM 驗證會在串流結尾拋出例外。
     *
     * @param cipherInput 密文輸入串流（從 OSS 讀取）
     * @param wrappedKey  已加密的金鑰字串
     * @return 解密的 CipherInputStream
     */
    public CipherInputStream decryptingStream(InputStream cipherInput, String wrappedKey) {
        byte[] unwrapped = unwrapKeyBytes(wrappedKey);
        byte[] fileKey = new byte[KEY_SIZE_BYTES];
        byte[] fileIv = new byte[IV_SIZE_BYTES];
        System.arraycopy(unwrapped, 0, fileKey, 0, KEY_SIZE_BYTES);
        System.arraycopy(unwrapped, KEY_SIZE_BYTES, fileIv, 0, IV_SIZE_BYTES);

        try {
            Cipher cipher = Cipher.getInstance(TRANSFORMATION);
            cipher.init(Cipher.DECRYPT_MODE,
                    new SecretKeySpec(fileKey, ALGORITHM),
                    new GCMParameterSpec(GCM_TAG_BITS, fileIv));
            return new CipherInputStream(cipherInput, cipher);
        } catch (Exception e) {
            throw MediaServiceException.internal("建立解密串流失敗", e);
        }
    }

    /**
     * 主密鑰是否已配置
     */
    public boolean isAvailable() {
        return masterKey != null;
    }

    /**
     * 解開已加密的金鑰，回傳 file_key[32] + file_iv[12] 的原始 bytes
     */
    private byte[] unwrapKeyBytes(String wrappedKey) {
        ensureMasterKeyAvailable();

        try {
            byte[] data = Base64.getDecoder().decode(wrappedKey);

            byte[] wrapIv = new byte[IV_SIZE_BYTES];
            byte[] encrypted = new byte[data.length - IV_SIZE_BYTES];
            System.arraycopy(data, 0, wrapIv, 0, IV_SIZE_BYTES);
            System.arraycopy(data, IV_SIZE_BYTES, encrypted, 0, encrypted.length);

            Cipher cipher = Cipher.getInstance(TRANSFORMATION);
            cipher.init(Cipher.DECRYPT_MODE, masterKey, new GCMParameterSpec(GCM_TAG_BITS, wrapIv));
            return cipher.doFinal(encrypted);
        } catch (Exception e) {
            throw MediaServiceException.internal("解密金鑰失敗", e);
        }
    }

    private void ensureMasterKeyAvailable() {
        if (masterKey == null) {
            throw MediaServiceException.internal("加密主密鑰未配置，無法處理機敏檔案");
        }
    }
}

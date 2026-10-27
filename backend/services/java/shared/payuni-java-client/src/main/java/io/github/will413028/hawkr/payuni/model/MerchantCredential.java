package io.github.will413028.hawkr.payuni.model;

/**
 * PayUni 商家金鑰憑證
 *
 * @param hashKey AES-256 金鑰（32 字元）
 * @param hashIv  GCM 初始向量（16 字元）
 */
public record MerchantCredential(String hashKey, String hashIv) {

    public MerchantCredential {
        if (hashKey == null || hashKey.length() != 32) {
            throw new IllegalArgumentException(
                    "hashKey 長度必須為 32 字元，實際 " + (hashKey != null ? hashKey.length() : 0));
        }
        if (hashIv == null || hashIv.length() != 16) {
            throw new IllegalArgumentException(
                    "hashIv 長度必須為 16 字元，實際 " + (hashIv != null ? hashIv.length() : 0));
        }
    }
}

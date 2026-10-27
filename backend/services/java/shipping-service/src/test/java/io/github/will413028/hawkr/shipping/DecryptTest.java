package io.github.will413028.hawkr.shipping;

import io.github.will413028.hawkr.payuni.crypto.PayUniCryptoService;

import java.util.Map;

/**
 * 解密 PayUni EncryptInfo 的快速工具
 * 將 hex 格式的 EncryptInfo 貼到 ENCRYPT_INFO 常數即可
 */
public class DecryptTest {

    private static final String HASH_KEY = "TestOnlyFakeHashKey0123456789AB1";
    private static final String HASH_IV = "TestOnlyFakeIV00";

    // 貼上要解密的 EncryptInfo (hex)
    private static final String ENCRYPT_INFO = "77765a716a6c70356e6e755845773546782b5a547264796d35494c57544f6d59444c6f445345367965373878727a2f726b5a612f4d524d704b502b2f64696c6c4b667666556f6a4a773354644f52666d4572396d577977414d754d465a53562f6c2b6f7449696143596b42675270326c577a37724358566d5069745274796d5942744864454e4e4a396f7265494c5a6449304a677272563941354252416f436e616f44472f553432525a544f492f41625a6f624d496a4e52747a396e4377594a536d6f47534d6e6334664132702f456b566946644176494a7a51477831413d3d3a3a3a33524b582b6d47416c31465a4b7839655062367435513d3d";

    public static void main(String[] args) {
        PayUniCryptoService cryptoService = new PayUniCryptoService(HASH_KEY, HASH_IV);
        try {
            Map<String, String> decrypted = cryptoService.decrypt(ENCRYPT_INFO);
            System.out.println("===== 解密結果 =====");
            decrypted.forEach((k, v) -> System.out.println("  " + k + " = " + v));
        } catch (Exception e) {
            System.err.println("解密失敗: " + e.getMessage());
            e.printStackTrace();
        }
    }
}

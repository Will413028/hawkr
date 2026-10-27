package io.github.will413028.hawkr.order.util;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;

/**
 * order-service 共用雜湊工具（純函式、無狀態）。
 */
public final class HashUtil {

    private HashUtil() {
    }

    /**
     * 計算輸入字串的 SHA-256 十六進位摘要。
     * 演算法不可用時回空字串（保留原 inline 實作的容錯語意）。
     */
    public static String sha256Hex(String input) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(digest.digest(input.getBytes(StandardCharsets.UTF_8)));
        } catch (NoSuchAlgorithmException e) {
            return "";
        }
    }
}

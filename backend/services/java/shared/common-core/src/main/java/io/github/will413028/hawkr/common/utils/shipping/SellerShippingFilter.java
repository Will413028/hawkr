package io.github.will413028.hawkr.common.utils.shipping;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Collection;
import java.util.Set;
import java.util.TreeSet;

/**
 * 賣家當下已開通配送方式的過濾與指紋計算工具。
 *
 * <p>背景：賣家 `seller_shipment_settings.enabled` 為靜默單寫，
 * 不發 MQ event、不主動清下游 cache（06 §3.4 / docs/開發規範/後端服務責任邊界規範.md）。
 * 公開商品 read path（02 §2.7.2）與結帳 read path（03 §4.3）均需在 return 前
 * 呼叫 shipping-service `getSellerShippingConfig` 取當下狀態，
 * 配合本 utility 取交集剔除 dangling shipping options。
 *
 * <p>本 class 不依賴 proto type，僅接受純字串 set；
 * 各 service 自行將 proto 結果轉成 `availableMethodKeys` 後傳入。
 */
public final class SellerShippingFilter {

    private SellerShippingFilter() {}

    /**
     * 把 (shippingMethod, goodsType) 組成統一字串 key。
     * goodsType 為 null 視同空字串（宅配無 goodsType）。
     */
    public static String toMethodKey(String shippingMethod, String goodsType) {
        return shippingMethod + ":" + (goodsType == null ? "" : goodsType);
    }

    /**
     * 判斷 (shippingMethod, temperature) 組合是否在賣家當下已啟用的配送方式內。
     *
     * <p>匹配規則：
     * <ul>
     *   <li>先比 (shippingMethod, temperature) — 對應超商 goodsType=temperature</li>
     *   <li>再比 (shippingMethod, "") — 對應宅配 goodsType 為空</li>
     * </ul>
     *
     * <p>fail-close：若 {@code availableMethodKeys} 為空（賣家完全沒可用方式），一律回 {@code false}。
     * 對應 03 §4.3 「不再因 if (!sellerAvailableMethods.isEmpty()) guard 而 fail-open」。
     */
    public static boolean isAvailable(String shippingMethod, String temperature, Set<String> availableMethodKeys) {
        if (availableMethodKeys == null || availableMethodKeys.isEmpty()) {
            return false;
        }
        String fullKey = toMethodKey(shippingMethod, temperature);
        String noTempKey = toMethodKey(shippingMethod, "");
        return availableMethodKeys.contains(fullKey) || availableMethodKeys.contains(noTempKey);
    }

    /**
     * 計算賣家 onboarding 指紋，用於 cache key suffix。
     * 同一 seller toggle 後指紋變動 → 舊 cache key 自然 miss → TTL 過期清除。
     *
     * <p>格式：SHA1(sellerAccountId + ":" + sortedKeys.join(",")) 前 8 字元（小寫 hex）
     */
    public static String onboardingSig(long sellerAccountId, Collection<String> availableMethodKeys) {
        TreeSet<String> sorted = availableMethodKeys == null
                ? new TreeSet<>()
                : new TreeSet<>(availableMethodKeys);
        String input = sellerAccountId + ":" + String.join(",", sorted);
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-1");
            byte[] hash = md.digest(input.getBytes(StandardCharsets.UTF_8));
            StringBuilder sb = new StringBuilder(8);
            for (int i = 0; i < 4; i++) {
                sb.append(String.format("%02x", hash[i]));
            }
            return sb.toString();
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-1 不可用", e);
        }
    }

    /**
     * 把多個 seller 的 onboarding 指紋再聚合成 list 場景用的指紋。
     * 用於商品列表 cache key — 列表中任一 seller toggle 即 invalidate。
     */
    public static String listOnboardingSig(Collection<String> perSellerSigs) {
        TreeSet<String> sorted = perSellerSigs == null
                ? new TreeSet<>()
                : new TreeSet<>(perSellerSigs);
        try {
            MessageDigest md = MessageDigest.getInstance("SHA-1");
            byte[] hash = md.digest(String.join(",", sorted).getBytes(StandardCharsets.UTF_8));
            StringBuilder sb = new StringBuilder(8);
            for (int i = 0; i < 4; i++) {
                sb.append(String.format("%02x", hash[i]));
            }
            return sb.toString();
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-1 不可用", e);
        }
    }
}

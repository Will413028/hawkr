package io.github.will413028.hawkr.common.utils.shipping;

import io.github.will413028.hawkr.common.utils.code.ShippingArea;
import io.github.will413028.hawkr.common.utils.code.ShippingRoute;
import io.github.will413028.hawkr.common.utils.code.helper.ShippingRouteRules;

import java.util.Set;

/**
 * 根據三碼郵遞區號判斷出貨地區（本島 / 離島）
 * <p>
 * 離島定義（配送費率差異區域）：
 * - 澎湖縣：880-885
 * - 金門縣：890-896
 * - 連江縣（馬祖）：209-212
 */
public final class ShippingAreaResolver {

    /** 離島郵遞區號前三碼集合 */
    private static final Set<String> OFFSHORE_ZIP_CODES = Set.of(
            // 澎湖縣
            "880", "881", "882", "883", "884", "885",
            // 金門縣
            "890", "891", "892", "893", "894", "895", "896",
            // 連江縣（馬祖）
            "209", "210", "211", "212",
            // 台東縣離島
            "951",  // 綠島
            "952",  // 蘭嶼
            // 屏東縣離島
            "929"   // 小琉球
    );

    private ShippingAreaResolver() {
    }

    /**
     * 根據三碼郵遞區號判斷 ShippingArea
     *
     * @param zipCode 三碼郵遞區號，null 或空值視為本島
     * @return MAIN_ISLAND 或 OFFSHORE_ISLAND
     */
    public static ShippingArea resolve(String zipCode) {
        if (zipCode == null || zipCode.isBlank()) {
            return ShippingArea.MAIN_ISLAND;
        }
        String trimmed = zipCode.trim();
        // 支援五碼郵遞區號（取前三碼）
        if (trimmed.length() > 3) {
            trimmed = trimmed.substring(0, 3);
        }
        return OFFSHORE_ZIP_CODES.contains(trimmed)
                ? ShippingArea.OFFSHORE_ISLAND
                : ShippingArea.MAIN_ISLAND;
    }

    /**
     * 根據寄件人郵遞區號 + 收件人郵遞區號，決定運費路線
     *
     * @param senderZipCode   寄件人（賣家）郵遞區號
     * @param receiverZipCode 收件人（買家）郵遞區號
     * @return ShippingRoute（DOMESTIC / TO_OFFSHORE / FROM_OFFSHORE / OFFSHORE_TO_OFFSHORE）
     */
    public static ShippingRoute resolveRoute(String senderZipCode, String receiverZipCode) {
        ShippingArea senderArea = resolve(senderZipCode);
        ShippingArea receiverArea = resolve(receiverZipCode);
        return ShippingRouteRules.resolve(senderArea, receiverArea);
    }
}

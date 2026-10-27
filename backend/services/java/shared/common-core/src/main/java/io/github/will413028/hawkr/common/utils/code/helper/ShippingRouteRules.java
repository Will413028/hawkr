package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.ShippingArea;
import io.github.will413028.hawkr.common.utils.code.ShippingRoute;

/**
 * §8.5 ShippingRoute 路線解析 helper（跨 product / order / common-core 共用）。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 將原本寫在 ShippingRoute enum 內的 `resolve(area, area)` 移到此處。
 * 跨服務共用，故放在 common-core `utils/code/helper/` 子套件。
 */
public final class ShippingRouteRules {

    private ShippingRouteRules() {
    }

    /**
     * 根據寄件區域 + 收件區域，決定運費路線。
     */
    public static ShippingRoute resolve(ShippingArea senderArea, ShippingArea receiverArea) {
        if (senderArea == ShippingArea.MAIN_ISLAND && receiverArea == ShippingArea.MAIN_ISLAND) {
            return ShippingRoute.DOMESTIC;
        } else if (senderArea == ShippingArea.MAIN_ISLAND && receiverArea == ShippingArea.OFFSHORE_ISLAND) {
            return ShippingRoute.TO_OFFSHORE;
        } else if (senderArea == ShippingArea.OFFSHORE_ISLAND && receiverArea == ShippingArea.MAIN_ISLAND) {
            return ShippingRoute.FROM_OFFSHORE;
        } else {
            return ShippingRoute.OFFSHORE_TO_OFFSHORE;
        }
    }
}

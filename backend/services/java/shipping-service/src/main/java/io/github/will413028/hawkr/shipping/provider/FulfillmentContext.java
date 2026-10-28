package io.github.will413028.hawkr.shipping.provider;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

/**
 * capability 解析的上下文。
 *
 * <p>🔴 <b>為什麼 capability 的 getter 必須帶 context，而不是無參數</b>：
 * capability 集合目前對 {@code 3001}／{@code 3002}／{@code 3003} 是 per-method 常數，
 * 但同一個 provider 的能力集合可能隨店家或配送單而異。
 * 無參數的 getter 表達不出這件事，而表達不出的後果是日後只能在 service 層再加一組 if。
 *
 * <p>⚠️ 因此 <b>回傳值不得快取成 per-method 單例</b>。adapter 實作若要快取，
 * 只能以 (method, storeId) 為鍵，且要能被賣家設定變更失效。
 *
 * <p>{@code shipment} 可為 null：{@code onboard()} 與 {@code storeLocator()} 的解析發生在
 * 還沒有任何配送單的時候（建店、買家選門市）。要求呼叫端硬湊一個假 shipment 只會讓
 * 「這個欄位是真的還是湊的」變成下一個人的問題。
 */
public record FulfillmentContext(
        long storeId,
        ShippingMethod method,
        ShipmentEntity shipment
) {
    public FulfillmentContext {
        if (method == null) {
            throw new IllegalArgumentException("FulfillmentContext.method 不得為 null——"
                    + "capability 解析的第一層分派就是配送方式，null 會讓 registry 落到「查無 adapter」而非「不支援」");
        }
    }

    /** 建店／門市地圖等「尚無配送單」的情境。 */
    public static FulfillmentContext forStore(long storeId, ShippingMethod method) {
        return new FulfillmentContext(storeId, method, null);
    }

    /** 已有配送單的情境（lifecycle 絕大多數路徑）。 */
    public static FulfillmentContext forShipment(ShipmentEntity shipment) {
        if (shipment == null) {
            throw new IllegalArgumentException("forShipment(null)：請改用 forStore");
        }
        return new FulfillmentContext(
                shipment.getStoreId() == null ? 0L : shipment.getStoreId(),
                ShippingMethod.fromValue(shipment.getShippingMethod()),
                shipment);
    }

    /** 這一筆是不是逆向 leg。leg 是維度不是狀態，capability 對正逆向可以不同。 */
    public boolean isReturnLeg() {
        return shipment != null && Boolean.TRUE.equals(shipment.getIsReturn());
    }
}

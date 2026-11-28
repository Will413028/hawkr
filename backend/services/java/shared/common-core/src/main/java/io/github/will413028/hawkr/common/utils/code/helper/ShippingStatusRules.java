package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.ShippingStatus;

/**
 * §8.5 ShippingStatus 業務狀態規則 helper（跨 shipping + order 服務共用）。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/dev-guidelines/java-microservices-development-guidelines.md §6）。
 * 本 helper 將原本寫在 enum 內的 4 個 predicate 移到此處。放在 common-core
 * `utils/code/helper/` 子套件而非單一服務內，是因為 shipping-service 與
 * order-service 都需要同一份 predicate 邏輯，避免複製造成不一致。
 * 此 helper 為純函式，不引入任何外部依賴。
 */
public final class ShippingStatusRules {

    private ShippingStatusRules() {
    }

    /** 是否為終態（不可再轉移） */
    public static boolean isTerminal(ShippingStatus status) {
        return status == ShippingStatus.PICKED_UP || status == ShippingStatus.CANCELLED;
    }

    /** 是否為異常相關狀態（觸發訂單 SHIPPING_ABNORMAL） */
    public static boolean isAbnormal(ShippingStatus status) {
        return status == ShippingStatus.ABNORMAL
                || status == ShippingStatus.COMPENSATION
                || status == ShippingStatus.PACKAGE_LOST
                || status == ShippingStatus.PACKAGE_ABANDONED
                || status == ShippingStatus.RETURN_GENERAL
                || status == ShippingStatus.BUYER_NOT_PICKED
                || status == ShippingStatus.RETURN_TO_WAREHOUSE
                || status == ShippingStatus.SELLER_NOT_PICKED
                || status == ShippingStatus.RETURN_BY_HOME_DELIVERY
                || status == ShippingStatus.STORE_TRANSFER
                || status == ShippingStatus.PENDING_HOME_RETURN
                || status == ShippingStatus.CLAIM_PENDING;
    }

    /** 是否推進訂單狀態到待出貨(20) */
    public static boolean isPendingShipment(ShippingStatus status) {
        return status == ShippingStatus.UNPROCESSED
                || status == ShippingStatus.PROCESSING
                || status == ShippingStatus.PENDING_SHIPMENT
                || status == ShippingStatus.CVS_CODE_RECEIVED
                || status == ShippingStatus.COURIER_ASSIGNED;
    }

    /** 是否推進訂單狀態到配送中(30)：宅配叫件 / 超商賣家確認出貨 */
    public static boolean isConfirmedShipment(ShippingStatus status) {
        return status == ShippingStatus.TCAT_CALLED
                || status == ShippingStatus.SELLER_CONFIRMED
                || status == ShippingStatus.HANDED_OVER;
    }
}

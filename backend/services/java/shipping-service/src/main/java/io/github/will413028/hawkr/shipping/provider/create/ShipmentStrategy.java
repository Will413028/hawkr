package io.github.will413028.hawkr.shipping.provider.create;

import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

/**
 * 物流單建立策略介面
 *
 * MQ 驅動自動建單：order.paid → 背景呼叫 PayUni API 建立物流單
 * 寄件人資訊來自 seller_shipment_settings（各配送方式獨立設定）
 * merId 來自 OrderShippingInfo（訂單建立時已快照）
 */
public interface ShipmentStrategy {

    /**
     * 背景自動建立物流單（呼叫第三方 API + 組裝 Entity）
     * Entity 尚未持久化，由 ShippingService 統一 insert
     *
     * @param orderNo            訂單編號
     * @param orderInfo          訂單資訊（含 merId、收件人、金額等）
     * @param onboardingSetting  建店設定（含寄件人資訊；Uber 即時配送可為 null）
     * @return 已組裝的 ShipmentEntity（未持久化）
     */
    ShipmentEntity createShipmentFromOrder(String orderNo,
                                            OrderShippingInfo orderInfo,
                                            SellerShipmentSettingEntity onboardingSetting);

    /**
     * 取得配送方式（ShippingMethod.value()）
     */
    String getShippingMethod();
}

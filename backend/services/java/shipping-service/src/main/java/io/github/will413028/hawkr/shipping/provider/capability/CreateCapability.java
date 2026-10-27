package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

/** 建單能力。 */
public interface CreateCapability {

    /**
     * 呼叫 provider 建單並組裝 Entity（<b>尚未持久化</b>，由呼叫端統一 insert）。
     *
     * @param onboardingSetting 建店設定；provider 不需要建店時（見 {@link #requiresOnboarding()}）可為 null
     */
    ShipmentEntity createFromOrder(String orderNo,
                                   OrderShippingInfo orderInfo,
                                   SellerShipmentSettingEntity onboardingSetting);

    /**
     * 建單前是否必須先完成建店。
     *
     * <p>🔴 這一格吸收的是 {@code ShippingService.createShipmentFromOrder} 內
     * 「{@code !UBER_INSTANT} → 必須已建店」的字面分支——那是 provider 的性質，不是流程的性質。
     */
    boolean requiresOnboarding();
}

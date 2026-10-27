package io.github.will413028.hawkr.shipping.provider.create;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import org.springframework.stereotype.Component;

import java.time.LocalDate;
import java.time.format.DateTimeFormatter;

/**
 * Uber 即時配送策略（Phase 1: 僅建立內部物流單，不呼叫 Uber API）
 *
 * Uber 不需要建店，onboardingSetting 可為 null。
 * 寄件人資訊在 Phase 1 暫不填（Uber 有自己的取件流程）。
 */
@Component
public class UberShipmentStrategy implements ShipmentStrategy {

    private static final ObsLogger log = ObsLogger.forClass(UberShipmentStrategy.class);
    private static final DateTimeFormatter DATE_FMT = DateTimeFormatter.ofPattern("yyyyMMdd");

    @Override
    public ShipmentEntity createShipmentFromOrder(String orderNo,
                                                   OrderShippingInfo orderInfo,
                                                   SellerShipmentSettingEntity onboardingSetting) {
        Long shipmentId = TsidUtil.generate();
        String shipmentNo = generateShipmentNo();

        log.info("UberShipmentStrategy.createShipment",
                "Uber 即時配送建單（Phase 1 預留），shipmentNo=" + shipmentNo);

        String receiverAddress = orderInfo.getShippingAddress();

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(shipmentId);
        entity.setShipmentNo(shipmentNo);
        entity.setOrderNo(orderNo);
        entity.setStoreId(ProtoConversionUtils.parseLongRequired(orderInfo.getStoreId(), "store_id"));
        entity.setBuyerId(ProtoConversionUtils.parseLongRequired(orderInfo.getBuyerId(), "buyerId"));
        entity.setShippingMethod(ShippingMethod.UBER_INSTANT.value());
        entity.setStatus(ShippingStatus.UNPROCESSED.value());
        entity.setTradeAmt(ProtoConversionUtils.parseLongRequired(orderInfo.getTotalActualAmount(), "totalActualAmount"));
        // Uber Phase 1: 寄件人資訊暫不填（Uber 有自己的取件流程）
        entity.setSenderName("");
        entity.setSenderPhone("");
        entity.setRecipientName(orderInfo.getRecipientName());
        entity.setRecipientPhone(orderInfo.getRecipientPhone());
        entity.setRecipientAddress(receiverAddress);
        entity.setProdDesc(orderInfo.getProductName());

        return entity;
    }

    @Override
    public String getShippingMethod() {
        return ShippingMethod.UBER_INSTANT.value();
    }

    private String generateShipmentNo() {
        long seq = TsidUtil.generate() % 1_000_000_000L;
        return "S" + String.format("%09d", seq);
    }
}

package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a shipment is created
 * Consumers: order-service (update status), im-service (notify buyer), notification-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class ShipmentCreatedEvent extends BaseEvent {

    private Long shipmentId;
    private Long orderId;
    private String orderSn;
    private Long memberId;
    private Long sellerAccountId;
    private String carrier;  // 7-11, TCAT
    private String trackingNumber;
    private String shipTradeNo;

    public static ShipmentCreatedEvent create(Long shipmentId, Long orderId, String orderSn,
                                               Long memberId, Long sellerAccountId, String carrier,
                                               String trackingNumber, String shipTradeNo) {
        ShipmentCreatedEvent event = ShipmentCreatedEvent.builder()
                .shipmentId(shipmentId)
                .orderId(orderId)
                .orderSn(orderSn)
                .memberId(memberId)
                .sellerAccountId(sellerAccountId)
                .carrier(carrier)
                .trackingNumber(trackingNumber)
                .shipTradeNo(shipTradeNo)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_SHIPMENT_CREATED, "shipping-service");
        return event;
    }
}

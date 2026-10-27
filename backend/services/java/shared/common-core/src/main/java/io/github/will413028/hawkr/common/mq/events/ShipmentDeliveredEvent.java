package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

import java.time.Instant;

/**
 * Event published when a shipment is delivered
 * Consumers: order-service (mark completed), marketing-service (award points), notification-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class ShipmentDeliveredEvent extends BaseEvent {

    private Long shipmentId;
    private Long orderId;
    private String orderSn;
    private Long memberId;
    private Long sellerAccountId;
    private String carrier;
    private String trackingNumber;
    private Instant deliveredAt;
    private String signedBy;

    public static ShipmentDeliveredEvent create(Long shipmentId, Long orderId, String orderSn,
                                                 Long memberId, Long sellerAccountId, String carrier,
                                                 String trackingNumber, String signedBy) {
        ShipmentDeliveredEvent event = ShipmentDeliveredEvent.builder()
                .shipmentId(shipmentId)
                .orderId(orderId)
                .orderSn(orderSn)
                .memberId(memberId)
                .sellerAccountId(sellerAccountId)
                .carrier(carrier)
                .trackingNumber(trackingNumber)
                .deliveredAt(Instant.now())
                .signedBy(signedBy)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_SHIPMENT_DELIVERED, "shipping-service");
        return event;
    }
}

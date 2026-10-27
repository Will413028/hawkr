package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;


/**
 * Event published to request PAYUNi seller status check
 * seller-service -> payment-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class SellerPayuniStatusCheckEvent extends BaseEvent {

    private Long sellerAccountId;
    private String payuniMerId;  // PAYUNi MerID (e.g., HAWK0001)

    /**
     * Create a new SELLER_PAYUNI_STATUS_CHECK event
     */
    public static SellerPayuniStatusCheckEvent create(Long sellerAccountId, String payuniMerId) {
        SellerPayuniStatusCheckEvent event = SellerPayuniStatusCheckEvent.builder()
                .sellerAccountId(sellerAccountId)
                .payuniMerId(payuniMerId)
                .build();

        event.initializeEvent(MqConstants.EVENT_TYPE_SELLER_PAYUNI_STATUS_CHECK, "seller-service");

        return event;
    }
}

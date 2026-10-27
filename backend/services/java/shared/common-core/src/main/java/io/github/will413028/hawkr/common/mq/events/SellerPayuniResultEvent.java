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
 * Event published when PAYUNi registration completes (success or failure)
 * payment-service -> seller-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class SellerPayuniResultEvent extends BaseEvent {

    private Long sellerAccountId;
    private String payuniMerId;         // Generated MerID (e.g., HAWK0001)
    private Boolean success;
    private String errorMessage;
    private Instant registeredAt;

    /**
     * Create a SUCCESS result event
     */
    public static SellerPayuniResultEvent success(Long sellerAccountId, String payuniMerId) {
        SellerPayuniResultEvent event = SellerPayuniResultEvent.builder()
                .sellerAccountId(sellerAccountId)
                .payuniMerId(payuniMerId)
                .success(true)
                .registeredAt(Instant.now())
                .build();

        event.initializeEvent(MqConstants.EVENT_TYPE_SELLER_PAYUNI_REGISTERED, "payment-service");

        return event;
    }

    /**
     * Create a FAILURE result event
     */
    public static SellerPayuniResultEvent failure(Long sellerAccountId, String errorMessage) {
        SellerPayuniResultEvent event = SellerPayuniResultEvent.builder()
                .sellerAccountId(sellerAccountId)
                .success(false)
                .errorMessage(errorMessage)
                .build();

        event.initializeEvent(MqConstants.EVENT_TYPE_SELLER_PAYUNI_FAILED, "payment-service");

        return event;
    }
}

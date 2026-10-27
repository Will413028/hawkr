package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a product's status changes (publish/unpublish).
 * Consumers: cart-service (handle unavailable products).
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class ProductStatusChangedEvent extends BaseEvent {

    private Long productId;
    private Long sellerAccountId;
    private Integer oldStatus;  // 0=draft, 1=published, 2=delisted
    private Integer newStatus;
    private String reason;  // e.g., "out of stock", "seller request"

    public static ProductStatusChangedEvent create(Long productId, Long sellerAccountId,
                                                    Integer oldStatus, Integer newStatus, String reason) {
        ProductStatusChangedEvent event = ProductStatusChangedEvent.builder()
                .productId(productId)
                .sellerAccountId(sellerAccountId)
                .oldStatus(oldStatus)
                .newStatus(newStatus)
                .reason(reason)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_PRODUCT_STATUS_CHANGED, "product-service");
        return event;
    }

    public boolean isPublished() {
        return newStatus != null && newStatus == 1;
    }

    public boolean isDelisted() {
        return newStatus != null && newStatus == 2;
    }
}

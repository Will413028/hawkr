package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

import java.math.BigDecimal;

/**
 * Event published when a product is updated.
 * Consumers: cart-service (update cached prices).
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class ProductUpdatedEvent extends BaseEvent {

    private Long productId;
    private Long sellerAccountId;
    private String productName;
    private Long categoryId;
    private BigDecimal price;
    private BigDecimal originalPrice;

    public static ProductUpdatedEvent create(Long productId, Long sellerAccountId, String productName,
                                              Long categoryId, BigDecimal price, BigDecimal originalPrice) {
        ProductUpdatedEvent event = ProductUpdatedEvent.builder()
                .productId(productId)
                .sellerAccountId(sellerAccountId)
                .productName(productName)
                .categoryId(categoryId)
                .price(price)
                .originalPrice(originalPrice)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_PRODUCT_UPDATED, "product-service");
        return event;
    }
}

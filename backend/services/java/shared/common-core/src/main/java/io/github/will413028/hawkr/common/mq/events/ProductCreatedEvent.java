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
 * Event published when a new product is created.
 * Consumers: (none currently — search-service was retired in 2026-06;
 *  search is now served by product-service)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class ProductCreatedEvent extends BaseEvent {

    private Long productId;
    private Long sellerAccountId;
    private String productName;
    private Long categoryId;
    private BigDecimal price;
    private Integer status;  // 0=draft, 1=published

    public static ProductCreatedEvent create(Long productId, Long sellerAccountId, String productName,
                                              Long categoryId, BigDecimal price, Integer status) {
        ProductCreatedEvent event = ProductCreatedEvent.builder()
                .productId(productId)
                .sellerAccountId(sellerAccountId)
                .productName(productName)
                .categoryId(categoryId)
                .price(price)
                .status(status)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_PRODUCT_CREATED, "product-service");
        return event;
    }
}

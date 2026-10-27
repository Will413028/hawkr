package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a review is posted
 * Consumers: product-service (update rating), notification-service (notify seller)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class ReviewPostedEvent extends BaseEvent {

    private Long reviewId;
    private Long productId;
    private Long orderId;
    private Long memberId;
    private Long sellerAccountId;
    private Integer rating;  // 1-5 stars
    private String content;
    private Boolean hasImages;

    public static ReviewPostedEvent create(Long reviewId, Long productId, Long orderId,
                                            Long memberId, Long sellerAccountId, Integer rating,
                                            String content, Boolean hasImages) {
        ReviewPostedEvent event = ReviewPostedEvent.builder()
                .reviewId(reviewId)
                .productId(productId)
                .orderId(orderId)
                .memberId(memberId)
                .sellerAccountId(sellerAccountId)
                .rating(rating)
                .content(content)
                .hasImages(hasImages)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_REVIEW_POSTED, "content-service");
        return event;
    }
}

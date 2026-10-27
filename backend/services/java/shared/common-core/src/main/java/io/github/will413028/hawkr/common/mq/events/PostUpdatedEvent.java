package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

import java.util.List;

/**
 * Event published when a post is updated.
 * Consumers: (none currently — search-service was retired in 2026-06;
 *  search is now served by product-service)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class PostUpdatedEvent extends BaseEvent {

    private Long postId;
    private Long authorId;
    private String content;
    private List<String> imageUrls;
    private List<Long> productIds;

    public static PostUpdatedEvent create(Long postId, Long authorId, String content,
                                           List<String> imageUrls, List<Long> productIds) {
        PostUpdatedEvent event = PostUpdatedEvent.builder()
                .postId(postId)
                .authorId(authorId)
                .content(content)
                .imageUrls(imageUrls)
                .productIds(productIds)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_POST_UPDATED, "content-service");
        return event;
    }
}

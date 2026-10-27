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
 * Event published when a post is created.
 * Consumers: notification-service (notify followers).
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class PostCreatedEvent extends BaseEvent {

    private Long postId;
    private Long authorId;
    private String authorName;
    private String content;
    private List<String> imageUrls;
    private List<Long> productIds;  // Tagged products
    private Integer visibility;  // 0=public, 1=followers only

    public static PostCreatedEvent create(Long postId, Long authorId, String authorName,
                                           String content, List<String> imageUrls,
                                           List<Long> productIds, Integer visibility) {
        PostCreatedEvent event = PostCreatedEvent.builder()
                .postId(postId)
                .authorId(authorId)
                .authorName(authorName)
                .content(content)
                .imageUrls(imageUrls)
                .productIds(productIds)
                .visibility(visibility)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_POST_CREATED, "content-service");
        return event;
    }
}

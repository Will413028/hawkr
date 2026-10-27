package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a post is deleted.
 * Consumers: media-service (cleanup images).
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class PostDeletedEvent extends BaseEvent {

    private Long postId;
    private Long authorId;
    private String reason;  // user_request, moderation, account_deleted

    public static PostDeletedEvent create(Long postId, Long authorId, String reason) {
        PostDeletedEvent event = PostDeletedEvent.builder()
                .postId(postId)
                .authorId(authorId)
                .reason(reason)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_POST_DELETED, "content-service");
        return event;
    }
}

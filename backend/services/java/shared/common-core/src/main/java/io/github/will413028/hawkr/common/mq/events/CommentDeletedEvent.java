package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a comment is deleted
 * Consumers: notification-service (update notification status)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class CommentDeletedEvent extends BaseEvent {

    private Long commentId;
    private Long postId;
    private Long deleterId;  // Who deleted (author or moderator)
    private String reason;  // user_request, moderation

    public static CommentDeletedEvent create(Long commentId, Long postId, Long deleterId, String reason) {
        CommentDeletedEvent event = CommentDeletedEvent.builder()
                .commentId(commentId)
                .postId(postId)
                .deleterId(deleterId)
                .reason(reason)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_COMMENT_DELETED, "content-service");
        return event;
    }
}

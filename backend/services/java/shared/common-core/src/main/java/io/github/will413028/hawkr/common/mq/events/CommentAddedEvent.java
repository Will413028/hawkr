package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a comment is added to a post
 * Consumers: notification-service (notify post author), im-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class CommentAddedEvent extends BaseEvent {

    private Long commentId;
    private Long postId;
    private Long postAuthorId;
    private Long commenterId;
    private String commenterName;
    private String content;
    private Long parentCommentId;  // null if top-level comment
    private Long replyToUserId;  // null if not replying to someone

    public static CommentAddedEvent create(Long commentId, Long postId, Long postAuthorId,
                                            Long commenterId, String commenterName, String content,
                                            Long parentCommentId, Long replyToUserId) {
        CommentAddedEvent event = CommentAddedEvent.builder()
                .commentId(commentId)
                .postId(postId)
                .postAuthorId(postAuthorId)
                .commenterId(commenterId)
                .commenterName(commenterName)
                .content(content)
                .parentCommentId(parentCommentId)
                .replyToUserId(replyToUserId)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_COMMENT_ADDED, "content-service");
        return event;
    }
}

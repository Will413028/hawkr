package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a user removes a like
 * Consumers: (internal analytics)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class LikeRemovedEvent extends BaseEvent {

    private Long likeId;
    private Long likerId;
    private String targetType;  // POST, COMMENT
    private Long targetId;

    public static LikeRemovedEvent create(Long likeId, Long likerId, String targetType, Long targetId) {
        LikeRemovedEvent event = LikeRemovedEvent.builder()
                .likeId(likeId)
                .likerId(likerId)
                .targetType(targetType)
                .targetId(targetId)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_LIKE_REMOVED, "content-service");
        return event;
    }
}

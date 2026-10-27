package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a user unfollows another user/seller
 * Consumers: (internal analytics)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class FollowRemovedEvent extends BaseEvent {

    private Long followId;
    private Long followerId;
    private Long followeeId;
    private String followeeType;  // USER, SELLER

    public static FollowRemovedEvent create(Long followId, Long followerId, Long followeeId, String followeeType) {
        FollowRemovedEvent event = FollowRemovedEvent.builder()
                .followId(followId)
                .followerId(followerId)
                .followeeId(followeeId)
                .followeeType(followeeType)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_FOLLOW_REMOVED, "content-service");
        return event;
    }
}

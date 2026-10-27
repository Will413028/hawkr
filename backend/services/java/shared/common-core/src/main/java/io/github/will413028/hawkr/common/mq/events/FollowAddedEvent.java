package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a user follows another user/seller
 * Consumers: notification-service (notify followee), im-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class FollowAddedEvent extends BaseEvent {

    private Long followId;
    private Long followerId;
    private String followerName;
    private Long followeeId;
    private String followeeType;  // USER, SELLER

    public static FollowAddedEvent create(Long followId, Long followerId, String followerName,
                                           Long followeeId, String followeeType) {
        FollowAddedEvent event = FollowAddedEvent.builder()
                .followId(followId)
                .followerId(followerId)
                .followerName(followerName)
                .followeeId(followeeId)
                .followeeType(followeeType)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_FOLLOW_ADDED, "content-service");
        return event;
    }
}

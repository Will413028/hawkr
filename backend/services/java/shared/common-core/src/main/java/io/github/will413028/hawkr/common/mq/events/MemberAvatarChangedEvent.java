package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a member changes their avatar
 * Consumers: content-service, im-service (update cached avatar)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class MemberAvatarChangedEvent extends BaseEvent {

    private Long memberId;
    private String oldAvatarUrl;
    private String newAvatarUrl;

    public static MemberAvatarChangedEvent create(Long memberId, String oldAvatarUrl, String newAvatarUrl) {
        MemberAvatarChangedEvent event = MemberAvatarChangedEvent.builder()
                .memberId(memberId)
                .oldAvatarUrl(oldAvatarUrl)
                .newAvatarUrl(newAvatarUrl)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_MEMBER_AVATAR_CHANGED, "buyer-service");
        return event;
    }
}

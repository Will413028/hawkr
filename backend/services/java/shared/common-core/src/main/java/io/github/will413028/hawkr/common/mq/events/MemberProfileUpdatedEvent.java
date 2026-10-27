package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a member's profile is updated
 * Consumers: notification-service, im-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class MemberProfileUpdatedEvent extends BaseEvent {

    private Long memberId;
    private String nickname;
    private String mobile;
    private String email;
    private Integer gender;

    public static MemberProfileUpdatedEvent create(Long memberId, String nickname,
                                                    String mobile, String email, Integer gender) {
        MemberProfileUpdatedEvent event = MemberProfileUpdatedEvent.builder()
                .memberId(memberId)
                .nickname(nickname)
                .mobile(mobile)
                .email(email)
                .gender(gender)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_MEMBER_PROFILE_UPDATED, "buyer-service");
        return event;
    }
}

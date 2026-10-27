package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a member changes their password
 * Consumers: notification-service (send security alert)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class MemberPasswordChangedEvent extends BaseEvent {

    private Long memberId;
    private String email;
    private String mobile;

    public static MemberPasswordChangedEvent create(Long memberId, String email, String mobile) {
        MemberPasswordChangedEvent event = MemberPasswordChangedEvent.builder()
                .memberId(memberId)
                .email(email)
                .mobile(mobile)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_MEMBER_PASSWORD_CHANGED, "account-service");
        return event;
    }
}

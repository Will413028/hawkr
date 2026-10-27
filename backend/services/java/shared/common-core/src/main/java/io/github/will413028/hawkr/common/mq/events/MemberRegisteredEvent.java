package io.github.will413028.hawkr.common.mq.events;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * User registration event
 * Triggered when a new user registers
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class MemberRegisteredEvent implements Serializable {

    private static final long serialVersionUID = 1L;

    /**
     * Event ID (unique identifier for this event)
     */
    private String eventId;

    /**
     * User ID (ums_user.id)
     */
    private Long userId;

    /**
     * Nickname
     */
    private String nickname;

    /**
     * User display ID (10-digit, e.g., 2612345678)
     */
    private String userDisplayId;

    /**
     * Mobile phone
     */
    private String mobile;

    /**
     * Email
     */
    private String email;

    /**
     * Avatar URL
     */
    private String avatarUrl;

    /**
     * User type: 1=regular user, 2=seller
     */
    private Integer userType;

    /**
     * Registration time
     */
    private LocalDateTime registeredAt;

    /**
     * Event timestamp
     */
    private LocalDateTime eventTime;

    /**
     * Check if this is a seller type user
     * @return true if userType is 2 (seller)
     */
    public boolean isSellerType() {
        return userType != null && userType == 2;
    }

    /**
     * Factory method to create a MemberRegisteredEvent
     */
    public static MemberRegisteredEvent create(
            Long userId,
            String nickname,
            String mobile,
            String email,
            Integer userType,
            String userDisplayId) {
        return MemberRegisteredEvent.builder()
                .eventId(java.util.UUID.randomUUID().toString())
                .userId(userId)
                .nickname(nickname)
                .mobile(mobile)
                .email(email)
                .userType(userType)
                .userDisplayId(userDisplayId)
                .registeredAt(DateTimeUtils.nowUtc())
                .eventTime(DateTimeUtils.nowUtc())
                .build();
    }
}

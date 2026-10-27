package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a user likes a post or comment
 * Consumers: notification-service (notify content owner)
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class LikeAddedEvent extends BaseEvent {

    private Long likeId;
    private Long likerId;
    private String likerName;
    private String targetType;  // POST, COMMENT
    private Long targetId;  // postId or commentId
    private Long targetOwnerId;  // Owner of the liked content

    public static LikeAddedEvent create(Long likeId, Long likerId, String likerName,
                                         String targetType, Long targetId, Long targetOwnerId) {
        LikeAddedEvent event = LikeAddedEvent.builder()
                .likeId(likeId)
                .likerId(likerId)
                .likerName(likerName)
                .targetType(targetType)
                .targetId(targetId)
                .targetOwnerId(targetOwnerId)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_LIKE_ADDED, "content-service");
        return event;
    }

    /**
     * 此處 targetType 為 LikeTargetType 命名空間（按讚對象類型），
     * 非 {@link io.github.will413028.hawkr.common.utils.code.AssetReferenceType}（雖然 POST 字面相同但語意不同）。
     * LikeTargetType 由 social-service 內部維護，不在 CodeMeta 範圍。
     */
    public boolean isPostLike() {
        return "POST".equals(targetType);
    }

    public boolean isCommentLike() {
        return "COMMENT".equals(targetType);
    }
}

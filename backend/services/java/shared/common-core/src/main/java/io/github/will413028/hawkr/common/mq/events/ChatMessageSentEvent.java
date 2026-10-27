package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a chat message is sent
 * Consumers: notification-service (push notification if offline), analytics-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class ChatMessageSentEvent extends BaseEvent {

    private Long messageId;
    private Long conversationId;
    private Long senderId;
    private String senderName;
    private Long receiverId;
    private String messageType;  // TEXT, IMAGE, ORDER_CARD, PRODUCT_CARD
    private String content;
    private Long orderId;  // For order-related chats
    private Long productId;  // For product inquiry chats

    public static ChatMessageSentEvent create(Long messageId, Long conversationId, Long senderId,
                                               String senderName, Long receiverId, String messageType,
                                               String content, Long orderId, Long productId) {
        ChatMessageSentEvent event = ChatMessageSentEvent.builder()
                .messageId(messageId)
                .conversationId(conversationId)
                .senderId(senderId)
                .senderName(senderName)
                .receiverId(receiverId)
                .messageType(messageType)
                .content(content)
                .orderId(orderId)
                .productId(productId)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_CHAT_MESSAGE_SENT, "im-service");
        return event;
    }
}

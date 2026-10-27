package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

/**
 * Event published when a seller application is approved
 * Consumers: payment-service (trigger PAYUNi registration), notification-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class SellerApprovedEvent extends BaseEvent {

    private Long sellerAccountId;
    private Long memberId;
    private String sellerName;
    private String email;
    private String phone;

    public static SellerApprovedEvent create(Long sellerAccountId, Long memberId, String sellerName,
                                              String email, String phone) {
        SellerApprovedEvent event = SellerApprovedEvent.builder()
                .sellerAccountId(sellerAccountId)
                .memberId(memberId)
                .sellerName(sellerName)
                .email(email)
                .phone(phone)
                .build();
        event.initializeEvent(MqConstants.EVENT_TYPE_SELLER_APPROVED, "seller-service");
        return event;
    }
}

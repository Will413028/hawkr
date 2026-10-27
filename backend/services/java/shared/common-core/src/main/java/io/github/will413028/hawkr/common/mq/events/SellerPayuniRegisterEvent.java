package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;


/**
 * Event published when seller requests PAYUNi registration
 * seller-service -> payment-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class SellerPayuniRegisterEvent extends BaseEvent {

    private Long sellerAccountId;
    private Long userId;
    private String sellerName;          // MerName
    private String sellerNameEn;        // MerEngName
    private String businessUbn;         // UBN
    private String email;               // Email, MerEmail
    private String phone;               // ContactMobile
    private String contactName;         // ContactName
    private String address;             // MerAddr (full address)
    private String postalCode;          // MerAddrPost
    private String description;         // MerDesc

    /**
     * Create a new SELLER_PAYUNI_REGISTER event
     */
    public static SellerPayuniRegisterEvent create(
            Long sellerAccountId,
            Long userId,
            String sellerName,
            String sellerNameEn,
            String businessUbn,
            String email,
            String phone,
            String contactName,
            String address,
            String postalCode,
            String description) {

        SellerPayuniRegisterEvent event = SellerPayuniRegisterEvent.builder()
                .sellerAccountId(sellerAccountId)
                .userId(userId)
                .sellerName(sellerName)
                .sellerNameEn(sellerNameEn)
                .businessUbn(businessUbn)
                .email(email)
                .phone(phone)
                .contactName(contactName)
                .address(address)
                .postalCode(postalCode)
                .description(description)
                .build();

        event.initializeEvent(MqConstants.EVENT_TYPE_SELLER_PAYUNI_REGISTER, "seller-service");

        return event;
    }
}

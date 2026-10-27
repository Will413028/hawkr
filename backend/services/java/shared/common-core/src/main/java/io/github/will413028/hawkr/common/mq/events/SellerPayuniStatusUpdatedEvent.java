package io.github.will413028.hawkr.common.mq.events;

import io.github.will413028.hawkr.common.mq.BaseEvent;
import io.github.will413028.hawkr.common.mq.MqConstants;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.EqualsAndHashCode;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;


/**
 * Event published when PAYUNi seller status is queried and updated
 * payment-service -> seller-service
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
@EqualsAndHashCode(callSuper = true)
public class SellerPayuniStatusUpdatedEvent extends BaseEvent {

    private Long sellerAccountId;
    private String payuniMerId;

    // PAYUNi status codes (raw values from API)
    private String merStatus;       // 商店狀態: 1=啟用, 2=關閉, 8=拒絕, 9=審核中, 91=待補資料, 92=審核失敗, 99=停用/停權
    private String memberStatus;    // 會員狀態: 1=啟用, 5=待補資料, 8=拒絕, 9=待審核, 92=審核失敗, 99=關閉/停權
    private String creditStatus;    // 信用卡支付狀態
    private String applePayStatus;  // Apple Pay 狀態
    private String googlePayStatus; // Google Pay 狀態
    private String atmStatus;       // ATM 狀態

    // Derived status
    private Boolean approved;       // true if MerStatus=1 and MemberStatus=1
    private String errorMessage;    // Error message if status check failed

    /**
     * Create a new SELLER_PAYUNI_STATUS_UPDATED event for successful status check
     */
    public static SellerPayuniStatusUpdatedEvent success(
            Long sellerAccountId,
            String payuniMerId,
            String merStatus,
            String memberStatus,
            String creditStatus,
            String applePayStatus,
            String googlePayStatus,
            String atmStatus) {

        // Check if approved: MerStatus=1 means enabled
        boolean isApproved = "1".equals(merStatus);

        SellerPayuniStatusUpdatedEvent event = SellerPayuniStatusUpdatedEvent.builder()
                .sellerAccountId(sellerAccountId)
                .payuniMerId(payuniMerId)
                .merStatus(merStatus)
                .memberStatus(memberStatus)
                .creditStatus(creditStatus)
                .applePayStatus(applePayStatus)
                .googlePayStatus(googlePayStatus)
                .atmStatus(atmStatus)
                .approved(isApproved)
                .build();

        event.initializeEvent(MqConstants.EVENT_TYPE_SELLER_PAYUNI_STATUS_UPDATED, "payment-service");

        return event;
    }

    /**
     * Create a new SELLER_PAYUNI_STATUS_UPDATED event for failed status check
     */
    public static SellerPayuniStatusUpdatedEvent failure(
            Long sellerAccountId,
            String payuniMerId,
            String errorMessage) {

        SellerPayuniStatusUpdatedEvent event = SellerPayuniStatusUpdatedEvent.builder()
                .sellerAccountId(sellerAccountId)
                .payuniMerId(payuniMerId)
                .approved(false)
                .errorMessage(errorMessage)
                .build();

        event.initializeEvent(MqConstants.EVENT_TYPE_SELLER_PAYUNI_STATUS_UPDATED, "payment-service");

        return event;
    }
}

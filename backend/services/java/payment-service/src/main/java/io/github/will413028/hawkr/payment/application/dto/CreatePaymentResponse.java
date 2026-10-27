package io.github.will413028.hawkr.payment.application.dto;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payuni.payment.model.UppFormData;

import java.time.LocalDateTime;
import java.time.LocalDate;

/**
 * 建立支付回應 DTO
 */
public record CreatePaymentResponse(
        String tradeNo,
        String orderId,
        PaymentMethod paymentType,
        long amount,
        PaymentTransactionStatus status,

        // 錯誤訊息（幕後即時付失敗時使用）
        String errorMessage,

        // 信用卡 UPP 表單資料（前端需組裝 Form POST 到 PayUni）
        UppFormData uppFormData,

        // ATM 專用
        String atmBankCode,
        String atmVirtualAccount,
        LocalDate atmExpireDate,

        LocalDateTime expireAt
) {

    /**
     * 建立信用卡支付回應（UPP 表單資料）
     */
    public static CreatePaymentResponse forCreditCard(String tradeNo, String orderId, long amount, UppFormData uppFormData) {
        return new CreatePaymentResponse(
                tradeNo, orderId, PaymentMethod.CREDIT_CARD, amount,
                PaymentTransactionStatus.INIT, null, uppFormData,
                null, null, null, null
        );
    }

    /**
     * 建立 Apple Pay 支付回應（幕後即時付 - 同步返回結果）
     *
     * @param tradeNo      交易編號
     * @param orderId      訂單編號
     * @param amount       金額
     * @param status       交易狀態
     * @param errorMessage 錯誤訊息（失敗時傳遞 PayUni 返回的錯誤原因）
     */
    public static CreatePaymentResponse forApplePay(String tradeNo, String orderId, long amount,
                                                    PaymentTransactionStatus status, String errorMessage) {
        return new CreatePaymentResponse(
                tradeNo, orderId, PaymentMethod.APPLE_PAY, amount,
                status, errorMessage, null,
                null, null, null, null
        );
    }

    /**
     * 建立 Google Pay 支付回應（幕後即時付 - 同步返回結果）
     *
     * @param tradeNo      交易編號
     * @param orderId      訂單編號
     * @param amount       金額
     * @param status       交易狀態
     * @param errorMessage 錯誤訊息（失敗時傳遞 PayUni 返回的錯誤原因）
     */
    public static CreatePaymentResponse forGooglePay(String tradeNo, String orderId, long amount,
                                                     PaymentTransactionStatus status, String errorMessage) {
        return new CreatePaymentResponse(
                tradeNo, orderId, PaymentMethod.GOOGLE_PAY, amount,
                status, errorMessage, null,
                null, null, null, null
        );
    }

    /**
     * 建立零元訂單支付回應（§2A.1）
     * <p>
     * 全額折抵後無需付款，不呼叫 PayUni，直接 PAID。
     */
    public static CreatePaymentResponse forZeroAmount(String tradeNo, String orderId) {
        return new CreatePaymentResponse(
                tradeNo, orderId, PaymentMethod.ZERO_AMOUNT, 0L,
                PaymentTransactionStatus.PAID, null, null,
                null, null, null, null
        );
    }

    /**
     * 建立 ATM 支付回應
     */
    public static CreatePaymentResponse forAtm(String tradeNo, String orderId, long amount,
                                               String bankCode, String virtualAccount, LocalDate expireDate, LocalDateTime expireAt) {
        return new CreatePaymentResponse(
                tradeNo, orderId, PaymentMethod.ATM, amount,
                PaymentTransactionStatus.INIT, null, null,
                bankCode, virtualAccount, expireDate, expireAt
        );
    }
}

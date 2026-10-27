package io.github.will413028.hawkr.payment.application.dto;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;

/**
 * 建立支付請求 DTO
 *
 * mer_id / amount / product_name 由 PaymentApplicationService 從 Order Service 查詢，不由前端傳入
 */
public record CreatePaymentRequest(
        String orderNo,
        long buyerId,
        PaymentMethod paymentType,
        Integer expireDays,      // ATM 有效天數（選填）
        String applePayToken,    // Apple Pay Token（幕後即時付）
        String googlePayToken,   // Google Pay Token（幕後即時付）
        String bankType,         // ATM 銀行代碼（004=台灣銀行, 822=中信銀行, 013=國泰世華）
        String userEmail,        // 使用者信箱（ATM 通知用）
        String clientType,       // CodeMeta ClientType value（MOBILE_APP / WEB_BROWSER），空字串/null 視為 MOBILE_APP
        String appType           // CodeMeta AppType value（BUYER / SELLER），空字串/null 視為 BUYER
) {

    /**
     * 取得有效天數（預設由 PaymentApplicationService 注入 application.yml 設定值）
     */
    public Integer getExpireDays() {
        return expireDays;
    }

    /**
     * 取得銀行代碼（預設 822 中國信託）
     * 支援：004=台灣銀行, 822=中信銀行, 013=國泰世華
     */
    public String getBankType() {
        return bankType != null && !bankType.isEmpty() ? bankType : "822";
    }
}

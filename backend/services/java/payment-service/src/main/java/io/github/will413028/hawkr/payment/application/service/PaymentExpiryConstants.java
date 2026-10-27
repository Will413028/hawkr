package io.github.will413028.hawkr.payment.application.service;

/**
 * §2A.4 信用卡頁面過期常數。
 *
 * 對應 PayUni UPP 的 TradeLExpireSec 參數。
 * 信用卡建立時同步傳 TradeLExpireSec=600 給 PayUni，
 * 並設 expire_at = nowUtc + 600s，確保兩端一致。
 */
public final class PaymentExpiryConstants {

    private PaymentExpiryConstants() {
    }

    /**
     * 信用卡/Apple Pay/Google Pay 頁面過期秒數。
     * 與 PayUni TradeLExpireSec 同步，由我方定義。
     */
    public static final int CREDIT_CARD_EXPIRE_SECONDS = 600;
}

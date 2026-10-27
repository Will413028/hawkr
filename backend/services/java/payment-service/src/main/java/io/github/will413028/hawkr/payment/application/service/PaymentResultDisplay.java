package io.github.will413028.hawkr.payment.application.service;

/**
 * 付款結果頁面 UI 顯示分類。
 *
 * <p>用於 {@link PaymentPageService} 內部 HTML 渲染與 deep link 構造。獨立於
 * {@code PaymentTransactionStatus} / {@code PaymentStatus} 等 CodeMeta enum——
 * 此處只表達「結果頁面要顯示哪一類視覺」（成功 / 失敗 / 處理中 / 系統異常），
 * 與交易實際狀態解耦。</p>
 */
public enum PaymentResultDisplay {
    SUCCESS,
    FAILED,
    PENDING,
    ERROR;

    public String code() {
        return name();
    }
}

package io.github.will413028.hawkr.payment.domain.repository;

import io.github.will413028.hawkr.payment.domain.model.RefundTransaction;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import io.github.will413028.hawkr.payment.infrastructure.persistence.dto.ProviderStatusRow;
import java.util.Map;
import java.util.Optional;

/**
 * 退款交易 Repository 介面
 */
public interface RefundTransactionRepository {

    /**
     * 儲存退款記錄
     */
    RefundTransaction save(RefundTransaction refund);

    /**
     * 根據退款編號查詢
     */
    Optional<RefundTransaction> findByRefundNo(String refundNo);

    /**
     * 根據原交易編號查詢所有退款記錄
     */
    List<RefundTransaction> findByTradeNo(String tradeNo);

    /**
     * 根據 PayUni 退款編號查詢
     */
    Optional<RefundTransaction> findByProviderRefundNo(String providerRefundNo);

    /**
     * 條件式更新狀態為 PROCESSING
     *
     * @return 影響行數
     */
    int updateStatusToProcessing(String refundNo, String providerRefundNo, int currentVersion);

    /**
     * 條件式更新狀態為 SUCCEEDED（含 PayUni 回傳資料 + bankDate 里程碑）
     * bankDate 僅 ATM async webhook RefundStatus=1 時有值；其他情境傳 null。
     *
     * @return 影響行數
     */
    int updateStatusToSucceeded(String refundNo, String providerRefundNo,
                                String providerStatus, String providerMessage,
                                LocalDate bankDate,
                                LocalDateTime completedAt, int currentVersion);

    /**
     * 條件式更新狀態為 FAILED（含 PayUni 回傳資料）
     *
     * @return 影響行數
     */
    int updateStatusToFailed(String refundNo, String providerStatus,
                             String providerMessage, int currentVersion);

    /**
     * ATM sync（頁面版受理）成功：REQUESTED → PROCESSING + 填入 estimated_refund_date。
     * 由 AfterSales.handleRefundPageReturn → Payment.AckAtmRefundAccepted 呼叫進來。
     */
    int markAtmRefundAccepted(String refundNo, String providerStatus, String providerMessage,
                              LocalDate estimatedRefundDate);

    /**
     * ATM async webhook 路徑：以 trade_no 找目前活躍（REQUESTED/PROCESSING）的退款 row。
     */
    Optional<RefundTransaction> findActiveByTradeNo(String tradeNo);

    /**
     * 以 trade_no 找最新一筆退款（不限狀態）。webhook finalize 找不到 active row 時判定終態用。
     */
    Optional<RefundTransaction> findLatestByTradeNo(String tradeNo);

    /**
     * ATM webhook finalize → SUCCEEDED（狀態機冪等，WHERE status IN(0,1)，無 version 守衛）。
     *
     * @return 影響行數
     */
    int finalizeAtmRefundSucceeded(String refundNo, String providerRefundNo,
                                   String providerStatus, String providerMessage,
                                   LocalDate bankDate, LocalDateTime completedAt);

    /**
     * ATM webhook finalize → FAILED（狀態機冪等，WHERE status IN(0,1)，無 version 守衛）。
     *
     * @return 影響行數
     */
    int finalizeAtmRefundFailed(String refundNo, String providerStatus, String providerMessage);

    /**
     * ATM webhook 狀態衝突校正 → 從 FAILED('3') 校正為 SUCCEEDED（WHERE status='3'，無 version 守衛）。
     * 專供「本地 FAILED 但 PayUni 回成功」的校正路徑；重入冪等（status 已 '2' → 0 rows）。
     *
     * @return 影響行數
     */
    int correctFailedToSucceeded(String refundNo, String providerRefundNo,
                                 String providerStatus, String providerMessage,
                                 LocalDate bankDate, LocalDateTime completedAt);

    /**
     * 批次查詢：依 case_id 取每 case 最新 payment_refunds 的 provider_status/message。
     * 回傳 key=case_id, value=(status, message)。
     */
    List<ProviderStatusRow> findProviderStatusByCaseIds(List<String> caseIds);

    /**
     * 批次查詢：依 order_id 取每 order 最新 payment_refunds 的 provider_status/message。
     */
    List<ProviderStatusRow> findProviderStatusByOrderIds(List<String> orderIds);

    /**
     * 計算原交易的已退款總額（在途 + 成功，status IN ('0','1','2')）。
     * 供「本筆退款尚未存在於表中」的 caller（如 createRefund，本筆此時尚未 save）。
     */
    long sumRefundedAmountByTradeNo(String tradeNo);

    /**
     * §5.5.5 L2「排除本筆」：同 {@link #sumRefundedAmountByTradeNo}，但排除 excludeRefundNo 的本筆退款。
     * 供「本筆退款已預建於表中」的 caller（如 getRefundPageUrl），避免把本筆自己算進已退累計而誤擋。
     * excludeRefundNo 為 null 時退化為含全部。
     */
    long sumRefundedAmountByTradeNoExcluding(String tradeNo, String excludeRefundNo);

    /**
     * 檢查是否有進行中的退款
     */
    boolean hasProcessingRefund(String tradeNo);

    /**
     * §5.6.1 撥款接管主判準：該交易是否存在<b>已完成</b>退款（status = SUCCEEDED('2')，僅終態成功）。
     * <p>
     * 刻意不含在途（REQUESTED/PROCESSING）：PayUni 接管撥款的領域事實以「退款完成」為準
     * （對齊 order 端 refund_total_amount 僅由 aftersales.refund.completed 累加），
     * 在途退款最終可能 FAILED，不構成接管。
     */
    boolean hasSucceededRefund(String tradeNo);

    /**
     * 根據冪等鍵查詢退款記錄
     */
    Optional<RefundTransaction> findByIdempotencyKey(String idempotencyKey);
}

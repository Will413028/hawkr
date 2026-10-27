package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.FinancialLogType;
import io.github.will413028.hawkr.common.utils.code.RefundSource;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 賣家取消出貨完成事件的交易性處理（E3，58 §6.3.2.3）。
 *
 * <p>從 {@link AfterSalesEventConsumer#onCancelCompleted} 獨立出來，確保 Spring
 * {@code @Transactional} proxy 正常運作（同 class 內 {@code this.} 呼叫不經 proxy，
 * 事務完全失效——order-service 曾有四處帶「InTx」之名卻從未有事務的前例）。
 *
 * <p><b>本 handler 補上了一個長期缺口</b>：{@code onCancelCompleted} 的 JavaDoc 從第一天就寫著
 * 「冪等寫入 financial_log」，但方法本體只做了 {@code updateNeedRefundPage}——
 * <b>賣家取消出貨的退款從來不在資金流水裡</b>。而 {@code order.refund.finalized} 的
 * {@code refund_ref} 依定義就是 {@code financial_logs.external_txn_id}，
 * 不補這一筆流水，這條發布點就沒有合法的冪等鍵可用。
 *
 * <p>同一事務內依序：
 * <ol>
 *   <li>冪等寫入 financial_log（{@code REFUND_ORIGINAL}，{@code external_txn_id = "AS_CANCEL_" + cancelId}）</li>
 *   <li>落 {@code order.refund.finalized} outbox</li>
 *   <li>清除 need_refund_page</li>
 * </ol>
 *
 * <p><b>金額來源是 {@code orders.total_actual_amount}</b>，不改 aftersales 的 payload：
 * {@code seller_cancel_requests} 的語意就是「未出貨前整單取消 ＋ 全額退款」，
 * 且 {@code uk_seller_cancel_order_no} 保證一訂單一筆。
 *
 * <p>{@code @Transactional} 內不做 gRPC。
 */
@Service
public class SellerCancelRefundTxHandler {

    private static final ObsLogger log = ObsLogger.forClass(SellerCancelRefundTxHandler.class);

    private final OrderRepository orderRepository;
    private final FinancialLogRepository financialLogRepository;
    private final OrderRefundFinalizedOutboxWriter refundFinalizedOutboxWriter;

    public SellerCancelRefundTxHandler(OrderRepository orderRepository,
                                       FinancialLogRepository financialLogRepository,
                                       OrderRefundFinalizedOutboxWriter refundFinalizedOutboxWriter) {
        this.orderRepository = orderRepository;
        this.financialLogRepository = financialLogRepository;
        this.refundFinalizedOutboxWriter = refundFinalizedOutboxWriter;
    }

    /**
     * @param cancelId 取消單號；空值 fail-loud（見方法內註解）
     */
    @Transactional
    public void handleCancelCompleted(String orderNo, String cancelId) {
        // fail-loud：cancelId 是 refund_ref 的唯一組成，空值會讓所有賣家取消退款共用
        // 同一個 "AS_CANCEL_" 鍵 ⇒ financial_logs 的 UNIQUE 只讓第一筆進去、其餘靜默 DO NOTHING，
        // 而下游的 REFUND:{refund_ref} 也會把它們全部視為同一筆退款。
        if (cancelId == null || cancelId.isBlank()) {
            throw new IllegalStateException(
                    "cancel.completed payload 缺少 cancel_id，無法組成 refund_ref, orderNo=" + orderNo);
        }

        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        if (order == null) {
            throw new IllegalStateException("cancel.completed 找不到訂單, orderNo=" + orderNo);
        }
        long refundAmount = order.getTotalActualAmount() == null ? 0L : order.getTotalActualAmount();
        String refundRef = "AS_CANCEL_" + cancelId;

        int logRows = financialLogRepository.insertIdempotent(
                TsidUtil.generate(),
                orderNo,
                FinancialLogType.REFUND_ORIGINAL.value(),
                refundAmount,
                refundRef);
        if (logRows == 0) {
            log.info("SellerCancelRefundTxHandler.duplicate",
                    "取消出貨退款流水已存在（重複投遞），冪等 skip 累加, refundRef=" + refundRef,
                    "orderNo", orderNo);
        }

        // E3（58 §6.3.2.3）：同事務落 outbox。
        //
        // 不需要 E1 的 duplicate 守衛，這個不對稱是刻意的：本事件沒有無限重發的 producer
        // （aftersales 側只在賣家實際取消時發一次），且 uk_seller_cancel_order_no 保證一訂單一筆；
        // 真的重投時下游仍以 REFUND:{refund_ref} 去重（上面那條講的是「不要讓 outbox 白白長胖」，
        // 不是唯一防線）。
        refundFinalizedOutboxWriter.write(orderNo, RefundSource.SELLER_CANCEL, refundRef,
                refundAmount, null, null, cancelId);

        // 清除 need_refund_page 標記（原本在 consumer 內、非交易性；一併收進本事務）
        orderRepository.updateNeedRefundPage(orderNo, false);
    }
}

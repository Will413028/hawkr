package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 履約責任判定 financially final ⇒ {@code SHIPPING_ABNORMAL(35)} →
 * {@code DELIVERY_FAILED_NO_REFUND(54)} 的交易性處理（03 §5.7.3）。
 *
 * <p><b>本類別存在的理由與 {@link OrderCompletionTxHandler} 完全相同</b>：
 * {@code @Transactional} 靠 Spring proxy，同 class 內的 {@code this.} 呼叫不經 proxy
 * ⇒ 事務實際失效。而這條路徑的兩件事<b>必須原子</b>：
 *
 * <pre>
 *   ① CAS  : UPDATE orders SET status='54' WHERE order_no=? AND status='35'
 *   ② affected &gt; 0 ⇒ INSERT grant_payment_tasks(PENDING)
 * </pre>
 *
 * <p>🔴 <b>拆成兩個交易的後果不對稱，所以不能「反正兩邊都會重試」帶過</b>：
 * <ul>
 *   <li>先 commit 狀態、再建任務而失敗 ⇒ 訂單已進 <b>無出邊的財務終態</b>，
 *       而撥款任務不存在。{@code 54} 不在任何排程的掃描述詞內
 *       （{@code AutoCompleteScheduler} 掃的是 {@code status='40'}），
 *       也沒有人工改狀態端點 ⇒ <b>那筆錢永遠不會撥給賣家，且沒有任何告警</b>。</li>
 *   <li>先建任務、再轉狀態而失敗 ⇒ 撥款任務對一張還在 {@code 35} 的訂單生效，
 *       而 {@code 35} 的語意是「責任還沒判完」⇒ <b>錢先撥出去了</b>。</li>
 * </ul>
 * 兩個方向都是不可逆的錢，所以只有同一個交易是對的。
 *
 * <p>🔴 <b>本 handler 不觸發任何退款</b>：不建 {@code auto_refund_tasks}、不呼叫退款 RPC、
 * 不寫退款類 {@code financial_logs}。{@code BUYER_CAUSED} 的財務結論就是
 * {@code refund_to_buyer = 0}（65 F），而「不退款」不是一個要執行的動作。
 *
 * <p>🔴 <b>也不寫收貨相關欄位</b>：不寫 {@code confirmed_at}／{@code confirm_type}／
 * {@code receipt_photo_keys}、不發 {@code order.receipt.confirmed}。
 * {@code 54} <b>不經過</b> {@code DELIVERED(40)}，買家從來沒有收到貨。
 *
 * <p>🔴 <b>54 不可逆</b>：本類別不得產生任何把訂單推離 {@code 54} 的路徑
 * （無 {@code 54 → 70}、無 {@code 54 → 80}、無 post-54 financial reversal、無賣家扣回）。
 * 責任爭議的收斂點在 financially final <b>之前</b>——未 final 期間訂單停在 {@code 35}、
 * 撥款任務根本還不存在 ⇒「撥款跑贏申訴」這個 race 在本路徑不存在，不需要 post-final 補救。
 */
@Service
public class ResponsibilityFinalizedTxHandler {

    private static final ObsLogger log = ObsLogger.forClass(ResponsibilityFinalizedTxHandler.class);

    /** CAS 與撥款任務建立的結果。🔴 每一格的後續處置都不同，所以不合併成布林。 */
    public enum Outcome {
        /** CAS 成功 ＋ 撥款任務已在同一交易建立。 */
        APPLIED,
        /**
         * 訂單已經是 {@code 54} 且撥款任務存在 ⇒ 正常重送，idempotent no-op。
         * 🔴 這是 at-least-once 世界的常態，<b>不是</b>失敗。
         */
        ALREADY_FINAL,
        /**
         * 🔴 訂單既不在 {@code 35} 也不是 {@code 54}（例如已被推到 {@code 70}），
         * 或它是 {@code 54} 卻查不到撥款任務。
         *
         * <p><b>呼叫端不得靜默 ACK</b>：這代表事件與既有終態不一致，
         * 必須有人看到（03 §5.7.3 逐字）。
         */
        INCONSISTENT
    }

    private final OrderRepository orderRepository;
    private final GrantPaymentTaskRepository grantPaymentTaskRepository;
    private final OrderEventPublisher eventPublisher;

    public ResponsibilityFinalizedTxHandler(OrderRepository orderRepository,
                                            GrantPaymentTaskRepository grantPaymentTaskRepository,
                                            OrderEventPublisher eventPublisher) {
        this.orderRepository = orderRepository;
        this.grantPaymentTaskRepository = grantPaymentTaskRepository;
        this.eventPublisher = eventPublisher;
    }

    /**
     * {@code 35 → 54} ＋ 建撥款任務（同一交易）。
     *
     * <p>🔴 <b>{@code affected = 0} 不得逕行當成「已處理」</b>（03 §5.7.3 逐字）：
     * 必須讀出當前狀態分辨是「重送」還是「不一致」，兩者的處置完全相反——
     * 前者 ACK，後者必須建案件並讓告警看得到。
     *
     * @return 見 {@link Outcome}
     */
    @Transactional
    public Outcome applyBuyerCausedFinality(String orderNo) {
        int affected = orderRepository.markDeliveryFailedNoRefund(
                orderNo,
                OrderStatus.SHIPPING_ABNORMAL.value(),
                OrderStatus.DELIVERY_FAILED_NO_REFUND.value());

        if (affected == 0) {
            return classifyNoop(orderNo);
        }

        // 🔴 與 ① 同一交易。撥款任務一律 PENDING，PayUni 接管分流不在建立時判斷
        //    （由 GrantPaymentWorker 依 payment-service 回應收斂，03 §3.11.2 單一收斂點）。
        //    ⚠️ uk_grant_payment_order UNIQUE(order_no)：同一訂單終其一生只有一列
        //    ⇒ 若這裡撞到唯一鍵，代表這張訂單先前已經有撥款任務（例如曾走過 40→50），
        //      而它現在竟然還能從 35 被 CAS 成功 ⇒ 不變式壞了，讓例外把整個交易 rollback。
        GrantPaymentTaskEntity task = new GrantPaymentTaskEntity();
        task.setTaskId(TsidUtil.generate());
        task.setOrderNo(orderNo);
        // 🔴 confirmType 走 SRC_SYSTEM 語意：買家從未確認收貨，這筆撥款是責任判定的結果，
        //    不是收貨確認的結果。不得沿用買家確認的 confirmType 值而讓稽核以為有人按過。
        task.setConfirmType(OrderConstants.CONFIRM_TYPE_SYSTEM);
        task.setStatus(OrderConstants.TASK_STATUS_PENDING);
        task.setRetryCount(0);
        task.setMaxRetryCount(OrderConstants.DEFAULT_MAX_RETRY_COUNT);
        grantPaymentTaskRepository.insert(task);

        log.info("ResponsibilityFinalizedTxHandler.applied",
                "責任判定 BUYER_CAUSED，訂單進入配送失敗未退款終態並建立撥款任務",
                "orderNo", orderNo);

        // 03 §2.1 emit 矩陣 #9：逐路徑掛 emit，afterCommit 發賣家工作區 realtime。
        // ⚠️ 這是「逐路徑掛」不是「重構出統一收斂點」——該矩陣明令不得藉此重構。
        eventPublisher.emitSellerOrderUpdated(orderNo, OrderEventPublisher.SRC_SYSTEM,
                OrderStatus.SHIPPING_ABNORMAL.value(),
                OrderStatus.DELIVERY_FAILED_NO_REFUND.value());
        return Outcome.APPLIED;
    }

    /**
     * {@code affected = 0} 的分流。
     *
     * <p>🔴 <b>「已經是 54」還不夠</b>：必須同時確認撥款任務存在。只看狀態的話，
     * 一筆「狀態轉了但任務沒建」的訂單會被讀成正常重送而永遠沒有人發現——
     * 而那正是本 handler 用單一交易在防的失效形狀，不該由它自己的冪等路徑掩蓋掉。
     */
    private Outcome classifyNoop(String orderNo) {
        var order = orderRepository.findByOrderNo(orderNo);
        String current = order == null ? null : order.getStatus();

        if (!OrderStatus.DELIVERY_FAILED_NO_REFUND.value().equals(current)) {
            log.error("ResponsibilityFinalizedTxHandler.inconsistentState",
                    "責任判定事件與訂單既有狀態不一致，不建立第二筆撥款任務 orderNo=" + orderNo
                            + " currentStatus=" + current);
            return Outcome.INCONSISTENT;
        }
        if (grantPaymentTaskRepository.findByOrderNo(orderNo) == null) {
            log.error("ResponsibilityFinalizedTxHandler.missingGrantTask",
                    "訂單已是 DELIVERY_FAILED_NO_REFUND 但查無撥款任務——"
                            + "狀態轉換與撥款任務曾非原子 orderNo=" + orderNo);
            return Outcome.INCONSISTENT;
        }
        log.info("ResponsibilityFinalizedTxHandler.replay",
                "責任判定事件重送，訂單已在終態且撥款任務存在", "orderNo", orderNo);
        return Outcome.ALREADY_FINAL;
    }
}

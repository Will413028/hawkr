package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentEventType;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.common.utils.code.RescueReason;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.payment.exception.PaymentBusinessException;
import io.github.will413028.hawkr.payment.infrastructure.mq.OutboxEventFactory;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Isolation;
import org.springframework.transaction.annotation.Transactional;

import java.util.LinkedHashMap;
import java.util.Map;

import java.time.LocalDateTime;
import java.util.EnumSet;
import java.util.Optional;
import java.util.Set;

/**
 * §2A.6 重複付款處理服務。
 * <p>
 * 偵測同一 order_id 收到兩筆 PAID 的情境，依 §2A.6.3 演算法選擇退款對象，
 * 翻轉主/副 row 標記，並發起 rescue 退款 event。
 * <p>
 * 整個 {@link #handleDuplicateFromWebhook} 包在 REPEATABLE_READ 事務內，
 * 配合 FOR UPDATE 行鎖避免併發 race。
 */
@Service
public class DuplicatePaymentService {

    private static final ObsLogger log = ObsLogger.forClass(DuplicatePaymentService.class);

    private static final Set<PaymentMethod> ELECTRONIC_INSTANT = EnumSet.of(
            PaymentMethod.CREDIT_CARD, PaymentMethod.APPLE_PAY, PaymentMethod.GOOGLE_PAY);
    private static final Set<PaymentMethod> OFFLINE = EnumSet.of(PaymentMethod.ATM);

    private enum Category {ELECTRONIC_INSTANT, OFFLINE}

    private final PaymentTransactionRepository transactionRepository;
    private final PaymentDomainService domainService;
    private final OrderGrpcClient orderGrpcClient;
    private final OutboxEventMapper outboxMapper;
    private final ObjectMapper objectMapper;

    public DuplicatePaymentService(PaymentTransactionRepository transactionRepository,
                                   PaymentDomainService domainService,
                                   OrderGrpcClient orderGrpcClient,
                                   OutboxEventMapper outboxMapper,
                                   ObjectMapper objectMapper) {
        this.transactionRepository = transactionRepository;
        this.domainService = domainService;
        this.orderGrpcClient = orderGrpcClient;
        this.outboxMapper = outboxMapper;
        this.objectMapper = objectMapper;
    }

    /**
     * §2A.7 觸發點 #5：翻轉主/副後呼叫 snapshot 寫入新主的 payment_type。
     */
    private void snapshotMain(PaymentTransaction newMain) {
        orderGrpcClient.snapshotPaymentInfo(
                newMain.orderId(), newMain.tradeNo(),
                newMain.paymentType().value(),
                newMain.atmBankCode(), newMain.atmVirtualAccount(),
                newMain.atmExpireDate() != null ? newMain.atmExpireDate().toString() : null);
    }

    /**
     * Webhook 段 3 入口（§2A.3.6 + §2A.6.2 管道 A）。
     * <p>
     * 呼叫前提：webhook handler 已確認
     * (1) findByProviderTradeNo 找不到（段 1 失敗），
     * (2) findActiveByOrderId 找不到 INIT row（段 2 失敗），
     * (3) findHistoryByOrderId 至少找到一筆 PAID（觸發本流程）。
     * <p>
     * 本方法在單一 REPEATABLE_READ 事務內：
     * 1. 鎖既有「主」PAID row（FOR UPDATE）
     * 2. INSERT 新 row（is_duplicate=TRUE 暫定，後可能翻轉）
     * 3. chooseRefundTarget 決定退誰
     * 4. 必要時翻轉主/副
     * 5. 發起 rescue 退款 event（直接 publish；外層 @Transactional 確保 rollback 一致性）
     *
     * @return 新建立的 row 與 refund target
     */
    @Transactional(isolation = Isolation.REPEATABLE_READ)
    public DuplicateResult handleDuplicateFromWebhook(
            String orderId,
            String webhookProviderTradeNo,
            PaymentMethod webhookPaymentType,
            long webhookAmount,
            String merId,
            LocalDateTime providerPaidAt,
            String webhookProviderStatus,
            String webhookProviderMessage) {

        // 1. 鎖既有「主」PAID row
        Optional<PaymentTransaction> mainOpt = transactionRepository.lockExistingPaidByOrderId(orderId);
        if (mainOpt.isEmpty()) {
            log.error("DuplicatePaymentService.handle.noMain",
                    "段 3 重複付款偵測但找不到主 PAID row order_id=" + orderId);
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_DUPLICATE_DETECTED, orderId);
        }
        PaymentTransaction main = mainOpt.get();

        // 1A. 金額一致性檢查（§2A.6.10 業務邊界）
        if (main.amount() != webhookAmount) {
            log.error("DuplicatePaymentService.handle.amountMismatch",
                    "重複付款金額不一致 order_id=" + orderId
                            + " main=" + main.amount() + " new=" + webhookAmount);
            throw new PaymentBusinessException(
                    GrpcResponseCode.PAYMENT_AMOUNT_MISMATCH, main.amount(), webhookAmount);
        }

        // 1B. 付款方式分類（不支援的拋例外）
        Category mainCat = paymentCategory(main.paymentType());
        Category newCat = paymentCategory(webhookPaymentType);

        // 2. INSERT 新 row（is_duplicate=TRUE 暫定）
        //    §2A.3.6 MerTradeNo 序列化：trade_no = orderId + attemptSeq，避免 UNIQUE 衝突
        int seq = transactionRepository.findMaxDuplicateSequenceByOrderId(orderId) + 1;
        int nextAttemptSeq = transactionRepository.findMaxAttemptSeqByOrderId(orderId)
                .map(s -> s + 1).orElse(1);
        String newTradeNo = orderId + String.format("%03d", nextAttemptSeq);
        LocalDateTime now = DateTimeUtils.nowUtc();
        PaymentTransaction newRow = PaymentTransaction.builder()
                .id(io.github.will413028.hawkr.common.utils.TsidUtil.generate())
                .tradeNo(newTradeNo)
                .orderId(orderId)
                .attemptSeq(nextAttemptSeq)
                .merId(merId != null ? merId : main.merId())
                .buyerId(main.buyerId())
                .paymentType(webhookPaymentType)
                .amount(webhookAmount)
                .productName(main.productName())
                .status(PaymentTransactionStatus.PAID)
                .providerTradeNo(webhookProviderTradeNo)
                .providerStatus(webhookProviderStatus)
                .providerMessage(webhookProviderMessage)
                .payTime(now)
                .providerPaidAt(providerPaidAt)
                .createdAt(now)
                .updatedAt(now)
                .isDuplicate(true)
                .duplicateOfPaymentId(main.id())
                .duplicateSequence(seq)
                .version(0)
                .build();
        transactionRepository.save(newRow);

        // 3. chooseRefundTarget
        PaymentTransaction refundTarget = chooseRefundTarget(main, newRow, mainCat, newCat);
        boolean flip = (refundTarget.id().equals(main.id()));

        // 4. 必要時翻轉主/副
        if (flip) {
            transactionRepository.unmarkDuplicate(newRow.id());
            transactionRepository.markAsDuplicate(main.id(), newRow.id(), seq);
            log.info("DuplicatePaymentService.handle.flipped",
                    "主/副翻轉 order_id=" + orderId
                            + " new_main=" + newRow.id() + " new_dup=" + main.id());
        }

        // 5. §2A.7 翻轉後 snapshot 新主到 orders.payment_method
        PaymentTransaction newMain = flip ? newRow : main;
        snapshotMain(newMain);

        // 6. 發起 rescue 退款 event（§2A.6.2 outbox pattern：INSERT outbox_events，
        //    由 OutboxEventPublisherScheduler 非同步派送，與本事務 commit 同步）
        publishRescueRefundInitiated(refundTarget, RescueReason.RESCUE_DUPLICATE_PAYMENT);

        return new DuplicateResult(newRow, refundTarget, flip);
    }

    /**
     * §2A.3.6 段 1.b 入口：「切換後舊管道完成單一交易」場景。
     * <p>
     * 呼叫前提：webhook 段 1 findByProviderTradeNo 找到 row，且該 row 是
     * CLOSED_BY_SWITCH/EXPIRED/CLOSED 終態（被切換掉），webhook 帶 PAID。
     * <p>
     * 此情境**不是**重複付款偵測，是「同一筆 PayUni 交易（同 provider_trade_no）
     * 在我方被切換掉後，使用者用舊管道完成付款」。row 與 webhook 是**同一筆 PayUni 交易**，
     * 不能 INSERT 新 row（會撞 uq_payment_transactions_provider_trade_no UNIQUE）。
     * <p>
     * 但仍須觸發 §2A.6 退款流程，因為同 order 已有另一筆 PAID（切換後完成的），
     * 使用者**事實上**付了兩次（兩筆獨立 PayUni 交易：信用卡的 + ATM 的）。
     * <p>
     * 處理流程：
     * 1. 鎖既有「主」PAID row（切換後完成的那筆）
     * 2. 金額一致檢查
     * 3. reactivate 段 1 row：CLOSED_BY_SWITCH → PAID + is_duplicate=TRUE + duplicate_of=main.id
     * 4. chooseRefundTarget(reactivated, main)
     * 5. 必要時翻轉
     * 6. snapshot orders.payment_method
     * 7. publish rescue 退款 event 對 refundTarget
     */
    @Transactional(isolation = Isolation.REPEATABLE_READ)
    public DuplicateResult handleSwitchBackToPaid(
            PaymentTransaction reactivatedRow,
            LocalDateTime providerPaidAt) {

        String orderId = reactivatedRow.orderId();

        // 1. 鎖既有「主」PAID row（切換後新建並完成的那筆，另一個 paymentType）
        Optional<PaymentTransaction> mainOpt = transactionRepository.lockExistingPaidByOrderId(orderId);
        if (mainOpt.isEmpty()) {
            log.error("DuplicatePaymentService.switchBack.noMain",
                    "段 1.b 找不到主 PAID row order_id=" + orderId
                            + "（reactivated row=" + reactivatedRow.id() + "）");
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_DUPLICATE_DETECTED, orderId);
        }
        PaymentTransaction main = mainOpt.get();

        // 1A. 金額一致性檢查
        if (main.amount() != reactivatedRow.amount()) {
            log.error("DuplicatePaymentService.switchBack.amountMismatch",
                    "金額不一致 order_id=" + orderId
                            + " main=" + main.amount() + " reactivated=" + reactivatedRow.amount());
            throw new PaymentBusinessException(
                    GrpcResponseCode.PAYMENT_AMOUNT_MISMATCH, main.amount(), reactivatedRow.amount());
        }

        // 1B. 付款方式分類（不支援的拋例外）
        Category mainCat = paymentCategory(main.paymentType());
        Category reactivatedCat = paymentCategory(reactivatedRow.paymentType());

        // 2. reactivate 段 1 row：CLOSED_BY_SWITCH → PAID + 標 duplicate
        //    （直接 mark + status 翻成 PAID 由 mapper 處理；初始 reactivated 為「副」，下一步可能翻轉）
        int seq = transactionRepository.findMaxDuplicateSequenceByOrderId(orderId) + 1;
        int reactivated = transactionRepository.reactivateRowAsDuplicate(
                reactivatedRow.id(), main.id(), seq, providerPaidAt);
        if (reactivated == 0) {
            log.error("DuplicatePaymentService.switchBack.reactivateFailed",
                    "reactivate 失敗 row_id=" + reactivatedRow.id());
            throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_DUPLICATE_DETECTED, orderId);
        }
        log.info("DuplicatePaymentService.switchBack.reactivated",
                "row reactivate 成功 order_id=" + orderId
                        + " reactivated_id=" + reactivatedRow.id()
                        + " main_id=" + main.id() + " sequence=" + seq);

        // 3. chooseRefundTarget
        // reactivated 此時 status=PAID 但 in-memory 物件還是舊狀態，這裡只用 paymentType / id 比較不影響
        PaymentTransaction refundTarget = chooseRefundTarget(reactivatedRow, main, reactivatedCat, mainCat);
        boolean flip = refundTarget.id().equals(main.id());  // 退主 → 翻轉

        // 4. 必要時翻轉主/副
        // 注意順序：必須先 markAsDuplicate(main) 再 unmarkDuplicate(reactivated)
        // 否則中間瞬間兩筆都 is_duplicate=FALSE 會撞 partial unique index
        if (flip) {
            // 退舊主 → 翻轉：reactivated 變新主、main 變副
            transactionRepository.markAsDuplicate(main.id(), reactivatedRow.id(), seq);
            transactionRepository.unmarkDuplicate(reactivatedRow.id());
            log.info("DuplicatePaymentService.switchBack.flipped",
                    "主/副翻轉 order_id=" + orderId
                            + " new_main=" + reactivatedRow.id() + " new_dup=" + main.id());
        }

        // 5. §2A.7 snapshot 新主到 orders.payment_method
        PaymentTransaction newMain = flip ? reactivatedRow : main;
        snapshotMain(newMain);

        // 6. 發起 rescue 退款 event
        publishRescueRefundInitiated(refundTarget, RescueReason.RESCUE_DUPLICATE_PAYMENT);

        return new DuplicateResult(reactivatedRow, refundTarget, flip);
    }

    /**
     * §2A.6.2 管道 B（reconcile 補救偵測）入口。
     * 呼叫前提：reconcile candidate 自身狀態為 EXPIRED→PAID（PayUni query 確認）且
     * 同 order_id 已有其他 PAID row。
     */
    @Transactional(isolation = Isolation.REPEATABLE_READ)
    public DuplicateResult handleDuplicateFromReconcile(PaymentTransaction candidate,
                                                        PaymentTransaction otherPaid) {
        if (candidate.amount() != otherPaid.amount()) {
            log.error("DuplicatePaymentService.reconcile.amountMismatch",
                    "重複付款金額不一致 order_id=" + candidate.orderId()
                            + " a=" + candidate.amount() + " b=" + otherPaid.amount());
            throw new PaymentBusinessException(
                    GrpcResponseCode.PAYMENT_AMOUNT_MISMATCH, otherPaid.amount(), candidate.amount());
        }
        Category catA = paymentCategory(candidate.paymentType());
        Category catB = paymentCategory(otherPaid.paymentType());

        // 標 candidate 為 duplicate（暫定）
        int seq = transactionRepository.findMaxDuplicateSequenceByOrderId(candidate.orderId()) + 1;
        transactionRepository.markAsDuplicate(candidate.id(), otherPaid.id(), seq);

        PaymentTransaction refundTarget = chooseRefundTarget(otherPaid, candidate, catB, catA);
        boolean flip = refundTarget.id().equals(otherPaid.id());

        if (flip) {
            transactionRepository.unmarkDuplicate(candidate.id());
            transactionRepository.markAsDuplicate(otherPaid.id(), candidate.id(), seq);
        }

        // §2A.7 翻轉後 snapshot
        snapshotMain(flip ? candidate : otherPaid);

        publishRescueRefundInitiated(refundTarget, RescueReason.RESCUE_DUPLICATE_PAYMENT);
        return new DuplicateResult(candidate, refundTarget, flip);
    }

    /**
     * §2A.6.3 演算法。
     */
    private PaymentTransaction chooseRefundTarget(PaymentTransaction rowA,
                                                  PaymentTransaction rowB,
                                                  Category catA,
                                                  Category catB) {
        // R1: 跨類別 → 退電子即時類那筆
        if (catA != catB) {
            return catA == Category.ELECTRONIC_INSTANT ? rowA : rowB;
        }
        // R2/R3: 同類別 → 退較晚那筆
        return rowA.createdAt().isBefore(rowB.createdAt()) ? rowB : rowA;
    }

    private Category paymentCategory(PaymentMethod type) {
        if (ELECTRONIC_INSTANT.contains(type)) return Category.ELECTRONIC_INSTANT;
        if (OFFLINE.contains(type)) return Category.OFFLINE;
        throw new PaymentBusinessException(
                GrpcResponseCode.UNSUPPORTED_PAYMENT_TYPE_FOR_DUPLICATE,
                type != null ? type.value() : "null");
    }

    /**
     * §2A.6.2 outbox pattern：rescue 退款 event 不直接 publish 到 MQ，
     * 改為 INSERT outbox_events 讓 OutboxEventPublisherScheduler 非同步派送。
     * <p>
     * 由於本方法只在外層 @Transactional(REPEATABLE_READ) 內被呼叫，INSERT
     * 與 row 變更同 transaction COMMIT，DB rollback 時 event 也回滾，從根本
     * 上消除「DB 寫成功 + MQ 失敗」或「MQ 成功 + DB rollback」兩個 race。
     */
    private void publishRescueRefundInitiated(PaymentTransaction target, RescueReason reason) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        // payload key 一律 snake_case（與全專案 PaymentDomainEvent 慣例一致，
        // 避免 RescueRefundConsumer 等下游 consumer key 對不上）
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("trade_no", target.tradeNo());
        payload.put("order_id", target.orderId());
        payload.put("mer_id", target.merId());
        payload.put("provider_trade_no", target.providerTradeNo());
        payload.put("amount", target.amount());
        payload.put("payment_type", target.paymentType() != null ? target.paymentType().value() : "");
        payload.put("rescue_reason", reason.code());
        // 事件 envelope metadata（36 §7.4 Conformance Contract：event_id 為 UUID v4 必含欄位）
        OutboxEventFactory.applyEnvelopeMetadata(payload,
                PaymentEventType.PAYMENT_RESCUE_REFUND_INITIATED.value(), "payment",
                target.tradeNo(), now.toString());

        String payloadJson;
        try {
            payloadJson = objectMapper.writeValueAsString(payload);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("rescue refund payload 序列化失敗: " + e.getMessage(), e);
        }

        OutboxEventEntity entity = OutboxEventFactory.newPendingEntity(
                MqConstants.EXCHANGE_PAYMENT,
                PaymentEventType.PAYMENT_RESCUE_REFUND_INITIATED.value(),
                MqConstants.ROUTING_KEY_PAYMENT_RESCUE_REFUND_INITIATED,
                payloadJson, now);
        outboxMapper.insert(entity);

        log.info("DuplicatePaymentService.outbox.rescueRefund.queued",
                "rescue 退款 outbox 已建立 trade_no=" + target.tradeNo()
                        + " outbox_id=" + entity.getId()
                        + " refund_target_id=" + target.id() + " reason=" + reason.code());
    }

    /** 處理結果 */
    public record DuplicateResult(PaymentTransaction newRow,
                                  PaymentTransaction refundTarget,
                                  boolean flipped) {
    }
}

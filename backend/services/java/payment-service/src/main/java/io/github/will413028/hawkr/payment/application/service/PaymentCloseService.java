package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.payment.domain.PaymentStatusRules;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentEventPublisher;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.Optional;

/**
 * §5.4 ClosePayment 獨立 tx 載體。
 * <p>
 * 抽到獨立 bean 的理由（對應 §40 §2.5 / §2A.6.5 Tx 載體規定）：
 * Spring AOP CGLIB proxy 不攔截同 class self-invocation，若 {@code closePayment} 與
 * {@code closePaymentByOrderNo} 同住 {@link PaymentApplicationService}，
 * {@code closePaymentByOrderNo} 內以 {@code this.closePayment(...)} 呼叫會繞過 proxy，
 * 讓 {@code closePayment} 上的 {@code @Transactional} 失效（語義謊言）。
 * <p>
 * 抽到本獨立 bean 後，所有 caller（PaymentGrpcService.closePayment + closePaymentByOrderNo）
 * 都跨 bean 呼叫 → 經 proxy → {@code @Transactional} 真實生效，tx 邊界與 method 位置對齊。
 */
@Service
public class PaymentCloseService {

    private static final ObsLogger log = ObsLogger.forClass(PaymentCloseService.class);

    private final PaymentTransactionRepository transactionRepository;
    private final PaymentDomainService domainService;
    private final PaymentEventPublisher eventPublisher;

    public PaymentCloseService(
            PaymentTransactionRepository transactionRepository,
            PaymentDomainService domainService,
            PaymentEventPublisher eventPublisher
    ) {
        this.transactionRepository = transactionRepository;
        this.domainService = domainService;
        this.eventPublisher = eventPublisher;
    }

    /**
     * 關閉付款交易（INIT → CLOSED）。
     * <p>
     * 同 tx 內：updateToClosed UPDATE + PaymentClosed event 經 PaymentEventPublisher 直接發布
     * （payment.closed 非付款結果事件、不走 outbox；發布失敗即 @Transactional rollback）。
     */
    @Transactional
    public boolean closePayment(String tradeNo) {
        Optional<PaymentTransaction> optTransaction = transactionRepository.findCanonicalByTradeNo(tradeNo);
        if (optTransaction.isEmpty()) {
            return false;
        }

        PaymentTransaction transaction = optTransaction.get();
        if (!domainService.canClose(transaction)) {
            log.warn("PaymentCloseService.close.rejected",
                    "交易無法關閉，當前狀態: " + transaction.status(), "trade_no", tradeNo);
            return false;
        }

        LocalDateTime closedAt = DateTimeUtils.nowUtc();
        int updated = transactionRepository.updateToClosed(tradeNo, closedAt, transaction.version());

        if (updated > 0) {
            PaymentDomainEvent.PaymentClosed event = domainService.createPaymentClosedEvent(transaction, closedAt);
            eventPublisher.publish(event);
            log.info("PaymentCloseService.close.success", "交易關閉成功", "trade_no", tradeNo);
            return true;
        }

        return false;
    }
}

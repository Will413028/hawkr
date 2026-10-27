package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * 逾期交易處理排程
 *
 * 定期檢查並標記逾期的交易
 *
 * <p><b>搶佔語意（issue #2294 B4 — 例外 A：同 tx outbox）</b>
 *
 * <p>{@code updateToExpired}（{@code SET status='3' WHERE trade_no=? AND status='0'
 * AND expire_at < now AND version=?}）是<b>業務狀態機的永久轉換</b>：{@code '3'} 是終態，
 * CAS 成功後該列永久離開候選集合，沒有任何時間欄能讓它回來 ⇒
 * 依《多副本排程搶佔規範》§二 通則 0 第 4 步，這是永久旗標，必須改。
 *
 * <p>三條出路中選<b>例外 A（同 tx outbox）</b>，理由：
 * <ul>
 *   <li><b>例外 B 不適用</b>：被移到 publish 之後的那一句是業務狀態轉換本身，不是事後記錄
 *       （規範 §四「例外 B 的適用邊界」）——換順序會在 CAS 回 0 時發出一則
 *       「這筆交易已逾期」但交易其實剛付款成功的事件，而且不可收斂。</li>
 *   <li><b>改 lease 不適用</b>：狀態不能「租」。</li>
 *   <li>payment-service <b>已有</b> {@code outbox_events} ＋ {@link PaymentOutboxWriter}
 *       ⇒ 例外 A 的基建成本為零、零 migration。</li>
 * </ul>
 *
 * <p>⚠️ <b>原本掛在本類別上的 {@code @Transactional} 在這裡零保護</b>：舊實作把
 * {@code eventPublisher.publish(event)} 的例外交給<b>同一方法內</b>的 per-item catch 吞掉
 * ⇒ 例外沒有穿過 proxy ⇒ 事務不會被標 rollback-only ⇒ {@code updateToExpired} 照樣 commit，
 * 事件卻永久遺失、無告警、無重試。
 * ⇒ 因此本次同時把事務邊界從「整批一個 tx」改成<b>逐筆跨 bean 短 tx</b>
 * （{@link ExpiredTransactionCloseService#closeExpiredInTx}），
 * 讓「enqueue 失敗 ⇒ 該筆 rollback ⇒ 下一 tick 重來」真的成立；
 * 本類別剩下的 per-item catch 只決定「要不要中斷整批」，不再影響原子性。
 */
@Component
public class ExpiredTransactionScheduler {

    private static final ObsLogger log = ObsLogger.forClass(ExpiredTransactionScheduler.class);

    private static final int BATCH_SIZE = 100;

    private final PaymentTransactionRepository transactionRepository;
    private final PaymentDomainService domainService;
    private final ExpiredTransactionCloseService closeService;

    public ExpiredTransactionScheduler(
            PaymentTransactionRepository transactionRepository,
            PaymentDomainService domainService,
            ExpiredTransactionCloseService closeService
    ) {
        this.transactionRepository = transactionRepository;
        this.domainService = domainService;
        this.closeService = closeService;
    }

    /**
     * 每分鐘檢查逾期交易
     */
    @Scheduled(fixedDelay = 60000)
    public void processExpiredTransactions() {
        List<PaymentTransaction> expiredList = transactionRepository.findExpiredTransactions(BATCH_SIZE);

        if (expiredList.isEmpty()) {
            return;
        }

        int processed = 0;
        for (PaymentTransaction transaction : expiredList) {
            try {
                if (processExpiredTransaction(transaction)) {
                    processed++;
                }
            } catch (Exception e) {
                log.error("ExpiredTransactionScheduler.process.failed", "處理逾期交易失敗: " + transaction.tradeNo(), e);
            }
        }

        if (processed > 0) {
            log.info("ExpiredTransactionScheduler.process.done", "逾期交易處理完成，共處理 " + processed + " 筆");
        }
    }

    /**
     * 處理單筆逾期交易。
     *
     * <p>純 domain 判定留在這裡（無 I/O）；狀態 CAS ＋ outbox 落在
     * {@link ExpiredTransactionCloseService#closeExpiredInTx} 的<b>跨 bean 短 tx</b> 內。
     */
    private boolean processExpiredTransaction(PaymentTransaction transaction) {
        if (!domainService.canMarkAsExpired(transaction)) {
            return false;
        }
        return closeService.closeExpiredInTx(transaction);
    }
}

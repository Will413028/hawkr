package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * 逾期交易關單的交易性處理（issue #2294 B4 — 《多副本排程搶佔規範》§四 <b>例外 A：同 tx outbox</b>）。
 *
 * <p><b>為什麼要獨立成一個 bean</b>（兩個理由，缺一都不成立）：
 * <ol>
 *   <li><b>self-invocation 會讓 {@code @Transactional} 失效</b>（10 檔 §11.1）：
 *       在 {@link ExpiredTransactionScheduler} 內以 {@code this.} 呼叫自家 {@code @Transactional}
 *       方法不經 Spring proxy，事務完全不存在。</li>
 *   <li><b>逐筆短 tx 才能讓「enqueue 失敗 ⇒ 該筆 rollback」成立</b>：舊實作把
 *       {@code @Transactional} 掛在<b>整批</b>掃描方法上，而 per-item 的 catch 在<b>同一個方法內</b>
 *       把例外吞掉 ⇒ 例外沒有穿過 proxy ⇒ 事務不會被標 rollback-only ⇒
 *       {@code updateToExpired} 照樣 commit、事件卻沒落 outbox。
 *       改成跨 bean 的逐筆 tx 之後，例外會穿過 proxy 觸發<b>該筆</b> rollback，
 *       呼叫端的 catch 只決定「要不要中斷整批」，不再影響原子性。</li>
 * </ol>
 *
 * <p><b>同一事務內</b>：{@code payment_transactions} 的 {@code INIT('0') → EXPIRED('3')} CAS
 * ＋ {@code outbox_events} 的 {@code payment.expired} INSERT。
 * rollback ⇒ 兩者皆不落地、狀態仍為 {@code '0'} 且 {@code expire_at} 仍已過
 * ⇒ 完全滿足 {@code findExpiredTransactions} 的候選述詞 ⇒ 下一 tick（60 秒）必然重撈。
 *
 * <p><b>殘餘風險（誠實標註）</b>：outbox INSERT 成功、commit 成功、afterCommit 即時投遞失敗
 * ⇒ 事件留在 {@code outbox_events} 的 PENDING，由 {@code OutboxEventPublisherScheduler}
 * 每 5s 兜底重投；事件不會遺失，只會延遲。
 *
 * <p><b>tx 內零網路呼叫</b>：{@code createPaymentExpiredEvent} 是純函式（只讀 domain model 欄位），
 * {@code enqueueAndDispatch} 只做序列化 ＋ INSERT，實際投遞在 afterCommit（tx 之外）。
 */
@Service
public class ExpiredTransactionCloseService {

    private final PaymentTransactionRepository transactionRepository;
    private final PaymentDomainService domainService;
    private final PaymentOutboxWriter outboxWriter;

    public ExpiredTransactionCloseService(PaymentTransactionRepository transactionRepository,
                                          PaymentDomainService domainService,
                                          PaymentOutboxWriter outboxWriter) {
        this.transactionRepository = transactionRepository;
        this.domainService = domainService;
        this.outboxWriter = outboxWriter;
    }

    /**
     * 單筆逾期關單（TX 內）：CAS {@code INIT → EXPIRED} ＋ {@code payment.expired} 落 outbox。
     *
     * @return true = 本次由我方轉成 EXPIRED 並已落 outbox；false = CAS 回 0（已被他路徑變更，冪等 skip）
     */
    @Transactional
    public boolean closeExpiredInTx(PaymentTransaction transaction) {
        int updated = transactionRepository.updateToExpired(transaction.tradeNo(), transaction.version());
        if (updated == 0) {
            return false;
        }

        // 例外 A：逾期事件與上面的狀態 CAS 同一個事務落 outbox（不是事務外 best-effort 直發）。
        // 序列化失敗拋例外 → 穿過 proxy → 本筆 rollback → 狀態沒轉、事件沒發，下一 tick 重來。
        PaymentDomainEvent.PaymentExpired event = domainService.createPaymentExpiredEvent(transaction);
        outboxWriter.enqueueAndDispatch(event);
        return true;
    }
}

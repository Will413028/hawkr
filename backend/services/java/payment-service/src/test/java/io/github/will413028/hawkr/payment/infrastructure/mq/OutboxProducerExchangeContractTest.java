package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.utils.code.RescueReason;
import io.github.will413028.hawkr.payment.application.service.DuplicatePaymentService;
import io.github.will413028.hawkr.payment.domain.event.PaymentDomainEvent;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.payment.mq.RescueRefundConsumer;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 三條既有 outbox 寫入路徑的契約：<b>exchange 一律 explicit 且為 {@code payment.events}</b>，
 * 而且 <b>delivery timing 語意未被 genericize 改變</b>（36 §3.3.1 BL6）。
 *
 * <p><b>為什麼 timing 也要測</b>：本輪把 exchange 做成 factory 的必填參數、並把
 * {@code PaymentOutboxWriter} 的底層抽成 generic enqueue。這類重構最容易順手「統一」
 * 各路徑的投遞時機——而它們現在是<b>刻意分裂</b>的：
 * <ul>
 *   <li>A1 {@code PaymentOutboxWriter}
 *       ＝ INSERT ＋ afterCommit 即時投遞 ＋ scheduler 兜底</li>
 *   <li>A3 {@code DuplicatePaymentService} / A4 {@code RescueRefundConsumer}
 *       ＝ 純 INSERT，只靠 scheduler 每 5s 派送（<b>刻意沒有</b> afterCommit）</li>
 * </ul>
 * 幫 A3／A4 加上 afterCommit 會改變它們的失敗語意與重入次數，是產品行為變更，不是重構。
 *
 * <p><b>A3／A4 用反射呼叫 private 方法</b>：它們的公開入口需要大量 stub（gRPC client、
 * repository、Rabbit Message／Channel），而本測試要驗的只是「那一行 insert 寫了什麼」。
 * 反射的代價是方法改名時會壞——所以刻意讓找不到方法時 {@code fail()} 而不是 skip：
 * 那時測試會叫出來，而不是靜默轉綠（§3.10 第 4 問）。
 */
class OutboxProducerExchangeContractTest {

    /** 契約字面值刻意硬寫，不引用 MqConstants——期望值來自被測物的話，常數改掉測試會跟著改而永遠通過。 */
    private static final String PAYMENT_EXCHANGE = "payment.events";

    private static final LocalDateTime NOW = LocalDateTime.of(2026, 9, 16, 12, 0, 0);

    @AfterEach
    void cleanUpSynchronization() {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.clearSynchronization();
        }
        // actualTransactionActive 是獨立的 ThreadLocal，clearSynchronization() 不會清它。
        // 漏清會污染同一條 thread 上的後續測試（讓它們誤以為自己在交易裡）。
        TransactionSynchronizationManager.setActualTransactionActive(false);
    }

    /**
     * 進入「有交易」的測試環境。
     *
     * <p>兩件事都要做，而且它們是<b>不同的</b> ThreadLocal：
     * <ul>
     *   <li>{@code initSynchronization()} → {@code isSynchronizationActive()}：
     *       afterCommit 這類 synchronization 能不能註冊</li>
     *   <li>{@code setActualTransactionActive(true)} → {@code isActualTransactionActive()}：
     *       是否真的有一個 DB 交易在跑 —— {@code enqueue} 的閘門看的是<b>這一個</b></li>
     * </ul>
     * 只做前者不足以通過閘門，那一條由 {@code PaymentOutboxWriterTransactionGuardTest#T2} 專責證明。
     * 本檔的目的是 exchange 與 timing，所以在這裡把環境補齊；
     * 真 DB 的交易行為（含 rollback 原子性）由 {@code OutboxEnqueueTransactionIntegrationTest} 承擔。
     */
    private static void enterTransaction() {
        TransactionSynchronizationManager.initSynchronization();
        TransactionSynchronizationManager.setActualTransactionActive(true);
    }

    private static OutboxEventEntity captureInsert(OutboxEventMapper mapper) {
        ArgumentCaptor<OutboxEventEntity> c = ArgumentCaptor.forClass(OutboxEventEntity.class);
        verify(mapper).insert(c.capture());
        return c.getValue();
    }

    // ───────────────── A1 PaymentOutboxWriter（afterCommit 路徑）─────────────────

    @Test
    @DisplayName("A1 PaymentOutboxWriter：exchange=payment.events，且 afterCommit 即時投遞仍在")
    void a1_paymentOutboxWriter() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        PaymentOutboxWriter writer = new PaymentOutboxWriter(
                mapper, new ObjectMapper(), mock(PaymentOutboxPublisher.class), new SimpleMeterRegistry());

        enterTransaction();
        writer.enqueueAndDispatch(new PaymentDomainEvent.PaymentSucceeded(
                "T1", "O1", "B1", 100L, "P1", "1", NOW, NOW));

        OutboxEventEntity row = captureInsert(mapper);
        assertEquals(PAYMENT_EXCHANGE, row.getExchange());
        assertEquals("payment.success", row.getRoutingKey());
        assertFalse(TransactionSynchronizationManager.getSynchronizations().isEmpty(),
                "A1 的 afterCommit 即時投遞不得消失——沒有它，付款事件要等 5 秒才出得去");
    }

    /**
     * 🔴 36 §3.3.1 / 65：業務 metric 不得下沉到 generic path。
     *
     * <p>{@code hawkr_payment_total} 是 {@code business-alerts.yml} 的
     * {@code PaymentFailureSpike} 的分母。未來 fulfillment 事件若走 generic enqueue 而它會計數，
     * 付款失敗率被稀釋 ⇒ 告警在最需要的時候不響，而且完全無聲。
     */
    @Test
    @DisplayName("A1 generic enqueue：不遞增 payment 業務 metric、不註冊 afterCommit（timing 交給 caller）")
    void a1_genericEnqueue_hasNoPaymentMetricAndNoDispatch() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        MeterRegistry registry = new SimpleMeterRegistry();
        PaymentOutboxWriter writer = new PaymentOutboxWriter(
                mapper, new ObjectMapper(), mock(PaymentOutboxPublisher.class), registry);

        enterTransaction();
        OutboxEventEntity returned = writer.enqueue(
                "hawkr.shipping", "fulfillment.responsibility.finalized",
                "fulfillment.responsibility.finalized", "{}");

        assertEquals("hawkr.shipping", returned.getExchange());
        assertEquals("hawkr.shipping", captureInsert(mapper).getExchange());

        assertEquals(0.0, registry.counter("hawkr_payment").count(),
                "generic path 遞增了 hawkr_payment ⇒ PaymentFailureSpike 的分母被非付款事件稀釋");
        assertEquals(0.0, registry.counter("hawkr_payment_failed").count(),
                "generic path 遞增了 hawkr_payment_failed");
        assertTrue(TransactionSynchronizationManager.getSynchronizations().isEmpty(),
                "generic enqueue 不得替 caller 決定投遞時機");
    }

    /** 對照組：證明上一支的 0.0 不是因為 metric 根本沒接上。 */
    @Test
    @DisplayName("對照組：payment wrapper 走 afterCommit 時確實會遞增 metric（證明計數器真的有接）")
    void a1_wrapperDoesIncrementMetricOnCommit() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        MeterRegistry registry = new SimpleMeterRegistry();
        PaymentOutboxWriter writer = new PaymentOutboxWriter(
                mapper, new ObjectMapper(), mock(PaymentOutboxPublisher.class), registry);

        enterTransaction();
        writer.enqueueAndDispatch(new PaymentDomainEvent.PaymentSucceeded(
                "T1", "O1", "B1", 100L, "P1", "1", NOW, NOW));
        TransactionSynchronizationManager.getSynchronizations().forEach(s -> s.afterCommit());

        assertEquals(1.0, registry.counter("hawkr_payment").count(),
                "payment wrapper 的業務 metric 必須留著——它不是 generic path 的東西，但也不能被一併刪掉");
    }

    // ───────────────── A3／A4（純 INSERT，刻意無 afterCommit）─────────────────

    private static Method privateMethod(Class<?> owner, String name, Class<?>... params) {
        try {
            Method m = owner.getDeclaredMethod(name, params);
            m.setAccessible(true);
            return m;
        } catch (NoSuchMethodException e) {
            // 刻意 fail 而非 skip：方法被改名時要叫出來，靜默轉綠才是真正的損失。
            fail("找不到 " + owner.getSimpleName() + "#" + name
                    + "——它若被改名或移除，本測試守的 exchange／timing 契約就沒有人看著了", e);
            throw new AssertionError("unreachable");
        }
    }

    private static void invoke(Method m, Object target, Object... args) {
        try {
            m.invoke(target, args);
        } catch (IllegalAccessException | InvocationTargetException e) {
            throw new AssertionError("呼叫 " + m.getName() + " 失敗", e);
        }
    }

    @Test
    @DisplayName("A3 DuplicatePaymentService：exchange=payment.events，且**不**註冊 afterCommit（純 INSERT）")
    void a3_duplicatePaymentService() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        PaymentTransaction tx = mock(PaymentTransaction.class);
        when(tx.tradeNo()).thenReturn("T-dup");

        DuplicatePaymentService svc = new DuplicatePaymentService(
                mock(io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository.class),
                mock(io.github.will413028.hawkr.payment.domain.service.PaymentDomainService.class),
                mock(io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient.class),
                mapper, new ObjectMapper());

        TransactionSynchronizationManager.initSynchronization();
        invoke(privateMethod(DuplicatePaymentService.class, "publishRescueRefundInitiated",
                PaymentTransaction.class, RescueReason.class), svc, tx, RescueReason.RESCUE_DUPLICATE_PAYMENT);

        OutboxEventEntity row = captureInsert(mapper);
        assertEquals(PAYMENT_EXCHANGE, row.getExchange());
        assertEquals("payment.rescue.refund_initiated", row.getRoutingKey());
        assertTrue(TransactionSynchronizationManager.getSynchronizations().isEmpty(),
                "A3 原本就是純 INSERT + scheduler 派送；genericize 不得替它加上 afterCommit 即時投遞");
    }

    @Test
    @DisplayName("A4 RescueRefundConsumer：exchange=payment.events，且**不**註冊 afterCommit（純 INSERT）")
    void a4_rescueRefundConsumer() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        PaymentTransaction tx = mock(PaymentTransaction.class);
        when(tx.tradeNo()).thenReturn("T-rescue");

        RescueRefundConsumer consumer = new RescueRefundConsumer(
                mock(io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository.class),
                mock(io.github.will413028.hawkr.payuni.payment.PaymentClient.class),
                new ObjectMapper(), mapper,
                mock(org.springframework.transaction.support.TransactionTemplate.class));

        TransactionSynchronizationManager.initSynchronization();
        invoke(privateMethod(RescueRefundConsumer.class, "insertRescueCompletedOutbox",
                        PaymentTransaction.class, LocalDateTime.class, String.class, String.class),
                consumer, tx, NOW, "{}", "9101");

        OutboxEventEntity row = captureInsert(mapper);
        assertEquals(PAYMENT_EXCHANGE, row.getExchange());
        assertEquals("payment.rescue.refund_completed", row.getRoutingKey());
        assertTrue(TransactionSynchronizationManager.getSynchronizations().isEmpty(),
                "A4 原本就是純 INSERT + scheduler 派送；genericize 不得替它加上 afterCommit 即時投遞");
    }
}

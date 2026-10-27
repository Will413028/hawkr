package io.github.will413028.hawkr.aftersales.mq;

import com.baomidou.mybatisplus.core.MybatisConfiguration;
import com.baomidou.mybatisplus.core.metadata.TableInfoHelper;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import com.rabbitmq.client.Channel;
import org.apache.ibatis.builder.MapperBuilderAssistant;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.nio.charset.StandardCharsets;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 09 §8.1.1 / §5.4.3 Tx 載體規定回歸測試。
 *
 * <p>驗證 {@link RefundResultConsumer#onRefundResult} 對每個 handler 都透過
 * {@link org.springframework.transaction.support.TransactionTemplate} 程式式包覆，
 * 而非 self-invocation `@Transactional protected`（CGLIB proxy 不攔截，會讓 annotation 失效）。
 *
 * <p>用 in-memory {@link TestTxManager} 計數 begin / commit / rollback，
 * 確保 caller chain 真的有走 tx 包覆，並在 mid-flight throw 時觸發 rollback 而非 commit。
 */
class RefundResultConsumerTest {

    private final RefundTaskMapper refundTaskMapper = mock(RefundTaskMapper.class);
    private final AfterSaleCaseMapper caseMapper = mock(AfterSaleCaseMapper.class);
    private final SellerCancelRequestMapper cancelMapper = mock(SellerCancelRequestMapper.class);
    private final AfterSaleAuditService auditService = mock(AfterSaleAuditService.class);
    private final AfterSalesEventPublisherImpl eventPublisher = mock(AfterSalesEventPublisherImpl.class);
    private final io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler atmRefundReconciler =
            mock(io.github.will413028.hawkr.aftersales.service.AtmRefundReconciler.class);
    private final Channel channel = mock(Channel.class);
    private TestTxManager txManager;
    private RefundResultConsumer consumer;

    /**
     * 初始化 MyBatis-Plus lambda cache —— LambdaUpdateWrapper.set(Entity::getter, value)
     * 需要 TableInfo 已註冊，否則拋 "can not find lambda cache for this entity"。
     * 單元測試無 Spring context，必須手動 init 一次。
     */
    @BeforeAll
    static void initLambdaCache() {
        MybatisConfiguration config = new MybatisConfiguration();
        MapperBuilderAssistant assistant = new MapperBuilderAssistant(config, "");
        TableInfoHelper.initTableInfo(assistant, RefundTaskEntity.class);
    }

    @BeforeEach
    void setUp() {
        txManager = new TestTxManager();
        consumer = new RefundResultConsumer(refundTaskMapper, caseMapper, cancelMapper,
                auditService, eventPublisher, atmRefundReconciler, txManager);
    }

    /**
     * Happy path：ATM webhook 成功 → handler 須在 tx 內執行 → commit。
     * 同時驗證 begin >= 1（tx 確實開啟，未走 self-invocation bypass）。
     */
    @Test
    void atmWebhookSucceededRunsHandlerInsideTransactionAndCommits() throws Exception {
        RefundTaskEntity task = buildAtmTask(WorkerTaskStatus.AWAITING_WEBHOOK.value());
        when(refundTaskMapper.selectOne(any())).thenReturn(task);
        when(refundTaskMapper.selectById(task.getId())).thenReturn(task);

        Message msg = buildMessage(MqConstants.ROUTING_KEY_REFUND_SUCCEEDED,
                "{\"refundNo\":\"RFD001\",\"tradeNo\":\"TRD001\",\"payuniStatus\":\"1\","
                        + "\"payuniMessage\":\"OK\",\"bankDate\":\"2026-06-20\"}");

        consumer.onRefundResult(msg, channel, 1L);

        assertThat(txManager.beginCount)
                .as("tx must be opened via TransactionTemplate (proves self-invocation bypass is gone)")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.commitCount)
                .as("happy path 必須 commit")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.rollbackCount)
                .as("無 throw，禁止 rollback")
                .isEqualTo(0);
        verify(refundTaskMapper).update(any(), any());
        verify(caseMapper).closeIfNotClosed(any(), any());
        verify(channel).basicAck(1L, false);
    }

    /**
     * 09 §8.1.1 核心保證：handler body 中段 throw 必須觸發 rollback（不是 commit），
     * 避免 refund_tasks 已 UPDATE 但 audit / event 未補的破壞性副作用（INV-3 / INV-4 違反）。
     *
     * <p>本 test 透過讓 caseMapper.closeIfNotClosed throw 模擬 mid-flight 例外。
     * 若 self-invocation regression 重現（handler 不被 TransactionTemplate 包覆）→
     * rollbackCount == 0 且 refundTask UPDATE 已落地不會回滾。
     */
    @Test
    void midFlightThrowTriggersRollbackNotCommit() throws Exception {
        RefundTaskEntity task = buildAtmTask(WorkerTaskStatus.AWAITING_WEBHOOK.value());
        when(refundTaskMapper.selectOne(any())).thenReturn(task);
        when(refundTaskMapper.selectById(task.getId())).thenReturn(task);
        doThrow(new RuntimeException("simulated mid-flight failure"))
                .when(caseMapper).closeIfNotClosed(any(), any());

        Message msg = buildMessage(MqConstants.ROUTING_KEY_REFUND_SUCCEEDED,
                "{\"refundNo\":\"RFD002\",\"tradeNo\":\"TRD002\",\"payuniStatus\":\"1\","
                        + "\"payuniMessage\":\"OK\",\"bankDate\":\"2026-06-20\"}");

        // onRefundResult catches all Exception → MqErrorHandler.handleFailure；不會拋出。
        consumer.onRefundResult(msg, channel, 2L);

        assertThat(txManager.beginCount)
                .as("tx must be opened — 否則 self-invocation bypass 重現")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.rollbackCount)
                .as("mid-flight throw 必須 rollback，不可 commit")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.commitCount)
                .as("throw 不應 commit")
                .isEqualTo(0);
    }

    /**
     * 失敗分支：ATM FAILED → revertAtmTaskToAwaitingPage 也必須 in-tx。
     */
    @Test
    void atmFailedRevertRunsInsideTransaction() throws Exception {
        RefundTaskEntity task = buildAtmTask(WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value());
        when(refundTaskMapper.selectOne(any())).thenReturn(task);
        when(refundTaskMapper.revertFromAnyAwaitingToAwaitingPage(any(), any(), any())).thenReturn(1);

        Message msg = buildMessage(MqConstants.ROUTING_KEY_REFUND_FAILED,
                "{\"refundNo\":\"RFD003\",\"tradeNo\":\"TRD003\",\"payuniStatus\":\"2\","
                        + "\"payuniMessage\":\"BANK_REJECT\"}");

        consumer.onRefundResult(msg, channel, 3L);

        assertThat(txManager.beginCount).isGreaterThanOrEqualTo(1);
        assertThat(txManager.commitCount).isGreaterThanOrEqualTo(1);
        assertThat(txManager.rollbackCount).isEqualTo(0);
        verify(refundTaskMapper, atLeastOnce()).revertFromAnyAwaitingToAwaitingPage(any(), any(), any());
        verify(auditService).log(any(), any(), any(), any(), any(), any());
    }

    /**
     * 冪等 skip 路徑（task 已是終態）不應開 tx — 避免空 tx round-trip。
     */
    @Test
    void terminalTaskSkipsTransactionEntirely() throws Exception {
        RefundTaskEntity task = buildAtmTask(WorkerTaskStatus.DONE.value());
        when(refundTaskMapper.selectOne(any())).thenReturn(task);

        Message msg = buildMessage(MqConstants.ROUTING_KEY_REFUND_SUCCEEDED,
                "{\"refundNo\":\"RFD004\",\"tradeNo\":\"TRD004\",\"bankDate\":\"2026-06-20\"}");

        consumer.onRefundResult(msg, channel, 4L);

        assertThat(txManager.beginCount).as("已終態 skip，禁止開 tx").isEqualTo(0);
        verify(refundTaskMapper, never()).update(any(), any());
        verify(channel).basicAck(4L, false);
    }

    // ==================== 副作用失敗無訊號修復（出路 4） ====================

    /**
     * 核心保證：{@code aftersales.refund.completed} 送不出去時，短 tx 必須 rollback。
     *
     * <p>不 rollback 的話：refund_task 已 DONE、case 已 CLOSED(50)，重投時
     * {@code isTerminalState} 直接 ack skip、{@code closeIfNotClosed} 回 0
     * ⇒ 錢已退但訂單永遠不會推進 FULLY_REFUNDED/PARTIALLY_REFUNDED，且零告警。
     *
     * <p>鑑別力：把 {@code tryPublishRefundCompleted} 換回吞例外的 {@code publishRefundCompleted}
     * （或把 {@code if (!published) throw} 拿掉）⇒ rollbackCount 變 0、commitCount 變 1 ⇒ 轉紅。
     */
    @Test
    void refundCompletedPublishFailure_rollsBackSoStateDoesNotStickWithoutEvent() throws Exception {
        RefundTaskEntity task = buildAtmTask(WorkerTaskStatus.AWAITING_WEBHOOK.value());
        task.setRefundOutcome(AfterSalePlatformDecisionOutcome.FULL_REFUND.value());
        when(refundTaskMapper.selectOne(any())).thenReturn(task);
        when(refundTaskMapper.selectById(task.getId())).thenReturn(task);
        when(caseMapper.closeIfNotClosed(any(), any())).thenReturn(1);
        when(eventPublisher.resolveEventPrincipalsBestEffort(any(), any()))
                .thenReturn(new EventPrincipals(4001L, 5001L));
        when(eventPublisher.tryPublishRefundCompleted(
                any(), any(), any(), any(), any(), any(), any(), any(), any()))
                .thenReturn(false);

        Message msg = buildMessage(MqConstants.ROUTING_KEY_REFUND_SUCCEEDED,
                "{\"refundNo\":\"RFD005\",\"tradeNo\":\"TRD005\",\"payuniStatus\":\"1\","
                        + "\"payuniMessage\":\"OK\",\"bankDate\":\"2026-06-20\"}");

        consumer.onRefundResult(msg, channel, 5L);

        assertThat(txManager.rollbackCount)
                .as("publish 失敗必須 rollback —— 否則 DONE/CLOSED 已落地而事件永久遺失")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.commitCount)
                .as("publish 失敗禁止 commit")
                .isEqualTo(0);
        // 失敗走 MqErrorHandler：ack 原訊息後 republish 到同一 queue（retry-count+1），下一輪重來
        verify(channel).basicPublish(any(), any(), any(), any());
    }

    /**
     * 出路 4 硬性邊界：身分反解（兩次無 deadline 的 gRPC）必須在 <b>tx 之外</b>。
     *
     * <p>鑑別力：把 {@code resolveCloseContext(...)} 搬回 txTemplate 內（或改回在 publisher 內部
     * 反解）⇒ 記錄到的 beginCount 會 ≥ 1 ⇒ 轉紅。
     */
    @Test
    void principalResolutionHappensBeforeTransactionOpens() throws Exception {
        RefundTaskEntity task = buildAtmTask(WorkerTaskStatus.AWAITING_WEBHOOK.value());
        task.setRefundOutcome(AfterSalePlatformDecisionOutcome.FULL_REFUND.value());
        when(refundTaskMapper.selectOne(any())).thenReturn(task);
        when(refundTaskMapper.selectById(task.getId())).thenReturn(task);
        when(caseMapper.closeIfNotClosed(any(), any())).thenReturn(1);
        when(eventPublisher.tryPublishRefundCompleted(
                any(), any(), any(), any(), any(), any(), any(), any(), any()))
                .thenReturn(true);

        int[] beginCountAtResolve = {-1};
        when(eventPublisher.resolveEventPrincipalsBestEffort(any(), any())).thenAnswer(inv -> {
            beginCountAtResolve[0] = txManager.beginCount;
            return new EventPrincipals(4001L, 5001L);
        });

        Message msg = buildMessage(MqConstants.ROUTING_KEY_REFUND_SUCCEEDED,
                "{\"refundNo\":\"RFD006\",\"tradeNo\":\"TRD006\",\"payuniStatus\":\"1\","
                        + "\"payuniMessage\":\"OK\",\"bankDate\":\"2026-06-20\"}");

        consumer.onRefundResult(msg, channel, 6L);

        assertThat(beginCountAtResolve[0])
                .as("身分反解必須發生在開 tx 之前（tx 內不得有任何網路呼叫）")
                .isEqualTo(0);
        assertThat(txManager.commitCount).as("publish 成功必須 commit").isGreaterThanOrEqualTo(1);
        verify(channel).basicAck(6L, false);
    }

    // ---------- helpers ----------

    private RefundTaskEntity buildAtmTask(String status) {
        RefundTaskEntity task = new RefundTaskEntity();
        task.setId(9001L);
        task.setCaseNo("C0001");
        task.setOrderNo("O0001");
        task.setPaymentMethod(PaymentMethod.ATM.value());
        task.setStatus(status);
        task.setRefundAmount(680L);
        return task;
    }

    private Message buildMessage(String routingKey, String json) {
        MessageProperties props = new MessageProperties();
        props.setReceivedRoutingKey(routingKey);
        return new Message(json.getBytes(StandardCharsets.UTF_8), props);
    }

    /**
     * In-memory PlatformTransactionManager：不真的開 DB 連線，只計數 begin / commit / rollback，
     * 讓 caller code path 走完 TransactionTemplate.execute 並斷言 tx 行為。
     */
    private static class TestTxManager implements PlatformTransactionManager {
        int beginCount = 0;
        int commitCount = 0;
        int rollbackCount = 0;

        @Override
        public TransactionStatus getTransaction(TransactionDefinition definition) {
            beginCount++;
            return new SimpleTransactionStatus(true);
        }

        @Override
        public void commit(TransactionStatus status) {
            commitCount++;
        }

        @Override
        public void rollback(TransactionStatus status) {
            rollbackCount++;
        }
    }
}

package io.github.will413028.hawkr.aftersales.scheduler;

import com.baomidou.mybatisplus.core.MybatisConfiguration;
import com.baomidou.mybatisplus.core.metadata.TableInfoHelper;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.PaymentGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.aftersales.mapper.SellerCancelRequestMapper;
import io.github.will413028.hawkr.aftersales.mq.AfterSalesEventPublisherImpl;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.payment.grpc.CreateRefundResponse;
import io.github.will413028.hawkr.payment.grpc.RefundDetail;
import org.apache.ibatis.builder.MapperBuilderAssistant;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.util.List;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * issue #2294 C1：{@code RefundTaskWorker} 的 claim 從「{@code SET status=PROCESSING} 永久旗標」
 * 改成「推 {@code next_retry_time} 的 lease」。
 *
 * <p>本 worker 在本 issue 之前<b>零測試</b>，claim 閘門的兩條分支都是新覆蓋：
 * <ul>
 *   <li>claim=1（搶到租約）→ 才呼叫 payment {@code CreateRefund}</li>
 *   <li>claim=0（租約由他人持有）→ <b>絕不</b>呼叫 {@code CreateRefund}（真實退款是不可逆副作用）</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class RefundTaskWorkerClaimTest {

    @Mock private RefundTaskMapper refundTaskMapper;
    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private SellerCancelRequestMapper cancelMapper;
    @Mock private PaymentGrpcClient paymentGrpcClient;
    @Mock private AfterSaleAuditService auditService;
    @Mock private AfterSalesEventPublisherImpl eventPublisher;

    private RefundTaskWorker worker;

    @BeforeAll
    static void initLambdaCache() {
        // LambdaUpdateWrapper 需要 MyBatis-Plus 的 lambda column cache 才能解析 method reference
        MybatisConfiguration config = new MybatisConfiguration();
        MapperBuilderAssistant assistant = new MapperBuilderAssistant(config, "");
        TableInfoHelper.initTableInfo(assistant, RefundTaskEntity.class);
    }

    @BeforeEach
    void setUp() {
        worker = new RefundTaskWorker(refundTaskMapper, caseMapper, cancelMapper,
                paymentGrpcClient, auditService, eventPublisher);
        when(refundTaskMapper.selectList(any())).thenReturn(List.of(pendingTask()));
        when(refundTaskMapper.selectById(1001L)).thenReturn(pendingTask());
    }

    @Test
    void claimSucceeds_callsCreateRefund() {
        when(refundTaskMapper.update(any(), any())).thenReturn(1);
        when(paymentGrpcClient.createRefund(anyString(), anyString(), anyString(), anyString()))
                .thenReturn(CreateRefundResponse.newBuilder()
                        .setCode(0)
                        .setData(RefundDetail.newBuilder().setRefundNo("RF-1").build())
                        .build());

        worker.processRefundTasks();

        verify(paymentGrpcClient).createRefund(eq("TXN-1"), anyString(), anyString(), anyString());
    }

    @Test
    void claimLost_doesNotCallCreateRefund() {
        // 租約由另一個 pod 持有 ⇒ 本 pod 不得打 PayUni。
        // 沒有這一支斷言，claim 閘門形同虛設（Mockito int 預設 0 只會讓上面那支紅）。
        when(refundTaskMapper.update(any(), any())).thenReturn(0);

        worker.processRefundTasks();

        verify(paymentGrpcClient, never()).createRefund(anyString(), anyString(), anyString(), anyString());
    }

    /**
     * issue #2294 P1-1：租約的<b>下界</b>不變式 —— 《多副本排程搶佔規範》§三第 5 步
     * 「租約長度必須 &gt; 單筆工作的最壞耗時」。
     *
     * <p>claim 之後、下一次狀態寫入之前唯一的長工作是 {@code CreateRefund}，其最壞耗時由
     * {@link PaymentGrpcClient#CREATE_REFUND_DEADLINE_SEC} 封頂。兩個常數任一被單獨調整
     * （典型誤修：「租約 30 秒好像太長，調短一點」／「deadline 拉長一點比較保險」）
     * 就會讓租約短於最壞耗時 ⇒ 原持有者還在打 PayUni、租約已到期、下一 tick 直接重做同一筆
     * ⇒ <b>搶佔等於沒有</b>。這條斷言是那個退化的唯一機械攔截點。
     */
    @Test
    void claimLease_mustExceedCreateRefundDeadline() {
        org.junit.jupiter.api.Assertions.assertTrue(
                RefundTaskWorker.CLAIM_LEASE_SEC > PaymentGrpcClient.CREATE_REFUND_DEADLINE_SEC,
                "租約(" + RefundTaskWorker.CLAIM_LEASE_SEC + "s) 必須 > CreateRefund deadline("
                        + PaymentGrpcClient.CREATE_REFUND_DEADLINE_SEC + "s)，否則租約到期時"
                        + "原持有者仍在打 PayUni，等於沒有搶佔");
    }

    private RefundTaskEntity pendingTask() {
        RefundTaskEntity t = new RefundTaskEntity();
        t.setId(1001L);
        t.setCaseNo("A001");
        t.setOrderNo("O100");
        t.setTransactionId("TXN-1");
        t.setRefundAmount(500L);
        t.setPaymentMethod(PaymentMethod.CREDIT_CARD.value());
        t.setRefundOutcome(AfterSalePlatformDecisionOutcome.FULL_REFUND.value());
        t.setStatus(WorkerTaskStatus.PENDING.value());
        t.setRetryCount(0);
        return t;
    }
}

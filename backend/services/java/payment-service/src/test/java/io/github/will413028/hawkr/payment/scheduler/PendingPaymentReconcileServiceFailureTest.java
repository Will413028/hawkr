package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.application.service.DuplicatePaymentService;
import io.github.will413028.hawkr.payment.application.service.PaymentApplicationService;
import io.github.will413028.hawkr.payment.application.service.RefundApplicationService;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.function.Consumer;

import java.time.LocalDateTime;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * 對應設計書 §5.7.1：reconcile 失敗保護
 *
 * 不論 PayUni 查詢 / 退款 / 任何 downstream 失敗，
 * scheduler 都必須更新 last_reconciled_at 以避免「同一筆訂單在每個 tick 重複嘗試」
 * 的熱循環，並且這個保護必須 robust 到「rescue 路徑拋例外、PayUni 連續失敗、
 * audit log 寫失敗」都不能讓 finally 區塊被跳過。
 */
@ExtendWith(MockitoExtension.class)
class PendingPaymentReconcileServiceFailureTest {

    @Mock PaymentTransactionRepository transactionRepository;
    @Mock PaymentApplicationService paymentApplicationService;
    @Mock RefundApplicationService refundApplicationService;
    @Mock PaymentClient paymentClient;
    @Mock PaymentOutboxWriter outboxWriter;
    @Mock PaymentLogMapper paymentLogMapper;
    @Mock DuplicatePaymentService duplicatePaymentService;
    @Mock TransactionTemplate transactionTemplate;

    PendingPaymentReconcileService service;

    @BeforeEach
    void setUp() {
        // Stub txTemplate.executeWithoutResult(Consumer<TransactionStatus>) → 同步執行 lambda
        // 讓 tx 包覆對單元測試呈現「無 tx」直通行為，因 mock 環境無真實 DataSource
        lenient().doAnswer(inv -> {
            Consumer<TransactionStatus> cb = inv.getArgument(0);
            cb.accept(null);
            return null;
        }).when(transactionTemplate).executeWithoutResult(any());

        service = new PendingPaymentReconcileService(
                transactionRepository,
                paymentApplicationService,
                refundApplicationService,
                paymentClient,
                outboxWriter,
                paymentLogMapper,
                duplicatePaymentService,
                transactionTemplate
        );
    }

    private PaymentTransaction expiredAtm(String tradeNo) {
        return PaymentTransaction.builder()
                .id(1L)
                .tradeNo(tradeNo)
                .orderId(tradeNo)
                .merId("HAWK9")
                .buyerId("2001")
                .paymentType(PaymentMethod.ATM)
                .amount(100)
                .status(PaymentTransactionStatus.EXPIRED)
                .createdAt(LocalDateTime.now().minusMinutes(10))
                .updatedAt(LocalDateTime.now().minusMinutes(10))
                .expireAt(LocalDateTime.now().minusMinutes(1))
                .version(0)
                .build();
    }

    private PaymentTransaction initAtm(String tradeNo) {
        return PaymentTransaction.builder()
                .id(2L)
                .tradeNo(tradeNo)
                .orderId(tradeNo)
                .merId("HAWK9")
                .buyerId("2001")
                .paymentType(PaymentMethod.ATM)
                .amount(100)
                .status(PaymentTransactionStatus.INIT)
                .createdAt(LocalDateTime.now().minusMinutes(10))
                .updatedAt(LocalDateTime.now().minusMinutes(10))
                .version(0)
                .build();
    }

    @Test
    @DisplayName("PayUni 查詢失敗時回 NO_OP，且不得自行 stamp last_reconciled_at（#2294 H6：退避 stamp 已上移為 Scheduler 的 claim 閘門）")
    void payUniFailure_returnsNoOpAndDoesNotStamp() {
        var candidate = expiredAtm("T_FAIL_1");

        when(paymentClient.queryTransaction(eq("HAWK9"), eq("T_FAIL_1")))
                .thenReturn(new PayUniApiResponse("ERROR", "某個錯誤", null));

        var outcome = service.reconcileSingle(candidate);

        assertEquals(PendingPaymentReconcileService.ReconcileOutcome.NO_OP, outcome);
        verify(refundApplicationService, never()).createRescueRefund(any(), anyLong(), any(), any());
        // issue #2294 H6：退避 stamp 已上移為 Scheduler 的 claimForReconcile 閘門；
        // reconcileSingle 不得再自行寫 last_reconciled_at（那會洗掉 claim 的 CAS 保證）。
        verify(transactionRepository, never()).claimForReconcile(any(), any(), any());
    }

    @Test
    @DisplayName("paymentClient 直接拋例外時回 FAILED，且不得自行 stamp last_reconciled_at（#2294 H6）")
    void payUniThrows_returnsFailedAndDoesNotStamp() {
        var candidate = expiredAtm("T_FAIL_2");

        when(paymentClient.queryTransaction(eq("HAWK9"), eq("T_FAIL_2")))
                .thenThrow(new RuntimeException("connection refused"));

        var outcome = service.reconcileSingle(candidate);

        assertEquals(PendingPaymentReconcileService.ReconcileOutcome.FAILED, outcome);
        // issue #2294 H6：stamp 已上移為 Scheduler 的 claim 閘門，本方法不得再自行寫入
        verify(transactionRepository, never()).claimForReconcile(any(), any(), any());
    }

    @Test
    @DisplayName("INIT 路徑 syncPaymentStatus 拋例外時回 FAILED，且不得自行 stamp last_reconciled_at（#2294 H6）")
    void initPathThrows_returnsFailedAndDoesNotStamp() {
        var candidate = initAtm("T_FAIL_3");

        when(paymentApplicationService.syncPaymentStatus(eq("T_FAIL_3")))
                .thenThrow(new RuntimeException("downstream timeout"));

        var outcome = service.reconcileSingle(candidate);

        assertEquals(PendingPaymentReconcileService.ReconcileOutcome.FAILED, outcome);
        // issue #2294 H6：stamp 已上移為 Scheduler 的 claim 閘門，本方法不得再自行寫入
        verify(transactionRepository, never()).claimForReconcile(any(), any(), any());
    }

    @Test
    @DisplayName("連續多筆都失敗時彼此隔離：每一筆各自回 outcome，且沒有任何一筆自行 stamp（#2294 H6）")
    void multipleFailures_areIsolatedAndDoNotStamp() {
        var c1 = expiredAtm("T_BATCH_1");
        var c2 = expiredAtm("T_BATCH_2");
        var c3 = expiredAtm("T_BATCH_3");

        when(paymentClient.queryTransaction(eq("HAWK9"), anyString()))
                .thenThrow(new RuntimeException("rate limited"));

        service.reconcileSingle(c1);
        service.reconcileSingle(c2);
        service.reconcileSingle(c3);

        // issue #2294 H6：三筆都不得自行 stamp（stamp 已上移為 Scheduler 的 claim 閘門）
        verify(transactionRepository, never()).claimForReconcile(any(), any(), any());
    }

    @Test
    @DisplayName("rescue 事件 outbox 寫入失敗必須傳播使 tx rollback（markRescued 不得孤兒 commit）")
    void rescueOutboxWriteFailure_propagatesSoTxRollsBack() {
        // 原子化回歸守門：publishRescueEvents 改走 outbox 後不得吞例外。
        // 若 outboxWriter.enqueueAndDispatch 失敗（例：payload 序列化錯），例外必須傳播出
        // txTemplate 使其 rollback → markRescued 一併回滾 → rescued_at 不落地 → 下次 reconcile 重掃重試。
        // 一旦被吞：rescued_at 已 commit 但事件遺失 → candidate SQL 永久排除該 row → status 卡 EXPIRED。
        var candidate = expiredAtm("T_ATOMIC");

        when(paymentClient.queryTransaction(eq("HAWK9"), eq("T_ATOMIC")))
                .thenReturn(new PayUniApiResponse(
                        "SUCCESS",
                        "查詢成功",
                        Map.of("Result[0][TradeStatus]", "1")  // 1 = 已付款 → 觸發 rescue
                ));
        when(refundApplicationService.createRescueRefund(eq("T_ATOMIC"), eq(100L), any(), any()))
                .thenReturn(new RefundApplicationService.RescueRefundResult("R_ATOMIC", false, true, "SUCCEEDED"));
        doThrow(new IllegalStateException("outbox payload 序列化失敗"))
                .when(outboxWriter).enqueueAndDispatch(any());

        var outcome = service.reconcileSingle(candidate);

        // outbox 失敗傳播 → reconcileSingle catch → FAILED（未被 publishRescueEvents 內部吞掉當 SUCCESS）
        assertEquals(PendingPaymentReconcileService.ReconcileOutcome.FAILED, outcome);
        // markRescued 在同一 lambda 內先於 outbox 執行，實務上會隨 tx rollback（此處僅證其確實在 tx 內被嘗試）
        verify(transactionRepository, times(1)).markRescued(eq("T_ATOMIC"), any(), any());
        verify(outboxWriter, atLeastOnce()).enqueueAndDispatch(any());
        // issue #2294 H6：stamp 已上移為 Scheduler 的 claim 閘門，本方法不得再自行寫入
        verify(transactionRepository, never()).claimForReconcile(any(), any(), any());
    }

    @Test
    @DisplayName("PayUni 回 SUCCESS 但 TradeStatus 非 1（非衝突）時不應觸發 rescue")
    void payUniSuccessButNotPaid_skipsRescue() {
        var candidate = expiredAtm("T_NO_RESCUE");

        when(paymentClient.queryTransaction(eq("HAWK9"), eq("T_NO_RESCUE")))
                .thenReturn(new PayUniApiResponse(
                        "SUCCESS",
                        "查詢成功",
                        Map.of("Result[0][TradeStatus]", "9")  // 9 = 未付款
                ));

        var outcome = service.reconcileSingle(candidate);

        assertEquals(PendingPaymentReconcileService.ReconcileOutcome.NO_OP, outcome);
        verify(refundApplicationService, never()).createRescueRefund(any(), anyLong(), any(), any());
        // issue #2294 H6：stamp 已上移為 Scheduler 的 claim 閘門，本方法不得再自行寫入
        verify(transactionRepository, never()).claimForReconcile(any(), any(), any());
    }
}

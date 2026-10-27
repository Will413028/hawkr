package io.github.will413028.hawkr.payment.scheduler;

import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.payment.config.ReconcileProperties;
import io.github.will413028.hawkr.payment.domain.model.PaymentTransaction;
import io.github.will413028.hawkr.common.utils.code.PaymentTransactionStatus;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 H6：{@code PendingPaymentReconcileScheduler} 的 claim 閘門。
 *
 * <p>缺陷：原本 {@code updateLastReconciledAt} 是 {@code WHERE trade_no=?}（非 CAS）
 * 且在 {@code reconcileSingle} 的 {@code finally} 內 —— 兩個 pod 各得 affected=1，
 * <b>各自打一次 PayUni 查詢、各寫一筆 rescue audit log、各跑一次管道 B</b>
 * （三者都沒有冪等鍵），後到者再拿 unique violation 誤觸發
 * {@code PaymentReconcileConflict} 告警。
 *
 * <p>修法：閘門上移到 {@code reconcileSingle} <b>之前</b>，並加上 CAS 述詞
 * （lease on {@code last_reconciled_at}，述詞＝候選 SQL 第 4 條）。
 */
class PendingPaymentReconcileSchedulerClaimTest {

    private static PaymentTransaction candidate(String tradeNo) {
        return PaymentTransaction.builder()
                .id(1L)
                .tradeNo(tradeNo)
                .orderId(tradeNo)
                .merId("HAWK9")
                .buyerId("2001")
                .paymentType(PaymentMethod.ATM)
                .amount(100)
                .status(PaymentTransactionStatus.EXPIRED)
                .createdAt(LocalDateTime.now().minusHours(1))
                .updatedAt(LocalDateTime.now().minusHours(1))
                .version(0)
                .build();
    }

    private static ReconcileProperties props() {
        return new ReconcileProperties();
    }

    /**
     * ⚠️ 這一支是本組的鑑別點：Mockito 對 {@code int} 的預設回傳是 <b>0</b>，
     * 所以「忘記 stub claim」與「claim 真的回 0」在測試裡長得一模一樣。
     * 沒有本測試，把 claim 閘門整個拿掉不會有任何測試轉紅。
     */
    @Test
    @DisplayName("claim 回 0（另一個 pod 已認領）→ 連 reconcileSingle 都不進，PayUni 查詢/audit/管道 B 全不發生")
    void claimLost_doesNotEnterReconcileSingle() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PendingPaymentReconcileService service = mock(PendingPaymentReconcileService.class);
        when(repo.findReconcileCandidates(any(), any(), any(), anyInt()))
                .thenReturn(List.of(candidate("T_CLAIM_LOST")));
        when(repo.claimForReconcile(eq("T_CLAIM_LOST"), any(), any())).thenReturn(0);

        new PendingPaymentReconcileScheduler(repo, service, props(), new SimpleMeterRegistry())
                .runReconcile();

        verifyNoInteractions(service);
    }

    @Test
    @DisplayName("claim 回 1 → 才進 reconcileSingle，且 claim 嚴格先於 reconcileSingle（InOrder）")
    void claimWon_entersReconcileSingleAfterClaim() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PendingPaymentReconcileService service = mock(PendingPaymentReconcileService.class);
        PaymentTransaction c = candidate("T_CLAIM_WON");
        when(repo.findReconcileCandidates(any(), any(), any(), anyInt())).thenReturn(List.of(c));
        when(repo.claimForReconcile(eq("T_CLAIM_WON"), any(), any())).thenReturn(1);
        when(service.reconcileSingle(any()))
                .thenReturn(PendingPaymentReconcileService.ReconcileOutcome.NO_OP);

        new PendingPaymentReconcileScheduler(repo, service, props(), new SimpleMeterRegistry())
                .runReconcile();

        InOrder order = inOrder(repo, service);
        order.verify(repo).claimForReconcile(eq("T_CLAIM_WON"), any(), any());
        order.verify(service).reconcileSingle(c);
    }

    /**
     * claim 的兩個時間參數必須是「租約語意」：stamp 寫 {@code now}，
     * CAS 述詞用 {@code retryBefore = now - retryIntervalMinutes}（＝候選 SQL 第 4 條）。
     * 因此 {@code retryBefore} 必定早於 {@code now} —— 這正是「租約會過期、該列會回來」的來源。
     */
    @Test
    @DisplayName("claim 是 lease 不是永久旗標：retryBefore 嚴格早於 stamp 的 now，退避間隔到期該列自動回候選")
    void claimUsesLeaseSemantics() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PendingPaymentReconcileService service = mock(PendingPaymentReconcileService.class);
        when(repo.findReconcileCandidates(any(), any(), any(), anyInt()))
                .thenReturn(List.of(candidate("T_LEASE")));
        when(repo.claimForReconcile(eq("T_LEASE"), any(), any())).thenReturn(0);

        ReconcileProperties properties = props();
        new PendingPaymentReconcileScheduler(repo, service, properties, new SimpleMeterRegistry())
                .runReconcile();

        ArgumentCaptor<LocalDateTime> nowCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> retryBeforeCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo).claimForReconcile(eq("T_LEASE"), nowCaptor.capture(), retryBeforeCaptor.capture());

        assertFalse(retryBeforeCaptor.getValue().isAfter(nowCaptor.getValue()),
                "retryBefore 必須早於（或等於）stamp 的 now —— 兩者相等或倒置代表租約永不過期，"
                        + "lease 會退化成永久旗標");
        assertFalse(
                retryBeforeCaptor.getValue()
                        .isAfter(nowCaptor.getValue().minusMinutes(properties.getRetryIntervalMinutes() - 1)),
                "retryBefore 必須約等於 now - retryIntervalMinutes（租約長度＝退避間隔）");
        verify(repo, never()).findById(any());
    }

    /**
     * issue #2294 <b>R4-P1-2</b>：stamp（＝租約基準）必須在<b>迴圈內</b>取，不能用 tick 起點。
     *
     * <p>本排程的有效租約 = {@code stamp + retryIntervalMinutes}（預設 5 分）。
     * 單筆最壞耗時是一次 PayUni 同步查詢（{@code PayUniHttpClient} connect/write/read 各 30 秒
     * ⇒ 90 秒）＋ 可能一次退款；{@code batchSize} 預設 <b>50</b>
     * ⇒ 批次上界 ≈ <b>75 分鐘 ≫ 5 分鐘租約</b>。拿 tick 起點當基準時，跑到第 4 筆之後
     * 租約就已經過期 ⇒ 另一個 pod 合法搶走同一筆，而 {@code reconcileSingle} 內的
     * PayUni 查詢／rescue audit INSERT／管道 B 重複付款處理<b>三者皆無冪等鍵</b>。
     *
     * <p><b>鑑別方式</b>：讓第一筆 {@code reconcileSingle} 耗掉一段可觀測的時間，
     * 再比較兩筆的 stamp。基準在迴圈內 ⇒ 第二筆嚴格晚於第一筆；tick 起點 ⇒ 兩者<b>完全相等</b>。
     */
    @Test
    @DisplayName("R4-P1-2：stamp 取 claim 當下——後一筆的 stamp 嚴格晚於前一筆（共用 tick 起點會轉紅）")
    void leaseBaselineIsTakenPerItemNotPerTick() {
        PaymentTransactionRepository repo = mock(PaymentTransactionRepository.class);
        PendingPaymentReconcileService service = mock(PendingPaymentReconcileService.class);
        PaymentTransaction first = candidate("T_SLOW");
        PaymentTransaction second = candidate("T_NEXT");
        when(repo.findReconcileCandidates(any(), any(), any(), anyInt()))
                .thenReturn(List.of(first, second));
        when(repo.claimForReconcile(any(), any(), any())).thenReturn(1);
        when(service.reconcileSingle(any())).thenAnswer(inv -> {
            Thread.sleep(40L);
            return PendingPaymentReconcileService.ReconcileOutcome.NO_OP;
        });

        ReconcileProperties properties = props();
        properties.setPerItemSleepMs(0L); // 排除「對 PayUni 友善」的 sleep，讓鑑別點只來自單筆工作耗時
        new PendingPaymentReconcileScheduler(repo, service, properties, new SimpleMeterRegistry())
                .runReconcile();

        ArgumentCaptor<LocalDateTime> stamps = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo, org.mockito.Mockito.times(2))
                .claimForReconcile(any(), stamps.capture(), any());
        long deltaMs = java.time.Duration
                .between(stamps.getAllValues().get(0), stamps.getAllValues().get(1)).toMillis();

        assertTrue(deltaMs >= 20L,
                "第二筆的 stamp 必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                        + "兩者相等 ⇒ 租約基準是 tick 起點 ⇒ batchSize=50 × 單筆最壞 90 秒的批次"
                        + "會讓後段項目在 claim 當下租約就已過期。");
    }
}

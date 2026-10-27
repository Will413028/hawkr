package io.github.will413028.hawkr.order.scheduler;

import io.github.will413028.hawkr.order.config.OrderConfig;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 H4：{@code ShipmentCreationBackstopScheduler} 的兩道 lease 閘門。
 *
 * <p><b>缺陷 A（多 pod）</b>：{@code bumpShipmentCreateAttempt} 原是
 * {@code WHERE order_no=?} 純 PK 更新 ⇒ 兩個 pod 各得 affected=1、各 publish 一次
 * {@code shipment.create.requested}，且 {@code attempts} 一次 tick 跳 +2。
 *
 * <p><b>缺陷 B（單 pod 也壞）</b>：give-up 分支<b>完全不寫任何狀態</b>
 * ⇒ 候選述詞四個條件全部維持成立 ⇒ 每 5 分鐘、每個 pod 各發一次「取消 + 退款」，
 * 直到訂單離開 PAID 為止。
 */
class ShipmentCreationBackstopSchedulerClaimTest {

    private static final int MAX_ATTEMPTS = 5;
    private static final long BACKOFF_MIN = 30L;

    /**
     * issue #2294 <b>R4-P1-1</b>：give-up 租約的<b>上界</b>（分）。
     *
     * <p>give-up 租約同時是「{@code publishShipmentCreationFailed} 靜默失敗後的救援窗口」
     * （{@code OrderEventPublisher.publish} 吞掉所有例外，03 §8.13 明列不改），
     * 所以它不只要滿足下界，還要滿足一個<b>業務上界</b>：買家已付款、無物流單、無取消、
     * 無退款、無告警的狀態最多能持續多久。
     *
     * <p>取本排程自己的重試節奏（{@code backoffMinutes}，預設 30 分）：give-up 是終局救援，
     * 沒有理由比非終局的重試分支更慢。曾經是 <b>1440</b>（獨立設定
     * {@code shipmentBackstopGiveUpLeaseMinutes}），相對修復前「每 tick 重發、恢復窗口 5 分」
     * 是 <b>288 倍退化</b>，且落在金流路徑上。
     */
    private static final long MAX_GIVE_UP_LEASE_MIN = 30L;

    private static OrderEntity order(String orderNo, int attempts) {
        OrderEntity o = new OrderEntity();
        o.setOrderNo(orderNo);
        o.setShipmentCreateAttempts(attempts);
        return o;
    }

    private static OrderConfig config() {
        OrderConfig c = new OrderConfig();
        c.setShipmentBackstopGraceMinutes(30);
        c.setShipmentBackstopMaxAttempts(MAX_ATTEMPTS);
        c.setShipmentBackstopBackoffMinutes((int) BACKOFF_MIN);
        return c;
    }

    /**
     * ⚠️ 鑑別點（重試分支）：Mockito 對 {@code int} 預設回 <b>0</b>，
     * 「忘記 stub claim」與「claim 真的回 0」長得一樣。沒有本測試，
     * 把閘門整個拿掉不會有任何測試轉紅。
     */
    @Test
    @DisplayName("重試分支 claim 回 0（另一個 pod 已認領）→ 不 publish shipment.create.requested")
    void retryBranch_claimLost_doesNotPublish() {
        OrderRepository repo = mock(OrderRepository.class);
        OrderEventPublisher publisher = mock(OrderEventPublisher.class);
        when(repo.findPaidOrdersWithoutShipmentPastGrace(any(), any(), anyInt()))
                .thenReturn(List.of(order("ORD_RETRY", 1)));
        when(repo.bumpShipmentCreateAttempt(eq("ORD_RETRY"), any(), any())).thenReturn(0);

        new ShipmentCreationBackstopScheduler(repo, publisher, config()).scanStuckPaidOrders();

        verifyNoInteractions(publisher);
    }

    @Test
    @DisplayName("重試分支 claim 回 1 → 才 publish，且 claim 嚴格先於 publish（InOrder）")
    void retryBranch_claimWon_publishesAfterClaim() {
        OrderRepository repo = mock(OrderRepository.class);
        OrderEventPublisher publisher = mock(OrderEventPublisher.class);
        when(repo.findPaidOrdersWithoutShipmentPastGrace(any(), any(), anyInt()))
                .thenReturn(List.of(order("ORD_RETRY", 1)));
        when(repo.bumpShipmentCreateAttempt(eq("ORD_RETRY"), any(), any())).thenReturn(1);

        new ShipmentCreationBackstopScheduler(repo, publisher, config()).scanStuckPaidOrders();

        InOrder order = inOrder(repo, publisher);
        order.verify(repo).bumpShipmentCreateAttempt(eq("ORD_RETRY"), any(), any());
        order.verify(publisher).publishShipmentCreateRequested("ORD_RETRY");
        verify(publisher, never()).publishShipmentCreationFailed(anyString());
    }

    /**
     * ⚠️ 鑑別點（give-up 分支）：舊實作這一支<b>完全沒有</b>任何寫入，
     * 所以只要沒有本測試，把 claim 拿掉就會退回「每個 tick、每個 pod 各發一次取消退款」。
     */
    @Test
    @DisplayName("give-up 分支 claim 回 0 → 不 publish shipment.creation.failed（不重複觸發取消退款）")
    void giveUpBranch_claimLost_doesNotPublish() {
        OrderRepository repo = mock(OrderRepository.class);
        OrderEventPublisher publisher = mock(OrderEventPublisher.class);
        when(repo.findPaidOrdersWithoutShipmentPastGrace(any(), any(), anyInt()))
                .thenReturn(List.of(order("ORD_GIVEUP", MAX_ATTEMPTS)));
        when(repo.claimShipmentBackstopGiveUp(eq("ORD_GIVEUP"), any(), any())).thenReturn(0);

        new ShipmentCreationBackstopScheduler(repo, publisher, config()).scanStuckPaidOrders();

        verifyNoInteractions(publisher);
        verify(repo, never()).bumpShipmentCreateAttempt(anyString(), any(), any());
    }

    @Test
    @DisplayName("give-up 分支 claim 回 1 → 才 publish shipment.creation.failed，且 claim 先於 publish")
    void giveUpBranch_claimWon_publishesAfterClaim() {
        OrderRepository repo = mock(OrderRepository.class);
        OrderEventPublisher publisher = mock(OrderEventPublisher.class);
        when(repo.findPaidOrdersWithoutShipmentPastGrace(any(), any(), anyInt()))
                .thenReturn(List.of(order("ORD_GIVEUP", MAX_ATTEMPTS)));
        when(repo.claimShipmentBackstopGiveUp(eq("ORD_GIVEUP"), any(), any())).thenReturn(1);

        new ShipmentCreationBackstopScheduler(repo, publisher, config()).scanStuckPaidOrders();

        InOrder order = inOrder(repo, publisher);
        order.verify(repo).claimShipmentBackstopGiveUp(eq("ORD_GIVEUP"), any(), any());
        order.verify(publisher).publishShipmentCreationFailed("ORD_GIVEUP");
        verify(publisher, never()).publishShipmentCreateRequested(anyString());
    }

    /**
     * give-up 分支寫的是<b>租約</b>而不是「永久放棄」旗標：
     * 若寫成永久旗標，publish 拋例外或 pod 在兩者之間崩潰 ⇒ 該訂單永遠停在 PAID、
     * 永遠沒有物流單、永遠不會被取消退款，且沒有任何告警。
     */
    @Test
    @DisplayName("兩個分支寫入的都是未來時間（lease）：重試分支 = backoffMinutes、give-up 分支 = giveUpLeaseMinutes")
    void bothBranchesWriteFutureLeases() {
        OrderRepository repo = mock(OrderRepository.class);
        OrderEventPublisher publisher = mock(OrderEventPublisher.class);
        when(repo.findPaidOrdersWithoutShipmentPastGrace(any(), any(), anyInt()))
                .thenReturn(List.of(order("ORD_RETRY", 1), order("ORD_GIVEUP", MAX_ATTEMPTS)));

        new ShipmentCreationBackstopScheduler(repo, publisher, config()).scanStuckPaidOrders();

        ArgumentCaptor<LocalDateTime> retryLease = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> retryNow = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo).bumpShipmentCreateAttempt(eq("ORD_RETRY"), retryLease.capture(), retryNow.capture());
        assertLeaseIsBackoffFromClaimInstant(retryLease.getValue(), retryNow.getValue(), "重試分支");

        ArgumentCaptor<LocalDateTime> giveUpLease = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> giveUpNow = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo).claimShipmentBackstopGiveUp(eq("ORD_GIVEUP"), giveUpLease.capture(), giveUpNow.capture());
        assertLeaseIsBackoffFromClaimInstant(giveUpLease.getValue(), giveUpNow.getValue(),
                "give-up 分支（與重試分支同值）");
        assertTrue(giveUpLease.getValue().isAfter(giveUpNow.getValue()),
                "give-up 的租約必須是未來時間——寫 NULL / 過去 / 永久旗標即回到 at-most-once");
    }

    /**
     * 租約 = <b>claim 當下</b> + backoffMinutes，所以它相對 tick 起點 {@code tickNow} 落在
     * {@code [tickNow+backoff, tickNow+backoff+TOLERANCE]} 這個半開區間，而不是精確相等
     *（R4-P1-2 之前是精確相等，因為基準就是 tick 起點）。
     * 上界留 {@code LEASE_BASELINE_TOLERANCE_MS} 是為了讓「基準被改成某個更晚的東西」
     *（例如誤用 publish 之後的時間）仍然轉紅。
     */
    private static void assertLeaseIsBackoffFromClaimInstant(LocalDateTime lease, LocalDateTime tickNow,
                                                             String branch) {
        LocalDateTime earliest = tickNow.plusMinutes(BACKOFF_MIN);
        assertTrue(!lease.isBefore(earliest),
                branch + "的租約不得早於 tickNow + backoffMinutes（實測差 "
                        + Duration.between(earliest, lease).toMillis() + " ms）");
        assertTrue(Duration.between(earliest, lease).toMillis() <= LEASE_BASELINE_TOLERANCE_MS,
                branch + "的租約 = claim 當下 + backoffMinutes，相對 tick 起點的偏移實測 "
                        + Duration.between(earliest, lease).toMillis() + " ms，超過容差 "
                        + LEASE_BASELINE_TOLERANCE_MS + " ms");
    }

    /** 見 {@link #assertLeaseIsBackoffFromClaimInstant}：per-item 基準相對 tick 起點的允許偏移。 */
    private static final long LEASE_BASELINE_TOLERANCE_MS = 5_000L;

    /**
     * issue #2294 <b>R4-P1-2</b>：租約基準必須在<b>迴圈內</b>取，不能用 tick 起點。
     *
     * <p>同批另外四支（{@code OrderOutboxPublisherScheduler}、{@code StockCompensationWorker}、
     * payment {@code PendingPaymentReconcileScheduler}、payuni-webhook {@code OutboxPublisherScheduler}）
     * 都有這一條，本支原本沒有 —— 而它的 {@code BATCH_SIZE} 是全批最大的 <b>100</b>。
     *
     * <p><b>今天不觸發，但沒有東西守著</b>：單筆工作只有一次本地 MQ publish（毫秒級），
     * {@code 100 × 毫秒 ≪ 30 分}租約，判定式（{@code batchSize × 單筆最壞耗時 > 租約}）不成立。
     * 那是<b>參數巧合不是結構保證</b>——哪天有人往 {@code handleOne} 裡加一次 gRPC
     *（例如組事件 payload 要先查訂單快照，10 秒 deadline ⇒ {@code 100 × 10s ≈ 17 分}，
     * 再加一次就破 30 分），共用 tick 起點的租約會在原持有者仍在跑時到期，
     * 另一個 pod 合法搶走同一筆 ⇒ 對已付款訂單重複觸發「取消 + 退款」。
     * <b>本測試就是那個守門人。</b>
     *
     * <p><b>鑑別方式</b>：讓第一筆的 publish 耗掉一段可觀測的時間，再比較兩筆的租約。
     * 基準在迴圈內 ⇒ 第二筆嚴格晚於第一筆；基準是 tick 起點 ⇒ 兩者<b>完全相等</b>。
     */
    @Test
    @DisplayName("R4-P1-2：租約基準取 claim 當下——後一筆的租約嚴格晚於前一筆（共用 tick 起點會轉紅）")
    void leaseBaselineIsTakenPerItemNotPerTick() {
        OrderRepository repo = mock(OrderRepository.class);
        OrderEventPublisher publisher = mock(OrderEventPublisher.class);
        when(repo.findPaidOrdersWithoutShipmentPastGrace(any(), any(), anyInt()))
                .thenReturn(List.of(order("ORD_A", 1), order("ORD_B", 1)));
        when(repo.bumpShipmentCreateAttempt(anyString(), any(), any())).thenReturn(1);
        // 模擬「單筆工作耗時」：真實世界是 publish（毫秒）＋未來可能加進來的 gRPC，
        // 這裡只需要一段可量測的時間。
        org.mockito.Mockito.doAnswer(inv -> {
            Thread.sleep(40L);
            return null;
        }).when(publisher).publishShipmentCreateRequested("ORD_A");

        new ShipmentCreationBackstopScheduler(repo, publisher, config()).scanStuckPaidOrders();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo, org.mockito.Mockito.times(2))
                .bumpShipmentCreateAttempt(anyString(), leases.capture(), any());
        LocalDateTime lease1 = leases.getAllValues().get(0);
        LocalDateTime lease2 = leases.getAllValues().get(1);

        assertTrue(Duration.between(lease1, lease2).toMillis() >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 "
                        + Duration.between(lease1, lease2).toMillis()
                        + " ms）。兩者相等 ⇒ 租約基準是 tick 起點 ⇒ 排在後面的項目在 claim 當下"
                        + "就已經吃掉前面所有項目的耗時，批次夠長時租約直接是過去時間。");
    }

    /**
     * issue #2294 <b>R4-P1-1</b> 的鑑別測試：give-up 租約的<b>上界</b>。
     *
     * <p>⚠️ 這一支釘的是「1440 不准回來」。上一支（{@code bothBranchesWriteFutureLeases}）
     * 只釘住「租約 = 某個設定值」，把設定值改成 1440 它<b>照樣綠</b>——租約是不是未來時間、
     * 是不是等於設定值，對 1440 全部成立。真正被違反的是<b>業務上界</b>：
     * give-up 租約 ＝ publish 靜默失敗後的救援窗口，1440 分 ⇒ 買家已付款、無單、無退款、
     * 無告警 24 小時。
     *
     * <p>用 {@code new OrderConfig()} 的<b>預設值</b>（＝ application.yml 的 env 預設值）
     * 而非測試自訂的 config，才驗得到「實際跑起來的那個數字」。
     */
    @Test
    @DisplayName("R4-P1-1：預設配置下 give-up 租約 ≤ 30 分、且不超過重試節奏（1440 會轉紅）")
    void giveUpLeaseIsBoundedByRetryBackoff() {
        OrderRepository repo = mock(OrderRepository.class);
        OrderEventPublisher publisher = mock(OrderEventPublisher.class);
        OrderConfig defaults = new OrderConfig();
        when(repo.findPaidOrdersWithoutShipmentPastGrace(any(), any(), anyInt()))
                .thenReturn(List.of(order("ORD_GIVEUP", defaults.getShipmentBackstopMaxAttempts())));

        new ShipmentCreationBackstopScheduler(repo, publisher, defaults).scanStuckPaidOrders();

        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> now = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo).claimShipmentBackstopGiveUp(eq("ORD_GIVEUP"), lease.capture(), now.capture());

        long leaseMinutes = Duration.between(now.getValue(), lease.getValue()).toMinutes();

        assertTrue(leaseMinutes > 0,
                "give-up 租約仍必須是正的未來時間（0 或負 = 永久旗標 / at-most-once）");
        assertTrue(leaseMinutes <= MAX_GIVE_UP_LEASE_MIN,
                "give-up 租約 = publish 靜默失敗後的救援窗口，實測 " + leaseMinutes
                        + " 分 > 上界 " + MAX_GIVE_UP_LEASE_MIN + " 分。"
                        + "1440（1 天）＝ 買家已付款、無物流單、無取消、無退款、無告警 24 小時，"
                        + "相對修復前的 5 分鐘恢復窗口是 288 倍退化。");
        assertTrue(leaseMinutes <= defaults.getShipmentBackstopBackoffMinutes(),
                "give-up 是終局救援，其救援延遲不得長於非終局的重試節奏 backoffMinutes="
                        + defaults.getShipmentBackstopBackoffMinutes() + " 分");
    }
}

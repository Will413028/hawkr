package io.github.will413028.hawkr.order.scheduler;

import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import io.github.will413028.hawkr.order.mq.OrderOutboxPublisher;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 H1 / H1-DEAD：order outbox 的 claim 閘門與候選述詞。
 *
 * <p>本排程是 {@link OrderOutboxPublisher#publish} 的<b>唯一</b>呼叫點，
 * 所以閘門放在這裡、在 publish 之前，是 <b>lease on {@code next_retry_at}</b>。
 */
class OrderOutboxPublisherSchedulerClaimTest {

    private static OrderOutboxEntity pendingEvent(long id) {
        OrderOutboxEntity e = new OrderOutboxEntity();
        e.setId(id);
        e.setAggregateId("ORD" + id);
        e.setEventType("order.paid");
        e.setExchange("order.events");
        e.setRoutingKey("order.paid");
        e.setPayload("{}");
        e.setStatus(OutboxEventStatus.PENDING.value());
        e.setRetryCount(0);
        return e;
    }

    /**
     * ⚠️ 鑑別點：Mockito 對 {@code int} 預設回 <b>0</b>，
     * 「忘記 stub claim」與「claim 真的回 0」長得一樣。沒有本測試，
     * 把 claim 閘門整個拿掉不會有任何測試轉紅。
     */
    @Test
    @DisplayName("claim 回 0（另一個 pod 已認領）→ 完全不呼叫 publisher")
    void claimLost_doesNotPublish() {
        OrderOutboxMapper mapper = mock(OrderOutboxMapper.class);
        OrderOutboxPublisher publisher = mock(OrderOutboxPublisher.class);
        when(mapper.findDispatchable(any(), anyInt())).thenReturn(List.of(pendingEvent(1L)));
        when(mapper.claimForDispatch(eq(1L), any(), any())).thenReturn(0);

        new OrderOutboxPublisherScheduler(mapper, publisher).publishPendingEvents();

        verifyNoInteractions(publisher);
    }

    @Test
    @DisplayName("claim 回 1 → 才投遞，且 claim 嚴格先於 publish（InOrder）")
    void claimWon_publishesAfterClaim() {
        OrderOutboxMapper mapper = mock(OrderOutboxMapper.class);
        OrderOutboxPublisher publisher = mock(OrderOutboxPublisher.class);
        OrderOutboxEntity event = pendingEvent(2L);
        when(mapper.findDispatchable(any(), anyInt())).thenReturn(List.of(event));
        when(mapper.claimForDispatch(eq(2L), any(), any())).thenReturn(1);

        new OrderOutboxPublisherScheduler(mapper, publisher).publishPendingEvents();

        InOrder order = inOrder(mapper, publisher);
        order.verify(mapper).claimForDispatch(eq(2L), any(), any());
        order.verify(publisher).publish(event);
    }

    @Test
    @DisplayName("claim 帶未來租約時間（lease），長度 = CLAIM_LEASE_SEC")
    void claimUsesFutureLease() {
        OrderOutboxMapper mapper = mock(OrderOutboxMapper.class);
        OrderOutboxPublisher publisher = mock(OrderOutboxPublisher.class);
        when(mapper.findDispatchable(any(), anyInt())).thenReturn(List.of(pendingEvent(3L)));
        when(mapper.claimForDispatch(eq(3L), any(), any())).thenReturn(0);

        new OrderOutboxPublisherScheduler(mapper, publisher).publishPendingEvents();

        ArgumentCaptor<LocalDateTime> leaseCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> nowCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper).claimForDispatch(eq(3L), leaseCaptor.capture(), nowCaptor.capture());

        assertTrue(leaseCaptor.getValue().isAfter(nowCaptor.getValue()),
                "租約到期時間必須是未來時間——寫成 now 或過去即退化成永久旗標");
        assertTrue(leaseCaptor.getValue()
                        .isAfter(nowCaptor.getValue()
                                .plusSeconds(OrderOutboxPublisherScheduler.CLAIM_LEASE_SEC - 1)),
                "租約長度必須是 CLAIM_LEASE_SEC（" + OrderOutboxPublisherScheduler.CLAIM_LEASE_SEC + " 秒）");
    }

    @Test
    @DisplayName("超過重試上限 → 走具名 markDeadGuarded，不 claim、不投遞、不再 updateById 全欄覆寫")
    void overMaxRetries_marksDeadWithoutClaiming() {
        OrderOutboxMapper mapper = mock(OrderOutboxMapper.class);
        OrderOutboxPublisher publisher = mock(OrderOutboxPublisher.class);
        OrderOutboxEntity event = pendingEvent(4L);
        event.setRetryCount(10);
        when(mapper.findDispatchable(any(), anyInt())).thenReturn(List.of(event));

        new OrderOutboxPublisherScheduler(mapper, publisher).publishPendingEvents();

        verify(mapper).markDeadGuarded(eq(4L));
        verify(mapper, never()).claimForDispatch(anyLong(), any(), any());
        verify(mapper, never()).updateById(any());
        verifyNoInteractions(publisher);
    }

    /**
     * issue #2294 <b>R4-P1-2</b>：租約基準必須在<b>迴圈內</b>取，不能用 tick 起點。
     *
     * <p>用 tick 起點時，全批共用同一個到期時間 ⇒ 第 k 筆的<b>有效</b>租約
     * = {@code CLAIM_LEASE_SEC} − (前 k−1 筆的耗時)。本排程 {@code BATCH_SIZE=50}、
     * 單筆最壞 30 秒（enrich gRPC 3×10 秒 deadline）⇒ 批次上界 ≈ 25 分鐘 ≫ 60 秒租約
     * ⇒ 租約在原持有者仍在跑 enrich 時就到期，另一個 pod 合法搶走同一筆、重複投遞。
     *
     * <p><b>鑑別方式</b>：讓第一筆的 publish 耗掉一段可觀測的時間，然後比較兩筆的租約。
     * 基準在迴圈內 ⇒ 第二筆的租約嚴格晚於第一筆；基準是 tick 起點 ⇒ 兩者<b>完全相等</b>。
     */
    @Test
    @DisplayName("R4-P1-2：租約基準取 claim 當下——後一筆的租約嚴格晚於前一筆（共用 tick 起點會轉紅）")
    void leaseBaselineIsTakenPerItemNotPerTick() throws Exception {
        OrderOutboxMapper mapper = mock(OrderOutboxMapper.class);
        OrderOutboxPublisher publisher = mock(OrderOutboxPublisher.class);
        OrderOutboxEntity first = pendingEvent(10L);
        OrderOutboxEntity second = pendingEvent(11L);
        when(mapper.findDispatchable(any(), anyInt())).thenReturn(List.of(first, second));
        when(mapper.claimForDispatch(anyLong(), any(), any())).thenReturn(1);
        // 模擬「單筆工作耗時」：真實世界是 enrich gRPC（最壞 30 秒），這裡只需要一段可量測的時間
        org.mockito.Mockito.doAnswer(inv -> {
            Thread.sleep(40L);
            return null;
        }).when(publisher).publish(eq(first));

        new OrderOutboxPublisherScheduler(mapper, publisher).publishPendingEvents();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper, org.mockito.Mockito.times(2))
                .claimForDispatch(anyLong(), leases.capture(), any());
        LocalDateTime lease1 = leases.getAllValues().get(0);
        LocalDateTime lease2 = leases.getAllValues().get(1);

        assertTrue(java.time.Duration.between(lease1, lease2).toMillis() >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 "
                        + java.time.Duration.between(lease1, lease2).toMillis()
                        + " ms）。兩者相等 ⇒ 租約基準是 tick 起點 ⇒ 排在後面的項目"
                        + "在 claim 當下就已經吃掉前面所有項目的耗時，批次夠長時租約直接是過去時間。");
    }

    /**
     * issue #2294 <b>H1-DEAD</b>：候選述詞必須是<b>正面枚舉</b> {@code IN (PENDING, FAILED)}。
     *
     * <p>舊述詞 {@code status <> '8222'}(SENT) 對 {@code '8224'}(DEAD) <b>恆真</b>，
     * 且 {@code markDead} 把 {@code next_retry_at} 設 NULL（第二個條件對 NULL 也恆真）
     * ⇒ DEAD 列每 5 秒被撈回一次、永遠佔住 {@code ORDER BY id ASC LIMIT 50} 的最前面
     * ⇒ 累積 50 筆即整條 outbox starvation（<b>單 pod 也壞</b>）。
     *
     * <p>本測試不連 DB，改驗 default method 傳給 SQL 的兩個狀態參數就是
     * PENDING / FAILED —— 那正是「正面枚舉」這件事在 Java 側唯一可斷言的地方。
     */
    @Test
    @DisplayName("H1-DEAD：候選述詞正面枚舉 PENDING/FAILED（DEAD 與 SENT 皆被排除）")
    void candidatePredicateEnumeratesPendingAndFailedOnly() {
        OrderOutboxMapper mapper = mock(OrderOutboxMapper.class,
                org.mockito.Mockito.withSettings().defaultAnswer(org.mockito.Mockito.CALLS_REAL_METHODS));

        LocalDateTime now = LocalDateTime.now();
        mapper.findDispatchable(now, 50);

        ArgumentCaptor<String> pending = ArgumentCaptor.forClass(String.class);
        ArgumentCaptor<String> failed = ArgumentCaptor.forClass(String.class);
        verify(mapper).findDispatchableByStatus(eq(now), eq(50), pending.capture(), failed.capture());

        assertEquals(OutboxEventStatus.PENDING.value(), pending.getValue());
        assertEquals(OutboxEventStatus.FAILED.value(), failed.getValue());
        // 反向保證：終態不得出現在候選集合
        assertTrue(!OutboxEventStatus.SENT.value().equals(pending.getValue())
                        && !OutboxEventStatus.SENT.value().equals(failed.getValue())
                        && !OutboxEventStatus.DEAD.value().equals(pending.getValue())
                        && !OutboxEventStatus.DEAD.value().equals(failed.getValue()),
                "SENT / DEAD 兩個終態都不得進入候選集合——DEAD 留在候選集合正是 H1-DEAD 的 starvation 成因");
    }
}

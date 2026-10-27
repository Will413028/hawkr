package io.github.will413028.hawkr.webhook.scheduler;

import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.webhook.infrastructure.mq.OutboxPublisher;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.OutboxEvent;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.OutboxEventMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 H3：payuni-webhook outbox 的 claim 閘門。
 *
 * <p>本排程是 {@link OutboxPublisher#publish} 的<b>唯一</b>進入點（webhook 側沒有
 * payment 那種 afterCommit 即時投遞），所以閘門放在這裡、在 publish 之前。
 * 閘門是 <b>lease on {@code next_retry_at}</b>，不是永久旗標。
 */
class OutboxPublisherSchedulerClaimTest {

    private static OutboxEvent pendingEvent(long id) {
        OutboxEvent e = new OutboxEvent();
        e.setId(id);
        e.setAggregateType("payuni_webhook");
        e.setAggregateId("W" + id);
        e.setExchange("payment.events");
        e.setTopic("payment.success");
        e.setMessageKey("K" + id);
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
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        OutboxPublisher publisher = mock(OutboxPublisher.class);
        when(mapper.findPendingEvents(anyInt())).thenReturn(List.of(pendingEvent(1L)));
        when(mapper.claimForDispatch(eq(1L), any(), any())).thenReturn(0);

        new OutboxPublisherScheduler(mapper, publisher, 5).publishPendingEvents();

        verifyNoInteractions(publisher);
    }

    @Test
    @DisplayName("claim 回 1 → 才投遞，且 claim 嚴格先於 publish（InOrder）")
    void claimWon_publishesAfterClaim() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        OutboxPublisher publisher = mock(OutboxPublisher.class);
        OutboxEvent event = pendingEvent(2L);
        when(mapper.findPendingEvents(anyInt())).thenReturn(List.of(event));
        when(mapper.claimForDispatch(eq(2L), any(), any())).thenReturn(1);

        new OutboxPublisherScheduler(mapper, publisher, 5).publishPendingEvents();

        InOrder order = inOrder(mapper, publisher);
        order.verify(mapper).claimForDispatch(eq(2L), any(), any());
        order.verify(publisher).publish(event);
    }

    @Test
    @DisplayName("claim 帶未來租約時間（lease），長度 = CLAIM_LEASE_SEC")
    void claimUsesFutureLease() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        OutboxPublisher publisher = mock(OutboxPublisher.class);
        when(mapper.findPendingEvents(anyInt())).thenReturn(List.of(pendingEvent(3L)));
        when(mapper.claimForDispatch(eq(3L), any(), any())).thenReturn(0);

        new OutboxPublisherScheduler(mapper, publisher, 5).publishPendingEvents();

        ArgumentCaptor<LocalDateTime> leaseCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> nowCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper).claimForDispatch(eq(3L), leaseCaptor.capture(), nowCaptor.capture());

        assertTrue(leaseCaptor.getValue().isAfter(nowCaptor.getValue()),
                "租約到期時間必須是未來時間——寫成 now 或過去即退化成永久旗標");
        assertTrue(leaseCaptor.getValue()
                        .isAfter(nowCaptor.getValue().plusSeconds(OutboxPublisherScheduler.CLAIM_LEASE_SEC - 1)),
                "租約長度必須是 CLAIM_LEASE_SEC（" + OutboxPublisherScheduler.CLAIM_LEASE_SEC + " 秒）");
    }

    /**
     * issue #2294 <b>R4-P1-2</b>：租約基準必須在<b>迴圈內</b>取，不能用 tick 起點。
     *
     * <p>用 tick 起點時，全批共用同一個到期時間 ⇒ 第 k 筆的<b>有效</b>租約
     * = {@code CLAIM_LEASE_SEC} − (前 k−1 筆的耗時)，批次夠長就變成負數
     * ⇒ 租約在原持有者仍在跑時到期，另一個 pod 合法搶走同一筆。
     * 本服務單筆是毫秒級、現行參數下不會踩到，但那是<b>參數巧合不是結構保證</b>
     * （改 {@code BATCH_SIZE} 或給 send 加上同步 confirm 就會踩到）。
     *
     * <p><b>鑑別方式</b>：讓第一筆的 publish 耗掉一段可觀測的時間，然後比較兩筆的租約。
     * 基準在迴圈內 ⇒ 第二筆嚴格晚於第一筆；基準是 tick 起點 ⇒ 兩者<b>完全相等</b>。
     */
    @Test
    @DisplayName("R4-P1-2：租約基準取 claim 當下——後一筆的租約嚴格晚於前一筆（共用 tick 起點會轉紅）")
    void leaseBaselineIsTakenPerItemNotPerTick() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        OutboxPublisher publisher = mock(OutboxPublisher.class);
        OutboxEvent first = pendingEvent(10L);
        OutboxEvent second = pendingEvent(11L);
        when(mapper.findPendingEvents(anyInt())).thenReturn(List.of(first, second));
        when(mapper.claimForDispatch(anyLong(), any(), any())).thenReturn(1);
        org.mockito.Mockito.doAnswer(inv -> {
            Thread.sleep(40L);
            return null;
        }).when(publisher).publish(eq(first));

        new OutboxPublisherScheduler(mapper, publisher, 5).publishPendingEvents();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper, org.mockito.Mockito.times(2))
                .claimForDispatch(anyLong(), leases.capture(), any());
        long deltaMs = java.time.Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();

        assertTrue(deltaMs >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                        + "兩者相等 ⇒ 租約基準是 tick 起點 ⇒ 排在後面的項目在 claim 當下"
                        + "就已經吃掉前面所有項目的耗時。");
    }

    @Test
    @DisplayName("超過重試上限 → 走具名 markDeadGuarded，不 claim、不投遞、不再 updateById 全欄覆寫")
    void overMaxRetries_marksDeadWithoutClaiming() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        OutboxPublisher publisher = mock(OutboxPublisher.class);
        OutboxEvent event = pendingEvent(4L);
        event.setRetryCount(5);
        when(mapper.findPendingEvents(anyInt())).thenReturn(List.of(event));

        new OutboxPublisherScheduler(mapper, publisher, 5).publishPendingEvents();

        verify(mapper).markDeadGuarded(eq(4L), anyString());
        verify(mapper, never()).claimForDispatch(anyLong(), any(), any());
        verify(mapper, never()).updateById(any());
        verifyNoInteractions(publisher);
    }
}

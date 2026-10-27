package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertFalse;
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
 * issue #2294 H2：payment outbox 的 claim 閘門。
 *
 * <p>閘門刻意放在 {@link PaymentOutboxPublisher#publish} 的第一行而不是 Scheduler ——
 * payment 的 outbox 有<b>兩條進入點</b>（Scheduler 每 5s 兜底 ＋ {@code PaymentOutboxWriter}
 * 的 afterCommit 即時投遞），閘門必須放在兩者的共同收斂點。
 */
class PaymentOutboxPublisherClaimTest {

    private static OutboxEventEntity pendingEvent() {
        OutboxEventEntity e = new OutboxEventEntity();
        e.setId(11L);
        // 36 §3.3.1：每一列都必須帶 explicit persisted exchange。
        // 不設它，publisher 的 fail-closed 守衛會直接擋下（那個行為本身由
        // PaymentOutboxExchangeContractTest 專責覆蓋）。
        e.setExchange("payment.events");
        e.setEventType("payment.success");
        e.setRoutingKey("payment.success");
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
    @DisplayName("claim 回 0（另一個 pod 已認領）→ 不打 MQ、不寫任何狀態、回 false")
    void claimLost_doesNotPublishOrWriteStatus() {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        when(mapper.claimForDispatch(eq(11L), any(), any())).thenReturn(0);

        PaymentOutboxPublisher publisher = new PaymentOutboxPublisher(rabbit, mapper);

        assertFalse(publisher.publish(pendingEvent()));

        verifyNoInteractions(rabbit);
        verify(mapper, never()).markSent(anyLong(), any());
        verify(mapper, never()).markFailedGuarded(anyLong(), anyInt(), anyString(), any());
    }

    @Test
    @DisplayName("claim 回 1 → 才投遞；claim 嚴格先於 publishConfirmed，成功後走具名 markSent")
    void claimWon_publishesThenMarksSent() throws Exception {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        when(mapper.claimForDispatch(eq(11L), any(), any())).thenReturn(1);
        // ⚠️ 第一個參數用 eq(...) 不用 anyString()：exchange 是本輪的契約本體，
        //    用 anyString() 的話「publisher 讀錯欄位」不會有任何測試轉紅。
        when(rabbit.publishConfirmed(eq("payment.events"), anyString(), any(), anyString(), anyLong()))
                .thenReturn(true);

        PaymentOutboxPublisher publisher = new PaymentOutboxPublisher(rabbit, mapper);

        assertTrue(publisher.publish(pendingEvent()));

        InOrder order = inOrder(mapper, rabbit);
        order.verify(mapper).claimForDispatch(eq(11L), any(), any());
        order.verify(rabbit).publishConfirmed(eq("payment.events"), anyString(), any(), anyString(), anyLong());
        order.verify(mapper).markSent(eq(11L), any());
        verify(mapper, never()).markFailedGuarded(anyLong(), anyInt(), anyString(), any());
    }

    @Test
    @DisplayName("broker 未 ack → 走具名 markFailedGuarded（守衛 status<>SENT），不再 updateById 全欄覆寫")
    void notAcked_marksFailedGuarded() throws Exception {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        when(mapper.claimForDispatch(eq(11L), any(), any())).thenReturn(1);
        when(rabbit.publishConfirmed(eq("payment.events"), anyString(), any(), anyString(), anyLong()))
                .thenReturn(false);

        PaymentOutboxPublisher publisher = new PaymentOutboxPublisher(rabbit, mapper);

        assertFalse(publisher.publish(pendingEvent()));

        verify(mapper).markFailedGuarded(eq(11L), eq(1), anyString(), any());
        verify(mapper, never()).markSent(anyLong(), any());
        verify(mapper, never()).updateById(any());
    }

    /**
     * claim 必須寫「未來時間」的租約（lease），不是永久旗標；
     * 且租約長度必須大於單筆工作的最壞耗時（＝ publisher-confirm timeout，預設 3 秒）。
     */
    @Test
    @DisplayName("claim 帶未來租約時間（lease），長度遠大於 publisher-confirm timeout")
    void claimUsesFutureLease() {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        when(mapper.claimForDispatch(eq(11L), any(), any())).thenReturn(0);

        PaymentOutboxPublisher publisher = new PaymentOutboxPublisher(rabbit, mapper);
        publisher.publish(pendingEvent());

        ArgumentCaptor<LocalDateTime> leaseCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> nowCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper).claimForDispatch(eq(11L), leaseCaptor.capture(), nowCaptor.capture());

        assertTrue(leaseCaptor.getValue().isAfter(nowCaptor.getValue()),
                "租約到期時間必須是未來時間——寫成 now 或過去即退化成永久旗標");
        assertTrue(leaseCaptor.getValue()
                        .isAfter(nowCaptor.getValue().plusSeconds(PaymentOutboxPublisher.CLAIM_LEASE_SEC - 1)),
                "租約長度必須是 CLAIM_LEASE_SEC（" + PaymentOutboxPublisher.CLAIM_LEASE_SEC + " 秒）");
    }
}

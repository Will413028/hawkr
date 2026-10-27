package io.github.will413028.hawkr.webhook.infrastructure.mq;

import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.OutboxEvent;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.OutboxEventMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.core.RabbitTemplate;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * issue #2294 H3：{@link OutboxPublisher} 的狀態寫入改成<b>具名守衛 UPDATE</b>。
 *
 * <p>原本用 {@code updateById(event)} 全欄覆寫，在多副本下有兩個問題：
 * ① 會把另一個 pod 剛寫入的 claim 租約整列蓋掉；
 * ② 遲到的失敗回寫會把「另一個 pod 已標 SENT」改成 FAILED ⇒ 事件重複投遞一輪。
 */
class OutboxPublisherGuardedWriteTest {

    private static OutboxEvent event() {
        OutboxEvent e = new OutboxEvent();
        e.setId(7L);
        e.setAggregateType("payuni_webhook");
        e.setAggregateId("W7");
        e.setExchange("payment.events");
        e.setTopic("payment.success");
        e.setMessageKey("K7");
        e.setPayload("{}");
        e.setStatus(OutboxEventStatus.PENDING.value());
        e.setRetryCount(0);
        return e;
    }

    @Test
    @DisplayName("投遞成功 → markSent(id, sentAt) 具名守衛 UPDATE，不再 updateById")
    void success_usesGuardedMarkSent() {
        RabbitTemplate rabbit = mock(RabbitTemplate.class);
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);

        assertTrue(new OutboxPublisher(rabbit, mapper).publish(event()));

        ArgumentCaptor<LocalDateTime> sentAt = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper).markSent(eq(7L), sentAt.capture());
        verify(mapper, never()).updateById(any());
        verify(mapper, never()).markFailedGuarded(anyLong(), anyInt(), anyString(), any());
    }

    @Test
    @DisplayName("投遞拋例外 → markFailedGuarded(id, retryCount+1, error, nextRetryAt)，不再 updateById")
    void failure_usesGuardedMarkFailed() {
        RabbitTemplate rabbit = mock(RabbitTemplate.class);
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        doThrow(new RuntimeException("broker down"))
                .when(rabbit).send(anyString(), anyString(), any(Message.class));

        assertFalse(new OutboxPublisher(rabbit, mapper).publish(event()));

        ArgumentCaptor<LocalDateTime> nextRetryAt = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper).markFailedGuarded(eq(7L), eq(1), anyString(), nextRetryAt.capture());
        verify(mapper, never()).markSent(anyLong(), any());
        verify(mapper, never()).updateById(any());
        // ⚠️ 一律用 DateTimeUtils.nowUtc() 比較：生產碼寫的是 UTC，
        // 拿本機時區的 LocalDateTime.now() 比會在非 UTC 時區得到假失敗（10 檔 §4）。
        assertTrue(nextRetryAt.getValue()
                        .isAfter(io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc().plusSeconds(3)),
                "退避時間必須指向未來（指數退避第一段 5 秒）");
    }
}

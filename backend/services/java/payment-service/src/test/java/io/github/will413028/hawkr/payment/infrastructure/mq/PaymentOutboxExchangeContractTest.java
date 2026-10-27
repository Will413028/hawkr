package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.NullSource;
import org.junit.jupiter.params.provider.ValueSource;
import org.mockito.ArgumentCaptor;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * 36 §3.3.1「Outbox persisted exchange」的 publisher 側契約（BL7 fail-closed 守衛）。
 *
 * <p><b>它證明什麼</b>：{@link PaymentOutboxPublisher#publish} 的 exchange 一律取自
 * 該列持久化的 {@code exchange} 欄，而且<b>只</b>取自那裡——不 hardcode、不從
 * {@code routing_key} 推導、不從 {@code event_type} 推導、缺值時不 fallback。
 *
 * <p><b>為什麼期望值全部硬寫字面值</b>：若寫成 {@code MqConstants.EXCHANGE_PAYMENT}，
 * 常數被改動時測試會跟著改動而永遠通過（期望值來自被測物的另一種形態）。
 * 這裡的 {@code "payment.events"} / {@code "hawkr.shipping"} 是 36 號 catalog 的
 * 契約字面值，刻意與 code 脫鉤。
 *
 * <p><b>鑑別力最高的一支是 {@link #exchangeWins_overRoutingKeyPrefix()}</b>：
 * 它造一列「routing key 看起來完全是 payment、exchange 卻是 shipping」的事件。
 * 任何「從 routing key 前綴猜 exchange」的實作都會在那一支轉紅，而其他支不會。
 */
class PaymentOutboxExchangeContractTest {

    private static final String PAYMENT_EXCHANGE = "payment.events";
    private static final String SHIPPING_EXCHANGE = "hawkr.shipping";

    private static OutboxEventEntity row(String exchange, String eventType, String routingKey) {
        OutboxEventEntity e = new OutboxEventEntity();
        e.setId(77L);
        e.setExchange(exchange);
        e.setEventType(eventType);
        e.setRoutingKey(routingKey);
        e.setPayload("{}");
        e.setStatus(OutboxEventStatus.PENDING.value());
        e.setRetryCount(0);
        return e;
    }

    private static OutboxEventMapper claimGranted() {
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        when(mapper.claimForDispatch(anyLong(), any(), any())).thenReturn(1);
        return mapper;
    }

    // ───────────────── ① 讀 persisted exchange（兩種 exchange 各一條 round-trip）─────────────────

    @Test
    @DisplayName("payment 列 → 投遞到 payment.events")
    void paymentRow_goesToPaymentExchange() throws Exception {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = claimGranted();
        when(rabbit.publishConfirmed(anyString(), anyString(), any(), anyString(), anyLong())).thenReturn(true);

        assertTrue(new PaymentOutboxPublisher(rabbit, mapper)
                .publish(row(PAYMENT_EXCHANGE, "payment.success", "payment.success")));

        ArgumentCaptor<String> ex = ArgumentCaptor.forClass(String.class);
        verify(rabbit).publishConfirmed(ex.capture(), anyString(), any(), anyString(), anyLong());
        assertEquals(PAYMENT_EXCHANGE, ex.getValue());
        verify(mapper).markSent(eq(77L), any());
    }

    @Test
    @DisplayName("shipping 列 → 投遞到 hawkr.shipping（同一支 publisher、同一張表）")
    void shippingRow_goesToShippingExchange() throws Exception {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = claimGranted();
        when(rabbit.publishConfirmed(anyString(), anyString(), any(), anyString(), anyLong())).thenReturn(true);

        assertTrue(new PaymentOutboxPublisher(rabbit, mapper)
                .publish(row(SHIPPING_EXCHANGE, "fulfillment.responsibility.finalized",
                        "fulfillment.responsibility.finalized")));

        ArgumentCaptor<String> ex = ArgumentCaptor.forClass(String.class);
        verify(rabbit).publishConfirmed(ex.capture(), anyString(), any(), anyString(), anyLong());
        assertEquals(SHIPPING_EXCHANGE, ex.getValue(),
                "publisher 必須讀該列的 exchange 欄；讀到 payment.events 代表它仍在 hardcode");
    }

    /**
     * 🔴 本檔鑑別力最高的一支：routing key 與 exchange <b>刻意矛盾</b>。
     *
     * <p>{@code routing_key = "payment.success"}（看起來百分之百是 payment 事件）
     * 而 {@code exchange = "hawkr.shipping"}。任何從 routing key／event type 前綴
     * 推導 exchange 的實作，都只會在這一支轉紅。
     */
    @Test
    @DisplayName("routing key 與 exchange 矛盾時，exchange 欄勝出（證明沒有 routing-key 推導）")
    void exchangeWins_overRoutingKeyPrefix() throws Exception {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = claimGranted();
        when(rabbit.publishConfirmed(anyString(), anyString(), any(), anyString(), anyLong())).thenReturn(true);

        new PaymentOutboxPublisher(rabbit, mapper)
                .publish(row(SHIPPING_EXCHANGE, "payment.success", "payment.success"));

        ArgumentCaptor<String> ex = ArgumentCaptor.forClass(String.class);
        verify(rabbit).publishConfirmed(ex.capture(), eq("payment.success"), any(), anyString(), anyLong());
        assertEquals(SHIPPING_EXCHANGE, ex.getValue(),
                "exchange 必須來自列上的欄位，不是從 routing key 前綴推導出來的");
    }

    // ───────────────── ② fail-closed：null／空／空白 ─────────────────

    /**
     * 空白的四種形態各驗一次。{@code "\t\n"} 這一組是刻意的：
     * V11 的 CHECK 原本寫成 {@code btrim(exchange) <> ''}，而 {@code btrim} 預設只剝空格
     * ——本輪實測 tab／換行組成的值會穿過去，因此 DB 側改用 {@code exchange ~ '\S'}、
     * Java 側用 {@code isBlank()}（涵蓋全部 whitespace）。兩層語意必須一致。
     */
    @ParameterizedTest(name = "exchange={0} → 不投遞、標 FAILED")
    @NullSource
    @ValueSource(strings = {"", "   ", "\t\n"})
    @DisplayName("exchange 為 null／空／空白 → Rabbit 零互動、走 guarded FAILED、不 fallback")
    void invalidExchange_failsClosed(String invalid) {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = claimGranted();

        assertFalse(new PaymentOutboxPublisher(rabbit, mapper)
                .publish(row(invalid, "payment.success", "payment.success")));

        // no publish：連一次 Rabbit 呼叫都不能有（fallback 回 payment.events 會在這裡轉紅）
        verifyNoInteractions(rabbit);
        // 走既有 guarded FAILED path，retry_count 遞增 → 沿用既有退避與 DEAD lifecycle
        verify(mapper).markFailedGuarded(eq(77L), eq(1), anyString(), any());
        verify(mapper, never()).markSent(anyLong(), any());
    }

    @Test
    @DisplayName("fail-closed 的 last_error 要說得出「是 exchange 的問題」")
    void invalidExchange_errorMessageIsDiagnosable() {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = claimGranted();

        new PaymentOutboxPublisher(rabbit, mapper).publish(row(null, "payment.success", "payment.success"));

        ArgumentCaptor<String> err = ArgumentCaptor.forClass(String.class);
        verify(mapper).markFailedGuarded(eq(77L), anyInt(), err.capture(), any());
        assertTrue(err.getValue().contains("exchange"),
                "last_error 必須指出 invalid/missing exchange，否則 DEAD 之後沒人查得出原因，實際值="
                        + err.getValue());
    }

    /**
     * 守衛的位置契約：<b>claim 之後</b>才驗 exchange。
     *
     * <p>放在 claim 之前的話，沒搶到本筆的 pod 也會去寫 {@code markFailedGuarded}，
     * 兩個 pod 互相覆蓋 {@code retry_count}。所以 claim 失敗時，連 FAILED 都不該寫。
     */
    @Test
    @DisplayName("claim 失敗時即使 exchange 無效，也不寫任何狀態（守衛在 claim 之後）")
    void claimLost_takesPrecedenceOverExchangeValidation() {
        ObsRabbitPublisher rabbit = mock(ObsRabbitPublisher.class);
        OutboxEventMapper mapper = mock(OutboxEventMapper.class);
        when(mapper.claimForDispatch(anyLong(), any(), any())).thenReturn(0);

        assertFalse(new PaymentOutboxPublisher(rabbit, mapper)
                .publish(row(null, "payment.success", "payment.success")));

        verifyNoInteractions(rabbit);
        verify(mapper, never()).markFailedGuarded(anyLong(), anyInt(), anyString(), any());
        verify(mapper, never()).markSent(anyLong(), any());
    }
}

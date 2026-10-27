package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.mq.ProviderWebhookEnvelope;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.mapper.ShippingWebhookDedupMapper;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.util.Map;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@link UberWebhookConsumer} 的三條契約：
 * <b>raw bytes 原封不動送進 adapter</b>、<b>解析一律走 adapter</b>、<b>失敗不靜默 ACK</b>。
 *
 * <h2>變異對照</h2>
 * <ul>
 *   <li>consumer 改成自己 parse JSON 讀 status ⇒ {@link #rawBytesReachAdapterUntouched} 與
 *       {@link #normalizationComesFromAdapter} 轉紅（stub 的回傳值刻意與 payload 無關）</li>
 *   <li>查無配送單時改成 {@code basicAck} ⇒ {@link #missingShipmentIsNotSilentlyAcked} 轉紅</li>
 *   <li>拿掉 consumer 端 dedup ⇒ {@link #duplicateEventIsSkipped} 轉紅</li>
 * </ul>
 */
class UberWebhookConsumerTest {

    /** stub 回傳值刻意與 payload 內容無關：consumer 若自己解析就拿不到它。 */
    private static final ShippingStatus STUB_NORMALIZED = ShippingStatus.IN_TRANSIT;
    private static final String STUB_DEDUP_KEY = "uber-dedup-key-from-adapter";

    private ShippingService shippingService;
    private ShippingWebhookDedupMapper dedupMapper;
    private UberWebhookConsumer consumer;
    private Channel channel;
    private byte[] capturedRawBody;

    @BeforeEach
    void setUp() {
        shippingService = mock(ShippingService.class);
        dedupMapper = mock(ShippingWebhookDedupMapper.class);
        channel = mock(Channel.class);
        consumer = new UberWebhookConsumer(shippingService, dedupMapper);
        when(dedupMapper.selectCount(any())).thenReturn(0L);
        capturedRawBody = null;
    }

    /**
     * DaaS webhook body。欄名抄自 {@code Uber Direct API 總覽.md} §7.2
     * （{@code id}／{@code kind}／{@code delivery_id}／{@code data}／{@code external_id}）。
     * 刻意保留非標準空白與 JSON 逸出 —— 那是 re-serialize 會抹平的特徵。
     */
    private static byte[] uberBody() {
        return ("{\n  \"kind\"  : \"event.delivery_status\",\n"
                + "  \"id\": \"evt_\\u5b85\",\n"
                + "  \"delivery_id\": \"del_abc\",\n"
                + "  \"data\": { \"status\": \"pickup_complete\", \"external_id\": \"S000000123\" }\n}\n")
                .getBytes(StandardCharsets.UTF_8);
    }

    private Message envelopeMessage(byte[] rawBody) throws Exception {
        ProviderWebhookEnvelope env = ProviderWebhookEnvelope.ofJson(
                "8242", "event.delivery_status", "evt_1", rawBody,
                Map.of("x-uber-signature", "sig"), "2026-09-12T01:02:03Z");
        byte[] body = new ObjectMapper().writeValueAsBytes(env.toWireMap());
        return new Message(body, new MessageProperties());
    }

    private InboundCapability stubInbound() {
        return new InboundCapability() {
            @Override
            public InboundStatusEvent parseStatusEvent(InboundPayload raw) {
                capturedRawBody = raw.rawBody();
                return new InboundStatusEvent(Optional.of(STUB_NORMALIZED.value()),
                        "pickup_complete", "stub", LocalDateTime.of(2026, 9, 12, 1, 2, 3));
            }

            @Override
            public boolean isLabelEvent(InboundPayload raw) {
                return false;
            }

            @Override
            public Optional<InboundLabelEvent> parseLabelEvent(InboundPayload raw) {
                return Optional.empty();
            }

            @Override
            public String dedupKey(InboundPayload raw) {
                capturedRawBody = raw.rawBody();
                return STUB_DEDUP_KEY;
            }
        };
    }

    @Test
    @DisplayName("🔴 raw bytes 逐位元組送達 adapter（驗簽是 HMAC over raw body，陷阱 A2）")
    void rawBytesReachAdapterUntouched() throws Exception {
        byte[] original = uberBody();
        when(shippingService.inboundFor("S000000123")).thenReturn(Optional.of(stubInbound()));

        consumer.onUberWebhook(envelopeMessage(original), channel, 1L);

        assertThat(capturedRawBody)
                .as("adapter 收到的必須與 Uber 送來的逐位元組相同")
                .isEqualTo(original);
    }

    @Test
    @DisplayName("正規化與去重鍵都由 adapter 提供，consumer 不得自己解析")
    void normalizationComesFromAdapter() throws Exception {
        when(shippingService.inboundFor("S000000123")).thenReturn(Optional.of(stubInbound()));

        consumer.onUberWebhook(envelopeMessage(uberBody()), channel, 1L);

        ArgumentCaptor<InboundCapability.InboundStatusEvent> captor =
                ArgumentCaptor.forClass(InboundCapability.InboundStatusEvent.class);
        verify(shippingService).applyInboundStatus(eq("S000000123"), captor.capture());
        assertThat(captor.getValue().normalizedStatus())
                .as("consumer 必須交出 adapter 正規化的結果；自己解析就拿不到這個值")
                .contains(STUB_NORMALIZED.value());

        ArgumentCaptor<io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity> dedupCaptor =
                ArgumentCaptor.forClass(io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity.class);
        verify(dedupMapper).insert(dedupCaptor.capture());
        assertThat(dedupCaptor.getValue().getEventKey()).isEqualTo(STUB_DEDUP_KEY);

        // 🔴 legacy 自解路徑零呼叫：不得存在第二份 status processing SSOT
        verify(shippingService, never())
                .processWebhookStatusUpdate(anyString(), anyString(), anyString(), anyString());
        verify(channel).basicAck(1L, false);
    }

    @Test
    @DisplayName("亂序／非法轉移不由 consumer 判斷：它只把正規化結果交給中央分派點")
    void orderingIsDelegatedToStatusMapper() throws Exception {
        when(shippingService.inboundFor("S000000123")).thenReturn(Optional.of(stubInbound()));

        consumer.onUberWebhook(envelopeMessage(uberBody()), channel, 1L);

        // consumer 沒有任何「先讀目前狀態再決定要不要寫」的動作 —— 那是 applyInboundStatus 內
        // isValidTransition 的職責（66 §6.3.5 第 ② 道）。
        verify(shippingService).applyInboundStatus(anyString(), any());
        verify(shippingService, never()).findByShipTradeNo(anyString());
    }

    @Test
    @DisplayName("重複事件（consumer 端 dedup 命中）→ 直接 ACK，不再套用狀態")
    void duplicateEventIsSkipped() throws Exception {
        when(shippingService.inboundFor("S000000123")).thenReturn(Optional.of(stubInbound()));
        when(dedupMapper.selectCount(any())).thenReturn(1L);

        consumer.onUberWebhook(envelopeMessage(uberBody()), channel, 1L);

        verify(shippingService, never()).applyInboundStatus(anyString(), any());
        verify(dedupMapper, never()).insert(any());
        verify(channel).basicAck(1L, false);
    }

    @Test
    @DisplayName("🔴 查無配送單 → 不得靜默 ACK（走 MqErrorHandler：重試 → DLQ）")
    void missingShipmentIsNotSilentlyAcked() throws Exception {
        when(shippingService.inboundFor(anyString())).thenReturn(Optional.empty());

        consumer.onUberWebhook(envelopeMessage(uberBody()), channel, 1L);

        // MqErrorHandler 第一次失敗走 ACK＋republish（retry），而不是「處理完成的 ACK」——
        // 判別點是「有沒有真的套用狀態」與「有沒有重投」。
        verify(shippingService, never()).applyInboundStatus(anyString(), any());
        verify(dedupMapper, never()).insert(any());
        verify(channel).basicPublish(eq(""), any(), any(), any());
    }

    @Test
    @DisplayName("🔴 信封缺 raw body → 契約壞了，不得靜默 ACK")
    void missingRawBodyIsNotSilentlyAcked() throws Exception {
        ProviderWebhookEnvelope env = ProviderWebhookEnvelope.ofFormParams(
                "8242", "event.delivery_status", "evt_1", Map.of("a", "b"), "2026-09-12T01:02:03Z");
        Message msg = new Message(new ObjectMapper().writeValueAsBytes(env.toWireMap()),
                new MessageProperties());

        consumer.onUberWebhook(msg, channel, 1L);

        verify(shippingService, never()).applyInboundStatus(anyString(), any());
        verify(channel).basicPublish(eq(""), any(), any(), any());
    }

    @Test
    @DisplayName("🔴 找不到我方識別碼 → 不得靜默 ACK（否則配送狀態永遠不會更新且無人知曉）")
    void missingCorrelationKeyIsNotSilentlyAcked() throws Exception {
        byte[] noExternalId = ("{\"kind\":\"event.delivery_status\",\"id\":\"evt_2\","
                + "\"delivery_id\":\"del_x\",\"data\":{\"status\":\"pickup\"}}")
                .getBytes(StandardCharsets.UTF_8);

        consumer.onUberWebhook(envelopeMessage(noExternalId), channel, 1L);

        verify(shippingService, never()).inboundFor(anyString());
        verify(shippingService, never()).applyInboundStatus(anyString(), any());
        verify(channel).basicPublish(eq(""), any(), any(), any());
    }
}

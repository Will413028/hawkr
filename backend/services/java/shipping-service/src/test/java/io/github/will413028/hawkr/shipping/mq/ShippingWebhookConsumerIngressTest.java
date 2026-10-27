package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.mapper.ShippingWebhookDedupMapper;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundPayload;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.service.CvsPrintWaitingRegistry;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * <b>BLOCKER 4 的守衛：provider 回調必須匯流到 canonical ingress。</b>
 *
 * <h2>這條路徑先前零測試覆蓋</h2>
 * {@code ShippingWebhookConsumer} 在 {@code src/test} 全 repo 零命中；
 * {@code processWebhookStatusUpdate} 雖有 12 支測試，但全部直接呼叫 {@code ShippingService}，
 * <b>繞過 consumer 與去重層</b>。於是「consumer 自己解 PayUni 欄名、自己組去重鍵」
 * 這件事沒有任何東西看得到。
 *
 * <h2>M5 變異</h2>
 * 把 consumer 改回 legacy bypass（自己讀 {@code ShipStatus} 並呼叫
 * {@code processWebhookStatusUpdate}）⇒ {@link #statusCallbackGoesThroughAdapterNotSelfParsing} 轉紅：
 * 它斷言 consumer 交出去的是<b>adapter 正規化過的事件</b>，而且 legacy 方法一次都沒被呼叫。
 */
class ShippingWebhookConsumerIngressTest {

    private ShippingService shippingService;
    private ShippingWebhookDedupMapper dedupMapper;
    private CvsPrintWaitingRegistry waitingRegistry;
    private ShippingWebhookConsumer consumer;
    private Channel channel;

    /** 這一支 stub 的回傳值刻意與 payload 內容無關——consumer 若自己解析就拿不到它。 */
    private static final ShippingStatus STUB_NORMALIZED = ShippingStatus.IN_TRANSIT;

    @BeforeEach
    void setUp() {
        shippingService = mock(ShippingService.class);
        dedupMapper = mock(ShippingWebhookDedupMapper.class);
        waitingRegistry = mock(CvsPrintWaitingRegistry.class);
        channel = mock(Channel.class);
        consumer = new ShippingWebhookConsumer(shippingService, dedupMapper, waitingRegistry);
        when(dedupMapper.selectCount(any())).thenReturn(0L);
    }

    @Test
    @DisplayName("貨態回調：解析走 adapter，legacy 自解路徑一次都不得被呼叫")
    void statusCallbackGoesThroughAdapterNotSelfParsing() throws Exception {
        when(shippingService.inboundFor("SC-1")).thenReturn(Optional.of(stubInbound()));

        consumer.onShippingWebhook(msg(Map.of(
                "ShipTradeNo", "SC-1", "MerID", "HAWK1", "ShipStatus", "300",
                "ShipStatusDesc", "配送中", "ShipStatusTime", "2026-09-10 14:00:00")), channel, 1L);

        ArgumentCaptor<InboundCapability.InboundStatusEvent> captor =
                ArgumentCaptor.forClass(InboundCapability.InboundStatusEvent.class);
        verify(shippingService).applyInboundStatus(anyString(), captor.capture());
        assertThat(captor.getValue().normalizedStatus())
                .as("consumer 必須交出 adapter 正規化的結果；自己解析就拿不到這個值")
                .contains(STUB_NORMALIZED.value());

        // 🔴 legacy 自解路徑必須零呼叫——這一條才是「不得存在兩份 status processing SSOT」的斷言
        verify(shippingService, never())
                .processWebhookStatusUpdate(anyString(), anyString(), anyString(), anyString());
    }

    @Test
    @DisplayName("去重鍵由 adapter 提供：consumer 不得自己組")
    void dedupKeyComesFromAdapter() throws Exception {
        when(shippingService.inboundFor("SC-2")).thenReturn(Optional.of(stubInbound()));

        consumer.onShippingWebhook(msg(Map.of(
                "ShipTradeNo", "SC-2", "MerID", "HAWK1", "ShipStatus", "300")), channel, 2L);

        ArgumentCaptor<io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity> captor =
                ArgumentCaptor.forClass(io.github.will413028.hawkr.shipping.entity.ShippingWebhookDedupEntity.class);
        verify(dedupMapper).insert(captor.capture());
        assertThat(captor.getValue().getEventKey())
                .as("去重鍵必須是 adapter 給的那一個")
                .isEqualTo("STUB-DEDUP-KEY");
    }

    /**
     * 🔴 解析不出 adapter 時<b>仍要保存 provider 說了什麼</b>。
     *
     * <p>本測試原本斷言「legacy 方法一次都不得被呼叫」，於是把 raw 值一起丟掉了：
     * 配送單存在而只是 registry 查不到 adapter 時，物流商回報的
     * 「包裹遺失」在系統裡會完全沒有痕跡。
     * 相容入口對這個情境有明確處置（{@code WEBHOOK_NO_ADAPTER}），交給它才是對的。
     *
     * <p>⚠️ 這不與「不得存在兩份 status processing SSOT」衝突：
     * 有 adapter 時一律走 canonical path（由
     * {@link #statusCallbackGoesThroughAdapterNotSelfParsing} 守），
     * 這條只在「沒有 adapter 可用」時走得到——那本身就是異常狀態。
     */
    @Test
    @DisplayName("provider 無回調能力：ACK，但必須交給相容入口保存 raw，不得靜默丟棄")
    void unresolvableCallbackStillPreservesRawValue() throws Exception {
        when(shippingService.inboundFor("SC-404")).thenReturn(Optional.empty());

        consumer.onShippingWebhook(msg(Map.of(
                "ShipTradeNo", "SC-404", "MerID", "HAWK1", "ShipStatus", "300",
                "ShipStatusDesc", "包裹遺失")), channel, 3L);

        verify(channel).basicAck(3L, false);
        verify(shippingService, never()).applyInboundStatus(anyString(), any());
        verify(shippingService).processWebhookStatusUpdate("SC-404", "300", "包裹遺失", "");
    }

    /**
     * 🔴 「這是不是標籤事件」必須由 <b>adapter</b> 回答，不是 consumer 自己讀 PayUni 欄名。
     *
     * <p>本測試的鑑別力來自一個刻意的錯配：payload <b>沒有</b> {@code ApiType=Print}，
     * 而 stub 的 {@code isLabelEvent} 回 {@code true}。
     * consumer 若自己判 {@code "Print".equals(ApiType)} 就會走貨態分支 ⇒ 斷言失敗。
     *
     * <p>為什麼這件事重要：adapter 的 {@code dedupKey} <b>已經</b>在做同一個判定
     * （決定 {@code PRINT:} 還是 {@code STATUS:} 前綴）。兩份 provider 知識分屬兩層、
     * 必須手動保持同步——今天 PayUni 讀同一個欄所以不會分歧，
     * 第二個 provider 一進來，consumer 的 if 就永遠不會把它的 label 事件路由過去。
     */
    @Test
    @DisplayName("標籤事件的判定由 adapter 給：payload 沒有 ApiType 但 adapter 說是，就必須走標籤分支")
    void labelEventDiscriminationComesFromAdapterNotConsumer() throws Exception {
        when(shippingService.inboundFor("SC-5")).thenReturn(Optional.of(alwaysLabelInbound()));

        consumer.onShippingWebhook(msg(Map.of(
                "ShipTradeNo", "SC-5", "MerID", "HAWK1",
                "PartnerId", "8B2", "Odno", "S999", "ValidationNo", "1234")), channel, 5L);

        verify(shippingService).processWebhookPrint(anyString(), any());
        verify(shippingService, never()).applyInboundStatus(anyString(), any());
    }

    @Test
    @DisplayName("Print 回調：三段碼從 adapter 的 segmentCodes 取，並回填等待中的列印請求")
    void printCallbackUsesAdapterSegments() throws Exception {
        when(shippingService.inboundFor("SC-3")).thenReturn(Optional.of(stubInbound()));

        consumer.onShippingWebhook(msg(Map.of(
                "ShipTradeNo", "SC-3", "MerID", "HAWK1", "ApiType", "Print",
                "PartnerId", "8B2", "Odno", "S3097983", "ValidationNo", "3067")), channel, 4L);

        @SuppressWarnings("unchecked")
        ArgumentCaptor<Map<String, String>> captor = ArgumentCaptor.forClass(Map.class);
        verify(shippingService).processWebhookPrint(anyString(), captor.capture());
        assertThat(captor.getValue()).containsEntry(LabelCapability.SEGMENT_ODNO, "S3097983");
        verify(waitingRegistry).complete(anyString(), any());
    }

    // ==================== helpers ====================

    private static Message msg(Map<String, String> payload) {
        StringBuilder json = new StringBuilder("{");
        boolean first = true;
        for (Map.Entry<String, String> e : new LinkedHashMap<>(payload).entrySet()) {
            if (!first) {
                json.append(',');
            }
            first = false;
            json.append('"').append(e.getKey()).append("\":\"").append(e.getValue()).append('"');
        }
        json.append('}');
        return new Message(json.toString().getBytes(StandardCharsets.UTF_8), new MessageProperties());
    }

    /**
     * 刻意回「與 payload 無關」的固定值：consumer 若自己解析 payload，
     * 斷言就對不上 —— 那正是本測試的鑑別力來源。
     */
    /** 刻意與 payload 脫鉤：不管內容一律說「這是標籤事件」。 */
    private static InboundCapability alwaysLabelInbound() {
        InboundCapability base = stubInbound();
        return new InboundCapability() {
            @Override
            public InboundStatusEvent parseStatusEvent(InboundPayload p) {
                return base.parseStatusEvent(p);
            }

            @Override
            public boolean isLabelEvent(InboundPayload p) {
                return true;
            }

            @Override
            public Optional<InboundLabelEvent> parseLabelEvent(InboundPayload p) {
                return base.parseLabelEvent(p);
            }

            @Override
            public String dedupKey(InboundPayload p) {
                return base.dedupKey(p);
            }
        };
    }

    private static InboundCapability stubInbound() {
        return new InboundCapability() {
            @Override
            public InboundStatusEvent parseStatusEvent(InboundPayload raw) {
                return new InboundStatusEvent(Optional.of(STUB_NORMALIZED.value()),
                        "STUB-RAW", "stub", LocalDateTime.of(2026, 9, 10, 6, 0));
            }

            @Override
            public boolean isLabelEvent(InboundPayload raw) {
                // stub 也用 provider 專屬欄名——但那是**在 adapter 這一側**，
                // 正是本波要把它移到的地方。consumer 只拿中性結論。
                return "Print".equals(raw.formParams().get("ApiType"));
            }

            @Override
            public Optional<InboundLabelEvent> parseLabelEvent(InboundPayload raw) {
                Map<String, String> rawPayload = raw.formParams();
                return Optional.of(new InboundLabelEvent(rawPayload.get("ShipTradeNo"), Map.of(
                        LabelCapability.SEGMENT_PARTNER_ID, rawPayload.getOrDefault("PartnerId", ""),
                        LabelCapability.SEGMENT_ODNO, rawPayload.getOrDefault("Odno", ""),
                        LabelCapability.SEGMENT_VALIDATION_NO, rawPayload.getOrDefault("ValidationNo", ""))));
            }

            @Override
            public String dedupKey(InboundPayload raw) {
                return "STUB-DEDUP-KEY";
            }
        };
    }
}

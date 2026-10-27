package io.github.will413028.hawkr.webhook.uber;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.ProviderWebhookEnvelope;
import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import io.github.will413028.hawkr.webhook.service.WebhookInboxTxOps;
import io.github.will413028.hawkr.webhook.service.WebhookOutboxTxOps;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.dao.DuplicateKeyException;

import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;

import static org.mockito.Mockito.verifyNoInteractions;
import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Uber ingress 的三條硬契約：<b>raw bytes 保真</b>、<b>inbox 去重</b>、<b>路由決策</b>。
 *
 * <h2>變異對照（每條斷言都要有一個「該讓它失敗」的改法）</h2>
 * <ul>
 *   <li>把 {@code rawBody} 改成「parse 後再 serialize」⇒ {@link RawBodyFidelity} 轉紅（陷阱 A2）</li>
 *   <li>拿掉 {@code catch (DuplicateKeyException)} 的 dedup 分支 ⇒ {@link InboxDedup} 轉紅</li>
 *   <li>把 {@code courier_update} 改成 PUBLISH ⇒ {@link RouteDecision} 轉紅</li>
 * </ul>
 */
class UberWebhookIngressContractTest {

    /** 任意非空碼值：本測試刻意不依賴 WebhookProvider CodeMeta 是否已新增 UBER。 */
    private static final String PROVIDER_VALUE = "8242";

    /**
     * 測試用 signing key。
     *
     * 🔴 驗簽在 2026-09-12 補回 ingress：未驗簽就落地 ＋ inbox 以 uq_inbox_event 去重
     * ＝ 任何人搶先送一個同 event_id 的偽造事件，就能讓真正的 Uber 事件被判為重複而丟棄。
     * ⇒ 本檔每一個「應該成功落地」的案例都必須帶**真的算得出來**的簽章，
     * 不能再像先前那樣塞 "abc123"。
     */
    private static final String TEST_SIGNING_KEY = "ingress-test-signing-key";

    private WebhookInboxMapper inboxMapper;
    private WebhookInboxTxOps inboxTxOps;
    private WebhookOutboxTxOps outboxTxOps;
    private UberWebhookProviderCode providerCode;
    private UberWebhookReceiveService service;

    @BeforeEach
    void setUp() {
        inboxMapper = mock(WebhookInboxMapper.class);
        inboxTxOps = mock(WebhookInboxTxOps.class);
        outboxTxOps = mock(WebhookOutboxTxOps.class);
        providerCode = mock(UberWebhookProviderCode.class);
        when(providerCode.value()).thenReturn(PROVIDER_VALUE);
        // 模擬 MyBatis 回填自增主鍵
        doAnswer(inv -> {
            inv.getArgument(0, WebhookInbox.class).setId(4242L);
            return null;
        }).when(inboxTxOps).insertInbox(any());
        service = new UberWebhookReceiveService(inboxMapper, inboxTxOps, outboxTxOps,
                providerCode, new ObjectMapper(),
                new UberWebhookVerifier(java.util.List.of(TEST_SIGNING_KEY)));
    }

    /**
     * 一份<b>刻意不規範</b>的 body：鍵序與官方範例不同、含多餘空白與換行、
     * 含 JSON 的 &#92;uXXXX 逸出、含非 ASCII、結尾有換行。
     *
     * <p>這些正是「parse 再 re-serialize」會抹平掉的東西 —— 而驗簽（HMAC-SHA256 over raw body）
     * 對的就是它們。欄名取自 API 總覽 §7.2。
     */
    private static byte[] awkwardBody() {
        String json = "{\n"
                + "  \"kind\"  : \"event.delivery_status\",\n"
                + "  \"id\": \"evt_\\u5b85\",\n"
                + "  \"delivery_id\": \"del_abc\",\n"
                + "  \"created\": \"2026-09-12T01:02:03Z\",\n"
                + "  \"data\": { \"status\": \"pickup_complete\", \"external_id\": \"S000000123\" }\n"
                + "}\n";
        return json.getBytes(StandardCharsets.UTF_8);
    }

    private String capturedOutboxPayload() {
        @SuppressWarnings("unchecked")
        ArgumentCaptor<List<WebhookOutboxTxOps.OutboxRoute>> captor =
                ArgumentCaptor.forClass(List.class);
        verify(outboxTxOps).enqueueAndMark(anyLong(), anyString(), captor.capture());
        assertEquals(1, captor.getValue().size(), "delivery_status 應只有一條 route");
        return captor.getValue().get(0).payloadJson();
    }

    @Nested
    @DisplayName("🔴 raw bytes 逐位元組保真（陷阱 A2）")
    class RawBodyFidelity {

        @Test
        @DisplayName("送進 MQ 的 raw_body_base64 解出來，必須與 HTTP body 逐位元組相同")
        void rawBodySurvivesTheHop() {
            byte[] original = awkwardBody();

            service.receive(original, signed(original));

            ProviderWebhookEnvelope env =
                    ProviderWebhookEnvelope.parse(capturedOutboxPayload().getBytes(StandardCharsets.UTF_8));
            assertArrayEquals(original, env.rawBody(),
                    "raw body 被改動了 —— 驗簽是 HMAC-SHA256 over raw body，"
                            + "任何 parse/re-serialize 都會讓它偶發失敗（66 §6.3.1 陷阱 A2）");
        }

        @Test
        @DisplayName("fixture 本身確實含有會被 re-serialize 抹平的特徵（否則上一條沒有鑑別力）")
        void fixtureActuallyHasNonCanonicalFeatures() {
            String s = new String(awkwardBody(), StandardCharsets.UTF_8);
            assertTrue(s.contains("\\u5b85"), "fixture 必須含 JSON unicode 逸出");
            assertTrue(s.contains("\"kind\"  :"), "fixture 必須含非標準空白");
            assertTrue(s.endsWith("\n"), "fixture 必須以換行結尾");
            // 反證：Jackson 重新序列化後這三個特徵全部消失
            String reserialized;
            try {
                reserialized = new ObjectMapper().readTree(s).toString();
            } catch (Exception e) {
                throw new AssertionError(e);
            }
            assertTrue(!reserialized.contains("\\u5b85")
                            && !reserialized.contains("\"kind\"  :")
                            && !reserialized.endsWith("\n"),
                    "re-serialize 後仍保留原特徵？那本測試就驗不到 A2 了");
        }

        @Test
        @DisplayName("header 走白名單：只有簽章 header 會被轉發")
        void onlySignatureHeadersAreForwarded() {
            String validSig = hmacHex(awkwardBody());
            service.receive(awkwardBody(), Map.of(
                    "X-Uber-Signature", validSig,
                    "x-postmates-signature", "sig-old",
                    "Cookie", "session=should-not-travel",
                    "Authorization", "Bearer should-not-travel"));

            ProviderWebhookEnvelope env =
                    ProviderWebhookEnvelope.parse(capturedOutboxPayload().getBytes(StandardCharsets.UTF_8));
            assertEquals(Map.of("x-uber-signature", validSig, "x-postmates-signature", "sig-old"),
                    env.headers(),
                    "只有 API 總覽 §6.1 的兩個簽章 header 可以進 MQ（10 檔 §10.3）");
        }
    }

    @Nested
    @DisplayName("🔴 inbox 去重（Uber 重送同一個 event_id）")
    class InboxDedup {

        @Test
        @DisplayName("同 event_key 第二次進來（existing=ENQUEUED）⇒ 不得再投遞 MQ")
        void replayOfEnqueuedEventDoesNotRepublish() {
            WebhookInbox existing = new WebhookInbox();
            existing.setId(1L);
            existing.setStatus(WebhookInboxStatus.ENQUEUED.value());
            doThrow(new DuplicateKeyException("uq_inbox_event")).when(inboxTxOps).insertInbox(any());
            when(inboxMapper.findByEventKey(eq(PROVIDER_VALUE), anyString(), anyString()))
                    .thenReturn(existing);

            service.receive(awkwardBody(), signed(awkwardBody()));

            verify(outboxTxOps, never()).enqueueAndMark(anyLong(), anyString(), any());
            verify(outboxTxOps, never()).enqueueAndMark(anyLong(), any());
        }

        @Test
        @DisplayName("上次卡在 RECEIVED ⇒ 走 replay 補建 outbox（不是靜默丟棄）")
        void orphanReceivedRowIsReplayed() {
            WebhookInbox existing = new WebhookInbox();
            existing.setId(7L);
            existing.setStatus(WebhookInboxStatus.RECEIVED.value());
            doThrow(new DuplicateKeyException("uq_inbox_event")).when(inboxTxOps).insertInbox(any());
            when(inboxMapper.findByEventKey(eq(PROVIDER_VALUE), anyString(), anyString()))
                    .thenReturn(existing);

            service.receive(awkwardBody(), signed(awkwardBody()));

            verify(outboxTxOps).enqueueAndMark(eq(7L), eq(PROVIDER_VALUE), any());
        }
    }

    @Nested
    @DisplayName("§6.3.2 落點：只有 delivery_status 進 MQ")
    class RouteDecision {

        @Test
        @DisplayName("delivery_status → outbox route 用 uber.shipping.status，且 provider 存的是碼值")
        void deliveryStatusGoesToUberRoutingKey() {
            service.receive(awkwardBody(), signed(awkwardBody()));

            @SuppressWarnings("unchecked")
            ArgumentCaptor<List<WebhookOutboxTxOps.OutboxRoute>> captor =
                    ArgumentCaptor.forClass(List.class);
            verify(outboxTxOps).enqueueAndMark(eq(4242L), eq(PROVIDER_VALUE), captor.capture());
            WebhookOutboxTxOps.OutboxRoute route = captor.getValue().get(0);
            assertEquals("payuni.events", route.exchange());
            assertEquals("uber.shipping.status", route.topic());
            assertEquals(MqConstants.EXCHANGE_PAYUNI_WEBHOOK, route.exchange());
            assertEquals(MqConstants.ROUTING_KEY_UBER_SHIPPING_STATUS, route.topic());

            ArgumentCaptor<WebhookInbox> inboxCaptor = ArgumentCaptor.forClass(WebhookInbox.class);
            verify(inboxTxOps).insertInbox(inboxCaptor.capture());
            WebhookInbox inbox = inboxCaptor.getValue();
            assertEquals(PROVIDER_VALUE, inbox.getProvider());
            assertEquals("event.delivery_status", inbox.getWebhookType());
            assertEquals(null, inbox.getMerId(), "Uber 沒有 MerID，不得塞 sentinel（66 §5.1.6）");
        }

        @Test
        @DisplayName("courier_update 落 inbox 但不進 MQ（陷阱 A3：每 20 秒一發）")
        void courierUpdateLandsButDoesNotPublish() {
            signedReceive(("{\"kind\":\"event.courier_update\",\"id\":\"evt_c1\","
                    + "\"delivery_id\":\"del_abc\"}").getBytes(StandardCharsets.UTF_8));

            verify(inboxTxOps).insertInbox(any());
            verify(inboxTxOps).markIgnored(4242L);
            verify(outboxTxOps, never()).enqueueAndMark(anyLong(), anyString(), any());
        }

        @Test
        @DisplayName("refund_request 落 inbox 但不進 MQ（§6.3.6：錢的事不自動化，且本波無落點）")
        void refundRequestLandsButDoesNotPublish() {
            signedReceive(("{\"kind\":\"event.refund_request\",\"id\":\"evt_r1\"}")
                    .getBytes(StandardCharsets.UTF_8));

            verify(inboxTxOps).insertInbox(any());
            verify(inboxTxOps).markIgnored(4242L);
            verify(outboxTxOps, never()).enqueueAndMark(anyLong(), anyString(), any());
        }

        @Test
        @DisplayName("body 不是 JSON → 仍落 inbox（丟掉才是永久遺失），不拋例外")
        void garbageBodyStillLands() {
            signedReceive("<html>502</html>".getBytes(StandardCharsets.UTF_8));

            ArgumentCaptor<WebhookInbox> inboxCaptor = ArgumentCaptor.forClass(WebhookInbox.class);
            verify(inboxTxOps).insertInbox(inboxCaptor.capture());
            assertEquals("unparseable", inboxCaptor.getValue().getWebhookType());
            // JSONB 塞不進非 JSON ⇒ 可讀欄位留 null，證據在 raw_envelope
            assertEquals(null, inboxCaptor.getValue().getDecryptedPayload());
            assertTrue(inboxCaptor.getValue().getRawEnvelope().contains("raw_body_base64"));
            verify(outboxTxOps, never()).enqueueAndMark(anyLong(), anyString(), any());
        }
    }

    @Nested
    @DisplayName("🔴 落不了地就必須讓例外往上（→ 5xx → Uber 重送）")
    class FailureIsNotSwallowed {

        @Test
        @DisplayName("inbox 寫入失敗（非 DupKey）⇒ 例外往上拋，不得吞掉")
        void inboxWriteFailurePropagates() {
            doThrow(new RuntimeException("DB down")).when(inboxTxOps).insertInbox(any());
            assertThrows(RuntimeException.class, () -> service.receive(awkwardBody(), signed(awkwardBody())));
        }

        @Test
        @DisplayName("outbox 寫入失敗 ⇒ 標 FAILED 後仍往上拋")
        void outboxFailureMarksFailedAndPropagates() {
            doThrow(new RuntimeException("outbox down"))
                    .when(outboxTxOps).enqueueAndMark(anyLong(), anyString(), any());
            assertThrows(RuntimeException.class, () -> service.receive(awkwardBody(), signed(awkwardBody())));
            verify(inboxTxOps).markFailed(eq(4242L), anyString());
        }
    }

    @Nested
    @DisplayName("🔴 驗簽在落地之前（未認證的事件壓制攻擊面）")
    class SignatureGate {

        /**
         * 🔴 <b>這是本檔最重要的一條。</b>
         *
         * <p>攻擊形狀：{@code webhook_inbox} 有
         * {@code uq_inbox_event UNIQUE (provider, webhook_type, event_key)}，
         * 而 event_key 來自 payload 裡的 {@code id}／{@code delivery_id}——
         * 那是攻擊者<b>猜得到或側面取得</b>的東西。
         * 若未驗簽就落地，攻擊者只要搶先送一個同 event_id 的空事件，
         * 真正的 Uber 事件到達時就會被判為重複而丟棄。
         *
         * <p>⚠️ 注意斷言的重點<b>不是「有沒有拋例外」</b>，而是
         * <b>「inbox 有沒有被寫入」</b>：先落地再驗簽、驗完才 rollback 的實作一樣會拋，
         * 但那個 UNIQUE 鍵位在交易期間已經被佔住過，而且失敗後還可能留下
         * {@code status=FAILED} 的殘留列。只驗例外型別的測試對那種實作零鑑別力。
         */
        @Test
        @DisplayName("偽造簽章 ⇒ 拋 401 例外，且**完全不碰 inbox**（不得佔用去重鍵位）")
        void forgedSignatureNeverReachesInbox() {
            byte[] body = awkwardBody();

            assertThrows(UberWebhookSignatureException.class,
                    () -> service.receive(body, Map.of(
                            UberWebhookVerifier.SIGNATURE_HEADER,
                            "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")));

            verifyNoInteractions(inboxTxOps);
            verifyNoInteractions(inboxMapper);
            verifyNoInteractions(outboxTxOps);
        }

        @Test
        @DisplayName("完全沒有簽章 header ⇒ 同樣被拒（缺席不等於豁免）")
        void missingSignatureIsRejected() {
            assertThrows(UberWebhookSignatureException.class,
                    () -> service.receive(awkwardBody(), Map.of()));
            verifyNoInteractions(inboxTxOps);
        }

        /**
         * 🔴 簽的必須是 <b>raw bytes</b>：對 parse 後 re-serialize 的版本算出來的簽章
         * 必須被拒。少了這一條，把驗簽對象改成正規化後的 JSON 不會有任何東西轉紅，
         * 而那個改動會讓驗簽<b>偶發</b>失敗（陷阱 A2）。
         */
        @Test
        @DisplayName("對 re-serialize 後的 JSON 算的簽章必須被拒")
        void signatureOverReserializedJsonIsRejected() {
            byte[] raw = awkwardBody();
            byte[] reserialized;
            try {
                reserialized = new ObjectMapper().readTree(new String(raw, StandardCharsets.UTF_8))
                        .toString().getBytes(StandardCharsets.UTF_8);
            } catch (Exception e) {
                throw new AssertionError(e);
            }
            // 前提：兩者確實不同，否則本條沒有鑑別力。
            assertTrue(raw.length != reserialized.length,
                    "fixture 的 raw 與 re-serialize 後相同 ⇒ 本條驗不到東西");

            assertThrows(UberWebhookSignatureException.class,
                    () -> service.receive(raw, Map.of(
                            UberWebhookVerifier.SIGNATURE_HEADER, hmacHex(reserialized))));
            verifyNoInteractions(inboxTxOps);
        }

        @Test
        @DisplayName("合法簽章 ⇒ 照常落地（陽性對照；沒有它，一個「一律拒絕」的 bug 會全綠）")
        void validSignatureStillLands() {
            signedReceive(awkwardBody());
            verify(inboxTxOps).insertInbox(any());
        }
    }

    /** 簽好再送。 */
    private void signedReceive(byte[] body) {
        service.receive(body, signed(body));
    }

    /** 以 TEST_SIGNING_KEY 對 body 簽好的 header。 */
    private static Map<String, String> signed(byte[] body) {
        return Map.of(UberWebhookVerifier.SIGNATURE_HEADER, hmacHex(body));
    }

    /**
     * HMAC-SHA256 的 hex。
     *
     * ⚠️ 這裡**必須**自己算而不能呼叫被測物：本檔驗的是「ingress 有沒有驗簽」，
     * 期望值若由被測物產生，驗簽被整支拿掉時這些案例照樣會綠。
     */
    private static String hmacHex(byte[] body) {
        try {
            javax.crypto.Mac mac = javax.crypto.Mac.getInstance("HmacSHA256");
            mac.init(new javax.crypto.spec.SecretKeySpec(
                    TEST_SIGNING_KEY.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
            byte[] sig = mac.doFinal(body);
            StringBuilder sb = new StringBuilder(sig.length * 2);
            for (byte b : sig) {
                sb.append(String.format("%02x", b));
            }
            return sb.toString();
        } catch (Exception e) {
            throw new AssertionError(e);
        }
    }
}

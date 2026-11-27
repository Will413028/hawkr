package io.github.will413028.hawkr.webhook.uber;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.ProviderWebhookEnvelope;
import io.github.will413028.hawkr.common.webhook.UberWebhookVerifier;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookVerifyStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import io.github.will413028.hawkr.webhook.service.WebhookInboxTxOps;
import io.github.will413028.hawkr.webhook.service.WebhookOutboxTxOps;
import io.github.will413028.hawkr.webhook.service.WebhookOutboxTxOps.OutboxRoute;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;

import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Uber Direct webhook 的 ingress（66 §6.3／§6.4.1）。
 *
 * <h2>本服務只做四件事：驗簽、落地、去重、投遞</h2>
 * 🔴 <b>驗簽在最前面，而且必須在落地之前</b>——理由見 {@link #receive} 內的註解
 * （未驗簽就落地 ＋ inbox 去重 ＝ 未認證的事件壓制攻擊面）。
 * 「這一則回調是什麼狀態、對應哪一張配送單、能不能推進狀態機」全部在 shipping-service 側的
 * {@code InboundCapability}（66 §6.4.1 逐字：webhook 服務「只負責驗簽、入 inbox、發事件」）。
 *
 * <h2>🔴 回應只有 200 與 5xx 兩種</h2>
 * Uber 只對 {@code 500/502/503/504} 與 timeout 重試，<b>4xx 不重試</b>
 * ⇒ 我方回 4xx ＝ 該事件永久丟失（API 總覽 §6.3）。
 * 所以本服務的規則是「<b>能落地就 200，落不了地就 5xx</b>」：
 * <ul>
 *   <li>body 不是 JSON／不是我方訂閱的事件 ⇒ 仍然<b>落 inbox</b>（標 IGNORED）＋ 200
 *       —— 丟掉才是永久遺失，留著至少查得到</li>
 *   <li>DB 寫不進去 ⇒ 例外往上拋 ⇒ 500 ⇒ Uber 重送（66 §6.3.4 逐字）</li>
 * </ul>
 *
 * <h2>🔴 raw bytes 全程不重新序列化</h2>
 * 驗簽是 HMAC-SHA256 over raw body；先 parse 再 re-serialize 會<b>偶發</b>失敗、測不出來、
 * 上線才炸（66 §6.3.1 陷阱 A2）。本服務把 HTTP body 的那一份位元組 base64 進
 * {@link ProviderWebhookEnvelope#rawBodyBase64()}，consumer 解出來的與 Uber 送來的逐位元組相同。
 * ⚠️ {@code webhook_inbox.decrypted_payload} 是 <b>JSONB</b>，PostgreSQL 會正規化鍵序與空白
 * ⇒ <b>那一欄不可用於驗簽</b>；忠實的那一份在 {@code raw_envelope.raw_body_base64}。
 */
@Service
public class UberWebhookReceiveService {

    private static final ObsLogger log = ObsLogger.forClass(UberWebhookReceiveService.class);

    /**
     * 🔴 轉發到 MQ 的 header <b>白名單</b>，不是全部轉發。
     *
     * <p>兩個名字來自 API 總覽 §6.1：{@code x-uber-signature}（新）／
     * {@code x-postmates-signature}（舊，Delivery Status／Courier Update 才接受）。
     * 白名單而非黑名單的理由：ingress 前面有 Gateway／ingress controller／CDN，
     * 它們會加上什麼 header 不在我方控制內，而 MQ 訊息會被持久化、被 log、被 re-drive
     * ⇒ 全量轉發等於把任意上游 header（含可能的 cookie／authorization）寫進佇列（10 檔 §10.3）。
     */
    static final List<String> FORWARDED_SIGNATURE_HEADERS =
            List.of("x-uber-signature", "x-postmates-signature");

    private final WebhookInboxMapper inboxMapper;
    private final WebhookInboxTxOps inboxTxOps;
    private final WebhookOutboxTxOps outboxTxOps;
    private final UberWebhookProviderCode providerCode;
    private final ObjectMapper objectMapper;
    private final UberWebhookVerifier verifier;

    public UberWebhookReceiveService(WebhookInboxMapper inboxMapper,
                                      WebhookInboxTxOps inboxTxOps,
                                      WebhookOutboxTxOps outboxTxOps,
                                      UberWebhookProviderCode providerCode,
                                      ObjectMapper objectMapper,
                                      UberWebhookVerifier verifier) {
        this.inboxMapper = inboxMapper;
        this.inboxTxOps = inboxTxOps;
        this.outboxTxOps = outboxTxOps;
        this.providerCode = providerCode;
        this.objectMapper = objectMapper;
        this.verifier = verifier;
    }

    /**
     * @param rawBody HTTP body 的<b>原始位元組</b>（controller 用 {@code @RequestBody byte[]} 取得）
     * @param headers HTTP header（任意大小寫；本方法自己過白名單並正規化成小寫 key）
     * @throws RuntimeException 落地失敗 —— 呼叫端<b>不得</b>吞掉，必須讓它變成 5xx
     */
    public void receive(byte[] rawBody, Map<String, String> headers) {
        Map<String, String> signatureHeaders = filterSignatureHeaders(headers);

        if (!verifier.verify(rawBody, signatureHeaders)) {
            throw new UberWebhookSignatureException("Uber webhook 驗簽失敗");
        }

        UberWebhookEvent event = UberWebhookParser.parse(rawBody);
        String provider = providerCode.value();

        WebhookInbox inbox = buildInbox(provider, event, rawBody, signatureHeaders);
        try {
            inboxTxOps.insertInbox(inbox);
        } catch (DuplicateKeyException dup) {
            WebhookInbox existing = inboxMapper.findByEventKey(provider, event.kind(), event.eventKey());
            if (existing == null) {
                log.error("UberWebhookReceiveService.dedup.lookupMissing",
                        "DupKey 後找不到 existing row eventKey=" + event.eventKey(), dup);
                return;
            }
            if (!isReplayable(existing.getStatus())) {
                log.warn("UberWebhookReceiveService.dedup.hit",
                        "重複 Uber webhook 已忽略 eventKey=" + event.eventKey()
                                + " kind=" + event.kind() + " status=" + existing.getStatus()
                                + " keyTier=" + event.keyTier(),
                        "eventKey", event.eventKey());
                return;
            }
            log.warn("UberWebhookReceiveService.replay",
                    "進入 replay 路徑，補建 outbox eventKey=" + event.eventKey()
                            + " prevStatus=" + existing.getStatus(),
                    "eventKey", event.eventKey());
            inbox = existing;
        }

        if (event.route() != UberWebhookEvent.Route.PUBLISH) {
            inboxTxOps.markIgnored(inbox.getId());
            logNonPublished(event);
            return;
        }

        try {
            ProviderWebhookEnvelope envelope = ProviderWebhookEnvelope.ofJson(
                    provider, event.kind(), event.eventKey(), rawBody, signatureHeaders,
                    DateTimeUtils.nowUtc().toString());
            OutboxRoute route = new OutboxRoute(event.eventKey(),
                    MqConstants.EXCHANGE_PAYUNI_WEBHOOK,
                    MqConstants.ROUTING_KEY_UBER_SHIPPING_STATUS,
                    objectMapper.writeValueAsString(envelope.toWireMap()));
            outboxTxOps.enqueueAndMark(inbox.getId(), provider, List.of(route));
            log.info("UberWebhookReceiveService.receive.success",
                    "Uber webhook 處理完成 kind=" + event.kind(), "eventKey", event.eventKey());
        } catch (Exception e) {
            try {
                inboxTxOps.markFailed(inbox.getId(), e.getMessage());
            } catch (Exception markErr) {
                log.error("UberWebhookReceiveService.markFailed.error",
                        "T2 失敗後標 FAILED 又失敗 eventKey=" + event.eventKey(), markErr);
            }
            log.error("UberWebhookReceiveService.receive.t2Failed",
                    "T2 失敗，inbox 已標 FAILED，將回 5xx 觸發 Uber 重送 eventKey=" + event.eventKey(), e);
            throw e instanceof RuntimeException re ? re : new RuntimeException(e);
        }
    }

    private void logNonPublished(UberWebhookEvent event) {
        if (UberWebhookParser.routeOf(event.kind()) == UberWebhookEvent.Route.INBOX_ONLY) {
            log.info("UberWebhookReceiveService.inboxOnly",
                    "事件已落地，依 66 §6.3.2 不投遞 MQ kind=" + event.kind(),
                    "eventKey", event.eventKey());
            return;
        }
        log.error("UberWebhookReceiveService.unhandledKind",
                "收到未處理的 Uber webhook 事件，已落 inbox 並標 IGNORED kind=" + event.kind()
                        + " eventKey=" + event.eventKey(),
                "eventKey", event.eventKey());
    }

    private static boolean isReplayable(String status) {
        return WebhookInboxStatus.RECEIVED.value().equals(status)
                || WebhookInboxStatus.FAILED.value().equals(status);
    }

    /** 白名單 ＋ key 正規化成小寫（HTTP header 名大小寫不敏感，而不同 ingress 正規化結果不同）。 */
    static Map<String, String> filterSignatureHeaders(Map<String, String> headers) {
        Map<String, String> out = new LinkedHashMap<>();
        if (headers == null) {
            return out;
        }
        headers.forEach((k, v) -> {
            if (k == null || v == null) {
                return;
            }
            String lower = k.toLowerCase(Locale.ROOT);
            if (FORWARDED_SIGNATURE_HEADERS.contains(lower)) {
                out.put(lower, v);
            }
        });
        return out;
    }

    private WebhookInbox buildInbox(String provider, UberWebhookEvent event,
                                     byte[] rawBody, Map<String, String> signatureHeaders) {
        WebhookInbox inbox = new WebhookInbox();
        inbox.setProvider(provider);
        inbox.setMerId(null);
        inbox.setWebhookType(event.kind());
        inbox.setEventKey(event.eventKey());
        inbox.setStatus(WebhookInboxStatus.RECEIVED.value());
        inbox.setVerifyStatus(WebhookVerifyStatus.PASSED.value());
        inbox.setRawEnvelope(toJson(Map.of(
                ProviderWebhookEnvelope.KEY_RAW_BODY_BASE64,
                java.util.Base64.getEncoder().encodeToString(rawBody == null ? new byte[0] : rawBody),
                ProviderWebhookEnvelope.KEY_HEADERS, signatureHeaders)));
        inbox.setDecryptedPayload(readableBody(rawBody));
        inbox.setReceivedAt(DateTimeUtils.nowUtc());
        return inbox;
    }

    /**
     * 給人看的那一份。Uber 沒有加密，所以「decrypted」對它只是「可讀」的意思。
     *
     * <p>🔴 <b>不可用於驗簽</b>：欄位型別是 JSONB，PostgreSQL 會重排鍵序、去空白。
     * body 不是合法 JSON 時回 null（JSONB 塞不進去），證據仍在 {@code raw_envelope}。
     */
    private String readableBody(byte[] rawBody) {
        if (rawBody == null || rawBody.length == 0) {
            return null;
        }
        String text = new String(rawBody, StandardCharsets.UTF_8);
        try {
            objectMapper.readTree(text);
            return text;
        } catch (Exception e) {
            return null;
        }
    }

    private String toJson(Object obj) {
        try {
            return objectMapper.writeValueAsString(obj);
        } catch (Exception e) {
            log.error("UberWebhookReceiveService.toJson.failed", "JSON 序列化失敗", e);
            return "{}";
        }
    }
}

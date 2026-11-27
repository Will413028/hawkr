package io.github.will413028.hawkr.webhook.service;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PayUniEventType;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.common.utils.code.WebhookVerifyStatus;
import io.github.will413028.hawkr.payuni.webhook.PayUniWebhookParser;
import io.github.will413028.hawkr.payuni.webhook.WebhookParseResult;
import io.github.will413028.hawkr.payuni.webhook.WebhookVerifyException;
import io.github.will413028.hawkr.webhook.infrastructure.mq.PayUniEventRouting;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import io.github.will413028.hawkr.webhook.service.WebhookOutboxTxOps.OutboxRoute;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;

/**
 * Webhook 接收核心服務。
 * <p>
 * 設計書 §4.5「事務邊界設計（Dedup Commitment 修正）」：
 * <ul>
 *   <li>{@code receive(...)} 本身**不開外層事務**</li>
 *   <li>T1（inbox INSERT）由 {@link WebhookInboxTxOps#insertInbox} REQUIRES_NEW 獨立 commit</li>
 *   <li>T2（outbox INSERT + inbox status=ENQUEUED）由 {@link WebhookOutboxTxOps#enqueueAndMark} REQUIRES_NEW</li>
 *   <li>T1 DupKey 後 SELECT existing 須在 T1 之外（§2.5.1 PostgreSQL aborted state）</li>
 *   <li>T2 失敗 → markFailed + rethrow → controller 回 5xx → PayUni 重送 → T1 dedup 命中 FAILED → REPLAY 路徑</li>
 *   <li>UNKNOWN webhookType → 標 IGNORED 終態，不投遞 MQ</li>
 * </ul>
 */
@Service
public class WebhookReceiveService {

    private static final ObsLogger log = ObsLogger.forClass(WebhookReceiveService.class);

    private final PayUniWebhookParser webhookParser;
    private final WebhookInboxMapper inboxMapper;
    private final ObjectMapper objectMapper;
    private final FailedInboxRecorder failedInboxRecorder;
    private final WebhookInboxTxOps inboxTxOps;
    private final WebhookOutboxTxOps outboxTxOps;

    public WebhookReceiveService(PayUniWebhookParser webhookParser,
                                  WebhookInboxMapper inboxMapper,
                                  ObjectMapper objectMapper,
                                  FailedInboxRecorder failedInboxRecorder,
                                  WebhookInboxTxOps inboxTxOps,
                                  WebhookOutboxTxOps outboxTxOps) {
        this.webhookParser = webhookParser;
        this.inboxMapper = inboxMapper;
        this.objectMapper = objectMapper;
        this.failedInboxRecorder = failedInboxRecorder;
        this.inboxTxOps = inboxTxOps;
        this.outboxTxOps = outboxTxOps;
    }

    /**
     * 接收並處理 PayUni Webhook 統一入口。
     */
    public void receive(String encryptInfo, String hashInfo) {
        WebhookParseResult parseResult;
        try {
            parseResult = webhookParser.parse(encryptInfo, hashInfo);
        } catch (WebhookVerifyException e) {
            log.error("WebhookReceiveService.verify.failed", "驗簽失敗", e);
            failedInboxRecorder.record(encryptInfo, hashInfo, e.getMessage());
            return;
        } catch (Exception e) {
            log.error("WebhookReceiveService.parse.failed", "解析失敗", e);
            failedInboxRecorder.record(encryptInfo, hashInfo, e.getMessage());
            return;
        }

        Map<String, String> payload = parseResult.rawMap();
        String merId = payload.getOrDefault("MerID", "UNKNOWN");
        PayUniEventType actualType = resolveType(parseResult);
        String eventKey = EventKeyBuilder.build(actualType, payload);

        WebhookInbox inbox = buildInbox(merId, actualType, eventKey, encryptInfo, hashInfo, payload);

        try {
            inboxTxOps.insertInbox(inbox);
            log.info("WebhookReceiveService.inbox.saved", "Webhook 已入庫", "eventKey", eventKey);
        } catch (DuplicateKeyException dup) {
            WebhookInbox existing = inboxMapper.findByEventKey(
                    WebhookProvider.PAYUNI.value(), actualType.value(), eventKey);
            if (existing == null) {
                log.error("WebhookReceiveService.dedup.lookupMissing",
                        "DupKey 後找不到 existing row eventKey=" + eventKey, dup);
                return;
            }
            inbox = handleDedupHit(existing, payload, eventKey);
            if (inbox == null) {
                return;
            }
        }

        if (actualType == PayUniEventType.UNKNOWN) {
            inboxTxOps.markIgnored(inbox.getId());
            log.warn("WebhookReceiveService.unknown.markedIgnored",
                    "UNKNOWN webhook 已寫入 inbox 並標 IGNORED eventKey=" + eventKey,
                    "eventKey", eventKey);
            return;
        }

        try {
            String envelopeJson = toJson(buildEventEnvelope(actualType, merId, eventKey, payload));
            List<OutboxRoute> routes = buildRoutes(actualType, eventKey, envelopeJson, payload);
            outboxTxOps.enqueueAndMark(inbox.getId(), routes);
            log.info("WebhookReceiveService.receive.success",
                    "Webhook 處理完成", "eventKey", eventKey);
        } catch (Exception e) {
            try {
                inboxTxOps.markFailed(inbox.getId(), e.getMessage());
            } catch (Exception markErr) {
                log.error("WebhookReceiveService.markFailed.error",
                        "T2 失敗後標 FAILED 又失敗 eventKey=" + eventKey, markErr);
            }
            log.error("WebhookReceiveService.receive.t2Failed",
                    "T2 失敗，inbox 已標 FAILED，將回 5xx 觸發 PayUni 重送 eventKey=" + eventKey, e);
            throw e instanceof RuntimeException re ? re : new RuntimeException(e);
        }
    }

    /**
     * DupKey 後依 existing.status 決定 dedup hit 或進入 replay。
     * @return null 表示 dedup hit / lookup 異常（呼叫端 silent return）；非 null 表示 replay 用的 inbox row
     */
    private WebhookInbox handleDedupHit(WebhookInbox existing, Map<String, String> payload, String eventKey) {
        String status = existing.getStatus();
        if (WebhookInboxStatus.ENQUEUED.value().equals(status)
                || WebhookInboxStatus.IGNORED.value().equals(status)
                || WebhookInboxStatus.DEDUPED.value().equals(status)) {
            log.warn("WebhookReceiveService.dedup.hit",
                    "重複 Webhook 已忽略 eventKey=" + eventKey + " status=" + status
                            + " payload=" + toJson(payload),
                    "eventKey", eventKey);
            return null;
        }
        if (WebhookInboxStatus.RECEIVED.value().equals(status)
                || WebhookInboxStatus.FAILED.value().equals(status)) {
            log.warn("WebhookReceiveService.receive.replay",
                    "進入 replay 路徑，補建 outbox eventKey=" + eventKey + " prevStatus=" + status,
                    "eventKey", eventKey);
            return existing;
        }
        log.error("WebhookReceiveService.dedup.unexpectedStatus",
                "DupKey 後 existing 狀態異常 eventKey=" + eventKey + " status=" + status,
                "eventKey", eventKey);
        return null;
    }

    private WebhookInbox buildInbox(String merId, PayUniEventType type, String eventKey,
                                     String encryptInfo, String hashInfo, Map<String, String> payload) {
        WebhookInbox inbox = new WebhookInbox();
        inbox.setProvider(WebhookProvider.PAYUNI.value());
        inbox.setMerId(merId);
        inbox.setWebhookType(type.value());
        inbox.setEventKey(eventKey);
        inbox.setStatus(WebhookInboxStatus.RECEIVED.value());
        inbox.setVerifyStatus(WebhookVerifyStatus.PASSED.value());
        inbox.setRawEnvelope(toJson(Map.of("EncryptInfo", encryptInfo, "HashInfo", hashInfo)));
        inbox.setDecryptedPayload(toJson(payload));
        inbox.setReceivedAt(DateTimeUtils.nowUtc());
        return inbox;
    }

    private List<OutboxRoute> buildRoutes(PayUniEventType type, String eventKey,
                                           String envelopeJson, Map<String, String> payload) {
        if (type == PayUniEventType.AGENT_NOTIFY) {
            List<OutboxRoute> agentRoutes = new ArrayList<>(1);
            if (payload.containsKey("MerStatus")) {
                agentRoutes.add(new OutboxRoute(eventKey,
                        MqConstants.EXCHANGE_SELLER,
                        MqConstants.ROUTING_KEY_SELLER_REVIEW_RESULT,
                        envelopeJson));
            }
            return agentRoutes;
        }
        List<OutboxRoute> routes = new ArrayList<>(1);
        routes.add(new OutboxRoute(eventKey,
                PayUniEventRouting.exchange(type),
                PayUniEventRouting.topic(type),
                envelopeJson));
        return routes;
    }

    private PayUniEventType resolveType(WebhookParseResult parseResult) {
        return switch (parseResult.type()) {
            case SHIPPING_STATUS -> PayUniEventType.SHIPPING_STATUS;
            case SHIPPING_PRINT -> PayUniEventType.SHIPPING_PRINT;
            case AGENT_NOTIFY -> PayUniEventType.AGENT_NOTIFY;
            case UNKNOWN -> PayUniEventType.UNKNOWN;
        };
    }

    private Map<String, Object> buildEventEnvelope(PayUniEventType type, String merId,
                                                     String eventKey, Map<String, String> payload) {
        return Map.of(
                "event_id", UUID.randomUUID().toString(),
                "provider", WebhookProvider.PAYUNI.value(),
                "webhook_type", type.value(),
                "mer_id", merId,
                "event_key", eventKey,
                "occurred_at", DateTimeUtils.nowUtc().toString(),
                "payload", payload,
                "meta", Map.of("version", "1.0")
        );
    }

    private String toJson(Object obj) {
        try {
            return objectMapper.writeValueAsString(obj);
        } catch (JsonProcessingException e) {
            log.error("WebhookReceiveService.toJson.failed", "JSON 序列化失敗", e);
            return "{}";
        }
    }
}

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
        // Step 1: 驗簽 + 解密 + 解析（無事務）
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

        // Step 2: T1 — INSERT inbox（REQUIRES_NEW）
        WebhookInbox inbox = buildInbox(merId, actualType, eventKey, encryptInfo, hashInfo, payload);

        try {
            inboxTxOps.insertInbox(inbox);
            log.info("WebhookReceiveService.inbox.saved", "Webhook 已入庫", "eventKey", eventKey);
        } catch (DuplicateKeyException dup) {
            // T1 已 rollback 釋放 connection，後續 SELECT 在新交易中執行
            WebhookInbox existing = inboxMapper.findByEventKey(
                    WebhookProvider.PAYUNI.value(), actualType.value(), eventKey);
            if (existing == null) {
                // 理論不會走到（UNIQUE 撞了卻找不到 row）
                log.error("WebhookReceiveService.dedup.lookupMissing",
                        "DupKey 後找不到 existing row eventKey=" + eventKey, dup);
                return;
            }
            inbox = handleDedupHit(existing, payload, eventKey);
            if (inbox == null) {
                return; // dedup hit / lookup 異常：silent return
            }
            // 否則進入 replay 路徑：inbox 是 existing，繼續往下走
        }
        // 注意：T1 其他例外（非 DupKey）不被 catch，向上拋 → controller 回 500 → PayUni 重送

        // Step 3: UNKNOWN 處理（§2.2 業務 pivot 2026-05-14：保守失敗處理）
        // - payload 無 MerID → 無法綁定任何業務 task，維持既有 IGNORED 終態（不投遞 MQ）
        // - payload 含 MerID → 投遞 payuni.unknown.fallback 由 aftersales / payment 兩個 fallback consumer 接手
        if (actualType == PayUniEventType.UNKNOWN) {
            String rawMerId = payload.get("MerID");
            boolean hasMerId = rawMerId != null && !rawMerId.isBlank();
            if (!hasMerId) {
                inboxTxOps.markIgnored(inbox.getId());
                log.warn("WebhookReceiveService.unknown.noMerId.markedIgnored",
                        "UNKNOWN webhook 無 MerID 無法綁定，已寫入 inbox 並標 IGNORED eventKey=" + eventKey,
                        "eventKey", eventKey);
                return;
            }
            // 含 MerID：log.error 觸發 alert（§F.5 universal alert），然後繼續走 Step 4 T2 投遞 fallback queue
            log.error("WebhookReceiveService.unknown.fallback",
                    "UNKNOWN webhook 含 MerID，投遞 fallback queue 由下游 consumer 接手 merId=" + rawMerId
                            + " eventKey=" + eventKey,
                    "eventKey", eventKey);
            // 不 return，繼續 Step 4
        }

        // Step 4: T2 — outbox INSERT(s) + inbox status=ENQUEUED（REQUIRES_NEW）
        try {
            String envelopeJson = toJson(buildEventEnvelope(actualType, merId, eventKey, payload));
            List<OutboxRoute> routes = buildRoutes(actualType, eventKey, envelopeJson, payload);
            outboxTxOps.enqueueAndMark(inbox.getId(), routes);
            log.info("WebhookReceiveService.receive.success",
                    "Webhook 處理完成", "eventKey", eventKey);
        } catch (Exception e) {
            // T2 失敗：標 inbox FAILED（REQUIRES_NEW，不被 T2 rollback 牽連）→ rethrow → 5xx → PayUni 重送 → REPLAY
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
            // §2.5.1：dedup hit 必須 log.warn + 含 eventKey + 完整 payload
            log.warn("WebhookReceiveService.dedup.hit",
                    "重複 Webhook 已忽略 eventKey=" + eventKey + " status=" + status
                            + " payload=" + toJson(payload),
                    "eventKey", eventKey);
            return null;
        }
        if (WebhookInboxStatus.RECEIVED.value().equals(status)
                || WebhookInboxStatus.FAILED.value().equals(status)) {
            // Orphan：上次 T2 失敗或卡住，本次 PayUni 重送觸發 replay
            log.warn("WebhookReceiveService.receive.replay",
                    "進入 replay 路徑，補建 outbox eventKey=" + eventKey + " prevStatus=" + status,
                    "eventKey", eventKey);
            return existing;
        }
        // 未知狀態（防呆）
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
        // AGENT_NOTIFY 已收斂為單通道（#1757）：原 payuni.events/payuni.agent.notify primary route
        // 無任何 consumer（過渡期雙通道殘留），已物理移除；僅在含 MerStatus 時發 seller.review.result，
        // 由 seller-service agent_notify_consumer 推進審核狀態。無 MerStatus → 零 route，
        // 行為等價現況（原 primary route 本就被丟棄，webhook 不因 route 減少而 500）。
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
            case PAYMENT_UOP -> PayUniEventType.PAYMENT_UOP;
            case SHIPPING_STATUS -> PayUniEventType.SHIPPING_STATUS;
            case SHIPPING_PRINT -> PayUniEventType.SHIPPING_PRINT;
            case AGENT_NOTIFY -> PayUniEventType.AGENT_NOTIFY;
            case REFUND_NOTIFY -> PayUniEventType.REFUND_NOTIFY;
            case REFUND_ACCEPTED -> PayUniEventType.REFUND_ACCEPTED;
            case UNKNOWN -> PayUniEventType.UNKNOWN;
        };
    }

    private Map<String, Object> buildEventEnvelope(PayUniEventType type, String merId,
                                                     String eventKey, Map<String, String> payload) {
        // 36 §7.4 Conformance Contract：envelope 鍵採 snake_case。
        // PayUniWebhookEnvelope.parse() 只認 snake_case（判定鍵為 event_id）——過渡期的 camelCase
        // 雙鍵 fallback 已於 PR #1004 移除，沒有任何自動適配。parse() 僅剩的另一分支是「裸 payload」
        // （根層無 event_id → 整個 JSON 當 payload，metadata 全空字串），那不是 camelCase 相容路徑。
        // ⇒ 本方法的鍵名若與 parse() 不一致，consumer 讀到的 metadata 會全空且不報錯。
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

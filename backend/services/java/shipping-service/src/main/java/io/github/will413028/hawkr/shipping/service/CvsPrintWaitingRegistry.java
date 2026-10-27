package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.stereotype.Component;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

/**
 * 超商列印同步等待元件
 * <p>
 * 橋接 PrintCvsLabel gRPC 請求（同步等待方）與 ShippingWebhookConsumer（非同步 Webhook 到達方），
 * 讓列印 API 可在指定秒數內同步取得交貨便代碼。
 * <p>
 * 透過 Redis Pub/Sub 支援多實例部署：
 * - complete() 改為 Redis publish，所有實例都能收到通知
 * - onRedisMessage() 從 localMap 找到對應 future 並 complete
 */
@Component
public class CvsPrintWaitingRegistry {

    private static final ObsLogger log = ObsLogger.forClass(CvsPrintWaitingRegistry.class);
    private static final String CHANNEL_PREFIX = "cvs:print:";
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final StringRedisTemplate redisTemplate;

    /**
     * key = shipTradeNo, value = 等待中的 Future（僅存在於本 JVM 實例）
     */
    private final ConcurrentHashMap<String, CompletableFuture<CvsPrintResult>> localMap
            = new ConcurrentHashMap<>();

    public CvsPrintWaitingRegistry(StringRedisTemplate redisTemplate) {
        this.redisTemplate = redisTemplate;
    }

    /**
     * 註冊等待（PrintCvsLabel 呼叫）
     */
    public CompletableFuture<CvsPrintResult> register(String shipTradeNo) {
        CompletableFuture<CvsPrintResult> future = new CompletableFuture<>();
        localMap.put(shipTradeNo, future);
        return future;
    }

    /**
     * 通知結果到達（ShippingWebhookConsumer 呼叫）
     * 透過 Redis publish 通知所有實例
     */
    public void complete(String shipTradeNo, CvsPrintResult result) {
        try {
            String channel = CHANNEL_PREFIX + shipTradeNo;
            String payload = MAPPER.writeValueAsString(result);
            redisTemplate.convertAndSend(channel, payload);
            log.info("CvsPrintWaitingRegistry.published",
                    "已發布 Redis 通知", "shipTradeNo", shipTradeNo);
        } catch (JsonProcessingException e) {
            log.error("CvsPrintWaitingRegistry.serializeError",
                    "序列化失敗: " + e.getMessage(), "shipTradeNo", shipTradeNo);
            // 降級：直接嘗試 local complete
            completeLocal(shipTradeNo, result);
        } catch (Exception e) {
            log.error("CvsPrintWaitingRegistry.publishError",
                    "Redis publish 失敗: " + e.getMessage(), "shipTradeNo", shipTradeNo);
            // 降級：Redis 不可用時退化為本地通知
            completeLocal(shipTradeNo, result);
        }
    }

    /**
     * Redis 訂閱回調（所有實例都會收到，由 CvsPrintRedisConfig 觸發）
     */
    public void onRedisMessage(String channel, String message) {
        String shipTradeNo = channel.substring(CHANNEL_PREFIX.length());
        CompletableFuture<CvsPrintResult> future = localMap.remove(shipTradeNo);
        if (future != null) {
            try {
                CvsPrintResult result = MAPPER.readValue(message, CvsPrintResult.class);
                future.complete(result);
                log.info("CvsPrintWaitingRegistry.completed",
                        "已通知等待中的請求", "shipTradeNo", shipTradeNo);
            } catch (JsonProcessingException e) {
                log.error("CvsPrintWaitingRegistry.deserializeError",
                        "反序列化失敗: " + e.getMessage(), "shipTradeNo", shipTradeNo);
            }
        }
    }

    /**
     * 等待結果（PrintCvsLabel 呼叫，阻塞最多 timeoutSeconds 秒）
     *
     * @return CvsPrintResult 或 null（超時）
     */
    public CvsPrintResult waitAndRemove(String shipTradeNo, CompletableFuture<CvsPrintResult> future,
                                         long timeoutSeconds) {
        try {
            return future.get(timeoutSeconds, TimeUnit.SECONDS);
        } catch (TimeoutException e) {
            log.info("CvsPrintWaitingRegistry.timeout",
                    "等待超時", "shipTradeNo", shipTradeNo);
            return null;
        } catch (Exception e) {
            log.warn("CvsPrintWaitingRegistry.error",
                    "等待異常: " + e.getMessage(), "shipTradeNo", shipTradeNo);
            return null;
        } finally {
            localMap.remove(shipTradeNo);
        }
    }

    /**
     * 降級：Redis 不可用時直接通知本地 future
     */
    private void completeLocal(String shipTradeNo, CvsPrintResult result) {
        CompletableFuture<CvsPrintResult> future = localMap.remove(shipTradeNo);
        if (future != null) {
            future.complete(result);
            log.info("CvsPrintWaitingRegistry.localComplete",
                    "降級為本地通知", "shipTradeNo", shipTradeNo);
        }
    }

    /**
     * Webhook 回填結果
     */
    public record CvsPrintResult(String partnerId, String odno, String validationNo) {}
}

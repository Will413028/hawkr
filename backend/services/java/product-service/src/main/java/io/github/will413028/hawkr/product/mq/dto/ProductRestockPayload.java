package io.github.will413028.hawkr.product.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

/**
 * product.restock MQ 事件 Payload（02 §5.13b / 36 §3.6h / 39 §5.5a）。
 * <p>
 * 觸發點（兩處）：
 * <ol>
 *   <li>StockService.cascadeProductStock 偵測 oldStock=0 AND newStock&gt;0 AND status=4004 AND deleted_at IS NULL</li>
 *   <li>ProductService.updateSellerProductStatus / updateProductStatus 將 status 切到 4004 且當下 stock_qty&gt;0</li>
 * </ol>
 * <p>
 * 由 ProductEventPublisher.publishRestockAfterCommit 包裝 spring afterCommit hook 發布，
 * isSynchronizationActive=false 時 fallback 為 eager publish + WARN log。
 * <p>
 * imageKey 為商品主圖 objectKey（原 imageUrl，49 號設計書 §6.3；顯示 URL 由消費端組），
 * 若商品無圖則為空字串。wire key = image_key（@JsonNaming SnakeCaseStrategy）。
 * occurredAt 為 ISO 8601 UTC 帶 Z 字尾，由 DateTimeUtils 產生。
 *
 * @param eventId     UUID v4（buyer-service 端 dedup 表 PK，39 §5.5a.2）
 * @param eventType   固定 "PRODUCT_RESTOCK"（CodeMeta ProductEventType）
 * @param productId   商品 ID（TSID 字串）
 * @param productName 商品名稱（推播文案 #{productName} 變數）
 * @param imageKey    商品主圖 objectKey（推播圖示，顯示 URL 由消費端組）
 * @param storeId     賣家 store 代理鍵（D7 Stage 2 更名，原 seller_account_id）
 * @param occurredAt  事件發生時間（ISO 8601 UTC）
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record ProductRestockPayload(
        String eventId,
        String eventType,
        String productId,
        String productName,
        String imageKey,
        Long storeId,
        String occurredAt
) {}

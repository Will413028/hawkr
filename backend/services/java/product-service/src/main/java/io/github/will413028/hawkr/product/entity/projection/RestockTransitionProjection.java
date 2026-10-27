package io.github.will413028.hawkr.product.entity.projection;

import java.time.LocalDateTime;

/**
 * 補貨 transition 偵測 projection（02 §5.13b）。
 * <p>
 * 用於 StockService.cascadeProductStock reflow 前一次批量查詢
 * 需要的 product 層級欄位（不回傳完整 ProductEntity 以遵守 docs/開發規範/Java 微服務開發規範.md §9.2
 * partial projection 禁令）。
 * <p>
 * primaryObjectKey = gallery->0->>'objectKey'，由 Caller 原樣填入 MQ payload 的
 * imageKey 欄位（49 §6.3：MQ 只帶 objectKey，顯示 URL 由消費端組）。
 */
public record RestockTransitionProjection(
        Long productId,
        Long stockQty,
        String status,
        String name,
        String primaryObjectKey,
        Long storeId,
        LocalDateTime deletedAt
) {}

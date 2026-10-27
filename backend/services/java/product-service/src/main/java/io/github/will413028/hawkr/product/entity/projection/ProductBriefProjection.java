package io.github.will413028.hawkr.product.entity.projection;

/**
 * 商品輕量 projection（40 號 §4.3b GetProductsBriefInternal）。
 *
 * <p>僅回傳 (id, name, status) 三欄，避免 partial-projection 反模式（docs/開發規範/Java 微服務開發規範.md §9.2）。
 *
 * @param id     product TSID
 * @param name   商品名稱
 * @param status {@link io.github.will413028.hawkr.common.utils.code.ProductStatus#value()}
 */
public record ProductBriefProjection(Long id, String name, String status) {}

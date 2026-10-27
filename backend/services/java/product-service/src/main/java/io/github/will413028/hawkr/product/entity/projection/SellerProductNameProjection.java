package io.github.will413028.hawkr.product.entity.projection;

/**
 * 賣家既有商品「同名偵測」projection（02 §9.5）。
 *
 * <p>用於 {@code BatchCheckSellerProductNames}：在不暴露 {@code ProductEntity} 整體欄位的前提下，
 * 僅回傳 (id, name, status) 三欄。避免 partial-projection 反模式（docs/開發規範/Java 微服務開發規範.md §9.2）。
 *
 * @param id     product TSID
 * @param name   商品名稱
 * @param status {@link io.github.will413028.hawkr.common.utils.code.ProductStatus#value()}
 */
public record SellerProductNameProjection(Long id, String name, String status) {}

package io.github.will413028.hawkr.productimport.entity.projection;

/**
 * Partial projection — asset 依 status 分組計數。
 */
public record AssetCountByStatusProjection(
        String status,
        Long cnt) {
}

package io.github.will413028.hawkr.productimport.entity.projection;

/**
 * Partial projection — ListImportItems 列表查詢專用（不含 description / hashtags / certifications 等大欄位）。
 *
 * <p>對齊 {@code ImportItemBrief} proto message；{@code price_min / price_max / first_image_url}
 * 在 service 層補（需 JOIN sku / asset，避免在此 projection 攜帶更多欄位）。
 *
 * <p>40 §3.5 / §4.2.6：{@code existingProductId / existingProductName / existingProductStatusLabel}
 * 由 product_import_item_error LEFT JOIN（WHERE source='L2_VALIDATOR' AND
 * error_type='DUPLICATE_NAME_VS_EXISTING'）取得，列表頁無 N+1。
 */
public record ImportItemBriefProjection(
        Long importItemId,
        String productKey,
        String status,
        String name,
        Long categoryId,
        String catL3Name,
        Integer skuCount,
        Integer blockingErrorCount,
        Integer warningCount,
        String commitFailedReason,
        Long formalProductId,
        Long duplicateExistingProductId,
        Integer commitRevision,
        Integer version,
        Long existingProductId,
        String existingProductName,
        String existingProductStatusLabel) {
}

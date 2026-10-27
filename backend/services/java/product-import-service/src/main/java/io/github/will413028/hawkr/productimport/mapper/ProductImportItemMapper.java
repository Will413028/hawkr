package io.github.will413028.hawkr.productimport.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.projection.ImportItemBriefProjection;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;
import java.util.Map;

/**
 * product_import_item mapper。
 *
 * <p>注意：partial projection（部分欄位查詢）必須回 record / DTO，禁止回 Entity（docs/開發規範/Java 微服務開發規範.md §9.2）。
 */
public interface ProductImportItemMapper extends BaseMapper<ProductImportItemEntity> {

    /** Counters 查詢 — 一次 SQL 回傳 status → count 的 map。 */
    @Select("""
            SELECT status, COUNT(*) AS cnt FROM product_import_item
            WHERE import_job_id = #{importJobId}
            GROUP BY status
            """)
    List<Map<String, Object>> countByStatusGroup(@Param("importJobId") Long importJobId);

    /**
     * ListImportItems brief — partial projection 走 ImportItemBriefProjection record。
     *
     * <p>40 §3.5：existing_product_* 來自 product_import_item_error LATERAL（每 item 至多 1 筆 DUPLICATE
     * row；用 LEFT JOIN + DISTINCT ON-like 取最新一筆，避免同 item 出現兩筆 dup 錯誤造成 row 翻倍）。
     * 採子查詢取 MAX(id) 對應的 row 而非 DISTINCT ON（MyBatis-PG 子查詢更穩定）。
     */
    @Select("""
            SELECT
              pii.id                              AS import_item_id,
              pii.product_key                     AS product_key,
              pii.status                          AS status,
              pii.name                            AS name,
              pii.category_id                     AS category_id,
              pii.cat_l3_name                     AS cat_l3_name,
              pii.sku_count                       AS sku_count,
              pii.blocking_error_count            AS blocking_error_count,
              pii.warning_count                   AS warning_count,
              pii.commit_failed_reason            AS commit_failed_reason,
              pii.formal_product_id               AS formal_product_id,
              pii.duplicate_existing_product_id   AS duplicate_existing_product_id,
              pii.commit_revision                 AS commit_revision,
              pii.version                         AS version,
              dup.existing_product_id             AS existing_product_id,
              dup.existing_product_name           AS existing_product_name,
              dup.existing_product_status_label   AS existing_product_status_label
            FROM product_import_item pii
            LEFT JOIN LATERAL (
                SELECT existing_product_id, existing_product_name, existing_product_status_label
                FROM product_import_item_error
                WHERE import_item_id = pii.id
                  AND error_type = 'DUPLICATE_NAME_VS_EXISTING'
                  AND existing_product_id IS NOT NULL
                ORDER BY id DESC
                LIMIT 1
            ) dup ON TRUE
            WHERE pii.import_job_id = #{importJobId}
              AND (#{includeRemoved} OR pii.status <> '9205')
            ORDER BY pii.first_csv_line ASC
            LIMIT #{limit} OFFSET #{offset}
            """)
    List<ImportItemBriefProjection> listBrief(@Param("importJobId") Long importJobId,
                                              @Param("includeRemoved") boolean includeRemoved,
                                              @Param("limit") int limit,
                                              @Param("offset") int offset);

    /**
     * 40 號設計書 §4.2.3 / §5.5d：「需修正」item 數 — 新資料應為 status ∈ {READY (9203),
     * COMMIT_FAILED (9208)} 且 blocking_error_count > 0。PARSED (9201) 僅為相容舊資料：舊版
     * asset settled 會把 blocking item 留在 PARSED，導致進度區漏算需修正。
     */
    @Select("""
            SELECT COUNT(*) FROM product_import_item
            WHERE import_job_id = #{importJobId}
              AND status IN ('9201','9203','9208')
              AND blocking_error_count > 0
            """)
    Long countNeedsFix(@Param("importJobId") Long importJobId);

    /**
     * 40 §3.5 / §4.3a / §5.5：對指定 itemIds 重算 blocking_error_count =
     * SELECT COUNT(*) FROM product_import_item_error WHERE severity='ERROR'。
     * commit final guard 失敗（VALIDATION_DRIFT）後同事務內呼叫，避免「parse / save 時通過但
     * commit 時 ValidateProductDraft 失敗」場景 blocking 沒被計入。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE product_import_item pii
            SET blocking_error_count = (
                    SELECT COUNT(*) FROM product_import_item_error piie
                    WHERE piie.import_item_id = pii.id
                      AND piie.severity = 'ERROR'
                      AND NOT (
                          piie.error_type = 'DUPLICATE_NAME_VS_EXISTING'
                          AND pii.allow_duplicate_name = TRUE
                      )
                ),
                updated_at = NOW()
            WHERE pii.id = #{itemId}
            """)
    int recalcBlockingErrorCount(@Param("itemId") Long itemId);

    /**
     * 對指定 item 同步重算 validator 尾端會影響 UI / commit eligibility 的兩個計數。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE product_import_item pii
            SET blocking_error_count = (
                    SELECT COUNT(*) FROM product_import_item_error piie
                    WHERE piie.import_item_id = pii.id
                      AND piie.severity = 'ERROR'
                      AND NOT (
                          piie.error_type = 'DUPLICATE_NAME_VS_EXISTING'
                          AND pii.allow_duplicate_name = TRUE
                      )
                ),
                warning_count = (
                    SELECT COUNT(*) FROM product_import_item_error piie
                    WHERE piie.import_item_id = pii.id
                      AND piie.severity = 'WARNING'
                ),
                updated_at = NOW()
            WHERE pii.id = #{itemId}
            """)
    int recalcValidationCounts(@Param("itemId") Long itemId);

    /** Item 的 blocking_error_count 同事務 +1 / -1。 */
    @Select("""
            UPDATE product_import_item
            SET blocking_error_count = blocking_error_count + #{delta},
                updated_at = NOW()
            WHERE id = #{itemId}
            RETURNING blocking_error_count
            """)
    Integer adjustBlockingErrorCount(@Param("itemId") Long itemId,
                                     @Param("delta") int delta);
}

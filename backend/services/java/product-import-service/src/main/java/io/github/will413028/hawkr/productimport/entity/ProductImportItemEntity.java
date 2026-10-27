package io.github.will413028.hawkr.productimport.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.productimport.entity.handler.CertificationListTypeHandler;
import io.github.will413028.hawkr.productimport.entity.handler.DescriptionJsonTypeHandler;
import io.github.will413028.hawkr.productimport.entity.handler.StringListTypeHandler;
import com.fasterxml.jackson.databind.JsonNode;
import lombok.Data;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

/**
 * 暫存商品主表（40 號設計書 §3.2）。一筆 = 一個 product_key 群組。
 */
@Data
@TableName(value = "product_import_item", autoResultMap = true)
public class ProductImportItemEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private Long importJobId;
    private Long storeId;
    private String productKey;

    /** {@link io.github.will413028.hawkr.common.utils.code.ImportItemStatus#value()} */
    private String status;

    /** 賣家每次有效編輯 +1；用於 idempotency_key。 */
    private Integer commitRevision;

    // ===== 商品本體（與 ProductService.CreateProductFromImportInternal 對齊） =====
    private String name;
    private Long categoryId;
    private String origin;
    /** 覆蓋 job 級 target_status；NULL = 繼承。 */
    private String targetStatus;

    /** ProductDescription.blocks 結構。 */
    @TableField(typeHandler = DescriptionJsonTypeHandler.class)
    private JsonNode descriptionJson;

    /** 上限 10。 */
    @TableField(typeHandler = StringListTypeHandler.class)
    private List<String> hashtags;

    /** {@code [{name: CertificationType value, certNo: string}]} */
    @TableField(typeHandler = CertificationListTypeHandler.class)
    private List<Map<String, Object>> certifications;

    // ===== 預覽用欄位（reflow） =====
    private Long catL1Id;
    private String catL1Name;
    private Long catL2Id;
    private String catL2Name;
    private String catL3Name;

    // ===== 解析來源追溯 =====
    private Integer firstCsvLine;
    private Integer skuCount;
    private Integer warningCount;
    private Integer blockingErrorCount;

    // ===== 重名偵測 =====
    private Boolean allowDuplicateName;
    private Long duplicateExistingProductId;

    // ===== 終態 =====
    private Long formalProductId;
    private String commitFailedReason;
    private LocalDateTime removedAt;
    private LocalDateTime committedAt;

    @Version
    private Integer version;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

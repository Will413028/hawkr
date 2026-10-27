package io.github.will413028.hawkr.productimport.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 暫存圖片資產表（40 號設計書 §3.4）。
 *
 * <p>用途：GALLERY（商品主圖 1~5）/ DESCRIPTION（description blocks 內嵌圖）。
 */
@Data
@TableName("product_import_item_asset")
public class ProductImportItemAssetEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private Long importItemId;
    private Long importJobId;

    /** {@link io.github.will413028.hawkr.common.utils.code.ImportAssetUsage#value()} */
    private String usage;

    private Integer sortOrder;
    /** DESCRIPTION 用，標記回填到第幾個 image block；GALLERY 時 NULL。 */
    private Integer blockIndex;

    /** URL（外部下載）/ STAGED（賣家本地上傳取得 staged_token）。 */
    private String sourceType;

    private String sourceUrl;
    private String sourceUrlHash;

    /** source_type=STAGED 時必填。 */
    private String stagedToken;

    /** {@link io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus#value()} */
    private String status;

    /** Media 端 import_asset.id（UUID）。 */
    private String mediaImportAssetId;
    private String objectKey;
    private String previewUrl;

    /** {@link io.github.will413028.hawkr.common.utils.code.ImportAssetFailureReason#value()} */
    private String failureReason;
    private String failureDetail;
    private Integer retryCount;
    private Long finalizedAssetId;

    @Version
    private Integer version;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

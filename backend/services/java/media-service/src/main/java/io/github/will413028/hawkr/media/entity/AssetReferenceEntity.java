package io.github.will413028.hawkr.media.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 引用關聯表 Entity
 * 對應設計書 2.3 asset_reference 表
 * <p>
 * 記錄誰在用這張圖
 */
@Data
@TableName(value = "asset_reference", autoResultMap = true)
public class AssetReferenceEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 主鍵 ID（自動遞增）
     */
    @TableId(type = IdType.AUTO)
    private Long id;

    /**
     * 資產 ID，外鍵關聯 media_asset
     */
    private Long assetId;

    /**
     * 業務類型，例如 PRODUCT、SELLER_LOGO
     * @see io.github.will413028.hawkr.common.utils.code.AssetReferenceType
     */
    private String refType;

    /**
     * 業務 ID，例如 product_id
     */
    private String refId;

    /**
     * 建立時間
     */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

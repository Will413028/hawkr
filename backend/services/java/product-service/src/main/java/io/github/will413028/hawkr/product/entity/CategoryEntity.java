package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;
import java.util.List;

/**
 * 商品分類 Entity
 * 對應 category 表（三層結構，透過 parent_id 自關聯）
 */
@Data
@TableName("category")
public class CategoryEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 分類 ID
     */
    @TableId(type = IdType.AUTO)
    private Long id;

    /**
     * 分類名稱
     */
    private String name;

    /**
     * 狀態（active/inactive）
     */
    private String status;

    /**
     * 建立者
     */
    private String createdBy;

    /**
     * 建立時間
     */
    @TableField(fill = FieldFill.INSERT, updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime createdAt;

    /**
     * 更新者
     */
    private String updatedBy;

    /**
     * 更新時間
     */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;

    /**
     * 父分類 ID（null 表示頂層分類）
     */
    private Long parentId;

    /**
     * 軟刪除時間（status=9003 時與 updated_at 同步寫入；還原時清空）
     */
    private LocalDateTime deletedAt;

    /**
     * 子分類列表（非資料庫欄位，用於樹狀結構組裝）
     */
    @TableField(exist = false)
    private List<CategoryEntity> children;

    /**
     * 父分類名稱（非資料庫欄位，用於回應組裝）
     */
    @TableField(exist = false)
    private String parentName;
}

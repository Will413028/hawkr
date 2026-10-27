package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 冪等性請求表 Entity
 * 對應設計書 2.3 idempotency_request 表
 */
@Data
@TableName(value = "idempotency_request", autoResultMap = true)
public class IdempotencyRequestEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 冪等鍵（客戶端生成的 UUID）
     * PK，由客戶端提供
     */
    @TableId(type = IdType.INPUT)
    private String key;

    /**
     * 賣家 store 代理鍵（對應 DB column {@code store_id}）
     */
    private Long storeId;

    /**
     * 操作端點，例如 "CreateProduct"
     */
    private String endpoint;

    /**
     * 請求狀態（使用 IdempotencyRequestStatus.value()）
     * 8001=IN_PROGRESS, 8002=SUCCEEDED, 8003=FAILED
     * @see io.github.will413028.hawkr.common.utils.code.IdempotencyRequestStatus
     */
    private String status;

    /**
     * 請求內容的 SHA-256 雜湊值
     * 用於驗證相同 key 的請求內容是否一致
     */
    private String requestHash;

    /**
     * 成功後的完整回傳 JSON，用於重播
     * 直接儲存 JSON 字串，不使用 TypeHandler
     */
    private String responseBody;

    /**
     * 建立時間（UPDATE 時不可修改）
     */
    @TableField(fill = FieldFill.INSERT, updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime createdAt;

    /**
     * 更新時間
     */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;

}

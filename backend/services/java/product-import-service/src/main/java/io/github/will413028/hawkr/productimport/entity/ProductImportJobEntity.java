package io.github.will413028.hawkr.productimport.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.productimport.entity.handler.ValidationErrorListTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

/**
 * 匯入任務主表（40 號設計書 §3.1）。
 *
 * <p>一筆 = 一次 CSV 上傳。狀態流轉見 {@link io.github.will413028.hawkr.common.utils.code.ImportJobStatus}。
 */
@Data
@TableName(value = "product_import_job", autoResultMap = true)
public class ProductImportJobEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private Long storeId;

    /** {@link io.github.will413028.hawkr.common.utils.code.ImportJobStatus#value()} */
    private String status;

    private String sourceFilename;
    private String sourceObjectKey;
    private Long sourceFileSize;
    private String sourceFileHash;

    private Integer totalRows;
    private Integer totalItems;
    private String parseError;

    /** ACTIVE | DRAFT | PENDING_REVIEW */
    private String targetStatus;

    private LocalDateTime expiresAt;

    /**
     * issue #2294 L9：GC tierA（過期任務）的<b>租約到期時間</b>（非「已處理旗標」）。
     * NULL 或 &lt;= NOW() 表示可搶佔；由 {@code claimExpiredJobForGc} 推進。
     */
    private LocalDateTime gcLeaseUntil;
    private LocalDateTime committedAt;
    private LocalDateTime cancelledAt;

    /** 一次性 flag —— MediaCallbackListener 在解析+下載完成時寫入 NOW()。 */
    private LocalDateTime parsedEventPublishedAt;

    private LocalDateTime commitFinishedEventPublishedAt;
    private String commitFinishedEventId;

    /** L2 失敗時的明細，結構：{@code [{csv_line, field, error_type, message, raw_value}]}。 */
    @TableField(typeHandler = ValidationErrorListTypeHandler.class)
    private List<Map<String, Object>> validationErrors;

    @Version
    private Integer version;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

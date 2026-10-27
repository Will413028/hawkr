package io.github.will413028.hawkr.productimport.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Commit 嘗試紀錄（40 號設計書 §3.6）。
 *
 * <p>{@code idempotency_key} UNIQUE，由 {@code "import:{item_id}:rev:{rev}"} 組成。
 */
@Data
@TableName("product_import_commit_attempt")
public class ProductImportCommitAttemptEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private Long importItemId;
    private Long importJobId;

    /** 對應 product_import_item.commit_revision。 */
    private Integer revision;
    private String idempotencyKey;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime startedAt;
    private LocalDateTime finishedAt;

    /** IN_PROGRESS | SUCCEEDED | FAILED */
    private String result;
    private Long formalProductId;
    private String failureReason;
    private Integer grpcCode;
}

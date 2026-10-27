package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.order.config.typehandler.JsonbTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 庫存交易協調表 Entity
 *
 * 用途：記錄 Deduct/Restore 操作快照與狀態，支援崩潰復原
 */
@Data
@TableName(value = "stock_tx_records", autoResultMap = true)
public class StockTxRecordEntity {

    @TableId(type = IdType.INPUT, value = "stock_tx_id")
    private String stockTxId;

    private String orderNo;

    private String phase;

    private String status;

    /**
     * items 快照（sku_id + qty），JSONB
     */
    @TableField(typeHandler = JsonbTypeHandler.class)
    private String payloadSnapshot;

    private String lastError;

    private Integer retryCount;

    private LocalDateTime nextRetryTime;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

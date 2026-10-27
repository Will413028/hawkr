package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 資金流水表 Entity
 *
 * 冪等唯一性：(type, external_txn_id) UNIQUE
 */
@Data
@TableName("financial_logs")
public class FinancialLogEntity {

    @TableId(type = IdType.INPUT, value = "log_id")
    private Long logId;

    private String sourceRefNo;

    private String type;

    private Long amount;

    private String externalTxnId;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

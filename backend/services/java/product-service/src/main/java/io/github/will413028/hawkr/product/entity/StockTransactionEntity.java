package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 庫存交易冪等表 Entity
 * 對應 stock_transaction 表
 */
@Data
@TableName("stock_transaction")
public class StockTransactionEntity {

    /**
     * 交易 ID（Order Service 生成，用於冪等）
     */
    @TableId(type = IdType.INPUT)
    private String stockTxId;

    /**
     * 操作類型：DEDUCT / RESTORE
     */
    private String operation;

    /**
     * 庫存項目快照 [{sku_id, quantity}]
     */
    private String payload;

    /**
     * 狀態：SUCCESS
     */
    private String status;

    /**
     * 建立時間
     */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

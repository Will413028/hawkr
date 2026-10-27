package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 支付事件冪等表 Entity
 *
 * 以 transaction_id（Payment Service 的 trade_no）作為冪等 PK
 */
@Data
@TableName("payment_event_dedup")
public class PaymentEventDedupEntity {

    @TableId(type = IdType.INPUT, value = "transaction_id")
    private String transactionId;

    private String orderNo;

    private String eventType;

    private String payloadHash;

    private LocalDateTime processedTime;
}

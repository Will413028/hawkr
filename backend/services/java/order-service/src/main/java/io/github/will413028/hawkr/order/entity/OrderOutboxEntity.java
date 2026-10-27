package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 訂單事件發件匣 Entity（order.paid 可靠投遞，03 §3.12 / §8.10）。
 *
 * <p>付款成功事務內寫入，由 OrderOutboxPublisherScheduler 輪詢派送至 MQ。
 * status 採 {@code OutboxEventStatus} value：8221=PENDING / 8222=SENT / 8223=FAILED / 8224=DEAD。
 */
@Data
@TableName("order_outbox")
public class OrderOutboxEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String aggregateId;

    private String eventType;

    private String exchange;

    private String routingKey;

    private String payload;

    private String status;

    private Integer retryCount;

    private LocalDateTime nextRetryAt;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    private LocalDateTime sentAt;
}

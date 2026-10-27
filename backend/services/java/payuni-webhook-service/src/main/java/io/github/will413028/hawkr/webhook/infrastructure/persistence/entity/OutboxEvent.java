package io.github.will413028.hawkr.webhook.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.typehandler.JsonbStringTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * Outbox 事件（可靠投遞）
 */
@Data
@TableName(value = "outbox_events", autoResultMap = true)
public class OutboxEvent {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String aggregateType;
    private String aggregateId;
    private String exchange;
    private String topic;
    private String messageKey;

    /** 事件 payload（JSONB） */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String payload;

    /** MQ headers（JSONB） */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String headers;

    /** OutboxEventStatus value（8221=PENDING / 8222=SENT / 8223=FAILED / 8224=DEAD） */
    private String status;
    private Integer retryCount;
    private LocalDateTime nextRetryAt;
    private LocalDateTime createdAt;
    private LocalDateTime sentAt;
    private String lastError;
}

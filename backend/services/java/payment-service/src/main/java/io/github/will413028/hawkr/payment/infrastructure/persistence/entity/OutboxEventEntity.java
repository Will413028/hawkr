package io.github.will413028.hawkr.payment.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * §2A.6 Outbox event entity（payment-service 自有 schema）。
 * <p>
 * 對應 V1 migration 的 outbox_events 表。由 DuplicatePaymentService 在
 * @Transactional 內 INSERT，由 OutboxEventPublisherScheduler 非同步派送到 MQ。
 * <p>
 * status 採用 OutboxEventStatus enum value：
 * 8221=PENDING / 8222=SENT / 8223=FAILED / 8224=DEAD。
 */
@Data
@TableName(value = "outbox_events", autoResultMap = true)
public class OutboxEventEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    /**
     * 本列要投遞到的 RabbitMQ exchange（explicit persisted exchange，36 §3.3.1）。
     * <p>
     * publisher 讀這一欄，<b>不</b>從 {@code routing_key} 推導；為 null／空白時 fail-closed
     * （不投遞、標 FAILED），不得 fallback 回 {@code payment.events}。
     * DB 側對應 {@code exchange VARCHAR(64) NOT NULL DEFAULT 'payment.events'}
     * ＋ {@code CHECK (exchange ~ '\S')}（V11）。
     */
    private String exchange;

    private String eventType;
    private String routingKey;

    /** 事件 payload（JSONB） */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String payload;

    private String status;
    private Integer retryCount;
    private LocalDateTime nextRetryAt;
    private LocalDateTime sentAt;
    private String lastError;
    private LocalDateTime createdAt;
}

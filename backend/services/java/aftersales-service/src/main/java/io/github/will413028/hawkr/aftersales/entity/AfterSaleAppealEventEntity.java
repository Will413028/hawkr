package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 申訴事件流水（§10.4.4）。BIGSERIAL 內部表，供 GetArbitration timeline 使用。
 */
@Data
@TableName("aftersale_appeal_events")
public class AfterSaleAppealEventEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String caseNo;

    /** AfterSaleAppealEventType value */
    private String eventType;

    /** BUYER / ADMIN / SYSTEM */
    private String actorType;

    private Long actorId;

    private String label;

    private String description;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

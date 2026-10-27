package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 平台裁決記錄（§10.4.3）。BIGSERIAL 內部審計表，不對外暴露 id。
 */
@Data
@TableName("aftersale_platform_decisions")
public class AfterSalePlatformDecisionEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String caseNo;

    /** AfterSalePlatformDecisionOutcome value */
    private String outcome;

    /** 僅 PARTIAL_REFUND 時填入 */
    private Long refundAmount;

    private String reason;

    private Long decidedByAdminId;

    /** 由 PlatformDecisionService 顯式填入（非 MetaObjectHandler 管轄的 createdAt 慣例欄位） */
    private LocalDateTime decidedAt;
}

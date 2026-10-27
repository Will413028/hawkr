package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.aftersales.config.typehandler.JsonbTypeHandler;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName(value = "aftersale_audit_logs", autoResultMap = true)
public class AfterSaleAuditLogEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String caseNo;

    private Long cancelId;

    /** 真實登入帳號 ID，僅 actorType=ADMIN(8404) 時填入。 */
    private Long actorAdminAccountId;

    /** 買家 ID，僅 actorType=BUYER(8401) 時填入。 */
    private Long actorBuyerId;

    /** 賣家商店 ID，僅 actorType=SELLER(8402) 時填入。 */
    private Long actorStoreId;

    private String actorType;

    private String action;

    @TableField(typeHandler = JsonbTypeHandler.class)
    private String detail;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

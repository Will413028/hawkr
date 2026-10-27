package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("seller_shipment_onboarding_logs")
public class OnboardingLogEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private Long settingId;
    private Long storeId;
    private String actionType;

    @TableField(typeHandler = io.github.will413028.hawkr.shipping.config.typehandler.JsonbStringTypeHandler.class)
    private String requestSnapshot;

    @TableField(typeHandler = io.github.will413028.hawkr.shipping.config.typehandler.JsonbStringTypeHandler.class)
    private String responseSnapshot;

    private String resultStatus;
    private String errorMessage;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

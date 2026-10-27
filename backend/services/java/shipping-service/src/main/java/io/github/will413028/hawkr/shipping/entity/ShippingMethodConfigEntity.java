package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.math.BigDecimal;
import java.time.LocalDateTime;

@Data
@TableName("shipping_method_configs")
public class ShippingMethodConfigEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private String shippingMethod;
    private String deliveryTemp;
    private Integer maxDimensionSumCm;
    private Integer maxSingleSideCm;
    private Long maxWeightGrams;
    /**
     * 體積上限（立方公分 L×W×H），0 表示不限（沿用同表既有慣例）。
     *
     * <p>⚠️ 與 {@link #maxDimensionSumCm} <b>量綱不同</b>：後者是 L+W+H 相加（cm），本欄是相乘（cm³）。
     * 兩者都存在不是重複——黑貓走材積和、Uber 走體積（66 §7.4 道 3：repo 現況完全沒有體積概念）。
     */
    private Long maxVolumeCm3;
    private Integer estimatedDays;
    private Integer settlementDays;
    private BigDecimal commissionRate;
    private Long commissionMin;
    private Boolean enabled;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

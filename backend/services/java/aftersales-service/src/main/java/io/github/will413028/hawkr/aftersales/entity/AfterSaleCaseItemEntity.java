package io.github.will413028.hawkr.aftersales.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("aftersale_case_items")
public class AfterSaleCaseItemEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String caseNo;

    private Long orderItemId;

    private Long skuId;

    private String skuName;

    private Integer quantity;

    private Long originalPrice;

    private Long actualPayAmount;

    private String imageObjectKey;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

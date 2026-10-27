package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("buyer_store_pickups")
public class BuyerStorePickupEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private Long buyerId;
    private String storeCode;
    private String storeName;
    private String storeAddress;
    private String receiverName;
    private String receiverPhone;
    private Boolean isDefault;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

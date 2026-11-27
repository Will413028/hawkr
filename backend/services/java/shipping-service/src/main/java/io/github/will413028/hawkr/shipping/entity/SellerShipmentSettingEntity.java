package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

@Data
@TableName("seller_shipment_settings")
public class SellerShipmentSettingEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private Long storeId;
    private String shippingType;
    private String goodsType;

    private String senderName;
    private String senderPhone;

    private String senderCity;
    private String senderDistrict;
    private String senderAddressDetail;
    private String senderPostalCode;

    private String returnName;
    private String returnPhone;
    private String returnCity;
    private String returnDistrict;
    private String returnAddressDetail;
    private String returnPostalCode;

    private String returnStoreCode;
    private String returnStoreName;
    private String returnStoreAddress;
    private String labelCompanyName;
    private String productCategoryCode;
    private String merId;
    private String partnerId;
    private String eshipId;
    private String providerStatus;
    private String providerMessage;
    private String onboardingStatus;
    private Boolean enabled;
    /** 建店送 PayUni 狀態機 value（PayUniSubmitStatus，§2.4.5.3）：4301 NONE / 4302 REQUESTED / 4303 SUCCEEDED / 4304 FAILED */
    private String payuniSubmitStatus;
    /** 最近一次 transition 到 REQUESTED 的時間，ops 告警判定 */
    private LocalDateTime payuniSubmitRequestedAt;
    /** 最近一次 transition 到 SUCCEEDED 的時間 */
    private LocalDateTime payuniSubmittedAt;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

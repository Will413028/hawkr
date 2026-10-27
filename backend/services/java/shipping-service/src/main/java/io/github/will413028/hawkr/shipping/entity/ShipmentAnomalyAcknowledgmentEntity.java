package io.github.will413028.hawkr.shipping.entity;

import com.baomidou.mybatisplus.annotation.FieldFill;
import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * §5.21 配送異常處置記錄（issue #1327 擴 actor 維度）。
 *
 * <p>UNIQUE(shipment_id, anomaly_type, actor_type)：同 (shipment, anomaly_type) 每個 actor_type
 * 各一筆（SELLER 與 ADMIN 互不覆蓋），後續 action 變更走 UPDATE，不新增 row。
 *
 * <p>本表只記使用者操作，不存 anomaly 狀態本身（異常存在性由 shipments 既有欄位推導）。
 *
 * 設計書：06 §5.21.4
 */
@Data
@TableName("shipment_anomaly_acknowledgments")
public class ShipmentAnomalyAcknowledgmentEntity {

    @TableId(type = IdType.AUTO)
    private Long id;

    private Long shipmentId;
    private String orderNo;
    /** ShipmentAnomalyType.value（寫入前必須 CodeMetaValidator.validate） */
    private String anomalyType;
    /** actor 類別：SELLER / ADMIN（CHECK 約束） */
    private String actorType;
    /** actor 識別：SELLER=賣家 store surrogate（原 seller_id 語意）；ADMIN=admin account_id */
    private Long actorId;
    /** ShipmentAnomalyAckAction.value：ACKNOWLEDGED / IGNORED */
    private String action;
    private LocalDateTime actedAt;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

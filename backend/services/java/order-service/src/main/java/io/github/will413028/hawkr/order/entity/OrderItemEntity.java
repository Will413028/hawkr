package io.github.will413028.hawkr.order.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 訂單明細表 Entity
 */
@Data
@TableName("order_items")
public class OrderItemEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private String orderNo;

    /** 商品 ID 快照（下單時從 Product Service 複製） */
    private Long productId;

    /** 商品名稱快照（下單時從 Product Service 複製） */
    private String productName;

    private Long skuId;

    private String skuName;

    private Integer quantity;

    /**
     * 原價快照（永遠 = 下單時 SKU.price，與是否參與特價無關）。
     * 給前端 strike-through 顯示用；判定品項參與特價 = unitPrice &lt; listPrice。
     */
    private Long listPrice;

    /**
     * 實際成交單價快照（無特價=listPrice；有特價=當時 sale_price）。
     * 原欄位 originalPrice 已重命名為本欄位（03 設計書 §3.2.1）。
     */
    private Long unitPrice;

    private Long shareDiscount;

    private Long actualPayAmount;

    private String refundStatus;

    /** 商品圖片 objectKey 快照（下單時從 Product Service 複製） */
    private String imageObjectKey;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;
}

package io.github.will413028.hawkr.cart.entity;

import com.baomidou.mybatisplus.annotation.*;
import lombok.Data;

import java.time.LocalDateTime;

/**
 * 購物車明細實體（對應 cart_items 表）
 *
 * 設計決策：不建立 carts 主表，直接以 buyer_id/guest_id 聚合
 * Owner 規則：buyer_id XOR guest_id（擇一必填）
 */
@Data
@TableName("cart_items")
public class CartItemEntity {

    /** TSID 主鍵 */
    @TableId(type = IdType.INPUT)
    private Long id;

    /** 買家身份（gateway 已解析，與 guest_id 擇一；純賣家帳號逛購物車時可能是 raw account_id，見遷移檔 column comment） */
    private Long buyerId;

    /** 訪客 UUID（前端生成，與 buyer_id 擇一） */
    private String guestId;

    /** 賣家分組 key（store 代理鍵，Product Service 取得，用於分組） */
    private Long storeId;

    /** 商品 SKU ID（= product_spec.id） */
    private Long skuId;

    /** 數量 */
    private Integer quantity;

    /** 加入時實際成交單價快照（新台幣元，無特價=list_price，有特價=當時 sale_price）。原欄位 addedPrice 已重命名為本欄位 */
    private Long addedUnitPrice;

    /** 加入時原價快照（新台幣元，永遠 = SKU.price，給前端 strike-through 顯示用） */
    private Long addedListPrice;

    /** 加入時限購上限快照（= product_spec.stock_qty） */
    private Integer maxLimitSnapshot;

    /** 加入時商品名稱快照（展示用） */
    private String skuNameSnapshot;

    /** 商品 ID（AddItem 時從 ProductSimpleInfo 取得） */
    private Long productId;

    /** SKU 規格名稱快照（product_sku.sku_name） */
    private String specNameSnapshot;

    /** 商品主圖 objectKey 快照（GetCart 時透過 MediaUrlProperties 組裝完整 URL） */
    private String imageObjectKey;

    /** 狀態：0=NORMAL, 1=LOCKED */
    private String status;

    /** 結帳批次 ID（鎖定時寫入） */
    private String checkoutId;

    /** 鎖定過期時間 */
    private LocalDateTime lockExpireTime;

    /** 建立時間 */
    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    /** 更新時間 */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

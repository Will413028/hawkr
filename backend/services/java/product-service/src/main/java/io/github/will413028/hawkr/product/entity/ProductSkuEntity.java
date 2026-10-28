package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.product.config.typehandler.RestrictedCategoryCodeListTypeHandler;
import io.github.will413028.hawkr.product.config.typehandler.ShippingMethodItemListTypeHandler;
import lombok.Data;

import java.io.Serializable;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;

/**
 * 商品 SKU 表 Entity
 * 對應設計書 2.2 product_sku 表
 */
@Data
@TableName(value = "product_sku", autoResultMap = true)

public class ProductSkuEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * SKU ID (自增)
     * 使用 IdType.AUTO 由資料庫自動生成
     */
    @TableId(type = IdType.AUTO)
    private Long id;

    /**
     * 所屬商品 ID
     */
    private Long productId;

    /**
     * SKU 名稱
     */
    private String skuName;

    /**
     * 重量（搭配 unit 使用）
     */
    private BigDecimal weight;

    /**
     * 單位（例如：公斤、顆）
     */
    private String unit;

    /**
     * 原價（新台幣元，list price）
     */
    private Long price;

    /**
     * 特價（新台幣元，NULL=無特價，非 NULL 必須 > 0 且 < price）。
     * 實際成交單價 = salePrice != null ? salePrice : price
     *
     * updateStrategy = IGNORED：明確支援「賣家送 salePrice=null 即清空特價」（02 設計書 §2.8.2），
     * 不能沿用 MyBatisPlus 預設的 NOT_NULL 策略（會把 null 視為「不更新」）。
     */
    @TableField(updateStrategy = FieldStrategy.IGNORED)
    private Long salePrice;

    /**
     * 庫存數量
     */
    private Integer stockQty;

    /**
     * 長（單位固定 cm）
     */
    private BigDecimal lengthCm;

    /**
     * 寬（單位固定 cm）
     */
    private BigDecimal widthCm;

    /**
     * 高（單位固定 cm）
     */
    private BigDecimal heightCm;

    /**
     * 運送方式 JSONB
     * 結構: [{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3102"}]
     * - type:          ShippingMethod value（3001=TAQBIN / 3002=SEVEN_ELEVEN_STORE_PICKUP / 3003=UBER_INSTANT）
     * - temperature:   ShippingTemperature value（3201=AMBIENT / 3202=CHILLED / 3203=FROZEN）
     * - freeThreshold: 免運門檻（新台幣元，0 表示無免運）
     * - feePayer:      ShippingFeePayer value（3101=BUYER / 3102=SELLER）
     */
    @TableField(typeHandler = ShippingMethodItemListTypeHandler.class)
    private List<ShippingMethodItem> shippingMethod;

    /**
     * 禁運品／品類標記 JSONB（②）。
     *
     * <p>結構：{@code ["<RestrictedGoodsCategory value>", ...]}；空陣列 = 未標記任何受限品類。
     *
     * <p>🔴 <b>本波零消費者是預期狀態</b>：寫入者是 ③ 的賣家上架 UI，
     * 讀取者是 §7.4 道 6 的結帳檢查，兩者都在後續波次。欄位先存在，是為了讓那兩件接上來時
     * 不是「檢查照跑但每筆都是空陣列 ⇒ 恆為通過」—— 逐字把那個組合稱為最糟的組合。
     *
     * <p>🔴 <b>值域待 UP-4</b>：CodeMeta 尚未建立 {@code RestrictedGoodsCategory}，
     * DB 的 {@code chk_product_sku_restricted_category_codes} 因此只允許空陣列
     * （V19 migration，fail-closed）。UP-4 定案時要同批做三件：建碼表、置換該 CHECK、
     * 寫入路徑走 {@code CodeMetaValidator}。
     *
     * <p>insert 時為 null ⇒ MyBatis-Plus 預設 insertStrategy=NOT_NULL 會把該欄從 INSERT
     * 欄位清單省略 ⇒ 由 DB {@code DEFAULT '[]'} 填入（與本類 {@code version} 同模式）；
     * update 時為 null 同理被略過，不會把既有標記洗成空。
     */
    @TableField(typeHandler = RestrictedCategoryCodeListTypeHandler.class)
    private List<String> restrictedCategoryCodes;

    /**
     * SKU 樂觀鎖版本（56 §5.4 / #2545）。
     *
     * <p>{@code updateById} 由 common-core 全域註冊的 {@code OptimisticLockerInnerInterceptor}
     * 自動 CAS + 1；{@code @Update} 自訂 SQL 不受攔截器保護，需逐句手寫 {@code version = version + 1}
     * （{@link io.github.will413028.hawkr.product.repository.ProductSkuRepository#deductStock}／
     * {@link io.github.will413028.hawkr.product.repository.ProductSkuRepository#restoreStock} 即為此，
     * 覆蓋由 {@code scripts/check-version-bump-coverage.py} 機械保證）。
     *
     * <p>insert 時為 null ⇒ MyBatis-Plus 預設 insertStrategy=NOT_NULL 會把該欄從 INSERT
     * 欄位清單省略 ⇒ 由 DB {@code DEFAULT 0} 填入（與 {@code ProductEntity.version} 同模式）。
     */
    @Version
    private Long version;

    /**
     * 建立時間（UPDATE 時不可修改）
     */
    @TableField(fill = FieldFill.INSERT, updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime createdAt;

    /**
     * 更新時間
     */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;

    /**
     * 運送方式項目結構（每個 method×temperature 組合一筆）
     */
    @Data
    public static class ShippingMethodItem implements Serializable {
        private static final long serialVersionUID = 1L;
        /** ShippingMethod Enum value（如 3001、3002） */
        private String type;
        /** ShippingTemperature Enum value（如 3201、3202、3203） */
        private String temperature;
        /** 免運門檻（新台幣元，數字字串，訂單金額 >= 此值則免運） */
        private String freeThreshold;
        /** ShippingFeePayer Enum value（如 3101、3102） */
        private String feePayer;
    }
}

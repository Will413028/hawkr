package io.github.will413028.hawkr.product.dto.request;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;
import java.util.List;

/**
 * 商品 SKU DTO
 * 對應 product_sku 表
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductSkuDTO {

    /**
     * SKU ID，更新時需要
     */
    private String id;

    /**
     * SKU 名稱
     */
    private String skuName;

    /**
     * 重量
     */
    private BigDecimal weight;

    /**
     * 單位（如：公斤、顆）
     */
    private String unit;

    /**
     * 原價（新台幣元，list price）
     */
    private Long price;

    /**
     * 特價（新台幣元，可為 null=無特價；非 null 必須 > 0 且 < price）
     */
    private Long salePrice;

    /**
     * 庫存數量
     */
    private Integer stockQty;

    /**
     * 長（cm）
     */
    private BigDecimal lengthCm;

    /**
     * 寬（cm）
     */
    private BigDecimal widthCm;

    /**
     * 高（cm）
     */
    private BigDecimal heightCm;

    /**
     * 運送方式列表
     */
    private List<ShippingMethodDTO> shippingMethods;

    /**
     * SKU 樂觀鎖版本（輸出方向，#2545）。讀取路徑填入，寫入路徑不看它。
     */
    private String version;

    /**
     * 期望版本（輸入方向，#2545）。null／空字串 = 不比對（舊 client 的既有行為）。
     *
     * <p>與 {@code UpdateProductDTO.expectedVersion}（product 層）是**兩個不同的鎖**：
     * product 層的反正規化計數欄刻意不 bump version（56 §5.4 F41 第 11 列），對扣庫存失明；
     * SKU 層這個才擋得住「表單開著期間賣掉的庫存被寫回復活」。
     */
    private String expectedVersion;
}

package io.github.will413028.hawkr.productimport.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.productimport.entity.handler.ShippingMethodListTypeHandler;
import lombok.Data;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

/**
 * 暫存 SKU 表（40 號設計書 §3.3）。每 CSV row 一筆。
 */
@Data
@TableName(value = "product_import_item_sku", autoResultMap = true)
public class ProductImportItemSkuEntity {

    @TableId(type = IdType.INPUT)
    private Long id;

    private Long importItemId;
    private Integer csvLine;

    private String skuName;
    /** 元（與 Product 對齊）。 */
    private Long price;
    private Long salePrice;
    private Integer stockQty;

    private BigDecimal weight;
    /** WeightUnit value（KG / CATTY / GRAM）。 */
    private String unit;

    private BigDecimal lengthCm;
    private BigDecimal widthCm;
    private BigDecimal heightCm;

    /** {@code [{method, temperature}]} */
    @TableField(typeHandler = ShippingMethodListTypeHandler.class)
    private List<Map<String, Object>> shippingMethods;

    private Integer sortOrder;

    @TableField(fill = FieldFill.INSERT)
    private LocalDateTime createdAt;

    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;
}

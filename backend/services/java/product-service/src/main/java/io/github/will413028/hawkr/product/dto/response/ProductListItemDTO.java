package io.github.will413028.hawkr.product.dto.response;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;

/**
 * 商品列表項目 DTO（簡化資料，不含 skus）
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductListItemDTO implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 商品 ID (UUID)
     */
    private String id;

    /**
     * 商品名稱
     */
    private String name;

    /**
     * 評分
     */
    private BigDecimal rating;

    /**
     * 狀態
     */
    private String status;

    /**
     * 分類 ID
     */
    private Long categoryId;

    /**
     * 分類名稱
     */
    private String categoryName;

    /**
     * 產地
     */
    private String origin;

    /**
     * 圖片集（用於顯示縮圖）
     */
    private List<GalleryItemDTO> gallery;

    /**
     * 最低實際售價（新台幣元，彙總自 SKU effective_price = salePrice ?? price）
     */
    private Long priceMin;

    /**
     * 最高實際售價（新台幣元）
     */
    private Long priceMax;

    /**
     * 最低原價（新台幣元，給前端 strike-through 顯示用）
     */
    private Long listPriceMin;

    /**
     * 最高原價（新台幣元）
     */
    private Long listPriceMax;

    /**
     * 最低特價（新台幣元，皆無特價時為 null）
     */
    private Long salePriceMin;

    /**
     * 最高特價（新台幣元）
     */
    private Long salePriceMax;

    /**
     * 建立時間
     */
    private LocalDateTime createdAt;

    /**
     * 評論數（保留欄位，日後透過其他 gRPC 取得）
     */
    private Integer reviewCount;

    /**
     * 商品擁有者 ID（= product.store_id，store 代理鍵）
     */
    private Long storeId;

    /**
     * 樂觀鎖版本（56 §6.6.3）。列表要能直接帶出 expectedVersion，代行批次下架逐項用。
     */
    private Long version;

    /**
     * 累計銷量（35 設計書）
     */
    private Long salesCount;

    /**
     * 商家資訊（公開 API 填充）
     */
    private SellerInfoDTO seller;

    /**
     * SKU 規格名稱列表（如「大顆裝 1kg」「小顆裝 500g」）
     */
    private List<String> skuNames;

    /**
     * Hashtag 列表（依 product_hashtag.position 升序，含讀取時計算的公開商品數）。
     */
    private List<HashtagItemDTO> hashtags;

    /**
     * 配送溫層聯集（02 §5.3.6）：所有 ACTIVE SKU 的 ShippingTemperature.value distinct 集合。
     */
    private List<String> deliveryTemp;

    /**
     * 商品認證列表（02 §5.3.4）：[{name, certNo}]，與 ProductDetail.certifications 對稱，供商品卡顯示 badge。
     */
    private List<io.github.will413028.hawkr.product.dto.request.CertificationDTO> certifications;

    /**
     * 即時庫存（總和，02 §3.2 / §5.13a）。
     * 對應 product.stock_qty 反正規化欄位。
     */
    private Long stockQty;

    /**
     * 收藏人次（02 §3.2 / §5.14）。
     * 對應 product.favorite_count 反正規化欄位。
     */
    private Long favoriteCount;

    /**
     * 運送方式聯集（02 §3.2 / §5.13a）。
     * 對應 product.shipping_methods 反正規化欄位。
     */
    private List<io.github.will413028.hawkr.product.entity.ProductSkuEntity.ShippingMethodItem> shippingMethods;
}

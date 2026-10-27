package io.github.will413028.hawkr.product.dto.response;

import io.github.will413028.hawkr.product.dto.request.CertificationDTO;
import io.github.will413028.hawkr.product.dto.request.ProductDescriptionDTO;
import io.github.will413028.hawkr.product.dto.request.ProductSkuDTO;
import io.github.will413028.hawkr.product.dto.request.ShippingMethodDTO;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.Serializable;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;

/**
 * 商品詳情 DTO（完整資料）
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductDetailDTO implements Serializable {
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
     * 累計銷量（35 設計書）
     */
    private Long salesCount;

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
     * 評論數（02 §3.2 / §5.14a，#1156）。
     * 對應 product.review_count 反正規化欄位。
     */
    private Long reviewCount;

    /**
     * 狀態
     */
    private String status;

    /** 公開生命週期（ProductPublicationState value）。 */
    private String publicationState;

    /** publicationState、status 與 stockQty 共同推導的可購買旗標。 */
    private Boolean purchasable;

    /**
     * 分類 ID
     */
    private Long categoryId;

    /**
     * 分類名稱（關聯類別的名稱，用於前端顯示）
     */
    private String categoryName;

    /**
     * 產地
     */
    private String origin;

    /**
     * 描述（圖文混排）
     */
    private ProductDescriptionDTO description;

    /**
     * 圖片集
     */
    private List<GalleryItemDTO> gallery;

    /**
     * 認證列表
     */
    private List<CertificationDTO> certifications;

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
     * 配送溫層 Enum value 陣列
     * @see io.github.will413028.hawkr.common.utils.code.ShippingTemperature
     */
    private List<String> deliveryTemp;

    /**
     * 運送方式
     */
    private List<ShippingMethodDTO> shippingMethods;

    /**
     * SKU 列表
     */
    private List<ProductSkuDTO> skus;

    /**
     * 建立時間
     */
    private LocalDateTime createdAt;

    /**
     * 更新時間
     */
    private LocalDateTime updatedAt;

    /**
     * 建立者 ID（audit only）
     */
    private Long createdBy;

    /**
     * 商品擁有權 key（= product.store_id，store 代理鍵；② re-key，D7 Stage 2 更名）。
     * createdBy 自此為純 audit。
     */
    private Long storeId;

    /**
     * 樂觀鎖版本（56 §6.6.3）。代行 PATCH 的 expectedVersion 來源；每次成功寫入 +1。
     */
    private Long version;

    // ==================== 三層分類 ====================

    /** 第三層分類 ID（product.category 指向的分類） */
    private Long categoryLv3Id;

    /** 第三層分類名稱 */
    private String categoryLv3Name;

    /** 第二層分類 ID */
    private Long categoryLv2Id;

    /** 第二層分類名稱 */
    private String categoryLv2Name;

    /** 第一層分類 ID */
    private Long categoryLv1Id;

    /** 第一層分類名稱 */
    private String categoryLv1Name;

    // ==================== 商家資訊（僅公開 API 填充） ====================

    /** 商家資訊 */
    private SellerInfoDTO seller;

    // ==================== 評論預覽（僅公開 API 填充） ====================

    /** 評論預覽（最多 2 筆） */
    private List<ReviewPreviewDTO> reviews;

    // ==================== Hashtag（02 設計書 §3.7-§3.9） ====================

    /** Hashtag 列表（依 product_hashtag.position 升序，含讀取時計算的公開商品數）。 */
    private List<HashtagItemDTO> hashtags;
}

package io.github.will413028.hawkr.product.dto.request;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

/**
 * 商品查詢 DTO
 * 對應 gRPC GetProductListRequest
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ProductQueryDTO {

    /**
     * 分類 ID 列表（多值聯集，02 §5.3.1）；null/空 表示不篩選；
     * 每個 id 由 service 層偵測層級後分組為 lv1/lv2/lv3 三組以 OR 串接
     */
    private java.util.List<Long> categoryIds;

    /**
     * 關鍵字搜尋
     */
    private String keyword;

    /**
     * 頁碼（從 1 開始）
     */
    private Integer page;

    /**
     * 每頁筆數
     */
    private Integer pageSize;

    /**
     * 排序方式（created_at, price_min, price_max）
     */
    private String sort;

    /**
     * 狀態篩選
     */
    private String status;

    /**
     * 操作者擁有權篩選（Gateway resolver 解析後的 store surrogate，用於賣家管理自己的商品）
     */
    private Long ownerStoreId;

    /**
     * 賣家 store ID（公開 API 篩選指定賣家的商品，如 /sellers/:id/products）
     */
    private Long storeId;

    /**
     * 是否為公開 API（true 時填充商家資訊）
     */
    private boolean publicApi;

    /**
     * Hashtag 精確篩選（任意大小寫，server normalize 後比對 canonical）
     */
    private java.util.List<String> hashtags;

    /**
     * Hashtag 多值匹配模式：false=ANY (default, OR)、true=ALL (AND)
     */
    private boolean hashtagMatchAll;

    /**
     * 配送溫層篩選（ShippingTemperature.value 列表，OR 命中）— 02 §5.3.4
     */
    private java.util.List<String> deliveryTempFilter;

    /**
     * 商品認證篩選（CertificationType value，如 3701/3702，OR 命中）— 02 §5.3.4
     * 比對 certifications[].name；name 視為不透明字串，不做 enum 轉換
     */
    private java.util.List<String> certificationTypes;

    /**
     * 價格下限（新台幣元，null=不篩選）
     */
    private Long priceMinFilter;

    /**
     * 價格上限（新台幣元，null=不篩選）
     */
    private Long priceMaxFilter;

    /**
     * 多賣家篩選（與 storeId 單一語意並存時取交集）— 02 §5.3.4
     */
    private java.util.List<Long> storeIds;

    /** Seller registration city ISO 3166-2:TW code; resolved via seller-service. */
    private String sellerCityCode;

    /**
     * 僅回登入者收藏的商品（true 時需先呼 buyer service ListFavoritedProductIds）— 02 §5.3.4
     */
    private boolean followedProductOnly;

    /**
     * 僅回登入者追蹤的賣家所有商品（true 時需先呼 buyer service ListSubscribedSellerIds）— 02 §5.3.4
     * service 層會將取得的 sellerIds 與既有 storeIds 取交集後傳入 SQL；不另開 SQL 路徑
     */
    private boolean followedSellerOnly;

    /**
     * 當前登入觀看者的 buyer ID，由 gateway 解析後傳入（與 ownerStoreId 語意分離，
     * 不影響 owner filter）。僅供 followed_product_only / followed_seller_only 等
     * viewer-aware 篩選使用。
     */
    private Long viewerBuyerId;

    /**
     * followedProductOnly=true 時，pipeline 先呼 buyer client 填入此欄位作為 IN filter
     * （上限 5000，超過取前 5000）；null = pipeline 尚未填入
     */
    private java.util.List<Long> followedProductIds;

    /**
     * 取得有效的頁碼（最小為 1）
     */
    public int getEffectivePage() {
        return page != null && page > 0 ? page : 1;
    }

    /**
     * 取得有效的每頁筆數（預設 20，最大 100）
     */
    public int getEffectivePageSize() {
        if (pageSize == null || pageSize <= 0) {
            return 20;
        }
        return Math.min(pageSize, 100);
    }

    /**
     * 計算 offset
     */
    public int getOffset() {
        return (getEffectivePage() - 1) * getEffectivePageSize();
    }
}

package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.*;
import io.github.will413028.hawkr.product.config.typehandler.CertificationListTypeHandler;
import io.github.will413028.hawkr.product.config.typehandler.GalleryListTypeHandler;
import io.github.will413028.hawkr.product.config.typehandler.ProductDescriptionTypeHandler;
import io.github.will413028.hawkr.product.config.typehandler.ShippingMethodItemListTypeHandler;
import lombok.Data;

import java.io.Serializable;
import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

/**
 * 商品主表 Entity
 * 對應設計書 2.1 product 表
 */
@Data
@TableName(value = "product", autoResultMap = true)
public class ProductEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /**
     * 商品 ID (TSID)
     * 使用 INPUT 類型，由程式碼手動生成 TSID
     */
    @TableId(type = IdType.INPUT)
    private Long id;

    /**
     * 商品名稱
     */
    private String name;

    /**
     * 商品評分
     */
    private BigDecimal rating;

    /**
     * 產品狀態（使用 ProductStatus.value()）
     * 4001=草稿, 4002=審核中, 4003=未上架, 4004=已上架
     * @see io.github.will413028.hawkr.common.utils.code.ProductStatus
     */
    private String status;

    /**
     * 首次公開時間；一旦寫入不可修改。
     * 公開生命週期由此不可變事實、status 與刪除欄位推導，不另存第二套 mutable state。
     */
    private LocalDateTime firstPublishedAt;

    /**
     * 商品類別 ID（第三層分類）
     */
    private Long category;

    /**
     * 第二層分類 ID（反正規化欄位，由後端自動從 category.parent_id 填入）
     */
    private Long categoryLv2;

    /**
     * 第一層分類 ID（反正規化欄位，由後端自動從 category_lv2.parent_id 填入）
     */
    private Long categoryLv1;

    /**
     * 第一層分類名稱快照（寫入時 lookup category 表；分類改名觸發 cascade reflow）
     */
    private String catL1Name;

    /**
     * 第二層分類名稱快照
     */
    private String catL2Name;

    /**
     * 第三層分類名稱快照（等同 product.category 對應的 category.name）
     */
    private String catL3Name;

    /**
     * 產地
     */
    private String origin;

    /**
     * 產品描述 JSONB
     * 結構: {"version":"1.0","blocks":[...]}
     * @see ProductDescription
     */
    @TableField(typeHandler = ProductDescriptionTypeHandler.class)
    private ProductDescription description;

    /**
     * 純文字描述（從 description.blocks[?type=text].content + image.alt 萃取串接）
     * 供 SEO/檢索/LLM 使用；應用層寫入時同事務維護（見 02 設計書 §3.6 萃取規則）
     */
    private String pureDescription;

    /**
     * 檢索全文（彙整 name + pure_description + cat_l1/l2/l3_name + sku_names + hashtag canonical names）
     * pg_trgm GIN 索引基底；上限 8 KB（見 02 設計書 §3.9）
     */
    private String searchableText;

    /**
     * 照片集 JSONB
     * 結構: [{"asset_id":"xxx","object_key":"/p/xxx/1.jpg"}]
     */
    @TableField(typeHandler = GalleryListTypeHandler.class)
    private List<GalleryItem> gallery;

    /**
     * 商品認證 JSONB
     * 結構: [{"name":"有機農產品標章","cert_no":"1-008-131005"}]
     */
    @TableField(typeHandler = CertificationListTypeHandler.class)
    private List<CertificationItem> certifications;

    /**
     * 最低實際售價（新台幣元，冗餘欄位，彙總自 SKU effective_price = salePrice ?? price）。
     *
     * <p>updateStrategy = IGNORED（#2546）：SKU 集合為空時 updatePriceRange 會 setPriceMin(null)，
     * 而 MyBatis-Plus 預設的 NOT_NULL 策略會把 null 欄位整個排除在 SET 之外 ⇒ 舊價格原封不動留在 DB，
     * 商品已經沒有任何 SKU（stock_qty 被 aggregateStock(∅) 寫成 0）卻仍對外宣稱舊價格。
     * 同一個「空集合」在 stock 上寫得進去（原生 long 0）、在 price 上寫不進去（包裝 Long null），
     * 這個不對稱本身就是缺陷。策略與下方 salePriceMin / salePriceMax 對齊。
     */
    @TableField(updateStrategy = FieldStrategy.IGNORED)
    private Long priceMin;

    /**
     * 最高實際售價（新台幣元，冗餘欄位）；策略同 priceMin。
     */
    @TableField(updateStrategy = FieldStrategy.IGNORED)
    private Long priceMax;

    /**
     * 最低原價（新台幣元，冗餘欄位，彙總自 SKU.price，給前端 strike-through 顯示）；策略同 priceMin。
     */
    @TableField(updateStrategy = FieldStrategy.IGNORED)
    private Long listPriceMin;

    /**
     * 最高原價（新台幣元）；策略同 priceMin。
     */
    @TableField(updateStrategy = FieldStrategy.IGNORED)
    private Long listPriceMax;

    /**
     * 最低特價（新台幣元，彙總自 SKU.salePrice；皆無特價時為 null）。
     * updateStrategy = IGNORED：賣家清空所有特價時 updatePriceRange 會 setSalePriceMin(null)，
     * 須讓 null 也寫入 DB（02 設計書 §2.8 / §3.2）。
     */
    @TableField(updateStrategy = FieldStrategy.IGNORED)
    private Long salePriceMin;

    /**
     * 最高特價（新台幣元）；策略同 salePriceMin。
     */
    @TableField(updateStrategy = FieldStrategy.IGNORED)
    private Long salePriceMax;

    /**
     * 累計銷量（付款成功時 +N，反正規化；詳見 35 設計書）。
     * 由 SalesAccumulatedConsumer 同事務 UPDATE 維護，業務寫入路徑禁止手動 set。
     */
    @TableField(updateStrategy = FieldStrategy.NEVER)
    private Long salesCount;

    /**
     * 即時庫存（總和，反正規化；02 §3.2 / §5.13a）。
     * SUM(product_sku.stock_qty)。
     * 由 CreateProduct / UpdateProduct / DeductStock / RestoreStock 同事務維護；
     * 業務寫入路徑禁止手動 set，請走 ProductAggregateRules.aggregateStock(skus)
     * 或 ProductRepository.reaggregateStockFromSku(productId)（全量重算，02 §5.8）。
     */
    private Long stockQty;

    /**
     * 收藏人次（反正規化；02 §3.2 / §5.14）。
     * 由 FavoriteCountConsumer 同事務 ±1 維護，業務寫入路徑禁止手動 set。
     */
    @TableField(updateStrategy = FieldStrategy.NEVER)
    private Long favoriteCount;

    /**
     * 評論數（反正規化；02 §3.2 / §5.14a，#1156）。
     * 由 ReviewStatsConsumer 以 review-service 權威重算值 full-recompute 覆寫，業務寫入路徑禁止手動 set。
     */
    @TableField(updateStrategy = FieldStrategy.NEVER)
    private Long reviewCount;

    /**
     * 運送方式聯集（反正規化；02 §3.2 / §5.13a）。
     * 所有 SKU shipping_method JSONB 按 (type, temperature) 去重；
     * 同組合 freeThreshold 取 MIN、feePayer 取 sku.id ASC 第一筆。
     * 由 CreateProduct / UpdateProduct 同事務維護，業務寫入路徑禁止手動 set。
     */
    @TableField(typeHandler = ShippingMethodItemListTypeHandler.class)
    private List<ProductSkuEntity.ShippingMethodItem> shippingMethods;

    /**
     * 樂觀鎖版本（56 §5.4）。updateById / update(entity, wrapper) 由 common-core 全域註冊的
     * OptimisticLockerInnerInterceptor 自動 CAS + 1；@Update 自訂 SQL 不受攔截器保護，
     * 需逐句手寫 version = version + 1（F41 清單）。
     */
    @Version
    private Long version;

    /**
     * 建立時間（UPDATE 時不可修改）
     */
    @TableField(fill = FieldFill.INSERT, updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime createdAt;

    /**
     * 建立者 ID（audit only — ② re-key 後擁有權語意一律用 {@link #storeId}）
     */
    private Long createdBy;

    /**
     * 更新時間
     */
    @TableField(fill = FieldFill.INSERT_UPDATE)
    private LocalDateTime updatedAt;

    /**
     * 更新者 ID
     */
    private Long updatedBy;

    /**
     * 刪除時間（軟刪除）
     */
    private LocalDateTime deletedAt;

    /**
     * 賣家帳號刪除時間戳（seller.deleted 消費後由 stampSellerDeletedByMerchant 蓋章；Part A / doc 44 §11）。
     * IS NOT NULL = 此商品所屬賣家帳號已刪除，賣家再取得身分後不顯示給賣家；不影響 public（靠 status）/ order。
     * <p>updateStrategy = NEVER：只由專屬 @Update（蓋章）維護，業務寫入路徑（updateById）禁止手動 set，
     * 對齊 salesCount / favoriteCount / reviewCount 慣例，防 updateById 誤 clobber 旗標。
     */
    @TableField(updateStrategy = FieldStrategy.NEVER)
    private LocalDateTime sellerDeletedAt;

    /**
     * 商品擁有權 key（② re-key / design 2026-07-11 §3.2；更名 store_id，D7 Stage 2 / design
     * 2026-07-12-store-id-rename-stage2-design.md §2 PR-B — 純 rename，非新語意）。
     * 建立當下由請求 storeId snapshot（gateway resolver 提供 current store surrogate）；
     * created_by / updated_by 降級為純 audit，不再承載擁有權語意。
     * <p>updateStrategy = NEVER：owner 永不隨業務 update 改寫（防 updateById clobber），
     * 對齊 salesCount / sellerDeletedAt 慣例；存量 backfill 由 V8 migration SQL-side 完成，
     * V9 migration 只改欄位名。
     */
    @TableField(value = "store_id", updateStrategy = FieldStrategy.NEVER)
    private Long storeId;

    /**
     * 匯入任務 ID（02 §9.4）
     * NULLABLE — 一般 CreateProduct 寫 NULL；經 Product Import Service 轉正才寫入，用於審計線索。
     * 對應 V4__add_product_import_origin_columns.sql。
     */
    @TableField(updateStrategy = FieldStrategy.NEVER)
    private Long importJobId;

    /**
     * 匯入暫存 item ID（02 §9.4）
     * NULLABLE — 同 importJobId，由 Import Service 在 commit 階段傳入，記錄「此正式商品從哪 CSV row 轉正而來」。
     */
    @TableField(updateStrategy = FieldStrategy.NEVER)
    private Long importItemId;

    /**
     * 圖片項目結構
     */
    @Data
    public static class GalleryItem implements Serializable {
        private static final long serialVersionUID = 1L;
        private String assetId;
        private String objectKey;
    }

    /**
     * 認證項目結構
     */
    @Data
    public static class CertificationItem implements Serializable {
        private static final long serialVersionUID = 1L;
        private String name;
        private String certNo;
    }

    /**
     * 商品描述結構（支援圖文混排）
     */
    @Data
    public static class ProductDescription implements Serializable {
        private static final long serialVersionUID = 1L;
        /**
         * 版本號
         */
        private String version;
        /**
         * 區塊列表
         */
        private List<DescriptionBlock> blocks;
    }

    /**
     * 描述區塊結構
     * type=text: 文字區塊，使用 content 和 style
     * type=image: 圖片區塊，使用 assetId, objectKey, width, height, alt
     */
    @Data
    public static class DescriptionBlock implements Serializable {
        private static final long serialVersionUID = 1L;
        /**
         * 區塊類型：text / image
         */
        private String type;
        /**
         * 區塊識別碼
         */
        private String id;

        // === text 類型欄位 ===
        /**
         * 文字內容（type=text 時使用）
         */
        private String content;
        /**
         * 樣式設定（type=text 時使用）
         * 結構: {"fontSize":18,"fontWeight":"bold","color":"#333","alignment":"center"}
         */
        private Map<String, Object> style;

        // === image 類型欄位 ===
        /**
         * 暫存 Token（新增圖片時傳入，Finalize 後替換為 assetId）
         */
        private String stagedToken;
        /**
         * 資產 ID（已 Finalize 的圖片）
         */
        private String assetId;
        /**
         * OSS 儲存路徑（已 Finalize 的圖片）
         */
        private String objectKey;
        /**
         * 圖片寬度
         */
        private Integer width;
        /**
         * 圖片高度
         */
        private Integer height;
        /**
         * 圖片替代文字
         */
        private String alt;

        // === inline-run 模型欄位（02 §3.6）===
        /**
         * 行內 runs（type=text；取代 legacy content）
         */
        private java.util.List<InlineRun> runs;
        /**
         * 表格資料 JSON 字串（type=table；cells 內含 runs，後端 opaque）
         */
        private String tableJson;
        /**
         * 清單資料 JSON 字串（type=list；items 內含 runs，後端 opaque）
         */
        private String listJson;
    }

    /**
     * 行內片段（inline run；02 §3.6 inline-run 模型）。
     * JSONB 僅序列化非 null 欄位（bold/italic/underline 僅 true 時存在，維持正規形）。
     */
    @Data
    @com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
    public static class InlineRun implements Serializable {
        private static final long serialVersionUID = 1L;
        private String text;
        private Boolean bold;
        private Boolean italic;
        private Boolean underline;
        private String color;
        private String href;
    }
}

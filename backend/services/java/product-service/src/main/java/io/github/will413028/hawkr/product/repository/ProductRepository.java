package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.product.entity.ProductEntity;
import io.github.will413028.hawkr.product.entity.projection.ProductBriefProjection;
import io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.entity.projection.RestockTransitionProjection;
import io.github.will413028.hawkr.product.entity.projection.SellerProductNameProjection;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.util.Collection;
import java.util.List;
import java.util.Optional;

/**
 * 商品 Repository
 * 繼承 MyBatis-Plus BaseMapper
 */
@Mapper
public interface ProductRepository extends BaseMapper<ProductEntity> {

    /**
     * 根據 ID 查詢商品（排除已軟刪除）
     */
    @Select("SELECT * FROM product WHERE id = #{id} AND deleted_at IS NULL")
    @ResultMap("mybatis-plus_ProductEntity")
    Optional<ProductEntity> findById(@Param("id") Long id);

    /** Public visibility lookup that deliberately includes soft-deleted rows. */
    @Select("SELECT * FROM product WHERE id = #{id}")
    @ResultMap("mybatis-plus_ProductEntity")
    Optional<ProductEntity> findByIdIncludingDeleted(@Param("id") Long id);

    /**
     * 根據 ID 和擁有者查詢商品（擁有權 = store_id，store 代理鍵）。
     */
    @Select("SELECT * FROM product WHERE id = #{id} AND store_id = #{storeId} AND deleted_at IS NULL AND seller_deleted_at IS NULL")
    @ResultMap("mybatis-plus_ProductEntity")
    Optional<ProductEntity> findByIdAndStoreId(@Param("id") Long id, @Param("storeId") Long storeId);

    /**
     * 分頁查詢商品（支援三層分類篩選 + 多 token OR keyword + hashtag 精確篩選）。
     *
     * <p>多 token 規則（02 設計書 §5.3.3）：
     * <ul>
     *   <li>tokens 已由 Service 層拆分、normalize、過濾過短 token、去重後傳入</li>
     *   <li>tokens 之間 OR；命中 token 數越多排序越前</li>
     *   <li>tokens 為空集合 = 不執行 keyword WHERE</li>
     * </ul>
     *
     * <p>Hashtag 篩選（§3.9）：canonicalHashtags 為已 normalize 之 canonical name 集合；
     * matchAll=false 時 ANY（OR），true 時 ALL（AND）。
     *
     * @param requireExposableStore 公開（買家）視角：只列店家可曝光的商品（{@code store_exposure}，沒有列 = 不可曝光）；
     *                              條件在 SQL 內，分頁與總數才準。賣家看自己商品與 admin 視角傳 false
     * @param categoryLv1Ids lv1 分類 ID 列表（null/空 = 不篩 lv1）
     * @param categoryLv2Ids lv2 分類 ID 列表（null/空 = 不篩 lv2）
     * @param categoryLv3Ids lv3 分類 ID 列表（null/空 = 不篩 lv3）
     *                       三組以 OR 串接（02 §5.3.1）；命中任一即可；天然支援「遞迴含子分類」
     */
    @Select("""
        <script>
        SELECT p.*,
            (
                <choose>
                    <when test="tokens != null and !tokens.isEmpty()">
                        <foreach collection="tokens" item="t" separator="+">
                            (CASE WHEN p.searchable_text ILIKE CONCAT('%', #{t}, '%') THEN 1 ELSE 0 END)
                        </foreach>
                    </when>
                    <otherwise>0</otherwise>
                </choose>
            ) AS hit_count
        FROM product p
        WHERE p.deleted_at IS NULL
        <if test="storeId != null">
            AND p.store_id = #{storeId}
        </if>
        <if test="excludeSellerDeleted">
            AND p.seller_deleted_at IS NULL
        </if>
        <if test="requireExposableStore">
            AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = p.store_id AND se.exposable)
        </if>
        <if test="(categoryLv1Ids != null and !categoryLv1Ids.isEmpty()) or (categoryLv2Ids != null and !categoryLv2Ids.isEmpty()) or (categoryLv3Ids != null and !categoryLv3Ids.isEmpty())">
            AND (
                <trim prefixOverrides="OR ">
                <if test="categoryLv1Ids != null and !categoryLv1Ids.isEmpty()">
                    p.category_lv1 IN
                    <foreach collection="categoryLv1Ids" item="cid" open="(" separator="," close=")">#{cid}</foreach>
                </if>
                <if test="categoryLv2Ids != null and !categoryLv2Ids.isEmpty()">
                    OR p.category_lv2 IN
                    <foreach collection="categoryLv2Ids" item="cid" open="(" separator="," close=")">#{cid}</foreach>
                </if>
                <if test="categoryLv3Ids != null and !categoryLv3Ids.isEmpty()">
                    OR p.category IN
                    <foreach collection="categoryLv3Ids" item="cid" open="(" separator="," close=")">#{cid}</foreach>
                </if>
                </trim>
            )
        </if>
        <if test="status != null and status != ''">
            AND p.status = #{status}
        </if>
        <if test="tokens != null and !tokens.isEmpty()">
            AND (
            <foreach collection="tokens" item="t" separator=" OR ">
                p.searchable_text ILIKE CONCAT('%', #{t}, '%')
            </foreach>
            )
        </if>
        <if test="canonicalHashtags != null and !canonicalHashtags.isEmpty()">
            <choose>
                <when test="matchAll">
                    AND (
                        SELECT COUNT(DISTINCT h.id)
                        FROM product_hashtag ph
                        JOIN hashtag h ON h.id = ph.hashtag_id
                        WHERE ph.product_id = p.id
                          AND h.name IN
                          <foreach collection="canonicalHashtags" item="hn" open="(" separator="," close=")">#{hn}</foreach>
                    ) = #{hashtagCount}
                </when>
                <otherwise>
                    AND EXISTS (
                        SELECT 1 FROM product_hashtag ph
                        JOIN hashtag h ON h.id = ph.hashtag_id
                        WHERE ph.product_id = p.id
                          AND h.name IN
                          <foreach collection="canonicalHashtags" item="hn" open="(" separator="," close=")">#{hn}</foreach>
                    )
                </otherwise>
            </choose>
        </if>
        <!-- 02 §5.3.4 server-side filter -->
        <if test="deliveryTempFilter != null and !deliveryTempFilter.isEmpty()">
            AND EXISTS (
                SELECT 1 FROM product_sku ps,
                    jsonb_array_elements(ps.shipping_method) AS m
                WHERE ps.product_id = p.id
                  AND m->>'temperature' IN
                  <foreach collection="deliveryTempFilter" item="t" open="(" separator="," close=")">#{t}</foreach>
            )
        </if>
        <if test="certificationTypes != null and !certificationTypes.isEmpty()">
            AND (
            <foreach collection="certificationTypes" item="ct" separator=" OR ">
                p.certifications @> jsonb_build_array(jsonb_build_object('name', #{ct}))
            </foreach>
            )
        </if>
        <if test="priceMinFilter != null">
            AND p.price_max >= #{priceMinFilter}
        </if>
        <if test="priceMaxFilter != null">
            AND p.price_min &lt;= #{priceMaxFilter}
        </if>
        <if test="storeIds != null and !storeIds.isEmpty()">
            AND p.store_id IN
            <foreach collection="storeIds" item="sid" open="(" separator="," close=")">#{sid}</foreach>
        </if>
        <if test="followedProductIds != null and !followedProductIds.isEmpty()">
            AND p.id IN
            <foreach collection="followedProductIds" item="pid" open="(" separator="," close=")">#{pid}</foreach>
        </if>
        ORDER BY
        <if test="tokens != null and !tokens.isEmpty()">hit_count DESC,</if>
        <choose>
            <when test="sort == 'PRICE_ASC'">p.price_min ASC NULLS LAST, p.id DESC</when>
            <when test="sort == 'PRICE_DESC'">p.price_max DESC NULLS LAST, p.id DESC</when>
            <when test="sort == 'RATING'">p.rating DESC NULLS LAST, p.created_at DESC, p.id DESC</when>
            <when test="sort == 'SALES'">p.sales_count DESC, p.created_at DESC, p.id DESC</when>
            <otherwise>p.created_at DESC, p.id DESC</otherwise>
        </choose>
        </script>
        """)
    @ResultMap("mybatis-plus_ProductEntity")
    IPage<ProductEntity> findPageByCondition(
            Page<ProductEntity> page,
            @Param("storeId") Long storeId,
            @Param("categoryLv1Ids") List<Long> categoryLv1Ids,
            @Param("categoryLv2Ids") List<Long> categoryLv2Ids,
            @Param("categoryLv3Ids") List<Long> categoryLv3Ids,
            @Param("status") String status,
            @Param("tokens") List<String> tokens,
            @Param("canonicalHashtags") List<String> canonicalHashtags,
            @Param("matchAll") boolean matchAll,
            @Param("hashtagCount") int hashtagCount,
            @Param("sort") String sort,
            @Param("deliveryTempFilter") List<String> deliveryTempFilter,
            @Param("certificationTypes") List<String> certificationTypes,
            @Param("priceMinFilter") Long priceMinFilter,
            @Param("priceMaxFilter") Long priceMaxFilter,
            @Param("storeIds") List<Long> storeIds,
            @Param("followedProductIds") List<Long> followedProductIds,
            @Param("excludeSellerDeleted") boolean excludeSellerDeleted,
            @Param("requireExposableStore") boolean requireExposableStore
    );

    /**
     * 公開商品產地聚合（02 §5.13）。
     * 只計入 status=4004、店家可曝光（{@code store_exposure}）且 origin 非空的商品，依商品數降冪 + 產地升冪排序。
     * @param categoryId 分類 ID（可空，null 表示不篩選）
     * @param categoryLevel 1/2/3 對應 category_lv1/lv2/category；0=不篩選
     * @param limit 上限（service 層 clamp 至 1~100）
     */
    @Select("""
        <script>
        SELECT origin AS origin, COUNT(*)::text AS productCount
        FROM product
        WHERE deleted_at IS NULL
          AND status = '4004'
          AND origin IS NOT NULL
          AND origin &lt;&gt; ''
          AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = product.store_id AND se.exposable)
        <if test="categoryId != null">
            <choose>
                <when test="categoryLevel == 1">AND category_lv1 = #{categoryId}</when>
                <when test="categoryLevel == 3">AND category = #{categoryId}</when>
                <otherwise>AND category_lv2 = #{categoryId}</otherwise>
            </choose>
        </if>
        GROUP BY origin
        ORDER BY COUNT(*) DESC, origin ASC
        LIMIT #{limit}
        </script>
        """)
    List<OriginSummaryRow> findPublicOriginsSummary(
            @Param("categoryId") Long categoryId,
            @Param("categoryLevel") int categoryLevel,
            @Param("limit") int limit
    );

    @Select("""
        SELECT store_id AS storeId, COUNT(*) AS productCount
        FROM product
        WHERE deleted_at IS NULL
          AND seller_deleted_at IS NULL
          AND status = '4004'
          AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = product.store_id AND se.exposable)
        GROUP BY store_id
        """)
    List<StoreProductCountRow> findPublicProductCountsByStore();

    class StoreProductCountRow {
        private Long storeId;
        private Long productCount;

        public Long getStoreId() { return storeId; }
        public void setStoreId(Long storeId) { this.storeId = storeId; }
        public Long getProductCount() { return productCount; }
        public void setProductCount(Long productCount) { this.productCount = productCount; }
    }

    /**
     * 產地聚合結果列。
     */
    class OriginSummaryRow {
        private String origin;
        private String productCount;

        public OriginSummaryRow() {}

        public OriginSummaryRow(String origin, String productCount) {
            this.origin = origin;
            this.productCount = productCount;
        }

        public String getOrigin() { return origin; }
        public void setOrigin(String origin) { this.origin = origin; }
        public String getProductCount() { return productCount; }
        public void setProductCount(String productCount) { this.productCount = productCount; }
    }

    /**
     * 是否有商品掛在指定分類下（依層級用對應欄位）
     * level: 1=category_lv1, 2=category_lv2, 3=category
     */
    @Select("""
        <script>
        SELECT COUNT(*) > 0 FROM product WHERE deleted_at IS NULL
        <choose>
            <when test="level == 1">AND category_lv1 = #{categoryId}</when>
            <when test="level == 2">AND category_lv2 = #{categoryId}</when>
            <when test="level == 3">AND category = #{categoryId}</when>
            <otherwise>AND 1 = 0</otherwise>
        </choose>
        </script>
        """)
    boolean existsInCategoryLevel(
            @Param("categoryId") Long categoryId,
            @Param("level") int level);

    /**
     * 取分類底下的商品 ID 列表（給批次更換用：依層級查；不限狀態，含軟刪除以外的商品）
     */
    @Select("""
        <script>
        SELECT id FROM product WHERE deleted_at IS NULL
        <choose>
            <when test="level == 1">AND category_lv1 = #{categoryId}</when>
            <when test="level == 2">AND category_lv2 = #{categoryId}</when>
            <when test="level == 3">AND category = #{categoryId}</when>
            <otherwise>AND 1 = 0</otherwise>
        </choose>
        </script>
        """)
    List<Long> findIdsByCategoryLevel(
            @Param("categoryId") Long categoryId,
            @Param("level") int level);

    /**
     * 批次更換指定 product_ids 的分類三層欄位（lv3 / lv2 / lv1 一次同步寫入）。
     * 注意：MetaObjectHandler 對 INSERT_UPDATE 的 update 動作會自動填 updated_at；此 SQL 顯式帶 updated_at 確保。
     */
    @Update("""
        <script>
        UPDATE product
        SET category = #{lv3Id},
            category_lv2 = #{lv2Id},
            category_lv1 = #{lv1Id},
            updated_by = #{updatedBy},
            updated_at = NOW(),
            version = version + 1
        WHERE deleted_at IS NULL AND id IN
        <foreach collection="ids" item="id" open="(" separator="," close=")">
            #{id}
        </foreach>
        </script>
        """)
    int batchUpdateCategoryHierarchy(
            @Param("ids") List<Long> ids,
            @Param("lv3Id") Long lv3Id,
            @Param("lv2Id") Long lv2Id,
            @Param("lv1Id") Long lv1Id,
            @Param("updatedBy") Long updatedBy);

    /**
     * 查詢某賣家（store_id，store 代理鍵）目前指定狀態、未軟刪除的商品 ID 清單。
     * 用於賣家刪除時批次下架 ACTIVE 商品（先取 ID 以便逐筆做 hashtag 計數扣減 + cache 失效）。
     */
    @Select("""
        SELECT id FROM product
        WHERE store_id = #{storeId}
          AND deleted_at IS NULL
          AND status = #{status}
        """)
    List<Long> findProductIdsBySellerAndStatus(
            @Param("storeId") Long storeId,
            @Param("status") String status);

    /**
     * 查詢某店家（{@code store_id}）所有未軟刪除的商品 ID（不限狀態）。
     * 店家可曝光狀態改變時，用來清這些商品的詳情快取。
     */
    @Select("""
        SELECT id FROM product
        WHERE store_id = #{storeId}
          AND deleted_at IS NULL
        """)
    List<Long> findProductIdsByStoreId(@Param("storeId") Long storeId);

    /**
     * 批次更新指定 product_ids 的狀態（排除已軟刪除）。
     * 呼叫端須先以 {@link #findProductIdsBySellerAndStatus} 確認來源狀態，以維持冪等與 hashtag 計數正確。
     */
    @Update("""
        <script>
        UPDATE product
        SET status = #{status},
            updated_by = #{updatedBy},
            updated_at = NOW(),
            version = version + 1
        WHERE deleted_at IS NULL AND id IN
        <foreach collection="ids" item="id" open="(" separator="," close=")">
            #{id}
        </foreach>
        </script>
        """)
    int batchUpdateStatusByIds(
            @Param("ids") List<Long> ids,
            @Param("status") String status,
            @Param("updatedBy") Long updatedBy);

    /**
     * 賣家帳號刪除時，蓋章其名下「全部狀態」未軟刪商品的 seller_deleted_at（Part A / doc 44 §11）。
     *
     * <p>自訂 @Update（非 updateById）：MyBatis-Plus 全域 logic-delete 會對 BaseMapper SQL 自動加
     * deleted_at IS NULL 條件，此處顯式帶條件、且蓋「全狀態」（ACTIVE/INACTIVE/DRAFT/PENDING_REVIEW），
     * 非只 off-shelf 的 ACTIVE。seller_deleted_at IS NULL guard → 冪等（重投蓋 0 列）。
     *
     * @param storeId 被刪除賣家的 store 代理鍵（= product.store_id，② re-key，D7 Stage 2 更名）
     * @return 實際蓋章筆數
     */
    @Update("""
        UPDATE product
           SET seller_deleted_at = NOW(),
               updated_at = NOW()
         WHERE store_id = #{storeId}
           AND deleted_at IS NULL
           AND seller_deleted_at IS NULL
        """)
    int stampSellerDeletedByMerchant(@Param("storeId") Long storeId);

    /**
     * 累計銷量（35 設計書 §5.3）。
     * DB-side atomic increment（並發下的原子累加，無需 SELECT FOR UPDATE）。
     * 已軟刪除商品仍保留累計（不過濾 deleted_at），避免同一事件內部分商品被靜默漏計；查詢端透過 partial index 自動排除。
     * <p>注意：本方法只保證單次 UPDATE 的原子性，<b>不</b>提供 MQ redelivery 的冪等——
     * 重投去重由 {@code SalesAccumulator.applyAll} 同事務的 order_no dedup claim 保證。
     */
    @Update("UPDATE product SET sales_count = sales_count + #{qty} WHERE id = #{id}")
    int incrementSalesCount(@Param("id") Long id, @Param("qty") int qty);

    /**
     * stock_qty 反正規化欄位全量重算 cascade（02 §3.2 / §5.8）。
     * <p>
     * product.stock_qty = SUM(product_sku.stock_qty)，以 SKU 真值重算而非增量 ±delta：
     * <ul>
     *   <li>結構上不可能為負（SUM of 非負 SKU ≥ 0），永不違反 chk_product_stock_qty_nonneg</li>
     *   <li>自癒：每次庫存操作把反正規化值重新對齊 SKU，歷史 drift 下次操作即收斂</li>
     *   <li>與 CreateProduct / UpdateProduct（ProductAggregateRules.aggregateStock）統一模型</li>
     * </ul>
     * 不過濾 deleted_at（product_sku 無軟刪欄位；避免 cascade 失敗影響交易完整性）。
     */
    @Update("""
        UPDATE product SET stock_qty =
            (SELECT COALESCE(SUM(stock_qty), 0) FROM product_sku WHERE product_id = #{id})
        WHERE id = #{id}
        """)
    int reaggregateStockFromSku(@Param("id") Long id);

    /**
     * 讀回 product.stock_qty 當前值（供補貨 transition 0→>0 偵測，02 §5.13b）。
     */
    @Select("SELECT stock_qty FROM product WHERE id = #{id}")
    Long findStockQtyById(@Param("id") Long id);

    /**
     * favorite_count 反正規化欄位 cascade（02 §3.2 / §5.14）。
     * delta = +1 或 -1。
     * WHERE 含 favorite_count > 0 防止 underflow（與 CHECK 約束雙重保險）。
     * rowsAffected = 0 時，consumer 端記 WARN（重複事件或商品不存在 / 已軟刪 / 計數已為 0）。
     */
    @Update("""
        UPDATE product
           SET favorite_count = favorite_count + #{delta}
         WHERE id = #{id}
           AND deleted_at IS NULL
           AND (#{delta} > 0 OR favorite_count > 0)
        """)
    int incrementFavoriteCount(@Param("id") Long id, @Param("delta") long delta);

    /**
     * 評論聚合反正規化欄位 full-recompute 覆寫（02 §3.2 / §5.14a，#1156）。
     * rating / review_count 由 review-service 權威重算後透過 review.created / review.hidden 帶入，
     * consumer 直接覆寫（天然冪等，無需 dedup）。rowsAffected = 0 時 consumer 記 WARN（商品不存在 / 已軟刪）。
     */
    @Update("""
        UPDATE product
           SET rating = #{rating},
               review_count = #{reviewCount}
         WHERE id = #{id}
           AND deleted_at IS NULL
        """)
    int updateReviewStats(@Param("id") Long id,
                          @Param("rating") java.math.BigDecimal rating,
                          @Param("reviewCount") long reviewCount);

    /**
     * 覆寫 {@code product.certifications} 的 approved-only 公開投影（19 號設計書 §7.2）。
     *
     * <p>唯一寫入 {@code certifications} 的路徑 —— 收斂後任何業務路徑都<b>不得</b>再
     * {@code setCertifications} 直寫投影（§6.1），投影只由
     * {@code ProductCertificationApplicationService.rebuildProjection} 由當前所有
     * {@code APPROVED} 申請重算後帶進來。
     *
     * <p>🔴 手寫 {@code version = version + 1}：自訂 {@code @Update} 不受
     * {@code OptimisticLockerInnerInterceptor} 攔截（漏了會讓代行的樂觀鎖對「認證核准」這條
     * 寫入路徑完全失明，且 {@code scripts/check-version-bump-coverage.py} 會紅）。
     * {@code updated_at} 同理——MetaObjectHandler 只在 BaseMapper 路徑生效，這裡手寫。
     *
     * <p>已軟刪商品不再有公開面，但投影仍應與申請表一致（ADM 可能對已下架商品撤銷標章），
     * 故<b>不加</b> {@code deleted_at IS NULL} 述詞。
     *
     * @return 1 = 已覆寫；0 = 商品不存在
     */
    @Update("""
        UPDATE product
           SET certifications = #{certifications,
                   typeHandler=io.github.will413028.hawkr.product.config.typehandler.CertificationListTypeHandler},
               version = version + 1,
               updated_at = NOW()
         WHERE id = #{id}
        """)
    int updateCertificationProjection(
            @Param("id") Long id,
            @Param("certifications") List<ProductEntity.CertificationItem> certifications);

    /**
     * 補貨 transition projection 批量查詢（02 §5.13b）。
     * <p>
     * 用於 StockService.cascadeProductStock reflow 前一次批量取得偵測 0→&gt;0 transition
     * 所需的 product 層級欄位（stock_qty / status / name / 主圖 objectKey / store_id /
     * deleted_at），避免回傳完整 ProductEntity 造成 docs/開發規範/Java 微服務開發規範.md §9.2 partial-projection 反模式。
     * <p>
     * primaryObjectKey 從 gallery JSONB 第一個 element 抽出（{@code gallery->0->>'objectKey'}），
     * 由 caller 原樣填入 MQ payload 的 imageKey 欄位（49 §6.3：MQ 只帶 objectKey，顯示 URL 由消費端組）。
     * <p>
     * 不過濾 deleted_at（cascade 路徑允許訪問軟刪商品；deleted_at 由 caller 判斷以決定是否觸發 event）。
     */
    @Select("""
        <script>
        SELECT id              AS productId,
               stock_qty       AS stockQty,
               status          AS status,
               name            AS name,
               gallery->0->>'objectKey' AS primaryObjectKey,
               store_id        AS storeId,
               deleted_at      AS deletedAt
        FROM product
        WHERE id IN
        <foreach collection="ids" item="id" open="(" separator="," close=")">#{id}</foreach>
        </script>
        """)
    List<RestockTransitionProjection> findRestockTransitionFields(@Param("ids") List<Long> ids);

    /**
     * 商品 → 所屬賣家（store_id）批次反查（後端服務責任邊界規範 §9.3）。
     *
     * <p>用於 frontend cache revalidate：批次 mutation（改分類 / 分類 cascade / 庫存）影響多筆
     * 商品、可能橫跨多賣家時，反查各筆 sellerId 以組出 {@code seller:<sellerId>} bust 清單。
     * <p>不過濾 deleted_at（軟刪商品仍需 bust 其快取）。
     */
    @Select("""
        <script>
        SELECT id AS productId, store_id AS sellerId
        FROM product
        WHERE id IN
        <foreach collection="ids" item="id" open="(" separator="," close=")">#{id}</foreach>
        </script>
        """)
    List<ProductSellerRef> findProductSellerRefs(@Param("ids") List<Long> ids);

    /**
     * 批次查詢「該賣家底下名稱命中的商品」projection（02 §9.5）。
     *
     * <p>用於 {@code BatchCheckSellerProductNames}：對 csv.parse / UpdateImportItem 階段提供
     * 「賣家既有同名商品偵測」。回傳僅 (id, name, status) 三欄，由
     * {@link SellerProductNameProjection} 承載，避免 partial-projection 反模式（docs/開發規範/Java 微服務開發規範.md §9.2）。
     *
     * <p>不過濾 deleted_at IS NULL — 軟刪除商品不參與同名提示。
     *
     * <p>caller 限制 names 集合大小 ≤ 500（{@link io.github.will413028.hawkr.product.service.ProductImportService#MAX_NAMES_PER_BATCH}）；
     * 本 method 不二次校驗。
     */
    @Select("""
        <script>
        SELECT id, name, status
        FROM product
        WHERE store_id = #{storeId}
          AND deleted_at IS NULL
          AND seller_deleted_at IS NULL
          AND name IN
        <foreach collection="names" item="n" open="(" separator="," close=")">#{n}</foreach>
        </script>
        """)
    List<SellerProductNameProjection> findSellerProductNameProjections(
            @Param("storeId") Long storeId,
            @Param("names") Collection<String> names);

    /**
     * 批次取得商品輕量資訊（40 號 §4.3b GetProductsBriefInternal）。
     *
     * <p>僅取 (id, name, status) 三欄，由 {@link ProductBriefProjection} 承載，
     * 避免 partial-projection 反模式（docs/開發規範/Java 微服務開發規範.md §9.2）。
     *
     * <p>已軟刪除商品（deleted_at IS NOT NULL）不回傳；查不到的 id 不出現於結果。
     *
     * <p>caller 限制 ids 集合大小 ≤ 200（見 40 號 §4.3b），本 method 不二次校驗。
     */
    @Select("""
        <script>
        SELECT id, name, status
        FROM product
        WHERE deleted_at IS NULL
          AND id IN
        <foreach collection="ids" item="id" open="(" separator="," close=")">#{id}</foreach>
        </script>
        """)
    List<ProductBriefProjection> findBriefByIds(@Param("ids") Collection<Long> ids);

    /**
     * 批次取得商品輕量資訊，<b>刻意不套任何可見性過濾</b>（管理／稽核視角）。
     *
     * <p>與 {@link #findBriefByIds} 是兩支而不是一支參數化的方法。理由**不是**
     * 「那是 40 號 §4.3b 的文件化契約」——40 號其實沒有寫到軟刪（實查全文零提及）——
     * 而是這兩個可查證的事實：① 那支的 javadoc 與 SQL 都明寫已軟刪不回傳
     * ② 它的第二個 caller 是 gRPC {@code GetProductsBriefInternal}，跨服務消費端是
     * product-import-service 的 {@code ItemQueryService.batchFetchCommittedProductsBrief}。
     * 就地放寬會靜默改掉那個服務的畫面（該 surface 自己的缺口見 #2819）。
     *
     * <p>本方法對齊 {@link #findByIdIncludingDeleted} 的語意：**取名字不該被可見性過濾**。
     * 同服務的 MQ 路徑（{@code ProductCertificationApplicationService#publishDecidedAfterCommit}）
     * 早就是這樣取商品名的，gRPC 路徑一度沒跟上，那正是 #2818。
     *
     * <p>回傳三個時間欄供呼叫端交給 {@code ProductPublicationPolicy#deriveState} 判斷公開生命週期；
     * 本方法不自己下判斷（規則的 SSOT 是 {@code ProductPublicationState}）。
     *
     * <p>caller 需自行限制 ids 集合大小（與 {@link #findBriefByIds} 同慣例，≤ 200），
     * 並確保**不傳空集合**——{@code IN ()} 在 PostgreSQL 是語法錯誤，本方法不二次校驗。
     */
    @Select("""
        <script>
        SELECT p.id, p.name, p.status, p.first_published_at, p.deleted_at, p.seller_deleted_at,
               COALESCE(se.exposable, false) AS store_exposable
        FROM product p
        LEFT JOIN store_exposure se ON se.store_id = p.store_id
        WHERE p.id IN
        <foreach collection="ids" item="id" open="(" separator="," close=")">#{id}</foreach>
        </script>
        """)
    List<ProductPublicationBriefProjection> findPublicationBriefByIdsIncludingDeleted(
            @Param("ids") Collection<Long> ids);
}

package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.product.entity.ProductSkuEntity;
import io.github.will413028.hawkr.product.entity.projection.SkuCartProjection;
import io.github.will413028.hawkr.product.entity.projection.SkuNameProjection;
import io.github.will413028.hawkr.product.entity.projection.SkuProductRef;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.util.List;
import java.util.Optional;

/**
 * 商品 SKU Repository
 * 繼承 MyBatis-Plus BaseMapper
 */
@Mapper
public interface ProductSkuRepository extends BaseMapper<ProductSkuEntity> {

    /**
     * 根據商品 ID 查詢所有 SKU
     */
    @Select("SELECT * FROM product_sku WHERE product_id = #{productId} ORDER BY created_at ASC")
    @ResultMap("mybatis-plus_ProductSkuEntity")
    List<ProductSkuEntity> findByProductId(@Param("productId") Long productId);

    /**
     * 根據 SKU ID 查詢
     */
    @Select("SELECT * FROM product_sku WHERE id = #{id}")
    @ResultMap("mybatis-plus_ProductSkuEntity")
    Optional<ProductSkuEntity> findById(@Param("id") Long id);

    /**
     * 刪除商品的所有 SKU
     */
    @Delete("DELETE FROM product_sku WHERE product_id = #{productId}")
    int deleteByProductId(@Param("productId") Long productId);

    /**
     * 批量刪除指定 SKU
     */
    @Delete("""
        <script>
        DELETE FROM product_sku
        WHERE id IN
        <foreach item="id" collection="ids" open="(" separator="," close=")">
            #{id}
        </foreach>
        </script>
        """)
    int deleteByIdIn(@Param("ids") List<Long> ids);

    /**
     * 悲觀鎖查詢（SELECT ... FOR UPDATE）
     */
    @Select("SELECT * FROM product_sku WHERE id = #{id} FOR UPDATE")
    @ResultMap("mybatis-plus_ProductSkuEntity")
    ProductSkuEntity findByIdForUpdate(@Param("id") Long id);

    /**
     * 原子扣減庫存（stock_qty >= quantity 才成功）
     *
     * <p><b>必須 bump {@code version}</b>（#2545）：這是 SKU 唯一一條「賣家看不見卻會改
     * stock_qty」的寫入路徑。不 bump 的話，賣家把編輯表單開著期間賣掉的庫存對 SKU 層樂觀鎖
     * 完全失明——賣家送出時帶著開表單那一刻的 stockQty，CAS 照樣命中，賣掉的庫存被復活，
     * HTTP 200、無任何訊號。{@code @Update} 自訂 SQL 不受 OptimisticLockerInnerInterceptor
     * 攔截，只能逐句手寫（56 §5.4 F41 的 Java 側盲區）。
     *
     * <p>本句自身**不帶** {@code AND version = ?} 述詞：扣庫存是相對運算（-quantity）而非
     * 覆寫，且它的正確性由 {@code stock_qty >= quantity} 這個述詞保證，不需要版本 CAS；
     * 帶了反而會讓結帳在賣家同時編輯時失敗（見 §5.4「product_sku」表第 3 列）。
     *
     * @return 影響列數（0 = 庫存不足，1 = 扣減成功）
     */
    @Update("""
        UPDATE product_sku
           SET stock_qty = stock_qty - #{quantity},
               version = version + 1
         WHERE id = #{skuId} AND stock_qty >= #{quantity}
        """)
    int deductStock(@Param("skuId") Long skuId, @Param("quantity") int quantity);

    /**
     * 歸還庫存。
     *
     * <p>bump {@code version} 的理由與 {@link #deductStock} 完全相同，且**漏掉這條等於樂觀鎖
     * 只擋得住「賣掉」擋不住「取消訂單退回」**——訂單取消（{@code OrderCancelledConsumer}）
     * 同樣會在賣家的編輯窗口內改動 stock_qty。
     */
    @Update("""
        UPDATE product_sku
           SET stock_qty = stock_qty + #{quantity},
               version = version + 1
         WHERE id = #{skuId}
        """)
    int restoreStock(@Param("skuId") Long skuId, @Param("quantity") int quantity);

    /**
     * 批量查詢 SKU → product 反查 + 庫存量。
     * <p>
     * 用於庫存 cascade、cache eviction 等只需 (skuId, productId, stockQty) 三元組的場景。
     * <p>
     * 回傳 record projection 而非 ProductSkuEntity，使 caller 看見的欄位等同 SQL SELECT
     * 的欄位（避免 partial-projection 撒謊；docs/開發規範/Java 微服務開發規範.md §9.2）。
     */
    @Select("""
        <script>
        SELECT id AS sku_id, product_id, stock_qty FROM product_sku
        WHERE id IN
        <foreach item="id" collection="ids" open="(" separator="," close=")">
            #{id}
        </foreach>
        </script>
        """)
    List<SkuProductRef> findSkuProductRefs(@Param("ids") List<Long> ids);

    /**
     * 購物車批次查詢（02 §5.5.3 BatchGetSkuCartInfo）。
     * <p>
     * 單階段 LEFT JOIN 一次拿 SKU + Product 必要欄位（單一 PG snapshot），
     * 取代既有「findSkusByIds + findProductsByIds」兩階段查詢，避免並行
     * UpdateProductStatus / DeductStock 期間 snapshot 漂移造成 cart 假 DELISTED
     * （#1133）。
     * <p>
     * 回傳 record projection（docs/開發規範/Java 微服務開發規範.md §9.2），非 partial-projection 的 Entity，
     * 保證 caller 看到的欄位等同 SQL SELECT 列。
     * <p>
     * product 找不到（LEFT JOIN 後 productStatus 為 null）= product orphan，
     * caller 必須從 response 排除該 sku（caller 走 LOOKUP_FAILED 分支）。
     */
    @Select("""
        <script>
        SELECT s.id AS skuId,
               s.product_id AS productId,
               s.stock_qty AS stockQty,
               s.price AS price,
               s.sale_price AS salePrice,
               s.shipping_method::text AS shippingMethodJson,
               p.status AS productStatus,
               p.deleted_at AS productDeletedAt
        FROM product_sku s
        LEFT JOIN product p ON p.id = s.product_id
        WHERE s.id IN
        <foreach item="id" collection="ids" open="(" separator="," close=")">
            #{id}
        </foreach>
        </script>
        """)
    List<SkuCartProjection> findCartInfoBySkuIds(@Param("ids") List<Long> ids);

    /**
     * 批量查詢商品 SKU 名稱（供商品列表使用）。
     * <p>
     * 回傳 record projection 而非 ProductSkuEntity，避免 partial-projection 反模式
     * （docs/開發規範/Java 微服務開發規範.md §9.2）。
     */
    @Select("""
        <script>
        SELECT product_id, sku_name FROM product_sku
        WHERE product_id IN
        <foreach item="pid" collection="productIds" open="(" separator="," close=")">
            #{pid}
        </foreach>
        ORDER BY product_id, created_at ASC
        </script>
        """)
    List<SkuNameProjection> findSkuNames(@Param("productIds") List<Long> productIds);

    /**
     * 批量查詢商品配送溫層展平列（02 §5.3.6）。
     * temperature 儲存於 shipping_method JSONB 陣列，使用 jsonb_array_elements 展平為扁平 (productId, temperature) 列。
     * Java 層 GROUP BY productId 並 distinct + 排序，避免 JDBC text[] 映射的相容性問題。
     */
    @Select("""
        <script>
        SELECT DISTINCT ps.product_id AS productId,
               m->>'temperature' AS temperature
        FROM product_sku ps,
             jsonb_array_elements(ps.shipping_method) AS m
        WHERE ps.product_id IN
        <foreach item="pid" collection="productIds" open="(" separator="," close=")">
            #{pid}
        </foreach>
          AND m->>'temperature' IS NOT NULL
          AND m->>'temperature' &lt;&gt; ''
        ORDER BY ps.product_id, temperature
        </script>
        """)
    List<ProductDeliveryTempRow> findDeliveryTempByProductIds(@Param("productIds") List<Long> productIds);

    /**
     * SKU 配送溫層展平列（一行對應一個 product_id × temperature）。
     */
    class ProductDeliveryTempRow {
        private Long productId;
        private String temperature;

        public Long getProductId() { return productId; }
        public void setProductId(Long productId) { this.productId = productId; }
        public String getTemperature() { return temperature; }
        public void setTemperature(String temperature) { this.temperature = temperature; }
    }

}

package io.github.will413028.hawkr.productimport.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemSkuEntity;
import io.github.will413028.hawkr.productimport.entity.projection.SkuPriceRangeProjection;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;

/**
 * product_import_item_sku mapper。
 */
public interface ProductImportItemSkuMapper extends BaseMapper<ProductImportItemSkuEntity> {

    /** 列表查詢用 — 即時計算每 item 的 price_min / price_max（§3.10 / §3.11 設計選擇：不落地）。 */
    @Select("""
            <script>
            SELECT
              import_item_id  AS import_item_id,
              MIN(COALESCE(sale_price, price)) AS price_min,
              MAX(COALESCE(sale_price, price)) AS price_max
            FROM product_import_item_sku
            WHERE import_item_id IN
              <foreach collection='itemIds' item='id' open='(' separator=',' close=')'>#{id}</foreach>
            GROUP BY import_item_id
            </script>
            """)
    List<SkuPriceRangeProjection> priceRangeByItems(@Param("itemIds") List<Long> itemIds);
}

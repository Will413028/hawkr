package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.product.entity.HashtagEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.util.List;

/**
 * Hashtag 主表 Repository（02 設計書 §3.7 / §5.12 / §5.13）
 */
@Mapper
public interface HashtagRepository extends BaseMapper<HashtagEntity> {

    /**
     * UPSERT hashtag 主表並回傳 id。
     *
     * <p>名稱命中既有列時只更新 updated_at。RETURNING id 同時涵蓋 INSERT 與 ON CONFLICT 路徑。
     */
    @Select("""
        INSERT INTO hashtag(id, name, created_at, updated_at)
        VALUES (#{id}, #{name}, NOW(), NOW())
        ON CONFLICT (name) DO UPDATE SET updated_at = NOW()
        RETURNING id
        """)
    Long upsertByName(@Param("id") Long id, @Param("name") String name);

    /** 依 canonical name 批次取 hashtag 條目（保留 createdAt/updatedAt 用於展示）。 */
    @Select("""
        <script>
        SELECT * FROM hashtag WHERE name IN
        <foreach collection="names" item="n" open="(" separator="," close=")">#{n}</foreach>
        </script>
        """)
List<HashtagEntity> findByNames(@Param("names") List<String> names);

    /**
     * 依 id list 批次取 hashtag 名稱與公開商品數（一次查詢）。
     * 數量在讀取時計算，只計 ACTIVE、未刪、店家可曝光的商品（與 auto-complete 同一條件）；沒有符合商品時為 0。
     */
    @Select("""
        <script>
        SELECT h.id, h.name, c.product_count
        FROM hashtag h
        CROSS JOIN LATERAL (
            SELECT COUNT(*)::int AS product_count
            FROM product_hashtag ph
            JOIN product p ON p.id = ph.product_id
            WHERE ph.hashtag_id = h.id
              AND p.status = '4004'
              AND p.deleted_at IS NULL
              AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = p.store_id AND se.exposable)
        ) c
        WHERE h.id IN
        <foreach collection="ids" item="i" open="(" separator="," close=")">#{i}</foreach>
        </script>
        """)
    List<HashtagWithCount> findWithPublicCountByIds(@Param("ids") List<Long> ids);

    /** {@link #findWithPublicCountByIds} 結果列。 */
    class HashtagWithCount {
        private Long id;
        private String name;
        private Integer productCount;

        public Long getId() { return id; }
        public void setId(Long id) { this.id = id; }
        public String getName() { return name; }
        public void setName(String name) { this.name = name; }
        public Integer getProductCount() { return productCount; }
        public void setProductCount(Integer productCount) { this.productCount = productCount; }
    }

    /** PREFIX auto-complete（走 idx_hashtag_name_pattern）。數量在讀取時只計可曝光店的 ACTIVE 商品。 */
    @Select("""
        SELECT h.name, c.product_count
        FROM hashtag h
        JOIN LATERAL (
            SELECT COUNT(*)::int AS product_count
            FROM product_hashtag ph
            JOIN product p ON p.id = ph.product_id
            WHERE ph.hashtag_id = h.id
              AND p.status = '4004'
              AND p.deleted_at IS NULL
              AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = p.store_id AND se.exposable)
        ) c ON c.product_count > 0
        WHERE h.name LIKE CONCAT(#{prefix}, '%')
        ORDER BY c.product_count DESC, h.name ASC
        LIMIT #{limit}
        """)
    List<HashtagSuggestion> searchByPrefix(@Param("prefix") String prefix, @Param("limit") int limit);

    /** CONTAIN auto-complete（走 idx_hashtag_name_trgm，IG 風格）。 */
    @Select("""
        SELECT h.name, c.product_count
        FROM hashtag h
        JOIN LATERAL (
            SELECT COUNT(*)::int AS product_count
            FROM product_hashtag ph
            JOIN product p ON p.id = ph.product_id
            WHERE ph.hashtag_id = h.id
              AND p.status = '4004'
              AND p.deleted_at IS NULL
              AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = p.store_id AND se.exposable)
        ) c ON c.product_count > 0
        WHERE h.name ILIKE CONCAT('%', #{prefix}, '%')
        ORDER BY similarity(h.name, #{prefix}) DESC, c.product_count DESC, h.name ASC
        LIMIT #{limit}
        """)
    List<HashtagSuggestion> searchByContain(@Param("prefix") String prefix, @Param("limit") int limit);

    /** Auto-complete 結果列。 */
    class HashtagSuggestion {
        private String name;
        private Integer productCount;

        public String getName() { return name; }
        public void setName(String name) { this.name = name; }
        public Integer getProductCount() { return productCount; }
        public void setProductCount(Integer productCount) { this.productCount = productCount; }
    }
}

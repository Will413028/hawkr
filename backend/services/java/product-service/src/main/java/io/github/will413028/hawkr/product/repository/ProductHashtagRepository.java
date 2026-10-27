package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.product.entity.ProductHashtagEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.time.LocalDateTime;
import java.util.List;

/**
 * product 與 hashtag 關聯表 Repository（02 設計書 §3.8 / §5.2.4 / §5.4）
 */
@Mapper
public interface ProductHashtagRepository extends BaseMapper<ProductHashtagEntity> {

    /** 依 product_id 取所有綁定 hashtag（依 position 升序）。 */
    @Select("""
        SELECT * FROM product_hashtag
        WHERE product_id = #{productId}
        ORDER BY position ASC
        """)
List<ProductHashtagEntity> findByProductId(@Param("productId") Long productId);

    /** 取所有 hashtag_id（給 counter delta 同步使用，以 id ASC 序避免死鎖）。 */
    @Select("""
        SELECT hashtag_id FROM product_hashtag
        WHERE product_id = #{productId}
        ORDER BY hashtag_id ASC
        """)
    List<Long> findHashtagIdsByProductId(@Param("productId") Long productId);

    /** 批次 INSERT product_hashtag。 */
    @Insert("""
        <script>
        INSERT INTO product_hashtag(product_id, hashtag_id, display_name, position, created_at)
        VALUES
        <foreach collection="rows" item="r" separator=",">
            (#{r.productId}, #{r.hashtagId}, #{r.displayName}, #{r.position}, NOW())
        </foreach>
        </script>
        """)
    int batchInsert(@Param("rows") List<ProductHashtagEntity> rows);

    /** 批次 DELETE 指定 product_id + hashtag_ids 的關聯。 */
    @Delete("""
        <script>
        DELETE FROM product_hashtag
        WHERE product_id = #{productId}
          AND hashtag_id IN
          <foreach collection="hashtagIds" item="hid" open="(" separator="," close=")">#{hid}</foreach>
        </script>
        """)
    int deleteByProductAndHashtags(
            @Param("productId") Long productId,
            @Param("hashtagIds") List<Long> hashtagIds);

    /** 全清某 product 的 hashtag 綁定（用於商品刪除等）。 */
    @Delete("DELETE FROM product_hashtag WHERE product_id = #{productId}")
    int deleteAllByProductId(@Param("productId") Long productId);

    /**
     * 更新已存在綁定的 display_name 與 position（複合主鍵不能用 updateById）。
     */
    @org.apache.ibatis.annotations.Update("""
        UPDATE product_hashtag
        SET display_name = #{displayName}, position = #{position}
        WHERE product_id = #{productId} AND hashtag_id = #{hashtagId}
        """)
    int updateBinding(
            @Param("productId") Long productId,
            @Param("hashtagId") Long hashtagId,
            @Param("displayName") String displayName,
            @Param("position") Short position);

    /**
     * 區間熱門 hashtag 查詢（33-商品 Hashtag 模組詳細設計書 §5.2.3）。
     *
     * <p>三段 CTE：
     * <ol>
     *   <li>counts：區間內 distinct ACTIVE 且店家可曝光（{@code store_exposure}）的商品數降冪，取 top-N</li>
     *   <li>display_ranking：top-N hashtag 各 display 變體在區間內出現次數</li>
     *   <li>top_display：每個 hashtag_id 取出現次數最多的 display（同分取最新 created_at）</li>
     * </ol>
     *
     * @param sqlStart     區間下界（inclusive）
     * @param sqlEnd       區間上界（exclusive，左閉右開）
     * @param activeStatus product.status 的 ACTIVE 值（字串，如 "4004"）
     * @param limit        回傳筆數上限（1 ≤ limit ≤ 50）
     */
    @Select("""
        WITH counts AS (
            SELECT ph.hashtag_id,
                   COUNT(DISTINCT ph.product_id) AS cnt
            FROM product_hashtag ph
            JOIN product p ON p.id = ph.product_id
            WHERE ph.created_at >= #{sqlStart}
              AND ph.created_at <  #{sqlEnd}
              AND p.status      =  #{activeStatus}
              AND p.deleted_at  IS NULL
              AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = p.store_id AND se.exposable)
            GROUP BY ph.hashtag_id
            ORDER BY cnt DESC, ph.hashtag_id ASC
            LIMIT #{limit}
        ),
        display_ranking AS (
            SELECT ph.hashtag_id,
                   ph.display_name,
                   COUNT(*)           AS dn_count,
                   MAX(ph.created_at) AS dn_latest
            FROM product_hashtag ph
            JOIN product p ON p.id = ph.product_id
            JOIN counts c  ON c.hashtag_id = ph.hashtag_id
            WHERE ph.created_at >= #{sqlStart}
              AND ph.created_at <  #{sqlEnd}
              AND p.status      =  #{activeStatus}
              AND p.deleted_at  IS NULL
              AND EXISTS (SELECT 1 FROM store_exposure se WHERE se.store_id = p.store_id AND se.exposable)
            GROUP BY ph.hashtag_id, ph.display_name
        ),
        top_display AS (
            SELECT DISTINCT ON (hashtag_id) hashtag_id, display_name
            FROM display_ranking
            ORDER BY hashtag_id, dn_count DESC, dn_latest DESC
        )
        SELECT td.display_name AS hashtag,
               c.cnt::text     AS count
        FROM counts c
        JOIN top_display td USING (hashtag_id)
        ORDER BY c.cnt DESC, c.hashtag_id ASC
        """)
    List<PopularHashtagRow> selectPopularInRange(
            @Param("sqlStart") LocalDateTime sqlStart,
            @Param("sqlEnd") LocalDateTime sqlEnd,
            @Param("activeStatus") String activeStatus,
            @Param("limit") int limit);

    /** SQL 投影結果（hashtag + count 字串型別）。 */
    class PopularHashtagRow {
        private String hashtag;
        private String count;

        public String getHashtag() { return hashtag; }
        public void setHashtag(String hashtag) { this.hashtag = hashtag; }
        public String getCount() { return count; }
        public void setCount(String count) { this.count = count; }
    }
}

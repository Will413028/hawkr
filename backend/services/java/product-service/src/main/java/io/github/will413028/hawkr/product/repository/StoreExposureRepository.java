package io.github.will413028.hawkr.product.repository;

import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.Collection;
import java.util.List;

/**
 * 店家可曝光的讀取模型（{@code store_exposure}）。沒有列等同不可曝光。
 */
@Mapper
public interface StoreExposureRepository {

    /**
     * 依版本寫入：只有事件版本大於現有版本（或尚無列）才會改動，舊版本與重送都不改任何東西。
     *
     * @return 1 = 新增或更新了列；0 = 事件版本小於或等於現有版本而丟棄
     */
    @Insert("""
        INSERT INTO store_exposure (store_id, exposable, version, updated_at)
        VALUES (#{storeId}, #{exposable}, #{version}, now())
        ON CONFLICT (store_id) DO UPDATE
        SET exposable = EXCLUDED.exposable,
            version = EXCLUDED.version,
            updated_at = EXCLUDED.updated_at
        WHERE store_exposure.version < EXCLUDED.version
        """)
    int upsertIfNewer(
            @Param("storeId") long storeId,
            @Param("exposable") boolean exposable,
            @Param("version") long version);

    /** 目前的版本；沒有列回 null。 */
    @Select("SELECT version FROM store_exposure WHERE store_id = #{storeId}")
    Long findVersion(@Param("storeId") long storeId);

    /** 單一店家是否可曝光；沒有列回 false。 */
    @Select("""
        SELECT COALESCE((SELECT exposable FROM store_exposure WHERE store_id = #{storeId}), false)
        """)
    boolean isExposable(@Param("storeId") Long storeId);

    /** 批次：在給定店家中挑出目前可曝光的（沒有列的不會出現）。呼叫端須保證集合非空。 */
    @Select("""
        <script>
        SELECT store_id FROM store_exposure
        WHERE exposable
          AND store_id IN
        <foreach collection="storeIds" item="sid" open="(" separator="," close=")">#{sid}</foreach>
        </script>
        """)
    List<Long> findExposableStoreIds(@Param("storeIds") Collection<Long> storeIds);
}

package io.github.will413028.hawkr.media.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.media.entity.AssetReferenceEntity;
import org.apache.ibatis.annotations.*;

import java.util.List;

/**
 * 引用關聯 Repository
 */
@Mapper
public interface AssetReferenceRepository extends BaseMapper<AssetReferenceEntity> {

    /**
     * 插入引用（忽略衝突，實現冪等）
     * 使用 ON CONFLICT DO NOTHING
     */
    @Insert("""
        INSERT INTO asset_reference (asset_id, ref_type, ref_id, created_at)
        VALUES (#{assetId}, #{refType}, #{refId}, NOW())
        ON CONFLICT (asset_id, ref_type, ref_id) DO NOTHING
        """)
    int insertIgnoreConflict(
        @Param("assetId") Long assetId,
        @Param("refType") String refType,
        @Param("refId") String refId
    );

    /**
     * 批次插入引用（忽略衝突）
     *
     * @return 實際插入的行數
     */
    @Insert("""
        <script>
        INSERT INTO asset_reference (asset_id, ref_type, ref_id, created_at)
        VALUES
        <foreach item="ref" collection="references" separator=",">
            (#{ref.assetId}, #{ref.refType}, #{ref.refId}, NOW())
        </foreach>
        ON CONFLICT (asset_id, ref_type, ref_id) DO NOTHING
        </script>
        """)
    int batchInsertIgnoreConflict(@Param("references") List<AssetReferenceEntity> references);

    /**
     * 刪除指定引用
     */
    @Delete("""
        DELETE FROM asset_reference
        WHERE asset_id = #{assetId}
          AND ref_type = #{refType}
          AND ref_id = #{refId}
        """)
    int deleteReference(
        @Param("assetId") Long assetId,
        @Param("refType") String refType,
        @Param("refId") String refId
    );

    /**
     * 批次刪除引用
     */
    @Delete("""
        <script>
        DELETE FROM asset_reference
        WHERE ref_type = #{refType}
          AND ref_id = #{refId}
          AND asset_id IN
        <foreach item="assetId" collection="assetIds" open="(" separator="," close=")">
            #{assetId}
        </foreach>
        </script>
        """)
    int batchDeleteReferences(
        @Param("refType") String refType,
        @Param("refId") String refId,
        @Param("assetIds") List<Long> assetIds
    );

    /**
     * 依 (ref_type, ref_id) 批次刪除引用（不需 asset_id），可保留 keepAssetIds 名單。
     * <p>
     * 兩種呼叫語意：
     * <ul>
     *   <li>{@code keepAssetIds} 空 → 釋放全部（帳號 / 實體刪除）。</li>
     *   <li>{@code keepAssetIds} 非空 → 換圖：單一 SQL 內「刪舊留新」，
     *       不存在「新資產一度失去引用」的窗口。</li>
     * </ul>
     */
    @Delete("""
        <script>
        DELETE FROM asset_reference
        WHERE ref_type = #{refType}
          AND ref_id IN
        <foreach item="refId" collection="refIds" open="(" separator="," close=")">
            #{refId}
        </foreach>
        <if test="keepAssetIds != null and keepAssetIds.size() > 0">
          AND asset_id NOT IN
          <foreach item="assetId" collection="keepAssetIds" open="(" separator="," close=")">
            #{assetId}
          </foreach>
        </if>
        </script>
        """)
    int deleteByRefTypeAndRefIds(
        @Param("refType") String refType,
        @Param("refIds") List<String> refIds,
        @Param("keepAssetIds") List<Long> keepAssetIds
    );

    /**
     * 查詢資產的引用數量
     */
    @Select("SELECT COUNT(*) FROM asset_reference WHERE asset_id = #{assetId}")
    int countByAssetId(@Param("assetId") Long assetId);

    /**
     * 查詢指定業務的所有引用
     */
    @Select("""
        SELECT * FROM asset_reference
        WHERE ref_type = #{refType}
          AND ref_id = #{refId}
        """)
    List<AssetReferenceEntity> selectByRef(
        @Param("refType") String refType,
        @Param("refId") String refId
    );

    /**
     * 指定的 {@code (asset_id, ref_type, ref_id)} 三元組是否存在。
     *
     * <p>{@code ServerReadSecureAsset} 的授權述詞：呼叫端必須同時說得出資產路徑與
     * 「這份資產屬於哪一筆業務」，兩者對不上就當作資產不存在。三欄上有唯一索引
     * （見 {@link #insertIgnoreConflict} 的 {@code ON CONFLICT}），最多命中一列。
     */
    @Select("""
        SELECT COUNT(*) > 0 FROM asset_reference
        WHERE asset_id = #{assetId}
          AND ref_type = #{refType}
          AND ref_id = #{refId}
        """)
    boolean existsReference(
        @Param("assetId") Long assetId,
        @Param("refType") String refType,
        @Param("refId") String refId
    );

    /**
     * 查詢帳號是否有權存取資產
     */
    @Select("""
        SELECT COUNT(*) > 0 FROM asset_reference ar
        JOIN media_asset ma ON ar.asset_id = ma.id
        WHERE ar.asset_id = #{assetId}
          AND ma.uploader_account_id = #{accountId}
        """)
    boolean hasAccessByAccount(@Param("assetId") Long assetId, @Param("accountId") Long accountId);

    /**
     * 刪除 asset 已不存在的孤立引用（GC 第三階段用）
     */
    @Delete("""
        DELETE FROM asset_reference ar
        WHERE NOT EXISTS (
            SELECT 1 FROM media_asset ma WHERE ma.id = ar.asset_id
        )
        """)
    int deleteOrphanedReferences();
}

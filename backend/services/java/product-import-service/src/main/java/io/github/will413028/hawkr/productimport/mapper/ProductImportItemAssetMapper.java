package io.github.will413028.hawkr.productimport.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.projection.AssetCountByStatusProjection;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;

/**
 * product_import_item_asset mapper。
 */
public interface ProductImportItemAssetMapper extends BaseMapper<ProductImportItemAssetEntity> {

    /** 同 job 內檢查同 source_url_hash 是否已存在（dedup）。 */
    @Select("""
            SELECT * FROM product_import_item_asset
            WHERE import_job_id = #{importJobId}
              AND source_url_hash = #{sourceUrlHash}
            LIMIT 1
            """)
    ProductImportItemAssetEntity findByJobAndHash(@Param("importJobId") Long importJobId,
                                                   @Param("sourceUrlHash") String sourceUrlHash);

    /** Counters 用：job 內依 status 統計。 */
    @Select("""
            SELECT status AS status, COUNT(*) AS cnt
            FROM product_import_item_asset
            WHERE import_job_id = #{importJobId}
            GROUP BY status
            """)
    List<AssetCountByStatusProjection> countByStatusGroup(@Param("importJobId") Long importJobId);

    /** GC 用：掃 ORPHAN（status=9306）超過 grace 期間的 asset。 */
    @Select("""
            SELECT * FROM product_import_item_asset
            WHERE status = '9306'
              AND updated_at < #{cutoff}
            LIMIT #{limit}
            """)
    List<ProductImportItemAssetEntity> findOrphanCandidates(@Param("cutoff") java.time.LocalDateTime cutoff,
                                                            @Param("limit") int limit);

    /**
     * issue #2294 L9：callback timeout GC 的 <b>lease claim</b>（零 migration）。
     *
     * <p>候選述詞是 {@code status='9302'(DOWNLOADING) AND updated_at < #{cutoff}}
     * ——<b>候選述詞自己就用了一個時間欄</b>，所以把 {@code updated_at} 推到 <b>claim 當下</b>
     * （{@code claimedAt}），該列就退出候選集合 {@code callbackTimeoutMinutes}
     * （＝有效租約長度，預設 30 分），到期自動回來。
     * <p>⚠️ {@code claimedAt} 必須是<b>每一筆各自取</b>的 {@code nowUtc()}，不是 tick 起點那個：
     * 取 tick 起點 ⇒ 全批共用同一個到期時間 ⇒ 不變式退化成「批次總耗時 &lt; 租約」。
     *
     * <p>⚠️ 此處推進 {@code updated_at} 不違反 L5（aftersales {@code RefundSyncWorker}）
     * 對 {@code updated_at} 的禁令：L5 是拿它當「事件已補發」的記號（語意錯配），
     * 這裡它就是候選述詞本身的語意（「多久沒動過」）。
     *
     * <p>閘門之後的 {@code DOWNLOADING → FAILED} 業務轉換與 {@code job.parsed} publish
     * <b>順序不變</b>——那個轉換是狀態機本來就要推進的事，只是不能拿它當閘門。
     */
    @org.apache.ibatis.annotations.Update("""
            UPDATE product_import_item_asset
            SET updated_at = #{claimedAt}
            WHERE id = #{id}
              AND status = #{statusDownloading}
              AND updated_at < #{cutoff}
            """)
    int claimStaleDownloadingAsset(@Param("id") Long id,
                                   @Param("statusDownloading") String statusDownloading,
                                   @Param("cutoff") java.time.LocalDateTime cutoff,
                                   @Param("claimedAt") java.time.LocalDateTime claimedAt);
}

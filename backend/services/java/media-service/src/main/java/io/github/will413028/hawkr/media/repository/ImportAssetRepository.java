package io.github.will413028.hawkr.media.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.media.entity.ImportAssetEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

/**
 * import_asset Repository（08 號設計書 §7.1）。
 *
 * <p>PR1 階段：純查詢 helper（idempotency_key dedup、過期 GC 候選），實際 RPC handler stub。
 * 完整 worker pool 實作於 PR2。</p>
 */
@Mapper
public interface ImportAssetRepository extends BaseMapper<ImportAssetEntity> {

    /**
     * 以 (store_id, import_job_id, source_url_hash) 三元組查既有 row。
     * ImportDownloadAsset 入口冪等檢查用。
     */
    @Select("""
        SELECT * FROM import_asset
        WHERE store_id = #{storeId}
          AND import_job_id = #{importJobId}
          AND source_url_hash = #{sourceUrlHash}
        LIMIT 1
        """)
    Optional<ImportAssetEntity> findByDedupKey(
        @Param("storeId") Long storeId,
        @Param("importJobId") Long importJobId,
        @Param("sourceUrlHash") String sourceUrlHash
    );

    /**
     * 以 (store_id, import_job_id, staged_session_id) 三元組查既有 STAGED row。
     * BindStagedAsset 入口冪等檢查用。
     */
    @Select("""
        SELECT * FROM import_asset
        WHERE store_id = #{storeId}
          AND import_job_id = #{importJobId}
          AND source_type = 'STAGED'
          AND staged_session_id = #{stagedSessionId}
        LIMIT 1
        """)
    Optional<ImportAssetEntity> findByStagedDedupKey(
        @Param("storeId") Long storeId,
        @Param("importJobId") Long importJobId,
        @Param("stagedSessionId") String stagedSessionId
    );

    /**
     * 查過期未 Finalize 的 import_asset（GC 第 4 層第 1 階段）。
     * status ∈ {DOWNLOADING, UPLOADED, FAILED} AND expires_at < ?
     */
    @Select("""
        SELECT * FROM import_asset
        WHERE status IN ('9401', '9402', '9404')
          AND expires_at < #{now}
        LIMIT #{limit}
        """)
    List<ImportAssetEntity> selectExpiredCandidates(
        @Param("now") LocalDateTime now,
        @Param("limit") int limit
    );

    /**
     * 查同一 import_job 的所有非終態 asset（gc.expire_job MQ consumer 用）。
     */
    @Select("""
        SELECT * FROM import_asset
        WHERE import_job_id = #{importJobId}
          AND status NOT IN ('9403', '9405')
        """)
    List<ImportAssetEntity> selectActiveByJobId(@Param("importJobId") Long importJobId);

    /**
     * 以 FOR UPDATE 鎖定單筆 import_asset（worker pool 處理、FinalizeImportAsset 同事務內鎖列用）。
     */
    @Select("""
        SELECT * FROM import_asset
        WHERE id = #{id}
        FOR UPDATE
        """)
    Optional<ImportAssetEntity> findByIdForUpdate(@Param("id") String id);

    /**
     * 標記指定 job 下所有非終態 import_asset 為 EXPIRED（gc.expire_job 消費端 idempotent UPDATE）。
     * <p>條件帶 {@code status NOT IN (FINALIZED, EXPIRED)}，重複收到事件不會重複處理。</p>
     *
     * @return 影響筆數
     */
    @Update("""
        UPDATE import_asset
        SET status = '9405',
            expired_at = #{now},
            updated_at = #{now}
        WHERE import_job_id = #{importJobId}
          AND status NOT IN ('9403', '9405')
        """)
    int markExpiredByJobId(@Param("importJobId") Long importJobId,
                           @Param("now") LocalDateTime now);

    /**
     * 查已 EXPIRED 滿 N 天的 row（GC 第 4 層第 2 階段：DELETE 超過保留期者）。
     */
    @Select("""
        SELECT * FROM import_asset
        WHERE status = '9405'
          AND updated_at < #{threshold}
        LIMIT #{limit}
        """)
    List<ImportAssetEntity> selectExpiredRetentionCandidates(
        @Param("threshold") LocalDateTime threshold,
        @Param("limit") int limit
    );
}

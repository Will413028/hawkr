package io.github.will413028.hawkr.media.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;
import java.util.UUID;

/**
 * 上傳會話 Repository
 */
@Mapper
public interface UploadSessionRepository extends BaseMapper<UploadSessionEntity> {

    /**
     * 依 ID 與帳號查詢會話
     */
    @Select("SELECT * FROM upload_session WHERE id = #{id} AND account_id = #{accountId}")
    UploadSessionEntity selectByIdAndAccountId(@Param("id") UUID id, @Param("accountId") Long accountId);

    /**
     * 更新會話狀態（原子操作，帶狀態條件）
     *
     * @return 受影響的行數
     */
    @Update("UPDATE upload_session SET status = #{newStatus} WHERE id = #{id} AND status = #{expectedStatus}")
    int updateStatusWithCondition(
            @Param("id") UUID id,
            @Param("expectedStatus") String expectedStatus,
            @Param("newStatus") String newStatus
    );

    /**
     * 更新會話狀態與到期時間（原子操作，帶狀態條件）
     * 用於機敏上傳完成後，同時將狀態改為 COMPLETED 並延長 expires_at 為預覽有效期
     *
     * @return 受影響的行數
     */
    @Update("UPDATE upload_session SET status = #{newStatus}, expires_at = #{expiresAt} WHERE id = #{id} AND status = #{expectedStatus}")
    int updateStatusAndExpiresAt(
            @Param("id") UUID id,
            @Param("expectedStatus") String expectedStatus,
            @Param("newStatus") String newStatus,
            @Param("expiresAt") LocalDateTime expiresAt
    );

    /**
     * 更新會話狀態、到期時間、加密金鑰與 object key（原子操作，帶狀態條件）
     * 用於機敏上傳完成後，同時設定加密金鑰、把本次 attempt 真正寫入的 object key
     * 記錄為贏家，並改狀態為 COMPLETED。
     *
     * <p>issue #2759：每次 {@code secureUpload} 呼叫都用全新、per-attempt 專屬的 object
     * key（不重用 session 建立時就固定的 key），只有這個 CAS 贏家的 key 才會被寫回；
     * 輸家的 R2 物件不會被任何 DB 列引用，由 controller 自我刪除。
     *
     * @return 受影響的行數
     */
    @Update("UPDATE upload_session SET status = #{newStatus}, expires_at = #{expiresAt}, encryption_key = #{encryptionKey}, object_key = #{objectKey} WHERE id = #{id} AND status = #{expectedStatus}")
    int updateStatusExpiresAtEncryptionKeyAndObjectKey(
            @Param("id") UUID id,
            @Param("expectedStatus") String expectedStatus,
            @Param("newStatus") String newStatus,
            @Param("expiresAt") LocalDateTime expiresAt,
            @Param("encryptionKey") String encryptionKey,
            @Param("objectKey") String objectKey
    );

    /**
     * 回填轉正後的 asset_id（用於重入查詢）
     *
     * @return 受影響的行數
     */
    @Update("UPDATE upload_session SET asset_id = #{assetId} WHERE id = #{id}")
    int updateAssetId(@Param("id") UUID id, @Param("assetId") Long assetId);

    /**
     * 查詢過期且未完成的會話（用於 GC）
     *
     * <p>issue #2294 L8：加上 {@code gc_lease_until} 租約述詞——租約未到期的列由別的 pod 持有，
     * 本輪不撈。這同時是 {@code cleanExpiredSessions} 那個 do/while 迴圈的<b>終止條件</b>：
     * 被搶走或處理失敗的列都帶著未到期的租約，下一圈不會再被撈到。
     *
     * <p><b>R4-P1-2（2026-08-08）：{@code ORDER BY} 是 deadlock 的解，不是排版</b>。
     * 沒有它時 PostgreSQL 的 {@code synchronize_seqscans} 讓並行 seq scan 從不同位置起掃，
     * 兩個 pod 的取鎖順序可以相反 ⇒ deadlock（兩 pod 的 GC cron 同為每日 03:00）。
     * {@code (expires_at, gc_lease_until)} 部分索引（V8）直接吃 {@code expires_at ASC} 前綴，
     * {@code id} 只是打破併列的 tie-break。
     */
    default List<UploadSessionEntity> selectExpiredSessions(LocalDateTime now, int limit) {
        return selectExpiredSessionsByStatus(now, limit,
                MediaUploadSessionStatus.INIT.value(),
                MediaUploadSessionStatus.COMPLETED.value());
    }

    @Select("""
        SELECT * FROM upload_session
        WHERE status IN (#{statusInit}, #{statusCompleted})
          AND expires_at < #{now}
          AND (gc_lease_until IS NULL OR gc_lease_until <= #{now})
        ORDER BY expires_at ASC, id ASC
        LIMIT #{limit}
        """)
    List<UploadSessionEntity> selectExpiredSessionsByStatus(
            @Param("now") LocalDateTime now,
            @Param("limit") int limit,
            @Param("statusInit") String statusInit,
            @Param("statusCompleted") String statusCompleted
    );

    /**
     * issue #2294 L8：單筆 GC <b>lease claim</b>——搶到才可以刪 OSS 物件。
     *
     * <p>{@code SET gc_lease_until = #{leaseUntil}}（= claim 當下 + 60 分），affected=1 才做副作用。
     * ⚠️ 刻意<b>不</b>挪用 {@code expires_at}：那是業務有效性判準（過期擋用），往後推等於
     * 重新打開一個已過期的上傳窗口。
     */
    default int claimSessionForGc(UUID id, LocalDateTime now, LocalDateTime leaseUntil) {
        return claimSessionForGcByStatus(id, now, leaseUntil,
                MediaUploadSessionStatus.INIT.value(),
                MediaUploadSessionStatus.COMPLETED.value());
    }

    @Update("""
        UPDATE upload_session
        SET gc_lease_until = #{leaseUntil}
        WHERE id = #{id}
          AND status IN (#{statusInit}, #{statusCompleted})
          AND expires_at < #{now}
          AND (gc_lease_until IS NULL OR gc_lease_until <= #{now})
        """)
    int claimSessionForGcByStatus(
            @Param("id") UUID id,
            @Param("now") LocalDateTime now,
            @Param("leaseUntil") LocalDateTime leaseUntil,
            @Param("statusInit") String statusInit,
            @Param("statusCompleted") String statusCompleted
    );

    /**
     * issue #2294 L8：<b>單筆</b>標記過期（取代舊的整批 {@code markExpiredSessions}）。
     *
     * <p>舊版是「整批依條件更新」，無法只標記物件刪除成功的那幾筆，所以只要本輪有任何一筆
     * 儲存端失敗就整輪不標記；那個語意與 per-row 租約不相容（會把別的 pod 剛搶走的列也一起標）。
     * 改成 per-row 之後，成功的那幾筆立刻進終態，失敗的那幾筆留著租約等下一輪。
     */
    default int markSessionExpired(UUID id) {
        return markSessionExpiredByStatus(id,
                MediaUploadSessionStatus.EXPIRED.value(),
                MediaUploadSessionStatus.INIT.value(),
                MediaUploadSessionStatus.COMPLETED.value());
    }

    @Update("""
        UPDATE upload_session
        SET status = #{statusExpired}
        WHERE id = #{id}
          AND status IN (#{statusInit}, #{statusCompleted})
        """)
    int markSessionExpiredByStatus(
            @Param("id") UUID id,
            @Param("statusExpired") String statusExpired,
            @Param("statusInit") String statusInit,
            @Param("statusCompleted") String statusCompleted
    );
}

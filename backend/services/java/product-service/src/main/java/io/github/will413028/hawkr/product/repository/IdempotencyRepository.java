package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.IdempotencyRequestStatus;
import io.github.will413028.hawkr.product.entity.IdempotencyRequestEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.util.Optional;

/**
 * 冪等性請求 Repository
 * 繼承 MyBatis-Plus BaseMapper
 */
@Mapper
public interface IdempotencyRepository extends BaseMapper<IdempotencyRequestEntity> {

    /**
     * 根據冪等鍵查詢
     */
    @Select("SELECT * FROM idempotency_request WHERE key = #{key}")
    Optional<IdempotencyRequestEntity> findByKey(@Param("key") String key);

    /**
     * 根據冪等鍵和賣家查詢
     */
    @Select("SELECT * FROM idempotency_request WHERE key = #{key} AND store_id = #{storeId}")
    Optional<IdempotencyRequestEntity> findByKeyAndStoreId(@Param("key") String key, @Param("storeId") Long storeId);

    /**
     * 嘗試插入冪等記錄（靠 DB 唯一約束確保原子性）
     * 已存在相同 (key, store_id) 則不做任何事
     * 回傳受影響行數：1 表示成功插入，0 表示已存在。status 走 IdempotencyRequestStatus enum value。
     */
    default int tryInsert(String key, Long storeId, String endpoint, String requestHash) {
        return tryInsertWithStatus(key, storeId, endpoint, requestHash,
                IdempotencyRequestStatus.IN_PROGRESS.value());
    }

    /**
     * ⚠️ {@code ON CONFLICT DO NOTHING} 刻意<b>不指名衝突目標</b>（#2306）。
     *
     * <p>這張表在階段 1～2 之間同時存在兩個唯一約束：舊的單欄 {@code key} 主鍵，
     * 與新的 {@code (key, store_id)}。指名其中任一個都會在某個時點壞掉——指名舊的，
     * 階段 2 移除它之後每一次 INSERT 都報錯；指名新的，跨租戶撞鍵時會撞上尚未移除
     * 的舊主鍵。不帶 conflict target 的 {@code DO NOTHING} 對<b>任何</b>唯一約束衝突
     * 都靜默跳過，因此與「現在有哪些約束」無關，滾動更新沒有窗口。
     *
     * <p>⚠️ 不能改成「讓 DB 拋錯、Java 端 catch {@code DuplicateKeyException}」——
     * 那個做法在 PostgreSQL 上必定失敗：交易內任何一句 SQL 出錯就會讓整個交易進入
     * aborted 狀態，之後所有指令一律被拒（SQLSTATE 25P02）直到 ROLLBACK。於是後續
     * 的 {@code findByKeyAndStoreId} 會炸成 500 而不是走進「已存在」分支。
     * 2026-08-07 dev 實測踩到，這正是 ON CONFLICT 存在的理由。
     */
    @Update("""
        INSERT INTO idempotency_request (key, store_id, endpoint, status, request_hash)
        VALUES (#{key}, #{storeId}, #{endpoint}, #{status}, #{requestHash})
        ON CONFLICT DO NOTHING
        """)
    int tryInsertWithStatus(@Param("key") String key, @Param("storeId") Long storeId,
                            @Param("endpoint") String endpoint, @Param("requestHash") String requestHash,
                            @Param("status") String status);

    /**
     * 更新狀態為成功並保存回應。status 走 IdempotencyRequestStatus.SUCCEEDED value。
     */
    default int markSucceeded(String key, Long storeId, String responseBody) {
        return markStatus(key, storeId, IdempotencyRequestStatus.SUCCEEDED.value(), responseBody);
    }

    /**
     * 更新狀態為失敗。status 走 IdempotencyRequestStatus.FAILED value。
     */
    default int markFailed(String key, Long storeId) {
        return markStatus(key, storeId, IdempotencyRequestStatus.FAILED.value(), null);
    }

    @Update("""
        UPDATE idempotency_request
        SET status = #{status},
            response_body = COALESCE(CAST(#{responseBody} AS jsonb), response_body)
        WHERE key = #{key} AND store_id = #{storeId}
        """)
    int markStatus(@Param("key") String key, @Param("storeId") Long storeId,
                   @Param("status") String status, @Param("responseBody") String responseBody);

    /**
     * 刪除過期的冪等記錄（可用於清理任務）
     * 預設保留 24 小時內的記錄
     */
    @Update("DELETE FROM idempotency_request WHERE created_at < NOW() - INTERVAL '24 hours'")
    int deleteExpired();

    /**
     * 刪除單筆冪等記錄。
     *
     * <p>不用 BaseMapper.deleteById：那個走 {@code @TableId} 只認 key 單欄，
     * 在兩個賣家共用同一把 key 時會連別人的記錄一起刪掉（#2306）。
     */
    @Update("DELETE FROM idempotency_request WHERE key = #{key} AND store_id = #{storeId}")
    int deleteByKeyAndStoreId(@Param("key") String key, @Param("storeId") Long storeId);
}

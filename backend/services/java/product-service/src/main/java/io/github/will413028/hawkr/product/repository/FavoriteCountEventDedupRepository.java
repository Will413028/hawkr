package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.product.entity.FavoriteCountEventDedupEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;

import java.time.LocalDateTime;

/**
 * 收藏計數事件去重表 Repository（02 §3.5.1 / §5.14）。
 *
 * eventId 在 Java 層以 String（UUID v4 字串）表示，SQL 端 ::uuid cast。
 */
@Mapper
public interface FavoriteCountEventDedupRepository extends BaseMapper<FavoriteCountEventDedupEntity> {

    /**
     * 冪等寫入：ON CONFLICT DO NOTHING；rowsAffected = 1 代表本次是首次處理該 event。
     *
     * @return 1 = 首次寫入；0 = 重複事件已被忽略
     */
    @Insert("""
        INSERT INTO favorite_count_event_dedup(event_id, event_type, product_id)
        VALUES (#{eventId}::uuid, #{eventType}, #{productId})
        ON CONFLICT (event_id) DO NOTHING
        """)
    int insertIfAbsent(@Param("eventId") String eventId,
                       @Param("eventType") String eventType,
                       @Param("productId") Long productId);

    /**
     * 清理舊事件（{@link io.github.will413028.hawkr.product.scheduler.FavoriteEventDedupCleanupJob} 每日 02:30）。
     */
    @Delete("DELETE FROM favorite_count_event_dedup WHERE consumed_at < #{before}")
    int deleteBefore(@Param("before") LocalDateTime before);
}

package io.github.will413028.hawkr.product.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.product.entity.SalesCountEventDedupEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;

/**
 * 銷量累計事件去重表 Repository（35 設計書 §5）。
 *
 * 比照 {@link FavoriteCountEventDedupRepository} 建立；去重鍵為 order_no。
 */
@Mapper
public interface SalesCountEventDedupRepository extends BaseMapper<SalesCountEventDedupEntity> {

    /**
     * 冪等 claim：ON CONFLICT DO NOTHING；rowsAffected = 1 代表本次是首次處理該訂單。
     *
     * @return 1 = 首次寫入（本次應累計銷量）；0 = 重複事件已被忽略（跳過累計）
     */
    @Insert("""
        INSERT INTO sales_count_event_dedup(order_no)
        VALUES (#{orderNo})
        ON CONFLICT (order_no) DO NOTHING
        """)
    int insertIfAbsent(@Param("orderNo") String orderNo);
}

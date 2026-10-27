package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 收藏計數事件去重表（02 §3.5.1）。
 *
 * FavoriteCountConsumer 同事務 INSERT ON CONFLICT DO NOTHING；rowsAffected = 1 才執行 product.favorite_count ±1。
 *
 * 由 {@link io.github.will413028.hawkr.product.scheduler.FavoriteEventDedupCleanupJob} 每日 02:30 清理 30 天前。
 *
 * eventId 在 Java 層以 String 表示（buyer publisher 生成 UUID v4 字串），SQL 端以 ::uuid cast 對應 PostgreSQL UUID 欄位。
 */
@Data
@TableName(value = "favorite_count_event_dedup", autoResultMap = true)
public class FavoriteCountEventDedupEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /** 事件 ID（由 buyer-service publisher 生成 UUID v4 字串） */
    @TableId(type = IdType.INPUT)
    private String eventId;

    /** {@link io.github.will413028.hawkr.common.utils.code.BuyerEventType} Value（UPPER_SNAKE） */
    private String eventType;

    /** 目標商品 ID */
    private Long productId;

    /** 消費時間（DB DEFAULT NOW()） */
    private LocalDateTime consumedAt;
}

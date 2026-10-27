package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 銷量累計事件去重表（35 設計書 §5）。
 *
 * SalesAccumulator.applyAll 同事務 INSERT ON CONFLICT DO NOTHING；rowsAffected = 1 才逐筆累計
 * product.sales_count。去重鍵 order_no：一張訂單的銷量只累計一次，涵蓋 MQ 事件重投與重複發布兩種情境。
 *
 * <p>比照 {@link FavoriteCountEventDedupEntity}（收藏計數去重）建立。
 */
@Data
@TableName(value = "sales_count_event_dedup", autoResultMap = true)
public class SalesCountEventDedupEntity implements Serializable {
    private static final long serialVersionUID = 1L;

    /** 訂單編號（去重鍵；order.sales.accumulated 事件的自然鍵） */
    @TableId(type = IdType.INPUT)
    private String orderNo;

    /** 消費時間（DB DEFAULT NOW()） */
    private LocalDateTime consumedAt;
}

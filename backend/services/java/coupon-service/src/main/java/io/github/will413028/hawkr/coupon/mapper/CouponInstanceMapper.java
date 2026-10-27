package io.github.will413028.hawkr.coupon.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.coupon.entity.CouponInstanceEntity;
import io.github.will413028.hawkr.coupon.entity.projection.CouponExpiringRef;
import java.time.LocalDateTime;
import java.util.List;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

public interface CouponInstanceMapper extends BaseMapper<CouponInstanceEntity> {

    /**
     * 鎖定優惠券（原子操作，冪等）
     * status value: 0=UNUSED, 1=RESERVED
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '1',
            reserved_order_no = #{orderNo},
            reserved_at = NOW()
        WHERE instance_id = #{instanceId}
        AND owner_buyer_id = #{buyerId}
        AND (
            status = '0'
            OR (status = '1' AND reserved_order_no = #{orderNo})
        )
        """)
    int reserveCoupon(@Param("instanceId") Long instanceId,
                      @Param("buyerId") Long buyerId,
                      @Param("orderNo") String orderNo);

    /**
     * 消耗優惠券（RESERVED → USED，原子操作，冪等）
     * status value: 1=RESERVED, 2=USED
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '2',
            used_order_no = #{orderNo},
            used_at = NOW(),
            reserved_order_no = NULL,
            reserved_at = NULL
        WHERE instance_id = #{instanceId}
        AND status = '1'
        AND reserved_order_no = #{orderNo}
        """)
    int consumeCoupon(@Param("instanceId") Long instanceId, @Param("orderNo") String orderNo);

    /**
     * 釋放優惠券（原子操作，冪等）
     *
     * 涵蓋兩種起始狀態：
     * - RESERVED → UNUSED：付款超時 / 買家放棄付款 / 賣家取消未付款訂單（match reserved_order_no）
     * - USED     → UNUSED：賣家取消已付款／待出貨訂單（match used_order_no）
     *
     * 依 §2.6.1 業務規則：賣家取消是還券的唯一可從 USED 復原的路徑，與其他路徑共用
     * 同一個 RPC 入口，呼叫端不需要知道券當下狀態。
     *
     * status value: 0=UNUSED, 1=RESERVED, 2=USED
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '0',
            reserved_order_no = NULL,
            reserved_at = NULL,
            used_order_no = NULL,
            used_at = NULL
        WHERE instance_id = #{instanceId}
        AND (
            (status = '1' AND reserved_order_no = #{orderNo})
            OR (status = '2' AND used_order_no = #{orderNo})
        )
        """)
    int releaseCoupon(@Param("instanceId") Long instanceId, @Param("orderNo") String orderNo);

    /**
     * 重新預定既有 UNUSED 的 instance（released 後重用）
     * status value: 0=UNUSED → 1=RESERVED
     * 用於賣家取消訂單後再次套用同張券（其他路徑亦可重用）
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '1',
            reserved_order_no = #{orderNo},
            reserved_at = NOW()
        WHERE instance_id = #{instanceId}
        AND status = '0'
        """)
    int reReserveCoupon(@Param("instanceId") Long instanceId, @Param("orderNo") String orderNo);

    /**
     * 批量收回：將指定模板下的 UNUSED/RESERVED Instance 設為 REVOKED
     * status value: 0=UNUSED, 1=RESERVED, 4=REVOKED
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '4', revoked_at = NOW(), revoked_reason = #{reason}
        WHERE template_id = #{templateId}
        AND status IN ('0', '1')
        """)
    int revokeByTemplate(@Param("templateId") Long templateId, @Param("reason") String reason);

    /**
     * Admin 作廢單一 Instance
     * status value: 0=UNUSED, 5=ANNUL
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '5', voided_at = NOW(), voided_reason = #{reason}
        WHERE instance_id = #{instanceId}
        AND status = '0'
        """)
    int voidInstance(@Param("instanceId") Long instanceId, @Param("reason") String reason);

    /**
     * 買家刪除：批量作廢該 owner 全部 UNUSED Instance（07 §5.19 → W2/W3 設計書 §3.2 D6 遷移，2026-07-16）
     * 觸發源：BuyerDeletedConsumer 消費 buyer.deleted MQ（取代原 account.deleted 綁定）
     * 篩選範圍：僅 status='0' UNUSED；RESERVED / USED / EXPIRED / REVOKED / ANNUL 不動
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '5', voided_at = NOW(), voided_reason = #{reason}
        WHERE owner_buyer_id = #{ownerBuyerId}
        AND status = '0'
        """)
    int voidAllUnusedByOwner(@Param("ownerBuyerId") Long ownerBuyerId,
                             @Param("reason") String reason);

    /**
     * 對帳：撈出 RESERVED 狀態且 reserved_at 早於指定時間的 instance，
     * 由對帳 Job 逐筆查詢訂單真實狀態後決定動作。
     * 排除「剛 reserve 還在等付款」的正常案例，避免無謂查詢。
     */
    @Select("""
        SELECT * FROM coupon_instances
        WHERE status = '1'
        AND reserved_at < #{olderThan}
        ORDER BY reserved_at ASC
        LIMIT #{limit}
        """)
    java.util.List<CouponInstanceEntity> findStaleReserved(
            @Param("olderThan") java.time.LocalDateTime olderThan,
            @Param("limit") int limit);

    /**
     * 自動過期：將 EXPIRED Template 下的 UNUSED Instance 標記為 EXPIRED
     * status value: 0=UNUSED, 3=EXPIRED
     */
    @Update("""
        UPDATE coupon_instances
        SET status = '3'
        WHERE status = '0'
        AND template_id IN (
            SELECT template_id FROM coupon_templates WHERE status = '3'
        )
        """)
    int expireInstancesByTemplate();

    /**
     * 查詢買家持有某模板的有效 Instance 數量
     * status value: 5=ANNUL, 4=REVOKED
     */
    @Select("""
        SELECT COUNT(*) FROM coupon_instances
        WHERE template_id = #{templateId}
        AND owner_buyer_id = #{ownerBuyerId}
        AND status NOT IN ('5', '4')
        """)
    int countActiveByTemplateAndOwner(@Param("templateId") Long templateId,
                                     @Param("ownerBuyerId") Long ownerBuyerId);

    /**
     * 檢查是否已為 USED 且 order_no 匹配（冪等判斷用）
     * status value: 2=USED
     */
    @Select("""
        SELECT COUNT(*) FROM coupon_instances
        WHERE instance_id = #{instanceId}
        AND status = '2'
        AND used_order_no = #{orderNo}
        """)
    int countUsedWithOrderNo(@Param("instanceId") Long instanceId, @Param("orderNo") String orderNo);

    /**
     * 檢查是否已為 UNUSED（Release 冪等判斷用）
     * status value: 0=UNUSED
     */
    @Select("""
        SELECT COUNT(*) FROM coupon_instances
        WHERE instance_id = #{instanceId}
        AND status = '0'
        """)
    int countUnused(@Param("instanceId") Long instanceId);

    /**
     * 統計模板下各狀態的 Instance 數量
     */
    @Select("""
        SELECT COUNT(*) FROM coupon_instances
        WHERE template_id = #{templateId}
        AND status = #{status}
        """)
    int countByTemplateAndStatus(@Param("templateId") Long templateId, @Param("status") String status);

    /**
     * issue #1176 PR-4：掃描 UNUSED 且所屬 template 即將到期（(now, until] 窗口）的 instance，
     * 游標分頁（instance_id > cursor）。回 record projection（非 Entity，遵 docs/開發規範/Java 微服務開發規範.md §9.2）。
     */
    @Select("""
        SELECT ci.instance_id AS instanceId,
               ci.owner_buyer_id AS ownerBuyerId,
               ct.title AS couponName,
               ct.end_at AS endAt
        FROM coupon_instances ci
        JOIN coupon_templates ct ON ct.template_id = ci.template_id
        WHERE ci.status = '0'
          AND ct.end_at > #{now}
          AND ct.end_at <= #{until}
          AND ci.instance_id > #{cursor}
        ORDER BY ci.instance_id
        LIMIT #{limit}
        """)
    List<CouponExpiringRef> scanExpiringUnused(@Param("now") LocalDateTime now,
                                               @Param("until") LocalDateTime until,
                                               @Param("cursor") long cursor,
                                               @Param("limit") int limit);
}

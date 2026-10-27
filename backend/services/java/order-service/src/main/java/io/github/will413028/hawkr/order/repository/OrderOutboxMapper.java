package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.order.entity.OrderOutboxEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 訂單事件發件匣 Mapper（03 §8.10）。
 */
@Mapper
public interface OrderOutboxMapper extends BaseMapper<OrderOutboxEntity> {

    /**
     * 撈待派送事件（PENDING / FAILED 且已到重試時間）。
     *
     * <p><b>issue #2294 H1-DEAD</b>：述詞原為 {@code status <> '8222'}（SENT），
     * 而 {@code '8224'}（DEAD）對 {@code <> '8222'} <b>恆真</b>、{@code markDead} 又把
     * {@code next_retry_at} 設 NULL（第二個條件對 NULL 也恆真）⇒
     * <b>DEAD 列每 5 秒被撈回一次，永遠</b>。配上 {@code ORDER BY id ASC LIMIT 50}
     * （DEAD 列失敗最早、id 最小、永遠排最前）與「該表沒有任何清理機制」，
     * 累積 50 筆 DEAD 即造成 {@code order.paid} / {@code order.cancelled}
     * <b>整條 outbox 永久 starvation</b>——且<b>單 pod 也會壞</b>。
     *
     * <p>改為<b>正面枚舉</b>要處理的狀態（《多副本排程搶佔規範》§六）：新增終態時不會有人
     * 回頭改 {@code <>} / {@code NOT IN} 述詞，正面枚舉才不會漏。
     * payment / payuni-webhook 兩支 outbox 本來就是這個寫法，本次對齊。
     */
    default List<OrderOutboxEntity> findDispatchable(LocalDateTime now, int limit) {
        return findDispatchableByStatus(now, limit,
                OutboxEventStatus.PENDING.value(), OutboxEventStatus.FAILED.value());
    }

    @Select("""
            SELECT * FROM order_outbox
            WHERE status IN (#{pending}, #{failed})
              AND (next_retry_at IS NULL OR next_retry_at <= #{now})
            ORDER BY id ASC
            LIMIT #{limit}
            """)
    List<OrderOutboxEntity> findDispatchableByStatus(@Param("now") LocalDateTime now,
                                                     @Param("limit") int limit,
                                                     @Param("pending") String pending,
                                                     @Param("failed") String failed);

    // ==================== issue #2294 H1：多副本排程搶佔（lease on next_retry_at） ====================

    /**
     * 搶佔一筆待派送事件（<b>lease</b>，不是永久旗標）。
     *
     * <p>《多副本排程搶佔規範》§二 六步套用結果：claim 把 {@code next_retry_at} 推到
     * {@code now + lease}（一個<b>會過去的未來時間</b>）⇒ 該列暫時離開候選集合，租約到期自動回來。
     * pod 在 claim 與 publish 之間崩潰 ⇒ 最多延後一個租約重送，<b>at-least-once 契約不變</b>。
     *
     * <p>閘門放在 {@code OrderOutboxPublisherScheduler}——全 repo 對
     * {@code OrderOutboxPublisher.publish} 的<b>唯一</b>呼叫點就在那裡
     * （對照 payment 有 afterCommit 第二條進入點，必須把閘門放進 publisher）。
     *
     * <p>租約長度推導見 {@code OrderOutboxPublisherScheduler.CLAIM_LEASE_SEC}。
     *
     * @return 1 = 搶到；0 = 已被另一個 pod 搶走或狀態已變（呼叫端必須 skip，一個副作用都不做）
     */
    default int claimForDispatch(Long id, LocalDateTime leaseUntil, LocalDateTime now) {
        return claimForDispatchByStatus(id, leaseUntil, now,
                OutboxEventStatus.PENDING.value(), OutboxEventStatus.FAILED.value());
    }

    @Update("""
            UPDATE order_outbox
            SET next_retry_at = #{leaseUntil}
            WHERE id = #{id}
              AND status IN (#{pending}, #{failed})
              AND (next_retry_at IS NULL OR next_retry_at <= #{now})
            """)
    int claimForDispatchByStatus(@Param("id") Long id,
                                 @Param("leaseUntil") LocalDateTime leaseUntil,
                                 @Param("now") LocalDateTime now,
                                 @Param("pending") String pending,
                                 @Param("failed") String failed);

    /**
     * 標記已派送（具名 UPDATE，取代 {@code updateById} 全欄覆寫）。
     *
     * <p>守衛 {@code status <> SENT}：租約到期後原持有者若仍活著，可能與新持有者同時處理同一筆
     * （規範 §五 第 7 點「租約不是鎖」）。
     */
    default int markSent(Long id, LocalDateTime sentAt) {
        return markSentByStatus(id, sentAt, OutboxEventStatus.SENT.value());
    }

    @Update("""
            UPDATE order_outbox
            SET status = #{sent},
                sent_at = #{sentAt}
            WHERE id = #{id}
              AND status <> #{sent}
            """)
    int markSentByStatus(@Param("id") Long id,
                         @Param("sentAt") LocalDateTime sentAt,
                         @Param("sent") String sent);

    /**
     * 標記派送失敗 + 指數退避（具名 UPDATE，守衛 {@code status <> SENT}）。
     *
     * <p>守衛的用途：避免「另一個 pod 已送達並標 SENT」被本 pod 的遲到失敗回寫成 FAILED
     * ⇒ 事件被重複投遞一輪。
     */
    default int markFailedGuarded(Long id, int retryCount, LocalDateTime nextRetryAt) {
        return markFailedGuardedByStatus(id, retryCount, nextRetryAt,
                OutboxEventStatus.FAILED.value(), OutboxEventStatus.SENT.value());
    }

    @Update("""
            UPDATE order_outbox
            SET status = #{failed},
                retry_count = #{retryCount},
                next_retry_at = #{nextRetryAt}
            WHERE id = #{id}
              AND status <> #{sent}
            """)
    int markFailedGuardedByStatus(@Param("id") Long id,
                                  @Param("retryCount") int retryCount,
                                  @Param("nextRetryAt") LocalDateTime nextRetryAt,
                                  @Param("failed") String failed,
                                  @Param("sent") String sent);

    /**
     * 標記 DEAD（超過重試上限，具名 UPDATE，守衛 {@code status <> SENT}）。
     *
     * <p>配合上面的正面枚舉候選述詞，DEAD 列此後<b>永久退出候選集合</b>（H1-DEAD 的修復點）。
     */
    default int markDeadGuarded(Long id) {
        return markDeadGuardedByStatus(id,
                OutboxEventStatus.DEAD.value(), OutboxEventStatus.SENT.value());
    }

    @Update("""
            UPDATE order_outbox
            SET status = #{dead},
                next_retry_at = NULL
            WHERE id = #{id}
              AND status <> #{sent}
            """)
    int markDeadGuardedByStatus(@Param("id") Long id,
                                @Param("dead") String dead,
                                @Param("sent") String sent);
}

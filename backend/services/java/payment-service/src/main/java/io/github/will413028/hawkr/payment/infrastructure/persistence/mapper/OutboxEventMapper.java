package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * §2A.6 Outbox event mapper（payment-service）。
 */
@Mapper
public interface OutboxEventMapper extends BaseMapper<OutboxEventEntity> {

    /**
     * 查詢待投遞的 Outbox 事件（PENDING 或 FAILED 且已到重試時間）。
     */
    @ResultMap("mybatis-plus_OutboxEventEntity")
    @Select("""
            SELECT * FROM outbox_events
            WHERE status IN ('8221', '8223')
              AND (next_retry_at IS NULL OR next_retry_at <= NOW())
            ORDER BY created_at ASC
            LIMIT #{limit}
            """)
    List<OutboxEventEntity> findPendingEvents(int limit);

    // ==================== issue #2294 H2：多副本排程搶佔（lease on next_retry_at） ====================

    /**
     * 搶佔一筆待派送事件（<b>lease</b>，不是永久旗標）。
     *
     * <p>《多副本排程搶佔規範》§二 六步套用結果：claim 把 {@code next_retry_at} 推到
     * {@code now + lease}（一個<b>會過去的未來時間</b>）⇒ 該列暫時離開候選集合，租約到期自動回來。
     * pod 在 claim 與 publish 之間崩潰 ⇒ 最多延後一個租約重送，<b>at-least-once 契約不變</b>。
     *
     * <p>⚠️ payment 的 outbox 有<b>兩條進入點</b>——{@code OutboxEventPublisherScheduler} 每 5s 兜底
     * ＋ {@code PaymentOutboxWriter} 的 afterCommit 即時投遞——所以閘門放在
     * {@code PaymentOutboxPublisher.publish()} 的第一行（兩條路徑的共同收斂點），
     * 不放在 Scheduler（放 Scheduler 會漏掉 afterCommit 那條）。
     *
     * <p>租約長度推導見 {@code PaymentOutboxPublisher.CLAIM_LEASE_SEC}。
     *
     * @return 1 = 搶到；0 = 已被另一個 pod 搶走或狀態已變（呼叫端必須 skip，一個副作用都不做）
     */
    default int claimForDispatch(Long id, LocalDateTime leaseUntil, LocalDateTime now) {
        return claimForDispatchByStatus(id, leaseUntil, now,
                OutboxEventStatus.PENDING.value(), OutboxEventStatus.FAILED.value());
    }

    @Update("""
            UPDATE outbox_events
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
     * 標記已確認投遞（具名 UPDATE，取代 {@code updateById} 全欄覆寫）。
     *
     * <p>守衛 {@code status <> SENT}：租約到期後原持有者若仍活著，可能與新持有者同時處理同一筆
     * （規範 §五 第 7 點「租約不是鎖」）。
     */
    default int markSent(Long id, LocalDateTime sentAt) {
        return markSentByStatus(id, sentAt, OutboxEventStatus.SENT.value());
    }

    @Update("""
            UPDATE outbox_events
            SET status = #{sent},
                sent_at = #{sentAt},
                last_error = NULL
            WHERE id = #{id}
              AND status <> #{sent}
            """)
    int markSentByStatus(@Param("id") Long id,
                         @Param("sentAt") LocalDateTime sentAt,
                         @Param("sent") String sent);

    /**
     * 標記投遞失敗 + 指數退避（具名 UPDATE，守衛 {@code status <> SENT}）。
     *
     * <p>守衛的用途：避免「另一個 pod 已送達並標 SENT」被本 pod 的遲到失敗回寫成 FAILED
     * ⇒ 事件被重複投遞一輪。
     */
    default int markFailedGuarded(Long id, int retryCount, String lastError, LocalDateTime nextRetryAt) {
        return markFailedGuardedByStatus(id, retryCount, lastError, nextRetryAt,
                OutboxEventStatus.FAILED.value(), OutboxEventStatus.SENT.value());
    }

    @Update("""
            UPDATE outbox_events
            SET status = #{failed},
                retry_count = #{retryCount},
                last_error = #{lastError},
                next_retry_at = #{nextRetryAt}
            WHERE id = #{id}
              AND status <> #{sent}
            """)
    int markFailedGuardedByStatus(@Param("id") Long id,
                                  @Param("retryCount") int retryCount,
                                  @Param("lastError") String lastError,
                                  @Param("nextRetryAt") LocalDateTime nextRetryAt,
                                  @Param("failed") String failed,
                                  @Param("sent") String sent);

    /**
     * 標記 DEAD（超過重試上限，具名 UPDATE，守衛 {@code status <> SENT}）。
     *
     * <p>候選述詞是<b>正面枚舉</b> {@code status IN (PENDING, FAILED)}，DEAD 本來就不在其中
     * （規範 §六）。
     */
    default int markDeadGuarded(Long id, String lastError) {
        return markDeadGuardedByStatus(id, lastError,
                OutboxEventStatus.DEAD.value(), OutboxEventStatus.SENT.value());
    }

    @Update("""
            UPDATE outbox_events
            SET status = #{dead},
                last_error = #{lastError},
                next_retry_at = NULL
            WHERE id = #{id}
              AND status <> #{sent}
            """)
    int markDeadGuardedByStatus(@Param("id") Long id,
                                @Param("lastError") String lastError,
                                @Param("dead") String dead,
                                @Param("sent") String sent);

    /**
     * 清理已成功投遞且超過保留期的列（{@code payment.cleanup.outbox-sent-retention-days}）。
     *
     * <p><b>只刪 SENT</b>，述詞<b>正面列舉</b>狀態碼：
     * <ul>
     *   <li>🔴 不用 {@code status <> 8224} 這類否定述詞——日後新增一個狀態值，
     *       否定述詞會把它一起刪掉，而且完全無聲。</li>
     *   <li>🔴 <b>DEAD（8224）一列都不刪</b>：它是 permanent delivery failure 的證據，
     *       也是 {@code payment_outbox_dead_rows} 告警的來源。自動刪掉 ＝
     *       「事故沒解決 → 保留期到 → 列消失 → 告警自己恢復綠」。</li>
     *   <li>PENDING（8221）／FAILED（8223）仍在候選集合內，刪它們等於讓事件蒸發。</li>
     * </ul>
     *
     * <p><b>用 {@code sent_at} 而不是 {@code created_at}</b>：retention 的起點是「真正送出去」
     * 那一刻（與 {@code PaymentWebhookMapper.deleteProcessedBefore} 用 {@code processed_at} 同一條慣例）。
     * <br>⚠️ {@code sent_at} 為 NULL 的列<b>不會</b>被這句刪到（NULL 比較不成立）——
     * 方向是 fail-safe 的：拿不準的列留著，不會誤刪。
     * 本輪實查：dev 155 列 SENT 的 {@code sent_at} 全部非 NULL；code 面唯一寫入 SENT 的
     * {@code markSentByStatus} 同一句必設 {@code sent_at}。
     *
     * @param sentBefore 保留期界線，嚴格小於（{@code <}）——與同服務既有三支 cleanup 一致
     * @return 實際刪除筆數
     */
    default int deleteSentBefore(LocalDateTime sentBefore) {
        return deleteSentBeforeByStatus(sentBefore, OutboxEventStatus.SENT.value());
    }

    @Delete("""
            DELETE FROM outbox_events
            WHERE status = #{sent}
              AND sent_at < #{sentBefore}
            """)
    int deleteSentBeforeByStatus(@Param("sentBefore") LocalDateTime sentBefore,
                                 @Param("sent") String sent);
}

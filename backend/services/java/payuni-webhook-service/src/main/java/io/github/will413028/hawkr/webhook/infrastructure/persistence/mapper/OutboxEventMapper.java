package io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.OutboxEvent;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

@Mapper
public interface OutboxEventMapper extends BaseMapper<OutboxEvent> {

    /**
     * 查詢待投遞的 Outbox 事件（PENDING 或 FAILED 且已到重試時間）
     */
    @ResultMap("mybatis-plus_OutboxEvent")
    @Select("""
            SELECT * FROM outbox_events
            WHERE status IN ('8221', '8223')  -- OutboxEventStatus PENDING / FAILED
              AND (next_retry_at IS NULL OR next_retry_at <= NOW())
            ORDER BY created_at ASC
            LIMIT #{limit}
            """)
    List<OutboxEvent> findPendingEvents(int limit);

    /**
     * Replay-safe INSERT：透過 ON CONFLICT DO NOTHING 避開 §2.5.1 PostgreSQL aborted state 限制。
     * <p>
     * 設計書 §4.5.3：T2 重放時若 outbox 已存在（前次 T2 commit 後在某處中斷），
     * 命中 UNIQUE (aggregate_id, exchange, topic) 視為已建立、跳過該筆。
     * <p>
     * 使用 ON CONFLICT 而非 try/catch DuplicateKeyException 的理由：
     * PostgreSQL 在 transaction 內任何錯誤都會讓 connection 進入 aborted state，
     * 後續 SQL 全部失敗（含同 T2 內的下一筆 outbox INSERT 與 inbox UPDATE）。
     *
     * @return 1 = 新建一筆；0 = 已存在跳過
     */
    @Insert("""
            INSERT INTO outbox_events
                (aggregate_type, aggregate_id, exchange, topic, message_key,
                 payload, status, retry_count, created_at)
            VALUES
                (#{aggregateType}, #{aggregateId}, #{exchange}, #{topic}, #{messageKey},
                 CAST(#{payload} AS jsonb), #{status}, #{retryCount}, #{createdAt})
            ON CONFLICT ON CONSTRAINT uq_outbox_aggregate_route DO NOTHING
            """)
    int insertIfAbsent(@Param("aggregateType") String aggregateType,
                       @Param("aggregateId") String aggregateId,
                       @Param("exchange") String exchange,
                       @Param("topic") String topic,
                       @Param("messageKey") String messageKey,
                       @Param("payload") String payload,
                       @Param("status") String status,
                       @Param("retryCount") Integer retryCount,
                       @Param("createdAt") java.time.LocalDateTime createdAt);

    // ==================== issue #2294 H3：多副本排程搶佔（lease on next_retry_at） ====================

    /**
     * 搶佔一筆待派送事件（<b>lease</b>，不是永久旗標）。
     *
     * <p>《多副本排程搶佔規範》§二 六步套用結果：claim 把 {@code next_retry_at} 推到
     * {@code now + lease}（一個<b>會過去的未來時間</b>）⇒ 該列暫時離開候選集合，租約到期自動回來。
     * pod 在 claim 與 publish 之間崩潰 ⇒ 最多延後一個租約重送，<b>at-least-once 契約不變</b>。
     *
     * <p>租約長度推導見 {@code OutboxPublisherScheduler.CLAIM_LEASE_SEC}。
     *
     * <p>述詞刻意<b>與候選 SQL 完全一致</b>（{@code status IN (PENDING, FAILED)}
     * ＋ {@code next_retry_at} 到期），另加 {@code id} 定位 ⇒ 非純 PK 述詞（規範 §一 ⑦）。
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
     * 標記已送達（具名 UPDATE，取代 {@code updateById} 全欄覆寫）。
     *
     * <p>守衛 {@code status <> SENT}：租約到期後原持有者若仍活著，可能與新持有者同時處理同一筆，
     * 此守衛讓「已送達」不會被第二次寫入覆蓋（規範 §五 第 7 點「租約不是鎖」）。
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
     * 標記派送失敗 + 指數退避（具名 UPDATE，守衛 {@code status <> SENT}）。
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
     * <p>DEAD 之後 {@code next_retry_at} 設 NULL：候選述詞是<b>正面枚舉</b>
     * {@code status IN (PENDING, FAILED)}，DEAD 本來就不在其中（規範 §六）。
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
}

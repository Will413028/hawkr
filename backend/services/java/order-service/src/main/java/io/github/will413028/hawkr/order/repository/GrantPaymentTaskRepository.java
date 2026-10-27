package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 撥款指示任務表 Mapper（grant_payment_tasks）。
 *
 * <p>狀態欄位 {@code status} 一律存 {@link io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus}
 * value（"0"/"10"/"20"/"30"/"40"/"50"），對齊 03 §3.11 / §2.1 INV-3 規格。
 * <p>所有 status 值透過 default method 由 {@code WorkerTaskStatus.*.value()} 注入，
 * SQL 內不寫字面狀態碼（docs/開發規範/Java 微服務開發規範.md §6.1 禁止硬編碼；CodeMeta 為唯一真實來源）。
 */
public interface GrantPaymentTaskRepository extends BaseMapper<GrantPaymentTaskEntity> {

    /**
     * 掃描待執行的撥款任務（PENDING 或 FAILED 且 next_retry_time 已到）。
     */
    /**
     * 依訂單編號取撥款任務（{@code uk_grant_payment_order} 保證至多一筆）。
     *
     * <p>用途：N16 consumer 在 {@code 35 → 54} CAS 回 {@code affected = 0} 時，要分辨
     * 「正常重送」與「事件與既有終態不一致」（03 §5.7.3）。
     * 🔴 <b>訂單已是 {@code 54} 但查不到這一列</b>，代表狀態轉換與撥款任務曾經沒有原子
     * ——那正是這條路徑用單一交易在防的事，必須讓它現形而不是靜默 ACK。
     *
     * @return {@code null} ＝ 該訂單沒有撥款任務
     */
    @Select("SELECT * FROM grant_payment_tasks WHERE order_no = #{orderNo}")
    GrantPaymentTaskEntity findByOrderNo(@Param("orderNo") String orderNo);

    default List<GrantPaymentTaskEntity> findPendingTasks(int limit) {
        return findPendingTasksByStatus(limit,
                WorkerTaskStatus.PENDING.value(), WorkerTaskStatus.FAILED.value(),
                WorkerTaskStatus.PROCESSING.value());
    }

    /**
     * 候選述詞（issue #2294 C4）。
     *
     * <p>⚠️ {@code PROCESSING} <b>必須</b>在允許集合內：claim 改成 lease 之後，
     * 「被搶佔中」的列 status 仍是 {@code PROCESSING}，若候選述詞不收它，租約到期該列
     * <b>永遠回不來</b> —— 那只是把永久旗標寫得更像 lease 而已
     * （《多副本排程搶佔規範》§二 正解段的警告）。
     * 真正把它擋在候選之外的是 {@code next_retry_time}（租約未到期）那一條。
     *
     * <p>排序以 {@code task_id} 收尾：同一交易內批次建立的任務 {@code created_at} 取 NOW()＝交易
     * 開始時刻、完全相同，只靠它排序時「最舊先做」在 LIMIT 邊界上不成立（同時間戳的哪幾筆進得了
     * 這一批不決定性）。搶佔本身是逐列 CAS（{@link #claimTask}），順序只決定先試哪一列，
     * 補上收尾鍵不改變搶佔語意。
     */
    @Select("""
        SELECT * FROM grant_payment_tasks
        WHERE status IN (#{pending}, #{failed}, #{processing})
          AND (next_retry_time IS NULL OR next_retry_time <= NOW())
        ORDER BY created_at ASC, task_id ASC
        LIMIT #{limit}
        """)
    List<GrantPaymentTaskEntity> findPendingTasksByStatus(@Param("limit") int limit,
                                                          @Param("pending") String pending,
                                                          @Param("failed") String failed,
                                                          @Param("processing") String processing);

    /**
     * CAS 搶佔任務（issue #2294 C4：<b>lease on {@code next_retry_time}</b>，不是永久旗標）。
     *
     * <p><b>C4 是五支裡最嚴重的一支</b>：舊版 {@code SET status = PROCESSING} 是永久旗標，
     * 而 admin 的人工救濟 {@link #resetForRetryByStatus} 原本只收
     * {@code FAILED('30')} / {@code MANUAL_INTERVENTION('40')}
     * ⇒ 卡在 {@code PROCESSING} 的撥款任務<b>連人工重發都做不到</b>
     * （《多副本排程搶佔規範》§二 第 6 步要擋的正是這種「以為有救濟、實際收不到」）。
     * order-service prod {@code replicas=2}，這一支今天就在賭。
     *
     * <p><b>正解</b>：改「推時間」——{@code next_retry_time} 推到 {@code now + lease}。
     * {@code status = PROCESSING} 保留但降為可觀測資訊；候選述詞
     * （{@link #findPendingTasksByStatus}）與救濟路徑（{@link #resetForRetry}）同步涵蓋
     * {@code PROCESSING}。<b>零 migration</b>。
     *
     * <p><b>租約長度檢查</b>（§三）：候選述詞無時間上界 ⇒ 窗口寬度 ∞ ⇒ 上界無條件成立；
     * 下界見 {@code GrantPaymentWorker.CLAIM_LEASE_SEC} 的推導。
     *
     * @return 1 = 搶到（呼叫端才可以打 gRPC）；0 = 已被另一個 pod 認領（租約未到期）
     */
    default int claimTask(long taskId, LocalDateTime leaseUntil) {
        return claimTaskByStatus(taskId, leaseUntil, WorkerTaskStatus.PROCESSING.value(),
                WorkerTaskStatus.PENDING.value(), WorkerTaskStatus.FAILED.value());
    }

    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{processing},
            next_retry_time = #{leaseUntil},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status IN (#{pending}, #{failed}, #{processing})
          AND (next_retry_time IS NULL OR next_retry_time <= NOW())
        """)
    int claimTaskByStatus(@Param("taskId") long taskId, @Param("leaseUntil") LocalDateTime leaseUntil,
                          @Param("processing") String processing,
                          @Param("pending") String pending, @Param("failed") String failed);

    /**
     * 撥款資格未成立，推遲到 {@code nextEligibleAt} 再問（backend#3262）。
     *
     * <p>PROCESSING → PENDING ＋ 推 {@code next_retry_time}。狀態回 PENDING 而不是 FAILED，
     * 因為<b>這不是失敗</b>：任務還沒被執行過，只是還不到能執行的時候。
     * 候選述詞（{@link #findPendingTasksByStatus}）本來就有
     * {@code next_retry_time IS NULL OR next_retry_time <= NOW()}，
     * 推時間即等同排程 —— <b>零 migration，沿用既有欄位語意</b>。
     *
     * <p><b>刻意不動 {@code retry_count}</b>：hold 是業務資格未到，不該消耗重試額度。
     * 動它的話，一筆申訴審理數天的訂單會在重問幾次後被推進 MANUAL_INTERVENTION，
     * 把「還不能撥」誤報成「撥款失敗」。
     *
     * <p><b>CAS {@code AND status = PROCESSING}</b>：只有持有租約的那個 pod 能推遲，
     * 遲到的第二個 pod affected=0 走冪等 skip；也防覆蓋 admin 剛剛的
     * {@link #resetForRetry}（那支把任務打回 PENDING）。
     *
     * <p><b>admin 重發繞不過本閘門</b>：{@link #resetForRetryByStatus} 只收
     * FAILED / MANUAL_INTERVENTION / PROCESSING，收不到被 hold 住的 PENDING；
     * 即使 admin 對 FAILED 任務重發，worker 下一輪仍會先問一次 hold 才撥款
     * ——閘門在執行點而非建立點，就是為了讓任何「把任務叫醒」的路徑都必須重新過閘。
     *
     * @param nextEligibleAt 下次可執行時點（有時間界 hold 用 {@code hold_until}；無時間界用 now + recheck）
     * @return 1 = 已推遲；0 = 租約已易主 / 任務已被他路徑改變，caller 冪等 skip
     */
    default int deferForPayoutHold(long taskId, LocalDateTime nextEligibleAt, String reason) {
        return deferForPayoutHoldByStatus(taskId, nextEligibleAt, reason,
                WorkerTaskStatus.PENDING.value(), WorkerTaskStatus.PROCESSING.value());
    }

    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{pending},
            next_retry_time = #{nextEligibleAt},
            last_error = #{reason},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status = #{processing}
        """)
    int deferForPayoutHoldByStatus(@Param("taskId") long taskId,
                                   @Param("nextEligibleAt") LocalDateTime nextEligibleAt,
                                   @Param("reason") String reason,
                                   @Param("pending") String pending,
                                   @Param("processing") String processing);

    /**
     * Admin 重發撥款任務（收斂規範 §8.4 人工介入閉環）。
     * 僅當 task 為 FAILED(30) 或 MANUAL_INTERVENTION(40) 時生效；
     * 重設 retry_count=0、next_retry_time=NOW()，狀態回 PENDING(0)，交由既有 Worker 下一輪處理。
     * 不重設已 DONE(20) 的 task（避免重複撥款）。
     */
    default int resetForRetry(String orderNo) {
        return resetForRetryByStatus(orderNo, WorkerTaskStatus.PENDING.value(),
                WorkerTaskStatus.FAILED.value(), WorkerTaskStatus.MANUAL_INTERVENTION.value(),
                WorkerTaskStatus.PROCESSING.value());
    }

    /**
     * ⚠️ issue #2294 C4：允許集合必須<b>涵蓋 claim 之後的中間狀態 {@code PROCESSING}</b>。
     *
     * <p>《多副本排程搶佔規範》§二 第 6 步：「有人工救濟」不算回頭路，除非救濟路徑<b>收得到</b>
     * 這個狀態——寫「有 ops 可以處理」之前，先讀那支救濟 SQL 的 WHERE。
     *
     * <p><b>把 {@code PROCESSING} 放進來安全嗎？</b>安全，而且是既有設計已經處理過的情形：
     * 重發把 task 打回 {@code PENDING}，而 worker 後續的每一支狀態推進
     * （{@link #markGrantTaskDoneByStatus} 等）都帶 {@code AND status = PROCESSING} 樂觀鎖
     * ⇒ 遲到的完成會 affected=0，走既有的「冪等 skip」log 分支，不會覆蓋 admin 的重發。
     */
    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{pending},
            retry_count = 0,
            next_retry_time = NOW(),
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND status IN (#{failed}, #{manual}, #{processing})
        """)
    int resetForRetryByStatus(@Param("orderNo") String orderNo, @Param("pending") String pending,
                              @Param("failed") String failed, @Param("manual") String manual,
                              @Param("processing") String processing);

    /**
     * 03 §2.1 INV-3：撥款成功（PROCESSING 10 → DONE 20）。
     * 樂觀鎖 WHERE status='10' 防多 Pod 遲到完成覆蓋已被 admin 重發回 PENDING 的 task。
     *
     * @return 1 = 成功；0 = 已被他路徑變更，caller 應記 INFO log + 冪等 skip
     */
    default int markGrantTaskDone(long taskId) {
        return markGrantTaskDoneByStatus(taskId,
                WorkerTaskStatus.DONE.value(), WorkerTaskStatus.PROCESSING.value());
    }

    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{done},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status = #{processing}
        """)
    int markGrantTaskDoneByStatus(@Param("taskId") long taskId,
                                  @Param("done") String done, @Param("processing") String processing);

    /**
     * 03 §2.1 INV-3：業務錯誤直接轉人工介入（PROCESSING 10 → MANUAL_INTERVENTION 40）。
     */
    default int markGrantTaskManualIntervention(long taskId, String lastError) {
        return markGrantTaskManualInterventionByStatus(taskId, lastError,
                WorkerTaskStatus.MANUAL_INTERVENTION.value(), WorkerTaskStatus.PROCESSING.value());
    }

    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{manual},
            last_error = #{lastError},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status = #{processing}
        """)
    int markGrantTaskManualInterventionByStatus(@Param("taskId") long taskId,
                                                @Param("lastError") String lastError,
                                                @Param("manual") String manual,
                                                @Param("processing") String processing);

    /**
     * 03 §2.1 INV-3：重試上限達標轉人工介入（PROCESSING 10 → MANUAL_INTERVENTION 40，含 retry_count）。
     */
    default int markGrantTaskMaxRetryReached(long taskId, int retryCount, String lastError) {
        return markGrantTaskMaxRetryReachedByStatus(taskId, retryCount, lastError,
                WorkerTaskStatus.MANUAL_INTERVENTION.value(), WorkerTaskStatus.PROCESSING.value());
    }

    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{manual},
            retry_count = #{retryCount},
            last_error = #{lastError},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status = #{processing}
        """)
    int markGrantTaskMaxRetryReachedByStatus(@Param("taskId") long taskId,
                                             @Param("retryCount") int retryCount,
                                             @Param("lastError") String lastError,
                                             @Param("manual") String manual,
                                             @Param("processing") String processing);

    /**
     * 03 §2.1 INV-3：排程重試（PROCESSING 10 → FAILED 30，含 retry_count + next_retry_time）。
     */
    default int markGrantTaskRetryScheduled(long taskId, int retryCount,
                                            LocalDateTime nextRetryTime, String lastError) {
        return markGrantTaskRetryScheduledByStatus(taskId, retryCount, nextRetryTime, lastError,
                WorkerTaskStatus.FAILED.value(), WorkerTaskStatus.PROCESSING.value());
    }

    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{failed},
            retry_count = #{retryCount},
            next_retry_time = #{nextRetryTime},
            last_error = #{lastError},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status = #{processing}
        """)
    int markGrantTaskRetryScheduledByStatus(@Param("taskId") long taskId,
                                            @Param("retryCount") int retryCount,
                                            @Param("nextRetryTime") LocalDateTime nextRetryTime,
                                            @Param("lastError") String lastError,
                                            @Param("failed") String failed,
                                            @Param("processing") String processing);

    /**
     * 03 §3.11.2 / §2.1 INV-3：PayUni 主動撥款收斂（PROCESSING 10 → PROVIDER_AUTO 50）。
     * <p>
     * Worker 收到 GRANT 類拒絕（30023/30018/30019 等線索碼）且主判準 orders.refund_total_amount > 0
     * 成立時呼叫：PayUni 已對退款交易接管撥款，不再重試、不進 MANUAL_INTERVENTION，
     * task 收斂為純對帳記錄（終態）。
     */
    default int markGrantTaskProviderAuto(long taskId, String lastError) {
        return markGrantTaskProviderAutoByStatus(taskId, lastError,
                WorkerTaskStatus.PROVIDER_AUTO.value(), WorkerTaskStatus.PROCESSING.value());
    }

    @Update("""
        UPDATE grant_payment_tasks
        SET status = #{providerAuto},
            last_error = #{lastError},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status = #{processing}
        """)
    int markGrantTaskProviderAutoByStatus(@Param("taskId") long taskId,
                                          @Param("lastError") String lastError,
                                          @Param("providerAuto") String providerAuto,
                                          @Param("processing") String processing);

    /**
     * 03 §8.4 TaskCleanupJob：刪除指定時間前的已了結任務（DONE / PROVIDER_AUTO）。
     * PROVIDER_AUTO 與 DONE 同等對待：grant_payment_tasks 是 worker 佇列非帳本，
     * 帳務權威在 payment_transactions / financial_logs 與 PayUni 端。
     * <b>刻意排除 MANUAL_INTERVENTION</b>：撥款無 webhook 自動閉環，人工介入單是 Admin 異常清單
     * 與重發撥款（resetForRetry）的依據，未處理完就被清會讓失敗撥款永久失聯。
     */
    default int deleteCompletedBefore(LocalDateTime before) {
        return deleteCompletedBeforeByStatus(before,
                WorkerTaskStatus.DONE.value(),
                WorkerTaskStatus.PROVIDER_AUTO.value());
    }

    @Delete("""
        DELETE FROM grant_payment_tasks
        WHERE status IN (#{done}, #{providerAuto})
          AND updated_at < #{before}
        """)
    int deleteCompletedBeforeByStatus(
            @Param("before") LocalDateTime before,
            @Param("done") String done,
            @Param("providerAuto") String providerAuto);
}

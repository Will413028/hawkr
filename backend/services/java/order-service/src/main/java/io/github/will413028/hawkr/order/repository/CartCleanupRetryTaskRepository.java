package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.CartCleanupRetryTaskEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 購物車清理重試任務表 Mapper
 */
public interface CartCleanupRetryTaskRepository extends BaseMapper<CartCleanupRetryTaskEntity> {

    /**
     * 查詢待處理/失敗且到達重試時間的任務
     */
    default List<CartCleanupRetryTaskEntity> findRetryable(int limit) {
        return findRetryableByStatus(limit,
                OrderConstants.TASK_STATUS_PENDING,
                OrderConstants.TASK_STATUS_FAILED,
                OrderConstants.TASK_STATUS_PROCESSING);
    }

    /**
     * 候選述詞（issue #2294 C3）。
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
        SELECT * FROM cart_cleanup_retry_tasks
        WHERE status IN (#{statusPending}, #{statusFailed}, #{statusProcessing})
        AND (next_retry_time IS NULL OR next_retry_time <= NOW())
        ORDER BY created_at, task_id
        LIMIT #{limit}
    """)
    List<CartCleanupRetryTaskEntity> findRetryableByStatus(
            @Param("limit") int limit,
            @Param("statusPending") String statusPending,
            @Param("statusFailed") String statusFailed,
            @Param("statusProcessing") String statusProcessing);

    /**
     * Claim 任務（issue #2294 C3：<b>lease on {@code next_retry_time}</b>，不是永久旗標）。
     *
     * <p><b>改了什麼</b>：舊版是 {@code SET status = PROCESSING}
     * ⇒ 依《多副本排程搶佔規範》§二 通則 0 第 3–4 步，那是<b>永久旗標</b>：
     * {@code PROCESSING} 不在候選集合、也沒有任何時間欄能讓它回來，
     * <b>pod 被驅逐／OOM／節點失聯時沒有任何 catch 會跑</b>（第 5 步）⇒ 該任務永久卡死。
     *
     * <p><b>正解</b>：改「推時間」——{@code next_retry_time} 推到 {@code now + lease}。
     * {@code status = PROCESSING} <b>保留</b>但角色從「閘門」降為「可觀測資訊」
     * （讓運維在 DB 上讀得出「這筆正在被處理」）；候選述詞同步涵蓋 {@code PROCESSING}
     * （見 {@link #findRetryableByStatus}）。<b>零 migration</b>：欄位與索引皆既有。
     *
     * <p><b>租約長度檢查</b>（§三）：候選述詞只有下界（{@code next_retry_time <= NOW()}）
     * 沒有上界 ⇒ 窗口寬度 ∞ ⇒ 上界無條件成立；下界見
     * {@code CartCleanupRetryWorker.CLAIM_LEASE_SEC} 的推導。
     *
     * @return 1 = 搶到（呼叫端才可以往下做副作用）；0 = 已被另一個 pod 認領（租約未到期）
     */
    default int claimTask(long taskId, LocalDateTime leaseUntil) {
        return claimTaskByStatus(taskId, leaseUntil,
                OrderConstants.TASK_STATUS_PROCESSING,
                OrderConstants.TASK_STATUS_PENDING,
                OrderConstants.TASK_STATUS_FAILED);
    }

    @Update("""
        UPDATE cart_cleanup_retry_tasks
        SET status = #{statusProcessing},
            next_retry_time = #{leaseUntil},
            updated_at = NOW()
        WHERE task_id = #{taskId}
          AND status IN (#{statusPending}, #{statusFailed}, #{statusProcessing})
          AND (next_retry_time IS NULL OR next_retry_time <= NOW())
    """)
    int claimTaskByStatus(
            @Param("taskId") long taskId,
            @Param("leaseUntil") LocalDateTime leaseUntil,
            @Param("statusProcessing") String statusProcessing,
            @Param("statusPending") String statusPending,
            @Param("statusFailed") String statusFailed);

    @Update("""
        UPDATE cart_cleanup_retry_tasks
        SET status = #{status}, last_error = #{lastError},
            retry_count = retry_count + 1,
            next_retry_time = NOW() + (INTERVAL '1 second' * #{retryDelaySeconds}),
            updated_at = NOW()
        WHERE task_id = #{taskId}
    """)
    int updateFailed(
            @Param("taskId") long taskId,
            @Param("status") String status,
            @Param("lastError") String lastError,
            @Param("retryDelaySeconds") int retryDelaySeconds
    );

    /**
     * 標記任務完成
     */
    default int markDone(long taskId) {
        return markDoneByStatus(taskId, OrderConstants.TASK_STATUS_DONE);
    }

    @Update("UPDATE cart_cleanup_retry_tasks SET status = #{status}, updated_at = NOW() WHERE task_id = #{taskId}")
    int markDoneByStatus(@Param("taskId") long taskId, @Param("status") String status);

    /**
     * 標記任務需人工介入
     */
    default int markManualIntervention(long taskId) {
        return markManualInterventionByStatus(taskId, OrderConstants.TASK_STATUS_MANUAL_INTERVENTION);
    }

    @Update("UPDATE cart_cleanup_retry_tasks SET status = #{status}, updated_at = NOW() WHERE task_id = #{taskId}")
    int markManualInterventionByStatus(@Param("taskId") long taskId, @Param("status") String status);

    /**
     * 刪除指定時間前已完成/人工介入的任務
     */
    default int deleteCompletedBefore(LocalDateTime before) {
        return deleteCompletedBeforeByStatus(before,
                OrderConstants.TASK_STATUS_DONE,
                OrderConstants.TASK_STATUS_MANUAL_INTERVENTION);
    }

    @Delete("DELETE FROM cart_cleanup_retry_tasks WHERE status IN (#{statusDone}, #{statusIntervention}) AND updated_at < #{before}")
    int deleteCompletedBeforeByStatus(
            @Param("before") LocalDateTime before,
            @Param("statusDone") String statusDone,
            @Param("statusIntervention") String statusIntervention);
}

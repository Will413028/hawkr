package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.StockTxPhase;
import io.github.will413028.hawkr.common.utils.code.StockTxStatus;
import io.github.will413028.hawkr.order.entity.StockTxRecordEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Param;

import org.apache.ibatis.annotations.ResultMap;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 庫存交易協調表 Mapper
 *
 * 複合主鍵：(stock_tx_id, phase)
 */
public interface StockTxRecordRepository extends BaseMapper<StockTxRecordEntity> {

    @ResultMap("mybatis-plus_StockTxRecordEntity")
    @Select("SELECT * FROM stock_tx_records WHERE stock_tx_id = #{stockTxId} AND phase = #{phase}")
    StockTxRecordEntity findByStockTxIdAndPhase(@Param("stockTxId") String stockTxId,
                                                 @Param("phase") String phase);

    /**
     * 依訂單號查詢 DEDUCT 階段記錄
     */
    default StockTxRecordEntity findDeductByOrderNo(String orderNo) {
        return findByOrderNoAndPhase(orderNo, StockTxPhase.DEDUCT.value());
    }

    @ResultMap("mybatis-plus_StockTxRecordEntity")
    @Select("SELECT * FROM stock_tx_records WHERE order_no = #{orderNo} AND phase = #{phase}")
    StockTxRecordEntity findByOrderNoAndPhase(@Param("orderNo") String orderNo,
                                               @Param("phase") String phase);

    /**
     * 查詢需要補償的 INIT/FAILED 記錄（背景 Worker 用）
     */
    default List<StockTxRecordEntity> findPendingCompensation(int limit) {
        return findPendingCompensationByStatus(limit,
                StockTxPhase.DEDUCT.value(),
                StockTxStatus.INIT.value(),
                StockTxStatus.FAILED.value());
    }

    /**
     * 候選述詞（搶佔見 {@link #claimForCompensation}）。
     *
     * <p>排序以 {@code stock_tx_id} 收尾：{@code created_at} 取 NOW()＝交易開始時刻，同一次建單
     * 寫入的多列時間戳完全相同，只靠它排序時「最舊先做」在 LIMIT 邊界上不成立。述詞已把
     * {@code phase} 釘成單一值，該欄在結果集內為常數，故複合主鍵的另一半 {@code stock_tx_id}
     * 已足以構成全序。搶佔是逐列 CAS，順序只決定先試哪一列。
     */
    @ResultMap("mybatis-plus_StockTxRecordEntity")
    @Select("""
        SELECT * FROM stock_tx_records
        WHERE phase = #{phase}
        AND status IN (#{statusInit}, #{statusFailed})
        AND (next_retry_time IS NULL OR next_retry_time <= NOW())
        AND created_at < NOW() - INTERVAL '2 minutes'
        ORDER BY created_at, stock_tx_id
        LIMIT #{limit}
    """)
    List<StockTxRecordEntity> findPendingCompensationByStatus(
            @Param("limit") int limit,
            @Param("phase") String phase,
            @Param("statusInit") String statusInit,
            @Param("statusFailed") String statusFailed);

    /**
     * 查詢需要重試的 RESTORE 記錄
     */
    default List<StockTxRecordEntity> findFailedRestores(int limit) {
        return findFailedRestoresByStatus(limit,
                StockTxPhase.RESTORE.value(),
                StockTxStatus.FAILED.value());
    }

    /** 候選述詞；排序收尾鍵的理由同 {@link #findPendingCompensationByStatus}。 */
    @ResultMap("mybatis-plus_StockTxRecordEntity")
    @Select("""
        SELECT * FROM stock_tx_records
        WHERE phase = #{phase}
        AND status = #{status}
        AND (next_retry_time IS NULL OR next_retry_time <= NOW())
        ORDER BY created_at, stock_tx_id
        LIMIT #{limit}
    """)
    List<StockTxRecordEntity> findFailedRestoresByStatus(
            @Param("limit") int limit,
            @Param("phase") String phase,
            @Param("status") String status);

    /**
     * 搶佔一筆待補償記錄（issue #2294 H5，<b>lease on 既有的 {@code next_retry_time}</b>）。
     *
     * <p>兩個候選 SQL（{@link #findPendingCompensationByStatus} /
     * {@link #findFailedRestoresByStatus}）都已含 {@code (next_retry_time IS NULL OR
     * next_retry_time <= NOW())}，本 claim 把它推到 {@code now + lease}
     * ⇒ 該列暫時離開候選集合、租約到期自動回來（《多副本排程搶佔規範》§二 六步 → lease）。
     * <b>零 migration</b>：欄位與索引皆既有。
     *
     * <p><b>述詞非純 PK</b>（規範 §一 ⑦）：{@code (stock_tx_id, phase)} 是複合主鍵，
     * 真正決定勝負的是 {@code next_retry_time} 那一條。
     *
     * <p><b>租約長度檢查</b>（§三）：候選述詞只有下界（{@code created_at < NOW()-2min}）
     * 沒有上界 ⇒ 窗口寬度 ∞ ⇒ 上界無條件成立；下界方面 claim 之後唯一的長工作是
     * {@code ProductGrpcClient.restoreStock}，其最壞耗時由
     * {@code ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC}（10 秒）硬性封頂
     * ⇒ 取 {@code StockCompensationWorker.CLAIM_LEASE_SEC}（300 秒）遠大於它。
     *
     * <p>⚠️ <b>與失敗路徑的寫入順序</b>：{@link #updateFailed} 也寫 {@code next_retry_time}
     * （指數退避），且一定在 claim <b>之後</b>執行 ⇒ 退避值天然覆蓋租約值（設計如此）。
     * 租約只影響「pod 在 claim 與 gRPC 之間崩潰」的救援延遲。
     *
     * @return 1 = 搶到（呼叫端才可以往下做副作用）；0 = 已被另一個 pod 認領
     */
    @Update("""
        UPDATE stock_tx_records
        SET next_retry_time = #{leaseUntil}, updated_at = NOW()
        WHERE stock_tx_id = #{stockTxId} AND phase = #{phase}
          AND (next_retry_time IS NULL OR next_retry_time <= NOW())
    """)
    int claimForCompensation(@Param("stockTxId") String stockTxId,
                             @Param("phase") String phase,
                             @Param("leaseUntil") LocalDateTime leaseUntil);

    @Update("""
        UPDATE stock_tx_records
        SET status = #{status}, order_no = #{orderNo}, updated_at = NOW()
        WHERE stock_tx_id = #{stockTxId} AND phase = #{phase}
    """)
    int updateStatusAndOrderNo(
            @Param("stockTxId") String stockTxId,
            @Param("phase") String phase,
            @Param("status") String status,
            @Param("orderNo") String orderNo
    );

    @Update("""
        UPDATE stock_tx_records
        SET status = #{status}, last_error = #{lastError},
            retry_count = retry_count + 1,
            next_retry_time = NOW() + (INTERVAL '1 second' * #{retryDelaySeconds}),
            updated_at = NOW()
        WHERE stock_tx_id = #{stockTxId} AND phase = #{phase}
    """)
    int updateFailed(
            @Param("stockTxId") String stockTxId,
            @Param("phase") String phase,
            @Param("status") String status,
            @Param("lastError") String lastError,
            @Param("retryDelaySeconds") int retryDelaySeconds
    );

    /**
     * 變更狀態（不動 retry_count / next_retry_time），供補償成功後標終態 COMPENSATED 用（03 §8.1）。
     */
    @Update("""
        UPDATE stock_tx_records
        SET status = #{status}, updated_at = NOW()
        WHERE stock_tx_id = #{stockTxId} AND phase = #{phase}
    """)
    int updateStatus(@Param("stockTxId") String stockTxId,
                     @Param("phase") String phase,
                     @Param("status") String status);

    /**
     * 冪等寫入 RESTORE 記錄（同 (stock_tx_id, phase) 已存在則略過），防 stock_tx_records_pkey DuplicateKey（03 §8.1）。
     */
    @org.apache.ibatis.annotations.Insert("""
        INSERT INTO stock_tx_records
            (stock_tx_id, order_no, phase, status, payload_snapshot, retry_count, created_at, updated_at)
        VALUES
            (#{stockTxId}, #{orderNo}, #{phase}, #{status}, #{payloadSnapshot}::jsonb, 0, NOW(), NOW())
        ON CONFLICT (stock_tx_id, phase) DO NOTHING
    """)
    int insertRestoreIfAbsent(@Param("stockTxId") String stockTxId,
                              @Param("orderNo") String orderNo,
                              @Param("phase") String phase,
                              @Param("status") String status,
                              @Param("payloadSnapshot") String payloadSnapshot);

    /**
     * 標記為需人工介入（超過最大重試次數）。status 存 enum value（'8914'），禁止字面字串。
     */
    @Update("""
        UPDATE stock_tx_records
        SET status = #{status}, updated_at = NOW()
        WHERE stock_tx_id = #{stockTxId} AND phase = #{phase}
        """)
    int markManualInterventionByStatus(@Param("stockTxId") String stockTxId,
                                       @Param("phase") String phase,
                                       @Param("status") String status);

    default int markManualIntervention(String stockTxId, String phase) {
        return markManualInterventionByStatus(stockTxId, phase, StockTxStatus.MANUAL_INTERVENTION.value());
    }

    /**
     * 刪除指定時間前已完成的庫存協調記錄（終態：SUCCESS / COMPENSATED / MANUAL_INTERVENTION）。
     */
    default int deleteCompletedBefore(LocalDateTime before) {
        return deleteCompletedBeforeByStatus(before, List.of(
                StockTxStatus.SUCCESS.value(),
                StockTxStatus.COMPENSATED.value(),
                StockTxStatus.MANUAL_INTERVENTION.value()));
    }

    @Delete("""
        <script>
        DELETE FROM stock_tx_records
        WHERE updated_at &lt; #{before}
          AND status IN
          <foreach item="s" collection="statuses" open="(" separator="," close=")">#{s}</foreach>
        </script>
    """)
    int deleteCompletedBeforeByStatus(@Param("before") LocalDateTime before,
                                       @Param("statuses") List<String> statuses);
}

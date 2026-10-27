package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

/**
 * {@code logistics_collection_attempts} 的 mapper。
 *
 * <p>🔴 <b>送出前必須先落地這一列（pre-commit），才呼叫 PayUni。</b> 順序顛倒的後果：
 * HTTP 逾時 ⇒ 我方沒有紀錄 ⇒ 重試產生第二個 TradeNo ⇒ 重複扣款且事後查不出來。
 */
@Mapper
public interface LogisticsCollectionAttemptMapper extends BaseMapper<LogisticsCollectionAttemptEntity> {

    /**
     * pre-commit：落地 attempt。
     *
     * <p>⚠️ 這一句<b>刻意沒有</b> {@code ON CONFLICT}：{@code trade_no} 撞號代表我方 TSID 產生器
     * 或呼叫端邏輯出了問題，那必須 fail-loud，不是靜默跳過。
     */
    @Insert("""
            INSERT INTO logistics_collection_attempts (
                id, receivable_id, trade_no, gross_amount
            ) VALUES (#{id}, #{receivableId}, #{tradeNo}, #{grossAmount})
            """)
    int insertPreCommit(@Param("id") Long id,
                        @Param("receivableId") Long receivableId,
                        @Param("tradeNo") String tradeNo,
                        @Param("grossAmount") Long grossAmount);

    /**
     * 寫回受理結果。
     *
     * <p>🔴 <b>{@code gross_amount} 一併更新</b>：pre-commit 落地時還不知道要送多少
     * （gross-up 在 payment adapter，domain 不得知道費率），那一欄先放目標淨額當佔位值。
     * 拿到回應後<b>必須</b>改成 adapter 實際送出的金額——
     * ① {@code agent_fee_query} 的 key 是 {@code (MerID, TradeNo, Amt)} 三元組，帶錯就查不到；
     * ② {@code net_proceeds = gross − actual_fee}，用佔位值算會得到一個看起來合理但錯的實收。
     */
    @Update("""
            UPDATE logistics_collection_attempts
            SET submit_status = #{submitStatus},
                submit_message = #{submitMessage,jdbcType=VARCHAR},
                submitted_at = #{submittedAt},
                expected_fund_date = #{expectedFundDate,jdbcType=DATE},
                gross_amount = #{grossAmount},
                updated_at = NOW()
            WHERE id = #{id}
            """)
    int markSubmitted(@Param("id") Long id,
                      @Param("submitStatus") String submitStatus,
                      @Param("submitMessage") String submitMessage,
                      @Param("grossAmount") Long grossAmount,
                      @Param("submittedAt") LocalDateTime submittedAt,
                      @Param("expectedFundDate") LocalDate expectedFundDate);

    /**
     * 🔴 以 <b>lease</b> 搶佔待查詢的 attempt（10 檔 §29），<b>不是「已查過」旗標</b>。
     *
     * <p>租約基準在 claim 當下<b>逐筆</b>取（{@code NOW() + make_interval}），不在批次迴圈外
     * 算一次——後者會讓不變式從「單筆最壞耗時 &lt; 租約」退化成「批次總耗時 &lt; 租約」，
     * 隨 batch size 一起破。
     *
     *
     * <p>⚠️ <b>實測出來的限制，寫在這裡而不是假裝它不存在</b>：{@code NOW()} 在 PostgreSQL 是
     * <b>交易開始時間</b>，所以<b>同一次 claim 撈到的整批列共用同一個租約起點</b>
     * （實測：{@code batchSize=10} 一次撈 2 筆，兩筆的 {@code *_lease_until} 位元相同）。
     * ⇒ 本方法保證的是「租約起點由 DB 在 claim 當下產生」，<b>不是</b>「每一列各自從它被處理的
     * 那一刻開始算」。不變式因此是：
     * <pre>批次內所有列的處理總耗時 &lt; 租約長度</pre>
     * 🔴 呼叫端必須二選一，否則 10 檔 §29 警告的那個退化就在這裡發生：
     * ① {@code batchSize = 1}（不變式回到「單筆最壞耗時 &lt; 租約」，最穩）；
     * ② 租約長度 &gt; {@code batchSize × 單筆最壞耗時}（含每個外部呼叫的 deadline）。
     * <b>兩者都不做就是「隨 batch size 一起破」。</b>
     * <p>⚠️ 候選述詞是 {@code fund_date IS NULL AND submitted_at IS NOT NULL}：
     * 「還沒送出」與「已查到入帳」都不該進來。
     */
    @Select("""
            UPDATE logistics_collection_attempts
            SET query_lease_until = NOW() + make_interval(secs => #{leaseSeconds}),
                updated_at = NOW()
            WHERE id IN (
                SELECT a.id
                FROM logistics_collection_attempts a
                WHERE a.fund_date IS NULL
                  AND a.submitted_at IS NOT NULL
                  AND (a.query_lease_until IS NULL OR a.query_lease_until <= NOW())
                ORDER BY a.submitted_at
                LIMIT #{batchSize}
                FOR UPDATE SKIP LOCKED
            )
            RETURNING *
            """)
    List<LogisticsCollectionAttemptEntity> claimForQuery(@Param("leaseSeconds") int leaseSeconds,
                                                         @Param("batchSize") int batchSize);

    /**
     * 寫回查詢結果並歸還租約。
     *
     * <p>🔴 {@code net_proceeds = gross_amount − actual_fee}（{@code chk_logcoll_net} 在 DB 層
     * 守著同一式）。{@code residual = target_net − net_proceeds} 由呼叫端算（它才知道 target_net）。
     *
     * <p>⚠️ {@code fund_date} 與 {@code actual_fee} 必須同時有值（{@code chk_logcoll_settled_pair}）：
     * 單邊有值代表查詢只讀了一半。
     */
    @Update("""
            UPDATE logistics_collection_attempts
            SET fund_date = #{fundDate},
                actual_fee = #{actualFee},
                net_proceeds = #{netProceeds},
                residual = #{residual},
                query_lease_until = NULL,
                updated_at = NOW()
            WHERE id = #{id}
              AND fund_date IS NULL
            """)
    int recordQueryResult(@Param("id") Long id,
                          @Param("fundDate") LocalDate fundDate,
                          @Param("actualFee") Long actualFee,
                          @Param("netProceeds") Long netProceeds,
                          @Param("residual") Long residual);

    /**
     * 提前歸還查詢租約。⚠️ 最佳化不是正確性來源（同 {@code releaseSubmitLease}）。
     */
    @Update("""
            UPDATE logistics_collection_attempts
            SET query_lease_until = NULL,
                updated_at = NOW()
            WHERE id = #{id}
            """)
    int releaseQueryLease(@Param("id") Long id);

    @Select("""
            SELECT * FROM logistics_collection_attempts
            WHERE receivable_id = #{receivableId}
            ORDER BY created_at DESC
            """)
    List<LogisticsCollectionAttemptEntity> listByReceivable(@Param("receivableId") Long receivableId);

    @Select("""
            SELECT * FROM logistics_collection_attempts
            WHERE receivable_id = #{receivableId}
            ORDER BY created_at DESC
            LIMIT 1
            """)
    LogisticsCollectionAttemptEntity findLatestByReceivable(@Param("receivableId") Long receivableId);

    @Select("SELECT * FROM logistics_collection_attempts WHERE trade_no = #{tradeNo}")
    LogisticsCollectionAttemptEntity findByTradeNo(@Param("tradeNo") String tradeNo);
}

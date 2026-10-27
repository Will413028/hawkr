package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import java.util.List;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

/**
 * {@code provider_settlement_lines} 的 mapper。
 */
@Mapper
public interface ProviderSettlementLineMapper extends BaseMapper<ProviderSettlementLineEntity> {

    /**
     * 匯入一列帳單明細，以 {@code uq_provsettle_line} 去重。
     *
     * <p>⚠️ 該 UNIQUE 的欄位之一（{@code provider_shipment_ref}）<b>可為 NULL</b>，而
     * PostgreSQL 預設 {@code NULL} 之間互不相等 ⇒ <b>彙總列（該欄為 NULL）不會被這個 UNIQUE 去重</b>。
     * 重複匯入同一份帳單的彙總列會產生重複列。呼叫端要自己在匯入層擋
     * （例如以 {@code statement_ref} 為單位一次性匯入並先檢查是否已匯過）。
     * 這是<b>誠實標註的限制</b>，不是可以靠 {@code ON CONFLICT} 解掉的。
     *
     * @return 1 = 新建；0 = 該明細列已匯入過（僅對 {@code provider_shipment_ref} 非 NULL 者成立）
     */
    @Insert("""
            INSERT INTO provider_settlement_lines (
                id, provider, statement_ref, statement_period, provider_shipment_ref,
                order_no, fee_type, amount, recon_status, ledger_txn_no, exception_case_id, imported_by
            ) VALUES (
                #{e.id}, #{e.provider}, #{e.statementRef}, #{e.statementPeriod},
                #{e.providerShipmentRef,jdbcType=VARCHAR}, #{e.orderNo,jdbcType=VARCHAR},
                #{e.feeType}, #{e.amount}, #{e.reconStatus}, #{e.ledgerTxnNo,jdbcType=VARCHAR},
                #{e.exceptionCaseId,jdbcType=BIGINT}, #{e.importedBy}
            )
            ON CONFLICT ON CONSTRAINT uq_provsettle_line DO NOTHING
            """)
    int insertIfAbsent(@Param("e") ProviderSettlementLineEntity entity);

    @Select("SELECT * FROM provider_settlement_lines WHERE id = #{id}")
    ProviderSettlementLineEntity findById(@Param("id") Long id);

    @Select("""
            SELECT * FROM provider_settlement_lines
            WHERE provider = #{provider} AND statement_period = #{statementPeriod}
            ORDER BY imported_at
            """)
    List<ProviderSettlementLineEntity> listByStatement(@Param("provider") String provider,
                                                       @Param("statementPeriod") String statementPeriod);

    /**
     * 一份帳單（{@code provider} ＋ {@code statement_ref}）的全部明細列。
     *
     * <p>🔴 與 {@link #listByStatement} 的差別是<b>鍵不同</b>：那一支用
     * {@code statement_period}（同一期可能有多份帳單），本支用 {@code statement_ref}
     * （一份帳單）。匯入層的「彙總列是否已匯過」必須用<b>後者</b>判斷——
     * 用期別判斷會把同期第二份帳單誤判成重複。
     */
    @Select("""
            SELECT * FROM provider_settlement_lines
            WHERE provider = #{provider} AND statement_ref = #{statementRef}
            ORDER BY imported_at, id
            """)
    List<ProviderSettlementLineEntity> listByStatementRef(@Param("provider") String provider,
                                                          @Param("statementRef") String statementRef);

    @Select("""
            <script>
            SELECT * FROM provider_settlement_lines
            WHERE recon_status IN
              <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
            ORDER BY imported_at
            LIMIT #{limit}
            </script>
            """)
    List<ProviderSettlementLineEntity> listByReconStatus(@Param("statuses") List<String> statuses,
                                                         @Param("limit") int limit);

    /**
     * 比對成功（CAS）。
     *
     * <p>🔴 {@code chk_provsettle_ledger} 是<b>雙向</b>等式（{@code MATCHED} ⇔ {@code ledger_txn_no}
     * 非 NULL）⇒ 狀態與分錄號<b>必須同一句寫</b>，拆開任一邊都會被 DB 擋下。
     *
     * <p>allowed-from 由呼叫端傳入（本表同樣尚無 {@code *Rules} helper，見
     * {@code FulfillmentExceptionCaseMapper} 的同一段說明）。
     *
     * @return 1 = 成功；0 = 已不在 allowed-from
     */
    @Update("""
            <script>
            UPDATE provider_settlement_lines
            SET recon_status = #{matchedStatus},
                ledger_txn_no = #{ledgerTxnNo}
            WHERE id = #{id}
              AND recon_status IN
              <foreach collection="allowedFrom" item="s" open="(" separator="," close=")">#{s}</foreach>
            </script>
            """)
    int markMatched(@Param("id") Long id,
                    @Param("matchedStatus") String matchedStatus,
                    @Param("ledgerTxnNo") String ledgerTxnNo,
                    @Param("allowedFrom") List<String> allowedFrom);

    /**
     * 轉為爭議／核銷（CAS）。
     *
     * <p>🔴 同一條雙向等式要求：離開 {@code MATCHED} 時 {@code ledger_txn_no} 必須一起清空。
     * 本句無條件把它設成 NULL，正是為了讓「從 MATCHED 退回爭議」這條邊不會被 DB 擋下。
     */
    @Update("""
            <script>
            UPDATE provider_settlement_lines
            SET recon_status = #{toStatus},
                ledger_txn_no = NULL
            WHERE id = #{id}
              AND recon_status IN
              <foreach collection="allowedFrom" item="s" open="(" separator="," close=")">#{s}</foreach>
            </script>
            """)
    int markUnmatchedOutcome(@Param("id") Long id,
                             @Param("toStatus") String toStatus,
                             @Param("allowedFrom") List<String> allowedFrom);

    /** 回填責任／對帳調整所開的例外案件。 */
    @Update("""
            UPDATE provider_settlement_lines
            SET exception_case_id = #{exceptionCaseId}
            WHERE id = #{id}
              AND exception_case_id IS NULL
            """)
    int bindExceptionCase(@Param("id") Long id, @Param("exceptionCaseId") Long exceptionCaseId);
}

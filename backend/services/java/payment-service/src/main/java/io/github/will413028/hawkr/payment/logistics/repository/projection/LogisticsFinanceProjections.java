package io.github.will413028.hawkr.payment.logistics.repository.projection;

import java.time.LocalDateTime;

/**
 * 物流財務唯讀查詢的投影型別（10 檔 §11.3）。
 *
 * <p>🔴 <b>聚合查詢一律回 record，不回部分填充的 Entity。</b> 回 Entity 的後果寫在 §11.3 的
 * 教訓裡：簽章說「這是一個完整的 row」，實際只有兩欄有值，下游對其餘欄位做任何事都是 NPE，
 * 而編譯器與測試都看不到。
 *
 * <p>⚠️ 這裡的每一個 record 都對應一句 {@code SELECT}，欄位順序即 {@code SELECT} 的欄位順序
 * ——MyBatis 依<b>名字</b>對映（{@code mapUnderscoreToCamelCase}），所以改名要兩邊一起改。
 */
public final class LogisticsFinanceProjections {

    private LogisticsFinanceProjections() {
    }

    /**
     * 應收側的彙總。
     *
     * @param estimatedCostTotal  Σ {@code estimated_cost}（維度 1/11）
     * @param actualCostTotal     Σ {@code actual_cost}。🔴 SQL 的 {@code SUM} 忽略 NULL ⇒ 本值
     *                            只含<b>已對帳</b>的筆數；缺口由 {@code unreconciledCount} 表達，
     *                            不得把「尚未對帳」當成 0 併進來（維度 2/11）
     * @param unreconciledCount   {@code actual_cost IS NULL} 的筆數
     * @param targetNetTotal      Σ {@code target_net}（維度 3/11，賣家實際被收的物流費）
     * @param platformSubsidyTotal Σ {@code platform_subsidy}（維度 5/11）
     * @param carriedResidualTotal Σ {@code carried_residual}（可負）
     * @param receivableCount     期間內的應收筆數
     */
    public record ReceivableAggregate(long estimatedCostTotal,
                                      long actualCostTotal,
                                      long unreconciledCount,
                                      long targetNetTotal,
                                      long platformSubsidyTotal,
                                      long carriedResidualTotal,
                                      long receivableCount) {

        /** 查無任何應收時的零值（SQL 端已 COALESCE，這裡是給「完全沒有列」的呼叫端用）。 */
        public static ReceivableAggregate empty() {
            return new ReceivableAggregate(0L, 0L, 0L, 0L, 0L, 0L, 0L);
        }
    }

    /**
     * 收款側的彙總。
     *
     * @param actualFeeTotal        Σ {@code actual_fee}（維度 4/11）。🔴 只含已查到 {@code FundDate}
     *                              的；缺口由 {@code pendingFeeQueryCount} 表達
     * @param pendingFeeQueryCount  {@code fund_date IS NULL} 的 attempt 筆數
     * @param residualTotal         Σ {@code residual}（維度 10/11，可正可負）
     * @param creditGeneratedTotal  Σ {@code |residual|}（只計 {@code residual < 0} 的），
     *                              ＝ 這段期間產生的貸記本金合計
     * @param netProceedsTotal      Σ {@code net_proceeds}（平台真正收到）
     */
    public record CollectionAggregate(long actualFeeTotal,
                                      long pendingFeeQueryCount,
                                      long residualTotal,
                                      long creditGeneratedTotal,
                                      long netProceedsTotal) {

        public static CollectionAggregate empty() {
            return new CollectionAggregate(0L, 0L, 0L, 0L, 0L);
        }
    }

    /**
     * 依 {@code kind} 分組的調整單合計。
     *
     * <p>🔴 {@code RESPONSIBILITY}（維度 7/11）與 {@code RECONCILIATION}（維度 8/11）分兩列回，
     * <b>不在 SQL 裡就加起來</b>：一個是「這筆錢算誰的」、一個是「帳單與我方暫估差多少」，
     * 合併之後就再也分不出對帳差額有多少。
     *
     * @param amountTotal 可為負（{@code logistics_adjustments.amount} 允許負數）
     */
    public record AdjustmentKindTotal(String kind, long amountTotal, long adjustmentCount) {
    }

    /** 依應收狀態分組的筆數（維度 9/11）。 */
    public record ReceivableStatusCount(String status, long count) {
    }

    /**
     * 一份 provider 帳單（{@code provider} ＋ {@code statement_ref}）的彙總。
     *
     * <p>🔴 {@code mismatchCount} / {@code mismatchAmount} 是<b>推導值</b>而不是
     * {@code recon_status} 的碼值：V9 的 {@code chk_provsettle_recon} 只有四值
     * （UNMATCHED / MATCHED / DISPUTED / WRITTEN_OFF），<b>沒有 MISMATCH</b>。
     * 「金額對不上」在本模型的落點是 {@code kind='RECONCILIATION'} 的調整單
     * （{@code logistics_adjustments.statement_line_id} 指回帳單列）⇒ 這兩欄由那張表推出來。
     * <b>不在 CodeMeta 或 CHECK 裡新增碼值</b>——值域住在 V9，多一個值就是第二份碼表。
     */
    public record StatementAggregate(String provider,
                                     String statementRef,
                                     String statementPeriod,
                                     long lineCount,
                                     long totalAmount,
                                     long matchedCount,
                                     long unmatchedCount,
                                     long disputedCount,
                                     long writtenOffCount,
                                     long mismatchCount,
                                     long mismatchAmount,
                                     LocalDateTime firstImportedAt,
                                     LocalDateTime lastImportedAt) {
    }

    /**
     * 一列帳單明細的對帳調整推導值（mismatch 的單列形式）。
     *
     * @param adjustmentCount  該列有幾張 {@code kind='RECONCILIATION'} 的調整單
     * @param adjustmentAmount 其金額合計（可負）
     */
    public record LineReconciliationDelta(long lineId, long adjustmentCount, long adjustmentAmount) {
    }
}

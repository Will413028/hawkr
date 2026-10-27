package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerTransactionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.AdjustmentKindTotal;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.CollectionAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.LineReconciliationDelta;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableStatusCount;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.StatementAggregate;
import java.time.LocalDateTime;
import java.util.List;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * 物流財務的<b>唯讀</b>查詢 mapper（W6-3 賣家面 ＋ W6-4 Admin 面的資料來源）。
 *
 * <h2>為什麼獨立成一支而不是加進既有九支 mapper</h2>
 * 既有那九支是「寫入路徑 ＋ 它自己需要的讀」，每一支都帶著 CAS／lease／ON CONFLICT 的
 * 不變式。查詢面的需求（分頁、多條件篩選、跨表彙總）與那些不變式無關，混進去會讓
 * 「這一句是不是會改狀態」需要逐句讀才知道。分開之後，<b>本檔沒有任何 INSERT／UPDATE</b>
 * 是一個看一眼就成立的性質。
 *
 * <h2>🔴 賣家面的 store 範圍是 SQL 述詞，不是呼叫端的自律</h2>
 * 每一支 {@code seller*} 查詢都帶 {@code store_id = #{storeId}}（或 JOIN 回應收再比對），
 * 且 {@code storeId} 來自 Gateway 的 sellerid resolver。拿掉那個述詞 ⇒ 賣家看得到別人的
 * 應收，而回應形狀完全不變、沒有任何訊號。
 * {@code LogisticsFinanceQueryScopeContractTest} 逐句斷言那個述詞還在。
 *
 * <h2>🔴 record 投影靠「欄位順序」對映，不是靠名字</h2>
 * MyBatis 對 record 走 constructor automapping：專案沒有開 {@code -parameters} 編譯旗標
 * ⇒ 參數名在 class 檔裡不存在 ⇒ 對映退回<b>依序</b>。所以每一句 {@code SELECT} 的欄位順序
 * 必須與對應 record 的 component 順序<b>逐一對齊</b>，而且順序錯了是<b>靜默</b>的
 * （同型別的相鄰兩欄互換，編譯過、測試過、數字錯）。
 * {@code LogisticsFinanceProjectionOrderTest} 把這件事釘死。
 *
 * <h2>值域字面值的來源</h2>
 * SQL 內<b>沒有</b>任何業務碼值字面值：狀態、kind、recon_status 一律由呼叫端以
 * {@code #{}} 傳入，來源是 CodeMeta enum 或 V9 的 CHECK（10 檔 §8 禁止硬編碼）。
 */
@Mapper
public interface LogisticsFinanceQueryMapper {

    // =========================================================================
    // 賣家面
    // =========================================================================

    /**
     * 賣家應收側彙總（維度 1／2／3／5）。
     *
     * <p>🔴 {@code SUM(actual_cost)} 會忽略 NULL ⇒ 回來的是「已對帳部分」的合計。
     * 這正是要的語意，但呼叫端<b>不得</b>把它當成「全部的實際成本」——缺口由同一列的
     * {@code unreconciled_count} 表達。
     *
     * <p>欄位順序 ＝ {@link ReceivableAggregate} 的 component 順序（見類別註解）。
     */
    @Select("""
            <script>
            SELECT
              COALESCE(SUM(estimated_cost), 0)                AS estimated_cost_total,
              COALESCE(SUM(actual_cost), 0)                   AS actual_cost_total,
              COUNT(*) FILTER (WHERE actual_cost IS NULL)     AS unreconciled_count,
              COALESCE(SUM(target_net), 0)                    AS target_net_total,
              COALESCE(SUM(platform_subsidy), 0)              AS platform_subsidy_total,
              COALESCE(SUM(carried_residual), 0)              AS carried_residual_total,
              COUNT(*)                                        AS receivable_count
            FROM logistics_receivables
            WHERE store_id = #{storeId}
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
            </script>
            """)
    ReceivableAggregate sellerReceivableAggregate(@Param("storeId") long storeId,
                                                  @Param("createdFrom") LocalDateTime createdFrom,
                                                  @Param("createdTo") LocalDateTime createdTo);

    /**
     * 賣家收款側彙總（維度 4／10／11 的產生量）。
     *
     * <p>🔴 期間條件比對的是<b>應收</b>的 {@code created_at} 而不是 attempt 的——兩者同一個
     * 期間軸，報表才對得起來（同一筆應收的重送 attempt 可能落在下個月，但那筆費用屬於
     * 應收成立的那個月）。
     *
     * <p>JOIN 回 {@code logistics_receivables} 是<b>賣家範圍的唯一來源</b>：
     * {@code logistics_collection_attempts} 自己沒有 {@code store_id} 欄。
     */
    @Select("""
            <script>
            SELECT
              COALESCE(SUM(a.actual_fee), 0)                                          AS actual_fee_total,
              COUNT(*) FILTER (WHERE a.fund_date IS NULL)                             AS pending_fee_query_count,
              COALESCE(SUM(a.residual), 0)                                            AS residual_total,
              COALESCE(SUM(CASE WHEN a.residual &lt; 0 THEN -a.residual ELSE 0 END), 0) AS credit_generated_total,
              COALESCE(SUM(a.net_proceeds), 0)                                        AS net_proceeds_total
            FROM logistics_collection_attempts a
            JOIN logistics_receivables r ON r.id = a.receivable_id
            WHERE r.store_id = #{storeId}
              <if test="createdFrom != null">AND r.created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND r.created_at &lt;= #{createdTo}</if>
            </script>
            """)
    CollectionAggregate sellerCollectionAggregate(@Param("storeId") long storeId,
                                                  @Param("createdFrom") LocalDateTime createdFrom,
                                                  @Param("createdTo") LocalDateTime createdTo);

    /**
     * 賣家調整單彙總，<b>依 kind 分組</b>（維度 7 與 8）。
     *
     * <p>🔴 分組而不是加總：責任調整與對帳調整合併之後，就再也分不出「帳單與暫估差多少」。
     */
    @Select("""
            <script>
            SELECT kind                          AS kind,
                   COALESCE(SUM(amount), 0)      AS amount_total,
                   COUNT(*)                      AS adjustment_count
            FROM logistics_adjustments
            WHERE store_id = #{storeId}
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
            GROUP BY kind
            ORDER BY kind
            </script>
            """)
    List<AdjustmentKindTotal> sellerAdjustmentTotals(@Param("storeId") long storeId,
                                                     @Param("createdFrom") LocalDateTime createdFrom,
                                                     @Param("createdTo") LocalDateTime createdTo);

    /** 賣家應收的狀態分佈（維度 9）。 */
    @Select("""
            <script>
            SELECT status AS status, COUNT(*) AS count
            FROM logistics_receivables
            WHERE store_id = #{storeId}
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
            GROUP BY status
            ORDER BY status
            </script>
            """)
    List<ReceivableStatusCount> sellerStatusCounts(@Param("storeId") long storeId,
                                                   @Param("createdFrom") LocalDateTime createdFrom,
                                                   @Param("createdTo") LocalDateTime createdTo);

    /**
     * 賣家應收明細（分頁）。
     *
     * <p>🔴 {@code store_id = #{storeId}} 是賣家看不到別人應收的<b>唯一</b>機械保證。
     */
    @Select("""
            <script>
            SELECT * FROM logistics_receivables
            WHERE store_id = #{storeId}
              <if test="statuses != null and statuses.size() > 0">
                AND status IN
                <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
              </if>
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
            ORDER BY created_at DESC, id DESC
            LIMIT #{limit} OFFSET #{offset}
            </script>
            """)
    List<LogisticsReceivableEntity> sellerListReceivables(@Param("storeId") long storeId,
                                                          @Param("statuses") List<String> statuses,
                                                          @Param("createdFrom") LocalDateTime createdFrom,
                                                          @Param("createdTo") LocalDateTime createdTo,
                                                          @Param("limit") int limit,
                                                          @Param("offset") int offset);

    /** {@link #sellerListReceivables} 的總筆數（述詞必須逐字相同，否則分頁會與內容不一致）。 */
    @Select("""
            <script>
            SELECT COUNT(*) FROM logistics_receivables
            WHERE store_id = #{storeId}
              <if test="statuses != null and statuses.size() > 0">
                AND status IN
                <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
              </if>
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
            </script>
            """)
    long sellerCountReceivables(@Param("storeId") long storeId,
                                @Param("statuses") List<String> statuses,
                                @Param("createdFrom") LocalDateTime createdFrom,
                                @Param("createdTo") LocalDateTime createdTo);

    /**
     * 一批應收各自<b>最新</b>的收款嘗試。
     *
     * <p>🔴 用 {@code DISTINCT ON} 而不是在 Java 端撈全部再挑：一筆應收的 attempt 可以有多筆
     * （退回 PENDING 重送），全撈會讓列表頁的查詢量隨重送次數成長。
     *
     * <p>⚠️ {@code ORDER BY} 的第一個鍵<b>必須</b>是 {@code DISTINCT ON} 的欄位，這是 PostgreSQL
     * 的語法要求；第二個鍵才是「哪一筆算最新」。
     */
    @Select("""
            <script>
            SELECT DISTINCT ON (receivable_id) *
            FROM logistics_collection_attempts
            WHERE receivable_id IN
              <foreach collection="receivableIds" item="id" open="(" separator="," close=")">#{id}</foreach>
            ORDER BY receivable_id, created_at DESC, id DESC
            </script>
            """)
    List<LogisticsCollectionAttemptEntity> latestAttemptsByReceivableIds(
            @Param("receivableIds") List<Long> receivableIds);

    /** 一批應收各自掛著的調整單（用於把維度 7／8 填回單筆列）。 */
    @Select("""
            <script>
            SELECT * FROM logistics_adjustments
            WHERE receivable_id IN
              <foreach collection="receivableIds" item="id" open="(" separator="," close=")">#{id}</foreach>
            ORDER BY receivable_id, created_at
            </script>
            """)
    List<LogisticsAdjustmentEntity> adjustmentsByReceivableIds(
            @Param("receivableIds") List<Long> receivableIds);

    /**
     * 賣家的物流帳本表頭（分頁）。
     *
     * <p>🔴 <b>必須帶 {@code commerce_model}</b>：ADR-010 放寬 {@code chk_ledgtxn_model} 之後，
     * 舊商流的 dormant 帳本與新商流共用同一張表，不帶條件會把兩套帳混算。
     */
    @Select("""
            SELECT * FROM ledger_transactions
            WHERE store_id = #{storeId}
              AND commerce_model = #{commerceModel}
            ORDER BY posted_at DESC, id DESC
            LIMIT #{limit} OFFSET #{offset}
            """)
    List<LedgerTransactionEntity> sellerListLedgerTxns(@Param("storeId") long storeId,
                                                       @Param("commerceModel") String commerceModel,
                                                       @Param("limit") int limit,
                                                       @Param("offset") int offset);

    /** {@link #sellerListLedgerTxns} 的總筆數。 */
    @Select("""
            SELECT COUNT(*) FROM ledger_transactions
            WHERE store_id = #{storeId}
              AND commerce_model = #{commerceModel}
            """)
    long sellerCountLedgerTxns(@Param("storeId") long storeId,
                               @Param("commerceModel") String commerceModel);

    // =========================================================================
    // Admin 面
    // =========================================================================

    /**
     * Admin 應收列表。
     *
     * <p>⚠️ {@code minAgingDays} 的語意是「尚未進終態，且已經存在 N 天以上」——
     * 終態（CLOSED／WRITTEN_OFF）的列不算逾期，所以述詞同時排除它們。終態清單由呼叫端
     * 以 {@code terminalStatuses} 傳入（來源 {@code ReceivableStatusRules.isTerminal}），
     * SQL 自己不知道任何狀態值。
     */
    @Select("""
            <script>
            SELECT * FROM logistics_receivables
            <where>
              <if test="storeId != null">AND store_id = #{storeId}</if>
              <if test="statuses != null and statuses.size() > 0">
                AND status IN
                <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
              </if>
              <if test="provider != null">AND provider = #{provider}</if>
              <if test="orderNo != null">AND order_no = #{orderNo}</if>
              <if test="providerDeliveryRef != null">AND provider_delivery_ref = #{providerDeliveryRef}</if>
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
              <if test="unreconciledOnly">AND actual_cost IS NULL</if>
              <if test="agingBefore != null">
                AND created_at &lt;= #{agingBefore}
                AND status NOT IN
                <foreach collection="terminalStatuses" item="t" open="(" separator="," close=")">#{t}</foreach>
              </if>
            </where>
            ORDER BY created_at DESC, id DESC
            LIMIT #{limit} OFFSET #{offset}
            </script>
            """)
    List<LogisticsReceivableEntity> adminListReceivables(@Param("storeId") Long storeId,
                                                         @Param("statuses") List<String> statuses,
                                                         @Param("provider") String provider,
                                                         @Param("orderNo") String orderNo,
                                                         @Param("providerDeliveryRef") String providerDeliveryRef,
                                                         @Param("createdFrom") LocalDateTime createdFrom,
                                                         @Param("createdTo") LocalDateTime createdTo,
                                                         @Param("unreconciledOnly") boolean unreconciledOnly,
                                                         @Param("agingBefore") LocalDateTime agingBefore,
                                                         @Param("terminalStatuses") List<String> terminalStatuses,
                                                         @Param("limit") int limit,
                                                         @Param("offset") int offset);

    /** {@link #adminListReceivables} 的總筆數（述詞逐字相同）。 */
    @Select("""
            <script>
            SELECT COUNT(*) FROM logistics_receivables
            <where>
              <if test="storeId != null">AND store_id = #{storeId}</if>
              <if test="statuses != null and statuses.size() > 0">
                AND status IN
                <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
              </if>
              <if test="provider != null">AND provider = #{provider}</if>
              <if test="orderNo != null">AND order_no = #{orderNo}</if>
              <if test="providerDeliveryRef != null">AND provider_delivery_ref = #{providerDeliveryRef}</if>
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
              <if test="unreconciledOnly">AND actual_cost IS NULL</if>
              <if test="agingBefore != null">
                AND created_at &lt;= #{agingBefore}
                AND status NOT IN
                <foreach collection="terminalStatuses" item="t" open="(" separator="," close=")">#{t}</foreach>
              </if>
            </where>
            </script>
            """)
    long adminCountReceivables(@Param("storeId") Long storeId,
                               @Param("statuses") List<String> statuses,
                               @Param("provider") String provider,
                               @Param("orderNo") String orderNo,
                               @Param("providerDeliveryRef") String providerDeliveryRef,
                               @Param("createdFrom") LocalDateTime createdFrom,
                               @Param("createdTo") LocalDateTime createdTo,
                               @Param("unreconciledOnly") boolean unreconciledOnly,
                               @Param("agingBefore") LocalDateTime agingBefore,
                               @Param("terminalStatuses") List<String> terminalStatuses);

    /**
     * Admin 收款嘗試列表。
     *
     * <p>⚠️ {@code store_id} 只存在於 {@code logistics_receivables} ⇒ 一律 JOIN 回去；
     * 回傳同時帶回 {@code r.store_id}，所以本查詢<b>不能</b>用 {@code SELECT a.*} 回 Entity
     * ——那會是 partial projection 的反向（多一欄）。改成兩段：本查詢只回 attempt 的完整列，
     * store_id 由呼叫端以 receivable 反查（列表頁本來就要顯示應收編號）。
     */
    @Select("""
            <script>
            SELECT a.* FROM logistics_collection_attempts a
            JOIN logistics_receivables r ON r.id = a.receivable_id
            <where>
              <if test="receivableId != null">AND a.receivable_id = #{receivableId}</if>
              <if test="tradeNo != null">AND a.trade_no = #{tradeNo}</if>
              <if test="storeId != null">AND r.store_id = #{storeId}</if>
              <if test="submitStatus != null">AND a.submit_status = #{submitStatus}</if>
              <if test="pendingFundDateOnly">AND a.fund_date IS NULL</if>
              <if test="residualOnly">AND a.residual IS NOT NULL AND a.residual &lt;&gt; 0</if>
              <if test="createdFrom != null">AND a.created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND a.created_at &lt;= #{createdTo}</if>
            </where>
            ORDER BY a.created_at DESC, a.id DESC
            LIMIT #{limit} OFFSET #{offset}
            </script>
            """)
    List<LogisticsCollectionAttemptEntity> adminListCollections(@Param("receivableId") Long receivableId,
                                                                @Param("tradeNo") String tradeNo,
                                                                @Param("storeId") Long storeId,
                                                                @Param("submitStatus") String submitStatus,
                                                                @Param("pendingFundDateOnly") boolean pendingFundDateOnly,
                                                                @Param("residualOnly") boolean residualOnly,
                                                                @Param("createdFrom") LocalDateTime createdFrom,
                                                                @Param("createdTo") LocalDateTime createdTo,
                                                                @Param("limit") int limit,
                                                                @Param("offset") int offset);

    /** {@link #adminListCollections} 的總筆數（述詞逐字相同）。 */
    @Select("""
            <script>
            SELECT COUNT(*) FROM logistics_collection_attempts a
            JOIN logistics_receivables r ON r.id = a.receivable_id
            <where>
              <if test="receivableId != null">AND a.receivable_id = #{receivableId}</if>
              <if test="tradeNo != null">AND a.trade_no = #{tradeNo}</if>
              <if test="storeId != null">AND r.store_id = #{storeId}</if>
              <if test="submitStatus != null">AND a.submit_status = #{submitStatus}</if>
              <if test="pendingFundDateOnly">AND a.fund_date IS NULL</if>
              <if test="residualOnly">AND a.residual IS NOT NULL AND a.residual &lt;&gt; 0</if>
              <if test="createdFrom != null">AND a.created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND a.created_at &lt;= #{createdTo}</if>
            </where>
            </script>
            """)
    long adminCountCollections(@Param("receivableId") Long receivableId,
                               @Param("tradeNo") String tradeNo,
                               @Param("storeId") Long storeId,
                               @Param("submitStatus") String submitStatus,
                               @Param("pendingFundDateOnly") boolean pendingFundDateOnly,
                               @Param("residualOnly") boolean residualOnly,
                               @Param("createdFrom") LocalDateTime createdFrom,
                               @Param("createdTo") LocalDateTime createdTo);

    /** Admin 調整單列表。 */
    @Select("""
            <script>
            SELECT * FROM logistics_adjustments
            <where>
              <if test="storeId != null">AND store_id = #{storeId}</if>
              <if test="receivableId != null">AND receivable_id = #{receivableId}</if>
              <if test="kind != null">AND kind = #{kind}</if>
              <if test="responsibilityCause != null">AND responsibility_cause = #{responsibilityCause}</if>
              <if test="statementLineId != null">AND statement_line_id = #{statementLineId}</if>
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
            </where>
            ORDER BY created_at DESC, id DESC
            LIMIT #{limit} OFFSET #{offset}
            </script>
            """)
    List<LogisticsAdjustmentEntity> adminListAdjustments(@Param("storeId") Long storeId,
                                                         @Param("receivableId") Long receivableId,
                                                         @Param("kind") String kind,
                                                         @Param("responsibilityCause") String responsibilityCause,
                                                         @Param("statementLineId") Long statementLineId,
                                                         @Param("createdFrom") LocalDateTime createdFrom,
                                                         @Param("createdTo") LocalDateTime createdTo,
                                                         @Param("limit") int limit,
                                                         @Param("offset") int offset);

    /** {@link #adminListAdjustments} 的總筆數（述詞逐字相同）。 */
    @Select("""
            <script>
            SELECT COUNT(*) FROM logistics_adjustments
            <where>
              <if test="storeId != null">AND store_id = #{storeId}</if>
              <if test="receivableId != null">AND receivable_id = #{receivableId}</if>
              <if test="kind != null">AND kind = #{kind}</if>
              <if test="responsibilityCause != null">AND responsibility_cause = #{responsibilityCause}</if>
              <if test="statementLineId != null">AND statement_line_id = #{statementLineId}</if>
              <if test="createdFrom != null">AND created_at &gt;= #{createdFrom}</if>
              <if test="createdTo != null">AND created_at &lt;= #{createdTo}</if>
            </where>
            </script>
            """)
    long adminCountAdjustments(@Param("storeId") Long storeId,
                               @Param("receivableId") Long receivableId,
                               @Param("kind") String kind,
                               @Param("responsibilityCause") String responsibilityCause,
                               @Param("statementLineId") Long statementLineId,
                               @Param("createdFrom") LocalDateTime createdFrom,
                               @Param("createdTo") LocalDateTime createdTo);

    /**
     * Provider 帳單彙總（一份帳單一列）。
     *
     * <p>🔴 {@code mismatch_count} / {@code mismatch_amount} 是<b>推導值</b>：V9 的
     * {@code chk_provsettle_recon} 只有四個值，<b>沒有 MISMATCH</b>。「金額對不上」的落點是
     * {@code kind='RECONCILIATION'} 的調整單，所以這兩欄由 {@code logistics_adjustments}
     * 推出來，而不是在 CHECK 或 CodeMeta 裡新增第五個碼值。
     *
     * <p>⚠️ 四個 {@code recon_status} 字面值一律由呼叫端傳入（{@code #{}}），SQL 不持有碼值。
     */
    @Select("""
            <script>
            SELECT l.provider                                                     AS provider,
                   l.statement_ref                                                AS statement_ref,
                   MIN(l.statement_period)                                        AS statement_period,
                   COUNT(*)                                                       AS line_count,
                   COALESCE(SUM(l.amount), 0)                                     AS total_amount,
                   COUNT(*) FILTER (WHERE l.recon_status = #{matched})            AS matched_count,
                   COUNT(*) FILTER (WHERE l.recon_status = #{unmatched})          AS unmatched_count,
                   COUNT(*) FILTER (WHERE l.recon_status = #{disputed})           AS disputed_count,
                   COUNT(*) FILTER (WHERE l.recon_status = #{writtenOff})         AS written_off_count,
                   COUNT(*) FILTER (WHERE m.adj_count &gt; 0)                     AS mismatch_count,
                   COALESCE(SUM(m.adj_amount), 0)                                 AS mismatch_amount,
                   MIN(l.imported_at)                                             AS first_imported_at,
                   MAX(l.imported_at)                                             AS last_imported_at
            FROM provider_settlement_lines l
            LEFT JOIN LATERAL (
              SELECT COUNT(*) AS adj_count, COALESCE(SUM(a.amount), 0) AS adj_amount
              FROM logistics_adjustments a
              WHERE a.statement_line_id = l.id AND a.kind = #{reconciliationKind}
            ) m ON TRUE
            <where>
              <if test="provider != null">AND l.provider = #{provider}</if>
              <if test="statementPeriod != null">AND l.statement_period = #{statementPeriod}</if>
            </where>
            GROUP BY l.provider, l.statement_ref
            <if test="unsettledOnly">
              HAVING COUNT(*) FILTER (WHERE l.recon_status IN (#{unmatched}, #{disputed})) &gt; 0
            </if>
            ORDER BY MAX(l.imported_at) DESC, l.statement_ref DESC
            LIMIT #{limit} OFFSET #{offset}
            </script>
            """)
    List<StatementAggregate> adminListStatements(@Param("provider") String provider,
                                                 @Param("statementPeriod") String statementPeriod,
                                                 @Param("unsettledOnly") boolean unsettledOnly,
                                                 @Param("matched") String matched,
                                                 @Param("unmatched") String unmatched,
                                                 @Param("disputed") String disputed,
                                                 @Param("writtenOff") String writtenOff,
                                                 @Param("reconciliationKind") String reconciliationKind,
                                                 @Param("limit") int limit,
                                                 @Param("offset") int offset);

    /** {@link #adminListStatements} 的帳單份數（不是明細列數）。 */
    @Select("""
            <script>
            SELECT COUNT(*) FROM (
              SELECT 1
              FROM provider_settlement_lines l
              <where>
                <if test="provider != null">AND l.provider = #{provider}</if>
                <if test="statementPeriod != null">AND l.statement_period = #{statementPeriod}</if>
              </where>
              GROUP BY l.provider, l.statement_ref
              <if test="unsettledOnly">
                HAVING COUNT(*) FILTER (WHERE l.recon_status IN (#{unmatched}, #{disputed})) &gt; 0
              </if>
            ) t
            </script>
            """)
    long adminCountStatements(@Param("provider") String provider,
                              @Param("statementPeriod") String statementPeriod,
                              @Param("unsettledOnly") boolean unsettledOnly,
                              @Param("unmatched") String unmatched,
                              @Param("disputed") String disputed);

    /** 單份帳單的彙總（詳情頁表頭）；找不到回 null。 */
    @Select("""
            SELECT l.provider                                                 AS provider,
                   l.statement_ref                                            AS statement_ref,
                   MIN(l.statement_period)                                    AS statement_period,
                   COUNT(*)                                                   AS line_count,
                   COALESCE(SUM(l.amount), 0)                                 AS total_amount,
                   COUNT(*) FILTER (WHERE l.recon_status = #{matched})        AS matched_count,
                   COUNT(*) FILTER (WHERE l.recon_status = #{unmatched})      AS unmatched_count,
                   COUNT(*) FILTER (WHERE l.recon_status = #{disputed})       AS disputed_count,
                   COUNT(*) FILTER (WHERE l.recon_status = #{writtenOff})     AS written_off_count,
                   COUNT(*) FILTER (WHERE m.adj_count > 0)                    AS mismatch_count,
                   COALESCE(SUM(m.adj_amount), 0)                             AS mismatch_amount,
                   MIN(l.imported_at)                                         AS first_imported_at,
                   MAX(l.imported_at)                                         AS last_imported_at
            FROM provider_settlement_lines l
            LEFT JOIN LATERAL (
              SELECT COUNT(*) AS adj_count, COALESCE(SUM(a.amount), 0) AS adj_amount
              FROM logistics_adjustments a
              WHERE a.statement_line_id = l.id AND a.kind = #{reconciliationKind}
            ) m ON TRUE
            WHERE l.provider = #{provider} AND l.statement_ref = #{statementRef}
            GROUP BY l.provider, l.statement_ref
            """)
    StatementAggregate adminGetStatement(@Param("provider") String provider,
                                         @Param("statementRef") String statementRef,
                                         @Param("matched") String matched,
                                         @Param("unmatched") String unmatched,
                                         @Param("disputed") String disputed,
                                         @Param("writtenOff") String writtenOff,
                                         @Param("reconciliationKind") String reconciliationKind);

    /** 一份帳單的全部明細列。 */
    @Select("""
            SELECT * FROM provider_settlement_lines
            WHERE provider = #{provider} AND statement_ref = #{statementRef}
            ORDER BY id
            """)
    List<ProviderSettlementLineEntity> adminListStatementLines(@Param("provider") String provider,
                                                               @Param("statementRef") String statementRef);

    /** 一批帳單明細列各自的對帳調整推導值（mismatch 的單列形式）。 */
    @Select("""
            <script>
            SELECT l.id                              AS line_id,
                   COALESCE(m.adj_count, 0)          AS adjustment_count,
                   COALESCE(m.adj_amount, 0)         AS adjustment_amount
            FROM provider_settlement_lines l
            LEFT JOIN LATERAL (
              SELECT COUNT(*) AS adj_count, COALESCE(SUM(a.amount), 0) AS adj_amount
              FROM logistics_adjustments a
              WHERE a.statement_line_id = l.id AND a.kind = #{reconciliationKind}
            ) m ON TRUE
            WHERE l.id IN
              <foreach collection="lineIds" item="id" open="(" separator="," close=")">#{id}</foreach>
            ORDER BY l.id
            </script>
            """)
    List<LineReconciliationDelta> lineReconciliationDeltas(@Param("lineIds") List<Long> lineIds,
                                                           @Param("reconciliationKind") String reconciliationKind);

    /**
     * 一份帳單牽動的例外案件中，尚有待覆核提案的件數（W6-4 的 manual review 格）。
     *
     * <p>⚠️ 關聯路徑是 {@code provider_settlement_lines.exception_case_id} → 案件 → 提案。
     * 沒有這個關聯的爭議列不會被算進來，那是正確的：它們還沒開案。
     */
    @Select("""
            SELECT COUNT(*)
            FROM fulfillment_case_resolutions fcr
            WHERE fcr.status = #{awaitingReview}
              AND fcr.case_id IN (
                SELECT l.exception_case_id
                FROM provider_settlement_lines l
                WHERE l.provider = #{provider}
                  AND l.statement_ref = #{statementRef}
                  AND l.exception_case_id IS NOT NULL
              )
            """)
    long adminCountStatementAwaitingReview(@Param("provider") String provider,
                                           @Param("statementRef") String statementRef,
                                           @Param("awaitingReview") String awaitingReview);

    /**
     * 🔴 例外案件與處置提案的查詢<b>不在本檔</b>，在
     * {@code FulfillmentExceptionCaseMapper.adminListExceptionCases} 與
     * {@code FulfillmentCaseResolutionMapper.resolutionsByCaseIds}。
     *
     * <p>理由是機械的而不是風格的：那兩個 entity 帶 JSONB TypeHandler，取回時必須掛
     * {@code @ResultMap("mybatis-plus_XxxEntity")}，而那個 resultMap 是 mybatis-plus 在
     * <b>該 entity 的 BaseMapper 命名空間</b>下註冊的。本檔不繼承 BaseMapper ⇒ 短名稱在這裡
     * 解析不到，寫了會在啟動時炸 {@code IncompleteElementException}。放回它們自己的 mapper
     * 是唯一不必寫死跨命名空間全名的做法。
     */
}

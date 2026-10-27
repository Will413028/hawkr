package io.github.will413028.hawkr.payment.logistics.application;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.CommerceModel;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.domain.LogisticsAmounts;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentCaseResolutionRepository;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LedgerRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsFinanceQueryRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.SellerLogisticsAccountRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentCaseResolutionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerEntryEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerTransactionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.AdjustmentKindTotal;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.CollectionAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.LineReconciliationDelta;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableStatusCount;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.StatementAggregate;
import java.time.Duration;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

/**
 * 物流財務的<b>唯讀</b>讀取模型（W6-3 賣家面 ＋ W6-4 Admin 面）。
 *
 * <h2>🔴 本服務不寫任何東西</h2>
 * 應收只能由送達事件建立、狀態只能由 scheduler 以 CAS 推進、調整單只能由對帳流程開立。
 * 在查詢層開寫入入口等於開一條繞過 idempotency key 與 lease 的第二路徑（CLAUDE.md §22）。
 *
 * <h2>🔴 十一個維度各自是獨立欄位，不合併也不互相抵銷</h2>
 * 最容易犯的兩個合併，各自的後果寫在欄位旁：
 * <ul>
 *   <li><b>應收餘額 × 貸記餘額</b>合成淨額 ⇒ ADR-008 第 1 條「永久可追蹤」失效，
 *       而帳面上看起來一切正常（兩者可以同時存在：這期欠、上期多收）。</li>
 *   <li><b>平台補貼 × 賣家補貼</b>合成一個「補貼」⇒ 賣家替買家出的運費會去抵掉賣家欠平台的
 *       物流成本，而賣家出那筆錢並不會讓 provider 少收平台一毛錢（W4-4）。</li>
 * </ul>
 *
 * <h2>🔴 Seller Subsidy 在本服務<b>沒有來源</b>，所以讀取模型裡也沒有那個欄位</h2>
 * V9 的九張表對它零命中（它在 {@link LogisticsReconciliationService.TrueUpInput} 是 true-up 的
 * <b>瞬時輸入</b>，由對帳呼叫端帶進來，不落庫）。這裡刻意<b>不</b>放一個恆為 0 的欄位：
 * 放了就會讓「查不到」長得跟「真的是 0」一模一樣，而兩者的後續處置完全不同。
 * 對外契約以 proto 的 {@code optional} 表達「未設」，缺口因此是顯性的。
 *
 * <h2>賣家範圍</h2>
 * SQL 端以 {@code store_id = #{storeId}} 過濾（{@code LogisticsFinanceQueryMapper}），
 * 本服務再做一次<b>收斂點</b>比對：任何回來的列若不屬於呼叫端的 store，一律丟例外而不是
 * 過濾掉。兩道方向一致、不互相取代——過濾掉會讓「述詞壞了」變成靜默的少列，
 * 而那正是最難發現的一種。
 */
@Service
public class LogisticsFinanceQueryService {

    /**
     * {@code provider_settlement_lines.recon_status} 的第四個值。
     *
     * <p>⚠️ 前三個在 {@link LogisticsReconciliationService} 已有具名常數，這裡<b>引用</b>它們
     * 而不是重打一份。第四個（核銷）是本波第一次用到，所以定在這裡。
     * 誠實標註同那一段：這四個值住在 V9 的 {@code chk_provsettle_recon}，<b>沒有</b> CodeMeta
     * enum；收斂進 CodeMeta 是跨 repo 動作（四份前端拷貝），不在本波範圍。
     */
    static final String RECON_WRITTEN_OFF = "WRITTEN_OFF";

    /**
     * {@code logistics_adjustments.kind} 的另一個值（責任歸屬）。
     *
     * <p>🔴 與 {@link LogisticsReconciliationService#ADJ_KIND_RECONCILIATION} 是<b>兩個維度</b>
     * （7/11 與 8/11），彙總時分開列：一個是「這筆錢算誰的」、一個是「帳單與我方暫估差多少」。
     */
    static final String ADJ_KIND_RESPONSIBILITY = "RESPONSIBILITY";

    /** {@code fulfillment_case_resolutions.status}（V9 {@code chk_fcr_status}）＝ manual review。 */
    static final String RESOLUTION_STATUS_AWAITING_REVIEW = "AWAITING_REVIEW";

    /** 分頁上限。超過一律夾到這個值，不讓呼叫端用 page_size 拖垮查詢。 */
    static final int MAX_PAGE_SIZE = 100;

    private final LogisticsFinanceQueryRepository query;
    private final LogisticsReceivableRepository receivables;
    private final SellerLogisticsAccountRepository accounts;
    private final LedgerRepository ledger;
    private final FulfillmentExceptionCaseRepository cases;
    private final FulfillmentCaseResolutionRepository resolutions;
    private final Supplier<LocalDateTime> nowUtc;

    @Autowired
    public LogisticsFinanceQueryService(LogisticsFinanceQueryRepository query,
                                        LogisticsReceivableRepository receivables,
                                        SellerLogisticsAccountRepository accounts,
                                        LedgerRepository ledger,
                                        FulfillmentExceptionCaseRepository cases,
                                        FulfillmentCaseResolutionRepository resolutions) {
        this(query, receivables, accounts, ledger, cases, resolutions, DateTimeUtils::nowUtc);
    }

    /** 測試用：注入時鐘（10 檔 §21，帳齡是時間敏感邏輯，不得靠本機時鐘）。 */
    public LogisticsFinanceQueryService(LogisticsFinanceQueryRepository query,
                                        LogisticsReceivableRepository receivables,
                                        SellerLogisticsAccountRepository accounts,
                                        LedgerRepository ledger,
                                        FulfillmentExceptionCaseRepository cases,
                                        FulfillmentCaseResolutionRepository resolutions,
                                        Supplier<LocalDateTime> nowUtc) {
        this.query = query;
        this.receivables = receivables;
        this.accounts = accounts;
        this.ledger = ledger;
        this.cases = cases;
        this.resolutions = resolutions;
        this.nowUtc = nowUtc;
    }

    // =========================================================================
    // 讀取模型
    // =========================================================================

    /** 分頁結果。 */
    public record Paged<T>(List<T> records, long total, int page, int pageSize) {

        public static <T> Paged<T> of(List<T> records, long total, int page, int pageSize) {
            return new Paged<>(records == null ? List.of() : records, total, page, pageSize);
        }
    }

    /** 一個應收狀態的筆數（維度 9/11）。 */
    public record StatusCount(LogisticsReceivableStatus status, long count) {
    }

    /**
     * W6-3「Logistics Finance」的十一個維度（彙總）。
     *
     * <p>🔴 <b>本 record 刻意沒有 {@code sellerSubsidy} 欄位</b>——見類別註解。
     * 🔴 <b>{@code sellerCredit} 與 {@code receivableBalance} 是兩個欄位</b>，永遠不得合成淨額。
     *
     * @param actualProviderCost  只含<b>已對帳</b>的；缺口由 {@code unreconciledCount} 表達
     * @param payuniCollectionFee 只含<b>已查到 FundDate</b> 的；缺口由 {@code pendingFeeQueryCount} 表達
     * @param collectionResidual  可正可負（正 ＝ 未收足、負 ＝ 多收）
     * @param hasAccount          false ⇒ 兩個餘額是 0，但那是「還沒有帳戶」不是「結清了」
     */
    public record SellerFinanceSummary(long storeId,
                                       LocalDateTime createdFrom,
                                       LocalDateTime createdTo,
                                       long estimatedProviderCost,
                                       long actualProviderCost,
                                       long unreconciledCount,
                                       long sellerLogisticsCharge,
                                       long payuniCollectionFee,
                                       long pendingFeeQueryCount,
                                       long platformSubsidy,
                                       long responsibilityAdjustment,
                                       long reconciliationAdjustment,
                                       List<StatusCount> collectionStatusCounts,
                                       long collectionResidual,
                                       long sellerCredit,
                                       long receivableBalance,
                                       LocalDateTime lastActivityAt,
                                       long agingDays,
                                       boolean hasAccount) {
    }

    /**
     * 單筆應收的十一個維度。
     *
     * @param latestAttempt 可為 {@code null}（還沒送出過）。維度 4／10 取自它，
     *                      而它的 {@code actualFee}／{@code residual} 為 {@code null} 代表
     *                      <b>尚未查詢</b>——🔴 不是 0
     */
    public record SellerReceivableView(LogisticsReceivableEntity receivable,
                                       LogisticsCollectionAttemptEntity latestAttempt,
                                       long responsibilityAdjustment,
                                       long reconciliationAdjustment) {

        /** 維度 11/11：本筆產生的貸記本金（{@code residual < 0} 時 ＝ |residual|，否則 0）。 */
        public long sellerCredit() {
            if (latestAttempt == null || latestAttempt.getResidual() == null) {
                return 0L;
            }
            return LogisticsAmounts.creditFromResidual(latestAttempt.getResidual());
        }
    }

    /** 帳本表頭 ＋ 它的全部分錄。 */
    public record LedgerTxnView(LedgerTransactionEntity txn, List<LedgerEntryEntity> entries) {
    }

    /** Admin 應收詳情：應收 ＋ 全部收款嘗試 ＋ 調整單 ＋ 該賣家的帳戶餘額。 */
    public record AdminReceivableDetail(LogisticsReceivableEntity receivable,
                                        List<LogisticsCollectionAttemptEntity> collections,
                                        List<LogisticsAdjustmentEntity> adjustments,
                                        SellerLogisticsAccountEntity account) {
    }

    /** 一列帳單明細 ＋ 它的 mismatch 推導值。 */
    public record StatementLineView(ProviderSettlementLineEntity line,
                                    long reconciliationAdjustmentCount,
                                    long reconciliationAdjustmentAmount) {
    }

    /** Admin 帳單詳情：彙總 ＋ 全部明細列 ＋ 待覆核件數（manual review）。 */
    public record AdminStatementDetail(StatementAggregate statement,
                                       List<StatementLineView> lines,
                                       long awaitingReviewCount) {
    }

    /** Admin 例外案件 ＋ 它的處置提案（manual review 的載體）。 */
    public record AdminCaseView(FulfillmentExceptionCaseEntity exceptionCase,
                                List<FulfillmentCaseResolutionEntity> resolutions) {
    }

    // =========================================================================
    // 賣家面
    // =========================================================================

    /**
     * 賣家物流財務總覽。
     *
     * @param storeId Gateway 的 sellerid resolver 解出的<b>現任 store_id</b>。
     *                🔴 &lt;= 0 一律丟例外：Gateway 接線錯誤要 fail loud，不得塌成空結果
     *                ——空結果看起來就像「這家店沒有物流費」
     */
    public SellerFinanceSummary sellerSummary(long storeId, LocalDateTime from, LocalDateTime to) {
        requireStoreScope(storeId);

        ReceivableAggregate r = query.sellerReceivableAggregate(storeId, from, to);
        CollectionAggregate c = query.sellerCollectionAggregate(storeId, from, to);
        Map<String, Long> adjByKind = totalsByKind(query.sellerAdjustmentTotals(storeId, from, to));
        List<StatusCount> statusCounts = toStatusCounts(query.sellerStatusCounts(storeId, from, to));

        Optional<SellerLogisticsAccountEntity> account = accounts.find(storeId);
        long credit = account.map(SellerLogisticsAccountEntity::getCreditBalance).orElse(0L);
        long receivableBalance = account.map(SellerLogisticsAccountEntity::getReceivableBalance).orElse(0L);
        LocalDateTime lastActivityAt = account.map(SellerLogisticsAccountEntity::getLastActivityAt).orElse(null);

        return new SellerFinanceSummary(
                storeId, from, to,
                r.estimatedCostTotal(),
                r.actualCostTotal(),
                r.unreconciledCount(),
                r.targetNetTotal(),
                c.actualFeeTotal(),
                c.pendingFeeQueryCount(),
                r.platformSubsidyTotal(),
                adjByKind.getOrDefault(ADJ_KIND_RESPONSIBILITY, 0L),
                adjByKind.getOrDefault(LogisticsReconciliationService.ADJ_KIND_RECONCILIATION, 0L),
                statusCounts,
                c.residualTotal(),
                credit,
                receivableBalance,
                lastActivityAt,
                daysSince(lastActivityAt),
                account.isPresent());
    }

    /** 賣家自己的應收明細（分頁）。 */
    public Paged<SellerReceivableView> sellerReceivables(long storeId,
                                                         List<LogisticsReceivableStatus> statuses,
                                                         LocalDateTime from, LocalDateTime to,
                                                         int page, int pageSize) {
        requireStoreScope(storeId);
        int size = clampPageSize(pageSize);
        int pageNo = Math.max(page, 1);
        List<String> statusValues = statuses == null ? List.of()
                : statuses.stream().map(LogisticsReceivableStatus::value).toList();

        List<LogisticsReceivableEntity> rows =
                query.sellerListReceivables(storeId, statusValues, from, to, size, (pageNo - 1) * size);
        assertAllOwnedBy(storeId, rows);

        long total = query.sellerCountReceivables(storeId, statusValues, from, to);
        return Paged.of(enrich(rows), total, pageNo, size);
    }

    /**
     * 賣家自己的物流帳本流水。
     *
     * <p>🔴 只列 {@code SELLER_DIRECT} 的帳：放寬 {@code chk_ledgtxn_model} 之後，舊商流的
     * dormant 帳本與新商流共用同一張表，不帶條件會把兩套帳混算。
     */
    public Paged<LedgerTxnView> sellerLedger(long storeId, int page, int pageSize) {
        requireStoreScope(storeId);
        int size = clampPageSize(pageSize);
        int pageNo = Math.max(page, 1);
        String model = CommerceModel.SELLER_DIRECT.value();

        List<LedgerTransactionEntity> txns = query.sellerListLedgerTxns(storeId, model, size, (pageNo - 1) * size);
        List<LedgerTxnView> views = new ArrayList<>(txns.size());
        for (LedgerTransactionEntity txn : txns) {
            if (txn.getStoreId() == null || txn.getStoreId() != storeId) {
                throw new IllegalStateException(
                        "logistics: 帳本查詢回了不屬於呼叫端 store 的列（storeId=" + storeId
                                + ", txnStoreId=" + txn.getStoreId() + "）——查詢述詞的 store 範圍壞了");
            }
            views.add(new LedgerTxnView(txn, ledger.listEntries(txn.getId())));
        }
        return Paged.of(views, query.sellerCountLedgerTxns(storeId, model), pageNo, size);
    }

    // =========================================================================
    // Admin 面
    // =========================================================================

    /** Admin 應收列表。{@code minAgingDays <= 0} ＝ 不篩帳齡。 */
    public Paged<LogisticsReceivableEntity> adminReceivables(Long storeId,
                                                             List<LogisticsReceivableStatus> statuses,
                                                             String provider, String orderNo,
                                                             String providerDeliveryRef,
                                                             LocalDateTime from, LocalDateTime to,
                                                             boolean unreconciledOnly, long minAgingDays,
                                                             int page, int pageSize) {
        int size = clampPageSize(pageSize);
        int pageNo = Math.max(page, 1);
        List<String> statusValues = statuses == null ? List.of()
                : statuses.stream().map(LogisticsReceivableStatus::value).toList();
        LocalDateTime agingBefore = minAgingDays > 0 ? nowUtc.get().minusDays(minAgingDays) : null;
        List<String> terminal = terminalStatusValues();

        List<LogisticsReceivableEntity> rows = query.adminListReceivables(storeId, statusValues, provider,
                orderNo, providerDeliveryRef, from, to, unreconciledOnly, agingBefore, terminal,
                size, (pageNo - 1) * size);
        long total = query.adminCountReceivables(storeId, statusValues, provider, orderNo, providerDeliveryRef,
                from, to, unreconciledOnly, agingBefore, terminal);
        return Paged.of(rows, total, pageNo, size);
    }

    /**
     * Admin 應收詳情；找不到回 empty。
     *
     * <p>⚠️ 走 {@code LogisticsReceivableRepository.findById}（主鍵查詢）而不是拿列表查詢撈單筆：
     * 列表查詢帶 {@code ORDER BY … LIMIT}，「找不到」與「找到別筆」在它身上分不開。
     */
    public Optional<AdminReceivableDetail> adminReceivable(long receivableId) {
        return receivables.findById(receivableId).map(r -> {
            List<LogisticsCollectionAttemptEntity> attempts = query.adminListCollections(
                    receivableId, null, null, null, false, false, null, null, MAX_PAGE_SIZE, 0);
            List<LogisticsAdjustmentEntity> adjustments =
                    query.adjustmentsByReceivableIds(List.of(receivableId));
            SellerLogisticsAccountEntity account = r.getStoreId() == null
                    ? null : accounts.find(r.getStoreId()).orElse(null);
            return new AdminReceivableDetail(r, attempts, adjustments, account);
        });
    }

    /** Admin PayUni 收款嘗試列表。 */
    public Paged<LogisticsCollectionAttemptEntity> adminCollections(Long receivableId, String tradeNo,
                                                                    Long storeId, String submitStatus,
                                                                    boolean pendingFundDateOnly,
                                                                    boolean residualOnly,
                                                                    LocalDateTime from, LocalDateTime to,
                                                                    int page, int pageSize) {
        int size = clampPageSize(pageSize);
        int pageNo = Math.max(page, 1);
        List<LogisticsCollectionAttemptEntity> rows = query.adminListCollections(receivableId, tradeNo, storeId,
                submitStatus, pendingFundDateOnly, residualOnly, from, to, size, (pageNo - 1) * size);
        long total = query.adminCountCollections(receivableId, tradeNo, storeId, submitStatus,
                pendingFundDateOnly, residualOnly, from, to);
        return Paged.of(rows, total, pageNo, size);
    }

    /** Admin 調整單列表。 */
    public Paged<LogisticsAdjustmentEntity> adminAdjustments(Long storeId, Long receivableId, String kind,
                                                             String responsibilityCause, Long statementLineId,
                                                             LocalDateTime from, LocalDateTime to,
                                                             int page, int pageSize) {
        int size = clampPageSize(pageSize);
        int pageNo = Math.max(page, 1);
        List<LogisticsAdjustmentEntity> rows = query.adminListAdjustments(storeId, receivableId, kind,
                responsibilityCause, statementLineId, from, to, size, (pageNo - 1) * size);
        long total = query.adminCountAdjustments(storeId, receivableId, kind, responsibilityCause,
                statementLineId, from, to);
        return Paged.of(rows, total, pageNo, size);
    }

    /** Admin provider 帳單列表（一份帳單一列）。 */
    public Paged<StatementAggregate> adminStatements(String provider, String statementPeriod,
                                                     boolean unsettledOnly, int page, int pageSize) {
        int size = clampPageSize(pageSize);
        int pageNo = Math.max(page, 1);
        List<StatementAggregate> rows = query.adminListStatements(provider, statementPeriod, unsettledOnly,
                LogisticsReconciliationService.RECON_MATCHED,
                LogisticsReconciliationService.RECON_UNMATCHED,
                LogisticsReconciliationService.RECON_DISPUTED,
                RECON_WRITTEN_OFF,
                LogisticsReconciliationService.ADJ_KIND_RECONCILIATION,
                size, (pageNo - 1) * size);
        long total = query.adminCountStatements(provider, statementPeriod, unsettledOnly,
                LogisticsReconciliationService.RECON_UNMATCHED,
                LogisticsReconciliationService.RECON_DISPUTED);
        return Paged.of(rows, total, pageNo, size);
    }

    /**
     * Admin 單份帳單詳情。
     *
     * <p>🔴 {@code provider} 必填：{@code statement_ref} 只在同一個 provider 內唯一
     * （{@code uq_provsettle_line} 的第一欄就是 provider），不帶會跨 provider 撈到別人的帳單。
     */
    public Optional<AdminStatementDetail> adminStatement(String provider, String statementRef) {
        if (provider == null || provider.isBlank() || statementRef == null || statementRef.isBlank()) {
            throw new IllegalArgumentException("logistics: provider 與 statementRef 皆為必填");
        }
        return query.adminGetStatement(provider, statementRef,
                LogisticsReconciliationService.RECON_MATCHED,
                LogisticsReconciliationService.RECON_UNMATCHED,
                LogisticsReconciliationService.RECON_DISPUTED,
                RECON_WRITTEN_OFF,
                LogisticsReconciliationService.ADJ_KIND_RECONCILIATION)
                .map(agg -> {
                    List<ProviderSettlementLineEntity> lines =
                            query.adminListStatementLines(provider, statementRef);
                    Map<Long, LineReconciliationDelta> deltas = new HashMap<>();
                    for (LineReconciliationDelta d : query.lineReconciliationDeltas(
                            lines.stream().map(ProviderSettlementLineEntity::getId).toList(),
                            LogisticsReconciliationService.ADJ_KIND_RECONCILIATION)) {
                        deltas.put(d.lineId(), d);
                    }
                    List<StatementLineView> views = new ArrayList<>(lines.size());
                    for (ProviderSettlementLineEntity line : lines) {
                        LineReconciliationDelta d = deltas.get(line.getId());
                        views.add(new StatementLineView(line,
                                d == null ? 0L : d.adjustmentCount(),
                                d == null ? 0L : d.adjustmentAmount()));
                    }
                    long awaiting = query.adminCountStatementAwaitingReview(provider, statementRef,
                            RESOLUTION_STATUS_AWAITING_REVIEW);
                    return new AdminStatementDetail(agg, views, awaiting);
                });
    }

    /** Admin 例外案件佇列（含處置提案）。 */
    public Paged<AdminCaseView> adminExceptionCases(List<String> statuses, String severity, String category,
                                                    String subjectType, String subjectRef, Long storeId,
                                                    boolean awaitingReviewOnly, int page, int pageSize) {
        int size = clampPageSize(pageSize);
        int pageNo = Math.max(page, 1);
        List<FulfillmentExceptionCaseEntity> rows = cases.listForAdmin(statuses, severity, category,
                subjectType, subjectRef, storeId, awaitingReviewOnly, RESOLUTION_STATUS_AWAITING_REVIEW,
                size, (pageNo - 1) * size);
        long total = cases.countForAdmin(statuses, severity, category, subjectType, subjectRef, storeId,
                awaitingReviewOnly, RESOLUTION_STATUS_AWAITING_REVIEW);

        Map<Long, List<FulfillmentCaseResolutionEntity>> byCase = new LinkedHashMap<>();
        for (FulfillmentCaseResolutionEntity r : resolutions.listByCaseIds(
                rows.stream().map(FulfillmentExceptionCaseEntity::getId).toList())) {
            byCase.computeIfAbsent(r.getCaseId(), k -> new ArrayList<>()).add(r);
        }
        List<AdminCaseView> views = new ArrayList<>(rows.size());
        for (FulfillmentExceptionCaseEntity c : rows) {
            views.add(new AdminCaseView(c, byCase.getOrDefault(c.getId(), List.of())));
        }
        return Paged.of(views, total, pageNo, size);
    }

    /**
     * Admin 單一例外案件詳情（65 §6.1.1 A-2）。
     *
     * <p>🔴 <b>查無案件回 {@code null} 而不是空的 {@code AdminCaseView}</b>：呼叫端要能把
     * 「沒有這張案件」映成 {@code 9570}，而一個欄位全空的 view 會被前端渲染成一張
     * 空白案件，看起來像資料壞了而不是「打錯單號」。
     *
     * <p>⚠️ 提案清單為空是<b>正常的</b>（案件剛建、還沒有人提案），不得因此判定查無。
     */
    public AdminCaseView adminExceptionCaseByNo(String caseNo) {
        return cases.findByCaseNo(caseNo)
                .map(c -> new AdminCaseView(c, resolutions.listByCaseIds(List.of(c.getId()))))
                .orElse(null);
    }

    // =========================================================================
    // 內部
    // =========================================================================

    /**
     * 🔴 Gateway 契約：賣家面必須帶已解析的 store_id。
     *
     * <p>塌成空結果是**錯的**處置：空結果看起來就像「這家店沒有物流費」，而真相是接線壞了。
     */
    private static void requireStoreScope(long storeId) {
        if (storeId <= 0L) {
            throw new IllegalArgumentException(
                    "logistics: 賣家面查詢必須帶 Gateway 解析出的 store_id，got " + storeId);
        }
    }

    /**
     * 收斂點比對：任何回來的列都必須屬於呼叫端的 store。
     *
     * <p>🔴 <b>丟例外而不是過濾掉</b>：過濾掉會讓「SQL 的 store 述詞壞了」變成靜默的少列／多列，
     * 而那是最難發現的一種。這一道與 SQL 的 {@code store_id = #{storeId}} 方向一致、
     * 不互相取代。
     */
    private static void assertAllOwnedBy(long storeId, List<LogisticsReceivableEntity> rows) {
        for (LogisticsReceivableEntity row : rows) {
            if (row.getStoreId() == null || row.getStoreId() != storeId) {
                throw new IllegalStateException(
                        "logistics: 賣家查詢回了不屬於呼叫端 store 的應收（storeId=" + storeId
                                + ", rowStoreId=" + row.getStoreId() + "）——查詢述詞的 store 範圍壞了");
            }
        }
    }

    private List<SellerReceivableView> enrich(List<LogisticsReceivableEntity> rows) {
        if (rows.isEmpty()) {
            return List.of();
        }
        List<Long> ids = rows.stream().map(LogisticsReceivableEntity::getId).toList();

        Map<Long, LogisticsCollectionAttemptEntity> latest = new HashMap<>();
        for (LogisticsCollectionAttemptEntity a : query.latestAttemptsByReceivableIds(ids)) {
            latest.put(a.getReceivableId(), a);
        }

        Map<Long, long[]> adjByReceivable = new HashMap<>();
        for (LogisticsAdjustmentEntity adj : query.adjustmentsByReceivableIds(ids)) {
            long[] pair = adjByReceivable.computeIfAbsent(adj.getReceivableId(), k -> new long[2]);
            // [0] = RESPONSIBILITY（維度 7/11）；[1] = RECONCILIATION（維度 8/11）。
            // 🔴 兩格分開累加：合併之後就再也分不出「帳單與暫估差多少」。
            if (ADJ_KIND_RESPONSIBILITY.equals(adj.getKind())) {
                pair[0] = Math.addExact(pair[0], adj.getAmount());
            } else if (LogisticsReconciliationService.ADJ_KIND_RECONCILIATION.equals(adj.getKind())) {
                pair[1] = Math.addExact(pair[1], adj.getAmount());
            }
        }

        List<SellerReceivableView> views = new ArrayList<>(rows.size());
        for (LogisticsReceivableEntity r : rows) {
            long[] pair = adjByReceivable.getOrDefault(r.getId(), new long[2]);
            views.add(new SellerReceivableView(r, latest.get(r.getId()), pair[0], pair[1]));
        }
        return views;
    }

    private static Map<String, Long> totalsByKind(List<AdjustmentKindTotal> totals) {
        Map<String, Long> m = new HashMap<>();
        for (AdjustmentKindTotal t : totals) {
            m.put(t.kind(), t.amountTotal());
        }
        return m;
    }

    /**
     * 把 SQL 回來的狀態字串換成 CodeMeta enum。
     *
     * <p>🔴 認不出來的值一律丟例外（FAIL），<b>不是</b>靜默略過（SKIP）：DB 裡出現不在 CodeMeta
     * 的狀態值，代表有人繞過 CHECK 或 CodeMeta 與 V9 漂移了，那必須現形。
     */
    private static List<StatusCount> toStatusCounts(List<ReceivableStatusCount> raw) {
        List<StatusCount> out = new ArrayList<>(raw.size());
        for (ReceivableStatusCount c : raw) {
            // ⚠️ 先 isValidValue 再 fromValue：後者對未知值丟 IllegalArgumentException，
            //    那個訊息（"Unknown value: X"）指不出「這是 CodeMeta 與 V9 漂移」這件事。
            if (!LogisticsReceivableStatus.isValidValue(c.status())) {
                throw new IllegalStateException(
                        "logistics: DB 出現不在 CodeMeta 的應收狀態值 " + c.status()
                                + " —— CodeMeta 與 V9 的 chk_logrecv_status 漂移了");
            }
            out.add(new StatusCount(LogisticsReceivableStatus.fromValue(c.status()), c.count()));
        }
        return out;
    }

    private static List<String> terminalStatusValues() {
        List<String> terminal = new ArrayList<>();
        for (LogisticsReceivableStatus s : LogisticsReceivableStatus.values()) {
            if (ReceivableStatusRules.isTerminal(s)) {
                terminal.add(s.value());
            }
        }
        return List.copyOf(terminal);
    }

    /**
     * 單筆應收的帳齡天數（W6-4 的 overdue 格）。
     *
     * <p>🔴 <b>終態一律回 0</b>：CLOSED／WRITTEN_OFF 的應收不是逾期，只是老。把它們算進帳齡
     * 會讓 ADM 的逾期清單塞滿早就處理完的列，而真正該看的那幾筆被淹掉。
     * 終態判定走 {@link ReceivableStatusRules#isTerminal}（程式側唯一權威），不在這裡另寫一份。
     */
    public long agingDaysOf(LogisticsReceivableEntity receivable) {
        if (receivable == null || receivable.getStatus() == null) {
            return 0L;
        }
        // ⚠️ 認不出來的狀態一律當成**非終態**（帳齡照算）：讓它出現在逾期清單裡，
        //    比讓它從清單上消失好——後者是靜默的。
        if (LogisticsReceivableStatus.isValidValue(receivable.getStatus())
                && ReceivableStatusRules.isTerminal(
                        LogisticsReceivableStatus.fromValue(receivable.getStatus()))) {
            return 0L;
        }
        return daysSince(receivable.getCreatedAt());
    }

    /** 賣家帳戶的帳齡天數（錨是 {@code last_activity_at}，不是「餘額為正」）。 */
    public long agingDaysOfAccount(SellerLogisticsAccountEntity account) {
        return account == null ? 0L : daysSince(account.getLastActivityAt());
    }

    private long daysSince(LocalDateTime since) {
        if (since == null) {
            return 0L;
        }
        long days = Duration.between(since, nowUtc.get()).toDays();
        return Math.max(days, 0L);
    }

    private static int clampPageSize(int pageSize) {
        if (pageSize <= 0) {
            return 20;
        }
        return Math.min(pageSize, MAX_PAGE_SIZE);
    }
}

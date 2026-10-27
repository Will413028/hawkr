package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerTransactionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsFinanceQueryMapper;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.AdjustmentKindTotal;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.CollectionAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.LineReconciliationDelta;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableAggregate;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.ReceivableStatusCount;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.StatementAggregate;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * 物流財務唯讀查詢的 repository（thin adapter）。
 *
 * <p>🔴 <b>本類別沒有任何寫入方法，而且不得新增。</b> 應收只能由送達事件建立、狀態只能由
 * scheduler 以 CAS 推進——在查詢層開一個寫入入口，等於開一條繞過 idempotency key 與 lease
 * 的第二路徑（CLAUDE.md §22）。
 *
 * <p>⚠️ 空集合一律在這裡就地回空，不讓它變成 {@code IN ()} 的語法錯誤（同既有 repository
 * 的既有處置）。
 */
@Repository
public class LogisticsFinanceQueryRepository {

    private final LogisticsFinanceQueryMapper mapper;

    public LogisticsFinanceQueryRepository(LogisticsFinanceQueryMapper mapper) {
        this.mapper = mapper;
    }

    // ── 賣家面 ───────────────────────────────────────────────────────────────

    public ReceivableAggregate sellerReceivableAggregate(long storeId, LocalDateTime from, LocalDateTime to) {
        ReceivableAggregate agg = mapper.sellerReceivableAggregate(storeId, from, to);
        return agg == null ? ReceivableAggregate.empty() : agg;
    }

    public CollectionAggregate sellerCollectionAggregate(long storeId, LocalDateTime from, LocalDateTime to) {
        CollectionAggregate agg = mapper.sellerCollectionAggregate(storeId, from, to);
        return agg == null ? CollectionAggregate.empty() : agg;
    }

    public List<AdjustmentKindTotal> sellerAdjustmentTotals(long storeId, LocalDateTime from, LocalDateTime to) {
        return mapper.sellerAdjustmentTotals(storeId, from, to);
    }

    public List<ReceivableStatusCount> sellerStatusCounts(long storeId, LocalDateTime from, LocalDateTime to) {
        return mapper.sellerStatusCounts(storeId, from, to);
    }

    public List<LogisticsReceivableEntity> sellerListReceivables(long storeId, List<String> statuses,
                                                                 LocalDateTime from, LocalDateTime to,
                                                                 int limit, int offset) {
        return mapper.sellerListReceivables(storeId, statuses, from, to, limit, offset);
    }

    public long sellerCountReceivables(long storeId, List<String> statuses,
                                       LocalDateTime from, LocalDateTime to) {
        return mapper.sellerCountReceivables(storeId, statuses, from, to);
    }

    public List<LedgerTransactionEntity> sellerListLedgerTxns(long storeId, String commerceModel,
                                                              int limit, int offset) {
        return mapper.sellerListLedgerTxns(storeId, commerceModel, limit, offset);
    }

    public long sellerCountLedgerTxns(long storeId, String commerceModel) {
        return mapper.sellerCountLedgerTxns(storeId, commerceModel);
    }

    // ── 共用 ─────────────────────────────────────────────────────────────────

    public List<LogisticsCollectionAttemptEntity> latestAttemptsByReceivableIds(List<Long> receivableIds) {
        if (receivableIds == null || receivableIds.isEmpty()) {
            return List.of();
        }
        return mapper.latestAttemptsByReceivableIds(receivableIds);
    }

    public List<LogisticsAdjustmentEntity> adjustmentsByReceivableIds(List<Long> receivableIds) {
        if (receivableIds == null || receivableIds.isEmpty()) {
            return List.of();
        }
        return mapper.adjustmentsByReceivableIds(receivableIds);
    }

    // ── Admin 面 ─────────────────────────────────────────────────────────────

    public List<LogisticsReceivableEntity> adminListReceivables(Long storeId, List<String> statuses,
                                                                String provider, String orderNo,
                                                                String providerDeliveryRef,
                                                                LocalDateTime from, LocalDateTime to,
                                                                boolean unreconciledOnly,
                                                                LocalDateTime agingBefore,
                                                                List<String> terminalStatuses,
                                                                int limit, int offset) {
        return mapper.adminListReceivables(storeId, statuses, provider, orderNo, providerDeliveryRef,
                from, to, unreconciledOnly, agingBefore, terminalStatuses, limit, offset);
    }

    public long adminCountReceivables(Long storeId, List<String> statuses, String provider, String orderNo,
                                      String providerDeliveryRef, LocalDateTime from, LocalDateTime to,
                                      boolean unreconciledOnly, LocalDateTime agingBefore,
                                      List<String> terminalStatuses) {
        return mapper.adminCountReceivables(storeId, statuses, provider, orderNo, providerDeliveryRef,
                from, to, unreconciledOnly, agingBefore, terminalStatuses);
    }

    public List<LogisticsCollectionAttemptEntity> adminListCollections(Long receivableId, String tradeNo,
                                                                       Long storeId, String submitStatus,
                                                                       boolean pendingFundDateOnly,
                                                                       boolean residualOnly,
                                                                       LocalDateTime from, LocalDateTime to,
                                                                       int limit, int offset) {
        return mapper.adminListCollections(receivableId, tradeNo, storeId, submitStatus,
                pendingFundDateOnly, residualOnly, from, to, limit, offset);
    }

    public long adminCountCollections(Long receivableId, String tradeNo, Long storeId, String submitStatus,
                                      boolean pendingFundDateOnly, boolean residualOnly,
                                      LocalDateTime from, LocalDateTime to) {
        return mapper.adminCountCollections(receivableId, tradeNo, storeId, submitStatus,
                pendingFundDateOnly, residualOnly, from, to);
    }

    public List<LogisticsAdjustmentEntity> adminListAdjustments(Long storeId, Long receivableId, String kind,
                                                                String responsibilityCause, Long statementLineId,
                                                                LocalDateTime from, LocalDateTime to,
                                                                int limit, int offset) {
        return mapper.adminListAdjustments(storeId, receivableId, kind, responsibilityCause,
                statementLineId, from, to, limit, offset);
    }

    public long adminCountAdjustments(Long storeId, Long receivableId, String kind, String responsibilityCause,
                                      Long statementLineId, LocalDateTime from, LocalDateTime to) {
        return mapper.adminCountAdjustments(storeId, receivableId, kind, responsibilityCause,
                statementLineId, from, to);
    }

    public List<StatementAggregate> adminListStatements(String provider, String statementPeriod,
                                                        boolean unsettledOnly, String matched, String unmatched,
                                                        String disputed, String writtenOff,
                                                        String reconciliationKind, int limit, int offset) {
        return mapper.adminListStatements(provider, statementPeriod, unsettledOnly, matched, unmatched,
                disputed, writtenOff, reconciliationKind, limit, offset);
    }

    public long adminCountStatements(String provider, String statementPeriod, boolean unsettledOnly,
                                     String unmatched, String disputed) {
        return mapper.adminCountStatements(provider, statementPeriod, unsettledOnly, unmatched, disputed);
    }

    public Optional<StatementAggregate> adminGetStatement(String provider, String statementRef, String matched,
                                                          String unmatched, String disputed, String writtenOff,
                                                          String reconciliationKind) {
        return Optional.ofNullable(mapper.adminGetStatement(provider, statementRef, matched, unmatched,
                disputed, writtenOff, reconciliationKind));
    }

    public List<ProviderSettlementLineEntity> adminListStatementLines(String provider, String statementRef) {
        return mapper.adminListStatementLines(provider, statementRef);
    }

    public List<LineReconciliationDelta> lineReconciliationDeltas(List<Long> lineIds, String reconciliationKind) {
        if (lineIds == null || lineIds.isEmpty()) {
            return List.of();
        }
        return mapper.lineReconciliationDeltas(lineIds, reconciliationKind);
    }

    public long adminCountStatementAwaitingReview(String provider, String statementRef, String awaitingReview) {
        return mapper.adminCountStatementAwaitingReview(provider, statementRef, awaitingReview);
    }
}

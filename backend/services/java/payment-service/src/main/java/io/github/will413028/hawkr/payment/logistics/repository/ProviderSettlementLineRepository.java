package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.ProviderSettlementLineMapper;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * provider 帳單明細的 repository（thin adapter）。
 */
@Repository
public class ProviderSettlementLineRepository {

    private final ProviderSettlementLineMapper mapper;

    public ProviderSettlementLineRepository(ProviderSettlementLineMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * 匯入一列。
     *
     * <p>⚠️ 去重只對 {@code providerShipmentRef} 非 NULL 的明細列成立——{@code uq_provsettle_line}
     * 含該欄，而 PostgreSQL 的 NULL 互不相等 ⇒ <b>彙總列（該欄為 NULL）不會被去重</b>。
     * 彙總列的重複匯入必須由呼叫端在匯入層擋。
     *
     * @return {@code true} = 新建；{@code false} = 已匯入過
     */
    public boolean importIfAbsent(ProviderSettlementLineEntity entity) {
        return mapper.insertIfAbsent(entity) == 1;
    }

    public Optional<ProviderSettlementLineEntity> findById(long id) {
        return Optional.ofNullable(mapper.findById(id));
    }

    public List<ProviderSettlementLineEntity> listByStatement(String provider, String statementPeriod) {
        return mapper.listByStatement(provider, statementPeriod);
    }

    /**
     * 一份帳單（{@code provider} ＋ {@code statement_ref}）的全部明細列。
     *
     * <p>🔴 匯入層用它判斷「這份帳單匯過了沒」：{@code uq_provsettle_line} 含
     * {@code provider_shipment_ref}，而 PostgreSQL 的 NULL 互不相等 ⇒
     * <b>彙總列（該欄為 NULL）不會被那個 UNIQUE 去重</b>，重複匯入會產生重複列。
     */
    public List<ProviderSettlementLineEntity> listByStatementRef(String provider, String statementRef) {
        return mapper.listByStatementRef(provider, statementRef);
    }

    public List<ProviderSettlementLineEntity> listByReconStatus(List<String> statuses, int limit) {
        if (statuses == null || statuses.isEmpty()) {
            return List.of();
        }
        return mapper.listByReconStatus(statuses, limit);
    }

    /**
     * 比對成功（CAS）。
     *
     * <p>🔴 {@code recon_status = MATCHED} ⇔ {@code ledger_txn_no} 非 NULL 是<b>雙向</b>等式，
     * 兩者同一句寫。
     */
    public boolean markMatched(long id, String matchedStatus, String ledgerTxnNo, List<String> allowedFrom) {
        return mapper.markMatched(id, matchedStatus, ledgerTxnNo, allowedFrom) == 1;
    }

    /** 轉爭議／核銷（CAS）；🔴 同時清空 {@code ledger_txn_no} 以滿足同一條雙向等式。 */
    public boolean markUnmatchedOutcome(long id, String toStatus, List<String> allowedFrom) {
        return mapper.markUnmatchedOutcome(id, toStatus, allowedFrom) == 1;
    }

    public boolean bindExceptionCase(long id, long exceptionCaseId) {
        return mapper.bindExceptionCase(id, exceptionCaseId) == 1;
    }
}

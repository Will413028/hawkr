package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsReceivableMapper;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * 賣家物流應收的 repository（thin adapter）。
 *
 * <p>它只做三件事：把 mapper 的 {@code null} 包成 {@link Optional}、把 {@code status} 字串換成
 * CodeMeta enum、把租約／CAS 的語意寫進方法名。<b>不放任何業務規則</b>——規則在
 * {@link ReceivableStatusRules} 與 {@code LogisticsAmounts}。
 *
 * <p>🔴 <b>本 repository 刻意沒有「建立應收」以外的寫入入口</b>能繞過 idempotency key：
 * 應收只能由送達事件建立（ADR-003 應收只在送達完成成立），開一條無 key 的建立路徑
 * 等於開一條繞過那個前提的路。
 */
@Repository
public class LogisticsReceivableRepository {

    private final LogisticsReceivableMapper mapper;

    public LogisticsReceivableRepository(LogisticsReceivableMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * 建立應收（以 {@code idempotency_key} 去重）。
     *
     * @return {@code true} = 這次真的建了；{@code false} = 送達事件重放，已存在。
     *         🔴 {@code false} <b>不是錯誤</b>，呼叫端據此 ACK 而不是進 DLQ。
     */
    public boolean createIfAbsent(LogisticsReceivableEntity entity) {
        return mapper.insertIfAbsent(entity) == 1;
    }

    public Optional<LogisticsReceivableEntity> findById(long id) {
        return Optional.ofNullable(mapper.findById(id));
    }

    public Optional<LogisticsReceivableEntity> findByIdempotencyKey(String idempotencyKey) {
        return Optional.ofNullable(mapper.findByIdempotencyKey(idempotencyKey));
    }

    public List<LogisticsReceivableEntity> listByStore(long storeId, int limit, int offset) {
        return mapper.listByStore(storeId, limit, offset);
    }

    public long countByStore(long storeId) {
        return mapper.countByStore(storeId);
    }

    /** 依狀態列出（ADM 例外佇列用）。 */
    public List<LogisticsReceivableEntity> listByStatus(List<LogisticsReceivableStatus> statuses, int limit) {
        if (statuses == null || statuses.isEmpty()) {
            // 🔴 空集合 ⇒ 直接回空，不讓它變成 `status IN ()` 這種語法錯誤。
            return List.of();
        }
        return mapper.listByStatus(statuses.stream().map(LogisticsReceivableStatus::value).toList(), limit);
    }

    /**
     * CAS 推進狀態；allowed-from 由 {@link ReceivableStatusRules} 決定。
     *
     * @return {@code true} = 推進成功；{@code false} = 這筆已經不在允許的來源狀態。
     *         🔴 呼叫端<b>不得</b>把 {@code false} 讀成「別人先做過了」就靜默放行。
     */
    public boolean transition(long id, LogisticsReceivableStatus to) {
        return mapper.transitionStatus(id, to) == 1;
    }

    /**
     * 🔴 以租約搶佔待送出的應收（10 檔 §29）。
     *
     * <p>租約長度必須同時滿足兩個界：
     * <ul>
     *   <li><b>下界</b>：&gt; 單筆最壞耗時（含 PayUni {@code agent_fee} 呼叫的 deadline）。
     *       低於它 ⇒ 持有者還在打 PayUni 就被合法搶走 ⇒ 重複收款。</li>
     *   <li><b>上界</b>：&lt; 候選窗口的剩餘壽命。等於窗口 ⇒ lease 退化成永久旗標。</li>
     * </ul>
     * 兩個界都由呼叫端（worker）依它自己的 deadline 決定，repository 不猜。
     */
    public List<LogisticsReceivableEntity> claimPendingForSubmit(int leaseSeconds, int batchSize) {
        return mapper.claimPendingForSubmit(leaseSeconds, batchSize);
    }

    /** 提前歸還租約。⚠️ 最佳化，不是正確性來源。 */
    public void releaseSubmitLease(long id) {
        mapper.releaseSubmitLease(id);
    }

    /**
     * 對帳回填（{@code actual_cost} ＋ {@code statement_line_id} 同時寫）。
     *
     * @return {@code true} = 回填成功；{@code false} = 已經對過帳
     */
    public boolean applyTrueUp(long id, long actualCost, long statementLineId) {
        return mapper.applyTrueUp(id, actualCost, statementLineId) == 1;
    }

    /**
     * 以對帳關聯鍵（{@code provider_delivery_ref}）找應收。
     *
     * <p>🔴 回 List 是因為該欄<b>沒有</b> UNIQUE：命中 0 筆＝帳單有我方無紀錄的費用，
     * 命中多筆＝關聯鍵不唯一。兩者都必須由呼叫端處置，<b>不得</b>在這裡取「第一筆」。
     */
    public List<LogisticsReceivableEntity> findByProviderDeliveryRef(String providerDeliveryRef) {
        if (providerDeliveryRef == null || providerDeliveryRef.isBlank()) {
            // 🔴 空值不得變成「全表掃描後取第一筆」：彙總列的 provider_shipment_ref 可以是 NULL。
            return List.of();
        }
        return mapper.findByProviderDeliveryRef(providerDeliveryRef);
    }

    /**
     * 寫入責任成因的 audit snapshot（一次性，V15）。
     *
     * @return {@code true} ＝ 這次真的寫進去了；{@code false} ＝ 該列已有成因
     *         （呼叫端必須讀回比對：值相同 ＝ 重投；值不同 ＝ 衝突，不得靜默放行）
     */
    public boolean snapshotResponsibilityCause(long id, String responsibilityCause) {
        return mapper.snapshotResponsibilityCause(id, responsibilityCause) == 1;
    }

    /** @return {@code true} = 綁定成功；{@code false} = 已綁過別的憑證 */
    public boolean bindInvoiceDocument(long id, long invoiceDocumentId) {
        return mapper.bindInvoiceDocument(id, invoiceDocumentId) == 1;
    }
}

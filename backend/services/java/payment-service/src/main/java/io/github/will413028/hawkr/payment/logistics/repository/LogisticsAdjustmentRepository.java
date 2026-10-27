package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsAdjustmentMapper;
import java.util.List;
import org.springframework.stereotype.Repository;

/**
 * 責任／對帳調整單的 repository（thin adapter）。
 *
 * <p>🔴 <b>只有新增與查詢，沒有修改</b>：調整單是「這筆錢憑什麼動」的憑據，寫錯就再開一筆
 * 反向的，不改原列。
 */
@Repository
public class LogisticsAdjustmentRepository {

    private final LogisticsAdjustmentMapper mapper;

    public LogisticsAdjustmentRepository(LogisticsAdjustmentMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * 建立調整單。
     *
     * <p>⚠️ {@code receivableId} 可為 {@code null}：未送達的情形不建 receivable（ADR-003），
     * 但 provider 的取消費仍然發生 ⇒ 掛在 {@code storeId} ＋ {@code providerDeliveryRef} 上。
     * ⚠️ {@code amount} 允許負數，但<b>不得為 0</b>（{@code chk_logadj_amount_nonzero}）。
     */
    public void create(LogisticsAdjustmentEntity entity) {
        mapper.insertAdjustment(entity);
    }

    public List<LogisticsAdjustmentEntity> listByReceivable(long receivableId) {
        return mapper.listByReceivable(receivableId);
    }

    public List<LogisticsAdjustmentEntity> listByStore(long storeId, int limit, int offset) {
        return mapper.listByStore(storeId, limit, offset);
    }

    public List<LogisticsAdjustmentEntity> listByStatementLine(long statementLineId) {
        return mapper.listByStatementLine(statementLineId);
    }
}

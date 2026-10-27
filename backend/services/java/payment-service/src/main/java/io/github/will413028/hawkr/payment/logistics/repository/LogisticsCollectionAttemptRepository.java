package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsCollectionAttemptMapper;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * {@code agent_fee} 送出紀錄的 repository（thin adapter）。
 */
@Repository
public class LogisticsCollectionAttemptRepository {

    private final LogisticsCollectionAttemptMapper mapper;

    public LogisticsCollectionAttemptRepository(LogisticsCollectionAttemptMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * 🔴 pre-commit：<b>送出前必須先落地，才呼叫 PayUni</b>。
     *
     * @param grossAmount pre-commit 當下還不知道實際要送多少（gross-up 在 adapter），
     *                    先放目標淨額當佔位值，拿到回應後由
     *                    {@link #markSubmitted} 改成實際送出的金額
     */
    public void preCommit(long id, long receivableId, String tradeNo, long grossAmount) {
        mapper.insertPreCommit(id, receivableId, tradeNo, grossAmount);
    }

    /**
     * 寫回受理結果。
     *
     * @param grossAmount 🔴 <b>adapter 實際送出</b>的金額，不是 pre-commit 的佔位值——
     *                    {@code agent_fee_query} 的 key 是 {@code (MerID, TradeNo, Amt)} 三元組
     */
    public boolean markSubmitted(long id, String submitStatus, String submitMessage,
                                 long grossAmount, LocalDateTime submittedAt, LocalDate expectedFundDate) {
        return mapper.markSubmitted(id, submitStatus, submitMessage, grossAmount,
                submittedAt, expectedFundDate) == 1;
    }

    /** 🔴 以租約搶佔待查詢的 attempt（10 檔 §29），不是「已查過」旗標。 */
    public List<LogisticsCollectionAttemptEntity> claimForQuery(int leaseSeconds, int batchSize) {
        return mapper.claimForQuery(leaseSeconds, batchSize);
    }

    /**
     * 寫回查詢結果並歸還租約。
     *
     * @param netProceeds 必須 ＝ {@code grossAmount − actualFee}（{@code chk_logcoll_net} 同式）
     * @param residual    {@code target_net − net_proceeds}，可為負
     * @return {@code false} = 已經有 {@code fund_date}（另一個持有者先寫了）
     */
    public boolean recordQueryResult(long id, LocalDate fundDate, long actualFee,
                                     long netProceeds, long residual) {
        return mapper.recordQueryResult(id, fundDate, actualFee, netProceeds, residual) == 1;
    }

    /** 提前歸還查詢租約。⚠️ 最佳化，不是正確性來源。 */
    public void releaseQueryLease(long id) {
        mapper.releaseQueryLease(id);
    }

    public List<LogisticsCollectionAttemptEntity> listByReceivable(long receivableId) {
        return mapper.listByReceivable(receivableId);
    }

    public Optional<LogisticsCollectionAttemptEntity> findLatestByReceivable(long receivableId) {
        return Optional.ofNullable(mapper.findLatestByReceivable(receivableId));
    }

    public Optional<LogisticsCollectionAttemptEntity> findByTradeNo(String tradeNo) {
        return Optional.ofNullable(mapper.findByTradeNo(tradeNo));
    }
}

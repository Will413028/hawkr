package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentCaseResolutionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.FulfillmentCaseResolutionMapper;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * 例外案件處置提案／覆核的 repository（thin adapter）。
 */
@Repository
public class FulfillmentCaseResolutionRepository {

    private final FulfillmentCaseResolutionMapper mapper;

    public FulfillmentCaseResolutionRepository(FulfillmentCaseResolutionMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * 建立提案；同一張案件同時只能有一個待覆核提案。
     *
     * @return {@code true} = 建立成功；{@code false} = 該案件已有待覆核提案
     */
    public boolean proposeIfNonePending(FulfillmentCaseResolutionEntity entity) {
        return mapper.insertIfNoPendingProposal(entity) == 1;
    }

    public Optional<FulfillmentCaseResolutionEntity> findById(long id) {
        return Optional.ofNullable(mapper.findById(id));
    }

    /**
     * 覆核前讀回 maker 的提案，<b>帶列鎖</b>（V13）。
     *
     * <p>🔴 覆核必須用<b>提案列上持久化的成因</b>結案，而不是覆核者當下給的值——
     * 後者讓 {@code chk_fcr_two_person} 退化成「有兩個人簽過名」，
     * 而不是「第二個人批准了第一個人提的那一件事」。
     *
     * <p>⚠️ 鎖序固定 <b>case → resolution</b>，與其他路徑一致，避免 deadlock。
     */
    public Optional<FulfillmentCaseResolutionEntity> findByIdForUpdate(long id) {
        return Optional.ofNullable(mapper.findByIdForUpdate(id));
    }

    public List<FulfillmentCaseResolutionEntity> listByCase(long caseId) {
        return mapper.listByCase(caseId);
    }

    /**
     * 覆核決議（CAS）。
     *
     * <p>🔴 覆核者不得為提案者：DB 那道（{@code chk_fcr_two_person}）會丟 23514 ⇒ 呼叫端拿到 500；
     * 本方法的述詞讓同一件事變成 {@code false} ⇒ 呼叫端可以回一個有意義的業務碼。
     * <b>兩道方向一致，不互相取代。</b>
     *
     * @return {@code false} = 已不在 {@code fromStatus}，<b>或</b>覆核者就是提案者
     */
    public boolean decide(long id, String fromStatus, String toStatus, long reviewedByAccountId,
                          String reviewComment, LocalDateTime reviewedAt) {
        return mapper.decide(id, fromStatus, toStatus, reviewedByAccountId, reviewComment, reviewedAt) == 1;
    }

    /**
     * <b>責任判定</b>提案的覆核決議（CAS，V12）。
     *
     * <p>述詞比 {@link #decide} 多兩個：{@code case_id}（提案必須屬於這張案件）與
     * {@code resolution_type}（36 §3.3 N16 的 Atomic Boundary ① 逐字要求）。
     * 少了後者，非責任型別的提案也會被推成 APPROVED ⇒ N16 的「唯一產生點」不再唯一。
     *
     * @return {@code false} = 已不在 {@code fromStatus}、提案不屬於該案件、型別不符、
     *         或覆核者就是提案者。🔴 <b>不得</b>讀成「別人先做過了」就往下走發事件
     */
    public boolean decideResponsibility(long id, long caseId, String fromStatus, String toStatus,
                                        String resolutionType, long reviewedByAccountId,
                                        String reviewComment, LocalDateTime reviewedAt) {
        return mapper.decideResponsibility(id, caseId, fromStatus, toStatus, resolutionType,
                reviewedByAccountId, reviewComment, reviewedAt) == 1;
    }

    /**
     * 一批案件各自的處置提案（Admin 佇列頁一次帶回，避免 N+1）。
     *
     * <p>空輸入就地回空，不讓它變成 {@code IN ()} 的語法錯誤。
     */
    public List<FulfillmentCaseResolutionEntity> listByCaseIds(List<Long> caseIds) {
        if (caseIds == null || caseIds.isEmpty()) {
            return List.of();
        }
        return mapper.resolutionsByCaseIds(caseIds);
    }
}

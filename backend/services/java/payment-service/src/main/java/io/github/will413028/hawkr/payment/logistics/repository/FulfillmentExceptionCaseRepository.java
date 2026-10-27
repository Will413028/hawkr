package io.github.will413028.hawkr.payment.logistics.repository;

import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.FulfillmentExceptionCaseMapper;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import org.springframework.stereotype.Repository;

/**
 * 維運例外案件的 repository（thin adapter）。
 *
 * <p>⚠️ <b>誠實標註的缺口</b>：本表的狀態機目前沒有對應的 {@code *Rules} helper
 * （{@code ReceivableStatusRules} 只管應收那一條軸），所以 allowed-from 由呼叫端傳入。
 * CAS 保證「不會從錯的狀態跳過去」，但「哪些邊合法」在程式側還沒有唯一權威——
 * 那份權威出現之前，這裡不憑空造第二個碼表。
 */
@Repository
public class FulfillmentExceptionCaseRepository {

    private final FulfillmentExceptionCaseMapper mapper;

    public FulfillmentExceptionCaseRepository(FulfillmentExceptionCaseMapper mapper) {
        this.mapper = mapper;
    }

    /** {@code 'EX' + yyyyMMdd + 8 位流水}。 */
    public String nextCaseNo() {
        return mapper.nextCaseNo();
    }

    /**
     * 開案，以 {@code dedup_key} 去重（只在未結案的案件之間去重）。
     *
     * @return {@code true} = 這次真的開了；{@code false} = 已有未結案的同 key 案件
     */
    public boolean openIfAbsent(FulfillmentExceptionCaseEntity entity) {
        return mapper.insertIfNoOpenDuplicate(entity) == 1;
    }

    public Optional<FulfillmentExceptionCaseEntity> findById(long id) {
        return Optional.ofNullable(mapper.findById(id));
    }

    /**
     * 建立<b>責任判定</b>案件（V12）。同一 fulfillment leg 終其一生只能有一張。
     *
     * <p>🔴 走的是<b>不指名 conflict target</b> 的那一支（{@code insertResponsibilityCaseIfAbsent}）：
     * 責任判定案件同時受兩條唯一約束拘束，指名 dedup_key 的那一支撞到 lifetime UNIQUE 時會拋
     * 23505 並讓整個交易 aborted ⇒ 建案 500。理由全文見 mapper 的 javadoc（65 §5.5.2 E0 G4）。
     *
     * @return {@code true} = 這次真的開了（<b>只有這一次</b>才可以 INSERT N17 outbox）；
     *         {@code false} = 該 leg 已有責任案件（含已結案者）⇒ 不得發第二筆事件
     */
    public boolean openResponsibilityCaseIfAbsent(FulfillmentExceptionCaseEntity entity) {
        return mapper.insertResponsibilityCaseIfAbsent(entity) == 1;
    }

    /**
     * 取一列並鎖住，序列化同一張案件上的提案與定案。
     *
     * <p>🔴 <b>必須在交易內呼叫</b>：不在交易內時 PostgreSQL 會在語句結束時立刻釋放鎖 ⇒
     * 序列化保證消失而沒有任何錯誤。呼叫端以 {@code @Transactional} 保證。
     */
    /**
     * 依對外業務單號取案件（ADM 詳情與 maker-checker 入口）。
     *
     * <p>🔴 無鎖快照，理由見 {@code FulfillmentExceptionCaseMapper.findByCaseNo} 的 Javadoc：
     * 鎖必須留在交易內，不能跨越一次讀 order 的 gRPC。
     */
    public Optional<FulfillmentExceptionCaseEntity> findByCaseNo(String caseNo) {
        return Optional.ofNullable(mapper.findByCaseNo(caseNo));
    }

    public Optional<FulfillmentExceptionCaseEntity> findByIdForUpdate(long id) {
        return Optional.ofNullable(mapper.findByIdForUpdate(id));
    }

    /**
     * 依 subject 取責任判定案件（lifetime 只會有一張）。
     *
     * <p>{@code ON CONFLICT DO NOTHING} 不回傳衝突的那一列 ⇒ 建案回 {@code false} 之後
     * 要拿到既有案件只能再查一次。
     */
    public Optional<FulfillmentExceptionCaseEntity> findResponsibilityCase(String category,
                                                                          String subjectType,
                                                                          String subjectRef) {
        return Optional.ofNullable(mapper.findResponsibilityCase(category, subjectType, subjectRef));
    }

    public Optional<FulfillmentExceptionCaseEntity> findOpenByDedupKey(String dedupKey,
                                                                       List<String> openStatuses) {
        if (openStatuses == null || openStatuses.isEmpty()) {
            return Optional.empty();
        }
        return Optional.ofNullable(mapper.findOpenByDedupKey(dedupKey, openStatuses));
    }

    public List<FulfillmentExceptionCaseEntity> listQueue(List<String> statuses, int limit) {
        if (statuses == null || statuses.isEmpty()) {
            return List.of();
        }
        return mapper.listQueue(statuses, limit);
    }

    /**
     * 指派承辦人並推進狀態（CAS）。
     *
     * <p>🔴 {@code chk_exc_owner}：離開 {@code OPEN} ⇒ 必有承辦人與指派時間，三件同一句寫。
     *
     * @return {@code false} = 已不在 allowed-from（<b>不得</b>讀成「別人先做過了」就放行）
     */
    public boolean assignOwner(long id, String toStatus, long ownerAccountId, List<String> allowedFrom) {
        return mapper.assignOwner(id, toStatus, ownerAccountId, allowedFrom) == 1;
    }

    /**
     * 結案（CAS）。
     *
     * <p>🔴 {@code chk_exc_resolved}：結案三件套必須同時有值。
     */
    public boolean resolve(long id, String toStatus, String resolutionCode, String resolutionNote,
                           long resolvedByAccountId, LocalDateTime resolvedAt, List<String> allowedFrom) {
        return mapper.resolve(id, toStatus, resolutionCode, resolutionNote,
                resolvedByAccountId, resolvedAt, allowedFrom) == 1;
    }

    /**
     * Admin 例外案件佇列（分頁 ＋ 篩選；W6-4 的 manual review 格走 {@code awaitingReviewOnly}）。
     *
     * <p>⚠️ {@code statuses} 為空時<b>不篩狀態</b>（列全部），與 {@link #listQueue} 的
     * 「空集合回空」相反——兩者的語意不同：那一支是「取這幾個狀態的佇列」，這一支是
     * 「Admin 的總表，狀態是可選篩選條件」。
     */
    public List<FulfillmentExceptionCaseEntity> listForAdmin(List<String> statuses, String severity,
                                                             String category, String subjectType,
                                                             String subjectRef, Long storeId,
                                                             boolean awaitingReviewOnly, String awaitingReview,
                                                             int limit, int offset) {
        return mapper.adminListExceptionCases(statuses, severity, category, subjectType, subjectRef,
                storeId, awaitingReviewOnly, awaitingReview, limit, offset);
    }

    /** {@link #listForAdmin} 的總筆數。 */
    public long countForAdmin(List<String> statuses, String severity, String category, String subjectType,
                              String subjectRef, Long storeId, boolean awaitingReviewOnly,
                              String awaitingReview) {
        return mapper.adminCountExceptionCases(statuses, severity, category, subjectType, subjectRef,
                storeId, awaitingReviewOnly, awaitingReview);
    }
}

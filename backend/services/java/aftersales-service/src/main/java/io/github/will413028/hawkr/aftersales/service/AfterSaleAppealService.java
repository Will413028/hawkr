package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleAppealEventEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSalePlatformDecisionEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSalePlatformDecisionMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealEventType;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.AfterSaleResolutionType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import lombok.Data;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Set;

/**
 * §10 申訴服務。
 * 負責：
 * - 買家提交申訴（submitAppeal）
 * - 買家 / 賣家查詢裁決進度（getArbitration / getSellerArbitration）
 * - Admin 待裁決列表（listAppealsForAdmin）
 *
 * Admin 實際裁決請走 {@link PlatformDecisionService}。
 */
@Service
@RequiredArgsConstructor
public class AfterSaleAppealService {

    private static final ObsLogger log = ObsLogger.forClass(AfterSaleAppealService.class);

    /** Admin 申訴列表專屬 filter 關鍵字：不限 appeal_state（含已裁決）；非 CodeMeta 值。 */
    private static final String APPEAL_STATE_FILTER_ALL = "all";

    /**
     * Admin 申訴列表 appeal_state filter 的合法集合（AfterSaleAppealState Value）。
     * 不含 NONE('0')——申訴列表以 {@code appeal_state <> '0'} 為前提。
     */
    private static final Set<String> APPEAL_STATE_FILTER_VALUES = Set.of(
            AfterSaleAppealState.APPEALED.value(),
            AfterSaleAppealState.INVESTIGATING.value(),
            AfterSaleAppealState.RESOLVED.value());

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleCaseService caseService;
    private final AfterSaleAppealEventMapper appealEventMapper;
    private final AfterSalePlatformDecisionMapper platformDecisionMapper;
    private final AfterSaleResolutionMapper resolutionMapper;
    private final EvidenceService evidenceService;
    private final AfterSalesEventPublisher eventPublisher;
    private final AfterSalesProperties properties;

    /**
     * 買家提交申訴（§10.8.1）。
     * 流程：擁有權驗證 → <b>持鎖讀</b> → 資格判定(A1/A2/A3) → Media finalize →
     * 樂觀鎖 UPDATE → INSERT events → publish MQ → commit（鎖於此釋放）。
     *
     * <h3>backend#3262：為什麼持鎖必須在資格判定之前</h3>
     * 撥款閘門（{@link AfterSalePayoutHoldService}）要回答「這張訂單還會不會再生出退款」。
     * 它讀不到本交易未 commit 的申訴（MVCC），所以需要一個「有申訴正在途中」的訊號。
     * 舊版用 {@code payoutHoldGraceMinutes}（15 分）的<b>長度</b>去掩蓋這個窗口，
     * 前提是「本交易有已知的最壞耗時」——而那個前提是假的。
     *
     * <p><b>A3 通過 → commit 之間的同步依賴（獨立 QA FAIL 後逐項盤點，非只有 QA 指出的那一支）</b>：
     * <table border="1">
     *   <caption>有界性盤點</caption>
     *   <tr><th>#</th><th>依賴</th><th>有上界嗎</th></tr>
     *   <tr><td>1</td><td>{@code mediaGrpcClient.finalizeImages}（有附照片時）</td>
     *       <td>✅ 本次補上 {@code MEDIA_DEADLINE_SEC}（原無）</td></tr>
     *   <tr><td>2</td><td>{@code mediaGrpcClient.linkImages}（同上）</td>
     *       <td>✅ 本次補上（原無）</td></tr>
     *   <tr><td>3</td><td>{@code evidenceMapper.insert} × N</td>
     *       <td>❌ 全 repo 無 {@code statement_timeout} / {@code @Transactional(timeout)}</td></tr>
     *   <tr><td>4</td><td>{@link AfterSaleCaseMapper#markAppealed}</td><td>❌ 同上</td></tr>
     *   <tr><td>5</td><td>{@code appealEventMapper.insert}</td><td>❌ 同上</td></tr>
     *   <tr><td>6</td><td>{@code eventPublisher} 內的 seller/buyer principal 反解 gRPC</td>
     *       <td>✅ 兩支都有 {@code withDeadlineAfter}</td></tr>
     *   <tr><td>7</td><td>{@code rabbitTemplate.convertAndSend}（domain event ＋ 賣家 realtime）</td>
     *       <td>❌ 無顯式 timeout</td></tr>
     *   <tr><td>8</td><td>COMMIT 本身</td><td>❌</td></tr>
     *   <tr><td>9</td><td>JVM STW / pod 被重新排程</td><td>❌ 本質不可界</td></tr>
     * </table>
     *
     * <p>⇒ 3、5、7、8、9 補不出有限上界，所以<b>「grace 大於交易耗時」這條證明永遠建立不起來</b>
     * （獨立 QA 以 Testcontainers 坐實：deadline 過了 28 小時，{@code markAppealed} 仍回 {@code UPDATE 1}）。
     *
     * <p><b>改用順序保證</b>：在做任何資格判定之前先 {@code SELECT … FOR UPDATE} 鎖住該 case，
     * 鎖持有到 commit/rollback。撥款閘門在放行前以 {@code FOR UPDATE SKIP LOCKED} 探測同一列
     * ＋用重讀到的那一列重跑判定 ⇒ 「撥款放行」與「一筆已通過 A3 的申訴落地」不可能同時成立，
     * 而且<b>不依賴上表任何一格的耗時</b>。
     *
     * <p>⚠️ 因此上表剩下的 ❌ <b>不再是資金安全問題</b>，只影響「撥款最多被推遲多久」
     * ——日後在本交易內新增同步依賴時，該問的是「會不會讓賣家等太久」，不是「會不會誤撥」。
     *
     * <p>⚠️ 順序不可調換的兩個理由：
     * <ul>
     *   <li>先判定再上鎖 ⇒ 判定通過到上鎖之間仍有窗口，閘門探不到 ⇒ P0 復活</li>
     *   <li>先做 media 再上鎖<b>重檢</b> ⇒ 交易自己跑得慢就把買家合法取得的申訴權利打掉
     *       ——方向固定為「平台多等一下」，不是「買家少一點權利」</li>
     * </ul>
     * 兩者各由 {@code AfterSaleAppealLockOrderTest} 的順序斷言守著。
     *
     * @return 提交後的 caseNo（= appealId）
     */
    @Transactional
    public String submitAppeal(Long buyerId, String caseNo, String reason,
                                String description, List<String> evidencePhotoTokens) {
        // 擁有權驗證（非持鎖讀；只決定「這張單是不是你的」，不決定資格）
        caseService.getCaseForBuyer(buyerId, caseNo);

        // backend#3262 P0：<b>資格判定一律在持鎖之後</b>。
        // 鎖從這一行取得、持有到 commit/rollback 為止，於是「通過 A3 的申訴」與「撥款閘門放行」
        // 之間有了順序保證而非時間保證——閘門在放行前會探測這一列（tryLockCaseForPayoutRelease），
        // 探不到就 fail-closed。理由與完整推導見 AfterSaleCaseMapper#selectByCaseNoForUpdate。
        // 順序不可調換：先檢查再上鎖 ＝ 檢查通過到上鎖之間仍有窗口；
        // 先做 media 再上鎖重檢 ＝ 交易自己跑得慢就把買家合法取得的申訴權利打掉（禁止）。
        AfterSaleCaseEntity caseEntity = caseMapper.selectByCaseNoForUpdate(caseNo);
        if (caseEntity == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_FOUND);
        }

        // A1：前置狀態 = REJECTED 或 AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)
        // 2026-06-15 起 status=33 開放申訴（待寄回逾時自動關案後買家不服）
        if (!isAppealableStatus(caseEntity.getStatus())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_APPEAL_NOT_APPEALABLE);
        }

        // A2：appealState = NONE（一案一申訴）
        if (!AfterSaleAppealState.NONE.value().equals(nvl(caseEntity.getAppealState()))) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_APPEAL_ALREADY_SUBMITTED);
        }

        // A3：申訴期限檢查（依前置 status 分流：60→last_rejected_at+72h；33→closed_at+72h）
        // ⚠️ 這一刻起買家的申訴權利已確立：之後不論交易跑多久（media gRPC、DB、MQ、commit）
        // 都不得再以「時間過了」為由拒絕它——那是把後端的慢轉嫁成買家的權利損失。
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime deadline = computeAppealDeadline(caseEntity);
        if (deadline == null || now.isAfter(deadline)) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_APPEAL_DEADLINE_EXCEEDED);
        }

        // 證據照片數量上限
        int maxEvidence = properties.getAppeal().getMaxEvidenceCount();
        int tokenCount = evidencePhotoTokens == null ? 0 : evidencePhotoTokens.size();
        if (tokenCount > maxEvidence) {
            throw new IllegalArgumentException(
                    String.format("申訴證據照片超過上限（最多 %d 張）", maxEvidence));
        }

        // 證據先處理（TX 外呼叫 Media，但 linkImages 要進 DB 所以實際上在 TX 內；
        // 實作上 evidenceService 會 finalize + insert 一次完成，與既有 supplementEvidence 一致）
        if (tokenCount > 0) {
            evidenceService.finalizeAndInsert(caseNo, buyerId, evidencePhotoTokens,
                    io.github.will413028.hawkr.common.utils.code.AfterSaleEvidenceType.PHOTO.value());
        }

        // 樂觀鎖 CAS（同步寫入 appeal_reason / appeal_description，§10.6.6）
        int affected = caseMapper.markAppealed(caseNo, now, reason, description);
        if (affected != 1) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_CASE_UNDER_APPEAL);
        }

        // INSERT appeal event
        AfterSaleAppealEventEntity event = new AfterSaleAppealEventEntity();
        event.setCaseNo(caseNo);
        event.setEventType(AfterSaleAppealEventType.APPEAL_SUBMITTED.value());
        event.setActorType("BUYER");
        event.setActorId(buyerId);
        event.setLabel("申訴提交");
        event.setDescription(description == null || description.isBlank()
                ? "買家提交申訴：" + nvl(reason)
                : "買家提交申訴：" + reason + "；" + description);
        appealEventMapper.insert(event);

        // publish MQ（TX 內 publish，事件 best-effort）
        eventPublisher.publishAppealSubmitted(caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(), reason);

        log.info("AfterSaleAppealService.submitAppeal.success",
                "申訴提交成功, reason=" + reason, "caseNo", caseNo);
        return caseNo;
    }

    /**
     * 買家查詢裁決（§10.8.3）。
     */
    public ArbitrationResult getArbitration(Long buyerId, String caseNo) {
        AfterSaleCaseEntity caseEntity = caseService.getCaseForBuyer(buyerId, caseNo);
        return buildArbitration(caseEntity);
    }

    /**
     * 賣家 read-only 查詢裁決。
     */
    public ArbitrationResult getSellerArbitration(Long storeId, String caseNo) {
        AfterSaleCaseEntity caseEntity = caseService.getCaseForSeller(storeId, caseNo);
        return buildArbitration(caseEntity);
    }

    private ArbitrationResult buildArbitration(AfterSaleCaseEntity caseEntity) {
        if (AfterSaleAppealState.NONE.value().equals(nvl(caseEntity.getAppealState()))) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_CASE_NOT_UNDER_APPEAL);
        }

        List<AfterSaleAppealEventEntity> events =
                appealEventMapper.selectByCaseNoOrderByCreatedAt(caseEntity.getCaseNo());

        AfterSalePlatformDecisionEntity decision = null;
        if (AfterSaleAppealState.RESOLVED.value().equals(caseEntity.getAppealState())) {
            decision = platformDecisionMapper.selectByCaseNo(caseEntity.getCaseNo());
        }

        ArbitrationResult result = new ArbitrationResult();
        result.setAppealState(caseEntity.getAppealState());
        result.setEvents(events);
        result.setDecision(decision);
        return result;
    }

    /**
     * Admin 待裁決列表。
     *
     * @param appealStateFilter AfterSaleAppealState value：'10'（預設）/ '20' / '30'，或 'all'
     */
    public AppealListResult listAppealsForAdmin(String appealStateFilter, int page, int pageSize) {
        int defaultPageSize = properties.getAppeal().getDefaultPageSize();
        int maxPageSize = properties.getAppeal().getMaxPageSize();
        int p = page <= 0 ? 1 : page;
        int ps = pageSize <= 0 ? defaultPageSize : Math.min(pageSize, maxPageSize);

        String appealStateValue = resolveAppealStateFilter(appealStateFilter);
        long total = caseMapper.countAppealsForAdmin(appealStateValue);
        List<AfterSaleCaseEntity> records = caseMapper.selectAppealsForAdmin(
                appealStateValue, ps, (p - 1) * ps);

        AppealListResult result = new AppealListResult();
        result.setRecords(records);
        result.setTotal(total);
        result.setPage(p);
        result.setPageSize(ps);
        return result;
    }

    /**
     * 解析 filter：null/空字串 → '10'（APPEALED）；'all' → null（全部非 NONE）；
     * 其餘只收 AfterSaleAppealState 的 Value，其他一律 IllegalArgumentException。
     * <p>Value 是本欄位唯一的表示法——DB 存 Value、回應欄位吐 Value、Gateway 也只放行
     * Value，請求端沒有理由用 Name（收 Name 回 Value 的不對稱正是 #2104 的成因）。
     */
    private String resolveAppealStateFilter(String filter) {
        if (filter == null || filter.isBlank()) {
            return AfterSaleAppealState.APPEALED.value();
        }
        if (APPEAL_STATE_FILTER_ALL.equals(filter)) {
            return null;
        }
        if (APPEAL_STATE_FILTER_VALUES.contains(filter)) {
            return filter;
        }
        throw new IllegalArgumentException("invalid appeal_state filter: " + filter);
    }

    /**
     * 計算 can_appeal（供 detail/summary assembler 使用）。
     * 2026-06-15：status=REJECTED(60) 與 AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33) 都可申訴。
     */
    public boolean canAppeal(AfterSaleCaseEntity caseEntity) {
        if (!isAppealableStatus(caseEntity.getStatus())) {
            return false;
        }
        if (!AfterSaleAppealState.NONE.value().equals(nvl(caseEntity.getAppealState()))) {
            return false;
        }
        LocalDateTime deadline = computeAppealDeadline(caseEntity);
        if (deadline == null) {
            return false;
        }
        return !DateTimeUtils.nowUtc().isAfter(deadline);
    }

    /**
     * 計算 appeal_deadline（供 detail assembler 使用）；非可申訴狀態回傳 null。
     * <p>2026-06-15 起依前置 status 分流：
     * <ul>
     *   <li>REJECTED(60) → last_rejected_at + appeal.deadline-hours（既有）</li>
     *   <li>AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33) → closed_at + appeal.deadline-hours（新增）</li>
     * </ul>
     */
    public LocalDateTime computeAppealDeadline(AfterSaleCaseEntity caseEntity) {
        // backend#3262：規則本體移到 AppealDeadlineRules（純函式），與撥款閘門
        // AfterSalePayoutHoldService 共用同一份——兩處各算一次的話，撥款會與買家實際
        // 能申訴的時間錯開，而那正是本票要關的洞。
        return AppealDeadlineRules.computeDeadline(caseEntity, properties.getAppeal().getDeadlineHours());
    }

    /**
     * 2026-06-15 新增：判斷某 status 是否可申訴。
     */
    private boolean isAppealableStatus(String status) {
        return AfterSaleCaseStatus.REJECTED.value().equals(status)
                || AfterSaleCaseStatus.AUTO_COMPLETED_RETURN_PENDING_TIMEOUT.value().equals(status);
    }

    private static String nvl(String s) {
        return s == null ? AfterSaleAppealState.NONE.value() : s;
    }

    /**
     * §10.6.6 Admin 單案詳情。
     *
     * <p>查詢範圍不限 appeal_state，Admin 任何 case 都可查（含非申訴案件）。
     * 組裝 case + arbitration（若有）+ admin context（最新 REJECT resolution、decision 的 admin_id）。
     * Profile（買家/賣家 name/avatar）由 gRPC Service 層另外呼叫 Buyer/Seller gRPC Client 補上，
     * 本 service 只回 domain data。</p>
     */
    public AppealDetailForAdminResult getAppealDetailForAdmin(String caseNo) {
        AfterSaleCaseEntity caseEntity = caseService.getCase(caseNo);

        AppealDetailForAdminResult result = new AppealDetailForAdminResult();
        result.setCaseEntity(caseEntity);

        // arbitration（只有申訴過的 case 才有 events / decision）
        String appealState = caseEntity.getAppealState();
        if (appealState != null && !AfterSaleAppealState.NONE.value().equals(appealState)) {
            result.setAppealEvents(appealEventMapper.selectByCaseNoOrderByCreatedAt(caseNo));
            if (AfterSaleAppealState.RESOLVED.value().equals(appealState)) {
                result.setDecision(platformDecisionMapper.selectByCaseNo(caseNo));
            }
        }

        // 最新一筆賣家 REJECT resolution（§10.6.6；reopen 已於 #1228 淘汰，單案至多一筆 REJECT）
        AfterSaleResolutionEntity latestReject = resolutionMapper.selectLatestRejectByCaseNo(
                caseNo, AfterSaleResolutionType.REJECT.value());
        result.setLatestRejectResolution(latestReject);

        return result;
    }

    // ==================== DTO ====================

    @Data
    public static class ArbitrationResult {
        private String appealState;
        private List<AfterSaleAppealEventEntity> events;
        private AfterSalePlatformDecisionEntity decision;
    }

    @Data
    public static class AppealListResult {
        private List<AfterSaleCaseEntity> records;
        private long total;
        private int page;
        private int pageSize;
    }

    /** §10.6.6 Admin 詳情 service 層結果（不含 profile，由呼叫端另外補） */
    @Data
    public static class AppealDetailForAdminResult {
        private AfterSaleCaseEntity caseEntity;
        private List<AfterSaleAppealEventEntity> appealEvents;       // 未申訴則 null
        private AfterSalePlatformDecisionEntity decision;             // 未裁決則 null
        private AfterSaleResolutionEntity latestRejectResolution;     // 無 REJECT 則 null
    }
}

package io.github.will413028.hawkr.aftersales.scheduler;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties.TimeoutPolicy;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.aftersales.service.CaseAutoRejectService;
import io.github.will413028.hawkr.aftersales.service.CaseEscalationService;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * §5.10 賣家售後處理期限自動代決 Worker。
 *
 * <p>每分鐘掃描一次，找出符合以下條件的售後單：
 * <ul>
 *   <li>{@code status IN (PENDING(0))}</li>
 *   <li>{@code appeal_state = NONE(0)}</li>
 *   <li>{@code process_deadline < NOW()}</li>
 *   <li>{@code auto_escalated = false}</li>
 * </ul>
 *
 * <p>對每筆過期 case 依 {@code aftersales.case.timeout-policy} 分流：
 * <ul>
 *   <li>{@link TimeoutPolicy#AUTO_REJECT}（**預設**，2026-04-27 起）：透過
 *       {@link CaseAutoRejectService#autoReject} 在獨立 tx 內 INSERT resolution +
 *       UPDATE case (status→REJECTED, last_rejected_at=NOW, deadline=NULL) +
 *       INSERT audit log (actor=SYSTEM) + publish {@code aftersales.case.auto_rejected}</li>
 *   <li>{@link TimeoutPolicy#AUTO_ESCALATE}（**備援**，保留不刪）：透過
 *       {@link CaseEscalationService#escalate} 在獨立 tx 內升級 appeal_state→APPEALED +
 *       auto_escalated=true + publish {@code aftersales.case.auto_escalated}</li>
 * </ul>
 *
 * <p><b>issue #2294 B2/B3</b>：publish 已從本 worker 移進兩支 service 的既有 {@code @Transactional}
 * 內（出路 4 短 tx），走會傳遞失敗的 publish 變體 —— 發不出去就整個 rollback、下一 tick 重來，
 * 不再是「狀態已終態、事件永久遺失」。
 *
 * <p>樂觀鎖：AUTO_REJECT 透過 status CAS 防 race；AUTO_ESCALATE 透過 {@code auto_escalated=false} CAS 防 race。
 * 切換策略只需修改 config 並重啟服務；歷史案件不回溯。
 */
@Component
public class CaseProcessTimeoutWorker {

    private static final ObsLogger log = ObsLogger.forClass(CaseProcessTimeoutWorker.class);

    /** 一輪掃描的最大筆數，避免單次處理過多 */
    private static final int BATCH_LIMIT = 200;

    private final AfterSaleCaseMapper caseMapper;
    private final CaseAutoRejectService autoRejectService;
    private final CaseEscalationService escalationService;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher eventPublisher;

    public CaseProcessTimeoutWorker(AfterSaleCaseMapper caseMapper,
                                     CaseAutoRejectService autoRejectService,
                                     CaseEscalationService escalationService,
                                     AfterSalesProperties properties,
                                     AfterSalesEventPublisher eventPublisher) {
        this.caseMapper = caseMapper;
        this.autoRejectService = autoRejectService;
        this.escalationService = escalationService;
        this.properties = properties;
        this.eventPublisher = eventPublisher;
    }

    /**
     * 每 60 秒掃一次過期案件。
     * 使用 fixedDelay 確保前一輪完成後才開始下一輪，避免併發堆積。
     */
    @Scheduled(fixedDelay = 60_000)
    public void scanProcessTimeouts() {
        List<AfterSaleCaseEntity> candidates;
        try {
            candidates = caseMapper.selectProcessTimeoutCandidates(BATCH_LIMIT);
        } catch (Exception e) {
            log.error("CaseProcessTimeoutWorker.scan.error", "掃描過期案件失敗", e);
            return;
        }

        if (candidates.isEmpty()) {
            return;
        }

        TimeoutPolicy policy = properties.getCaseConfig().getTimeoutPolicy();
        log.info("CaseProcessTimeoutWorker.found",
                "發現 " + candidates.size() + " 筆賣家處理逾時案件, policy=" + policy);

        for (AfterSaleCaseEntity caseEntity : candidates) {
            try {
                handleOne(caseEntity, policy);
            } catch (PrincipalResolutionFailedException e) {
                // issue #2294 P1-2：tx 尚未開始 ⇒ 沒有 CAS、沒有事件、沒有 row lock。
                // 候選述詞（status / appeal_state / process_deadline / auto_escalated）完全未觸碰，
                // 下一 tick 必然重撈同一筆——是可觀測的重試，不是靜默跳過。
                log.error("CaseProcessTimeoutWorker.resolve_principals.failed",
                        "身分反解失敗，本輪跳過（未進 tx、未 CAS），下一 tick 重試, caseNo="
                                + caseEntity.getCaseNo() + ", policy=" + policy, e);
            } catch (Exception e) {
                log.error("CaseProcessTimeoutWorker.handle.error",
                        "代決單筆案件失敗, caseNo=" + caseEntity.getCaseNo() + ", policy=" + policy, e);
            }
        }
    }

    /**
     * 單筆案件代決流程：依 policy 分流至對應 service。
     * Service 內部 tx 失敗（樂觀鎖失敗 / DB error / <b>MQ publish 失敗</b>）會 rollback；
     * issue #2294 B2/B3 起 MQ publish 已收進 service 的同一個 tx，不再是 tx 外 best-effort。
     */
    private void handleOne(AfterSaleCaseEntity caseEntity, TimeoutPolicy policy) {
        switch (policy) {
            case AUTO_REJECT -> autoRejectOne(caseEntity);
            case AUTO_ESCALATE -> escalateOne(caseEntity);
        }
    }

    /**
     * AUTO_REJECT 路徑：系統代為拒絕。
     * 對齊 §5.2.5 賣家手動 reject 流程，但 actor=SYSTEM、reject_reason=SELLER_NO_RESPONSE。
     */
    private void autoRejectOne(AfterSaleCaseEntity caseEntity) {
        String caseNo = caseEntity.getCaseNo();
        LocalDateTime now = DateTimeUtils.nowUtc();
        int hours = properties.getCaseConfig().getProcessDeadlineHours();

        // issue #2294 P1-2：身分反解（兩次無 deadline 的 gRPC）在**進 tx 之前**做完；
        // 失敗 → 拋 PrincipalResolutionFailedException → 由 scanProcessTimeouts 的 per-item
        // catch 記 error 並繼續下一筆。此時尚未 CAS，四個候選條件全部未變，下一 tick 重撈。
        EventPrincipals principals =
                eventPublisher.resolveEventPrincipals(caseEntity.getStoreId(), caseEntity.getBuyerId());

        try {
            // issue #2294 B2：CAS ＋ audit ＋ publish 全在 service 的同一個 tx 內
            autoRejectService.autoReject(caseNo, now, hours,
                    caseEntity.getOrderNo(), caseEntity.getBuyerId(), caseEntity.getStoreId(),
                    principals);
        } catch (CaseAutoRejectService.OptimisticLockLostException e) {
            log.info("CaseProcessTimeoutWorker.skip",
                    "案件已被他人處理或狀態變更, affected=0", "caseNo", caseNo);
            return;
        }

        log.info("CaseProcessTimeoutWorker.auto_rejected",
                "案件已自動拒絕, processDeadlineHours=" + hours, "caseNo", caseNo);
    }

    /**
     * AUTO_ESCALATE 路徑（備援）：升級至 admin 仲裁。保留作為可切換的備援方案。
     */
    private void escalateOne(AfterSaleCaseEntity caseEntity) {
        String caseNo = caseEntity.getCaseNo();
        LocalDateTime now = DateTimeUtils.nowUtc();

        // issue #2294 P1-2：身分反解在進 tx 之前（理由與失敗語意同 autoRejectOne）
        EventPrincipals principals =
                eventPublisher.resolveEventPrincipals(caseEntity.getStoreId(), caseEntity.getBuyerId());

        // issue #2294 B3：CAS ＋ appeal_event ＋ publish 全在 service 的同一個 tx 內
        boolean escalated = escalationService.escalate(caseNo, now,
                caseEntity.getOrderNo(), caseEntity.getBuyerId(), caseEntity.getStoreId(),
                principals);
        if (!escalated) {
            log.info("CaseProcessTimeoutWorker.skip",
                    "案件已被他人處理或狀態變更, affected=0", "caseNo", caseNo);
            return;
        }

        int hours = properties.getCaseConfig().getProcessDeadlineHours();

        log.info("CaseProcessTimeoutWorker.escalated",
                "案件已自動升級, processDeadlineHours=" + hours, "caseNo", caseNo);
    }
}

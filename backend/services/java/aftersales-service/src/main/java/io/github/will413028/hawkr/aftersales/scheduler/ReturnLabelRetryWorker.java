package io.github.will413028.hawkr.aftersales.scheduler;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.aftersales.service.AfterSaleLabelRetryService;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.Map;

/**
 * 7-11 退貨出貨便代碼重試 Worker（§5.3.7）。
 *
 * <p>兩段掃描：
 * <ul>
 *   <li>{@link #autoRetry()}：5 分鐘未回填 + 未重試 → 觸發第 2 次 triggerCvsPrint（每 2 分鐘執行）</li>
 *   <li>{@link #checkExhausted()}：已重試 1 次 + 總時間超過 10 分鐘仍未回填 → 記 LABEL_RETRY_EXHAUSTED 停手（每 10 分鐘執行）</li>
 * </ul>
 */
@Component
public class ReturnLabelRetryWorker {

    private static final ObsLogger log = ObsLogger.forClass(ReturnLabelRetryWorker.class);

    private static final int SCAN_LIMIT = 50;

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleLabelRetryService retryService;
    private final AfterSaleAuditService auditService;

    public ReturnLabelRetryWorker(AfterSaleCaseMapper caseMapper,
                                   AfterSaleLabelRetryService retryService,
                                   AfterSaleAuditService auditService) {
        this.caseMapper = caseMapper;
        this.retryService = retryService;
        this.auditService = auditService;
    }

    @Scheduled(fixedDelayString = "${aftersales.label-retry.auto-interval-ms:120000}",
            initialDelayString = "${aftersales.label-retry.initial-delay-ms:30000}")
    public void autoRetry() {
        List<AfterSaleCaseEntity> candidates;
        try {
            candidates = caseMapper.selectRetryCandidates(SCAN_LIMIT);
        } catch (Exception e) {
            log.error("ReturnLabelRetryWorker.auto.scanFailed", "掃描重試候選失敗", e);
            return;
        }

        if (candidates.isEmpty()) {
            return;
        }

        log.info("ReturnLabelRetryWorker.auto.found",
                "發現 " + candidates.size() + " 筆 7-11 退貨待重試");

        for (AfterSaleCaseEntity caseEntity : candidates) {
            try {
                retryService.retryAutomatic(caseEntity);
            } catch (Exception e) {
                log.error("ReturnLabelRetryWorker.auto.caseError",
                        "自動重試處理失敗 caseNo=" + caseEntity.getCaseNo(), e);
            }
        }
    }

    @Scheduled(fixedDelayString = "${aftersales.label-retry.exhausted-interval-ms:600000}",
            initialDelayString = "${aftersales.label-retry.initial-delay-ms:30000}")
    public void checkExhausted() {
        List<AfterSaleCaseEntity> exhaustedCases;
        try {
            exhaustedCases = caseMapper.selectExhaustedCandidates(SCAN_LIMIT);
        } catch (Exception e) {
            log.error("ReturnLabelRetryWorker.exhausted.scanFailed", "掃描 exhausted 候選失敗", e);
            return;
        }

        if (exhaustedCases.isEmpty()) {
            return;
        }

        log.warn("ReturnLabelRetryWorker.exhausted.found",
                "發現 " + exhaustedCases.size() + " 筆 7-11 退貨代碼重試耗盡待 ops 介入");

        for (AfterSaleCaseEntity caseEntity : exhaustedCases) {
            try {
                // CAS 優先：先標記為 2（count=1→2），成功才記 audit log，避免同一 case 跨輪重複記 log
                int rows = caseMapper.markLabelRetryExhausted(caseEntity.getCaseNo());
                if (rows == 0) {
                    // 已被 ops reset 或他人處理，跳過
                    continue;
                }

                auditService.log(caseEntity.getCaseNo(), null, null,
                        AfterSaleAuditActor.SYSTEM.value(),
                        AfterSaleAuditAction.LABEL_RETRY_EXHAUSTED.value(),
                        Map.of("shipTradeNo",
                                caseEntity.getShipTradeNo() == null ? "" : caseEntity.getShipTradeNo()));

                log.warn("ReturnLabelRetryWorker.exhausted.logged",
                        "已記 LABEL_RETRY_EXHAUSTED 等待 ops caseNo=" + caseEntity.getCaseNo());
            } catch (Exception e) {
                log.error("ReturnLabelRetryWorker.exhausted.caseError",
                        "標記 exhausted 失敗 caseNo=" + caseEntity.getCaseNo(), e);
            }
        }
    }
}

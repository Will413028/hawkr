package io.github.will413028.hawkr.aftersales.scheduler;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.aftersales.service.ReturnPendingTimeoutCloseService;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;

/**
 * §5.3.8 退貨待寄回逾時自動關案 Worker（2026-06-15 重寫）。
 *
 * <p>政策：進入 RETURN_PENDING(20) 後買家逾 ship_back_deadline_at 未寄回 →
 * CAS 自動推進 status 20→33（AUTO_COMPLETED_RETURN_PENDING_TIMEOUT），退款資格解除，
 * 訂單款項照常撥付賣家。買家不服可走 §10 申訴。
 *
 * <p>對齊既有 StorePickupTimeoutScheduler / AutoConfirmReceiptScheduler 的「scheduler + deadline + CAS」pattern。
 *
 * <p>取代舊「每小時掃 created_at 寫 RETURN_TIMEOUT_ESCALATED audit log」邏輯（缺陷：同 case 每小時重複寫 log）。
 *
 * <p><b>搶佔語意（issue #2294 B1，出路 4）</b>：本 worker 只負責「撈候選 ＋ 逐筆委派」，
 * CAS ＋ audit ＋ publish 三件事收在 {@link ReturnPendingTimeoutCloseService#closeExpiredCase}
 * 的同一個 {@code @Transactional} 短 tx 內（<b>跨 bean</b> 呼叫，否則 self-invocation 不經 proxy）。
 * publish 失敗 ⇒ tx rollback ⇒ 該列仍是 status='20' ⇒ 下一 tick 重撈重發，事件不再永久遺失。
 */
@Component
public class ReturnTimeoutWorker {

    private static final ObsLogger log = ObsLogger.forClass(ReturnTimeoutWorker.class);

    private final AfterSaleCaseMapper caseMapper;
    private final ReturnPendingTimeoutCloseService closeService;
    private final AfterSalesEventPublisher eventPublisher;

    public ReturnTimeoutWorker(AfterSaleCaseMapper caseMapper,
                                ReturnPendingTimeoutCloseService closeService,
                                AfterSalesEventPublisher eventPublisher) {
        this.caseMapper = caseMapper;
        this.closeService = closeService;
        this.eventPublisher = eventPublisher;
    }

    @Scheduled(cron = "${aftersales.return.timeout-scheduler-cron:0 */10 * * * ?}")
    public void checkReturnTimeout() {
        LocalDateTime now = DateTimeUtils.nowUtc();

        // §5.3.8 掃描：status='20' AND ship_back_deadline_at <= NOW（partial index idx_aftersale_cases_return_timeout 覆蓋）
        List<AfterSaleCaseEntity> expired = caseMapper.selectList(
                new LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getStatus, AfterSaleCaseStatus.RETURN_PENDING.value())
                        .isNotNull(AfterSaleCaseEntity::getShipBackDeadlineAt)
                        .le(AfterSaleCaseEntity::getShipBackDeadlineAt, now)
        );

        if (expired.isEmpty()) {
            return;
        }

        log.info("ReturnTimeoutWorker.found", "發現 " + expired.size() + " 筆待寄回逾時案件");

        for (AfterSaleCaseEntity entity : expired) {
            try {
                // issue #2294 P1-2：身分反解（兩次無 deadline 的 gRPC）必須在**進 tx 之前**做完。
                // 失敗即跳過本筆——此時尚未 CAS，status 仍是 '20'、ship_back_deadline_at 仍到期，
                // 下一 tick（cron 10 分鐘）必然重撈。
                EventPrincipals principals =
                        eventPublisher.resolveEventPrincipals(entity.getStoreId(), entity.getBuyerId());
                closeService.closeExpiredCase(entity, now, principals);
            } catch (PrincipalResolutionFailedException e) {
                log.error("ReturnTimeoutWorker.resolve_principals.failed",
                        "身分反解失敗，本輪跳過（未進 tx、未 CAS），下一 tick 重試, caseNo="
                                + entity.getCaseNo(), e);
            } catch (Exception e) {
                // 含 EventPublishFailedException：短 tx 已 rollback，下一 tick 重撈重發
                log.error("ReturnTimeoutWorker.error",
                        "處理待寄回逾時案件異常, caseNo=" + entity.getCaseNo(), e);
            }
        }
    }
}

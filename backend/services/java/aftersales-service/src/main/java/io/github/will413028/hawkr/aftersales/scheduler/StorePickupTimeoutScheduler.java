package io.github.will413028.hawkr.aftersales.scheduler;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.aftersales.service.ReturnReceiptService;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.util.List;

/**
 * §5.3.10.2 超商待取逾時自動完成 Scheduler（2026-05-22 新增）。
 *
 * <p>每 10 分鐘掃描 {@code status='26' AND store_pickup_deadline_at <= NOW()} 的 case，
 * 呼叫 {@code ReturnReceiptService.autoCompleteFromAtStore} 系統路徑
 * （actor=SYSTEM、reason=STORE_PICKUP_TIMEOUT、全額退款）。
 *
 * <p><b>issue #2294 B6</b>：與 B5（{@link AutoConfirmReceiptScheduler}）逐字同形——事件身分反解
 * （兩次<b>無 deadline</b> 的 gRPC）在<b>進 tx 之前</b>做完再當參數傳入，反解失敗走 strict
 * （尚未 CAS ⇒ {@code status='26'} 與 {@code store_pickup_deadline_at} 未變 ⇒ 下一 tick 重撈）。
 * 兩處必須一起維持，否則同一支 service 會出現兩種 tx 形狀。
 *
 * <p>並發保護：
 * <ul>
 *   <li>樂觀鎖 {@code WHERE status='26'}：賣家剛從超商取貨觸發 26→27 時，後到 scheduler skip</li>
 *   <li>多實例部署：兩個 scheduler 同時 tick，誰先 UPDATE 誰贏，後到 skip</li>
 *   <li>同 case 連續被選兩次：第二次 status 已 31，{@code markAutoCompletedFromAtStore} skip</li>
 * </ul>
 */
@Component
public class StorePickupTimeoutScheduler {

    private static final ObsLogger log = ObsLogger.forClass(StorePickupTimeoutScheduler.class);

    /** 單次掃描最大筆數（避免單次處理過久阻塞下次 cron） */
    private static final int BATCH_LIMIT = 200;

    private final AfterSaleCaseMapper caseMapper;
    private final ReturnReceiptService returnReceiptService;
    private final AfterSalesProperties properties;

    public StorePickupTimeoutScheduler(AfterSaleCaseMapper caseMapper,
                                        ReturnReceiptService returnReceiptService,
                                        AfterSalesProperties properties) {
        this.caseMapper = caseMapper;
        this.returnReceiptService = returnReceiptService;
        this.properties = properties;
    }

    @Scheduled(cron = "${aftersales.return.store-pickup-scheduler-cron:0 */10 * * * ?}")
    public void checkStorePickupTimeout() {
        var cutoff = DateTimeUtils.nowUtc();
        List<String> expiredCaseNos = caseMapper.findStorePickupExpired(cutoff, BATCH_LIMIT);

        if (expiredCaseNos.isEmpty()) {
            return;
        }

        log.info("StorePickupTimeoutScheduler.found",
                "發現 " + expiredCaseNos.size() + " 筆 store_pickup 到期 case, cutoff=" + cutoff
                        + ", storePickupHours=" + properties.getReturnConfig().getStorePickupTimeoutHours());

        int succeeded = 0;
        int skipped = 0;
        int errors = 0;
        for (String caseNo : expiredCaseNos) {
            try {
                // issue #2294 B6：反解必須先於 tx（見類別 javadoc）
                EventPrincipals principals = returnReceiptService.resolvePrincipalsForCaseStrict(caseNo);
                boolean ok = returnReceiptService.autoCompleteFromAtStore(caseNo, principals);
                if (ok) {
                    succeeded++;
                } else {
                    skipped++;
                }
            } catch (PrincipalResolutionFailedException e) {
                errors++;
                log.error("StorePickupTimeoutScheduler.resolve_principals.failed",
                        "身分反解失敗，本輪跳過（未進 tx、未 CAS），下一 tick 重試, caseNo=" + caseNo, e);
            } catch (Exception e) {
                errors++;
                log.error("StorePickupTimeoutScheduler.error",
                        "處理 store pickup 逾時異常, caseNo=" + caseNo, e);
            }
        }

        log.info("StorePickupTimeoutScheduler.done",
                "store pickup 批次完成, succeeded=" + succeeded
                        + ", skipped=" + skipped + ", errors=" + errors);
    }
}

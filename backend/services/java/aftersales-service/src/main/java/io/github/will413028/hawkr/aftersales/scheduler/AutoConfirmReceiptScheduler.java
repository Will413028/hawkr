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
 * §5.3.10 退貨送達自動確認收貨 Scheduler。
 *
 * <p>每 10 分鐘掃描 {@code status='27' AND auto_confirm_at <= NOW()} 的 case，
 * 呼叫 {@code ReturnReceiptService.autoConfirmReceipt} 系統路徑（actor=SYSTEM，全額退款）。
 *
 * <p><b>issue #2294 B5</b>：事件身分反解（兩次<b>無 deadline</b> 的 gRPC）在<b>進 tx 之前</b>做完，
 * 結果當參數傳進 {@code @Transactional} —— 否則短 tx 會在 seller／buyer service 掛住時
 * 無限期持有 {@code aftersale_cases} 的 row lock（《多副本排程搶佔規範》§四「出路 4 的硬性邊界」）。
 * 反解失敗走 strict：該筆本輪不做，此時尚未 CAS ⇒ {@code status='27'} 與 {@code auto_confirm_at}
 * 皆未變 ⇒ 下一 tick 必然重撈。
 *
 * <p>並發保護：
 * <ul>
 *   <li>樂觀鎖 {@code WHERE status='27'}：賣家手動 / scheduler 自動同時動作時，後到者 affected=0 → skip</li>
 *   <li>多實例部署：兩個 scheduler 同時 tick，誰先 UPDATE 誰贏，後到 skip</li>
 *   <li>同 case 連續被選兩次：第二次 status 已 30，markReturnReceived skip</li>
 * </ul>
 */
@Component
public class AutoConfirmReceiptScheduler {

    private static final ObsLogger log = ObsLogger.forClass(AutoConfirmReceiptScheduler.class);

    /** 單次掃描最大筆數（避免單次處理過久阻塞下次 cron） */
    private static final int BATCH_LIMIT = 200;

    private final AfterSaleCaseMapper caseMapper;
    private final ReturnReceiptService returnReceiptService;
    private final AfterSalesProperties properties;

    public AutoConfirmReceiptScheduler(AfterSaleCaseMapper caseMapper,
                                        ReturnReceiptService returnReceiptService,
                                        AfterSalesProperties properties) {
        this.caseMapper = caseMapper;
        this.returnReceiptService = returnReceiptService;
        this.properties = properties;
    }

    @Scheduled(cron = "${aftersales.return.auto-confirm-scheduler-cron:0 */10 * * * ?}")
    public void checkAutoConfirm() {
        var cutoff = DateTimeUtils.nowUtc();
        List<String> expiredCaseNos = caseMapper.findAutoConfirmExpired(cutoff, BATCH_LIMIT);

        if (expiredCaseNos.isEmpty()) {
            return;
        }

        log.info("AutoConfirmReceiptScheduler.found",
                "發現 " + expiredCaseNos.size() + " 筆 auto_confirm 到期 case, cutoff=" + cutoff
                        + ", autoConfirmHours=" + properties.getReturnConfig().getAutoConfirmReceiptHours());

        int succeeded = 0;
        int skipped = 0;
        int errors = 0;
        for (String caseNo : expiredCaseNos) {
            try {
                // issue #2294 B5：反解必須先於 tx（見類別 javadoc）
                EventPrincipals principals = returnReceiptService.resolvePrincipalsForCaseStrict(caseNo);
                boolean ok = returnReceiptService.autoConfirmReceipt(caseNo, principals);
                if (ok) {
                    succeeded++;
                } else {
                    skipped++;
                }
            } catch (PrincipalResolutionFailedException e) {
                errors++;
                log.error("AutoConfirmReceiptScheduler.resolve_principals.failed",
                        "身分反解失敗，本輪跳過（未進 tx、未 CAS），下一 tick 重試, caseNo=" + caseNo, e);
            } catch (Exception e) {
                errors++;
                log.error("AutoConfirmReceiptScheduler.error",
                        "處理 auto confirm 異常, caseNo=" + caseNo, e);
                // 不中斷整批，繼續處理其他 case
            }
        }

        log.info("AutoConfirmReceiptScheduler.done",
                "auto confirm 批次完成, succeeded=" + succeeded
                        + ", skipped=" + skipped + ", errors=" + errors);
    }
}

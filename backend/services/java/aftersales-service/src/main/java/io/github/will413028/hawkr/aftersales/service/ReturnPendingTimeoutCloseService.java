package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.HashMap;
import java.util.Map;

/**
 * §5.3.8 待寄回逾時自動關案的原子操作（issue #2294 B1，出路 4「短 tx」）。
 *
 * <p><b>為什麼要有這個 bean</b>：{@code ReturnTimeoutWorker} 原本是
 * 「CAS（auto-commit）→ audit → publish（best-effort）」——{@code status='33'} 是<b>終態</b>，
 * CAS 成功後該列永久離開候選集合（候選述詞 {@code status='20'}），所以 publish 失敗＝
 * {@code case.auto_completed_return_pending_timeout} <b>永久遺失</b>：買賣雙方沒收到通知、
 * order-service 也不會恢復倒數。
 *
 * <p><b>修法</b>：把 CAS ＋ audit ＋ publish 收進<b>同一個</b> {@link Transactional} 短 tx，
 * publish 走會傳遞失敗的變體，失敗即拋 {@link EventPublishFailedException} 讓整個 tx rollback。
 * rollback 後 {@code status} 仍是 {@code '20'}、{@code ship_back_deadline_at <= NOW()} 仍成立
 * ⇒ 下一個 tick（cron 10 分鐘）必然重撈，事件不會遺失。
 *
 * <p><b>tx 內不得有網路呼叫（issue #2294 P1-2）</b>：{@code tryPublishCaseTransitionWithContext}
 * 的無 {@link EventPrincipals} 多載會在方法內做兩次<b>無 deadline</b> 的身分反解 gRPC——那會把
 * 「短 tx」變成「持有 row lock 等 seller-service 回應」。本方法改收 tx 外預解好的
 * {@link EventPrincipals}，讓 tx 內真的只剩一列 CAS ＋ audit ＋ 一次 publish。
 *
 * <p>⚠️ <b>不得把 {@link Transactional} 直接加在 worker 的 private 方法上</b>：同 class
 * self-invocation 不經 Spring AOP proxy，註解完全失效（10 檔 §11.1）。這正是本 bean 存在的理由，
 * 與 {@link CaseAutoRejectService} / {@link CaseEscalationService} 同一手法。
 *
 * <p><b>殘餘窗口（誠實標註）</b>：publish 成功、commit 失敗 ⇒ 事件已發、狀態未轉；
 * 但該列仍滿足候選述詞 ⇒ 下一 tick 重跑並重發，consumer 冪等吸收，窗口 = 一個 tick。
 * 這是<b>暫態</b>不一致，對照「例外 B 用在業務狀態轉換」時的<b>永久</b>不一致，是取捨的全部內容。
 *
 * <p>設計書：09 §5.3.8 / §8.5；規範：《多副本排程搶佔規範》§四 出路 4。
 */
@Service
public class ReturnPendingTimeoutCloseService {

    private static final ObsLogger log = ObsLogger.forClass(ReturnPendingTimeoutCloseService.class);

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleAuditService auditService;
    private final AfterSalesEventPublisher eventPublisher;
    private final AfterSalesProperties properties;

    public ReturnPendingTimeoutCloseService(AfterSaleCaseMapper caseMapper,
                                             AfterSaleAuditService auditService,
                                             AfterSalesEventPublisher eventPublisher,
                                             AfterSalesProperties properties) {
        this.caseMapper = caseMapper;
        this.auditService = auditService;
        this.eventPublisher = eventPublisher;
        this.properties = properties;
    }

    /**
     * 單筆待寄回逾時案件的自動關案（CAS 20→33 ＋ audit ＋ publish，同一 tx）。
     *
     * @param entity 候選案件（由 worker 批次撈出）
     * @param now    本輪掃描時間（UTC，整批共用同一時刻）
     * @param principals <b>tx 外</b>預先解析好的身分欄位（issue #2294 P1-2）
     * @return true = 已關案；false = CAS affected=0（買家壓哨寄出 20→25 或他路徑搶先），skip
     * @throws EventPublishFailedException publish 失敗 ⇒ 整個 tx rollback，下一 tick 重來
     */
    @Transactional
    public boolean closeExpiredCase(AfterSaleCaseEntity entity, LocalDateTime now,
                                     EventPrincipals principals) {
        String caseNo = entity.getCaseNo();

        // CAS 自動關案：status='20' AND ship_back_deadline_at <= NOW → status='33'
        // 競態：買家壓哨寄出 20→25 搶先 → affected=0 skip
        int affected = caseMapper.markReturnPendingTimeoutAutoClosed(caseNo, now);
        if (affected == 0) {
            log.info("ReturnPendingTimeoutCloseService.cas_skip",
                    "CAS affected=0（買家剛寄出或他路徑搶先）, caseNo=" + caseNo);
            return false;
        }

        // 寫 audit log（新流程改用 RETURN_TIMEOUT_AUTO_CLOSED，舊 RETURN_TIMEOUT_ESCALATED 已 Deprecated）
        int timeoutDays = properties.getReturnConfig().getTimeoutDays();
        LocalDateTime returnPendingAt = entity.getShipBackDeadlineAt() != null
                ? entity.getShipBackDeadlineAt().minusDays(timeoutDays)
                : null;
        Map<String, Object> auditDetail = new HashMap<>();
        auditDetail.put("shipBackDeadlineAt", entity.getShipBackDeadlineAt() != null
                ? entity.getShipBackDeadlineAt().toString() : null);
        auditDetail.put("returnPendingAt", returnPendingAt != null ? returnPendingAt.toString() : null);
        auditDetail.put("timeoutDays", timeoutDays);
        auditService.log(caseNo, null, null,
                AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.RETURN_TIMEOUT_AUTO_CLOSED.value(),
                auditDetail);

        // publish event（tx 內）：notification-service 推買賣雙方；order-service 嘗試恢復倒數
        Map<String, Object> extraPayload = new HashMap<>();
        extraPayload.put("shipping_method", entity.getReturnShippingMethod());
        extraPayload.put("deadline_at", entity.getShipBackDeadlineAt() != null
                ? entity.getShipBackDeadlineAt().toString() : null);
        extraPayload.put("auto_complete_reason", "RETURN_PENDING_TIMEOUT");
        boolean published = eventPublisher.tryPublishCaseTransitionWithContext(
                AfterSalesEventType.CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT.value(),
                AfterSalesEventType.CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT.name(),
                caseNo,
                entity.getOrderNo(),
                entity.getBuyerId(),
                entity.getStoreId(),
                "system",
                extraPayload,
                principals);
        if (!published) {
            throw new EventPublishFailedException(
                    AfterSalesEventType.CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT.value(), caseNo);
        }

        log.info("ReturnPendingTimeoutCloseService.auto_closed",
                "待寄回逾時自動關案 20→33, caseNo=" + caseNo
                        + ", shipBackDeadlineAt=" + entity.getShipBackDeadlineAt());
        return true;
    }
}

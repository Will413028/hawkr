package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleAppealEventEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealEventType;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;

/**
 * §5.10 售後單自動升級至 admin 仲裁的原子操作。
 *
 * <p>抽出獨立 bean 是為了讓 {@link Transactional} 透過 Spring AOP proxy 生效（避免
 * worker 內 self-invocation 失效的問題）。一個 transaction 包住「樂觀鎖 CAS UPDATE」、
 * 「INSERT aftersale_appeal_events」與「publish {@code aftersales.case.auto_escalated}」
 * （publish 於 issue #2294 B3 起收進本 tx），任一失敗 rollback。
 *
 * <p><b>搶佔語意（issue #2294 B3，出路 4）</b>：{@code auto_escalated=false→true} 是<b>永久旗標</b>，
 * CAS 成功後該列永久離開候選集合。publish 原本在 tx 之外 best-effort，一旦失敗
 * {@code aftersales.case.auto_escalated} <b>永久遺失</b>（admin 佇列裡出現一張沒人通知過的單）。
 * 改用會傳遞失敗的變體，失敗即拋 {@link EventPublishFailedException} → rollback →
 * 下一 tick（fixedDelay 60s）必然重撈。
 *
 * <p><b>tx 內不得有網路呼叫（issue #2294 P1-2）</b>：兩次身分反解 gRPC 由
 * {@code CaseProcessTimeoutWorker} 在<b>進本方法之前</b>完成，當 {@link EventPrincipals} 參數傳入。
 * 理由與失敗語意同 {@link CaseAutoRejectService}。
 *
 * <p>⚠️ <b>不可改走例外 B</b>：理由同 {@link CaseAutoRejectService}（設計書 09 §5.10.5）。
 */
@Service
public class CaseEscalationService {

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleAppealEventMapper appealEventMapper;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher eventPublisher;

    public CaseEscalationService(AfterSaleCaseMapper caseMapper,
                                  AfterSaleAppealEventMapper appealEventMapper,
                                  AfterSalesProperties properties,
                                  AfterSalesEventPublisher eventPublisher) {
        this.caseMapper = caseMapper;
        this.appealEventMapper = appealEventMapper;
        this.properties = properties;
        this.eventPublisher = eventPublisher;
    }

    /**
     * 嘗試將指定 case 升級至 admin 仲裁佇列。
     *
     * @param orderNo 訂單編號（事件 payload；worker 手上已有整個 entity）
     * @param buyerId 買家 ID（事件 payload）
     * @param storeId 賣家 store_id（事件 payload）
     * @param principals <b>tx 外</b>預先解析好的身分欄位（issue #2294 P1-2）
     * @return true = 升級成功（已寫 UPDATE + appeal_event + publish）；false = 樂觀鎖失敗（已被他人處理）
     * @throws EventPublishFailedException MQ 發佈失敗（整 tx rollback，下一 tick 重來）
     */
    @Transactional
    public boolean escalate(String caseNo, LocalDateTime now,
                            String orderNo, Long buyerId, Long storeId,
                            EventPrincipals principals) {
        int affected = caseMapper.markAutoEscalated(caseNo, now);
        if (affected != 1) {
            return false;
        }

        AfterSaleAppealEventEntity event = new AfterSaleAppealEventEntity();
        event.setCaseNo(caseNo);
        event.setEventType(AfterSaleAppealEventType.AUTO_ESCALATED.value());
        event.setActorType("SYSTEM");
        event.setActorId(null);
        event.setLabel("系統自動升級");
        int hours = properties.getCaseConfig().getProcessDeadlineHours();
        event.setDescription("賣家未在 " + hours + " 小時內處理，案件自動升級至平台仲裁");
        appealEventMapper.insert(event);

        // issue #2294 B3：publish 收進本 tx，失敗即 rollback（原本在 worker、tx 之外 best-effort）
        boolean published = eventPublisher.tryPublishCaseAutoEscalated(
                caseNo, orderNo, buyerId, storeId, hours, principals);
        if (!published) {
            throw new EventPublishFailedException(
                    AfterSalesEventType.CASE_AUTO_ESCALATED.value(), caseNo);
        }

        return true;
    }
}

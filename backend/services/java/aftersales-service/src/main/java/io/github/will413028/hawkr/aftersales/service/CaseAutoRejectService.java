package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSaleRejectReason;
import io.github.will413028.hawkr.common.utils.code.AfterSaleResolutionType;
import io.github.will413028.hawkr.common.utils.code.AfterSalesEventType;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.Map;

/**
 * §5.10 AUTO_REJECT 路徑：賣家處理逾時，系統代為拒絕的原子操作。
 *
 * <p>抽出獨立 bean 是為了讓 {@link Transactional} 透過 Spring AOP proxy 生效（避免
 * worker 內 self-invocation 失效）。一個 transaction 包住三件事：
 * <ol>
 *   <li>INSERT {@code aftersale_resolutions}（resolution_type=REJECT, reject_reason=SELLER_NO_RESPONSE,
 *       resolved_by=NULL，source 沿用 DB 預設 SELLER）</li>
 *   <li>UPDATE {@code aftersale_cases}（status=REJECTED(60), last_rejected_at=NOW, process_deadline=NULL；
 *       樂觀鎖透過 status CAS）</li>
 *   <li>INSERT {@code aftersale_audit_logs}（actor_type=SYSTEM, action=RESOLVE_REJECT）</li>
 *   <li><b>publish {@code aftersales.case.auto_rejected}</b>（issue #2294 B2 起收進本 tx）</li>
 * </ol>
 * 任一失敗 rollback，確保「決議寫入」「audit 紀錄」「對外事件」三者一致。
 *
 * <p><b>搶佔語意（issue #2294 B2，出路 4）</b>：{@code status='60'} 是<b>終態</b>，CAS 成功後該列
 * 永久離開 {@code selectProcessTimeoutCandidates} 的候選集合。publish 原本在 tx <b>之外</b>
 * best-effort（worker 的 catch 吞掉），一旦失敗 {@code aftersales.case.auto_rejected}
 * <b>永久遺失</b>（買家永遠不知道案件被系統代拒）。修法是把 publish 移進本 tx 並改用會傳遞失敗的
 * 變體：失敗即拋 {@link EventPublishFailedException} → rollback → 四個候選條件全部復原 →
 * 下一 tick（fixedDelay 60s）必然重撈。
 *
 * <p><b>tx 內不得有網路呼叫（issue #2294 P1-2）</b>：出路 4 的背書前提是「tx 內只有一列 CAS
 * 與一次 publish」（《多副本排程搶佔規範》§一）。組事件 payload 需要的兩次身分反解 gRPC
 * （{@code store_id}／{@code buyer_id} → 登入 {@code account_id}）<b>皆無 deadline</b>，
 * 留在 tx 內會讓 seller／buyer service 掛住時無限期持有本列 row lock 與一條 Hikari 連線。
 * ⇒ 由 {@code CaseProcessTimeoutWorker} 在<b>進本方法之前</b>解析好，當 {@link EventPrincipals}
 * 參數傳入；解析失敗則該筆本輪不做（候選述詞未變，下一 tick 重撈）。
 *
 * <p>⚠️ <b>不可改走例外 B（publish → 成功才 CAS）</b>：{@code status='60'} 是業務動作本身不是事後記錄，
 * 換順序會讓 CAS affected=0（賣家在 deadline 邊界剛好回應）變成「買家已收到『已自動拒絕』、
 * 案件其實被正常處理」，而且不會自我收斂（設計書 09 §5.10.5）。
 */
@Service
public class CaseAutoRejectService {

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSaleResolutionMapper resolutionMapper;
    private final AfterSaleAuditService auditService;
    private final AfterSalesEventPublisher eventPublisher;

    public CaseAutoRejectService(AfterSaleCaseMapper caseMapper,
                                  AfterSaleResolutionMapper resolutionMapper,
                                  AfterSaleAuditService auditService,
                                  AfterSalesEventPublisher eventPublisher) {
        this.caseMapper = caseMapper;
        this.resolutionMapper = resolutionMapper;
        this.auditService = auditService;
        this.eventPublisher = eventPublisher;
    }

    /**
     * 嘗試將指定 case 自動拒絕。
     *
     * <p>順序：先寫 resolution → 再做 status CAS → 再寫 audit log → 最後 publish（皆同一 tx）。
     * status CAS 失敗（affected=0）時整個 tx rollback，不會留下 orphan resolution。
     *
     * @param caseNo  案件編號
     * @param now     當前時間（UTC，由 caller 提供確保整批 worker 用同一時刻）
     * @param hours   原本的處理期限（用於 reject_message 文案 ＋ 事件 payload）
     * @param orderNo 訂單編號（事件 payload；worker 手上已有整個 entity）
     * @param buyerId 買家 ID（事件 payload）
     * @param storeId 賣家 store_id（事件 payload）
     * @param principals <b>tx 外</b>預先解析好的身分欄位（issue #2294 P1-2）
     * @return true = 拒絕成功（已寫 INSERT resolution + UPDATE case + INSERT audit log + publish）
     * @throws OptimisticLockLostException  樂觀鎖失敗（已被他人處理，整 tx rollback）
     * @throws EventPublishFailedException  MQ 發佈失敗（整 tx rollback，下一 tick 重來）
     */
    @Transactional
    public boolean autoReject(String caseNo, LocalDateTime now, int hours,
                              String orderNo, Long buyerId, Long storeId,
                              EventPrincipals principals) {
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setId(TsidUtil.generate());
        resolution.setCaseNo(caseNo);
        resolution.setResolutionType(AfterSaleResolutionType.REJECT.value());
        resolution.setRejectReason(AfterSaleRejectReason.SELLER_NO_RESPONSE.value());
        resolution.setRejectMessage("賣家未在 " + hours + " 小時內回應，系統代為拒絕");
        resolution.setResolvedBy(null);
        resolutionMapper.insert(resolution);

        int affected = caseMapper.markAutoRejected(caseNo, now);
        if (affected != 1) {
            throw new OptimisticLockLostException(caseNo);
        }

        auditService.log(caseNo, null, null,
                AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.RESOLVE_REJECT.value(),
                Map.of("rejectReason", AfterSaleRejectReason.SELLER_NO_RESPONSE.value(),
                        "trigger", "PROCESS_TIMEOUT"));

        // issue #2294 B2：publish 收進本 tx，失敗即 rollback（原本在 worker、tx 之外 best-effort）
        boolean published = eventPublisher.tryPublishCaseAutoRejected(
                caseNo, orderNo, buyerId, storeId, hours, principals);
        if (!published) {
            throw new EventPublishFailedException(
                    AfterSalesEventType.CASE_AUTO_REJECTED.value(), caseNo);
        }

        return true;
    }

    /**
     * 樂觀鎖失敗 sentinel，僅供 {@link #autoReject} 內部使用以觸發 tx rollback。
     * Caller 應 catch 此例外並視為「跳過此 case」。
     */
    public static class OptimisticLockLostException extends RuntimeException {
        public OptimisticLockLostException(String caseNo) {
            super("AUTO_REJECT optimistic lock lost: " + caseNo);
        }
    }
}

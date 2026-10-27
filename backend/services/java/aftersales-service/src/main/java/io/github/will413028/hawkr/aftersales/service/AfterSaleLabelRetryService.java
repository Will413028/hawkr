package io.github.will413028.hawkr.aftersales.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditAction;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAuditActor;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.Map;
import java.util.Set;

/**
 * 7-11 退貨出貨便代碼列印重試服務（§5.3.7）。
 *
 * <p>封裝自動重試（Worker 呼叫）與人工重試（ops 透過 retry API）共用邏輯：
 * <ol>
 *   <li>搶佔：自動走 {@code label_retry_at} 的 lease CAS（issue #2294 L7）；人工先 reset 成 0</li>
 *   <li>呼叫 Shipping {@code RetryReturnPrintLabel}（shipping 端以獨立執行緒觸發列印，不等 webhook）</li>
 *   <li>記 audit log（LABEL_RETRY_AUTO / LABEL_RETRY_MANUAL）</li>
 * </ol>
 *
 * <p>webhook 回來後由 {@link io.github.will413028.hawkr.aftersales.mq.ShipmentLabelConsumer} 寫入 partner_id/odno/validation_no。
 */
@Service
public class AfterSaleLabelRetryService {

    private static final ObsLogger log = ObsLogger.forClass(AfterSaleLabelRetryService.class);

    private static final Set<String> RETRY_APPLICABLE_STATUSES = Set.of(
            AfterSaleCaseStatus.RETURN_PENDING.value(),
            AfterSaleCaseStatus.RETURN_SHIPPING.value()
    );

    private static final String SEVEN_ELEVEN_METHOD = "3002";

    /**
     * issue #2294 L7：自動重試的租約長度（分）。
     *
     * <p><b>下界</b>——單筆工作是一次 shipping {@code RetryReturnPrintLabel} gRPC；
     * shipping 端以獨立執行緒觸發列印、不等 webhook，所以那是一次短往返。6 分鐘綽綽有餘。
     * <p><b>上界</b>——租約同時是「這次觸發沒成功、下次重來要等多久」的窗口，而
     * {@code ReturnLabelRetryWorker.checkExhausted()} 在 10 分鐘就會把這個案件標成耗盡並告警
     * ⇒ 租約必須明顯小於 10 分，否則案件在租約到期前就被判耗盡、等於只嘗試一次。
     * 取 6 分（＝ auto-interval 2 分 × 3）。
     */
    static final long LABEL_RETRY_LEASE_MINUTES = 6L;

    private final AfterSaleCaseMapper caseMapper;
    private final ShippingGrpcClient shippingGrpcClient;
    private final AfterSaleAuditService auditService;

    public AfterSaleLabelRetryService(AfterSaleCaseMapper caseMapper,
                                       ShippingGrpcClient shippingGrpcClient,
                                       AfterSaleAuditService auditService) {
        this.caseMapper = caseMapper;
        this.shippingGrpcClient = shippingGrpcClient;
        this.auditService = auditService;
    }

    /**
     * 自動重試（Worker 呼叫）：lease claim → 呼叫 shipping → 確認成功才消耗預算 + 記 audit log。
     *
     * <p><b>issue #2294 L7：搶佔語意</b>（規範 {@code docs/開發規範/多副本排程搶佔規範.md}）
     * <ul>
     *   <li><b>閘門</b>＝{@code AfterSaleCaseMapper.claimLabelRetry} 的 lease CAS
     *       （{@code label_retry_at} 推到未來），在 gRPC 副作用<b>之前</b>（規則 ①）。</li>
     *   <li><b>是租約不是永久旗標</b>（規則 ③）：pod 在 claim 與 gRPC 之間消失，
     *       租約到期該案件自動回到候選集合。</li>
     *   <li><b>預算（{@code label_retry_count}）移到副作用之後</b>：舊實作拿它的 0→1 CAS 當閘門，
     *       而候選述詞是 {@code count = 0} ⇒ CAS 成功即永久離開候選集合。副作用在其後、
     *       例外又被 catch 吞掉 ⇒ <b>不必等崩潰，一次 gRPC 失敗就永久用掉唯一一次自動重試</b>。</li>
     *   <li><b>失敗時釋放租約成「過去時間」</b>（{@link #releaseLease}）：下一 tick 就能重來，
     *       同時把「試過」這件事留在 {@code label_retry_at} 上，讓耗盡掃描仍撈得到
     *       —— 釋放成 {@code NULL} 會把耗盡告警的偵測條件一起抹掉。</li>
     * </ul>
     *
     * <p><b>為什麼不再包 {@code @Transactional} + {@code afterCommit}</b>：舊寫法用
     * afterCommit 只是為了「讓 shipping 看到已寫回的 retry_count」。現在 claim 是單句
     * 自動提交的 CAS，本來就在 gRPC 之前落庫，那層包裝失去理由；而它反而把 gRPC 的例外
     * 關進一個<b>沒有人能回應</b>的 callback（tx 早已 commit），正是缺陷的成因。
     *
     * <p><b>失敗訊號確實拿得到</b>（規範 §四 例外 B 第 4 條講的反例不成立）：
     * {@code ShippingGrpcClient.retryReturnPrintLabel} 是 blocking stub，傳輸層失敗會拋
     * {@code StatusRuntimeException}；業務失敗則在回應的 {@code code} 上——兩者本方法都判。
     *
     * @return true = 已確認觸發，false = 沒搶到租約、或 shipping 觸發失敗（預算未消耗，下輪重來）
     */
    public boolean retryAutomatic(AfterSaleCaseEntity caseEntity) {
        String caseNo = caseEntity.getCaseNo();
        String shipTradeNo = caseEntity.getShipTradeNo();

        // ⚠️ 租約基準取 **claim 當下**，不是 Worker 掃描批次的起點（規範 §三 第 6 點）：
        // 取批次起點時，第 k 筆的有效租約 = LEASE − (前 k−1 筆的耗時)，
        // SCAN_LIMIT=50 × 單筆一次 gRPC 就足以把它侵蝕掉。
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime leaseUntil = now.plusMinutes(LABEL_RETRY_LEASE_MINUTES);
        int claimed = caseMapper.claimLabelRetry(caseNo, now, leaseUntil);
        if (claimed == 0) {
            log.info("AfterSaleLabelRetryService.auto.claimLost",
                    "租約由他人持有或預算已用完，skip caseNo=" + caseNo);
            return false;
        }

        try {
            var resp = shippingGrpcClient.retryReturnPrintLabel(shipTradeNo);
            if (resp == null || resp.getCode() != GrpcResponseCode.SUCCESS.code()) {
                releaseLease(caseNo, leaseUntil, now,
                        "shipping 回非成功碼 code=" + (resp == null ? "null" : resp.getCode()));
                return false;
            }
        } catch (Exception e) {
            log.error("AfterSaleLabelRetryService.auto.shippingFailed",
                    "shipping retry 呼叫失敗 caseNo=" + caseNo, e);
            releaseLease(caseNo, leaseUntil, now,
                    e.getClass().getSimpleName() + ": " + e.getMessage());
            return false;
        }

        // 觸發已確認成功才消耗預算——這一句是「已確認觸發過」的事後記錄，不是閘門。
        caseMapper.markLabelRetried(caseNo);

        auditService.log(caseNo, null, null,
                AfterSaleAuditActor.SYSTEM.value(),
                AfterSaleAuditAction.LABEL_RETRY_AUTO.value(),
                Map.of("shipTradeNo", shipTradeNo == null ? "" : shipTradeNo));

        log.info("AfterSaleLabelRetryService.auto.triggered",
                "自動重試觸發成功 caseNo=" + caseNo + " shipTradeNo=" + shipTradeNo);
        return true;
    }

    /**
     * shipping 觸發沒成功時把租約設成<b>一個已過去的時間</b>（＝ claim 當下的 {@code now}），
     * 讓下一個 tick（2 分）就能重來，而不是等滿一個租約（{@link #LABEL_RETRY_LEASE_MINUTES} 分）。
     *
     * <p>🚨 <b>釋放成過去時間，不是還原成 claim 之前的舊值。</b> 曾經是還原舊值，而對一個
     * 「從未成功觸發過」的案件那個舊值<b>就是 {@code NULL}</b> ⇒ 耗盡掃描述詞
     * {@code count = 0 AND label_retry_at IS NOT NULL} 只在租約被持有的那幾秒成立，而
     * {@link io.github.will413028.hawkr.aftersales.scheduler.ReturnLabelRetryWorker#checkExhausted()}
     * 每 10 分鐘才取樣一次 ⇒ <b>shipping 長時間打不通時 {@code LABEL_RETRY_EXHAUSTED}
     * 幾乎永遠不會發</b>，案件每 2 分鐘靜靜重試、只留 error log，ops 動線不會被啟動。
     * 「加速恢復」（要證據消失才回得到候選集合）與「保留偵測證據」（要證據留著才看得到）
     * 兩個目標互相抵銷；過去時間同時滿足兩者。
     *
     * <p><b>三態</b>：{@code NULL} ＝ 從未嘗試過；過去 ＝ 嘗試過但無人持有；未來 ＝ 嘗試中。
     * 唯一能把它送回 {@code NULL} 的是 ops 的 {@code resetLabelRetryCount}（重新給一次機會）。
     *
     * <p><b>不會搶到別人的租約</b>：CAS 條件是「{@code label_retry_at} 仍等於我剛寫進去的值」。
     * <p><b>這不是正確性的一部分</b>：進程直接消失時本方法不會執行——那正是租約本身存在的理由
     *（規範 §二 第 5 步）。而那條路徑上租約自然到期，同樣落在「過去時間」這一態，語意一致。
     *
     * @param expiredAt claim 當下取的 {@code now}；它必定早於本次釋放時刻，且與
     *                  {@code claimLabelRetry} 的 {@code #{now}} 同一個時鐘基準
     */
    private void releaseLease(String caseNo, LocalDateTime leaseUntil,
                              LocalDateTime expiredAt, String reason) {
        try {
            int released = caseMapper.releaseLabelRetryLease(caseNo, leaseUntil, expiredAt);
            log.warn("AfterSaleLabelRetryService.auto.leaseReleased",
                    "shipping 觸發未成功，租約已釋放為過期（released=" + released
                            + "，0 = 期間已被他人重新 claim）caseNo=" + caseNo
                            + " reason=" + reason);
        } catch (Exception e) {
            // 釋放失敗不是新的破口：不釋放就等於等滿一個租約，正確性仍由租約保證，
            // 而租約值本身非 NULL ⇒ 耗盡掃描這段期間照樣看得到它。
            log.error("AfterSaleLabelRetryService.auto.leaseReleaseFailed",
                    "租約釋放失敗，本案件要等滿 " + LABEL_RETRY_LEASE_MINUTES + " 分鐘才會重試 caseNo="
                            + caseNo + " err=" + e.getMessage());
        }
    }

    /**
     * 人工重試（admin/ops 透過 retry API 呼叫）。
     * <p>ADMIN 權限由 Gateway requireAdmin + gRPC requireAccountType(ADMIN) 雙層保護，本 service 不再驗 case ownership。
     * <p>業務驗證：case 狀態 + 是否 7-11 + 是否已有代碼 + 是否已建立退貨物流（有 shipTradeNo）。
     * 動作：reset retry_count=0 → 呼 shipping → 記 audit log（actor=ADMIN）。
     *
     * @param adminAccountId 操作者 admin 帳號 ID（Gateway JWT 注入），僅用於 audit log
     * @param caseNo         售後案件編號
     */
    @Transactional
    public void retryManual(long adminAccountId, String caseNo) {
        AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                new LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getCaseNo, caseNo)
                        .last("LIMIT 1"));

        if (caseEntity == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_FOUND);
        }
        if (!RETRY_APPLICABLE_STATUSES.contains(caseEntity.getStatus())
                || !SEVEN_ELEVEN_METHOD.equals(caseEntity.getReturnShippingMethod())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_LABEL_RETRY_NOT_APPLICABLE);
        }
        if (caseEntity.getPartnerId() != null && !caseEntity.getPartnerId().isBlank()) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_LABEL_ALREADY_GENERATED);
        }
        String shipTradeNo = caseEntity.getShipTradeNo();
        if (shipTradeNo == null || shipTradeNo.isBlank()) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_LABEL_RETRY_NOT_APPLICABLE);
        }

        caseMapper.resetLabelRetryCount(caseNo);

        // TX commit 後才呼 shipping
        org.springframework.transaction.support.TransactionSynchronizationManager.registerSynchronization(
                new org.springframework.transaction.support.TransactionSynchronization() {
                    @Override
                    public void afterCommit() {
                        try {
                            shippingGrpcClient.retryReturnPrintLabel(shipTradeNo);
                            log.info("AfterSaleLabelRetryService.manual.triggered",
                                    "人工重試觸發成功 caseNo=" + caseNo + " shipTradeNo=" + shipTradeNo);
                        } catch (Exception e) {
                            log.error("AfterSaleLabelRetryService.manual.shippingFailed",
                                    "shipping retry 呼叫失敗 caseNo=" + caseNo, e);
                        }
                    }
                });

        auditService.log(caseNo, null, adminAccountId,
                AfterSaleAuditActor.ADMIN.value(),
                AfterSaleAuditAction.LABEL_RETRY_MANUAL.value(),
                Map.of("shipTradeNo", shipTradeNo));
    }
}

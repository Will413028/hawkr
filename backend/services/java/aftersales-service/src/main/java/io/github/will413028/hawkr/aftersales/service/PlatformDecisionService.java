package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleAppealEventEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSalePlatformDecisionEntity;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import io.github.will413028.hawkr.aftersales.exception.AfterSalesBusinessException;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleAppealEventMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSalePlatformDecisionMapper;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleResolutionMapper;
import io.github.will413028.hawkr.aftersales.mapper.RefundTaskMapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealEventType;
import io.github.will413028.hawkr.common.utils.code.AfterSaleAppealState;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import io.github.will413028.hawkr.common.utils.code.AfterSaleResolutionSource;
import io.github.will413028.hawkr.common.utils.code.AfterSaleResolutionType;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.PaymentMethod;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * §10 平台裁決服務。
 * 處理 Admin 對申訴案件做出的三種 outcome：FULL_REFUND / PARTIAL_REFUND / REJECT_APPEAL。
 *
 * 退款觸發：FULL/PARTIAL_REFUND 時自行建立 refund_tasks + 對 ATM 額外 publish awaiting_buyer_page
 * 事件，退款後續沿用既有 §8.3 管線。REJECT_APPEAL 不觸發退款，case 終態。
 */
@Service
@RequiredArgsConstructor
public class PlatformDecisionService {

    private static final ObsLogger log = ObsLogger.forClass(PlatformDecisionService.class);

    private final AfterSaleCaseMapper caseMapper;
    private final AfterSalePlatformDecisionMapper platformDecisionMapper;
    private final AfterSaleResolutionMapper resolutionMapper;
    private final AfterSaleAppealEventMapper appealEventMapper;
    private final RefundTaskMapper refundTaskMapper;
    private final AfterSalesProperties properties;
    private final AfterSalesEventPublisher eventPublisher;

    /**
     * Admin 裁決入口（§10.8.2）。
     *
     * @param adminAccountId Admin ID（存 decided_by_admin_id）
     * @param caseNo         案件編號
     * @param outcome        FULL_REFUND / PARTIAL_REFUND / REJECT_APPEAL
     * @param refundAmount   PARTIAL_REFUND 必填；FULL_REFUND / REJECT_APPEAL 不得傳或 = 0
     * @param reason         裁決理由（必填）
     */
    @Transactional
    public void decide(Long adminAccountId, String caseNo, String outcome,
                       Long refundAmount, String reason) {
        CodeMetaValidator.requireValidValue(AfterSalePlatformDecisionOutcome.class, outcome, "outcome");
        if (reason == null || reason.isBlank()) {
            throw new IllegalArgumentException("reason required");
        }

        AfterSaleCaseEntity caseEntity = caseMapper.selectOne(
                new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getCaseNo, caseNo)
        );
        if (caseEntity == null) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_NOT_FOUND);
        }

        // A2：appeal_state 必須為 APPEALED
        if (!AfterSaleAppealState.APPEALED.value().equals(caseEntity.getAppealState())) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_APPEAL_STATE_MISMATCH);
        }

        // 驗證 outcome × refund_amount 組合
        AfterSalePlatformDecisionOutcome outcomeEnum = AfterSalePlatformDecisionOutcome.fromValue(outcome);
        Long finalAmount = computeFinalAmount(outcomeEnum, refundAmount, caseEntity);

        String targetStatus = switch (outcomeEnum) {
            case FULL_REFUND, PARTIAL_REFUND -> AfterSaleCaseStatus.REFUND_PROCESSING.value();
            case REJECT_APPEAL -> AfterSaleCaseStatus.CLOSED.value();
        };

        // 樂觀鎖 CAS：appeal_state=APPEALED → RESOLVED
        java.time.LocalDateTime now = DateTimeUtils.nowUtc();
        int affected = caseMapper.markAppealResolved(caseNo, targetStatus, now);
        if (affected != 1) {
            throw new AfterSalesBusinessException(GrpcResponseCode.AFTERSALE_APPEAL_STATE_MISMATCH);
        }

        // INSERT aftersale_platform_decisions
        AfterSalePlatformDecisionEntity decision = new AfterSalePlatformDecisionEntity();
        decision.setCaseNo(caseNo);
        decision.setOutcome(outcome);
        decision.setRefundAmount(finalAmount);
        decision.setReason(reason);
        decision.setDecidedByAdminId(adminAccountId);
        // decided_at 不走 MetaObjectHandler（非標準 createdAt 命名），顯式填入
        decision.setDecidedAt(now);
        platformDecisionMapper.insert(decision);

        // INSERT aftersale_resolutions（source=PLATFORM，歷史記錄；不觸發退款）
        AfterSaleResolutionEntity resolution = new AfterSaleResolutionEntity();
        resolution.setId(TsidUtil.generate());
        resolution.setCaseNo(caseNo);
        resolution.setResolutionSource(AfterSaleResolutionSource.PLATFORM.value());
        resolution.setResolutionType(mapOutcomeToResolutionType(outcomeEnum));
        resolution.setRefundAmount(finalAmount);
        resolution.setResolvedBy(adminAccountId);
        resolution.setNegotiationReason(reason);
        resolutionMapper.insert(resolution);

        // INSERT appeal_events
        AfterSaleAppealEventEntity event = new AfterSaleAppealEventEntity();
        event.setCaseNo(caseNo);
        event.setEventType(AfterSaleAppealEventType.DECISION_MADE.value());
        event.setActorType("ADMIN");
        event.setActorId(adminAccountId);
        event.setLabel("平台裁決");
        event.setDescription(outcomeDescription(outcomeEnum, finalAmount) + "：" + reason);
        appealEventMapper.insert(event);

        // FULL/PARTIAL_REFUND: 建 refund_tasks
        RefundTaskEntity refundTask = null;
        if (outcomeEnum != AfterSalePlatformDecisionOutcome.REJECT_APPEAL) {
            refundTask = createRefundTask(caseEntity, finalAmount, outcomeEnum);
        }

        // publish MQ（順序：decision.made → awaiting_buyer_page if ATM）
        eventPublisher.publishPlatformDecisionMade(caseNo, caseEntity.getOrderNo(),
                caseEntity.getBuyerId(), caseEntity.getStoreId(),
                outcome, finalAmount);

        if (refundTask != null
                && PaymentMethod.ATM.value().equals(caseEntity.getPaymentMethod())) {
            eventPublisher.publishAwaitingBuyerRefundPage(
                    caseEntity.getOrderNo(), caseNo, refundTask.getId(), caseEntity.getBuyerId());
        }

        log.info("PlatformDecisionService.decide.success",
                "平台裁決完成, outcome=" + outcome + ", amount=" + finalAmount, "caseNo", caseNo);
    }

    /**
     * 計算最終退款金額；同時驗證 outcome × refund_amount 組合。
     */
    private Long computeFinalAmount(AfterSalePlatformDecisionOutcome outcome,
                                     Long refundAmount, AfterSaleCaseEntity caseEntity) {
        switch (outcome) {
            case FULL_REFUND -> {
                if (refundAmount != null && refundAmount > 0) {
                    throw new AfterSalesBusinessException(
                            GrpcResponseCode.AFTERSALE_PLATFORM_DECISION_INVALID_AMOUNT);
                }
                return caseEntity.getRefundableAmount();
            }
            case PARTIAL_REFUND -> {
                if (refundAmount == null || refundAmount <= 0
                        || refundAmount > caseEntity.getRefundableAmount()) {
                    throw new AfterSalesBusinessException(
                            GrpcResponseCode.AFTERSALE_PLATFORM_DECISION_INVALID_AMOUNT);
                }
                return refundAmount;
            }
            case REJECT_APPEAL -> {
                if (refundAmount != null && refundAmount > 0) {
                    throw new AfterSalesBusinessException(
                            GrpcResponseCode.AFTERSALE_PLATFORM_DECISION_INVALID_AMOUNT);
                }
                return null;
            }
            default -> throw new AfterSalesBusinessException(GrpcResponseCode.INVALID_PARAM);
        }
    }

    private String mapOutcomeToResolutionType(AfterSalePlatformDecisionOutcome outcome) {
        return switch (outcome) {
            case FULL_REFUND -> AfterSaleResolutionType.FULL_REFUND_NO_RETURN.value();
            case PARTIAL_REFUND -> AfterSaleResolutionType.PARTIAL_REFUND.value();
            case REJECT_APPEAL -> AfterSaleResolutionType.REJECT.value();
        };
    }

    private String outcomeDescription(AfterSalePlatformDecisionOutcome outcome, Long amount) {
        return switch (outcome) {
            case FULL_REFUND -> "全額退款 NT$ " + amount;
            case PARTIAL_REFUND -> "部分退款 NT$ " + amount;
            case REJECT_APPEAL -> "駁回申訴";
        };
    }

    /**
     * 建立退款任務（與 AfterSaleResolutionService 同樣邏輯，依 payment_method 決定初始 status）。
     * outcome 由 caller 從 AfterSalePlatformDecisionOutcome（FULL_REFUND / PARTIAL_REFUND）直接傳入；
     * REJECT_APPEAL 不會走到此 method。
     */
    private RefundTaskEntity createRefundTask(AfterSaleCaseEntity caseEntity, Long refundAmount,
                                                AfterSalePlatformDecisionOutcome outcome) {
        boolean isAtm = PaymentMethod.ATM.value().equals(caseEntity.getPaymentMethod());
        RefundTaskEntity task = new RefundTaskEntity();
        task.setId(TsidUtil.generate());
        task.setCaseNo(caseEntity.getCaseNo());
        task.setOrderNo(caseEntity.getOrderNo());
        task.setTransactionId(caseEntity.getTransactionId());
        task.setRefundAmount(refundAmount);
        task.setPaymentMethod(caseEntity.getPaymentMethod());
        task.setMerId(caseEntity.getMerId());
        task.setRefundOutcome(outcome.value());
        task.setStatus(isAtm
                ? WorkerTaskStatus.AWAITING_BUYER_REFUND_PAGE.value()
                : WorkerTaskStatus.PENDING.value());
        task.setRetryCount(0);
        task.setMaxRetryCount(properties.getRefund().getMaxRetryCount());
        refundTaskMapper.insert(task);
        return task;
    }
}

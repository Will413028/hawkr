package io.github.will413028.hawkr.coupon.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.*;
import io.github.will413028.hawkr.coupon.entity.CouponInstanceEntity;
import io.github.will413028.hawkr.coupon.entity.CouponRedemptionLogEntity;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import io.github.will413028.hawkr.coupon.event.CouponEventPublisher;
import io.github.will413028.hawkr.coupon.exception.CouponBusinessException;
import io.github.will413028.hawkr.coupon.mapper.CouponInstanceMapper;
import io.github.will413028.hawkr.coupon.mapper.CouponRedemptionLogMapper;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.CouponTargetType;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.List;

@Service
public class CouponInstanceService {

    private static final ObsLogger log = ObsLogger.forClass(CouponInstanceService.class);
    private static final String COUPON_ALREADY_USED_MESSAGE_PREFIX = "此券已用於訂單 ";
    private static final String COUPON_LOCKED_BY_ORDER_MESSAGE_PREFIX = "此券正用於訂單 ";
    private static final String COUPON_NOT_APPLICABLE_TO_SELLER_MESSAGE = "優惠券不適用此賣家";
    private final CouponInstanceMapper instanceMapper;
    private final CouponRedemptionLogMapper logMapper;
    private final CouponTemplateService templateService;
    private final CouponEventPublisher eventPublisher;
    private final CouponClaimService claimService;

    public CouponInstanceService(CouponInstanceMapper instanceMapper,
                                  CouponRedemptionLogMapper logMapper,
                                  CouponTemplateService templateService,
                                  CouponEventPublisher eventPublisher,
                                  CouponClaimService claimService) {
        this.instanceMapper = instanceMapper;
        this.logMapper = logMapper;
        this.templateService = templateService;
        this.eventPublisher = eventPublisher;
        this.claimService = claimService;
    }

    /**
     * 驗證優惠券模板（不消耗、不鎖定）
     * 基於 templateId 直接驗證模板，不需要預先建立 Instance。
     */
    public ValidateResult validateCoupon(Long templateId, Long buyerId,
                                          Long storeId, Long orderAmount) {
        CouponTemplateEntity template = templateService.getById(templateId);
        if (template == null) {
            return ValidateResult.invalid(GrpcResponseCode.COUPON_NOT_FOUND.message());
        }

        // 模板狀態
        if (!CouponTemplateStatus.ACTIVE.value().equals(template.getStatus())) {
            return ValidateResult.invalid(GrpcResponseCode.COUPON_CLAIM_REQUIRES_ACTIVE.message());
        }

        // 時間檢查
        LocalDateTime now = DateTimeUtils.nowUtc();
        if (now.isBefore(template.getStartAt()) || now.isAfter(template.getEndAt())) {
            return ValidateResult.invalid(GrpcResponseCode.COUPON_CLAIM_OUT_OF_WINDOW.message());
        }

        // 檢查買家是否已有 USED 或 RESERVED 的 Instance（依 §2.6.1 業務規則）
        // - USED：此券已用於某張已付款訂單，不可再用
        // - RESERVED：此券正鎖定於某張未付款訂單，須先完成付款或等該訂單被取消
        // 訊息精確帶上對應 orderNo，讓前端能引導買家「去完成 / 取消」上一張單
        CouponInstanceEntity existingInstance = findByTemplateAndOwner(templateId, buyerId);
        if (existingInstance != null) {
            String existStatus = existingInstance.getStatus();
            if (CouponInstanceStatus.USED.value().equals(existStatus)) {
                return ValidateResult.invalid(
                        COUPON_ALREADY_USED_MESSAGE_PREFIX + existingInstance.getUsedOrderNo());
            }
            if (CouponInstanceStatus.RESERVED.value().equals(existStatus)) {
                return ValidateResult.invalid(
                        COUPON_LOCKED_BY_ORDER_MESSAGE_PREFIX + existingInstance.getReservedOrderNo());
            }
        }

        // Scope 檢查
        if (template.getApplicableStoreId() != null && storeId != null
                && !template.getApplicableStoreId().equals(storeId)) {
            return ValidateResult.invalid(COUPON_NOT_APPLICABLE_TO_SELLER_MESSAGE);
        }

        // 領取檢查
        if (Boolean.TRUE.equals(template.getClaimRequired())) {
            boolean claimed = claimService.isClaimed(template.getTemplateId(), buyerId);
            if (!claimed) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_NOT_CLAIMED,
                        GrpcResponseCode.COUPON_NOT_CLAIMED.message());
            }
        }

        // 門檻檢查
        if (orderAmount < template.getThresholdAmount()) {
            return ValidateResult.invalid("未達滿額門檻 " + template.getThresholdAmount() + " 元");
        }

        // 計算折扣
        long discount = calculateDiscount(template, orderAmount);
        String description = buildDescription(template);

        return new ValidateResult(true, discount, template.getType(), description, templateId);
    }

    /**
     * 鎖定優惠券（結帳時建立 Instance）
     * Instance 在此時才被建立，狀態直接為 RESERVED。
     * 利用 (template_id, owner_buyer_id) 唯一約束保證冪等。
     */
    @Transactional
    public Long reserveCoupon(Long templateId, Long buyerId, String orderNo,
                               Long applicableStoreId) {
        // 領取檢查
        CouponTemplateEntity template = templateService.getById(templateId);
        if (template != null && Boolean.TRUE.equals(template.getClaimRequired())) {
            boolean claimed = claimService.isClaimed(templateId, buyerId);
            if (!claimed) {
                throw new CouponBusinessException(GrpcResponseCode.COUPON_NOT_CLAIMED,
                        GrpcResponseCode.COUPON_NOT_CLAIMED.message());
            }
        }

        // Pre-check 既有 instance（避開 UNIQUE 違規導致 transaction abort）
        CouponInstanceEntity existing = findByTemplateAndOwner(templateId, buyerId);
        if (existing != null) {
            String status = existing.getStatus();
            if (CouponInstanceStatus.RESERVED.value().equals(status)) {
                // 冪等：同 orderNo 直接回傳既有 instance
                if (orderNo.equals(existing.getReservedOrderNo())) {
                    return existing.getInstanceId();
                }
                // 被另一筆訂單鎖住
                throw new CouponBusinessException(GrpcResponseCode.COUPON_LOCKED_BY_OTHER_ORDER);
            }
            if (CouponInstanceStatus.UNUSED.value().equals(status)) {
                // 之前 release 後重用：UNUSED → RESERVED
                int affected = instanceMapper.reReserveCoupon(existing.getInstanceId(), orderNo);
                if (affected == 0) {
                    // 並發競爭：被別人搶先 reserve，重查確認
                    CouponInstanceEntity rechecked = findByTemplateAndOwner(templateId, buyerId);
                    if (rechecked != null
                            && CouponInstanceStatus.RESERVED.value().equals(rechecked.getStatus())
                            && orderNo.equals(rechecked.getReservedOrderNo())) {
                        return rechecked.getInstanceId();
                    }
                    throw new CouponBusinessException(GrpcResponseCode.COUPON_LOCKED_BY_OTHER_ORDER);
                }
                writeLog(existing.getInstanceId(), templateId, buyerId,
                        CouponRedemptionAction.RESERVE, orderNo, null, null);
                log.info("CouponInstanceService.reserveCoupon.reuseUnused",
                        "重用 UNUSED instance=" + existing.getInstanceId()
                                + " templateId=" + templateId + " orderNo=" + orderNo);
                return existing.getInstanceId();
            }
            // USED / REVOKED / ANNUL / EXPIRED → 不可用
            throw new CouponBusinessException(GrpcResponseCode.COUPON_ALREADY_USED);
        }

        // 全新 instance：直接 INSERT
        CouponInstanceEntity instance = new CouponInstanceEntity();
        instance.setInstanceId(TsidUtil.generate());
        instance.setTemplateId(templateId);
        instance.setOwnerBuyerId(buyerId);
        instance.setApplicableStoreId(applicableStoreId);
        instance.setStatus(CouponInstanceStatus.RESERVED.value());
        instance.setReservedOrderNo(orderNo);
        instance.setReservedAt(DateTimeUtils.nowUtc());
        instanceMapper.insert(instance);

        writeLog(instance.getInstanceId(), templateId, buyerId,
                CouponRedemptionAction.RESERVE, orderNo, null, null);

        log.info("CouponInstanceService.reserveCoupon.success",
                "鎖定優惠券 instanceId=" + instance.getInstanceId()
                        + " templateId=" + templateId + " orderNo=" + orderNo);

        return instance.getInstanceId();
    }

    /**
     * 消耗優惠券
     */
    public boolean consumeCoupon(Long instanceId, String orderNo) {
        int affected = instanceMapper.consumeCoupon(instanceId, orderNo);
        if (affected > 0) {
            writeLog(instanceId, null, null, CouponRedemptionAction.CONSUME, orderNo, null, null);
            return true;
        }
        // 冪等：已為 USED 且 order_no 匹配
        return instanceMapper.countUsedWithOrderNo(instanceId, orderNo) > 0;
    }

    /**
     * 釋放優惠券
     */
    public boolean releaseCoupon(Long instanceId, String orderNo) {
        int affected = instanceMapper.releaseCoupon(instanceId, orderNo);
        if (affected > 0) {
            writeLog(instanceId, null, null, CouponRedemptionAction.RELEASE, orderNo, null, null);
            return true;
        }
        // 冪等：已為 UNUSED
        return instanceMapper.countUnused(instanceId) > 0;
    }

    /**
     * 批量收回（寫入彙總稽核 log）
     */
    public int revokeByTemplate(Long templateId, String reason) {
        int revokedCount = instanceMapper.revokeByTemplate(templateId, reason);
        if (revokedCount > 0) {
            writeLog(0L, templateId, null,
                    CouponRedemptionAction.REVOKE, null, null,
                    "批量收回 " + revokedCount + " 張, 原因: " + reason);
        }
        return revokedCount;
    }

    /**
     * Admin 作廢
     */
    public boolean voidInstance(Long instanceId, String reason, Long adminAccountId) {
        int affected = instanceMapper.voidInstance(instanceId, reason);
        if (affected > 0) {
            writeLog(instanceId, null, null, CouponRedemptionAction.ANNUL, null, adminAccountId, reason);
            return true;
        }
        return false;
    }

    /**
     * 買家刪除批量作廢（07 §5.19 → W2/W3 設計書 §3.2 D6 遷移，2026-07-16）
     * 來源：BuyerDeletedConsumer 消費 hawkr.buyer / buyer.deleted MQ
     * SQL 僅 UNUSED → ANNUL；冪等：affected = 0 表示無 UNUSED 可作廢，正常重跑行為。
     * 回傳本次實際作廢數量供 caller 寫日誌。
     *
     * <p>{@code ownerBuyerId} 參數名（buyer_id 8-service rekey Phase 5 更新，2026-07-18：
     * 取代 W2/W3 設計書 D9 當時「命名一致性改名延後」的決議，欄位與參數名現已同步）。
     */
    @Transactional
    public int voidAllUnusedByOwner(Long ownerBuyerId, String reason) {
        int affected = instanceMapper.voidAllUnusedByOwner(ownerBuyerId, reason);
        if (affected > 0) {
            writeLog(0L, null, ownerBuyerId, CouponRedemptionAction.ANNUL, null, null,
                    "buyer.deleted batch void " + affected + " instances by buyerId=" + ownerBuyerId);
        }
        return affected;
    }

    /**
     * 查詢買家的優惠券列表（僅包含 RESERVED/USED 等已建立的 Instance）
     */
    public Page<CouponInstanceEntity> listByOwner(Long ownerAccountId, String status, int page, int pageSize) {
        LambdaQueryWrapper<CouponInstanceEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponInstanceEntity::getOwnerBuyerId, ownerAccountId);
        if (status != null && !status.isBlank()) {
            wrapper.eq(CouponInstanceEntity::getStatus, status);
        }
        wrapper.orderByDesc(CouponInstanceEntity::getCreatedAt);
        return instanceMapper.selectPage(new Page<>(page, pageSize), wrapper);
    }

    /**
     * 查詢賣家的買家持券
     */
    public Page<CouponInstanceEntity> listBuyerCouponsForSeller(Long storeId, Long buyerId,
                                                                  String status, int page, int pageSize) {
        LambdaQueryWrapper<CouponInstanceEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponInstanceEntity::getApplicableStoreId, storeId)
               .eq(CouponInstanceEntity::getOwnerBuyerId, buyerId);
        if (status != null && !status.isBlank()) {
            wrapper.eq(CouponInstanceEntity::getStatus, status);
        }
        wrapper.orderByDesc(CouponInstanceEntity::getCreatedAt);
        return instanceMapper.selectPage(new Page<>(page, pageSize), wrapper);
    }

    /**
     * 統計模板下的已使用/未使用數量
     */
    public int countByTemplateAndStatus(Long templateId, String status) {
        return instanceMapper.countByTemplateAndStatus(templateId, status);
    }

    /**
     * 對帳：撈出 reserved_at 早於指定時間且仍 RESERVED 的 instance（給對帳 Job 用）
     */
    public List<CouponInstanceEntity> findStaleReserved(LocalDateTime olderThan, int limit) {
        return instanceMapper.findStaleReserved(olderThan, limit);
    }

    /**
     * 自動過期
     */
    public int expireInstancesByTemplate() {
        return instanceMapper.expireInstancesByTemplate();
    }

    public CouponInstanceEntity getById(Long instanceId) {
        return instanceMapper.selectById(instanceId);
    }

    // ==================== 內部方法 ====================

    /**
     * 根據 templateId + ownerBuyerId 查詢 Instance（唯一約束查詢）
     */
    private CouponInstanceEntity findByTemplateAndOwner(Long templateId, Long ownerBuyerId) {
        LambdaQueryWrapper<CouponInstanceEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponInstanceEntity::getTemplateId, templateId)
               .eq(CouponInstanceEntity::getOwnerBuyerId, ownerBuyerId)
               .last("LIMIT 1");
        return instanceMapper.selectOne(wrapper);
    }

    /**
     * 計算模板對指定金額的折扣（public，供 GrpcService 計算 estimatedDiscount）
     */
    public long calculateDiscountForTemplate(CouponTemplateEntity template, long orderAmount) {
        return calculateDiscount(template, orderAmount);
    }

    private long calculateDiscount(CouponTemplateEntity template, long orderAmount) {
        CouponType type = CouponType.fromValue(template.getType());
        return switch (type) {
            case CASH -> Math.min(template.getDiscountAmount(), orderAmount);
            case PERCENT -> {
                long rawDiscount = orderAmount * template.getDiscountPercent() / 100;
                yield Math.min(rawDiscount, template.getMaxDiscountAmount());
            }
        };
    }

    private String buildDescription(CouponTemplateEntity template) {
        CouponType type = CouponType.fromValue(template.getType());
        return switch (type) {
            case CASH -> {
                if (template.getThresholdAmount() > 0) {
                    yield "滿" + template.getThresholdAmount() + "折" + template.getDiscountAmount();
                }
                yield "折扣" + template.getDiscountAmount() + "元";
            }
            case PERCENT -> {
                int off = 100 - template.getDiscountPercent();
                if (template.getThresholdAmount() > 0) {
                    yield "滿" + template.getThresholdAmount() + "打" + off / 10 + "." + off % 10 + "折";
                }
                yield template.getDiscountPercent() + "% off";
            }
        };
    }

    private void writeLog(Long instanceId, Long templateId, Long ownerBuyerId,
                           CouponRedemptionAction action, String orderNo, Long operatorAccountId, String detail) {
        try {
            CouponRedemptionLogEntity logEntity = new CouponRedemptionLogEntity();
            logEntity.setInstanceId(instanceId);
            logEntity.setTemplateId(templateId != null ? templateId : 0L);
            logEntity.setOwnerBuyerId(ownerBuyerId != null ? ownerBuyerId : 0L);
            logEntity.setAction(action.value());
            logEntity.setOrderNo(orderNo);
            logEntity.setOperatorAccountId(operatorAccountId);
            logEntity.setTraceId("");
            logEntity.setRequestId("");
            logEntity.setDetail(detail);
            logMapper.insert(logEntity);
        } catch (Exception e) {
            log.error("CouponInstanceService.writeLog.failed", "稽核日誌寫入失敗", e);
        }
    }

    /**
     * 驗證結果
     */
    public record ValidateResult(boolean valid, long discountAmount, String discountType,
                                  String description, long templateId) {
        public static ValidateResult invalid(String description) {
            return new ValidateResult(false, 0, "", description, 0L);
        }
    }
}

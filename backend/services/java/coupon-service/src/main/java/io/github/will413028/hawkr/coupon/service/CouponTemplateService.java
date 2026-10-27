package io.github.will413028.hawkr.coupon.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import com.baomidou.mybatisplus.extension.plugins.pagination.Page;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.*;
import io.github.will413028.hawkr.coupon.constant.CouponTargetTypeRules;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import io.github.will413028.hawkr.coupon.exception.CouponBusinessException;
import io.github.will413028.hawkr.coupon.mapper.CouponTemplateMapper;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import org.springframework.stereotype.Service;

import java.time.Instant;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Set;

@Service
public class CouponTemplateService {

    private static final ObsLogger log = ObsLogger.forClass(CouponTemplateService.class);
    private final CouponTemplateMapper templateMapper;

    public CouponTemplateService(CouponTemplateMapper templateMapper) {
        this.templateMapper = templateMapper;
    }

    /**
     * 建立模板（Admin 用，DRAFT 狀態）
     */
    public CouponTemplateEntity createTemplate(Long adminAccountId, String title, String type,
                                                String targetType,
                                                Long applicableStoreId,
                                                Long thresholdAmount, Long discountAmount,
                                                Integer discountPercent, Long maxDiscountAmount,
                                                LocalDateTime startAt, LocalDateTime endAt,
                                                Integer usageLimitPerAccount, Integer totalIssueLimit,
                                                List<Long> buyerIds,
                                                Boolean claimRequired) {
        validateTemplateParams(type, thresholdAmount, discountAmount, discountPercent, maxDiscountAmount, startAt, endAt);

        CouponTemplateEntity entity = new CouponTemplateEntity();
        entity.setTemplateId(TsidUtil.generate());
        entity.setTitle(title);
        entity.setType(type);
        entity.setTargetType(targetType);
        entity.setApplicableStoreId(applicableStoreId);
        entity.setIssuerType(IssuerType.ADMIN.value());
        entity.setIssuerAccountId(adminAccountId);
        entity.setThresholdAmount(thresholdAmount != null ? thresholdAmount : 0L);
        entity.setDiscountAmount(discountAmount);
        entity.setDiscountPercent(discountPercent);
        entity.setMaxDiscountAmount(maxDiscountAmount);
        entity.setStartAt(startAt);
        entity.setEndAt(endAt);
        entity.setUsageLimitPerAccount(usageLimitPerAccount != null ? usageLimitPerAccount : 1);
        entity.setTotalIssueLimit(totalIssueLimit != null ? totalIssueLimit : -1);
        entity.setIssuedCount(0);
        entity.setClaimRequired(claimRequired != null ? claimRequired : true);
        entity.setStatus(CouponTemplateStatus.DRAFT.value());

        templateMapper.insert(entity);
        log.info("CouponTemplateService.createTemplate.success", "建立模板", "templateId", String.valueOf(entity.getTemplateId()));
        return entity;
    }

    /**
     * 賣家即建即發模板（ACTIVE 狀態）
     *
     * @param storeId 呼叫方須傳入 gateway 已解析的 store_id surrogate（Plan C Phase 1
     *                語意修正 —— 呼叫端曾經誤傳原始 JWT accountId，見 CouponGrpcService）
     */
    public CouponTemplateEntity createSellerTemplate(Long storeId, String title, String type,
                                                      String targetType,
                                                      Long thresholdAmount, Long discountAmount,
                                                      Integer discountPercent, Long maxDiscountAmount,
                                                      LocalDateTime startAt, LocalDateTime endAt) {
        validateTemplateParams(type, thresholdAmount, discountAmount, discountPercent, maxDiscountAmount, startAt, endAt);

        CodeMetaValidator.requireValidValue(CouponTargetType.class, targetType, "targetType");
        if (!CouponTargetTypeRules.isSellerAllowed(CouponTargetType.fromValue(targetType))) {
            throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "賣家不允許建立此 targetType: " + targetType);
        }

        CouponTemplateEntity entity = new CouponTemplateEntity();
        entity.setTemplateId(TsidUtil.generate());
        entity.setTitle(title);
        entity.setType(type);
        entity.setTargetType(targetType);
        entity.setApplicableStoreId(storeId);
        entity.setIssuerType(IssuerType.SELLER.value());
        entity.setIssuerStoreId(storeId);
        entity.setThresholdAmount(thresholdAmount != null ? thresholdAmount : 0L);
        entity.setDiscountAmount(discountAmount);
        entity.setDiscountPercent(discountPercent);
        entity.setMaxDiscountAmount(maxDiscountAmount);
        entity.setStartAt(startAt);
        entity.setEndAt(endAt);
        entity.setUsageLimitPerAccount(1);
        entity.setClaimRequired(false);
        entity.setStatus(CouponTemplateStatus.ACTIVE.value());

        templateMapper.insert(entity);
        log.info("CouponTemplateService.createSellerTemplate.success", "建立賣家模板",
                "templateId", String.valueOf(entity.getTemplateId()));
        return entity;
    }

    /**
     * 建立補償券模板（Admin 發起）
     *
     * <p>{@code applicableStoreId} 必填：補償券針對單一筆退款／糾紛發放，適用範圍
     * 就是那筆訂單所屬的商店。{@code applicable_store_id} 為 NULL 在 Scope 檢查裡
     * 等同「任何商店都能用」（設計書 07 §2.3），因此這裡是防止「缺值被靜默放大成
     * 全平台券」的收斂點——呼叫端無論走 gRPC 或內部直呼都繞不過。平台級活動券請走
     * {@link #createTemplate}（設計書 07 §5.15，該處 scope 才是選填）。
     */
    public CouponTemplateEntity createCompensationTemplate(Long adminAccountId, Long amount,
                                                            Long applicableStoreId) {
        if (applicableStoreId == null || applicableStoreId <= 0) {
            throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "補償券必須指定適用商店 applicable_store_id");
        }
        CouponTemplateEntity entity = new CouponTemplateEntity();
        entity.setTemplateId(TsidUtil.generate());
        entity.setTitle("退款補償券");
        entity.setType(CouponType.CASH.value());
        entity.setTargetType(CouponTargetType.COMPENSATION.value());
        entity.setApplicableStoreId(applicableStoreId);
        entity.setIssuerType(IssuerType.ADMIN.value());
        entity.setIssuerAccountId(adminAccountId);
        entity.setThresholdAmount(0L);
        entity.setDiscountAmount(amount);
        entity.setStartAt(DateTimeUtils.nowUtc());
        entity.setEndAt(DateTimeUtils.nowUtc().plusMonths(3));
        entity.setUsageLimitPerAccount(1);
        entity.setTotalIssueLimit(1);
        entity.setIssuedCount(1);
        entity.setClaimRequired(false);
        entity.setStatus(CouponTemplateStatus.ACTIVE.value());

        templateMapper.insert(entity);
        return entity;
    }

    /**
     * 建立賣家補償券模板（售後流程中賣家決策發放）
     * <p>
     * 無到期日、限指定賣家使用、一次性發放。
     *
     * <p>{@code storeId} 呼叫端目前為 aftersales-service（Phase 3 尚未執行 rekey），
     * 現階段傳入值仍可能是原始 account_id —— 這是 Phase 3 的責任範圍，此方法簽名
     * 先行更名以便 Phase 3 落地時語意一致，本階段不強制呼叫端改值。
     */
    public CouponTemplateEntity createSellerCompensationTemplate(Long storeId, Long amount,
                                                                   Long minOrderAmount, String caseNo) {
        CouponTemplateEntity entity = new CouponTemplateEntity();
        entity.setTemplateId(TsidUtil.generate());
        entity.setTitle("售後補償券 " + (caseNo != null ? caseNo : ""));
        entity.setType(CouponType.CASH.value());
        entity.setTargetType(CouponTargetType.COMPENSATION.value());
        entity.setApplicableStoreId(storeId);
        entity.setIssuerType(IssuerType.SELLER.value());
        entity.setIssuerStoreId(storeId);
        entity.setThresholdAmount(minOrderAmount != null ? minOrderAmount : 0L);
        entity.setDiscountAmount(amount);
        entity.setStartAt(DateTimeUtils.nowUtc());
        // 無到期日：設定一個足夠遠的未來日期
        entity.setEndAt(LocalDateTime.of(2099, 12, 31, 23, 59, 59));
        entity.setUsageLimitPerAccount(1);
        entity.setTotalIssueLimit(1);
        entity.setIssuedCount(1);
        entity.setClaimRequired(false);
        entity.setStatus(CouponTemplateStatus.ACTIVE.value());

        templateMapper.insert(entity);
        log.info("CouponTemplateService.createSellerCompensationTemplate.success",
                "建立賣家補償券模板 templateId=" + entity.getTemplateId() + " caseNo=" + (caseNo != null ? caseNo : ""));
        return entity;
    }

    public CouponTemplateEntity getById(Long templateId) {
        return templateMapper.selectById(templateId);
    }

    public CouponTemplateEntity getActiveTemplate(Long templateId) {
        CouponTemplateEntity template = templateMapper.selectById(templateId);
        if (template == null) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_NOT_FOUND);
        }
        return template;
    }

    /**
     * 發佈模板（DRAFT → ACTIVE）
     */
    public void publishTemplate(Long templateId) {
        CouponTemplateEntity template = getActiveTemplate(templateId);
        if (!CouponTemplateStatus.DRAFT.value().equals(template.getStatus())) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_PUBLISH_REQUIRES_DRAFT);
        }
        template.setStatus(CouponTemplateStatus.ACTIVE.value());
        templateMapper.updateById(template);
    }

    /**
     * 停用模板（ACTIVE → INACTIVE）
     */
    public void disableTemplate(Long templateId) {
        CouponTemplateEntity template = getActiveTemplate(templateId);
        if (!CouponTemplateStatus.ACTIVE.value().equals(template.getStatus())) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_DISABLE_REQUIRES_ACTIVE);
        }
        template.setStatus(CouponTemplateStatus.INACTIVE.value());
        templateMapper.updateById(template);
    }

    /**
     * 賣家停權連動停用（07 §5.20）：把該賣家自建的 ACTIVE 模板批次停用為 INACTIVE，
     * 並標記 {@code disabled_by_suspension=TRUE} 供解除停權時精準還原。
     *
     * @param storeId 停權事件攜帶的 store_id（Plan C Phase 0 擴充，非事件的
     *                accountId — 見 SellerSuspendedConsumer，Task 2 of this plan）
     * @return 實際停用的模板數（0 = 無符合條件模板，天然冪等）
     */
    public int disableTemplatesBySuspension(Long storeId) {
        int affected = templateMapper.disableActiveTemplatesBySuspension(
                storeId, IssuerType.SELLER.value(),
                CouponTemplateStatus.ACTIVE.value(), CouponTemplateStatus.INACTIVE.value());
        log.info("CouponTemplateService.disableTemplatesBySuspension.done",
                "賣家停權連動停用模板 storeId=" + storeId + " affected=" + affected);
        return affected;
    }

    /**
     * 賣家解除停權精準還原（07 §5.20）：只還原 {@code disabled_by_suspension=TRUE} 的模板。
     *
     * @return 實際還原的模板數
     */
    public int restoreTemplatesFromSuspension(Long storeId) {
        int affected = templateMapper.restoreTemplatesFromSuspension(
                storeId, IssuerType.SELLER.value(),
                CouponTemplateStatus.ACTIVE.value(), CouponTemplateStatus.INACTIVE.value());
        log.info("CouponTemplateService.restoreTemplatesFromSuspension.done",
                "賣家解除停權還原模板 storeId=" + storeId + " affected=" + affected);
        return affected;
    }

    /**
     * 防超發遞增 issued_count
     */
    public boolean incrementIssuedCount(Long templateId, int count) {
        return templateMapper.incrementIssuedCount(templateId, count) > 0;
    }

    /**
     * 賣家收回模板
     *
     * <p>固定傳 {@code IssuerType.SELLER.value()} 給 mapper 當 WHERE issuer_type 過濾
     * 條件——這裡只可能是賣家自建模板的收回動作，不會是 admin 模板（比照
     * {@link #disableTemplatesBySuspension} 的既有作法）。
     */
    public boolean revokeTemplate(Long templateId, Long storeId, String reason) {
        return templateMapper.revokeTemplate(templateId, storeId, IssuerType.SELLER.value(), reason) > 0;
    }

    /**
     * 查詢可領取模板列表
     */
    public Page<CouponTemplateEntity> listAvailable(int page, int pageSize) {
        return listAvailable(page, pageSize, null);
    }

    /**
     * 查詢可領取的優惠券模板
     *
     * @param storeId 篩選商家（null = 全部，有值 = 該商家券 + 全平台券）
     */
    public Page<CouponTemplateEntity> listAvailable(int page, int pageSize, Long storeId) {
        LambdaQueryWrapper<CouponTemplateEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponTemplateEntity::getStatus, CouponTemplateStatus.ACTIVE.value())
               .le(CouponTemplateEntity::getStartAt, DateTimeUtils.nowUtc())
               .ge(CouponTemplateEntity::getEndAt, DateTimeUtils.nowUtc());

        // 篩選商家：該商家專屬券 + 全平台券（applicableStoreId IS NULL）
        if (storeId != null) {
            wrapper.and(w -> w
                    .eq(CouponTemplateEntity::getApplicableStoreId, storeId)
                    .or()
                    .isNull(CouponTemplateEntity::getApplicableStoreId));
        }

        wrapper.orderByDesc(CouponTemplateEntity::getCreatedAt);

        return templateMapper.selectPage(new Page<>(page, pageSize), wrapper);
    }

    /**
     * 買家視角列表查詢：可見性規則下放至 SQL，過濾先於分頁。
     * 對應設計書 §6.7、§6.8、§6.9。
     *
     * @param storeId 篩選商家（null=全平台券；有值=該商家券 + 全平台券）
     * @param buyerId         當前買家 account_id
     * @param buyerCreatedAtMs 買家註冊時間（epoch ms, UTC）
     * @param memberSellerIds 買家有完成交易紀錄的賣家集合（type=2 比對用）
     */
    public Page<CouponTemplateEntity> listAvailableForBuyer(int page, int pageSize,
                                                             Long storeId,
                                                             Long buyerId,
                                                             long buyerCreatedAtMs,
                                                             Set<Long> memberSellerIds) {
        LocalDateTime buyerCreatedAt = LocalDateTime.ofInstant(
                Instant.ofEpochMilli(buyerCreatedAtMs), ZoneOffset.UTC);

        // 空集合保底：MyBatis foreach 對空 Collection 會產生 IN () 非法 SQL，
        // 塞入哨兵 -1L（任何 seller_id 都不會等於 -1，type=2 分支自然 false）
        Collection<Long> sellers = (memberSellerIds == null || memberSellerIds.isEmpty())
                ? List.of(-1L)
                : new ArrayList<>(memberSellerIds);

        Page<CouponTemplateEntity> pageObj = new Page<>(page, pageSize);
        return (Page<CouponTemplateEntity>) templateMapper.selectAvailableForBuyer(
                pageObj, storeId, buyerId, buyerCreatedAt, sellers);
    }

    /**
     * 查詢賣家的模板列表
     */
    public Page<CouponTemplateEntity> listSellerTemplates(Long storeId, String status, int page, int pageSize) {
        LambdaQueryWrapper<CouponTemplateEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponTemplateEntity::getIssuerType, IssuerType.SELLER.value())
               .eq(CouponTemplateEntity::getIssuerStoreId, storeId);
        if (status != null && !status.isBlank()) {
            wrapper.eq(CouponTemplateEntity::getStatus, status);
        }
        wrapper.orderByDesc(CouponTemplateEntity::getCreatedAt);

        return templateMapper.selectPage(new Page<>(page, pageSize), wrapper);
    }

    /**
     * 統計賣家當前 ACTIVE 模板數（排除補償券）
     */
    public long countSellerActiveTemplates(Long storeId) {
        LambdaQueryWrapper<CouponTemplateEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponTemplateEntity::getIssuerType, IssuerType.SELLER.value())
               .eq(CouponTemplateEntity::getIssuerStoreId, storeId)
               .eq(CouponTemplateEntity::getStatus, CouponTemplateStatus.ACTIVE.value())
               .ne(CouponTemplateEntity::getTargetType, CouponTargetType.COMPENSATION.value());
        return templateMapper.selectCount(wrapper);
    }

    /**
     * 自動過期
     */
    public int expireTemplates() {
        return templateMapper.expireTemplates();
    }

    private void validateTemplateParams(String type, Long thresholdAmount, Long discountAmount,
                                         Integer discountPercent,
                                         Long maxDiscountAmount,
                                         LocalDateTime startAt, LocalDateTime endAt) {
        // thresholdAmount 可省略（=0 表示無門檻），但禁止負數
        if (thresholdAmount != null && thresholdAmount < 0) {
            throw new CouponBusinessException(GrpcResponseCode.INVALID_PARAM,
                    "thresholdAmount 不可為負數");
        }

        CouponType couponType = CouponType.fromValue(type);

        switch (couponType) {
            case CASH -> {
                if (discountAmount == null || discountAmount <= 0) {
                    throw new CouponBusinessException(GrpcResponseCode.COUPON_CASH_DISCOUNT_REQUIRED);
                }
            }
            case PERCENT -> {
                if (discountPercent == null || discountPercent < 1 || discountPercent > 99) {
                    throw new CouponBusinessException(GrpcResponseCode.COUPON_PERCENT_OUT_OF_RANGE);
                }
                if (maxDiscountAmount == null || maxDiscountAmount <= 0) {
                    throw new CouponBusinessException(GrpcResponseCode.COUPON_PERCENT_MAX_DISCOUNT_REQUIRED);
                }
            }
        }

        if (startAt == null) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_TEMPLATE_TIME_INVALID, "startAt 不可為空");
        }
        if (endAt == null) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_TEMPLATE_TIME_INVALID, "endAt 不可為空");
        }
        if (startAt.isAfter(endAt)) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_TEMPLATE_TIME_INVALID, "startAt 不得晚於 endAt");
        }
    }
}

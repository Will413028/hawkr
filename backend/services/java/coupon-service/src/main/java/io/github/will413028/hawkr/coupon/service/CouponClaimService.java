package io.github.will413028.hawkr.coupon.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.coupon.entity.CouponClaimEntity;
import io.github.will413028.hawkr.coupon.mapper.CouponClaimMapper;
import io.github.will413028.hawkr.coupon.mapper.CouponTemplateMapper;
import org.springframework.stereotype.Service;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Set;

@Service
public class CouponClaimService {

    private static final ObsLogger log = ObsLogger.forClass(CouponClaimService.class);
    private final CouponClaimMapper claimMapper;
    private final CouponTemplateMapper templateMapper;

    public CouponClaimService(CouponClaimMapper claimMapper, CouponTemplateMapper templateMapper) {
        this.claimMapper = claimMapper;
        this.templateMapper = templateMapper;
    }

    /**
     * 領取優惠券（冪等）
     * <p>
     * 若為新領取，遞增模板 issued_count 並回傳 claimed_at。
     * 若已領取過（affected=0），回傳既有 claimed_at。
     */
    public LocalDateTime claimCoupon(Long templateId, Long buyerId) {
        int affected = claimMapper.insertIgnore(templateId, buyerId);
        if (affected > 0) {
            templateMapper.incrementIssuedCount(templateId, 1);
            log.info("CouponClaimService.claimCoupon.success",
                    "新增領取 templateId=" + templateId + " buyerId=" + buyerId);
        }

        // 不論新增或已存在，都查詢回傳 claimed_at
        LambdaQueryWrapper<CouponClaimEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponClaimEntity::getTemplateId, templateId)
                .eq(CouponClaimEntity::getBuyerId, buyerId);
        CouponClaimEntity claim = claimMapper.selectOne(wrapper);
        return claim != null ? claim.getClaimedAt() : DateTimeUtils.nowUtc();
    }

    /**
     * 檢查買家是否已領取某模板
     */
    public boolean isClaimed(Long templateId, Long buyerId) {
        LambdaQueryWrapper<CouponClaimEntity> wrapper = new LambdaQueryWrapper<>();
        wrapper.eq(CouponClaimEntity::getTemplateId, templateId)
                .eq(CouponClaimEntity::getBuyerId, buyerId);
        return claimMapper.selectCount(wrapper) > 0;
    }

    /**
     * 取得買家已領取的所有 template_id（供列表 API 批次檢查）
     */
    public Set<Long> getClaimedTemplateIds(Long buyerId) {
        return claimMapper.selectTemplateIdsByBuyerId(buyerId);
    }

    /**
     * 批量領取（為多個買家新增領取紀錄）
     *
     * @return 實際新增的領取數量
     */
    public int batchClaimForBuyers(Long templateId, List<Long> buyerIds) {
        if (buyerIds == null || buyerIds.isEmpty()) {
            return 0;
        }
        int newClaims = claimMapper.batchInsertIgnore(templateId, buyerIds);
        if (newClaims > 0) {
            templateMapper.incrementIssuedCount(templateId, newClaims);
            log.info("CouponClaimService.batchClaimForBuyers.success",
                    "批量領取 templateId=" + templateId + " newClaims=" + newClaims);
        }
        return newClaims;
    }
}

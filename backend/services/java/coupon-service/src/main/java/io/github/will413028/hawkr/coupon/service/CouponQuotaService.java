package io.github.will413028.hawkr.coupon.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.coupon.entity.SellerCouponQuotaEntity;
import io.github.will413028.hawkr.coupon.exception.CouponBusinessException;
import io.github.will413028.hawkr.coupon.mapper.SellerCouponQuotaMapper;
import org.springframework.stereotype.Service;

@Service
public class CouponQuotaService {

    private static final ObsLogger log = ObsLogger.forClass(CouponQuotaService.class);
    private static final int DEFAULT_MAX_TEMPLATE_COUNT = 5;

    private final SellerCouponQuotaMapper quotaMapper;
    private final CouponTemplateService templateService;

    public CouponQuotaService(SellerCouponQuotaMapper quotaMapper, CouponTemplateService templateService) {
        this.quotaMapper = quotaMapper;
        this.templateService = templateService;
    }

    /**
     * 取得賣家最大模板數（查無記錄時使用預設值）
     */
    public int getMaxTemplateCount(Long storeId) {
        SellerCouponQuotaEntity quota = quotaMapper.selectById(storeId);
        return quota != null ? quota.getMaxTemplateCount() : DEFAULT_MAX_TEMPLATE_COUNT;
    }

    /**
     * 檢查賣家配額
     */
    public void checkQuota(Long storeId) {
        int max = getMaxTemplateCount(storeId);
        long current = templateService.countSellerActiveTemplates(storeId);
        if (current >= max) {
            throw new CouponBusinessException(GrpcResponseCode.COUPON_QUOTA_EXCEEDED,
                    "已達配額上限 " + max + " 種");
        }
    }

    /**
     * Admin 更新賣家配額
     */
    public void updateQuota(Long storeId, int maxTemplateCount) {
        SellerCouponQuotaEntity quota = quotaMapper.selectById(storeId);
        if (quota == null) {
            quota = new SellerCouponQuotaEntity();
            quota.setStoreId(storeId);
            quota.setMaxTemplateCount(maxTemplateCount);
            quotaMapper.insert(quota);
        } else {
            quota.setMaxTemplateCount(maxTemplateCount);
            quotaMapper.updateById(quota);
        }
        log.info("CouponQuotaService.updateQuota.success", "更新賣家配額",
                "storeId", String.valueOf(storeId));
    }
}

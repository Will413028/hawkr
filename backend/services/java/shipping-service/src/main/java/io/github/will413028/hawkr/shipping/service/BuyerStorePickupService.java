package io.github.will413028.hawkr.shipping.service;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.shipping.entity.BuyerStorePickupEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.mapper.BuyerStorePickupMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

@Service
public class BuyerStorePickupService {

    private static final ObsLogger log = ObsLogger.forClass(BuyerStorePickupService.class);

    private final BuyerStorePickupMapper pickupMapper;

    @Value("${hawkr.shipping.store-pickup.max-per-buyer:20}")
    private int maxPerBuyer;

    public BuyerStorePickupService(BuyerStorePickupMapper pickupMapper) {
        this.pickupMapper = pickupMapper;
    }

    /**
     * 新增買家常用門市
     */
    @Transactional
    public BuyerStorePickupEntity addStorePickup(long buyerId,
                                                  String storeCode,
                                                  String storeName,
                                                  String storeAddress,
                                                  String receiverName,
                                                  String receiverPhone,
                                                  boolean isDefault) {
        // 檢查數量上限
        long count = pickupMapper.selectCount(
                new LambdaQueryWrapper<BuyerStorePickupEntity>()
                        .eq(BuyerStorePickupEntity::getBuyerId, buyerId));
        if (count >= maxPerBuyer) {
            throw new ShippingBusinessException(GrpcResponseCode.BUYER_STORE_PICKUP_LIMIT_EXCEEDED, maxPerBuyer);
        }

        // 若設為預設，先清除舊的預設
        if (isDefault) {
            clearDefault(buyerId);
        }

        BuyerStorePickupEntity entity = new BuyerStorePickupEntity();
        entity.setId(TsidUtil.generate());
        entity.setBuyerId(buyerId);
        entity.setStoreCode(storeCode);
        entity.setStoreName(storeName);
        entity.setStoreAddress(storeAddress);
        entity.setReceiverName(receiverName);
        entity.setReceiverPhone(receiverPhone);
        entity.setIsDefault(isDefault);
        pickupMapper.insert(entity);

        log.info("BuyerStorePickup.add.success",
                "新增常用門市，storeCode=" + storeCode,
                "buyerId", String.valueOf(buyerId));
        return entity;
    }

    /**
     * 更新買家常用門市
     */
    @Transactional
    public BuyerStorePickupEntity updateStorePickup(long buyerId,
                                                     long pickupId,
                                                     String storeCode,
                                                     String storeName,
                                                     String storeAddress,
                                                     String receiverName,
                                                     String receiverPhone,
                                                     boolean isDefault) {
        BuyerStorePickupEntity entity = pickupMapper.selectById(pickupId);
        if (entity == null || !entity.getBuyerId().equals(buyerId)) {
            throw new ShippingBusinessException(GrpcResponseCode.BUYER_STORE_PICKUP_NOT_FOUND);
        }

        // 若設為預設，先清除舊的預設
        if (isDefault) {
            clearDefault(buyerId);
        }

        entity.setStoreCode(storeCode);
        entity.setStoreName(storeName);
        entity.setStoreAddress(storeAddress);
        entity.setReceiverName(receiverName);
        entity.setReceiverPhone(receiverPhone);
        entity.setIsDefault(isDefault);
        pickupMapper.updateById(entity);

        log.info("BuyerStorePickup.update.success",
                "更新常用門市，pickupId=" + pickupId,
                "buyerId", String.valueOf(buyerId));
        return entity;
    }

    /**
     * 列出買家所有常用門市
     */
    public List<BuyerStorePickupEntity> listStorePickups(long buyerId) {
        return pickupMapper.selectList(
                new LambdaQueryWrapper<BuyerStorePickupEntity>()
                        .eq(BuyerStorePickupEntity::getBuyerId, buyerId)
                        .orderByDesc(BuyerStorePickupEntity::getIsDefault)
                        .orderByDesc(BuyerStorePickupEntity::getCreatedAt));
    }

    /**
     * 刪除買家常用門市
     */
    @Transactional
    public void deleteStorePickup(long buyerId, long pickupId) {
        BuyerStorePickupEntity entity = pickupMapper.selectById(pickupId);
        if (entity == null || !entity.getBuyerId().equals(buyerId)) {
            throw new ShippingBusinessException(GrpcResponseCode.BUYER_STORE_PICKUP_NOT_FOUND);
        }
        pickupMapper.deleteById(pickupId);

        log.info("BuyerStorePickup.delete.success",
                "刪除常用門市，pickupId=" + pickupId,
                "buyerId", String.valueOf(buyerId));
    }

    /**
     * 清除該買家所有預設標記
     */
    private void clearDefault(long buyerId) {
        List<BuyerStorePickupEntity> defaults = pickupMapper.selectList(
                new LambdaQueryWrapper<BuyerStorePickupEntity>()
                        .eq(BuyerStorePickupEntity::getBuyerId, buyerId)
                        .eq(BuyerStorePickupEntity::getIsDefault, true));
        for (BuyerStorePickupEntity d : defaults) {
            d.setIsDefault(false);
            pickupMapper.updateById(d);
        }
    }
}

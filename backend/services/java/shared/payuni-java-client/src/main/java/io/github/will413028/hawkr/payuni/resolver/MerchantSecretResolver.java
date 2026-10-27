package io.github.will413028.hawkr.payuni.resolver;

import io.github.will413028.hawkr.payuni.model.MerchantCredential;

/**
 * 商家金鑰解析介面
 * <p>
 * 由消費端實作，負責根據 merId 取得對應的 hashKey/hashIv。
 * 代理商模式下，所有子商家共用代理商金鑰。
 */
public interface MerchantSecretResolver {

    /**
     * 根據商家編號解析金鑰憑證
     *
     * @param merId 商家編號
     * @return 商家金鑰憑證
     * @throws IllegalArgumentException 商家編號不存在或無效
     */
    MerchantCredential resolve(String merId);
}

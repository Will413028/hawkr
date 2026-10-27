package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

/**
 * 賣家刪除後批次下架其名下商品（消費 {@code seller.deleted} 事件）。
 *
 * <p>抽為獨立 {@link Service} 而非寫在
 * {@link io.github.will413028.hawkr.product.mq.SellerDeletedConsumer} 內，以避免 Spring
 * {@code @Transactional} self-invocation 失效（CGLIB proxy 不代理同 class 內呼叫），
 * 與 {@link FavoriteCountService} 同理。</p>
 *
 * <h3>為何在這裡下架而非列表查詢過濾</h3>
 * 賣家刪除時 seller-service 只標記 {@code sellers.status='deleted'}，但商品 {@code status}
 * 仍停在 ACTIVE（4004），導致所有公開面向（列表 / 商品詳情 / 相關商品 / 產地彙總）都會洩漏
 * 已刪除賣家的商品。把 ACTIVE 商品改為 INACTIVE（4003，下架）即可一次解決所有面向。
 *
 * <h3>冪等</h3>
 * 只挑「目前 ACTIVE」的商品處理；事件重送時第二次查到 0 筆 → 0 副作用，天然冪等，
 * 無需 dedup 表。
 */
@Service
public class SellerProductOffShelfService {

    private static final ObsLogger log = ObsLogger.forClass(SellerProductOffShelfService.class);

    private final ProductRepository productRepository;
    private final ProductCacheService cacheService;
    private final ProductRevalidationService productRevalidationService;

    public SellerProductOffShelfService(ProductRepository productRepository,
                                        ProductCacheService cacheService,
                                        ProductRevalidationService productRevalidationService) {
        this.productRepository = productRepository;
        this.cacheService = cacheService;
        this.productRevalidationService = productRevalidationService;
    }

    /**
     * 把該賣家（{@code store_id = storeId}，store 代理鍵；② re-key，D7 Stage 2 更名）
     * 所有 ACTIVE 商品下架為 INACTIVE，連帶維護 hashtag 計數與快取失效。
     *
     * @param storeId 被刪除賣家的 store 代理鍵（事件 payload snapshot；舊格式 fallback 值 == account_id）
     * @return 實際下架的商品數（0 = 沒有 ACTIVE 商品 / 已處理過）
     */
    @Transactional
    public int offShelfAllByMerchant(Long storeId) {
        String active = ProductStatus.ACTIVE.value();
        String inactive = ProductStatus.INACTIVE.value();

        // 0. 無條件蓋章：該賣家全部狀態商品標記 seller_deleted_at（賣家復活後對其隱形；Part A / doc 44 §11）。
        //    必須在下方 ACTIVE 早退之前——只剩 INACTIVE/DRAFT（無 ACTIVE）的賣家也要蓋。
        //    與下架同一 @Transactional；seller_deleted_at IS NULL guard → 冪等。
        productRepository.stampSellerDeletedByMerchant(storeId);

        List<Long> ids = productRepository.findProductIdsBySellerAndStatus(storeId, active);
        if (ids.isEmpty()) {
            // 無 ACTIVE 可下架，但蓋章改動了 seller_deleted_at → 賣家私有列表查詢（seam 1 exclude）結果變了，
            // 仍需 bust 列表快取，否則 stale 到 TTL（與下方 ACTIVE 分支一致）。
            cacheService.evictProductListForWrite(storeId);
            log.info("SellerProductOffShelf.noop",
                    "無 ACTIVE 商品可下架 storeId=" + storeId);
            return 0;
        }

        // 1. 批次 ACTIVE → INACTIVE
        productRepository.batchUpdateStatusByIds(ids, inactive, storeId);

        // 2. 商品詳情快取失效
        for (Long id : ids) {
            cacheService.evictProductDetail(String.valueOf(id));
        }

        // 3. 列表快取失效：賣家私有列表 + 公開列表（account_id=0）
        cacheService.evictProductListForWrite(storeId);

        // 4. frontend cache bust（§9.3）：賣家刪除後其商品應從 storefront / PDP 消失，
        //    必須 bust seller:<storeId>（列表 + 賣家 profile，賣家刪除路徑本身無 revalidate producer）
        //    + product:<id>×N（詳情），否則已刪賣家商品 stale 到 TTL（#1271）。
        productRevalidationService.revalidateProductsWithSellers(ids);

        log.info("SellerProductOffShelf.done",
                "賣家刪除批次下架 storeId=" + storeId + " count=" + ids.size());
        return ids.size();
    }
}

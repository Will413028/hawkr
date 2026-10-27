package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.StoreExposureRepository;
import org.springframework.stereotype.Service;

import java.util.List;

/**
 * 店家可曝光讀取模型的寫入（消費 {@code seller.exposure.changed}）。
 *
 * <p>公開讀取面在查詢時以 {@code store_exposure} 過濾，這裡只負責依版本寫入，並在有改動時清掉該店商品的
 * 詳情快取與列表快取（兩者都帶著舊的可曝光狀態）。寫入是單一 upsert 語句，版本小於現有的事件不改任何東西，
 * 所以事件亂序都安全；版本等於現有的重送不改列，但仍再清一次快取：快取清除不在 transaction 內，寫入已提交才清，
 * 清除失敗（商品查詢、Redis）讓訊息重送時，重送必須補上沒做完的清除，不能因為版本已存在就跳過。
 */
@Service
public class StoreExposureService {

    private static final ObsLogger log = ObsLogger.forClass(StoreExposureService.class);

    private final StoreExposureRepository storeExposureRepository;
    private final ProductRepository productRepository;
    private final ProductCacheService cacheService;
    private final ProductRevalidationService productRevalidationService;

    public StoreExposureService(StoreExposureRepository storeExposureRepository,
                                ProductRepository productRepository,
                                ProductCacheService cacheService,
                                ProductRevalidationService productRevalidationService) {
        this.storeExposureRepository = storeExposureRepository;
        this.productRepository = productRepository;
        this.cacheService = cacheService;
        this.productRevalidationService = productRevalidationService;
    }

    /**
     * @return true = 事件版本不小於現有版本（新寫入或同版本重送），已清除快取；false = 事件版本小於現有版本而丟棄
     */
    public boolean apply(long storeId, boolean exposable, long version) {
        int changed = storeExposureRepository.upsertIfNewer(storeId, exposable, version);
        if (changed == 0) {
            Long stored = storeExposureRepository.findVersion(storeId);
            if (stored == null || stored != version) {
                log.info("StoreExposure.stale",
                        "舊版本事件，未改動 storeId=" + storeId + " version=" + version + " stored=" + stored);
                return false;
            }
            log.info("StoreExposure.redelivered",
                    "同版本重送，重新清除快取 storeId=" + storeId + " version=" + version);
        }

        List<Long> productIds = productRepository.findProductIdsByStoreId(storeId);
        for (Long id : productIds) {
            cacheService.evictProductDetail(String.valueOf(id));
        }
        cacheService.evictProductListForWrite(storeId);
        if (!productIds.isEmpty()) {
            productRevalidationService.revalidateProductsWithSellers(productIds);
        }

        log.info("StoreExposure.applied",
                "店家可曝光狀態更新 storeId=" + storeId + " exposable=" + exposable
                        + " version=" + version + " products=" + productIds.size());
        return true;
    }
}

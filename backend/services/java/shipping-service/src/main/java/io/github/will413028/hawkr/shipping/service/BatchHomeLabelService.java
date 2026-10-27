package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.shipping.config.ShippingBatchProperties;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Semaphore;
import java.util.concurrent.ThreadFactory;

@Service
public class BatchHomeLabelService {

    private static final ObsLogger log = ObsLogger.forClass(BatchHomeLabelService.class);

    private final ShippingService shippingService;
    private final OnboardingService onboardingService;
    private final ShippingBatchProperties properties;

    public BatchHomeLabelService(ShippingService shippingService,
                                 OnboardingService onboardingService,
                                 ShippingBatchProperties properties) {
        this.shippingService = shippingService;
        this.onboardingService = onboardingService;
        this.properties = properties;
    }

    public BatchResult batchGenerate(long storeId, List<Item> items) {
        // 寫路徑：賣家當下意願防線（§2.3.1）— 整批共用同一個 seller，預先檢一次
        onboardingService.requireActiveSetting(storeId, ShippingMethod.TAQBIN);

        Semaphore semaphore = new Semaphore(properties.getHomeLabelConcurrency());
        ThreadFactory vtFactory = Thread.ofVirtual()
                .name("home-label-batch-", 0)
                .factory();

        try (ExecutorService executor = Executors.newThreadPerTaskExecutor(vtFactory)) {
            List<CompletableFuture<ItemResult>> futures = new ArrayList<>(items.size());
            for (Item item : items) {
                CompletableFuture<ItemResult> f = CompletableFuture.supplyAsync(() -> {
                    try {
                        semaphore.acquire();
                    } catch (InterruptedException ie) {
                        Thread.currentThread().interrupt();
                        return new ItemResult(item.orderNo(), "", "", false, "interrupted");
                    }
                    try {
                        return processOne(storeId, item);
                    } finally {
                        semaphore.release();
                    }
                }, executor);
                futures.add(f);
            }

            List<ItemResult> results = new ArrayList<>(futures.size());
            int successCount = 0;
            for (CompletableFuture<ItemResult> f : futures) {
                ItemResult r = f.join();
                results.add(r);
                if (r.success()) successCount++;
            }

            log.info("BatchHomeLabelService.batchGenerate.done",
                    "批次宅配取號完成 total=" + items.size() + " success=" + successCount);
            return new BatchResult(items.size(), successCount, results);
        }
    }

    private ItemResult processOne(long storeId, Item item) {
        try {
            String labelKey = shippingService.generateHomeLabelPdf(
                    storeId, item.orderNo(), item.shipDate(), item.deliveryDate(), List.of());
            ShipmentEntity entity = shippingService.getShipmentByOrderNo(item.orderNo());
            String shipmentNo = entity != null ? entity.getShipmentNo() : "";
            return new ItemResult(item.orderNo(), shipmentNo, labelKey, true, "");
        } catch (ShippingBusinessException sbe) {
            return new ItemResult(item.orderNo(), "", "", false, sbe.getMessage());
        } catch (RuntimeException re) {
            log.error("BatchHomeLabelService.processOne.unexpected",
                    "批次宅配取號單筆失敗: " + re.getMessage(), "orderNo", item.orderNo());
            return new ItemResult(item.orderNo(), "", "", false, re.getMessage());
        }
    }

    public record Item(String orderNo, String shipDate, String deliveryDate) {}

    public record ItemResult(String orderNo, String shipmentNo, String labelKey, boolean success, String message) {}

    public record BatchResult(int totalCount, int successCount, List<ItemResult> results) {}
}

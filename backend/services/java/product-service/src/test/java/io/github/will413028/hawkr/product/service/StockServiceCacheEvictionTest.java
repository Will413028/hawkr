package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.code.StockTxPhase;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.entity.projection.SkuProductRef;
import io.github.will413028.hawkr.product.mq.ProductEventPublisher;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.ProductSkuRepository;
import io.github.will413028.hawkr.product.repository.StockTransactionRepository;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.util.List;

import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 庫存快取失效收斂為 afterCommit 單一入口的行為測試（#1455）。
 *
 * <p>核心不變式：
 * <ol>
 *   <li>有 active transaction 時，evict 不在 {@code @Transactional} 內執行（race-free），
 *       僅於交易 commit 後（afterCommit）才觸發一次 detail + list 失效。</li>
 *   <li>deduct 與 restore 兩條寫路徑共用同一收斂入口（cascadeProductStock），
 *       OrderCancelledConsumer 經 restoreStock 亦自動取得 post-commit evict。</li>
 *   <li>無 active transaction（防禦路徑）→ fallback 直接 evict。</li>
 * </ol>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class StockServiceCacheEvictionTest {

    @Mock ProductSkuRepository skuRepository;
    @Mock StockTransactionRepository stockTxRepository;
    @Mock ProductRepository productRepository;
    @Mock ProductCacheService cacheService;
    @Mock ProductEventPublisher productEventPublisher;
    @Mock ProductRevalidationService productRevalidationService;
    @Mock io.github.will413028.hawkr.product.repository.StoreExposureRepository storeExposureRepository;

    StockService stockService;

    private static final long SKU_ID = 5001L;
    private static final long PRODUCT_ID = 9001L;
    private static final long OWNER_ID = 2683382637L;
    private static final int QTY = 2;

    @BeforeEach
    void setUp() {
        stockService = new StockService(skuRepository, stockTxRepository, productRepository,
                cacheService, productEventPublisher, productRevalidationService, storeExposureRepository);

        // cascadeProductStock 依賴：SKU → product 反查、product → owner 反查
        when(skuRepository.findSkuProductRefs(List.of(SKU_ID)))
                .thenReturn(List.of(new SkuProductRef(SKU_ID, PRODUCT_ID, 3)));
        when(productRepository.findProductSellerRefs(List.of(PRODUCT_ID)))
                .thenReturn(List.of(new ProductSellerRef(PRODUCT_ID, OWNER_ID)));
    }

    @AfterEach
    void tearDown() {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.clearSynchronization();
        }
    }

    /** 手動觸發已註冊的 afterCommit callback（模擬交易 commit）。 */
    private void fireAfterCommit() {
        for (TransactionSynchronization sync : TransactionSynchronizationManager.getSynchronizations()) {
            sync.afterCommit();
        }
    }

    @Test
    @DisplayName("deduct：交易內不 evict，afterCommit 後才清 detail + list（race-free）")
    void deductEvictsOnlyAfterCommit() {
        when(stockTxRepository.existsByStockTxIdAndOperation("TX-D1", StockTxPhase.DEDUCT.value()))
                .thenReturn(false);
        when(skuRepository.deductStock(eq(SKU_ID), eq(QTY))).thenReturn(1);

        // 模擬 Spring 交易的 synchronization 上下文
        TransactionSynchronizationManager.initSynchronization();

        StockService.StockResult result = stockService.deductStock(
                "TX-D1", List.of(new StockService.StockItemParam(SKU_ID, QTY)));

        // 交易「尚未 commit」→ 絕不可 evict（消除 evict-inside-txn race）
        verify(cacheService, never()).evictProductDetail(anyString());
        verify(cacheService, never()).evictProductListForWriteBatch(org.mockito.ArgumentMatchers.any());

        // commit 後才一次性 evict detail + list
        fireAfterCommit();
        verify(cacheService).evictProductDetail(String.valueOf(PRODUCT_ID));
        verify(cacheService).evictProductListForWriteBatch(List.of(OWNER_ID));

        org.junit.jupiter.api.Assertions.assertEquals(StockService.StockResult.SUCCESS, result);
    }

    @Test
    @DisplayName("restore：與 deduct 共用同一 afterCommit 收斂入口（OrderCancelledConsumer 路徑）")
    void restoreEvictsOnlyAfterCommit() {
        when(stockTxRepository.existsByStockTxIdAndOperation("TX-R1", StockTxPhase.RESTORE.value()))
                .thenReturn(false);
        // over-restore 因果閘門：需存在同 txId 已 commit 的 DEDUCT
        when(stockTxRepository.existsByStockTxIdAndOperation("TX-R1", StockTxPhase.DEDUCT.value()))
                .thenReturn(true);
        when(skuRepository.restoreStock(eq(SKU_ID), eq(QTY))).thenReturn(1);
        // restore（sign>0）會查 restock transition，回空即不觸發 restock event
        when(productRepository.findRestockTransitionFields(List.of(PRODUCT_ID)))
                .thenReturn(List.of());

        TransactionSynchronizationManager.initSynchronization();

        stockService.restoreStock("TX-R1", List.of(new StockService.StockItemParam(SKU_ID, QTY)));

        verify(cacheService, never()).evictProductDetail(anyString());

        fireAfterCommit();
        verify(cacheService).evictProductDetail(String.valueOf(PRODUCT_ID));
        verify(cacheService).evictProductListForWriteBatch(List.of(OWNER_ID));
    }

    @Test
    @DisplayName("無 active transaction（防禦路徑）：fallback 直接 evict")
    void fallbackEvictsImmediatelyWithoutTransaction() {
        when(stockTxRepository.existsByStockTxIdAndOperation("TX-D2", StockTxPhase.DEDUCT.value()))
                .thenReturn(false);
        when(skuRepository.deductStock(eq(SKU_ID), eq(QTY))).thenReturn(1);

        // 不 initSynchronization → isSynchronizationActive() == false
        stockService.deductStock("TX-D2", List.of(new StockService.StockItemParam(SKU_ID, QTY)));

        // 無交易上下文 → 立即 evict（不等 afterCommit）
        verify(cacheService).evictProductDetail(String.valueOf(PRODUCT_ID));
        verify(cacheService).evictProductListForWriteBatch(List.of(OWNER_ID));
    }

    private void stubRestoreWithRestockTransition() {
        when(stockTxRepository.existsByStockTxIdAndOperation("TX-R2", StockTxPhase.RESTORE.value())).thenReturn(false);
        when(stockTxRepository.existsByStockTxIdAndOperation("TX-R2", StockTxPhase.DEDUCT.value())).thenReturn(true);
        when(skuRepository.restoreStock(eq(SKU_ID), eq(QTY))).thenReturn(1);
        when(productRepository.findRestockTransitionFields(List.of(PRODUCT_ID))).thenReturn(List.of(
                new io.github.will413028.hawkr.product.entity.projection.RestockTransitionProjection(
                        PRODUCT_ID, 0L, "4004", "商品", null, OWNER_ID, null)));
        when(productRepository.findStockQtyById(PRODUCT_ID)).thenReturn((long) QTY);
    }

    @Test
    @DisplayName("補貨通知：店家可曝光時才發")
    void restockNotificationPublishedWhenStoreExposable() {
        stubRestoreWithRestockTransition();
        when(storeExposureRepository.isExposable(OWNER_ID)).thenReturn(true);
        TransactionSynchronizationManager.initSynchronization();

        stockService.restoreStock("TX-R2", List.of(new StockService.StockItemParam(SKU_ID, QTY)));

        verify(productEventPublisher).publishRestockAfterCommit(org.mockito.ArgumentMatchers.any());
    }

    @Test
    @DisplayName("補貨通知：店家不可曝光（含沒有列）不發")
    void restockNotificationSuppressedWhenStoreNotExposable() {
        stubRestoreWithRestockTransition();
        when(storeExposureRepository.isExposable(OWNER_ID)).thenReturn(false);
        TransactionSynchronizationManager.initSynchronization();

        stockService.restoreStock("TX-R2", List.of(new StockService.StockItemParam(SKU_ID, QTY)));

        verify(productEventPublisher, never()).publishRestockAfterCommit(org.mockito.ArgumentMatchers.any());
    }
}

package io.github.will413028.hawkr.productimport.service;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.product.grpc.DraftViolation;
import io.github.will413028.hawkr.product.grpc.ValidateLeafCategoryInternalResponse;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemSkuEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.grpc.client.MediaGrpcClient;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

class ItemMutationServiceTest {

    private final ProductImportJobMapper jobMapper = mock(ProductImportJobMapper.class);
    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ProductImportItemSkuMapper skuMapper = mock(ProductImportItemSkuMapper.class);
    private final ProductImportItemAssetMapper assetMapper = mock(ProductImportItemAssetMapper.class);
    private final ProductImportItemErrorMapper errorMapper = mock(ProductImportItemErrorMapper.class);
    private final ProductGrpcClient productClient = mock(ProductGrpcClient.class);
    private final MediaGrpcClient mediaClient = mock(MediaGrpcClient.class);
    private final ObsRabbitPublisher publisher = mock(ObsRabbitPublisher.class);
    private final TestTxManager txManager = new TestTxManager();

    private final ItemMutationService service = new ItemMutationService(
            jobMapper, itemMapper, skuMapper, assetMapper, errorMapper,
            productClient, mediaClient, publisher, txManager);

    @Test
    void updateRejectsEmptySkuFullListBeforeDeletingExistingRows() {
        ItemMutationService.UpdateInput input = new ItemMutationService.UpdateInput();
        input.skusSet = true;
        input.skus = List.of();

        assertThatThrownBy(() -> service.update(20L, 10L, 100L, 1, input))
                .isInstanceOf(StatusRuntimeException.class)
                .extracting(e -> ((StatusRuntimeException) e).getStatus().getCode())
                .isEqualTo(Status.Code.INVALID_ARGUMENT);

        verifyNoInteractions(itemMapper, skuMapper, productClient);
    }

    @Test
    void updateRerunsDraftValidationEvenWhenPayloadIsNoop() {
        ProductImportItemEntity snapshot = new ProductImportItemEntity();
        snapshot.setId(100L);
        snapshot.setImportJobId(10L);
        snapshot.setStoreId(20L);
        snapshot.setStatus(ImportItemStatus.READY.value());
        snapshot.setCommitRevision(3);
        snapshot.setVersion(1);
        when(itemMapper.selectById(100L)).thenReturn(snapshot);
        when(skuMapper.selectList(any())).thenReturn(List.of());
        when(assetMapper.selectList(any())).thenReturn(List.of());
        when(productClient.validateProductDraft(
                eq(20L), any(), eq(ValidateProductDraftRequest.ValidationContext.IMPORT_UPDATE)))
                .thenReturn(ValidateProductDraftResponse.newBuilder()
                        .addViolations(DraftViolation.newBuilder()
                                .setField("skus")
                                .setCode("SKUS_REQUIRED")
                                .setSeverity("ERROR")
                                .setMessage("至少需要 1 個 SKU")
                                .build())
                        .setHasBlockingError(true)
                        .build());

        service.update(20L, 10L, 100L, 1, new ItemMutationService.UpdateInput());

        verify(productClient).validateProductDraft(
                eq(20L), any(), eq(ValidateProductDraftRequest.ValidationContext.IMPORT_UPDATE));
        verify(errorMapper).delete(any());
        verify(errorMapper).insert(any());
        verify(itemMapper).recalcValidationCounts(100L);
    }

    @Test
    void batchUpdateCategoryClearsStaleCategoryErrorAndRerunsDraftValidation() {
        ProductImportJobEntity job = new ProductImportJobEntity();
        job.setId(10L);
        job.setStoreId(20L);
        when(jobMapper.selectById(10L)).thenReturn(job);

        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setId(100L);
        item.setImportJobId(10L);
        item.setStoreId(20L);
        item.setStatus(ImportItemStatus.READY.value());
        item.setCommitRevision(1);
        when(productClient.validateLeafCategory("300"))
                .thenReturn(ValidateLeafCategoryInternalResponse.newBuilder()
                        .setIsValid(true)
                        .setCatL1Id("100")
                        .setCatL1Name("水果")
                        .setCatL2Id("200")
                        .setCatL2Name("柑橘")
                        .setCatL3Id("300")
                        .setCatL3Name("橘子")
                        .build());
        when(itemMapper.selectList(any())).thenReturn(List.of(item));
        when(itemMapper.selectById(100L)).thenReturn(item);
        when(skuMapper.selectList(any())).thenReturn(List.of());
        when(assetMapper.selectList(any())).thenReturn(List.of());
        when(productClient.validateProductDraft(
                eq(20L), any(), eq(ValidateProductDraftRequest.ValidationContext.IMPORT_UPDATE)))
                .thenReturn(ValidateProductDraftResponse.newBuilder().build());

        service.batchUpdateCategory(20L, 10L, List.of(100L), false, "300");

        verify(itemMapper).updateById(item);
        verify(errorMapper, atLeastOnce()).delete(any());
        verify(productClient).validateProductDraft(
                eq(20L), any(), eq(ValidateProductDraftRequest.ValidationContext.IMPORT_UPDATE));
        verify(itemMapper).recalcValidationCounts(100L);
    }

    @Test
    void batchUpdateShippingClearsStaleShippingErrorsAndRerunsDraftValidation() {
        ProductImportJobEntity job = new ProductImportJobEntity();
        job.setId(10L);
        job.setStoreId(20L);
        when(jobMapper.selectById(10L)).thenReturn(job);

        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setId(100L);
        item.setImportJobId(10L);
        item.setStoreId(20L);
        item.setStatus(ImportItemStatus.READY.value());
        item.setCommitRevision(1);
        ProductImportItemSkuEntity sku = new ProductImportItemSkuEntity();
        sku.setId(200L);
        sku.setImportItemId(100L);

        when(itemMapper.selectList(any())).thenReturn(List.of(item));
        when(itemMapper.selectById(100L)).thenReturn(item);
        when(skuMapper.selectList(any())).thenReturn(List.of(sku), List.of(sku));
        when(assetMapper.selectList(any())).thenReturn(List.of());
        when(productClient.validateProductDraft(
                eq(20L), any(), eq(ValidateProductDraftRequest.ValidationContext.IMPORT_UPDATE)))
                .thenReturn(ValidateProductDraftResponse.newBuilder().build());

        service.batchUpdateShipping(20L, 10L, List.of(100L), false,
                List.of(new ItemMutationService.ShippingMethodInputEntry(
                        "TAQBIN", "AMBIENT", "")));

        verify(skuMapper).updateById(sku);
        verify(errorMapper, atLeastOnce()).delete(any());
        verify(productClient).validateProductDraft(
                eq(20L), any(), eq(ValidateProductDraftRequest.ValidationContext.IMPORT_UPDATE));
        verify(itemMapper).recalcValidationCounts(100L);
    }

    /**
     * 40 §2.5 / §5.5 階段 3:applyTx body 必須被 TransactionTemplate.execute 包覆,
     * mid-flight throw 必須觸發 rollback。本 test 透過 TestTxManager 計數 rollback 呼叫,
     * 驗證 caller chain 真的有走 tx 包覆(否則此計數會 = 0,證明 self-invocation regression)。
     *
     * <p>觸發 case:input.skus 中送入不屬於本 item 的 skuId → reconciliation 後仍 0 SKU
     * (pre-check predictedKeep=0 拋 INVALID_ARGUMENT)。
     * 此 throw 在進入 applyTxBody 後才發生,代表 tx 已開啟,應該觸發 rollback。
     */
    @Test
    void updateWrapsApplyTxInTransactionTemplateAndRollsBackOnThrow() {
        ProductImportItemEntity snapshot = new ProductImportItemEntity();
        snapshot.setId(100L);
        snapshot.setImportJobId(10L);
        snapshot.setStoreId(20L);
        snapshot.setStatus(ImportItemStatus.READY.value());
        snapshot.setVersion(1);
        when(itemMapper.selectById(100L)).thenReturn(snapshot);

        ItemMutationService.UpdateInput input = new ItemMutationService.UpdateInput();
        input.skusSet = true;
        ItemMutationService.SkuInput stranger = new ItemMutationService.SkuInput();
        stranger.skuId = 999L;
        stranger.delete = false;
        input.skus = List.of(stranger);
        // skuMapper.selectById(999) returns null → predictedKeep stays 0 → throw

        assertThatThrownBy(() -> service.update(20L, 10L, 100L, 1, input))
                .isInstanceOf(StatusRuntimeException.class)
                .extracting(e -> ((StatusRuntimeException) e).getStatus().getCode())
                .isEqualTo(Status.Code.INVALID_ARGUMENT);

        // 證明 tx 確實開了(進入 applyTxBody)→ rollback 觸發
        // 若 self-invocation regression 發生(直接 call applyTxBody 不過 tx),getBeginCount() = 0
        org.assertj.core.api.Assertions.assertThat(txManager.getBeginCount())
                .as("tx must be opened via TransactionTemplate")
                .isGreaterThanOrEqualTo(1);
        org.assertj.core.api.Assertions.assertThat(txManager.getRollbackCount())
                .as("mid-flight throw must trigger rollback")
                .isGreaterThanOrEqualTo(1);
        org.assertj.core.api.Assertions.assertThat(txManager.getCommitCount())
                .as("throw 不應 commit")
                .isEqualTo(0);
    }

    /**
     * In-memory PlatformTransactionManager for unit test:不真的開 DB 連線,
     * 只計數 begin/commit/rollback,讓 caller code path 走完 TransactionTemplate.execute。
     */
    private static class TestTxManager implements PlatformTransactionManager {
        private int beginCount = 0;
        private int commitCount = 0;
        private int rollbackCount = 0;

        int getBeginCount() { return beginCount; }
        int getCommitCount() { return commitCount; }
        int getRollbackCount() { return rollbackCount; }

        @Override
        public TransactionStatus getTransaction(
                org.springframework.transaction.TransactionDefinition definition) {
            beginCount++;
            return new SimpleTransactionStatus(true);
        }

        @Override
        public void commit(TransactionStatus status) {
            commitCount++;
        }

        @Override
        public void rollback(TransactionStatus status) {
            rollbackCount++;
        }
    }
}

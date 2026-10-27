package io.github.will413028.hawkr.productimport.service;

import com.baomidou.mybatisplus.core.conditions.Wrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.product.grpc.CreateProductFromImportInternalResponse;
import io.github.will413028.hawkr.product.grpc.DraftViolation;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftResponse;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.productimport.mapper.ProductImportCommitAttemptMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemErrorMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemSkuMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.CommitItemEvent;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.isNull;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class CommitItemServiceTest {

    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ProductImportItemSkuMapper skuMapper = mock(ProductImportItemSkuMapper.class);
    private final ProductImportItemAssetMapper assetMapper = mock(ProductImportItemAssetMapper.class);
    private final ProductImportItemErrorMapper errorMapper = mock(ProductImportItemErrorMapper.class);
    private final ProductImportCommitAttemptMapper attemptMapper = mock(ProductImportCommitAttemptMapper.class);
    private final ProductImportJobMapper jobMapper = mock(ProductImportJobMapper.class);
    private final ProductGrpcClient productClient = mock(ProductGrpcClient.class);
    private final ObsRabbitPublisher publisher = mock(ObsRabbitPublisher.class);
    private final TestTxManager txManager = new TestTxManager();
    private final CommitItemService service = new CommitItemService(
            itemMapper,
            skuMapper,
            assetMapper,
            errorMapper,
            attemptMapper,
            jobMapper,
            productClient,
            publisher,
            txManager
    );

    private final ProductImportJobEntity job = new ProductImportJobEntity();

    @BeforeEach
    void setUp() {
        job.setId(10L);
        job.setStoreId(20L);
        job.setStatus(ImportJobStatus.COMMITTING.value());
        when(jobMapper.selectById(10L)).thenReturn(job);
        when(jobMapper.update(isNull(), any(UpdateWrapper.class))).thenReturn(1);
    }

    // ====================================================================
    // maybeFinalizeJobBody 業務邏輯測試(透過 reflection 直接呼叫 private body method)
    // ====================================================================

    @Test
    void maybeFinalizeJobKeepsCommittingWhileAnyItemIsCommitting() {
        when(itemMapper.selectCount(any(Wrapper.class))).thenReturn(1L);

        ReflectionTestUtils.invokeMethod(service, "maybeFinalizeJobBody", 10L, 20L);

        verify(jobMapper, never()).updateById(any());
        verify(publisher, never()).publish(any(), any(), any(byte[].class), any());
    }

    @Test
    void maybeFinalizeJobReturnsReadyAfterPartialCommitWhenReadyItemsRemain() {
        when(itemMapper.selectCount(any(Wrapper.class)))
                .thenReturn(
                        0L, // COMMITTING
                        1L, // COMMITTED
                        2L, // READY
                        0L  // COMMIT_FAILED
                );

        ReflectionTestUtils.invokeMethod(service, "maybeFinalizeJobBody", 10L, 20L);

        ArgumentCaptor<ProductImportJobEntity> captor =
                ArgumentCaptor.forClass(ProductImportJobEntity.class);
        verify(jobMapper).updateById(captor.capture());
        assertThat(captor.getValue().getStatus()).isEqualTo(ImportJobStatus.READY.value());
        assertThat(captor.getValue().getCommittedAt()).isNull();
    }

    @Test
    void maybeFinalizeJobMarksCommittedOnlyWhenNoReadyOrFailedItemsRemain() {
        when(itemMapper.selectCount(any(Wrapper.class)))
                .thenReturn(
                        0L, // COMMITTING
                        3L, // COMMITTED
                        0L, // READY
                        0L  // COMMIT_FAILED
                );

        ReflectionTestUtils.invokeMethod(service, "maybeFinalizeJobBody", 10L, 20L);

        ArgumentCaptor<ProductImportJobEntity> captor =
                ArgumentCaptor.forClass(ProductImportJobEntity.class);
        verify(jobMapper).updateById(captor.capture());
        assertThat(captor.getValue().getStatus()).isEqualTo(ImportJobStatus.COMMITTED.value());
        assertThat(captor.getValue().getCommittedAt()).isNotNull();
    }

    // ====================================================================
    // 40 §2.5 / §5.7: TransactionTemplate 包覆 + rollback 行為驗證
    // commitOne 主鏈路 5 個 self-invocation method 必須走 tx,mid-flight throw 必須 rollback
    // ====================================================================

    /**
     * markCommitting body 任一 mid-flight throw(如 attempt INSERT 撞 UNIQUE)必須 rollback。
     * 證據:TestTxManager.beginCount >= 1(代表 tx 開了 = 走 TransactionTemplate)
     * 且 rollbackCount >= 1(代表 throw 後真的 rollback)。
     */
    @Test
    void markCommittingMidFlightThrowFiresRollback() {
        ProductImportItemEntity item = buildItem(100L, ImportItemStatus.READY.value(), 1);
        when(itemMapper.selectById(100L)).thenReturn(item);
        // 模擬 attempt INSERT 撞 UNIQUE
        when(attemptMapper.selectOne(any())).thenReturn(null);
        doThrow(new RuntimeException("simulated attempt INSERT duplicate"))
                .when(attemptMapper).insert(any());

        try {
            service.commitOne(buildEvent(100L, 1));
        } catch (RuntimeException ignored) {
            // 預期 throw 冒到外面讓 MQ listener requeue
        }

        assertThat(txManager.getBeginCount())
                .as("markCommittingBody 必須走 TransactionTemplate")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("mid-flight throw 必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
    }

    /**
     * applySuccess body mid-flight throw(item UPDATE 已執行,asset UPDATE 拋例外)必須 rollback,
     * 避免「item 已 COMMITTED 但 asset 仍 UPLOADED」造成 orphan GC 誤刪。
     */
    @Test
    void applySuccessMidFlightThrowFiresRollback() {
        setupHappyCommitChain(100L, "555");
        // 模擬 applySuccessBody 內 asset UPDATE 拋例外(item.update 後第二步)
        doThrow(new RuntimeException("simulated asset UPDATE failure"))
                .when(assetMapper).update(isNull(), any(UpdateWrapper.class));

        try {
            service.commitOne(buildEvent(100L, 1));
        } catch (RuntimeException ignored) {
        }

        assertThat(txManager.getBeginCount())
                .as("applySuccessBody 必須走 TransactionTemplate")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("mid-flight throw 必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
    }

    /**
     * applyValidationDrift body mid-flight throw 必須 rollback。
     * 路徑:validateDraft 回 hasBlockingError=true → applyValidationDrift → errorMapper.insert 第二筆拋例外。
     */
    @Test
    void applyValidationDriftMidFlightThrowFiresRollback() {
        ProductImportItemEntity item = buildItem(100L, ImportItemStatus.READY.value(), 1);
        when(itemMapper.selectById(100L)).thenReturn(item);
        when(skuMapper.selectList(any())).thenReturn(List.of());
        when(assetMapper.selectList(any())).thenReturn(List.of());
        when(attemptMapper.selectOne(any())).thenReturn(null);
        // validateDraft 回 hasBlockingError → 走 applyValidationDriftBody
        when(productClient.validateProductDraft(
                eq(20L), any(), eq(ValidateProductDraftRequest.ValidationContext.IMPORT_COMMIT)))
                .thenReturn(ValidateProductDraftResponse.newBuilder()
                        .addViolations(DraftViolation.newBuilder()
                                .setCode("SKUS_REQUIRED").setSeverity("ERROR")
                                .setField("skus").setMessage("required").build())
                        .setHasBlockingError(true)
                        .build());
        // applyValidationDriftBody 內 INSERT violation 拋例外
        doThrow(new RuntimeException("simulated error INSERT failure"))
                .when(errorMapper).insert(any());

        try {
            service.commitOne(buildEvent(100L, 1));
        } catch (RuntimeException ignored) {
        }

        assertThat(txManager.getBeginCount())
                .as("applyValidationDriftBody 必須走 TransactionTemplate")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("mid-flight throw 必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
    }

    /**
     * applyFailure body mid-flight throw 必須 rollback。
     * 路徑:createProduct 回 business error code → applyFailureBody → attempt UPDATE 拋例外。
     */
    @Test
    void applyFailureMidFlightThrowFiresRollback() {
        ProductImportItemEntity item = buildItem(100L, ImportItemStatus.READY.value(), 1);
        when(itemMapper.selectById(100L)).thenReturn(item);
        when(skuMapper.selectList(any())).thenReturn(List.of());
        when(assetMapper.selectList(any())).thenReturn(List.of());
        when(attemptMapper.selectOne(any())).thenReturn(null);
        when(productClient.validateProductDraft(eq(20L), any(), any()))
                .thenReturn(ValidateProductDraftResponse.newBuilder()
                        .setHasBlockingError(false).build());
        // createProduct 回 business failure
        when(productClient.createProductFromImport(any()))
                .thenReturn(CreateProductFromImportInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.INVALID_PARAM.code())
                        .setMessage("invalid param")
                        .build());
        // applyFailureBody 內 attempt UPDATE 拋例外
        doThrow(new RuntimeException("simulated attempt UPDATE failure"))
                .when(attemptMapper).update(isNull(), any(UpdateWrapper.class));

        try {
            service.commitOne(buildEvent(100L, 1));
        } catch (RuntimeException ignored) {
        }

        assertThat(txManager.getBeginCount())
                .as("applyFailureBody 必須走 TransactionTemplate")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.getRollbackCount())
                .as("mid-flight throw 必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
    }

    /**
     * maybeFinalizeJob body 結尾 MQ publish 拋例外時必須 rollback,
     * 避免「job 已標 COMMITTED + event_id 已 set 但 event 沒發」的不一致。
     * 透過 commitOne happy commit 路徑走到 maybeFinalizeJobBody,在 publish 階段 throw。
     */
    @Test
    void maybeFinalizeJobPublishThrowFiresRollback() throws Exception {
        setupHappyCommitChain(100L, "555");
        // 設定 COMMITTING=0 + COMMITTED=1 → 進入 finalize 路徑
        when(itemMapper.selectCount(any(Wrapper.class)))
                .thenReturn(0L, 1L, 0L, 0L);
        doThrow(new RuntimeException("simulated MQ publish failure"))
                .when(publisher).publish(any(), any(), any(byte[].class), any());

        try {
            service.commitOne(buildEvent(100L, 1));
        } catch (RuntimeException ignored) {
            // publish 失敗以 IllegalStateException 包裝冒出
        }

        assertThat(txManager.getBeginCount())
                .as("commitOne 經 markCommitting + applySuccess + maybeFinalizeJob 至少開 3 個 tx")
                .isGreaterThanOrEqualTo(3);
        assertThat(txManager.getRollbackCount())
                .as("MQ publish 失敗必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
    }

    /**
     * Happy path commit:全程經 TransactionTemplate,只 commit 不 rollback。
     */
    @Test
    void happyCommitGoesThroughTxAndOnlyCommits() {
        setupHappyCommitChain(100L, "555");
        when(itemMapper.selectCount(any(Wrapper.class)))
                .thenReturn(0L, 1L, 0L, 0L);  // COMMITTING=0 + COMMITTED=1 → job → COMMITTED

        service.commitOne(buildEvent(100L, 1));

        assertThat(txManager.getBeginCount())
                .as("commitOne 應該開啟多個 tx(markCommitting + applySuccess + maybeFinalizeJob 至少 3 個)")
                .isGreaterThanOrEqualTo(3);
        assertThat(txManager.getCommitCount())
                .as("happy path 全部 tx 應該 commit")
                .isEqualTo(txManager.getBeginCount());
        assertThat(txManager.getRollbackCount())
                .as("happy path 不應 rollback")
                .isEqualTo(0);
    }

    // ====================================================================
    // Helpers
    // ====================================================================

    private ProductImportItemEntity buildItem(long itemId, String status, int commitRev) {
        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setId(itemId);
        item.setImportJobId(10L);
        item.setStoreId(20L);
        item.setStatus(status);
        item.setCommitRevision(commitRev);
        return item;
    }

    private CommitItemEvent buildEvent(long itemId, int commitRev) {
        return new CommitItemEvent(itemId, 10L, 20L, commitRev, "4001",
                "idem:" + itemId + ":" + commitRev);
    }

    /** 設定 commitOne happy path 所需 mocks 走到 applySuccessBody。 */
    private void setupHappyCommitChain(long itemId, String productId) {
        ProductImportItemEntity item = buildItem(itemId, ImportItemStatus.READY.value(), 1);
        when(itemMapper.selectById(itemId)).thenReturn(item);
        when(skuMapper.selectList(any())).thenReturn(List.of());
        when(assetMapper.selectList(any())).thenReturn(List.of());
        when(attemptMapper.selectOne(any())).thenReturn(null);
        when(productClient.validateProductDraft(eq(20L), any(), any()))
                .thenReturn(ValidateProductDraftResponse.newBuilder()
                        .setHasBlockingError(false).build());
        when(productClient.createProductFromImport(any()))
                .thenReturn(CreateProductFromImportInternalResponse.newBuilder()
                        .setCode(GrpcResponseCode.SUCCESS.code())
                        .setProductId(productId)
                        .build());
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

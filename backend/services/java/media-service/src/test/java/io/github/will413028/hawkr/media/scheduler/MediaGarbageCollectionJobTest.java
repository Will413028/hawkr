package io.github.will413028.hawkr.media.scheduler;

import io.github.will413028.hawkr.common.utils.code.MediaUploadSessionStatus;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.repository.AssetReferenceRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.service.ImportAssetService;
import io.github.will413028.hawkr.media.storage.StorageService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.util.Collections;
import java.util.List;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * MediaGarbageCollectionJob 單元測試
 * <p>
 * 測試重點：
 * 1. 過期 Session 清理條件
 * 2. 孤兒 Asset 清理條件（7 天緩衝期）
 * 3. 09 §8.1.1 / 40 §2.5 Tx 載體規定：三個 clean method 必須由 {@code txTemplate.execute(...)} 包覆
 *
 * <p>因 self-invoke @Transactional 修正，三個 clean method 改為 {@code private}，
 * 不再可直接呼叫；本測試一律透過 {@code runGarbageCollection()} 入口驅動，
 * 並以 {@link TestTxManager} 計數 begin/commit/rollback 驗證 tx 包覆。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class MediaGarbageCollectionJobTest {

    @Mock
    private UploadSessionRepository uploadSessionRepository;

    @Mock
    private MediaAssetRepository mediaAssetRepository;

    @Mock
    private AssetReferenceRepository assetReferenceRepository;

    @Mock
    private StorageService storageService;

    @Mock
    private ImportAssetService importAssetService;

    private TestTxManager txManager;
    private MediaGarbageCollectionJob gcJob;

    @BeforeEach
    void setUp() {
        txManager = new TestTxManager();
        gcJob = new MediaGarbageCollectionJob(uploadSessionRepository, mediaAssetRepository,
                assetReferenceRepository, storageService, importAssetService, txManager);
        ReflectionTestUtils.setField(gcJob, "orphanAssetDays", 7);
        ReflectionTestUtils.setField(gcJob, "batchSize", 100);
        ReflectionTestUtils.setField(gcJob, "gcEnabled", true);
        ReflectionTestUtils.setField(gcJob, "importAssetExpiredRetentionDays", 30);
        // issue #2294 L8：加了 lease 閘門之後，Mockito 對 int 預設回 0 ⇒ 每一筆都 skip ⇒
        // 本檔所有「應該刪掉」的斷言全紅。這裡 stub 成「搶到」，讓原本的斷言回到它們要驗的事；
        // 閘門本身的兩條分支由 MediaGarbageCollectionJobClaimTest 覆蓋。
        when(uploadSessionRepository.claimSessionForGc(any(), any(), any())).thenReturn(1);
        when(mediaAssetRepository.claimAssetForGc(anyLong(), any(), any(), any())).thenReturn(1);
    }

    @Nested
    @DisplayName("過期 Session 清理測試")
    class ExpiredSessionTests {

        @Test
        @DisplayName("應清理狀態為 INIT 且過期的 Session")
        void shouldCleanExpiredInitSessions() {
            // Given
            UploadSessionEntity expiredSession = new UploadSessionEntity();
            expiredSession.setId(UUID.randomUUID());
            expiredSession.setObjectKey("tmp/20260101/test.jpg");
            expiredSession.setStatus(MediaUploadSessionStatus.INIT.value());
            expiredSession.setExpiresAt(DateTimeUtils.nowUtc().minusHours(1));

            when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(expiredSession))
                .thenReturn(Collections.emptyList());
            when(uploadSessionRepository.markSessionExpired(any())).thenReturn(1);

            // When — 透過 runGarbageCollection 入口走 txTemplate 包覆路徑
            gcJob.runGarbageCollection();

            // Then — issue #2294 L8：整批 markExpiredSessions 已改成 per-row markSessionExpired
            //（整批語意會把別的 pod 剛搶走的列一起標成 EXPIRED，與 per-row 租約不相容）
            verify(storageService).deleteObject(expiredSession.getObjectKey());
            verify(uploadSessionRepository, atLeastOnce()).markSessionExpired(expiredSession.getId());
            assertTrue(txManager.beginCount >= 1, "至少一個 clean method 需開 tx");
            assertEquals(0, txManager.rollbackCount, "無 throw，禁止 rollback");
        }

        @Test
        @DisplayName("應清理狀態為 COMPLETED 且過期的 Session")
        void shouldCleanExpiredCompletedSessions() {
            UploadSessionEntity expiredSession = new UploadSessionEntity();
            expiredSession.setId(UUID.randomUUID());
            expiredSession.setObjectKey("tmp/20260101/test.jpg");
            expiredSession.setStatus(MediaUploadSessionStatus.COMPLETED.value());
            expiredSession.setExpiresAt(DateTimeUtils.nowUtc().minusHours(1));

            when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(expiredSession))
                .thenReturn(Collections.emptyList());
            when(uploadSessionRepository.markSessionExpired(any())).thenReturn(1);

            gcJob.runGarbageCollection();

            verify(storageService).deleteObject(expiredSession.getObjectKey());
        }

        @Test
        @DisplayName("不應清理未過期的 Session")
        void shouldNotCleanNonExpiredSessions() {
            when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(Collections.emptyList());
            when(uploadSessionRepository.markSessionExpired(any())).thenReturn(0);

            gcJob.runGarbageCollection();

            verify(storageService, never()).deleteObject(anyString());
        }
    }

    @Nested
    @DisplayName("孤兒 Asset 清理測試")
    class OrphanedAssetTests {

        @Test
        @DisplayName("應清理無引用且超過 7 天的 Asset")
        void shouldCleanOrphanedAssets() {
            MediaAssetEntity orphanedAsset = new MediaAssetEntity();
            orphanedAsset.setId(123456789L);
            orphanedAsset.setObjectKey("public/20260101/test.jpg");
            orphanedAsset.setCreatedAt(DateTimeUtils.nowUtc().minusDays(10));

            when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(List.of(orphanedAsset))
                .thenReturn(Collections.emptyList());
            when(mediaAssetRepository.deleteByIds(anyList())).thenReturn(1);

            gcJob.runGarbageCollection();

            verify(storageService).deleteObject(orphanedAsset.getObjectKey());
            verify(mediaAssetRepository).deleteByIds(anyList());
        }

        @Test
        @DisplayName("不應清理有引用的 Asset（即使超過 7 天）")
        void shouldNotCleanAssetWithReferences() {
            when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());

            gcJob.runGarbageCollection();

            verify(storageService, never()).deleteObject(anyString());
            verify(mediaAssetRepository, never()).deleteByIds(anyList());
        }

        @Test
        @DisplayName("不應清理無引用但未超過 7 天的 Asset（緩衝期保護）")
        void shouldNotCleanRecentOrphanedAssets() {
            when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());

            gcJob.runGarbageCollection();

            verify(storageService, never()).deleteObject(anyString());
        }

        /**
         * R2 刪除失敗時**不得**刪 DB 列。舊實作只 warn 一行就整批 deleteByIds，
         * 於是 R2 上留下一個 DB 再也不認得的物件 —— 沒有 media_asset 列就不會被任何一輪
         * GC 掃到，永久孤兒（dev 上實測到活的例子）。
         */
        @Test
        @DisplayName("R2 刪除失敗 → 保留該 DB 列待下輪重試，只刪成功的那些")
        void shouldKeepDbRowWhenStorageDeleteFails() {
            MediaAssetEntity ok = new MediaAssetEntity();
            ok.setId(111L);
            ok.setObjectKey("public/ok.jpg");
            MediaAssetEntity failing = new MediaAssetEntity();
            failing.setId(222L);
            failing.setObjectKey("public/failing.jpg");

            when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(List.of(ok, failing))
                .thenReturn(Collections.emptyList());
            doThrow(new RuntimeException("R2 unavailable"))
                .when(storageService).deleteObject("public/failing.jpg");
            when(mediaAssetRepository.deleteByIds(anyList())).thenReturn(1);

            gcJob.runGarbageCollection();

            // 只刪掉物件真的刪成功的那一筆
            verify(mediaAssetRepository).deleteByIds(List.of(111L));
            verify(mediaAssetRepository, never()).deleteByIds(List.of(111L, 222L));
        }

        /**
         * 部分失敗時必須跳出批次迴圈：失敗那幾列會被下一輪 selectOrphanedAssets 再撈到，
         * 繼續 do/while 只會拿到同一批而無限迴圈。
         */
        @Test
        @DisplayName("部分失敗 → 提前結束批次迴圈，不重撈同一批")
        void shouldBreakBatchLoopOnPartialFailure() {
            MediaAssetEntity failing = new MediaAssetEntity();
            failing.setId(222L);
            failing.setObjectKey("public/failing.jpg");

            // 若沒有 break，第二次 select 會再回同一批 → 呼叫次數 > 1
            when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(List.of(failing));
            doThrow(new RuntimeException("R2 unavailable"))
                .when(storageService).deleteObject("public/failing.jpg");

            gcJob.runGarbageCollection();

            verify(mediaAssetRepository, times(1)).selectOrphanedAssets(any(), any(), anyInt());
            verify(mediaAssetRepository, never()).deleteByIds(anyList());
        }
    }

    @Nested
    @DisplayName("GC 開關測試")
    class GcEnabledTests {

        @Test
        @DisplayName("GC 停用時不應執行")
        void shouldSkipWhenDisabled() {
            ReflectionTestUtils.setField(gcJob, "gcEnabled", false);

            gcJob.runGarbageCollection();

            verify(uploadSessionRepository, never()).selectExpiredSessions(any(), anyInt());
            verify(mediaAssetRepository, never()).selectOrphanedAssets(any(), any(), anyInt());
            assertEquals(0, txManager.beginCount, "停用時禁止開 tx");
        }
    }

    @Nested
    @DisplayName("Tx 載體規定回歸測試（09 §8.1.1 / 40 §2.5）")
    class TxCarrierTests {

        @Test
        @DisplayName("三個 clean method 必須由 TransactionTemplate 包覆")
        void cleanMethodsAreWrappedInTransactionTemplate() {
            when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(Collections.emptyList());
            when(uploadSessionRepository.markSessionExpired(any())).thenReturn(0);
            when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());
            when(assetReferenceRepository.deleteOrphanedReferences()).thenReturn(0);

            gcJob.runGarbageCollection();

            // 三個 clean method 各開一次 tx：cleanExpiredSessions / cleanOrphanedAssets / cleanStaleReferences
            assertEquals(3, txManager.beginCount,
                    "self-invocation bypass 已修正：三個 clean method 各自開 tx");
            assertEquals(3, txManager.commitCount, "無 throw，三 tx 全 commit");
            assertEquals(0, txManager.rollbackCount);
        }

        @Test
        @DisplayName("clean method mid-flight throw 必須觸發 rollback（非 commit）")
        void midFlightThrowTriggersRollback() {
            UploadSessionEntity expired = new UploadSessionEntity();
            expired.setId(UUID.randomUUID());
            expired.setObjectKey("tmp/x.jpg");
            expired.setStatus(MediaUploadSessionStatus.INIT.value());
            when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(expired));
            // 模擬 mid-flight 失敗：markSessionExpired throw → tx 必須 rollback
            when(uploadSessionRepository.markSessionExpired(any()))
                .thenThrow(new RuntimeException("simulated DB failure"));

            // runGarbageCollection 內 catch all Exception，不會拋出
            gcJob.runGarbageCollection();

            assertTrue(txManager.rollbackCount >= 1,
                    "mid-flight throw 必須 rollback，否則 self-invocation 失效 regression");
        }
    }

    /**
     * In-memory PlatformTransactionManager — 不開實際 DB tx，計數 begin/commit/rollback。
     */
    private static class TestTxManager implements PlatformTransactionManager {
        int beginCount = 0;
        int commitCount = 0;
        int rollbackCount = 0;

        @Override
        public TransactionStatus getTransaction(TransactionDefinition definition) {
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

package io.github.will413028.hawkr.productimport.scheduler;

import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.helper.ImportJobProgressCalculator;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportMediaCallbackInboxMapper;
import io.github.will413028.hawkr.productimport.storage.ProductImportStorage;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.isNull;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 40 §2.5 / §5.9.2:5 個 tier method 必須走 TransactionTemplate,
 * tier 間獨立 — 單 tier 失敗不阻擋下一 tier(runTierSafely 保證)。
 */
class ProductImportGcJobTest {

    private final ProductImportJobMapper jobMapper = mock(ProductImportJobMapper.class);
    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ProductImportItemAssetMapper assetMapper = mock(ProductImportItemAssetMapper.class);
    private final ProductImportMediaCallbackInboxMapper inboxMapper =
            mock(ProductImportMediaCallbackInboxMapper.class);
    private final ProductImportStorage csvStorage = mock(ProductImportStorage.class);
    private final ObsRabbitPublisher publisher = mock(ObsRabbitPublisher.class);
    private final ImportJobProgressCalculator progressCalculator = mock(ImportJobProgressCalculator.class);
    private final TestTxManager txManager = new TestTxManager();

    private final ProductImportGcJob job = new ProductImportGcJob(
            jobMapper, itemMapper, assetMapper, inboxMapper,
            csvStorage, publisher, progressCalculator, txManager);


    // ==================== 副作用失敗無訊號修復（出路 4） ====================

    /**
     * 核心保證：{@code parsed_event_published_at} 的 CAS 與 {@code job.parsed} 的 publish
     * 必須在同一個短 tx 內，publish 失敗即 rollback。
     *
     * <p>{@code parsed_event_published_at} 是永久旗標（候選述詞 {@code IS NULL}，寫進 NOW() 之後
     * 永遠回不來）。修復前 publish 失敗只記 error log ⇒ 旗標已提交、事件沒送出
     * ⇒ {@code job.parsed} 永久不發，而 §22 禁止 reconciliation job ⇒ 無救濟。
     * 同 repo 的正確版本是 {@code AssetSettleHelper.onAssetSettled}（publish 失敗重拋）。
     *
     * <p>鑑別力：把 publish 的 catch 改回只記 log（或拿掉 publishTxTemplate 包覆）
     * ⇒ rollbackCount 變 0 ⇒ 轉紅。
     */
    @Test
    void jobParsedPublishFailure_rollsBackTheOneShotFlag() throws Exception {
        enableGc();
        stubOneStaleClaimedAssetReadyToPublish();
        doThrow(new RuntimeException("simulated MQ outage"))
                .when(publisher).publish(anyString(), anyString(), any(byte[].class), anyString());

        job.runCallbackTimeoutGc();

        assertThat(txManager.getRollbackCount())
                .as("publish 失敗必須 rollback，否則 parsed_event_published_at 已落地而事件永久遺失")
                .isEqualTo(1);
        assertThat(txManager.getCommitCount())
                .as("publish 失敗禁止 commit")
                .isEqualTo(0);
    }

    /** publish 成功 → CAS 與事件一起 commit。 */
    @Test
    void jobParsedPublishSuccess_commits() throws Exception {
        enableGc();
        stubOneStaleClaimedAssetReadyToPublish();

        job.runCallbackTimeoutGc();

        assertThat(txManager.getCommitCount()).isEqualTo(1);
        assertThat(txManager.getRollbackCount()).isEqualTo(0);
    }

    /**
     * 佈置：callback-timeout GC 撈到一筆 stale DOWNLOADING asset、claim 成功、
     * 該 job 的所有 asset 都已 settled ⇒ 會走到 checkAndPublishParsed 的 CAS + publish。
     */
    private void stubOneStaleClaimedAssetReadyToPublish() {
        ProductImportItemAssetEntity stale = new ProductImportItemAssetEntity();
        stale.setId(701L);
        stale.setImportJobId(70L);
        stale.setImportItemId(700L);
        stale.setStatus(ImportAssetLocalStatus.DOWNLOADING.value());
        when(assetMapper.selectList(any())).thenReturn(List.of(stale));
        when(assetMapper.claimStaleDownloadingAsset(eq(701L), any(), any(), any())).thenReturn(1);
        when(itemMapper.selectById(700L)).thenReturn(null);

        ProductImportJobEntity job70 = new ProductImportJobEntity();
        job70.setId(70L);
        job70.setStoreId(9001L);
        job70.setStatus(ImportJobStatus.PARSING.value());
        job70.setParsedEventPublishedAt(null);
        when(jobMapper.selectById(70L)).thenReturn(job70);
        when(progressCalculator.counters(70L)).thenReturn(new ImportJobProgressCalculator.Counters(
                1, 0, 0, 1, 0, 0, 0, 0, 0,
                1, 0, 0, 0, 1, 0));
        // CAS 命中（首次 publish）
        when(jobMapper.update(isNull(), any(UpdateWrapper.class))).thenReturn(1);
    }

    /** @Value 預設值在 unit test 不會 inject — 手動設 gcEnabled = true。 */
    private void enableGc() {
        ReflectionTestUtils.setField(job, "gcEnabled", true);
        ReflectionTestUtils.setField(job, "orphanGraceHours", 24);
        ReflectionTestUtils.setField(job, "expiredRetentionDays", 7);
        ReflectionTestUtils.setField(job, "committedRetentionDays", 7);
        ReflectionTestUtils.setField(job, "inboxRetentionDays", 14);
        ReflectionTestUtils.setField(job, "callbackTimeoutMinutes", 30);
    }

    @Test
    void tierAExpireJobsBodyThrowFiresRollbackAndDoesNotBlockOtherTiers() {
        enableGc();

        // tierA 有 1 個 expired candidate,UPDATE 拋例外 → 該 tier rollback
        ProductImportJobEntity expired = new ProductImportJobEntity();
        expired.setId(99L);
        expired.setStatus(ImportJobStatus.READY.value());
        when(jobMapper.findExpiredCandidates(any(), anyInt())).thenReturn(List.of(expired));
        // issue #2294 L9：tierA 加了 lease 閘門之後，Mockito 對 int 預設回 0 ⇒ 這一筆會被 skip
        // ⇒ updateById 根本不會被呼叫 ⇒ 不拋例外 ⇒ 本支的 rollback 斷言失效（轉紅）。
        // 這裡 stub 成「搶到」，讓 rollback 斷言回到它原本要驗的事（tier 間獨立）；
        // 閘門本身的兩條分支由 ProductImportGcJobClaimTest 覆蓋。
        when(jobMapper.claimExpiredJobForGc(anyLong(), any(), any())).thenReturn(1);
        doThrow(new RuntimeException("simulated tierA UPDATE failure"))
                .when(jobMapper).updateById(any());
        // tierB-E:空候選 list,正常跑完
        when(assetMapper.findOrphanCandidates(any(), anyInt())).thenReturn(List.of());
        when(jobMapper.selectList(any())).thenReturn(List.of());
        when(itemMapper.selectList(any())).thenReturn(List.of());
        when(inboxMapper.delete(any())).thenReturn(0);

        job.runDailyGc();

        // 5 tier 各開一次 tx，外加 R4-P1-2 之後 tierA 那一筆 claim 的獨立交易
        //（PROPAGATION_REQUIRES_NEW；claim 必須自己 commit 才會跨 pod 立即可見）。
        assertThat(txManager.getBeginCount())
                .as("5 tier 各走 TransactionTemplate 各開 1 個 tx，+1 個 tierA claim 的獨立 tx")
                .isEqualTo(6);
        // tierA throw → rollback;tierB-E 空 list 無寫入 → commit
        assertThat(txManager.getRollbackCount())
                .as("tierA throw 必須觸發 rollback")
                .isGreaterThanOrEqualTo(1);
        assertThat(txManager.getCommitCount())
                .as("tierB-E 即使 tierA 失敗仍然跑完")
                .isGreaterThanOrEqualTo(4);
    }

    @Test
    void runDailyGcHappyPathAllTiersCommit() {
        enableGc();
        when(jobMapper.findExpiredCandidates(any(), anyInt())).thenReturn(List.of());
        when(assetMapper.findOrphanCandidates(any(), anyInt())).thenReturn(List.of());
        when(jobMapper.selectList(any())).thenReturn(List.of());
        when(itemMapper.selectList(any())).thenReturn(List.of());
        when(inboxMapper.delete(any())).thenReturn(0);

        job.runDailyGc();

        assertThat(txManager.getBeginCount()).isEqualTo(5);
        assertThat(txManager.getCommitCount()).isEqualTo(5);
        assertThat(txManager.getRollbackCount()).isEqualTo(0);
    }

    @Test
    void disabledGcSkipsAllTiers() {
        ReflectionTestUtils.setField(job, "gcEnabled", false);

        job.runDailyGc();

        assertThat(txManager.getBeginCount()).isEqualTo(0);
    }

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

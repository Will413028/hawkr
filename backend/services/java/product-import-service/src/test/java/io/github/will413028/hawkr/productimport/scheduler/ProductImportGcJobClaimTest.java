package io.github.will413028.hawkr.productimport.scheduler;

import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
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
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>L9</b>：{@link ProductImportGcJob} 兩條會外溢副作用的 GC 路徑的 lease 閘門。
 *
 * <ul>
 *   <li>{@code runCallbackTimeoutGc}：lease on 既有的 {@code updated_at}（零 migration）</li>
 *   <li>{@code tierAExpireJobsBody}：lease on 新增的 {@code gc_lease_until}（V10）</li>
 * </ul>
 */
class ProductImportGcJobClaimTest {

    /** {@code gc.cron-daily} 每日 02:00 ⇒ tierA 週期（分）。租約必須明顯小於它。 */
    private static final long TIER_A_PERIOD_MINUTES = 24 * 60L;

    private final ProductImportJobMapper jobMapper = mock(ProductImportJobMapper.class);
    private final ProductImportItemMapper itemMapper = mock(ProductImportItemMapper.class);
    private final ProductImportItemAssetMapper assetMapper = mock(ProductImportItemAssetMapper.class);
    private final ProductImportMediaCallbackInboxMapper inboxMapper =
            mock(ProductImportMediaCallbackInboxMapper.class);
    private final ProductImportStorage csvStorage = mock(ProductImportStorage.class);
    private final ObsRabbitPublisher publisher = mock(ObsRabbitPublisher.class);
    private final ImportJobProgressCalculator progressCalculator = mock(ImportJobProgressCalculator.class);

    private final ProductImportGcJob job = new ProductImportGcJob(
            jobMapper, itemMapper, assetMapper, inboxMapper,
            csvStorage, publisher, progressCalculator, new NoopTxManager());

    @BeforeEach
    void enableGc() {
        ReflectionTestUtils.setField(job, "gcEnabled", true);
        ReflectionTestUtils.setField(job, "orphanGraceHours", 24);
        ReflectionTestUtils.setField(job, "expiredRetentionDays", 7);
        ReflectionTestUtils.setField(job, "committedRetentionDays", 7);
        ReflectionTestUtils.setField(job, "inboxRetentionDays", 14);
        ReflectionTestUtils.setField(job, "callbackTimeoutMinutes", 30);
    }

    private static ProductImportJobEntity expiredJob(long id) {
        ProductImportJobEntity j = new ProductImportJobEntity();
        j.setId(id);
        j.setStatus(ImportJobStatus.READY.value());
        j.setExpiresAt(DateTimeUtils.nowUtc().minusHours(1));
        return j;
    }

    private static ProductImportItemAssetEntity staleAsset(long id) {
        ProductImportItemAssetEntity a = new ProductImportItemAssetEntity();
        a.setId(id);
        a.setImportJobId(900L);
        a.setImportItemId(800L);
        a.setStatus(ImportAssetLocalStatus.DOWNLOADING.value());
        return a;
    }

    private void emptyOtherTiers() {
        when(assetMapper.findOrphanCandidates(any(), anyInt())).thenReturn(List.of());
        when(jobMapper.selectList(any())).thenReturn(List.of());
        when(itemMapper.selectList(any())).thenReturn(List.of());
        when(inboxMapper.delete(any())).thenReturn(0);
    }

    // ==================== tierA ====================

    /**
     * 反向斷言：claim 回 0 ⇒ 不准做狀態轉換、不准發 MQ。
     * Mockito 對 {@code int} 預設回 0，只有這一支能證明「0 真的擋住了」。
     */
    @Test
    @DisplayName("tierA claim 回 0 ⇒ 不轉 EXPIRED、不發 expire_job MQ")
    void tierAClaimLost_doesNothing() {
        when(jobMapper.findExpiredCandidates(any(), anyInt())).thenReturn(List.of(expiredJob(99L)));
        when(jobMapper.claimExpiredJobForGc(anyLong(), any(), any())).thenReturn(0);
        emptyOtherTiers();

        job.runDailyGc();

        verify(jobMapper, never()).updateById(any());
        verify(itemMapper, never()).update(any(), any());
        verify(publisher, never()).publish(anyString(), anyString(), any(byte[].class), anyString());
    }

    @Test
    @DisplayName("tierA claim 回 1 ⇒ 先 claim 再轉 EXPIRED 再發 MQ")
    void tierAClaimWon_processesInOrder() throws Exception {
        when(jobMapper.findExpiredCandidates(any(), anyInt())).thenReturn(List.of(expiredJob(99L)));
        when(jobMapper.claimExpiredJobForGc(anyLong(), any(), any())).thenReturn(1);
        emptyOtherTiers();

        job.runDailyGc();

        var inOrder = org.mockito.Mockito.inOrder(jobMapper, publisher);
        inOrder.verify(jobMapper).claimExpiredJobForGc(anyLong(), any(), any());
        inOrder.verify(jobMapper).updateById(any());
        inOrder.verify(publisher).publish(anyString(), anyString(), any(byte[].class), anyString());
    }

    /**
     * 租約長度的<b>上界</b>。只驗「是未來時間」對 1440 分照樣成立，沒有鑑別力；
     * 這裡釘的是「租約 ≤ tierA 週期的 1/10」——租約接近或超過每日週期時，
     * claim 之後崩潰的那一筆會連著跳過下一次 daily GC。
     */
    @Test
    @DisplayName("tierA 租約 = 60 分且 ≤ 排程週期的 1/10（改成天級會轉紅）")
    void tierALeaseIsBoundedBySchedulePeriod() {
        when(jobMapper.findExpiredCandidates(any(), anyInt())).thenReturn(List.of(expiredJob(99L)));
        when(jobMapper.claimExpiredJobForGc(anyLong(), any(), any())).thenReturn(1);
        emptyOtherTiers();

        LocalDateTime before = DateTimeUtils.nowUtc();
        job.runDailyGc();

        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(jobMapper).claimExpiredJobForGc(anyLong(), any(), lease.capture());
        long leaseMinutes = Duration.between(before, lease.getValue()).toMinutes();

        assertThat(leaseMinutes)
                .as("租約必須是未來時間（0 或負 ⇒ 任何人都搶得到 ⇒ 等於沒有搶佔）")
                .isGreaterThan(0);
        assertThat(leaseMinutes * 10)
                .as("租約 %d 分超過 tierA 排程週期(%d 分)的 1/10：claim 之後崩潰的那一筆"
                        + "會連著跳過下一次 daily GC", leaseMinutes, TIER_A_PERIOD_MINUTES)
                .isLessThanOrEqualTo(TIER_A_PERIOD_MINUTES);
        assertThat(ProductImportGcJob.TIER_A_LEASE_MINUTES).isEqualTo(60L);
    }

    /** 租約基準必須每筆各自取（tierA 一次撈 500 筆）。 */
    @Test
    @DisplayName("tierA 租約基準取 claim 當下——第二筆嚴格晚於第一筆")
    void tierALeaseBaselineIsPerItem() {
        when(jobMapper.findExpiredCandidates(any(), anyInt()))
                .thenReturn(List.of(expiredJob(99L), expiredJob(100L)));
        when(jobMapper.claimExpiredJobForGc(anyLong(), any(), any())).thenReturn(1);
        emptyOtherTiers();
        when(jobMapper.updateById(any())).thenAnswer(inv -> {
            Thread.sleep(40L);
            return 1;
        });

        job.runDailyGc();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(jobMapper, times(2)).claimExpiredJobForGc(anyLong(), any(), leases.capture());
        long deltaMs = Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();
        assertThat(deltaMs)
                .as("兩者相等 ⇒ 租約基準在迴圈外只算一次 ⇒ 不變式退化成「批次總耗時 < 租約」，"
                        + "而 tierA 一次撈 500 筆")
                .isGreaterThanOrEqualTo(20L);
    }

    // ==================== callback timeout ====================

    @Test
    @DisplayName("callback timeout claim 回 0 ⇒ 不轉 FAILED、不發 job.parsed")
    void callbackTimeoutClaimLost_doesNothing() {
        when(assetMapper.selectList(any())).thenReturn(List.of(staleAsset(1L)));
        when(assetMapper.claimStaleDownloadingAsset(anyLong(), anyString(), any(), any()))
                .thenReturn(0);

        job.runCallbackTimeoutGc();

        verify(assetMapper, never()).updateById(any());
        verify(jobMapper, never()).selectById(anyLong());
        verify(publisher, never()).publish(anyString(), anyString(), any(byte[].class), anyString());
    }

    @Test
    @DisplayName("callback timeout claim 回 1 ⇒ 先 claim 再 DOWNLOADING→FAILED")
    void callbackTimeoutClaimWon_processes() {
        when(assetMapper.selectList(any())).thenReturn(List.of(staleAsset(1L)));
        when(assetMapper.claimStaleDownloadingAsset(anyLong(), anyString(), any(), any()))
                .thenReturn(1);
        when(itemMapper.selectById(anyLong())).thenReturn(null);
        when(jobMapper.selectById(anyLong())).thenReturn(null);

        job.runCallbackTimeoutGc();

        var inOrder = org.mockito.Mockito.inOrder(assetMapper);
        inOrder.verify(assetMapper).claimStaleDownloadingAsset(anyLong(), anyString(), any(), any());
        inOrder.verify(assetMapper).updateById(any());
    }

    /**
     * 有效租約長度＝{@code callbackTimeoutMinutes}（把 {@code updated_at} 推到 claim 當下，
     * 該列要再過 30 分鐘才會重新滿足 {@code updated_at < now − 30min}）。
     * 這一支釘住「claim 寫進去的是<b>當下</b>而不是未來某個很遠的時間」——
     * 寫成 {@code now + 1 天} 會讓有效租約變成 1 天 + 30 分，遠超排程週期 5 分鐘。
     */
    @Test
    @DisplayName("callback timeout 的 claimedAt 是 claim 當下（不是未來時間）")
    void callbackTimeoutClaimStampIsNow() {
        when(assetMapper.selectList(any())).thenReturn(List.of(staleAsset(1L)));
        when(assetMapper.claimStaleDownloadingAsset(anyLong(), anyString(), any(), any()))
                .thenReturn(1);

        LocalDateTime before = DateTimeUtils.nowUtc();
        job.runCallbackTimeoutGc();
        LocalDateTime after = DateTimeUtils.nowUtc();

        ArgumentCaptor<LocalDateTime> stamp = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(assetMapper)
                .claimStaleDownloadingAsset(anyLong(), anyString(), any(), stamp.capture());
        assertThat(stamp.getValue())
                .as("claim 寫進 updated_at 的必須是「claim 當下」——寫未來時間會讓有效租約"
                        + "（= callbackTimeoutMinutes）憑空變長，超過排程週期")
                .isBetween(before.minusSeconds(1), after.plusSeconds(1));
    }

    // ==================== R4-P1-2 / P2-2 ====================

    /**
     * R4-P1-2（2026-08-08）：tierA 的 claim 必須是<b>獨立、立即 commit</b> 的 UPDATE，
     * 不能掛在 tierA 的批次交易裡。
     *
     * <p>tierA 特有的嚴重性：批次交易 rollback 會把 claim 一起撤掉，但同一輪
     * <b>已經 publish 出去的 {@code gc.expire_job} 撤不回</b>——事件說了一件沒發生的事，
     * 而下一輪還會再發一次。
     *
     * <p><b>怎麼量</b>：比較 claim 當下與 publish 當下的累計 commit 次數。claim 掛在批次交易裡
     * 時兩者相等（批次交易要到 tierA 整輪結束才 commit）。
     */
    @Test
    @DisplayName("tierA claim 在獨立交易內 commit（移回批次交易會轉紅）")
    void tierAClaimCommitsBeforePublish() {
        CountingTxManager tx = new CountingTxManager();
        ProductImportGcJob j = new ProductImportGcJob(jobMapper, itemMapper, assetMapper,
                inboxMapper, csvStorage, publisher, progressCalculator, tx);
        ReflectionTestUtils.setField(j, "gcEnabled", true);
        ReflectionTestUtils.setField(j, "orphanGraceHours", 24);
        ReflectionTestUtils.setField(j, "expiredRetentionDays", 7);
        ReflectionTestUtils.setField(j, "committedRetentionDays", 7);
        ReflectionTestUtils.setField(j, "inboxRetentionDays", 14);

        int[] commitsAtClaim = {-1};
        int[] commitsAtPublish = {-1};

        when(jobMapper.findExpiredCandidates(any(), anyInt())).thenReturn(List.of(expiredJob(99L)));
        when(jobMapper.claimExpiredJobForGc(anyLong(), any(), any())).thenAnswer(inv -> {
            commitsAtClaim[0] = tx.commitCount;
            return 1;
        });
        org.mockito.Mockito.doAnswer(inv -> {
            commitsAtPublish[0] = tx.commitCount;
            return null;
        }).when(publisher).publish(anyString(), anyString(), any(byte[].class), anyString());
        emptyOtherTiers();

        j.runDailyGc();

        assertThat(commitsAtClaim[0]).as("claim 必須被呼叫到（測試前提）").isGreaterThanOrEqualTo(0);
        assertThat(commitsAtPublish[0]).as("publish 必須被呼叫到（測試前提）").isGreaterThanOrEqualTo(0);
        assertThat(commitsAtPublish[0])
                .as("claim 與 publish 之間必須有一次 commit（實測 %d → %d）。相等 ⇒ claim 掛在 "
                        + "tierA 的批次交易裡：rollback 撤得掉 claim，撤不掉已經發出去的 "
                        + "gc.expire_job，而且對方 pod 的排程執行緒會被卡住整批 500 筆",
                        commitsAtClaim[0], commitsAtPublish[0])
                .isGreaterThan(commitsAtClaim[0]);
    }

    /**
     * R4-P2-2（2026-08-08）：claim 之後的 {@code updateById(job)} 會把<b>實體上那份查詢當下的
     * 舊 {@code gc_lease_until}</b>（多半是 NULL）寫回去，抵銷剛剛 claim 到的租約。
     *
     * <p>今天之所以沒炸，純粹因為同一句把 status 推成 {@code EXPIRED('9107')}，而 9107 在
     * {@code findExpiredCandidates} / {@code claimExpiredJobForGc} 的 {@code NOT IN} 排除清單裡，
     * 該列無論如何都離開候選集合——這是<b>隱性耦合</b>，候選述詞一改就變成 bug。
     */
    @Test
    @DisplayName("tierA claim 之後 updateById 寫回的是新租約，不是查詢當下的舊值")
    void tierAUpdateByIdCarriesClaimedLease() {
        when(jobMapper.findExpiredCandidates(any(), anyInt())).thenReturn(List.of(expiredJob(99L)));
        when(jobMapper.claimExpiredJobForGc(anyLong(), any(), any())).thenReturn(1);
        emptyOtherTiers();

        job.runDailyGc();

        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(jobMapper).claimExpiredJobForGc(anyLong(), any(), lease.capture());

        ArgumentCaptor<ProductImportJobEntity> saved =
                ArgumentCaptor.forClass(ProductImportJobEntity.class);
        verify(jobMapper).updateById(saved.capture());

        assertThat(saved.getValue().getGcLeaseUntil())
                .as("updateById 寫回的 gc_lease_until 必須等於剛 claim 到的租約；"
                        + "沿用實體上查詢當下的舊值（此處為 null）等於把 claim 自己抵銷掉")
                .isEqualTo(lease.getValue());
    }

    private static class NoopTxManager implements PlatformTransactionManager {
        @Override
        public TransactionStatus getTransaction(TransactionDefinition definition) {
            return new SimpleTransactionStatus(true);
        }

        @Override
        public void commit(TransactionStatus status) {
        }

        @Override
        public void rollback(TransactionStatus status) {
        }
    }

    /** 計數用 tx manager：commit 次數就足以量出「claim 有沒有自己 commit」。 */
    private static class CountingTxManager implements PlatformTransactionManager {
        int commitCount;

        @Override
        public TransactionStatus getTransaction(TransactionDefinition definition) {
            return new SimpleTransactionStatus(true);
        }

        @Override
        public void commit(TransactionStatus status) {
            commitCount++;
        }

        @Override
        public void rollback(TransactionStatus status) {
        }
    }
}

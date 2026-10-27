package io.github.will413028.hawkr.media.scheduler;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
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
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.Collections;
import java.util.List;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>L8</b>：{@link MediaGarbageCollectionJob} 的 lease 閘門。
 *
 * <p>本 job 的副作用是<b>不可逆的外部儲存刪除</b>（{@code StorageService.deleteObject}），
 * 而它原本<b>完全沒有搶佔</b>。閘門必須在 {@code deleteObject} 之前，
 * 所以「claim 回 0 ⇒ 一個物件都不准刪」是本檔最重要的斷言。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class MediaGarbageCollectionJobClaimTest {

    /** {@code media.gc.cron} 預設每日 03:00 ⇒ GC 週期（分）。租約必須明顯小於它。 */
    private static final long GC_PERIOD_MINUTES = 24 * 60L;

    @Mock private UploadSessionRepository uploadSessionRepository;
    @Mock private MediaAssetRepository mediaAssetRepository;
    @Mock private AssetReferenceRepository assetReferenceRepository;
    @Mock private StorageService storageService;
    @Mock private ImportAssetService importAssetService;

    private MediaGarbageCollectionJob gcJob;

    @BeforeEach
    void setUp() {
        gcJob = new MediaGarbageCollectionJob(uploadSessionRepository, mediaAssetRepository,
                assetReferenceRepository, storageService, importAssetService, new NoopTxManager());
        ReflectionTestUtils.setField(gcJob, "orphanAssetDays", 7);
        ReflectionTestUtils.setField(gcJob, "batchSize", 100);
        ReflectionTestUtils.setField(gcJob, "gcEnabled", true);
        ReflectionTestUtils.setField(gcJob, "importAssetExpiredRetentionDays", 30);
    }

    private static UploadSessionEntity session(String key) {
        UploadSessionEntity s = new UploadSessionEntity();
        s.setId(UUID.randomUUID());
        s.setObjectKey(key);
        s.setStatus(MediaUploadSessionStatus.INIT.value());
        s.setExpiresAt(DateTimeUtils.nowUtc().minusHours(1));
        return s;
    }

    private static MediaAssetEntity asset(long id, String key) {
        MediaAssetEntity a = new MediaAssetEntity();
        a.setId(id);
        a.setObjectKey(key);
        a.setCreatedAt(DateTimeUtils.nowUtc().minusDays(10));
        return a;
    }

    /**
     * 反向斷言（本檔核心）：claim 回 0 ⇒ <b>不准刪任何物件、不准標任何狀態</b>。
     *
     * <p>Mockito 對 {@code int} 預設回 0，所以只有這一支能證明「0 真的擋住了」——
     * 正向測試在閘門被寫成永遠成立的條件時照樣綠。
     */
    @Test
    @DisplayName("session claim 回 0 ⇒ 不刪 OSS 物件、不標 EXPIRED")
    void sessionClaimLost_deletesNothing() {
        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(session("tmp/a.jpg")))
                .thenReturn(Collections.emptyList());
        when(uploadSessionRepository.claimSessionForGc(any(), any(), any())).thenReturn(0);
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());

        gcJob.runGarbageCollection();

        verify(storageService, never()).deleteObject(anyString());
        verify(uploadSessionRepository, never()).markSessionExpired(any());
    }

    @Test
    @DisplayName("asset claim 回 0 ⇒ 不刪 OSS 物件、不刪 DB 列")
    void assetClaimLost_deletesNothing() {
        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(Collections.emptyList());
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(List.of(asset(111L, "public/a.jpg")))
                .thenReturn(Collections.emptyList());
        when(mediaAssetRepository.claimAssetForGc(anyLong(), any(), any(), any())).thenReturn(0);

        gcJob.runGarbageCollection();

        verify(storageService, never()).deleteObject(anyString());
        verify(mediaAssetRepository, never()).deleteByIds(any());
    }

    @Test
    @DisplayName("claim 回 1 ⇒ 先 claim 再 deleteObject，最後才 per-row 標 EXPIRED")
    void claimWon_claimsBeforeDeleting() {
        UploadSessionEntity s = session("tmp/a.jpg");
        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(s))
                .thenReturn(Collections.emptyList());
        when(uploadSessionRepository.claimSessionForGc(any(), any(), any())).thenReturn(1);
        when(uploadSessionRepository.markSessionExpired(any())).thenReturn(1);
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());

        gcJob.runGarbageCollection();

        var inOrder = org.mockito.Mockito.inOrder(uploadSessionRepository, storageService);
        inOrder.verify(uploadSessionRepository).claimSessionForGc(any(), any(), any());
        inOrder.verify(storageService).deleteObject("tmp/a.jpg");
        inOrder.verify(uploadSessionRepository).markSessionExpired(s.getId());
    }

    /**
     * 儲存端刪除失敗 ⇒ 該列<b>不</b>進終態（留著租約等下一輪），但已經刪成功的那些照標。
     *
     * <p>這是把舊版「任一筆失敗就整輪不標記」換成 per-row 之後的正確行為；
     * 舊語意與 per-row 租約不相容（會把別的 pod 剛搶走的列一起標成 EXPIRED）。
     */
    @Test
    @DisplayName("per-row：刪除失敗那筆不標 EXPIRED，成功那筆照標")
    void perRowMarking_onlyMarksSucceeded() {
        UploadSessionEntity ok = session("tmp/ok.jpg");
        UploadSessionEntity bad = session("tmp/bad.jpg");
        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(ok, bad))
                .thenReturn(Collections.emptyList());
        when(uploadSessionRepository.claimSessionForGc(any(), any(), any())).thenReturn(1);
        when(uploadSessionRepository.markSessionExpired(any())).thenReturn(1);
        org.mockito.Mockito.doThrow(new RuntimeException("R2 unavailable"))
                .when(storageService).deleteObject("tmp/bad.jpg");
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());

        gcJob.runGarbageCollection();

        verify(uploadSessionRepository).markSessionExpired(ok.getId());
        verify(uploadSessionRepository, never()).markSessionExpired(bad.getId());
    }

    /**
     * 租約長度的<b>上界</b>：必須明顯小於 GC 週期（每日），否則刪除失敗的那一列會連著
     * 跳過下一次 GC。只驗「是未來時間」對 1440 分照樣成立，沒有鑑別力。
     */
    @Test
    @DisplayName("租約 = 60 分且 ≤ GC 週期的 1/10（改成天級會轉紅）")
    void leaseLengthIsBoundedByGcPeriod() {
        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(session("tmp/a.jpg")))
                .thenReturn(Collections.emptyList());
        when(uploadSessionRepository.claimSessionForGc(any(), any(), any())).thenReturn(1);
        when(uploadSessionRepository.markSessionExpired(any())).thenReturn(1);
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());

        LocalDateTime before = DateTimeUtils.nowUtc();
        gcJob.runGarbageCollection();

        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(uploadSessionRepository).claimSessionForGc(any(), any(), lease.capture());
        long leaseMinutes = Duration.between(before, lease.getValue()).toMinutes();

        assertTrue(leaseMinutes > 0, "租約必須是未來時間（0 或負 ⇒ 等於沒有搶佔）");
        assertTrue(leaseMinutes * 10 <= GC_PERIOD_MINUTES,
                "租約實測 " + leaseMinutes + " 分 > GC 週期(" + GC_PERIOD_MINUTES
                        + " 分)的 1/10。租約接近或超過 GC 週期 ⇒ 刪除失敗的那一列會連著跳過"
                        + "下一次 GC，儲存端的孤兒物件多滯留一整天。");
        assertTrue(MediaGarbageCollectionJob.GC_LEASE_MINUTES == 60L,
                "租約常數應為 60 分，實際 " + MediaGarbageCollectionJob.GC_LEASE_MINUTES);
    }

    /** 租約基準必須在迴圈內、claim 當下取（batch-size 預設 100）。 */
    @Test
    @DisplayName("租約基準取 claim 當下——第二筆嚴格晚於第一筆")
    void leaseBaselineIsPerItem() {
        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(session("tmp/a.jpg"), session("tmp/b.jpg")))
                .thenReturn(Collections.emptyList());
        when(uploadSessionRepository.claimSessionForGc(any(), any(), any())).thenReturn(1);
        when(uploadSessionRepository.markSessionExpired(any())).thenReturn(1);
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());
        doAnswer(inv -> {
            Thread.sleep(40L);
            return null;
        }).when(storageService).deleteObject(anyString());

        gcJob.runGarbageCollection();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(uploadSessionRepository, times(2)).claimSessionForGc(any(), any(), leases.capture());
        long deltaMs = Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();
        assertTrue(deltaMs >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                        + "兩者相等 ⇒ 基準在迴圈外只算一次 ⇒ 不變式退化成「批次總耗時 < 租約」。");
    }

    // ==================== R4-P1-2：claim 必須在批次交易之外 ====================

    /**
     * R4-P1-2（2026-08-08）：<b>claim 必須是獨立、立即 commit 的 UPDATE</b>，不能掛在整批 GC
     * 的交易裡（規範《多副本排程搶佔規範》＋ 08 §8：「它必須跨 pod 立即可見」）。
     *
     * <p><b>怎麼量</b>：記錄 claim 當下與 {@code deleteObject}（副作用）當下的累計 commit 次數。
     * claim 若掛在批次交易裡，兩者相等（批次交易要到整輪結束才 commit）；
     * claim 若走自己的交易，中間必然多一次 commit。
     *
     * <p>這一支是本檔唯一量得到「交易邊界」的測試——既有的 inOrder／租約長度斷言全部只看
     * <b>呼叫順序與參數</b>，claim 掛在哪個交易裡對它們完全透明。
     */
    @Test
    @DisplayName("session claim 在獨立交易內 commit（移回批次交易會轉紅）")
    void sessionClaimCommitsBeforeSideEffect() {
        CountingTxManager tx = new CountingTxManager();
        MediaGarbageCollectionJob job = new MediaGarbageCollectionJob(uploadSessionRepository,
                mediaAssetRepository, assetReferenceRepository, storageService, importAssetService, tx);
        ReflectionTestUtils.setField(job, "orphanAssetDays", 7);
        ReflectionTestUtils.setField(job, "batchSize", 100);
        ReflectionTestUtils.setField(job, "gcEnabled", true);
        ReflectionTestUtils.setField(job, "importAssetExpiredRetentionDays", 30);

        int[] commitsAtClaim = {-1};
        int[] commitsAtDelete = {-1};

        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(List.of(session("tmp/a.jpg")))
                .thenReturn(Collections.emptyList());
        when(uploadSessionRepository.claimSessionForGc(any(), any(), any())).thenAnswer(inv -> {
            commitsAtClaim[0] = tx.commitCount;
            return 1;
        });
        doAnswer(inv -> {
            commitsAtDelete[0] = tx.commitCount;
            return null;
        }).when(storageService).deleteObject(anyString());
        when(uploadSessionRepository.markSessionExpired(any())).thenReturn(1);
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(Collections.emptyList());

        job.runGarbageCollection();

        assertTrue(commitsAtClaim[0] >= 0 && commitsAtDelete[0] >= 0,
                "claim 與 deleteObject 都必須被呼叫到（測試前提）");
        assertTrue(commitsAtDelete[0] > commitsAtClaim[0],
                "claim 與副作用之間必須有一次 commit（實測 claim 時 " + commitsAtClaim[0]
                        + " 次、deleteObject 時 " + commitsAtDelete[0] + " 次）。"
                        + "相等 ⇒ claim 掛在整批 GC 的交易裡，要到整輪結束才對其他 pod 可見："
                        + "對方 pod 的排程執行緒（pool.size 預設 1）被卡住整批，"
                        + "且無 ORDER BY 時兩 pod 取鎖順序相反會 deadlock。");
    }

    @Test
    @DisplayName("asset claim 在獨立交易內 commit（移回批次交易會轉紅）")
    void assetClaimCommitsBeforeSideEffect() {
        CountingTxManager tx = new CountingTxManager();
        MediaGarbageCollectionJob job = new MediaGarbageCollectionJob(uploadSessionRepository,
                mediaAssetRepository, assetReferenceRepository, storageService, importAssetService, tx);
        ReflectionTestUtils.setField(job, "orphanAssetDays", 7);
        ReflectionTestUtils.setField(job, "batchSize", 100);
        ReflectionTestUtils.setField(job, "gcEnabled", true);
        ReflectionTestUtils.setField(job, "importAssetExpiredRetentionDays", 30);

        int[] commitsAtClaim = {-1};
        int[] commitsAtDelete = {-1};

        when(uploadSessionRepository.selectExpiredSessions(any(), anyInt()))
                .thenReturn(Collections.emptyList());
        when(mediaAssetRepository.selectOrphanedAssets(any(), any(), anyInt()))
                .thenReturn(List.of(asset(111L, "public/a.jpg")))
                .thenReturn(Collections.emptyList());
        when(mediaAssetRepository.claimAssetForGc(anyLong(), any(), any(), any())).thenAnswer(inv -> {
            commitsAtClaim[0] = tx.commitCount;
            return 1;
        });
        doAnswer(inv -> {
            commitsAtDelete[0] = tx.commitCount;
            return null;
        }).when(storageService).deleteObject(anyString());

        job.runGarbageCollection();

        assertTrue(commitsAtClaim[0] >= 0 && commitsAtDelete[0] >= 0,
                "claim 與 deleteObject 都必須被呼叫到（測試前提）");
        assertTrue(commitsAtDelete[0] > commitsAtClaim[0],
                "claim 與副作用之間必須有一次 commit（實測 " + commitsAtClaim[0]
                        + " → " + commitsAtDelete[0] + "）。相等 ⇒ claim 掛在整批 GC 的交易裡。");
    }

    // ==================== R4-P2-1：claim 必須重述「無 asset_reference」 ====================

    /**
     * R4-P2-1（2026-08-08）：孤兒 asset 的 claim 述詞必須<b>重述候選查詢的每一個條件</b>，
     * 包含「沒有任何 {@code asset_reference}」。
     *
     * <p>候選查詢用 {@code LEFT JOIN asset_reference … WHERE ar.id IS NULL} 判「無引用」，
     * 但 SELECT 與 claim 之間隔著同批前幾筆的 OSS DELETE（秒級）。期間若有人替這個 asset
     * 建了引用（商品綁圖／匯入轉正），claim 少了這個條件就照樣搶得到，接著把一個
     * <b>已被引用</b>的物件從儲存端刪掉——不可逆。claim 是這條路徑上最後一個能重新驗證
     * 候選條件的位置。
     *
     * <p>用 SQL 註解掃描而非行為斷言：Repository 是 MyBatis {@code @Update} 註解介面，
     * 單元測試裡是 mock，唯一能證明「述詞真的長這樣」的就是註解本身。
     */
    @Test
    @DisplayName("claimAssetForGc 的 SQL 必須帶 asset_reference 的 NOT EXISTS 條件")
    void assetClaimRestatesNoReferencePredicate() throws Exception {
        String sql = MediaAssetRepository.class
                .getMethod("claimAssetForGc", Long.class, LocalDateTime.class,
                        LocalDateTime.class, LocalDateTime.class)
                .getAnnotation(org.apache.ibatis.annotations.Update.class)
                .value()[0]
                .replaceAll("\\s+", " ");

        assertTrue(sql.contains("NOT EXISTS") && sql.contains("asset_reference"),
                "claimAssetForGc 的述詞缺少「無 asset_reference」條件，SELECT 與 claim 之間"
                        + "新增引用的 asset 仍會被搶到並刪除物件（TOCTOU）。實際 SQL: " + sql);
        // 🔴 要擋的是「**未關聯**的 NOT EXISTS」——那等於問「asset_reference 表是不是空的」，
        // 恆為 false ⇒ 條件形同不存在，而症狀是刪掉已被引用的物件。
        //
        // ⚠️ 不能只認 `media_asset.id` 這個字面值：2026-09-02 為了讓法定文件例外能與候選查詢
        // 共用同一份述詞，UPDATE 加了別名 `ma`（`UPDATE media_asset ma … ar.asset_id = ma.id`）。
        // 兩種寫法**語意相同**，而只認其中一種會把一次正確的重構誤報成 TOCTOU 破口。
        // 判準因此改成「有沒有關聯到外層那張表的 id」，兩種形式都接受。
        assertTrue(sql.contains("ar.asset_id = media_asset.id") || sql.contains("ar.asset_id = ma.id"),
                "NOT EXISTS 必須關聯外層 media_asset 的 id（`media_asset.id` 或別名 `ma.id`），"
                        + "否則是無條件的存在性檢查。實際 SQL: " + sql);
        // 別名存在時，UPDATE 的目標表必須真的帶那個別名，否則 `ma.id` 指向不存在的關聯。
        if (sql.contains("ar.asset_id = ma.id")) {
            assertTrue(sql.contains("UPDATE media_asset ma"),
                    "用了別名 ma 關聯，但 UPDATE 沒有宣告該別名。實際 SQL: " + sql);
        }
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

    /** 計數用 tx manager：只需要 commit 次數就能量出「claim 有沒有自己 commit」。 */
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

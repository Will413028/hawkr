package io.github.will413028.hawkr.media.scheduler;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.entity.UploadSessionEntity;
import io.github.will413028.hawkr.media.repository.AssetReferenceRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import io.github.will413028.hawkr.media.repository.UploadSessionRepository;
import io.github.will413028.hawkr.media.service.ImportAssetService;
import io.github.will413028.hawkr.media.storage.StorageService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

/**
 * 媒體資源垃圾回收排程任務
 * <p>
 * 依設計書 4.4 定義：
 * 1. 清理 Staging 孤兒：找出 upload_session 狀態為 INIT/COMPLETED 且過期者
 * 2. 清理未引用 Asset (Soft GC)：找出無引用且創建超過 7 天的 media_asset
 *
 * <p>Tx 載體規定（對齊 09 設計書 §8.1.1 / 40 設計書 §2.5）：
 * 三個清理 method 從 {@code runGarbageCollection} 內部 self-invoke，必須由 caller 用
 * {@link TransactionTemplate#execute} 程式式包覆，**禁止**用 {@code @Transactional}
 * annotation —— Spring AOP CGLIB proxy 不攔截 {@code this.cleanXxx(...)}，annotation 失效後
 * 任一階段 mid-flight throw 不會 rollback，留下「OSS 已 deleteObject 但 DB row 未刪」
 * 的孤兒副作用。
 *
 * <p><b>搶佔語意（issue #2294 L8）</b>：原本<b>完全沒有搶佔</b>，而副作用是<b>不可逆的
 * 外部儲存刪除</b>。修法＝<b>per-row lease</b> on 新增的 {@code upload_session.gc_lease_until}
 * 與 {@code media_asset.gc_lease_until}（V8 migration），claim 成功才 {@code deleteObject}。
 * <ul>
 *   <li>⚠️ 不可挪用 {@code upload_session.expires_at} 當租約：它是<b>業務有效性判準</b>
 *       （{@code UploadSessionService} 拿它擋過期 session），
 *       往後推等於重新打開一個已過期的上傳窗口。</li>
 *   <li>⚠️ 舊版的整批 {@code markExpiredSessions} 與 per-row 租約<b>語意衝突</b>
 *       （它會把別的 pod 剛搶走的列一起標成 EXPIRED），已改成 per-row
 *       {@code markSessionExpired}——順帶修掉「任一筆失敗就整輪不標記」這個過度保守的行為。</li>
 *   <li>{@code cleanStaleReferences} <b>不加租約</b>，理由見該方法的 javadoc。</li>
 * </ul>
 *
 * <p><b>claim 必須在批次交易之外（R4-P1-2，2026-08-08）</b>：兩支 claim 一度被寫在
 * {@code txTemplate.execute(...)} 的批次交易<b>內</b>，違反《多副本排程搶佔規範》與 08 §8
 * 「claim 是自動提交的獨立 UPDATE，不放進 {@code TransactionTemplate.execute(...)} 內——
 * 它必須跨 pod 立即可見」。互斥本身仍成立（另一個 pod 的 CAS 會卡在 row lock，
 * 待本 tx commit 後重評述詞得到 0 列），但代價有三：
 * <ol>
 *   <li>對方 pod 的<b>排程執行緒</b>被卡在本 pod<b>整批</b> GC 的交易壽命上，而 Spring
 *       {@code spring.task.scheduling.pool.size} 預設是 1 ⇒ 該服務所有排程一起停；
 *       兩個 pod 的 cron 同為 {@code 0 0 3 * * ?}，碰撞是必然而非偶然。</li>
 *   <li>兩支候選查詢原本<b>沒有 {@code ORDER BY}</b>，PostgreSQL 的
 *       {@code synchronize_seqscans} 讓並行 seq scan 從不同位置起掃 ⇒ 兩個 pod 的取鎖順序
 *       可以相反 ⇒ <b>deadlock</b>，被 DB 砍掉的那個 pod 整輪 GC rollback。</li>
 *   <li>批次 rollback 會把已經 commit 不了的 claim 一起撤掉，而 {@code deleteObject}
 *       是撤不回的。</li>
 * </ol>
 * 修法＝claim 走獨立的 {@link #claimTxTemplate}（{@code PROPAGATION_REQUIRES_NEW}，
 * 單句 UPDATE 立即 commit、立即跨 pod 可見、鎖即刻釋放），並替兩支候選查詢加上穩定的
 * {@code ORDER BY}（消除 ② 的取鎖順序不一致；有序取鎖下互相等待不成環）。
 *
 * <p>設計書：08 §8
 */
@Component
public class MediaGarbageCollectionJob {

    private static final ObsLogger log = ObsLogger.forClass(MediaGarbageCollectionJob.class);

    /**
     * issue #2294 L8：GC 租約長度（分）。
     *
     * <p><b>下界</b>：單筆最壞耗時 = 一次 OSS/R2 {@code DeleteObject}（HTTP，秒級），60 分是兩個
     * 數量級以上的餘裕。<b>上界</b>：必須明顯小於 GC 週期（cron 每日 03:00 ＝ 1440 分），
     * 否則刪除失敗的那一列會連著跳過下一次 GC。取 1/24。
     *
     * <p>候選述詞（{@code expires_at < now} / {@code created_at < threshold}）都只有下界、
     * 沒有時間上界 ⇒ 候選窗口寬度 ∞ ⇒ 租約不會退化成永久旗標，唯一要守的就是上面那個上界。
     */
    static final long GC_LEASE_MINUTES = 60L;

    private final UploadSessionRepository uploadSessionRepository;
    private final MediaAssetRepository mediaAssetRepository;
    private final AssetReferenceRepository assetReferenceRepository;
    private final StorageService storageService;
    private final ImportAssetService importAssetService;
    // 09 §8.1.1 / 40 §2.5 Tx 載體規定：同 class self-invocation 用 TransactionTemplate 程式式包覆
    private final TransactionTemplate txTemplate;
    /**
     * R4-P1-2：<b>只給 lease claim 用</b>的獨立交易載體（{@code PROPAGATION_REQUIRES_NEW}）。
     *
     * <p>claim 必須在自己的交易裡立即 commit——它是跨 pod 的搶佔閘門，語意上要求
     * 「寫下去的瞬間別的 pod 就看得到」。掛在上面那個批次交易裡的話，它要等整批做完才可見，
     * 於是別的 pod 的排程執行緒被卡在本批次的壽命上（見 class javadoc）。
     * <p>⚠️ 不要為了省一個欄位而共用 {@link #txTemplate}：兩者的傳播行為不同，
     * 共用等於把 claim 放回批次交易。
     */
    private final TransactionTemplate claimTxTemplate;

    @Value("${media.gc.orphan-asset-days:7}")
    private int orphanAssetDays;

    @Value("${media.gc.batch-size:100}")
    private int batchSize;

    @Value("${media.gc.enabled:true}")
    private boolean gcEnabled;

    /** import_asset EXPIRED 後保留天數（08 號設計書 §7.6）：超過則 DELETE row。 */
    @Value("${media.import.asset.expired-retention-days:30}")
    private int importAssetExpiredRetentionDays;

    public MediaGarbageCollectionJob(UploadSessionRepository uploadSessionRepository,
                                     MediaAssetRepository mediaAssetRepository,
                                     AssetReferenceRepository assetReferenceRepository,
                                     StorageService storageService,
                                     ImportAssetService importAssetService,
                                     PlatformTransactionManager txManager) {
        this.uploadSessionRepository = uploadSessionRepository;
        this.mediaAssetRepository = mediaAssetRepository;
        this.assetReferenceRepository = assetReferenceRepository;
        this.storageService = storageService;
        this.importAssetService = importAssetService;
        this.txTemplate = new TransactionTemplate(txManager);
        this.claimTxTemplate = new TransactionTemplate(txManager);
        this.claimTxTemplate.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    }

    /**
     * R4-P1-2：在<b>獨立交易</b>內執行一次 lease claim 並立刻 commit。
     *
     * <p>回傳 affected rows；0 = 租約由他人持有。呼叫端<b>必須</b>走這裡，不可直接呼叫
     * repository 的 {@code claimXxxForGc}——那會讓 claim 落在外層批次交易內。
     */
    private int claimInOwnTransaction(java.util.function.Supplier<Integer> claim) {
        Integer affected = claimTxTemplate.execute(status -> claim.get());
        return affected == null ? 0 : affected;
    }

    /**
     * 每日凌晨 3 點執行 GC
     */
    @Scheduled(cron = "${media.gc.cron:0 0 3 * * ?}")
    public void runGarbageCollection() {
        if (!gcEnabled) {
            return;
        }

        try {
            // 1. 清理過期的 Staging 會話（同 class self-invoke，必須由 txTemplate 包覆）
            int expiredSessions = txTemplate.execute(status -> cleanExpiredSessions());

            // 2. 清理無引用的孤兒資產
            int orphanedAssets = txTemplate.execute(status -> cleanOrphanedAssets());

            // 3. 清理孤立引用（asset 已不存在的 reference 記錄）
            int staleRefs = txTemplate.execute(status -> cleanStaleReferences());

            // 4. 清理 import_asset：過期未 Finalize → EXPIRED + 刪 OSS tmp；已 EXPIRED 滿保留期 → DELETE
            //    跨 class 呼叫 importAssetService（proxy 生效），無 self-invoke 問題
            int expiredImportAssets = cleanupExpiredImportAssets();
            int prunedImportAssets = pruneExpiredImportAssetRetention();

            int total = expiredSessions + orphanedAssets + staleRefs
                + expiredImportAssets + prunedImportAssets;
            if (total > 0) {
                log.info("gc.done", "垃圾回收完成, sessions=" + expiredSessions
                    + ", assets=" + orphanedAssets
                    + ", staleRefs=" + staleRefs
                    + ", importExpired=" + expiredImportAssets
                    + ", importPruned=" + prunedImportAssets);
            }

        } catch (Exception e) {
            log.error("gc.failed", "垃圾回收失敗", e);
        }
    }

    /**
     * 清理過期的 Staging 會話
     * <p>
     * 找出 upload_session 狀態為 INIT/COMPLETED 且過期者，
     * 刪除對應的 OSS 檔案並標記 EXPIRED
     *
     * <p>09 §8.1.1 / 40 §2.5：必須由 caller 包進 {@code txTemplate.execute(...)}。
     * 禁止標 {@code @Transactional}（self-invocation 會繞過 Spring AOP CGLIB proxy）。
     */
    private int cleanExpiredSessions() {
        // ⚠️ 這個 now 只用於「候選述詞」與「租約未到期判定」。租約的**基準**不在這裡取
        //（見下方迴圈內），否則整批共用同一個到期時間。
        LocalDateTime now = DateTimeUtils.nowUtc();
        int totalCleaned = 0;

        // 分批處理
        List<UploadSessionEntity> expiredSessions;
        do {
            expiredSessions = uploadSessionRepository.selectExpiredSessions(now, batchSize);

            for (UploadSessionEntity session : expiredSessions) {
                // issue #2294 L8：lease claim —— 搶到才刪 OSS 物件（0 = 另一個 pod 已搶走）。
                // 副作用是**不可逆的外部儲存刪除**，所以閘門必須在 deleteObject 之前。
                // ⚠️ 租約基準取 claim 當下而非本方法開頭：取開頭 ⇒ 全批共用同一個到期時間 ⇒
                // 不變式退化成「批次總耗時 < 租約」（會隨 media.gc.batch-size 一起破）；
                // 取 claim 當下 ⇒ 不變式降為「**單筆**最壞耗時（一次 OSS DELETE）< 租約」。
                // ⚠️ claim 走 claimTxTemplate（REQUIRES_NEW）而非外層批次交易：閘門要在寫下去的
                // 瞬間跨 pod 可見，且鎖要即刻釋放（理由與三項代價見 class javadoc）。
                LocalDateTime leaseUntil = DateTimeUtils.nowUtc().plusMinutes(GC_LEASE_MINUTES);
                int claimed = claimInOwnTransaction(
                    () -> uploadSessionRepository.claimSessionForGc(session.getId(), now, leaseUntil));
                if (claimed == 0) {
                    log.info("gc.session.claimSkip", "租約由他人持有，skip",
                        "session_id", session.getId().toString());
                    continue;
                }

                try {
                    storageService.deleteObject(session.getObjectKey());
                } catch (Exception e) {
                    // 儲存端刪除失敗 ⇒ 不推進狀態，該列帶著租約留到下一輪重試。
                    // （舊版是「整批依條件」更新，無法只標記成功的那幾筆，所以任一筆失敗就整輪不標記；
                    //   那個語意與 per-row 租約不相容——它會把別的 pod 剛搶走的列也一起標成 EXPIRED。）
                    log.warn("gc.session.ossFailed", "刪除 OSS 檔案失敗，保留狀態待下輪重試: " + e.getMessage(),
                        "session_id", session.getId().toString());
                    continue;
                }

                // per-row 標記過期：只標物件真的刪成功的那一筆
                totalCleaned += uploadSessionRepository.markSessionExpired(session.getId());
            }

            // 迴圈終止性：本輪撈到的每一列，不論「搶到並處理」「搶不到」「刪除失敗」，
            // gc_lease_until 都已 > now（搶到）或本來就 > now（他人持有）⇒ 下一圈的候選述詞
            // 不會再撈到它們 ⇒ 不會無限迴圈。
        } while (!expiredSessions.isEmpty());

        return totalCleaned;
    }

    /**
     * 清理無引用的孤兒資產
     * <p>
     * 找出無引用且創建超過 7 天的 media_asset，
     * 刪除 OSS 檔案和 DB 記錄
     *
     * <p>09 §8.1.1 / 40 §2.5：必須由 caller 包進 {@code txTemplate.execute(...)}。
     * 禁止標 {@code @Transactional}。
     */
    private int cleanOrphanedAssets() {
        // ⚠️ now 只用於候選述詞與租約未到期判定；租約基準在迴圈內取（理由同 cleanExpiredSessions）
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime threshold = now.minusDays(orphanAssetDays);
        int totalCleaned = 0;

        // 分批處理
        List<MediaAssetEntity> orphanedAssets;
        do {
            orphanedAssets = mediaAssetRepository.selectOrphanedAssets(threshold, now, batchSize);

            if (orphanedAssets.isEmpty()) {
                break;
            }

            // 只有「物件真的刪掉了」的資產才可以刪 DB 列。
            //
            // 舊實作把儲存端失敗降級成一行 warn，然後照樣 deleteByIds 整批。結果是 R2 上
            // 留下一個系統再也不認得的物件：DB 沒有 media_asset 列 → 不會再被任何一輪 GC
            // 掃到 → 永久孤兒（dev 實測到活的例子，物件回 200 但 media_asset 查無此列）。
            // S3/R2 的 DeleteObject 對不存在的 key 也回成功，所以這裡拋例外代表的是
            // 「儲存端不可用／權限不足」這種真的該重試的情況，不是「本來就沒有」。
            List<Long> deletableIds = new ArrayList<>();
            int claimLost = 0;
            for (MediaAssetEntity asset : orphanedAssets) {
                // issue #2294 L8：lease claim —— 搶到才刪 R2/OSS 物件（0 = 另一個 pod 已搶走）。
                // 租約基準取 claim 當下（理由同 cleanExpiredSessions）。
                // claim 走 claimTxTemplate（REQUIRES_NEW），理由同 cleanExpiredSessions。
                LocalDateTime leaseUntil = DateTimeUtils.nowUtc().plusMinutes(GC_LEASE_MINUTES);
                if (claimInOwnTransaction(
                        () -> mediaAssetRepository.claimAssetForGc(
                            asset.getId(), threshold, now, leaseUntil)) == 0) {
                    claimLost++;
                    log.info("gc.asset.claimSkip", "租約由他人持有，skip",
                        "asset_id", asset.getId().toString());
                    continue;
                }
                try {
                    storageService.deleteObject(asset.getObjectKey());
                    deletableIds.add(asset.getId());
                } catch (Exception e) {
                    log.warn("gc.asset.ossFailed", "刪除 OSS 檔案失敗，保留 DB 列待下輪重試: " + e.getMessage(),
                        "asset_id", asset.getId().toString());
                }
            }

            if (!deletableIds.isEmpty()) {
                totalCleaned += mediaAssetRepository.deleteByIds(deletableIds);
            }

            // 這一批有任何一筆刪不掉（或被別的 pod 搶走）就收工。
            // ⚠️ 加了租約之後，「無限迴圈」這個理由其實已經不成立（失敗與被搶走的列都帶著
            // 未到期的租約，下一圈的候選述詞不會再撈到它們）；保留 break 是為了維持既有語意：
            // 儲存端正在出問題時，不要在同一輪繼續打它。
            if (deletableIds.size() < orphanedAssets.size()) {
                log.warn("gc.asset.deferred", "本輪有物件刪除失敗或租約被搶走，提前結束孤兒清理, "
                    + "batch=" + orphanedAssets.size() + ", deleted=" + deletableIds.size()
                    + ", claimLost=" + claimLost);
                break;
            }

        } while (!orphanedAssets.isEmpty());

        return totalCleaned;
    }

    /**
     * 清理孤立引用（asset 已被刪除但 reference 仍殘留）
     *
     * <p>09 §8.1.1 / 40 §2.5：必須由 caller 包進 {@code txTemplate.execute(...)}。
     * 禁止標 {@code @Transactional}。
     *
     * <p><b>issue #2294 L8：這一段刻意<b>不</b>加租約</b>（與同 job 的另外兩段不同）。
     * 它整段就是<b>單一 bulk DELETE</b>（{@code deleteOrphanedReferences}），沒有 per-row 迴圈、
     * 沒有任何外部副作用（不碰儲存、不發 MQ）。兩個 pod 同時跑的結果是：其中一個刪到 N 列、
     * 另一個刪到 0 列，DB 自己保證原子性，沒有「重複副作用」可言，也沒有可以被搶佔的 row 身分。
     * 為它加租約欄需要一個它根本沒有的 per-row 判準，而換來的保護是零。
     * <p>⚠️ 若日後把它改成「先撈 id 再逐筆處理」或加上任何外部呼叫，這個豁免立刻失效，
     * 必須比照 {@code cleanExpiredSessions} 補 per-row 租約。
     */
    private int cleanStaleReferences() {
        int deleted = assetReferenceRepository.deleteOrphanedReferences();
        if (deleted > 0) {
            log.info("gc.staleRef.done", "清理孤立引用 " + deleted + " 筆");
        }
        return deleted;
    }

    /**
     * 手動觸發 GC（用於測試或緊急清理）
     */
    public void triggerManualGc() {
        log.info("gc.manual.trigger", "手動觸發垃圾回收");
        runGarbageCollection();
    }

    /**
     * GC 第 4 層第 1 階段：過期未 Finalize 的 import_asset
     * （status ∈ {DOWNLOADING, UPLOADED, FAILED} 且 expires_at &lt; now → EXPIRED + 刪 OSS tmp）。
     * <p>對應 08 號設計書 §7.6（被動兜底，與 §7.5 主動 gc.expire_job MQ 並存且 idempotent）。
     *
     * <p>由 {@code importAssetService}（跨 class）內部以 {@code @Transactional} 包覆，
     * proxy 生效，無 self-invoke 問題。
     *
     * @return 標 EXPIRED 的筆數
     */
    public int cleanupExpiredImportAssets() {
        int total = 0;
        int round;
        do {
            round = importAssetService.cleanupExpiredCandidates(batchSize);
            total += round;
        } while (round == batchSize);
        if (total > 0) {
            log.info("gc.importAsset.expired", "import_asset 過期清理完成, count=" + total);
        }
        return total;
    }

    /**
     * GC 第 4 層第 2 階段：已 EXPIRED 滿保留期的 import_asset → DELETE。
     *
     * @return 刪除筆數
     */
    public int pruneExpiredImportAssetRetention() {
        int total = 0;
        int round;
        do {
            round = importAssetService.cleanupExpiredRetention(importAssetExpiredRetentionDays, batchSize);
            total += round;
        } while (round == batchSize);
        if (total > 0) {
            log.info("gc.importAsset.pruned", "import_asset 保留期過後刪除完成, count=" + total);
        }
        return total;
    }
}

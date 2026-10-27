package io.github.will413028.hawkr.productimport.scheduler;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.ImportAssetFailureReason;
import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.common.utils.code.ImportJobStatus;
import io.github.will413028.hawkr.common.utils.code.ProductImportEventType;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportMediaCallbackInboxEntity;
import io.github.will413028.hawkr.productimport.helper.ImportJobProgressCalculator;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportJobMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportMediaCallbackInboxMapper;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.ExpireJobEvent;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.ImportJobParsedEvent;
import io.github.will413028.hawkr.productimport.storage.ProductImportStorage;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.TransactionTemplate;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Set;
import java.util.UUID;

/**
 * Product Import GC 排程任務（40 號設計書 §5.9）。
 *
 * <p>四層 + callback timeout 兜底：
 * <ul>
 *   <li>每 5 分：callback timeout（asset DOWNLOADING 超過 30min → FAILED）</li>
 *   <li>每日 02:00：A 過期 job、B ORPHAN asset、C/D 滿保留期清理、E inbox 清理</li>
 * </ul>
 *
 * <h2>搶佔語意（issue #2294 L9）</h2>
 *
 * <p>兩條會產生<b>外溢副作用</b>（狀態轉換 ＋ 發 MQ）的路徑原本<b>完全沒有搶佔</b>，
 * 多副本下同一筆會被兩個 pod 各處理一次、各發一次事件：
 *
 * <ul>
 *   <li><b>{@code runCallbackTimeoutGc}</b>：lease on <b>既有的</b>
 *       {@code product_import_item_asset.updated_at}（候選述詞本來就是
 *       {@code status=DOWNLOADING AND updated_at < cutoff}）⇒ <b>零 migration</b>。
 *       閘門之後的 {@code DOWNLOADING → FAILED} 與 {@code job.parsed} publish 順序不變。</li>
 *   <li><b>{@code tierAExpireJobsBody}</b>：lease on <b>新增的</b>
 *       {@code product_import_job.gc_lease_until}（V10）。⚠️ 這一支<b>不能</b>拿
 *       「{@code status} 轉 EXPIRED」當閘門——那是業務狀態轉換本身，也是永久旗標：
 *       claim 之後 pod 崩潰（進程消失、沒有任何 catch 會跑）⇒ 該列再也回不到候選集合
 *       ⇒ {@code gc.expire_job} 事件永久遺失且無告警。{@code expires_at} 也不可挪用
 *       （業務時戳，推它等於延長任務壽命）。</li>
 * </ul>
 *
 * <p>tierB～tierE 不在本次射程：它們的副作用是純 DB 刪除（{@code deleteById} / bulk
 * {@code delete}），重複執行第二次刪到 0 列，無外溢；tierB 的 publish 在 delete 之前，
 * 重複投遞由消費端冪等吸收。詳見 40 §8.5。
 *
 * <p>設計書：40 §5.9（GC 本體）／§5.9.3＋§8.5（多副本搶佔語意）／§8.4（補償與人工修復策略）
 */
@Component
public class ProductImportGcJob {

    private static final ObsLogger log = ObsLogger.forClass(ProductImportGcJob.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    /**
     * issue #2294 L9：tierA 的租約長度（分）。
     *
     * <p><b>下界</b>：單筆最壞耗時 = 一次 {@code updateById} ＋ 一次 item 批次 UPDATE ＋
     * 一次 MQ publish（皆為本地／同叢集，秒級），60 分是兩個數量級以上的餘裕。
     * <b>上界</b>：必須明顯小於 tierA 的排程週期（{@code cron-daily} 每日 02:00 ＝ 1440 分），
     * 否則 claim 之後崩潰的那一筆會連著跳過下一次 daily GC。取 1/24。
     *
     * <p>候選述詞 {@code expires_at < now} 只有下界、沒有時間上界 ⇒ 候選窗口寬度 ∞
     * ⇒ 租約不會退化成永久旗標，唯一要守的就是上面那個上界。
     */
    static final long TIER_A_LEASE_MINUTES = 60L;
    private static final Set<String> JOB_ACTIVE_FOR_PARSED_EVENT = Set.of(
            ImportJobStatus.READY.value(),
            ImportJobStatus.PARSING.value(),
            ImportJobStatus.COMMITTING.value());

    @Value("${hawkr.product-import.gc.enabled:true}")
    private boolean gcEnabled;

    @Value("${hawkr.product-import.asset.callback-timeout-minutes:30}")
    private int callbackTimeoutMinutes;

    @Value("${hawkr.product-import.orphan-asset-grace-hours:24}")
    private int orphanGraceHours;

    @Value("${hawkr.product-import.expired-retention-days:7}")
    private int expiredRetentionDays;

    @Value("${hawkr.product-import.committed-retention-days:7}")
    private int committedRetentionDays;

    @Value("${hawkr.product-import.callback-inbox-retention-days:14}")
    private int inboxRetentionDays;

    private final ProductImportJobMapper jobMapper;
    private final ProductImportItemMapper itemMapper;
    private final ProductImportItemAssetMapper assetMapper;
    private final ProductImportMediaCallbackInboxMapper inboxMapper;
    private final ProductImportStorage csvStorage;
    private final ObsRabbitPublisher publisher;
    private final ImportJobProgressCalculator progressCalculator;
    // 40 §2.5 Tx 載體規定:各 tier 為 self-invocation 短 tx,用程式式包覆
    private final TransactionTemplate txTemplate;
    /**
     * R4-P1-2（2026-08-08）：<b>只給 tierA 的 lease claim 用</b>的獨立交易載體
     * （{@code PROPAGATION_REQUIRES_NEW}）。
     *
     * <p>claim 一度落在 tierA 的批次交易內，違反《多副本排程搶佔規範》與 40 §5.9.3
     * 「claim 是自動提交的獨立 UPDATE，不放進 {@code TransactionTemplate.execute(...)} 內」
     *（tierA 專屬的載體推導與兩項代價在 40 §8.5）。
     * 互斥仍成立（對方 pod 的 CAS 卡在 row lock，本 tx commit 後重評述詞得 0 列），但：
     * <ul>
     *   <li>對方 pod 的排程執行緒被卡在本 pod <b>整批 500 筆</b>的交易壽命上，而
     *       {@code spring.task.scheduling.pool.size} 預設 1 ⇒ 該服務所有排程一起停。</li>
     *   <li><b>tierA 特有、也最嚴重的一項</b>：批次交易 rollback 會把 claim 一起撤掉，
     *       但同一輪<b>已經 publish 出去的 {@code gc.expire_job} 撤不回</b>
     *       ⇒ 事件說了一件（rollback 之後）沒發生的事，而下一輪還會再發一次。
     *       claim 走獨立交易後，rollback 不再動它，該列老實等租約到期。</li>
     * </ul>
     * ⚠️ 不要共用 {@link #txTemplate}：傳播行為不同，共用等於把 claim 放回批次交易。
     */
    private final TransactionTemplate claimTxTemplate;
    /**
     * {@code checkAndPublishParsed} 專屬的短交易載體（{@code PROPAGATION_REQUIRES_NEW}）。
     *
     * <p>只包「{@code parsed_event_published_at} CAS ＋ 一次 publish」，publish 失敗即 rollback
     * （《多副本搶佔規範》§四 出路 4）。用 REQUIRES_NEW 是為了讓它在<b>任何</b>呼叫脈絡下
     * 都是自己的短交易——不論呼叫端有沒有外層批次交易，都不會把外層一起拖垮或被外層吞掉邊界。
     * ⚠️ 不要共用 {@link #txTemplate}（那是 daily GC 的批次交易）。
     */
    private final TransactionTemplate publishTxTemplate;

    public ProductImportGcJob(ProductImportJobMapper jobMapper,
                              ProductImportItemMapper itemMapper,
                              ProductImportItemAssetMapper assetMapper,
                              ProductImportMediaCallbackInboxMapper inboxMapper,
                              ProductImportStorage csvStorage,
                              ObsRabbitPublisher publisher,
                              ImportJobProgressCalculator progressCalculator,
                              PlatformTransactionManager txManager) {
        this.jobMapper = jobMapper;
        this.itemMapper = itemMapper;
        this.assetMapper = assetMapper;
        this.inboxMapper = inboxMapper;
        this.csvStorage = csvStorage;
        this.publisher = publisher;
        this.progressCalculator = progressCalculator;
        this.txTemplate = new TransactionTemplate(txManager);
        this.claimTxTemplate = new TransactionTemplate(txManager);
        this.claimTxTemplate.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRES_NEW);
        this.publishTxTemplate = new TransactionTemplate(txManager);
        this.publishTxTemplate.setPropagationBehavior(TransactionDefinition.PROPAGATION_REQUIRES_NEW);
    }

    /**
     * R4-P1-2：在<b>獨立交易</b>內執行一次 lease claim 並立刻 commit（0 = 租約由他人持有）。
     * 呼叫端必須走這裡，直接呼叫 mapper 會讓 claim 落回外層批次交易。
     */
    private int claimInOwnTransaction(java.util.function.Supplier<Integer> claim) {
        Integer affected = claimTxTemplate.execute(status -> claim.get());
        return affected == null ? 0 : affected;
    }

    @Scheduled(cron = "${hawkr.product-import.gc.cron-callback-timeout:0 */5 * * * ?}")
    public void runCallbackTimeoutGc() {
        if (!gcEnabled) return;
        log.info("ProductImportGcJob.callbackTimeout.start", "callback timeout GC started");
        try {
            LocalDateTime cutoff = DateTimeUtils.nowUtc().minusMinutes(callbackTimeoutMinutes);
            List<ProductImportItemAssetEntity> stale = assetMapper.selectList(
                    new QueryWrapper<ProductImportItemAssetEntity>()
                            .eq("status", ImportAssetLocalStatus.DOWNLOADING.value())
                            .lt("updated_at", cutoff));
            int claimed = 0;
            for (ProductImportItemAssetEntity a : stale) {
                // issue #2294 L9：lease claim —— 搶到才做 DOWNLOADING→FAILED 與後續 publish。
                // 租約基準取 **claim 當下**（每一筆各自取）：取 tick 起點 ⇒ 全批共用同一個到期
                // 時間 ⇒ 不變式退化成「批次總耗時 < 租約」；取 claim 當下 ⇒ 降為「**單筆**最壞
                // 耗時 < 租約」。述詞用的 cutoff 維持 tick 起點（那個方向讓可搶佔集合更小）。
                if (assetMapper.claimStaleDownloadingAsset(
                        a.getId(),
                        ImportAssetLocalStatus.DOWNLOADING.value(),
                        cutoff,
                        DateTimeUtils.nowUtc()) == 0) {
                    log.info("ProductImportGcJob.callbackTimeout.claimSkip",
                            "租約由他人持有或狀態已變更，skip assetId=" + a.getId());
                    continue;
                }
                claimed++;
                a.setStatus(ImportAssetLocalStatus.FAILED.value());
                a.setFailureReason(ImportAssetFailureReason.CALLBACK_TIMEOUT.value());
                assetMapper.updateById(a);
                ProductImportItemEntity item = itemMapper.selectById(a.getImportItemId());
                if (item != null && ImportItemStatus.ASSET_DOWNLOADING.value().equals(item.getStatus())) {
                    Long pendingAssets = assetMapper.selectCount(
                            new QueryWrapper<ProductImportItemAssetEntity>()
                                    .eq("import_item_id", item.getId())
                                    .in("status", ImportAssetLocalStatus.PENDING.value(),
                                            ImportAssetLocalStatus.DOWNLOADING.value()));
                    if (pendingAssets == null || pendingAssets == 0) {
                        item.setStatus(item.getBlockingErrorCount() != null && item.getBlockingErrorCount() > 0
                                ? ImportItemStatus.PARSED.value()
                                : ImportItemStatus.READY.value());
                        itemMapper.updateById(item);
                    }
                }
                // publish 失敗會拋（見 checkAndPublishParsed）：per-item catch 讓本筆的
                // parsed_event_published_at CAS rollback、其餘 asset 繼續處理。
                try {
                    checkAndPublishParsed(a.getImportJobId());
                } catch (Exception e) {
                    log.error("ProductImportGcJob.callbackTimeout.publishParsedFailed",
                            "job.parsed 發佈失敗，parsed_event_published_at 已 rollback "
                                    + "（仍為 NULL，可由 AssetSettleHelper 路徑補發），jobId="
                                    + a.getImportJobId(), e);
                }
            }
            log.info("ProductImportGcJob.callbackTimeout.success",
                    "candidates=" + stale.size() + " processed=" + claimed);
        } catch (Exception e) {
            log.error("ProductImportGcJob.callbackTimeout.error", "callback timeout GC failed", e);
        }
    }

    @Scheduled(cron = "${hawkr.product-import.gc.cron-daily:0 0 2 * * ?}")
    public void runDailyGc() {
        if (!gcEnabled) return;
        log.info("ProductImportGcJob.daily.start", "daily GC started");
        try {
            // 40 §2.5 / §5.9.2:各 tier 用程式式 tx 包覆,單一 tier 失敗不影響下一 tier
            // (try/catch 內個別 tier 拋例外會冒到此處由外層 catch 接住,保留「daily GC 全跑」語意)
            runTierSafely("tierA", () -> txTemplate.executeWithoutResult(status -> tierAExpireJobsBody()));
            runTierSafely("tierB", () -> txTemplate.executeWithoutResult(status -> tierBOrphanAssetsBody()));
            runTierSafely("tierC", () -> txTemplate.executeWithoutResult(status -> tierCExpiredCleanupBody()));
            runTierSafely("tierD", () -> txTemplate.executeWithoutResult(status -> tierDCommittedItemsCleanupBody()));
            runTierSafely("tierE", () -> txTemplate.executeWithoutResult(status -> tierEInboxCleanupBody()));
            log.info("ProductImportGcJob.daily.success", "daily GC success");
        } catch (Exception e) {
            log.error("ProductImportGcJob.daily.error", "daily GC failed", e);
        }
    }

    /** tier 之間獨立 — 單 tier 例外不影響下一 tier(對應 §5.9.2 設計意圖)。 */
    private void runTierSafely(String tierName, Runnable tier) {
        try {
            tier.run();
        } catch (Exception e) {
            log.error("ProductImportGcJob.daily.tier.error", tierName + " failed", e);
        }
    }

    // 40 §2.5 / §5.9.2:tx body — 必須由 caller 包進 txTemplate。
    private void tierAExpireJobsBody() {
        LocalDateTime now = DateTimeUtils.nowUtc();
        List<ProductImportJobEntity> expired = jobMapper.findExpiredCandidates(now, 500);
        int claimed = 0;
        for (ProductImportJobEntity job : expired) {
            // issue #2294 L9：lease claim —— 搶到才做 EXPIRED 轉換 / item 批次 REMOVED / 發 MQ。
            // ⚠️ 不能拿 status 轉終態當閘門：那是永久旗標，claim 後 pod 崩潰 ⇒ expire_job 事件
            // 永久遺失且無告警（規範通則 0 第 4 步）。租約基準取 claim 當下（每筆各自取）。
            // ⚠️ claim 走 claimTxTemplate（REQUIRES_NEW）而非本 tier 的批次交易：
            // 見 claimTxTemplate 的 javadoc（尤其「rollback 撤得掉 claim、撤不掉已發的 MQ」）。
            LocalDateTime leaseUntil = DateTimeUtils.nowUtc().plusMinutes(TIER_A_LEASE_MINUTES);
            if (claimInOwnTransaction(
                    () -> jobMapper.claimExpiredJobForGc(job.getId(), now, leaseUntil)) == 0) {
                log.info("ProductImportGcJob.tierA.claimSkip",
                        "租約由他人持有或狀態已變更，skip jobId=" + job.getId());
                continue;
            }
            claimed++;
            job.setStatus(ImportJobStatus.EXPIRED.value());
            // R4-P2-2（2026-08-08）：把剛 claim 到的租約同步進實體，否則接下來的
            // updateById 會用**查詢當下的舊 gc_lease_until**（多半是 NULL 或已過期的值）
            // 把剛寫進 DB 的租約覆蓋掉——等於 claim 被自己的下一句抵銷。
            // 今天之所以沒炸，純粹是因為同一句把 status 推成 EXPIRED('9107')，而 9107 在
            // findExpiredCandidates / claimExpiredJobForGc 的 NOT IN 排除清單裡，所以該列
            // 無論如何都離開候選集合。那是**隱性耦合**：哪天候選述詞改了（例如要讓 EXPIRED
            // 的列也能被重掃），這一行就會當場變成「租約憑空消失」的 bug。
            job.setGcLeaseUntil(leaseUntil);
            jobMapper.updateById(job);
            itemMapper.update(null,
                    new UpdateWrapper<ProductImportItemEntity>()
                            .eq("import_job_id", job.getId())
                            .notIn("status", ImportItemStatus.COMMITTED.value(),
                                    ImportItemStatus.REMOVED.value())
                            .set("status", ImportItemStatus.REMOVED.value())
                            .set("removed_at", DateTimeUtils.nowUtc()));
            try {
                publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                        MqConstants.RK_PRODUCT_IMPORT_GC_EXPIRE_JOB,
                        MAPPER.writeValueAsBytes(new ExpireJobEvent(job.getId())),
                        ProductImportEventType.PRODUCT_IMPORT_GC_EXPIRE_JOB.value());
            } catch (Exception e) {
                log.error("ProductImportGcJob.tierA.publish",
                        "publish expire_job failed for jobId=" + job.getId(), e);
            }
        }
        log.info("ProductImportGcJob.tierA",
                "expired_candidates=" + expired.size() + " claimed=" + claimed);
    }

    // 40 §2.5 / §5.9.2:tx body
    private void tierBOrphanAssetsBody() {
        LocalDateTime cutoff = DateTimeUtils.nowUtc().minusHours(orphanGraceHours);
        List<ProductImportItemAssetEntity> orphans = assetMapper.findOrphanCandidates(cutoff, 500);
        for (ProductImportItemAssetEntity a : orphans) {
            try {
                publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                        MqConstants.RK_PRODUCT_IMPORT_GC_EXPIRE_JOB,
                        MAPPER.writeValueAsBytes(new ExpireJobEvent(a.getImportJobId())),
                        ProductImportEventType.PRODUCT_IMPORT_GC_EXPIRE_JOB.value());
            } catch (Exception e) {
                log.warn("ProductImportGcJob.tierB.publish",
                        "publish orphan expire failed, assetId=" + a.getId());
            }
            assetMapper.deleteById(a.getId());
        }
        log.info("ProductImportGcJob.tierB", "orphan_assets=" + orphans.size());
    }

    // 40 §2.5 / §5.9.2:tx body
    private void tierCExpiredCleanupBody() {
        LocalDateTime cutoff = DateTimeUtils.nowUtc().minusDays(expiredRetentionDays);
        List<ProductImportJobEntity> stale = jobMapper.selectList(
                new QueryWrapper<ProductImportJobEntity>()
                        .eq("status", ImportJobStatus.EXPIRED.value())
                        .lt("updated_at", cutoff)
                        .last("LIMIT 200"));
        for (ProductImportJobEntity job : stale) {
            if (job.getSourceObjectKey() != null) csvStorage.deleteObject(job.getSourceObjectKey());
            jobMapper.deleteById(job.getId());
        }
        log.info("ProductImportGcJob.tierC", "expired_jobs_deleted=" + stale.size());
    }

    // 40 §2.5 / §5.9.2:tx body
    private void tierDCommittedItemsCleanupBody() {
        LocalDateTime cutoff = DateTimeUtils.nowUtc().minusDays(committedRetentionDays);
        List<ProductImportItemEntity> committed = itemMapper.selectList(
                new QueryWrapper<ProductImportItemEntity>()
                        .eq("status", ImportItemStatus.COMMITTED.value())
                        .lt("committed_at", cutoff)
                        .last("LIMIT 500"));
        for (ProductImportItemEntity it : committed) {
            itemMapper.deleteById(it.getId());
        }
        log.info("ProductImportGcJob.tierD", "committed_items_deleted=" + committed.size());
    }

    // 40 §2.5 / §5.9.2:tx body
    private void tierEInboxCleanupBody() {
        LocalDateTime cutoff = DateTimeUtils.nowUtc().minusDays(inboxRetentionDays);
        int n = inboxMapper.delete(new QueryWrapper<ProductImportMediaCallbackInboxEntity>()
                .lt("processed_at", cutoff));
        log.info("ProductImportGcJob.tierE", "inbox_deleted=" + n);
    }

    /**
     * 一次性 publish gate：CAS {@code parsed_event_published_at IS NULL → NOW()} 之後才 publish。
     *
     * <p>{@code parsed_event_published_at} 是<b>永久旗標</b>（候選述詞 {@code IS NULL}，寫進 NOW()
     * 之後永遠回不來）。原本 publish 失敗只記 error log ⇒ 旗標已提交、事件沒送出 ⇒
     * {@code job.parsed} <b>永久不發</b>，而 docs/開發規範/後端服務責任邊界規範.md 禁止寫 reconciliation job ⇒ 無救濟。
     *
     * <p>⇒ 改走**出路 4**：CAS ＋ publish 包進同一個短 tx，publish 失敗即重拋讓 tx rollback
     * （比照同 repo 的正確版本 {@code AssetSettleHelper.onAssetSettled}）。rollback 之後
     * {@code parsed_event_published_at} 仍為 NULL，{@code AssetSettleHelper} 路徑仍可補發。
     *
     * <p>刻意只把「CAS ＋ publish」放進 tx（規範 §四 出路 4 的形狀：一列、很短），
     * 不把上游的 claim 與 asset/item 狀態推進一起包進來——那會讓 claim 的 row lock 橫跨
     * 一次 AMQP publish，把「另一個 pod 的 claim」變成阻塞等待。
     *
     * @throws IllegalStateException publish 失敗（呼叫端負責 catch 並繼續下一筆）
     */
    private void checkAndPublishParsed(Long jobId) {
        ProductImportJobEntity job = jobMapper.selectById(jobId);
        if (job == null) return;
        if (!JOB_ACTIVE_FOR_PARSED_EVENT.contains(job.getStatus())) return;
        if (job.getParsedEventPublishedAt() != null) return;

        ImportJobProgressCalculator.Counters counters = progressCalculator.counters(jobId);
        boolean allSettled = counters.assetsTotal() > 0
                && counters.assetsPending() == 0 && counters.assetsDownloading() == 0;
        if (!allSettled) return;

        publishTxTemplate.executeWithoutResult(status -> claimAndPublishParsed(jobId, job, counters));
    }

    /** 出路 4 tx body — 必須由 caller 包進 publishTxTemplate。禁止標 @Transactional（self-invocation）。 */
    private void claimAndPublishParsed(Long jobId, ProductImportJobEntity job,
                                       ImportJobProgressCalculator.Counters counters) {
        int updated = jobMapper.update(null,
                new UpdateWrapper<ProductImportJobEntity>()
                        .eq("id", jobId)
                        .isNull("parsed_event_published_at")
                        .set("parsed_event_published_at", DateTimeUtils.nowUtc()));
        if (updated == 0) return;

        ImportJobParsedEvent ev = new ImportJobParsedEvent(
                UUID.randomUUID().toString(),
                jobId, job.getStoreId(),
                Math.toIntExact(counters.itemsTotal()),
                Math.toIntExact(counters.itemsReady()),
                0,
                Math.toIntExact(counters.assetsFailed()),
                DateTimeUtils.nowUtc().toString()
        );
        try {
            publisher.publish(MqConstants.EXCHANGE_PRODUCT_IMPORT,
                    MqConstants.RK_PRODUCT_IMPORT_JOB_PARSED,
                    MAPPER.writeValueAsBytes(ev),
                    ProductImportEventType.PRODUCT_IMPORT_JOB_PARSED.value());
        } catch (Exception e) {
            // 重拋 → 短 tx rollback → CAS 撤銷 → 事件仍可被補發（比照 AssetSettleHelper）
            throw new IllegalStateException("publish job.parsed failed jobId=" + jobId, e);
        }
    }
}

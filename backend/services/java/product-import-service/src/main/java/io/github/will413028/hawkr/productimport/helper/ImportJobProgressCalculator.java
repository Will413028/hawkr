package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportItemStatus;
import io.github.will413028.hawkr.productimport.entity.projection.AssetCountByStatusProjection;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemAssetMapper;
import io.github.will413028.hawkr.productimport.mapper.ProductImportItemMapper;
import org.springframework.stereotype.Component;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * 計算 ImportJob 的進度數據（40 號設計書 §5 / §7 / §3.10）。
 *
 * <p>產生兩種結構：
 * <ul>
 *   <li>{@link Counters} — 對應 proto {@code ImportJobCounters}（item / asset 維度計數）</li>
 *   <li>{@link Progress} — 對應 proto {@code ImportJobProgress}（百分比與 completed flag）</li>
 * </ul>
 *
 * <p>實作策略：兩次聚合 SQL（item by status + asset by status）後純記憶體換算，
 * 避免在 SQL 內塞百分比計算（差別不大，但易閱讀）。
 */
@Component
public class ImportJobProgressCalculator {

    private final ProductImportItemMapper itemMapper;
    private final ProductImportItemAssetMapper assetMapper;

    public ImportJobProgressCalculator(ProductImportItemMapper itemMapper,
                                       ProductImportItemAssetMapper assetMapper) {
        this.itemMapper = itemMapper;
        this.assetMapper = assetMapper;
    }

    /**
     * 計算 counters。
     *
     * @param importJobId 任務 id
     * @return Counters 結構
     */
    public Counters counters(Long importJobId) {
        Map<String, Long> itemByStatus = new HashMap<>();
        List<Map<String, Object>> rows = itemMapper.countByStatusGroup(importJobId);
        for (Map<String, Object> r : rows) {
            String status = (String) r.get("status");
            Long cnt = ((Number) r.get("cnt")).longValue();
            itemByStatus.put(status, cnt);
        }

        Map<String, Long> assetByStatus = new HashMap<>();
        List<AssetCountByStatusProjection> assetRows = assetMapper.countByStatusGroup(importJobId);
        for (AssetCountByStatusProjection r : assetRows) {
            assetByStatus.put(r.status(), r.cnt());
        }

        // 40 號設計書 §4.2.3 / §5.2 / §1.5：需修正（needs_fix）= READY + COMMIT_FAILED 且
        // blocking_error_count > 0 的 item 數。透過 ProductImportItemMapper 對齊 needs_fix 預覽列表的
        // 統一定義，避免「item.status=READY but blocking>0」被誤算成「已就緒」拖高 parsePercentage。
        long itemsNeedsFix = nullSafe(itemMapper.countNeedsFix(importJobId));

        return new Counters(
                sumAll(itemByStatus),
                itemByStatus.getOrDefault(ImportItemStatus.PARSED.value(), 0L),
                itemByStatus.getOrDefault(ImportItemStatus.ASSET_DOWNLOADING.value(), 0L),
                itemByStatus.getOrDefault(ImportItemStatus.READY.value(), 0L),
                itemsNeedsFix,
                itemByStatus.getOrDefault(ImportItemStatus.REMOVED.value(), 0L),
                itemByStatus.getOrDefault(ImportItemStatus.COMMITTING.value(), 0L),
                itemByStatus.getOrDefault(ImportItemStatus.COMMITTED.value(), 0L),
                itemByStatus.getOrDefault(ImportItemStatus.COMMIT_FAILED.value(), 0L),
                sumAll(assetByStatus),
                assetByStatus.getOrDefault(ImportAssetLocalStatus.PENDING.value(), 0L),
                assetByStatus.getOrDefault(ImportAssetLocalStatus.DOWNLOADING.value(), 0L),
                assetByStatus.getOrDefault(ImportAssetLocalStatus.UPLOADED.value(), 0L),
                assetByStatus.getOrDefault(ImportAssetLocalStatus.FAILED.value(), 0L),
                assetByStatus.getOrDefault(ImportAssetLocalStatus.LINKED.value(), 0L)
        );
    }

    /**
     * 計算進度百分比 + 完成 flag。
     */
    public Progress progress(Counters c) {
        long totalItems = c.itemsTotal();
        long totalAssets = c.assetsTotal();

        // parsePct：CSV row 級「已完成解析」比例。
        // 「PARSED」表示尚未派下載；其餘終態（READY / COMMITTING / COMMITTED / COMMIT_FAILED / REMOVED）
        // 都算解析路徑已過。注意 needs_fix 已包含於 READY / COMMIT_FAILED，不重複計入。
        int parsePct = totalItems == 0 ? 0
                : (int) Math.min(100, (c.itemsParsed() + c.itemsReady() + c.itemsCommitting()
                        + c.itemsCommitted() + c.itemsCommitFailed() + c.itemsRemoved()) * 100 / totalItems);

        int assetPct = totalAssets == 0 ? 100
                : (int) Math.min(100, (c.assetsUploaded() + c.assetsLinked() + c.assetsFailed()) * 100 / totalAssets);

        int commitPct = totalItems == 0 ? 0
                : (int) Math.min(100, (c.itemsCommitted() + c.itemsCommitFailed()) * 100 / totalItems);

        boolean parseDone = parsePct >= 100;
        boolean assetDone = assetPct >= 100;
        boolean commitDone = totalItems > 0 && (c.itemsCommitted() + c.itemsCommitFailed()) == totalItems;

        // overall 只算 parse + asset 兩段（賣家心智模型：「商品就緒 = 100%」）。
        // commit 是賣家主動觸發的後續動作，由「送出建立商品」按鈕語境呈現，不拖累 overall。
        int overall = (parsePct + assetPct) / 2;

        return new Progress(overall, parsePct, assetPct, commitPct, parseDone, assetDone, commitDone);
    }

    private static long sumAll(Map<String, Long> m) {
        long s = 0L;
        for (Long v : m.values()) s += v;
        return s;
    }

    private static long nullSafe(Long v) { return v == null ? 0L : v; }

    /** 對應 proto ImportJobCounters。 */
    public record Counters(long itemsTotal,
                           long itemsParsed,
                           long itemsAssetDownloading,
                           long itemsReady,
                           long itemsNeedsFix,
                           long itemsRemoved,
                           long itemsCommitting,
                           long itemsCommitted,
                           long itemsCommitFailed,
                           long assetsTotal,
                           long assetsPending,
                           long assetsDownloading,
                           long assetsUploaded,
                           long assetsFailed,
                           long assetsLinked) {}

    /** 對應 proto ImportJobProgress。 */
    public record Progress(int overallPercentage,
                           int parsePercentage,
                           int assetDownloadPercentage,
                           int commitPercentage,
                           boolean parseCompleted,
                           boolean assetDownloadCompleted,
                           boolean commitCompleted) {}
}

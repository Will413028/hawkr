package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.product.domain.hashtag.HashtagInputValidator;
import io.github.will413028.hawkr.product.domain.hashtag.NormalizedHashtag;
import io.github.will413028.hawkr.product.entity.HashtagEntity;
import io.github.will413028.hawkr.product.entity.ProductHashtagEntity;
import io.github.will413028.hawkr.product.repository.HashtagRepository;
import io.github.will413028.hawkr.product.repository.ProductHashtagRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

/**
 * Hashtag 業務 Service（33-商品 Hashtag 模組詳細設計書 §3、§4、§5.1）。
 *
 * <p>本 Service 提供：
 * <ul>
 *   <li>{@link #upsertHashtags(List)} — 規範化 + UPSERT hashtag 主表，回傳輸入序對應的 hashtag id 與 display</li>
 *   <li>{@link #applyProductHashtags(Long, List)} — 對 product 寫入 product_hashtag（含 diff）</li>
 *   <li>{@link #findHashtagsByProductId(Long)} — 取詳情頁 hashtag list（含讀取時計算的公開商品數）</li>
 *   <li>{@link #searchHashtags(String, io.github.will413028.hawkr.product.service.HashtagService.SearchMode, int)} — auto-complete</li>
 * </ul>
 *
 * <p>hashtag 的公開商品數不儲存，讀取時依商品狀態與店家可曝光計算（{@link HashtagRepository#findWithPublicCountByIds}）。
 */
@Service
public class HashtagService {

    private static final ObsLogger log = ObsLogger.forClass(HashtagService.class);

    /** auto-complete 預設 limit。 */
    public static final int DEFAULT_SUGGEST_LIMIT = 10;
    /** auto-complete max limit。 */
    public static final int MAX_SUGGEST_LIMIT = 50;

    private final HashtagRepository hashtagRepository;
    private final ProductHashtagRepository productHashtagRepository;

    public HashtagService(HashtagRepository hashtagRepository, ProductHashtagRepository productHashtagRepository) {
        this.hashtagRepository = hashtagRepository;
        this.productHashtagRepository = productHashtagRepository;
    }

    public enum SearchMode { PREFIX, CONTAIN }

    /** Hashtag 集合 diff 結果（供主流程後續 reflow 與 counter delta 使用）。 */
    public record HashtagApplyResult(
            /** 規範化後最終綁定的 hashtag（依 position 升序）。 */
            List<UpsertedHashtag> finalBindings,
            /** 本次新增的 hashtag id（按 id ASC）。 */
            List<Long> addedHashtagIds,
            /** 本次移除的 hashtag id（按 id ASC）。 */
            List<Long> removedHashtagIds
    ) {}

    /** UPSERT 後的 hashtag 條目（id + canonical + display）。 */
    public record UpsertedHashtag(Long id, String canonical, String display) {}

    /**
     * 規範化 raw 輸入 + UPSERT hashtag 主表，回傳依輸入序的 UpsertedHashtag list。
     * 不寫 product_hashtag。
     */
    @Transactional(propagation = Propagation.MANDATORY)
    public List<UpsertedHashtag> upsertHashtags(List<String> rawHashtags) {
        List<NormalizedHashtag> normalized = HashtagInputValidator.requireHashtagSet(rawHashtags);
        if (normalized.isEmpty()) {
            return List.of();
        }
        List<UpsertedHashtag> out = new ArrayList<>(normalized.size());
        for (NormalizedHashtag n : normalized) {
            Long id = hashtagRepository.upsertByName(TsidUtil.generate(), n.canonical());
            out.add(new UpsertedHashtag(id, n.canonical(), n.display()));
        }
        return out;
    }

    /**
     * 套用 product 與 hashtag 的綁定關係：
     * <ul>
     *   <li>計算與當前綁定的 diff，刪除 removed、新增 added、更新 kept 的 display/position</li>
     * </ul>
     */
    @Transactional(propagation = Propagation.MANDATORY)
    public HashtagApplyResult applyProductHashtags(
            Long productId,
            List<UpsertedHashtag> targets) {
        // 當前綁定（依 position 升序）
        List<ProductHashtagEntity> currentRows = productHashtagRepository.findByProductId(productId);
        Map<Long, ProductHashtagEntity> currentMap = currentRows.stream()
                .collect(Collectors.toMap(ProductHashtagEntity::getHashtagId, r -> r));

        // 目標：依輸入序產生 hashtag_id → (display, position)
        Map<Long, UpsertedHashtag> targetMap = new LinkedHashMap<>();
        for (int i = 0; i < targets.size(); i++) {
            targetMap.putIfAbsent(targets.get(i).id(), targets.get(i));
        }

        Set<Long> currentIds = currentMap.keySet();
        Set<Long> targetIds = targetMap.keySet();

        Set<Long> addedSet = new HashSet<>(targetIds);
        addedSet.removeAll(currentIds);
        Set<Long> removedSet = new HashSet<>(currentIds);
        removedSet.removeAll(targetIds);

        // 1. 移除舊綁定
        if (!removedSet.isEmpty()) {
            List<Long> removedSorted = removedSet.stream().sorted().toList();
            productHashtagRepository.deleteByProductAndHashtags(productId, removedSorted);
        }

        // 2. 新增 + kept 的 position/display 更新（採全清新增策略以保證 position 與 display 一致）
        // 為簡化邏輯：對 kept 採 (delete + insert) 也可，但會增加 counter 維護成本；
        // 此處改採 row-by-row UPDATE 對 kept；INSERT 對 added。
        short pos = 0;
        List<ProductHashtagEntity> toInsert = new ArrayList<>();
        for (Map.Entry<Long, UpsertedHashtag> e : targetMap.entrySet()) {
            Long hid = e.getKey();
            UpsertedHashtag u = e.getValue();
            ProductHashtagEntity existing = currentMap.get(hid);
            if (existing == null) {
                ProductHashtagEntity row = new ProductHashtagEntity();
                row.setProductId(productId);
                row.setHashtagId(hid);
                row.setDisplayName(u.display());
                row.setPosition(pos);
                toInsert.add(row);
            } else if (!u.display().equals(existing.getDisplayName()) || existing.getPosition() == null
                    || existing.getPosition() != pos) {
                // 複合主鍵 (product_id, hashtag_id) 不能 updateById，用自訂 SQL
                productHashtagRepository.updateBinding(productId, hid, u.display(), pos);
            }
            pos++;
        }
        if (!toInsert.isEmpty()) {
            productHashtagRepository.batchInsert(toInsert);
        }

        List<Long> addedSortedFinal = addedSet.stream().sorted().toList();
        List<Long> removedSortedFinal = removedSet.stream().sorted().toList();
        return new HashtagApplyResult(new ArrayList<>(targetMap.values()), addedSortedFinal, removedSortedFinal);
    }

    /**
     * 取某 product 的 hashtag list（依 position 升序）併入公開商品數（讀取時計算）。
     */
    public List<HashtagDisplayItem> findHashtagsByProductId(Long productId) {
        List<ProductHashtagEntity> rows = productHashtagRepository.findByProductId(productId);
        if (rows.isEmpty()) return List.of();
        List<Long> ids = rows.stream().map(ProductHashtagEntity::getHashtagId).toList();
        Map<Long, HashtagRepository.HashtagWithCount> idToHashtag = hashtagRepository.findWithPublicCountByIds(ids).stream()
                .collect(Collectors.toMap(HashtagRepository.HashtagWithCount::getId, h -> h));
        List<HashtagDisplayItem> out = new ArrayList<>(rows.size());
        for (ProductHashtagEntity r : rows) {
            HashtagRepository.HashtagWithCount h = idToHashtag.get(r.getHashtagId());
            if (h == null) continue;
            out.add(new HashtagDisplayItem(h.getName(), r.getDisplayName(),
                    String.valueOf(h.getProductCount() == null ? 0 : h.getProductCount())));
        }
        return out;
    }

    /**
     * 詳情/列表用 hashtag 條目（name + display + 公開商品數）。
     * <p>{@code productCount} 為字串型別，遵守 docs/開發規範/Java 微服務開發規範.md §2.1（對外數字一律 string）。
     */
    public record HashtagDisplayItem(String canonicalName, String displayName, String productCount) {}

    /**
     * 批次取多個 product 的 hashtag list（給商品列表填充用），回傳 productId → list。
     */
    public Map<Long, List<HashtagDisplayItem>> findHashtagsByProductIds(List<Long> productIds) {
        if (productIds == null || productIds.isEmpty()) return Map.of();
        Map<Long, List<HashtagDisplayItem>> out = new HashMap<>();
        for (Long pid : productIds) {
            out.put(pid, findHashtagsByProductId(pid));
        }
        return out;
    }

    /**
     * Auto-complete 查詢（§5.12）。
     *
     * @param prefix 用戶輸入（任意大小寫，server normalize）
     * @param mode   PREFIX / CONTAIN
     * @param limit  上限（< 1 視為 default 10，> 50 視為 50）
     * @return 候選列表（依熱門度排序）
     */
    public List<HashtagDisplayItem> searchHashtags(String prefix, SearchMode mode, int limit) {
        if (prefix == null) return List.of();
        NormalizedHashtag norm = io.github.will413028.hawkr.product.domain.hashtag.HashtagNormalizer.normalize(prefix);
        if (norm == null) return List.of();
        int effectiveLimit = limit;
        if (effectiveLimit < 1) effectiveLimit = DEFAULT_SUGGEST_LIMIT;
        if (effectiveLimit > MAX_SUGGEST_LIMIT) effectiveLimit = MAX_SUGGEST_LIMIT;
        List<HashtagRepository.HashtagSuggestion> rows = (mode == SearchMode.CONTAIN)
                ? hashtagRepository.searchByContain(norm.canonical(), effectiveLimit)
                : hashtagRepository.searchByPrefix(norm.canonical(), effectiveLimit);
        List<HashtagDisplayItem> out = new ArrayList<>(rows.size());
        for (HashtagRepository.HashtagSuggestion r : rows) {
            out.add(new HashtagDisplayItem(r.getName(), "",
                    String.valueOf(r.getProductCount() == null ? 0 : r.getProductCount())));
        }
        return out;
    }

    /**
     * 取多個 raw hashtag 字串對應的 canonical name list（用於 GetProductList 篩選；
     * 接受任意大小寫輸入，輸出 canonical 排序後 list）。
     */
    public List<String> normalizeForFilter(List<String> rawHashtags) {
        if (rawHashtags == null || rawHashtags.isEmpty()) return Collections.emptyList();
        Set<String> dedup = new HashSet<>();
        for (String raw : rawHashtags) {
            NormalizedHashtag n = io.github.will413028.hawkr.product.domain.hashtag.HashtagNormalizer.normalize(raw);
            if (n != null) {
                dedup.add(n.canonical());
            }
        }
        return new ArrayList<>(dedup);
    }

}

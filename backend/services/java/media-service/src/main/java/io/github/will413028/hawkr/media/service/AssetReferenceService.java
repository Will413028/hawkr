package io.github.will413028.hawkr.media.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.AssetReferenceType;
import io.github.will413028.hawkr.media.entity.AssetReferenceEntity;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import io.github.will413028.hawkr.media.exception.MediaServiceException;
import io.github.will413028.hawkr.media.repository.AssetReferenceRepository;
import io.github.will413028.hawkr.media.repository.MediaAssetRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;

/**
 * 資產引用服務
 * <p>
 * 職責：管理 LinkImages / UnlinkImages
 */
@Service
@RequiredArgsConstructor
public class AssetReferenceService {

    private static final ObsLogger log = ObsLogger.forClass(AssetReferenceService.class);

    private final AssetReferenceRepository assetReferenceRepository;
    private final MediaAssetRepository mediaAssetRepository;

    /**
     * 建立資產引用
     * <p>
     * 使用 ON CONFLICT DO NOTHING 實現冪等
     * 多次呼叫視為成功
     *
     * @param refType  引用類型（如 PRODUCT, SELLER_LOGO）
     * @param refId    引用 ID（如 product_id）
     * @param assetIds 資產 ID 列表
     * @return 成功建立的數量（含已存在）
     */
    @Transactional
    public int linkImages(String refType, String refId, List<String> assetIds) {
        log.info("start", "開始建立引用, ref_type=" + refType + ", ref_id=" + refId);

        // 驗證參數
        validateLinkParams(refType, refId, assetIds);

        // 轉換 asset ID。去重是必要的：呼叫端拿到的可能是同一個 asset 被回傳多次
        // （例如同一個 staged_token 在一次 FinalizeImages 裡出現兩次，重入語意會讓兩個
        // 位置都回同一個 assetId）。不去重的話 validateAssetsExist 會拿
        // selectByIds（SQL IN 天然去重）的 1 筆去比對 2 筆輸入而誤判「部分資產不存在」，
        // 回一個 missingIds 為空的 404 —— dev 實測 `部分資產不存在: `（後面什麼都沒有）。
        List<Long> idList = parseAssetIds(assetIds).stream().distinct().toList();

        // 驗證資產存在
        validateAssetsExist(idList);

        // 建立引用（批次插入，忽略衝突）
        List<AssetReferenceEntity> references = idList.stream()
            .map(assetId -> {
                AssetReferenceEntity ref = new AssetReferenceEntity();
                ref.setAssetId(assetId);
                ref.setRefType(refType);
                ref.setRefId(refId);
                return ref;
            })
            .collect(Collectors.toList());

        int linkedCount = 0;
        for (AssetReferenceEntity ref : references) {
            assetReferenceRepository.insertIgnoreConflict(
                ref.getAssetId(),
                ref.getRefType(),
                ref.getRefId()
            );
            linkedCount++;
        }

        log.info("link.success", "引用建立完成",
            "linked_count", String.valueOf(linkedCount));

        return linkedCount;
    }

    /**
     * 解除資產引用
     * <p>
     * 冪等操作：多次呼叫不會報錯
     *
     * @param refType  引用類型
     * @param refId    引用 ID
     * @param assetIds 要解除的資產 ID 列表
     * @return 成功解除的數量
     */
    @Transactional
    public int unlinkImages(String refType, String refId, List<String> assetIds) {
        log.info("start", "開始解除引用, ref_type=" + refType + ", ref_id=" + refId);

        // 驗證參數
        validateUnlinkParams(refType, refId, assetIds);

        // 轉換 asset ID
        List<Long> idList = parseAssetIds(assetIds);

        // 批次刪除引用
        int unlinkedCount = assetReferenceRepository.batchDeleteReferences(
            refType,
            refId,
            idList
        );

        log.info("unlink.success", "引用解除完成",
            "unlinked_count", String.valueOf(unlinkedCount));

        return unlinkedCount;
    }

    /**
     * 驗證 Link 參數
     */
    private void validateLinkParams(String refType, String refId, List<String> assetIds) {
        if (refType == null || refType.isBlank()) {
            throw MediaServiceException.invalidArgument("ref_type 不可為空");
        }
        if (refId == null || refId.isBlank()) {
            throw MediaServiceException.invalidArgument("ref_id 不可為空");
        }
        if (assetIds == null || assetIds.isEmpty()) {
            throw MediaServiceException.invalidArgument("asset_ids 不可為空");
        }

        if (!AssetReferenceType.isValidValue(refType)) {
            throw MediaServiceException.invalidArgument("無效的 ref_type: " + refType);
        }
    }

    /**
     * 驗證 Unlink 參數
     */
    private void validateUnlinkParams(String refType, String refId, List<String> assetIds) {
        if (refType == null || refType.isBlank()) {
            throw MediaServiceException.invalidArgument("ref_type 不可為空");
        }
        if (refId == null || refId.isBlank()) {
            throw MediaServiceException.invalidArgument("ref_id 不可為空");
        }
        if (assetIds == null || assetIds.isEmpty()) {
            throw MediaServiceException.invalidArgument("asset_ids 不可為空");
        }
        if (!AssetReferenceType.isValidValue(refType)) {
            throw MediaServiceException.invalidArgument("無效的 ref_type: " + refType);
        }
    }

    /**
     * 解析 Asset ID 字串列表為 Long 列表
     */
    private List<Long> parseAssetIds(List<String> assetIds) {
        List<Long> result = new ArrayList<>();
        for (String idStr : assetIds) {
            try {
                result.add(Long.parseLong(idStr));
            } catch (NumberFormatException e) {
                throw MediaServiceException.invalidArgument(
                    "無效的 asset_id 格式: " + idStr);
            }
        }
        return result;
    }

    /**
     * 驗證資產存在（設計書 3.2.4 建議啟用 NOT_FOUND）
     */
    private void validateAssetsExist(List<Long> assetIds) {
        List<MediaAssetEntity> existingAssets = mediaAssetRepository.selectByIds(assetIds);
        if (existingAssets.size() != assetIds.size()) {
            List<Long> existingIds = existingAssets.stream()
                .map(MediaAssetEntity::getId)
                .collect(Collectors.toList());

            List<String> missingIds = assetIds.stream()
                .filter(id -> !existingIds.contains(id))
                .map(Object::toString)
                .collect(Collectors.toList());

            throw MediaServiceException.notFound(
                "部分資產不存在: " + String.join(", ", missingIds));
        }
    }

    /**
     * 釋放引用：依 (ref_type, ref_id) 批次移除關聯（不需 asset_id）。
     * <p>
     * 冪等操作。兩種語意：
     * <ul>
     *   <li>{@code keepAssetIds} 空：釋放全部引用（帳號 / 實體刪除），資產變孤兒後由 GC 刪除物件。</li>
     *   <li>{@code keepAssetIds} 非空：換圖收斂——同一交易內刪舊留新。呼叫端的正確順序是
     *       <b>先 Link 新資產 → 寫業務列 → 再帶 keepAssetIds 釋放</b>，任一步失敗都只會殘留
     *       可回收的引用，不會產生「業務列指著、卻沒有引用」而被 GC 實體刪除的資產。</li>
     * </ul>
     *
     * @param refType      引用類型
     * @param refIds       要釋放的引用 ID 列表
     * @param keepAssetIds 保留名單（可為 null / 空）
     * @return 成功釋放的數量
     */
    @Transactional
    public int releaseReferences(String refType, List<String> refIds, List<String> keepAssetIds) {
        log.info("release.start", "開始釋放引用, ref_type=" + refType
            + ", ref_ids_count=" + (refIds == null ? 0 : refIds.size())
            + ", keep_asset_ids_count=" + (keepAssetIds == null ? 0 : keepAssetIds.size()));

        if (refType == null || refType.isBlank()) {
            throw MediaServiceException.invalidArgument("ref_type 不可為空");
        }
        if (refIds == null || refIds.isEmpty()) {
            throw MediaServiceException.invalidArgument("ref_ids 不可為空");
        }
        if (!AssetReferenceType.isValidValue(refType)) {
            throw MediaServiceException.invalidArgument("無效的 ref_type: " + refType);
        }

        List<Long> keepIds = (keepAssetIds == null || keepAssetIds.isEmpty())
            ? List.of()
            : parseAssetIds(keepAssetIds);

        int releasedCount = assetReferenceRepository.deleteByRefTypeAndRefIds(refType, refIds, keepIds);

        log.info("release.success", "引用釋放完成", "released_count", String.valueOf(releasedCount));
        return releasedCount;
    }

    /**
     * 查詢資產引用數量
     */
    public int getRefCount(Long assetId) {
        return assetReferenceRepository.countByAssetId(assetId);
    }

    /**
     * 查詢業務的所有引用
     */
    public List<AssetReferenceEntity> getRefsByBusiness(String refType, String refId) {
        return assetReferenceRepository.selectByRef(refType, refId);
    }

    /**
     * 資產是否被指定業務引用。
     *
     * <p>{@code ServerReadSecureAsset} 的授權述詞——呼叫端說得出 object_key 還不夠，
     * 還要說得出這份資產綁在哪一筆業務上。
     */
    public boolean hasReference(Long assetId, String refType, String refId) {
        return assetReferenceRepository.existsReference(assetId, refType, refId);
    }
}

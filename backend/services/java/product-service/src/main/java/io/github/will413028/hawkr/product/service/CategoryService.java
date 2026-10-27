package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.CommonStatus;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.dto.response.CategoryHierarchyDTO;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import io.github.will413028.hawkr.product.exception.ProductBusinessException;
import io.github.will413028.hawkr.product.repository.CategoryRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;
import java.util.*;
import java.util.stream.Collectors;

/**
 * 分類業務服務（2026-04-25 政策大改造後）
 *
 * <p>政策摘要：
 * <ul>
 *   <li>取消審核機制，新增即生效（status 預設 9004 ACTIVE）</li>
 *   <li>同名同父自動套用既有 ACTIVE 分類（透明回傳）</li>
 *   <li>撞到 INACTIVE（軟刪除）同名同父：拋 CATEGORY_NAME_RESERVED</li>
 *   <li>軟刪除：status=9003 + deleted_at；前置驗證底下無商品+無 ACTIVE 子分類</li>
 *   <li>還原：父分類存活 + 同名同父無 ACTIVE 才允許</li>
 * </ul>
 */
@Service
public class CategoryService {

    private static final ObsLogger log = ObsLogger.forClass(CategoryService.class);

    private final CategoryRepository categoryRepository;
    private final CategoryCacheService cacheService;
    private final ProductCascadeService productCascadeService;

    private final ProductRevalidationService productRevalidationService;

    public CategoryService(CategoryRepository categoryRepository,
                           CategoryCacheService cacheService,
                           ProductCascadeService productCascadeService,
                           ProductRevalidationService productRevalidationService) {
        this.categoryRepository = categoryRepository;
        this.cacheService = cacheService;
        this.productCascadeService = productCascadeService;
        this.productRevalidationService = productRevalidationService;
    }

    // ==================== 讀取 ====================

    /**
     * 取得分類樹（三層結構）
     *
     * @param activeOnly true=僅 ACTIVE；false=含 INACTIVE 軟刪除（admin 用）
     * @return 排序後的分類樹（根分類列表，每節點含 children）
     */
    public List<CategoryEntity> getCategoryTree(boolean activeOnly) {
        List<CategoryEntity> all = activeOnly
                ? cacheService.findByStatus(CommonStatus.ACTIVE.value())
                : cacheService.findAll();

        if (all.isEmpty()) {
            return new ArrayList<>();
        }

        Map<Long, CategoryEntity> idMap = all.stream()
                .collect(Collectors.toMap(CategoryEntity::getId, c -> c));

        for (CategoryEntity c : all) {
            if (c.getParentId() != null) {
                CategoryEntity parent = idMap.get(c.getParentId());
                if (parent != null) {
                    c.setParentName(parent.getName());
                }
            }
            c.setChildren(new ArrayList<>());
        }

        List<CategoryEntity> roots = new ArrayList<>();
        for (CategoryEntity c : all) {
            if (c.getParentId() == null) {
                roots.add(c);
            } else {
                CategoryEntity parent = idMap.get(c.getParentId());
                if (parent != null) {
                    parent.getChildren().add(c);
                }
            }
        }
        sortCategoryTree(roots);
        return roots;
    }

    /**
     * 根據 parent_id 取得 ACTIVE 子分類
     */
    public List<CategoryEntity> getChildCategories(Long parentId) {
        if (parentId == null || parentId < 0) {
            return new ArrayList<>();
        }
        if (parentId == 0) {
            return cacheService.findTopCategories().stream()
                    .filter(c -> CommonStatus.ACTIVE.value().equals(c.getStatus()))
                    .collect(Collectors.toList());
        }

        Optional<CategoryEntity> parentOpt = cacheService.findById(parentId);
        if (parentOpt.isEmpty()) {
            return new ArrayList<>();
        }
        CategoryEntity parent = parentOpt.get();

        List<CategoryEntity> children = cacheService.findByParentId(parentId).stream()
                .filter(c -> CommonStatus.ACTIVE.value().equals(c.getStatus()))
                .collect(Collectors.toList());
        for (CategoryEntity child : children) {
            child.setParentName(parent.getName());
        }
        return children;
    }

    // ==================== 寫入：新增（含自動套用） ====================

    /**
     * 新增分類，遵守自動套用規則：
     * <ol>
     *   <li>同 parentId + name 已存在 ACTIVE → 直接回該 ID（透明）</li>
     *   <li>同 parentId + name 已存在 INACTIVE → 拋 CATEGORY_NAME_RESERVED</li>
     *   <li>不存在 → INSERT (status=ACTIVE)</li>
     * </ol>
     *
     * @param parentId 父分類 ID（null=新增 L1）
     * @param name 分類名稱
     * @param createdBy 建立者 ID（從 JWT，可為 null = system）
     */
    @Transactional
    public CategoryEntity createCategory(Long parentId, String name, Long createdBy) {
        if (name == null || name.trim().isEmpty()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "name 不可為空");
        }
        String trimmedName = name.trim();

        Long resolvedParentId = (parentId != null && parentId > 0) ? parentId : null;
        if (resolvedParentId != null) {
            CategoryEntity parent = cacheService.findById(resolvedParentId)
                    .orElseThrow(() -> new ProductBusinessException(
                            GrpcResponseCode.CATEGORY_NOT_FOUND,
                            "父分類不存在，parent_id=" + resolvedParentId));
            if (CommonStatus.INACTIVE.value().equals(parent.getStatus())) {
                throw new ProductBusinessException(
                        GrpcResponseCode.CATEGORY_PARENT_DELETED,
                        "父分類已被軟刪除，parent_id=" + resolvedParentId);
            }
        }

        Optional<CategoryEntity> existing = categoryRepository.findByNameAndParentId(trimmedName, resolvedParentId);
        if (existing.isPresent()) {
            CategoryEntity e = existing.get();
            if (CommonStatus.ACTIVE.value().equals(e.getStatus())) {
                log.info("createCategory.autoApply",
                        "自動套用既有 ACTIVE 分類，id=" + e.getId() + "，name=" + trimmedName);
                fillParentName(e);
                return e;
            }
            if (CommonStatus.INACTIVE.value().equals(e.getStatus())) {
                throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NAME_RESERVED);
            }
        }

        CategoryEntity newCat = new CategoryEntity();
        newCat.setName(trimmedName);
        newCat.setParentId(resolvedParentId);
        newCat.setStatus(CommonStatus.ACTIVE.value());
        newCat.setCreatedBy(createdBy != null ? String.valueOf(createdBy) : "system");
        newCat.setUpdatedBy(createdBy != null ? String.valueOf(createdBy) : null);

        categoryRepository.insert(newCat);
        cacheService.evict();
        fillParentName(newCat);

        // frontend cache bust（§9.3）：新分類進入分類樹。自動套用既有 ACTIVE 分類的分支
        // 上面已 return，不會走到這裡——那條路徑什麼都沒改，不該失效任何快取。
        productRevalidationService.revalidateCategory(newCat.getId());

        log.info("createCategory.success",
                "新增分類成功，id=" + newCat.getId() + "，name=" + trimmedName + "，parent_id=" + resolvedParentId);
        return newCat;
    }

    // ==================== 寫入：改名 ====================

    /**
     * 修改分類名稱（admin 專屬，僅可改 name，不可改 parent_id）
     */
    @Transactional
    public CategoryEntity updateCategoryName(Long categoryId, String newName, Long updatedBy) {
        if (categoryId == null) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "category_id 不可為空");
        }
        if (newName == null || newName.trim().isEmpty()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "name 不可為空");
        }
        String trimmedName = newName.trim();

        CategoryEntity entity = categoryRepository.findById(categoryId)
                .orElseThrow(() -> new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND));
        if (CommonStatus.INACTIVE.value().equals(entity.getStatus())) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND, "分類已被軟刪除");
        }

        if (trimmedName.equals(entity.getName())) {
            return entity;
        }

        Optional<CategoryEntity> conflict = categoryRepository.findByNameAndParentId(trimmedName, entity.getParentId());
        if (conflict.isPresent() && !conflict.get().getId().equals(entity.getId())) {
            CategoryEntity c = conflict.get();
            if (CommonStatus.ACTIVE.value().equals(c.getStatus())) {
                throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NAME_TAKEN);
            }
            if (CommonStatus.INACTIVE.value().equals(c.getStatus())) {
                throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NAME_RESERVED);
            }
        }

        entity.setName(trimmedName);
        entity.setUpdatedBy(updatedBy != null ? String.valueOf(updatedBy) : null);
        categoryRepository.updateById(entity);
        cacheService.evict();

        // Cascade reflow（02 §5.11.5）：依層級判斷影響範圍，重算受影響 product 的 cat_*_name + searchable_text
        List<Long> affectedProductIds = cascadeReflowAfterCategoryRename(categoryId);

        // frontend cache bust（§9.3）：改名同時動到兩個 surface——分類樹顯示新名稱
        // （category:<id> → categories 展開），cascade 又改了這些商品的 cat_*_name /
        // searchable_text（product:<id> → products:list 展開）。同一 mutation 合成單次 POST。
        // 不共發 seller:<id>：改名不改變任何賣家的公開商品集合（§9.3 ★ 判定原則）。
        productRevalidationService.revalidateCategoryWithProducts(categoryId, affectedProductIds);

        fillParentName(entity);
        log.info("updateCategoryName.success",
                "改名成功，id=" + categoryId + "，new_name=" + trimmedName);
        return entity;
    }

    /**
     * 同事務內把所有掛此 category 的 product 之 {@code cat_*_name} 與 {@code searchable_text} 重算（02 §5.11.5）。
     */
    private List<Long> cascadeReflowAfterCategoryRename(Long categoryId) {
        int level = detectCategoryLevel(categoryId);
        if (level == -1) return List.of();
        List<Long> affectedIds = productCascadeService.findProductIdsByCategoryLevel(categoryId, level);
        if (affectedIds.isEmpty()) return List.of();
        productCascadeService.reflow(affectedIds, "CATEGORY_RENAME_CASCADE");
        return affectedIds;
    }

    // ==================== 寫入：軟刪除 ====================

    /**
     * 批次軟刪除（按拓樸排序執行：L3 → L2 → L1）。
     * <p>每筆前置驗證：底下無商品 + 無 ACTIVE 子分類；任一筆失敗整批 rollback。
     * <p>呼叫端必須提供 productExistsChecker，由 ProductService 注入用以查 product 表。
     */
    @Transactional
    public List<Long> softDelete(
            List<Long> categoryIds,
            Long updatedBy,
            ProductExistsChecker productExistsChecker) {
        if (categoryIds == null || categoryIds.isEmpty()) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "category_ids 不可為空");
        }

        List<CategoryEntity> targets = new ArrayList<>();
        for (Long id : categoryIds) {
            CategoryEntity e = categoryRepository.findById(id)
                    .orElseThrow(() -> new ProductBusinessException(
                            GrpcResponseCode.CATEGORY_NOT_FOUND,
                            "分類不存在，id=" + id));
            if (!CommonStatus.ACTIVE.value().equals(e.getStatus())) {
                throw new ProductBusinessException(
                        GrpcResponseCode.CATEGORY_NOT_FOUND,
                        "分類非 ACTIVE，無法刪除，id=" + id);
            }
            targets.add(e);
        }

        targets.sort((a, b) -> Integer.compare(detectCategoryLevel(b.getId()), detectCategoryLevel(a.getId())));

        Set<Long> beingDeleted = new HashSet<>(categoryIds);
        LocalDateTime now = DateTimeUtils.nowUtc();
        List<Long> deletedIds = new ArrayList<>();

        for (CategoryEntity entity : targets) {
            if (categoryRepository.hasActiveChildren(entity.getId())) {
                List<CategoryEntity> activeChildren = cacheService.findByParentId(entity.getId()).stream()
                        .filter(c -> CommonStatus.ACTIVE.value().equals(c.getStatus()))
                        .filter(c -> !beingDeleted.contains(c.getId()))
                        .toList();
                if (!activeChildren.isEmpty()) {
                    throw new ProductBusinessException(
                            GrpcResponseCode.CATEGORY_HAS_CHILDREN,
                            "分類底下尚有 ACTIVE 子分類，id=" + entity.getId());
                }
            }

            if (productExistsChecker.hasProducts(entity.getId(), detectCategoryLevel(entity.getId()))) {
                throw new ProductBusinessException(
                        GrpcResponseCode.CATEGORY_HAS_PRODUCTS,
                        "分類底下尚有商品，id=" + entity.getId());
            }

            int affected = categoryRepository.softDelete(
                    entity.getId(), now, updatedBy != null ? String.valueOf(updatedBy) : null);
            if (affected != 1) {
                throw new ProductBusinessException(
                        GrpcResponseCode.INTERNAL_ERROR,
                        "軟刪除影響筆數異常，id=" + entity.getId() + "，affected=" + affected);
            }
            deletedIds.add(entity.getId());
        }

        cacheService.evict();

        // frontend cache bust（§9.3）：這些分類離開分類樹。前置驗證保證底下無商品，
        // 故不發 product:<id>——沒有任何商品的顯示欄位被改到。
        productRevalidationService.revalidateCategories(deletedIds);

        log.info("softDelete.success", "批次軟刪除成功，count=" + deletedIds.size());
        return deletedIds;
    }

    /** 提供 ProductService 查商品存在性的 functional interface（避免雙向依賴）。 */
    @FunctionalInterface
    public interface ProductExistsChecker {
        /**
         * @param categoryId 分類 ID
         * @param level 該分類層級（1/2/3），決定查 category_lv1 / category_lv2 / category 哪個欄位
         */
        boolean hasProducts(Long categoryId, int level);
    }

    // ==================== 寫入：還原 ====================

    /**
     * 還原已軟刪除分類。
     * <ul>
     *   <li>非 INACTIVE → CATEGORY_NOT_DELETED</li>
     *   <li>父分類已被軟刪除 → CATEGORY_PARENT_DELETED</li>
     *   <li>同 parentId + name 已存在 ACTIVE → CATEGORY_NAME_TAKEN</li>
     * </ul>
     */
    @Transactional
    public CategoryEntity restore(Long categoryId, Long updatedBy) {
        if (categoryId == null) {
            throw new ProductBusinessException(GrpcResponseCode.INVALID_PARAM, "category_id 不可為空");
        }

        CategoryEntity entity = categoryRepository.findById(categoryId)
                .orElseThrow(() -> new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_FOUND));
        if (!CommonStatus.INACTIVE.value().equals(entity.getStatus())) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NOT_DELETED);
        }

        if (entity.getParentId() != null) {
            CategoryEntity parent = categoryRepository.findById(entity.getParentId())
                    .orElseThrow(() -> new ProductBusinessException(
                            GrpcResponseCode.CATEGORY_PARENT_DELETED, "父分類不存在"));
            if (CommonStatus.INACTIVE.value().equals(parent.getStatus())) {
                throw new ProductBusinessException(GrpcResponseCode.CATEGORY_PARENT_DELETED);
            }
        }

        Optional<CategoryEntity> sameName = categoryRepository.findByNameAndParentId(entity.getName(), entity.getParentId());
        if (sameName.isPresent() && !sameName.get().getId().equals(entity.getId())
                && CommonStatus.ACTIVE.value().equals(sameName.get().getStatus())) {
            throw new ProductBusinessException(GrpcResponseCode.CATEGORY_NAME_TAKEN);
        }

        LocalDateTime now = DateTimeUtils.nowUtc();
        int affected = categoryRepository.restore(
                entity.getId(), now, updatedBy != null ? String.valueOf(updatedBy) : null);
        if (affected != 1) {
            throw new ProductBusinessException(
                    GrpcResponseCode.INTERNAL_ERROR,
                    "還原影響筆數異常，id=" + entity.getId() + "，affected=" + affected);
        }
        cacheService.evict();

        // frontend cache bust（§9.3）：分類回到分類樹。
        productRevalidationService.revalidateCategory(entity.getId());

        CategoryEntity refreshed = categoryRepository.findById(entity.getId()).orElse(entity);
        fillParentName(refreshed);
        log.info("restore.success", "還原成功，id=" + categoryId);
        return refreshed;
    }

    // ==================== 名稱查詢（提供給 ProductService） ====================

    /** 不限狀態取分類名稱（用於商品列表/詳情顯示，含軟刪除分類，修破口）。 */
    public String getCategoryName(Long categoryId) {
        if (categoryId == null) {
            return null;
        }
        return cacheService.findById(categoryId)
                .map(CategoryEntity::getName)
                .orElse(null);
    }

    /** 限 ACTIVE 取分類名稱（用於分類自身查詢，例如賣家分類樹）。 */
    public String getActiveCategoryName(Long categoryId) {
        if (categoryId == null) {
            return null;
        }
        return cacheService.findById(categoryId)
                .filter(c -> CommonStatus.ACTIVE.value().equals(c.getStatus()))
                .map(CategoryEntity::getName)
                .orElse(null);
    }

    /** 不限狀態批次取分類名稱（商品列表填充）。 */
    public Map<Long, String> getCategoryNames(Set<Long> categoryIds) {
        if (categoryIds == null || categoryIds.isEmpty()) {
            return new HashMap<>();
        }
        Map<Long, CategoryEntity> map = cacheService.getAllCategoryMap();
        return categoryIds.stream()
                .filter(map::containsKey)
                .collect(Collectors.toMap(id -> id, id -> map.get(id).getName()));
    }

    /** 取分類三層層級資訊，不限狀態（商品詳情/列表用）。 */
    public CategoryHierarchyDTO getCategoryHierarchy(Long lv3Id, Long lv2Id) {
        if (lv3Id == null) {
            return null;
        }
        CategoryEntity lv3 = cacheService.findById(lv3Id).orElse(null);
        if (lv3 == null) {
            return null;
        }

        CategoryHierarchyDTO dto = CategoryHierarchyDTO.builder()
                .lv3Id(lv3.getId()).lv3Name(lv3.getName()).build();

        if (lv2Id != null) {
            CategoryEntity lv2 = cacheService.findById(lv2Id).orElse(null);
            if (lv2 != null) {
                dto.setLv2Id(lv2.getId());
                dto.setLv2Name(lv2.getName());
                if (lv2.getParentId() != null) {
                    CategoryEntity lv1 = cacheService.findById(lv2.getParentId()).orElse(null);
                    if (lv1 != null) {
                        dto.setLv1Id(lv1.getId());
                        dto.setLv1Name(lv1.getName());
                    }
                }
            }
        }
        return dto;
    }

    // ==================== ProductService 依賴的工具方法 ====================

    /** 檢查分類是否存在且為 ACTIVE 狀態（軟刪除回 false）。 */
    public boolean isActive(Long categoryId) {
        if (categoryId == null) {
            return false;
        }
        return cacheService.findById(categoryId)
                .map(c -> CommonStatus.ACTIVE.value().equals(c.getStatus()))
                .orElse(false);
    }

    /** 取分類 parent_id（給 ProductService 自動填入 category_lv2）。 */
    public Long getParentId(Long categoryId) {
        if (categoryId == null) {
            return null;
        }
        return cacheService.findById(categoryId)
                .map(CategoryEntity::getParentId)
                .orElse(null);
    }

    /** 取第一層分類 ID（給 ProductService 自動填入 category_lv1）。 */
    public Long getLv1Id(Long categoryLv2Id) {
        if (categoryLv2Id == null) {
            return null;
        }
        return cacheService.findById(categoryLv2Id)
                .map(CategoryEntity::getParentId)
                .orElse(null);
    }

    /** 偵測分類層級（1/2/3，-1 表示分類不存在）。 */
    public int detectCategoryLevel(Long categoryId) {
        if (categoryId == null) {
            return -1;
        }
        CategoryEntity cat = cacheService.findById(categoryId).orElse(null);
        if (cat == null) {
            return -1;
        }
        if (cat.getParentId() == null) {
            return 1;
        }
        CategoryEntity parent = cacheService.findById(cat.getParentId()).orElse(null);
        if (parent == null) {
            return -1;
        }
        if (parent.getParentId() == null) {
            return 2;
        }
        return 3;
    }

    // ==================== Internal helpers ====================

    private void fillParentName(CategoryEntity entity) {
        if (entity.getParentId() != null) {
            cacheService.findById(entity.getParentId())
                    .ifPresent(p -> entity.setParentName(p.getName()));
        }
    }

    private void sortCategoryTree(List<CategoryEntity> categories) {
        if (categories == null || categories.isEmpty()) {
            return;
        }
        categories.sort(Comparator.comparing(CategoryEntity::getId));
        for (CategoryEntity c : categories) {
            if (c.getChildren() != null && !c.getChildren().isEmpty()) {
                sortCategoryTree(c.getChildren());
            }
        }
    }
}

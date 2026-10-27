package io.github.will413028.hawkr.product.service;

import io.github.will413028.hawkr.common.cache.RevalidateClient;
import io.github.will413028.hawkr.product.entity.projection.ProductSellerRef;
import io.github.will413028.hawkr.product.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.Collection;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;

/**
 * 商品 / 賣家 / 分類 frontend cache revalidate 的唯一入口（後端服務責任邊界規範 §9）。
 *
 * <p>集中「組 tag → tx commit 後 bust」邏輯，尤其批次 mutation（改分類 / 分類 cascade /
 * 庫存變動）影響多筆商品、可能橫跨多賣家時，統一在此反查 sellerId 並去重，避免各 service
 * 各寫一份（docs/開發規範/Java 微服務開發規範.md §15）。tag 命名與 frontend {@code lib/seo/tags.ts} 對齊。
 *
 * <p><b>producer 只發 entity tag</b>（{@code product:<id>} / {@code seller:<id>} / {@code category:<id>}）；
 * domain-collection（{@code products:list} / {@code sellers:list} / {@code categories} …）由 web {@code /api/revalidate}
 * 於信任邊界依 entity 展開（53 號設計書 §4.1/§4.3），backend 不直接發，亦不再發 legacy {@code home}。
 *
 * <p><b>要不要共發 {@code seller:<id>} 的判準</b>（§9.3）：看該 mutation 是否改變
 * <b>「這個賣家有哪些公開商品」</b>。改變（建立 / 刪除 / 上下架跨 ACTIVE / 批次下架）→ 共發；
 * 不改變（欄位編輯 / 庫存增減 / 改分類）→ 只發 {@code product:<id>}，storefront 列表由
 * {@code product:<id> → products:list} 展開涵蓋。
 *
 * <p>底層 {@link RevalidateClient#bustAfterCommit} 為 fire-and-forget、fail-soft、連線層
 * IOException 冪等重試（§9.5）。
 */
@Service
public class ProductRevalidationService {

    private final RevalidateClient revalidateClient;
    private final ProductRepository productRepository;
    private final SellerGrpcClient sellerGrpcClient;

    public ProductRevalidationService(RevalidateClient revalidateClient,
                                      ProductRepository productRepository,
                                      SellerGrpcClient sellerGrpcClient) {
        this.revalidateClient = revalidateClient;
        this.productRepository = productRepository;
        this.sellerGrpcClient = sellerGrpcClient;
    }

    /**
     * 把一組 store_id 展開成「該賣家在 web 上被快取的每一把 key」。
     *
     * <p>web 以 <b>URL 路徑片段</b>當 cache key（{@code lib/api/cached.ts} 把 [id] 直接餵進
     * {@code sellerTag(id)}），而同一個賣家有兩個片段各自活著：
     * <ul>
     *   <li>{@code store_code}——店面頁 canonical 片段。{@code /sellers/{store_id}} 會 308 轉到
     *       {@code /sellers/{store_code}}，所以店面頁<b>只會</b>快取在這把 key 底下。</li>
     *   <li>{@code store_id}——商品詳情頁（PDP）的賣家卡片走
     *       {@code features/product/SellerIntroAsync.tsx}，它拿的是商品 payload 裡的
     *       {@code seller.storeId}，那條路徑不經過 308，所以快取在這把 key 底下。</li>
     * </ul>
     *
     * <p>只發 store_id（本方法引入前的行為）＝店面頁的失效永遠打不中，而
     * {@code /api/revalidate} 仍回 200，看起來成功；只發 store_code 則反過來讓 PDP 卡片變舊。
     * 兩把都要。store_code 由 seller-service 解析（{@code SellerMeta.store_code}）。
     *
     * <p>Fail-soft：解析不到就只回 store_id tag——退回原本的覆蓋範圍，比整批不發好。
     */
    private Set<String> sellerTags(Set<Long> storeIds) {
        Set<String> tags = new LinkedHashSet<>();
        if (storeIds == null || storeIds.isEmpty()) return tags;
        for (Long id : storeIds) {
            if (id != null) tags.add("seller:" + id);
        }
        Map<Long, SellerMeta> metas = sellerGrpcClient.batchGetSellerMetas(storeIds);
        for (SellerMeta meta : metas.values()) {
            String storeCode = meta.getStoreCode();
            if (storeCode != null && !storeCode.isBlank()) {
                tags.add("seller:" + storeCode);
            }
        }
        return tags;
    }

    // ==================== 商品集合不變：只發 product entity tag ====================

    /**
     * bust {@code product:<id>}（單筆商品欄位變更：改價 / 描述 / 圖片 / 庫存 / 分類）。
     *
     * <p>不共發 {@code seller:<id>}——賣家層 surface（profile / stats / 評論）不嵌任何商品欄位，
     * 而 storefront 商品列表同掛 {@code products:list}，由 {@code product:<id>} 展開即命中。
     */
    public void revalidateProduct(Long productId) {
        if (productId == null) return;
        revalidateClient.bustAfterCommit(List.of("product:" + productId));
    }

    /**
     * 批次版 {@link #revalidateProduct(Long)}：bust {@code product:<id>}×N（去重）。
     * 用於庫存變動 / 改分類 / 分類改名 cascade 等只動商品欄位的批次 mutation。
     */
    public void revalidateProducts(Collection<Long> productIds) {
        List<String> tags = distinctProductTags(productIds);
        if (tags.isEmpty()) return;
        revalidateClient.bustAfterCommit(tags);
    }

    // ==================== 商品集合變動 / 賣家實體變動：共發 seller tag ====================

    /**
     * bust {@code product:<id>} + {@code seller:<sellerId>}（改變賣家公開商品集合：建立即上架、
     * 上下架跨 ACTIVE、刪除前為 ACTIVE）。單次 POST 帶兩個 tag，避免重複註冊 afterCommit hook。
     */
    public void revalidateProductAndSeller(Long productId, Long sellerId) {
        Set<String> tags = new LinkedHashSet<>();
        if (productId != null) tags.add("product:" + productId);
        if (sellerId != null) tags.addAll(sellerTags(Set.of(sellerId)));
        if (!tags.isEmpty()) revalidateClient.bustAfterCommit(new ArrayList<>(tags));
    }

    /** bust {@code seller:<store_id>} + {@code seller:<store_code>}（僅賣家層級變更，
     *  如新建非公開（DRAFT）商品進賣家商品集合）。兩把 key 的理由見 {@link #sellerTags}。 */
    public void revalidateSeller(Long sellerId) {
        if (sellerId == null) return;
        Set<String> tags = sellerTags(Set.of(sellerId));
        if (!tags.isEmpty()) revalidateClient.bustAfterCommit(new ArrayList<>(tags));
    }

    /**
     * 批次：給一組 productId，反查各筆所屬賣家，bust {@code product:<id>}×N +
     * {@code seller:<sellerId>}×N（去重）。用於賣家批次下架 / 停權下架 / 解除停權還原——
     * 這些是「賣家公開商品集合整批變動 + 賣家實體狀態變動」，故共發 seller tag。
     */
    public void revalidateProductsWithSellers(Collection<Long> productIds) {
        if (productIds == null || productIds.isEmpty()) return;
        List<Long> ids = productIds.stream().filter(Objects::nonNull).distinct().toList();
        if (ids.isEmpty()) return;
        Set<String> tags = new LinkedHashSet<>();
        for (Long id : ids) tags.add("product:" + id);
        List<ProductSellerRef> refs = productRepository.findProductSellerRefs(ids);
        Set<Long> storeIds = new LinkedHashSet<>();
        for (ProductSellerRef r : refs) {
            if (r.sellerId() != null) storeIds.add(r.sellerId());
        }
        // 一次批次解析所有賣家的 store_code，不逐筆呼叫（賣家數 N ≪ 商品數）。
        tags.addAll(sellerTags(storeIds));
        revalidateClient.bustAfterCommit(new ArrayList<>(tags));
    }

    // ==================== 分類：分類樹 surface ====================

    /**
     * bust {@code category:<id>}（分類新增 / 軟刪除 / 還原——改變分類樹的組成）。
     *
     * <p>web {@code getCachedCategoryTree} 的 TTL 是 21600 秒（6 小時），是全站最長的一條；
     * 沒有 producer 時分類改名要最久 6 小時才反映。entity tag 由 web expansion
     * （{@code category:<id> → categories}）展開到分類樹快取（53 號設計書 §4.3）。
     */
    public void revalidateCategory(Long categoryId) {
        if (categoryId == null) return;
        revalidateClient.bustAfterCommit(List.of("category:" + categoryId));
    }

    /** 批次版 {@link #revalidateCategory(Long)}（批次軟刪除一次 POST 帶 N 個 tag）。 */
    public void revalidateCategories(Collection<Long> categoryIds) {
        List<String> tags = distinctTags("category:", categoryIds);
        if (tags.isEmpty()) return;
        revalidateClient.bustAfterCommit(tags);
    }

    /**
     * bust {@code category:<id>} + {@code product:<id>}×N，單次 POST（分類改名）。
     *
     * <p>改名同時動到兩個 surface：分類樹顯示新名稱，而 cascade reflow 改寫了掛在該分類下
     * 每筆商品的 {@code cat_*_name} / {@code searchable_text}。合成一次 POST 而非各發一次，
     * 因為它們是**同一個 mutation**（§9.8 禁止跨 mutation 合併，但同一 mutation 必須合併）。
     */
    public void revalidateCategoryWithProducts(Long categoryId, Collection<Long> productIds) {
        Set<String> tags = new LinkedHashSet<>();
        if (categoryId != null) tags.add("category:" + categoryId);
        tags.addAll(distinctProductTags(productIds));
        if (!tags.isEmpty()) revalidateClient.bustAfterCommit(new ArrayList<>(tags));
    }

    private List<String> distinctTags(String prefix, Collection<Long> ids) {
        if (ids == null || ids.isEmpty()) return List.of();
        Set<String> tags = new LinkedHashSet<>();
        for (Long id : ids) {
            if (id != null) tags.add(prefix + id);
        }
        return new ArrayList<>(tags);
    }

    private List<String> distinctProductTags(Collection<Long> productIds) {
        return distinctTags("product:", productIds);
    }
}

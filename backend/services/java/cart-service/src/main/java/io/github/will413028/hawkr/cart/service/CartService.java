package io.github.will413028.hawkr.cart.service;

import io.github.will413028.hawkr.common.utils.code.CartItemStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ProductPurchasabilityRules;
import io.github.will413028.hawkr.cart.entity.CartItemEntity;
import io.github.will413028.hawkr.cart.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.cart.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.cart.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.cart.repository.CartItemRepository;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.grpc.UnresolvedPrincipalException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.cart.exception.CartBusinessException;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.product.grpc.GetProductSimpleResponse;
import io.github.will413028.hawkr.product.grpc.ProductSimpleInfo;
import io.github.will413028.hawkr.product.grpc.SkuCartInfo;
import io.grpc.StatusRuntimeException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.LocalDateTime;
import java.util.*;
import java.util.stream.Collectors;

/**
 * 購物車核心業務邏輯（設計書 §5）
 */
@Service
public class CartService {

    private static final ObsLogger log = ObsLogger.forClass(CartService.class);

    private static final int LOCK_TTL_MINUTES = 2;
    private final CartItemRepository cartItemRepository;
    private final CartCacheService cartCacheService;
    private final ProductGrpcClient productGrpcClient;
    private final SellerGrpcClient sellerGrpcClient;
    private final BuyerGrpcClient buyerGrpcClient;

    public CartService(CartItemRepository cartItemRepository,
                       CartCacheService cartCacheService,
                       ProductGrpcClient productGrpcClient,
                       SellerGrpcClient sellerGrpcClient,
                       BuyerGrpcClient buyerGrpcClient) {
        this.cartItemRepository = cartItemRepository;
        this.cartCacheService = cartCacheService;
        this.productGrpcClient = productGrpcClient;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    /**
     * 解析賣家側 id → principal（Plan 2b）。硬依賴：Seller Service 失敗即拋
     * SERVICE_UNAVAILABLE，不 fail-open — 自買防護是防詐骨科。
     */
    private PrincipalResolution resolveSellerPrincipals(List<Long> storeIds, Long buyerId, String guestId) {
        try {
            return sellerGrpcClient.resolvePrincipals(storeIds);
        } catch (StatusRuntimeException e) {
            log.error("CartService.resolveSellerPrincipals.rpcFail",
                    "Seller Service principal 解析失敗 " + ownerTag(buyerId, guestId), e);
            throw new CartBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE);
        }
    }

    /**
     * 解析呼叫者身份（自 gateway PR#1585 起可能已是 buyer_id）→ 登入 principal account_id，
     * 用於自買防護跟 sellerPrincipal 比較（鏡像 0b0d3d0bd 的 seller 軸修復）。硬依賴，fail-closed。
     */
    private PrincipalResolution resolveBuyerPrincipals(Long buyerId, String guestId) {
        try {
            return buyerGrpcClient.resolvePrincipals(List.of(buyerId));
        } catch (StatusRuntimeException e) {
            log.error("CartService.resolveBuyerPrincipal.rpcFail",
                    "Buyer Service principal 解析失敗 " + ownerTag(buyerId, guestId), e);
            throw new CartBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE);
        }
    }

    /**
     * 自買防護的兩側身分都<b>必須是真正解析出來的</b> account_id。
     *
     * <p>seller-service / buyer-service 對查無在架列的 id（軟刪除或不存在）會把輸入原樣
     * 回吐成一個合法非零值，舊寫法 {@code getOrDefault(id, id)} 完全分辨不出來——於是
     * 「無法確認是不是同一個人」被當成「不是同一個人」，防護靜默失效（issue #2789）。
     * 未解析一律 fail-closed：拒絕該操作、記 log，不拿未驗證的 echo-back 做身分比對。
     *
     * <p>對外回 {@link GrpcResponseCode#INVALID_SKU}（「商品不存在或已下架」）：
     * store_id 查無在架賣家時商品本來就不該被購買，這是使用者看得懂、也是真實的原因；
     * 這一條和同方法 SKU 已下架走的是同一個碼。
     */
    private CartBusinessException rejectUnresolvedPrincipal(String event, String action,
                                                            UnresolvedPrincipalException e,
                                                            Long buyerId, String guestId) {
        log.warn(event,
                "身分反解查無在架資料，自買防護無法評估，fail-closed 拒絕" + action + " "
                        + ownerTag(buyerId, guestId) + "：" + e.getMessage());
        return new CartBusinessException(GrpcResponseCode.INVALID_SKU);
    }


    /**
     * 加入購物車
     *
     * @return AddItemResult 包含 code, message, totalItems
     */
    public AddItemResult addItem(Long buyerId, String guestId, UUID skuId, int quantity) {
        GetProductSimpleResponse productResp;
        try {
            productResp = productGrpcClient.getProductSimple(skuId.toString());
        } catch (StatusRuntimeException e) {
            log.error("CartService.addItem.productRpcFail",
                    "Product Service 呼叫失敗 " + ownerTag(buyerId, guestId) + " skuId=" + skuId, e);
            throw new CartBusinessException(GrpcResponseCode.SERVICE_UNAVAILABLE);
        }

        if (productResp.getCode() != GrpcResponseCode.SUCCESS.code()) {
            log.info("CartService.addItem.productNotFound",
                    "商品不存在 " + ownerTag(buyerId, guestId), "skuId", String.valueOf(skuId));
            throw new CartBusinessException(GrpcResponseCode.INVALID_SKU);
        }

        ProductSimpleInfo info = productResp.getData();
        if (!info.getIsActive()) {
            log.info("CartService.addItem.productInactive",
                    "商品已下架 " + ownerTag(buyerId, guestId), "skuId", String.valueOf(skuId));
            throw new CartBusinessException(GrpcResponseCode.INVALID_SKU);
        }

        Long storeId;
        Long unitPrice;
        Long listPrice;
        int maxLimit;
        UUID productId;
        try {
            storeId = Long.parseLong(info.getStoreId());
            unitPrice = Long.parseLong(info.getPrice());
            listPrice = info.getListPrice().isEmpty() ? unitPrice : Long.parseLong(info.getListPrice());
            maxLimit = info.getMaxLimit().isEmpty() ? Integer.MAX_VALUE : Integer.parseInt(info.getMaxLimit());
            productId = ProtoConversionUtils.parseUuidRequired(info.getProductId(), "product_id");
        } catch (IllegalArgumentException e) {
            log.error("CartService.addItem.parseError",
                    "商品資訊格式異常 " + ownerTag(buyerId, guestId),
                    "skuId", String.valueOf(skuId));
            throw new CartBusinessException(GrpcResponseCode.CART_PRODUCT_DATA_INVALID, e.getMessage());
        }
        if (buyerId != null) {
            PrincipalResolution sellerResolution = resolveSellerPrincipals(List.of(storeId), buyerId, guestId);
            PrincipalResolution buyerResolution = resolveBuyerPrincipals(buyerId, guestId);
            long sellerPrincipal;
            long buyerPrincipal;
            try {
                sellerPrincipal = sellerResolution.requireResolved(storeId, "store_id");
                buyerPrincipal = buyerResolution.requireResolved(buyerId, "buyer_id");
            } catch (UnresolvedPrincipalException e) {
                throw rejectUnresolvedPrincipal("CartService.addItem.principalUnresolved",
                        "加入購物車 skuId=" + skuId + " storeId=" + storeId, e, buyerId, guestId);
            }
            if (buyerPrincipal == sellerPrincipal) {
                throw new CartBusinessException(GrpcResponseCode.CART_SELF_ITEM_NOT_ALLOWED);
            }
        }

        String skuName = info.getName();
        String specName = info.getSkuName();
        String imageObjectKey = info.getImageObjectKey();

        int currentQty = 0;
        CartItemEntity existingItem = (buyerId != null)
                ? cartItemRepository.findByBuyerIdAndSkuId(buyerId, skuId)
                : cartItemRepository.findByGuestIdAndSkuId(guestId, skuId);
        if (existingItem != null) {
            currentQty = existingItem.getQuantity();
        }
        if (currentQty + quantity > maxLimit) {
            log.info("CartService.addItem.exceedsMaxLimit",
                    "超出購買上限 " + ownerTag(buyerId, guestId)
                            + " skuId=" + skuId + " currentQty=" + currentQty
                            + " requestedQty=" + quantity + " maxLimit=" + maxLimit,
                    "skuId", String.valueOf(skuId));
            throw new CartBusinessException(GrpcResponseCode.CART_QUANTITY_EXCEEDS_MAX_LIMIT,
                    String.format(GrpcResponseCode.CART_QUANTITY_EXCEEDS_MAX_LIMIT.message(), maxLimit));
        }

        CartItemEntity entity = new CartItemEntity();
        entity.setId(TsidUtil.generate());
        entity.setStoreId(storeId);
        entity.setSkuId(skuId);
        entity.setQuantity(quantity);
        entity.setAddedUnitPrice(unitPrice);
        entity.setAddedListPrice(listPrice);
        entity.setMaxLimitSnapshot(maxLimit);
        entity.setSkuNameSnapshot(skuName);
        entity.setProductId(productId);
        entity.setSpecNameSnapshot(specName);
        entity.setImageObjectKey(imageObjectKey);

        int rows;
        if (buyerId != null) {
            entity.setBuyerId(buyerId);
            rows = cartItemRepository.upsertMemberItem(entity);
        } else {
            entity.setGuestId(guestId);
            rows = cartItemRepository.upsertGuestItem(entity);
        }

        if (rows == 0) {
            log.info("CartService.addItem.locked",
                    "商品處於鎖定狀態 " + ownerTag(buyerId, guestId), "skuId", String.valueOf(skuId));
            throw new CartBusinessException(GrpcResponseCode.CART_ITEM_LOCKED);
        }

        cartCacheService.invalidateCartCache(buyerId, guestId);

        int totalItems = countItems(buyerId, guestId);

        log.info("CartService.addItem.success",
                "加入購物車成功 " + ownerTag(buyerId, guestId), "skuId", String.valueOf(skuId));
        return new AddItemResult(GrpcResponseCode.SUCCESS.code(), "", totalItems);
    }


    public GetCartResult getCart(Long buyerId, String guestId) {
        List<CartItemEntity> normalItems = cartCacheService.getCartCache(buyerId, guestId);
        if (normalItems == null) {
            normalItems = (buyerId != null)
                    ? cartItemRepository.findNormalByBuyerId(buyerId)
                    : cartItemRepository.findNormalByGuestId(guestId);
            cartCacheService.fillCartCache(buyerId, guestId, normalItems);
        }

        List<CartItemEntity> lockedItems = (buyerId != null)
                ? cartItemRepository.findLockedByBuyerId(buyerId)
                : cartItemRepository.findLockedByGuestId(guestId);

        List<CartItemEntity> allItems = new ArrayList<>(normalItems);
        allItems.addAll(lockedItems);

        if (allItems.isEmpty()) {
            return new GetCartResult(Collections.emptyList(), Collections.emptyMap(), 0L, 0);
        }

        Map<Long, List<CartItemEntity>> grouped = allItems.stream()
                .collect(Collectors.groupingBy(CartItemEntity::getStoreId, LinkedHashMap::new, Collectors.toList()));

        List<Long> allStoreIds = new ArrayList<>(grouped.keySet());
        Map<Long, CartCacheService.SellerMetaCacheEntry> sellerMetaCache = cartCacheService.getSellerMetaBatch(allStoreIds);

        List<Long> missStoreIds = allStoreIds.stream()
                .filter(id -> !sellerMetaCache.containsKey(id))
                .toList();

        if (!missStoreIds.isEmpty()) {
            Map<Long, SellerMeta> metaMap = sellerGrpcClient.batchGetSellerMeta(missStoreIds);
            for (var entry : metaMap.entrySet()) {
                SellerMeta meta = entry.getValue();
                var cacheEntry = new CartCacheService.SellerMetaCacheEntry(
                        meta.getSellerName(), meta.getLogoKey(),
                        meta.getCity(), meta.getDistrict(), meta.getStoreCode());
                sellerMetaCache.put(entry.getKey(), cacheEntry);
                cartCacheService.setSellerMeta(entry.getKey(), cacheEntry);
            }
        }

        List<String> allSkuIds = allItems.stream()
                .map(item -> String.valueOf(item.getSkuId()))
                .toList();
        Map<String, SkuCartInfo> skuCartInfoMap = productGrpcClient.batchGetSkuCartInfo(allSkuIds);

        List<SellerGroup> groups = new ArrayList<>();
        List<Long> storesWithoutMeta = new ArrayList<>();
        for (var entry : grouped.entrySet()) {
            Long storeId = entry.getKey();
            CartCacheService.SellerMetaCacheEntry metaEntry = sellerMetaCache.get(storeId);
            if (metaEntry == null) {
                storesWithoutMeta.add(storeId);
            }
            String sellerName = metaEntry != null ? metaEntry.name() : "";
            String logoUrl = metaEntry != null ? metaEntry.logoUrl() : "";
            String city = metaEntry != null ? metaEntry.city() : "";
            String district = metaEntry != null ? metaEntry.district() : "";
            String storeCode = metaEntry != null ? metaEntry.storeCode() : "";
            groups.add(new SellerGroup(storeId, sellerName, logoUrl, city, district, storeCode, entry.getValue()));
        }
        if (!storesWithoutMeta.isEmpty()) {
            log.error("CartService.getCart.sellerMetaUnavailable",
                    "賣家 meta 取不到，store_code 等欄位降級為空 " + ownerTag(buyerId, guestId),
                    "storeIds", storesWithoutMeta.toString());
        }

        long estimatedTotal = normalItems.stream()
                .filter(item -> {
                    SkuCartInfo info = skuCartInfoMap.get(String.valueOf(item.getSkuId()));
                    if (info == null) return false;
                    long stock;
                    try {
                        stock = info.getStockQty().isEmpty() ? 0L : Long.parseLong(info.getStockQty());
                    } catch (NumberFormatException e) {
                        stock = 0L;
                    }
                    long qty = item.getQuantity() != null ? item.getQuantity() : 0L;
                    return ProductPurchasabilityRules.evaluate(info.getIsActive(), stock, qty).purchasable();
                })
                .mapToLong(item -> item.getAddedUnitPrice() * item.getQuantity())
                .sum();

        int totalItems = allItems.size();

        return new GetCartResult(groups, skuCartInfoMap, estimatedTotal, totalItems);
    }


    public record MergeCartResult(int removedSelfOwnedCount) {}

    @Transactional
    public MergeCartResult mergeCart(Long buyerId, String guestId) {
        if (!cartCacheService.tryMergeLock(guestId, buyerId)) {
            log.info("CartService.mergeCart.skipDuplicate",
                    "合併鎖衝突，跳過 buyerId=" + buyerId, "guestId", guestId);
            return new MergeCartResult(0);
        }

        List<CartItemEntity> guestItems = cartItemRepository.findNormalByGuestIdForMerge(guestId);
        if (guestItems.isEmpty()) {
            return new MergeCartResult(0);
        }

        List<Long> distinctStoreIds = guestItems.stream()
                .map(CartItemEntity::getStoreId)
                .distinct()
                .toList();
        Map<Long, Long> sellerPrincipals;
        long buyerPrincipal;
        try {
            sellerPrincipals = resolveSellerPrincipals(distinctStoreIds, buyerId, guestId)
                    .requireAllResolved(distinctStoreIds, "store_id");
            buyerPrincipal = resolveBuyerPrincipals(buyerId, guestId)
                    .requireResolved(buyerId, "buyer_id");
        } catch (UnresolvedPrincipalException e) {
            throw rejectUnresolvedPrincipal("CartService.mergeCart.principalUnresolved",
                    "合併購物車（未刪除、未合併任何項目）", e, buyerId, guestId);
        }

        int removedSelfOwnedCount = (int) guestItems.stream()
                .filter(item -> sellerPrincipals.get(item.getStoreId()) == buyerPrincipal)
                .map(CartItemEntity::getSkuId)
                .distinct()
                .count();
        if (removedSelfOwnedCount > 0) {
            List<Long> selfOwnedStoreIds = distinctStoreIds.stream()
                    .filter(id -> sellerPrincipals.get(id) == buyerPrincipal)
                    .toList();
            cartItemRepository.deleteSelfOwnedGuestItems(guestId, selfOwnedStoreIds);
        }

        List<CartItemEntity> nonSelfOwnedItems = guestItems.stream()
                .filter(item -> sellerPrincipals.get(item.getStoreId()) != buyerPrincipal)
                .toList();

        for (CartItemEntity guestItem : nonSelfOwnedItems) {
            CartItemEntity memberItem = cartItemRepository.findByBuyerIdAndSkuId(buyerId, guestItem.getSkuId());

            if (memberItem == null) {
                cartItemRepository.transferOwnerToMember(guestItem.getId(), buyerId);
            } else {
                int newQty = memberItem.getQuantity() + guestItem.getQuantity();
                int finalQty = Math.min(newQty, memberItem.getMaxLimitSnapshot());
                memberItem.setQuantity(finalQty);
                cartItemRepository.updateById(memberItem);
                cartItemRepository.deleteById(guestItem.getId());
            }
        }

        cartCacheService.invalidateCartCache(buyerId, null);
        cartCacheService.invalidateCartCache(null, guestId);

        log.info("CartService.mergeCart.success",
                "合併購物車完成 removedSelfOwnedCount=" + removedSelfOwnedCount,
                "guestId", guestId);
        return new MergeCartResult(removedSelfOwnedCount);
    }


    public SetItemResult setItemQuantity(Long buyerId, String guestId, UUID skuId, int quantity) {
        CartItemEntity item = (buyerId != null)
                ? cartItemRepository.findByBuyerIdAndSkuId(buyerId, skuId)
                : cartItemRepository.findByGuestIdAndSkuId(guestId, skuId);

        if (item == null) {
            throw new CartBusinessException(GrpcResponseCode.CART_ITEM_NOT_FOUND);
        }

        if (CartItemStatus.LOCKED.value().equals(item.getStatus())) {
            throw new CartBusinessException(GrpcResponseCode.CART_ITEM_LOCKED);
        }

        if (quantity == 0) {
            if (buyerId != null) {
                cartItemRepository.deleteByBuyerSku(buyerId, skuId);
            } else {
                cartItemRepository.deleteByGuestSku(guestId, skuId);
            }
        } else {
            if (quantity > item.getMaxLimitSnapshot()) {
                log.info("CartService.setItemQuantity.exceedsMaxLimit",
                        "超出購買上限 " + ownerTag(buyerId, guestId)
                                + " skuId=" + skuId + " requestedQty=" + quantity
                                + " maxLimit=" + item.getMaxLimitSnapshot(),
                        "skuId", String.valueOf(skuId));
                throw new CartBusinessException(GrpcResponseCode.CART_QUANTITY_EXCEEDS_MAX_LIMIT,
                        String.format(GrpcResponseCode.CART_QUANTITY_EXCEEDS_MAX_LIMIT.message(),
                                item.getMaxLimitSnapshot()));
            }

            if (buyerId != null) {
                cartItemRepository.updateQuantityByBuyerSku(buyerId, skuId, quantity);
            } else {
                cartItemRepository.updateQuantityByGuestSku(guestId, skuId, quantity);
            }
        }

        cartCacheService.invalidateCartCache(buyerId, guestId);

        int totalItems = countItems(buyerId, guestId);

        return new SetItemResult(GrpcResponseCode.SUCCESS.code(), "", totalItems);
    }


    public int checkoutLock(Long buyerId, String checkoutId, List<UUID> skuIds) {
        if (skuIds == null || skuIds.isEmpty()) {
            throw new CartBusinessException(GrpcResponseCode.CART_SKU_IDS_REQUIRED);
        }

        List<UUID> distinctSkuIds = skuIds.stream().distinct().toList();
        if (distinctSkuIds.size() != skuIds.size()) {
            throw new CartBusinessException(GrpcResponseCode.CART_SKU_IDS_DUPLICATED);
        }

        LocalDateTime lockExpire = DateTimeUtils.nowUtc().plusMinutes(LOCK_TTL_MINUTES);
        int updatedRows = cartItemRepository.lockItemsByBuyerAndSkus(buyerId, checkoutId, lockExpire, distinctSkuIds);

        if (updatedRows != distinctSkuIds.size()) {
            log.info("CartService.checkoutLock.fail",
                    "鎖定失敗: expected=" + distinctSkuIds.size() + ", actual=" + updatedRows
                            + " buyerId=" + buyerId, "checkoutId", checkoutId);
            throw new CartBusinessException(GrpcResponseCode.CHECKOUT_LOCK_FAILED);
        }

        cartCacheService.invalidateCartCache(buyerId, null);

        log.info("CartService.checkoutLock.success",
                "結帳鎖定成功 buyerId=" + buyerId, "checkoutId", checkoutId);
        return GrpcResponseCode.SUCCESS.code();
    }


    public int checkoutUnlock(Long buyerId, String checkoutId, List<UUID> skuIds) {
        if (skuIds != null && !skuIds.isEmpty()) {
            long distinct = skuIds.stream().distinct().count();
            if (distinct != skuIds.size()) {
                throw new CartBusinessException(GrpcResponseCode.CART_SKU_IDS_DUPLICATED);
            }
        }

        cartItemRepository.unlockItemsByCheckoutId(buyerId, checkoutId, skuIds);

        cartCacheService.invalidateCartCache(buyerId, null);

        log.info("CartService.checkoutUnlock.success",
                "解除鎖定完成 buyerId=" + buyerId, "checkoutId", checkoutId);
        return GrpcResponseCode.SUCCESS.code();
    }


    public int removeItemsByCheckoutId(Long buyerId, String checkoutId, List<UUID> skuIds) {
        if (skuIds != null && !skuIds.isEmpty()) {
            long distinct = skuIds.stream().distinct().count();
            if (distinct != skuIds.size()) {
                throw new CartBusinessException(GrpcResponseCode.CART_SKU_IDS_DUPLICATED);
            }
        }

        cartItemRepository.removeByCheckoutId(buyerId, checkoutId, skuIds);

        cartCacheService.invalidateCartCache(buyerId, null);

        log.info("CartService.removeByCheckoutId.success",
                "結帳清理完成 buyerId=" + buyerId, "checkoutId", checkoutId);
        return GrpcResponseCode.SUCCESS.code();
    }


    /** 產生身分標記（用於日誌追蹤） */
    private String ownerTag(Long buyerId, String guestId) {
        return buyerId != null ? "buyerId=" + buyerId : "guest=" + guestFingerprint(guestId);
    }

    /**
     * 把訪客識別碼壓成不可逆的短指紋再進 log。
     *
     * <p>guestId 不是 PII，但它<b>是憑證</b>：後端對匿名購物車的讀 / 改 / 刪 / merge
     * 全部只認這一個值，沒有簽章也沒有擁有者檢查（#2295 ③）。原樣寫進 log 等於把
     * 「誰能接管那台裝置的購物車」交給任何有 log 讀取權的人——而 buyerId 沒有這個
     * 問題，它要配上 JWT 才有意義，所以兩者不該同樣處理。
     *
     * <p>取 SHA-256 前 8 個 hex：同一裝置的多行 log 仍然串得起來（除錯價值保留），
     * 但值本身推不回去。雜湊失敗時回 <code>"?"</code> 而非原值——log 輔助功能不該
     * 成為洩漏路徑，寧可少一行線索。
     */
    private static String guestFingerprint(String guestId) {
        if (guestId == null || guestId.isEmpty()) {
            return "-";
        }
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256")
                    .digest(guestId.getBytes(StandardCharsets.UTF_8));
            StringBuilder sb = new StringBuilder(8);
            for (int i = 0; i < 4; i++) {
                sb.append(String.format("%02x", digest[i]));
            }
            return sb.toString();
        } catch (NoSuchAlgorithmException e) {
            return "?";
        }
    }

    private int countItems(Long buyerId, String guestId) {
        return (buyerId != null)
                ? cartItemRepository.countByBuyerId(buyerId)
                : cartItemRepository.countByGuestId(guestId);
    }


    public record AddItemResult(int code, String message, int totalItems) {}

    public record SetItemResult(int code, String message, int totalItems) {}

    public record GetCartResult(List<SellerGroup> groups,
                                Map<String, io.github.will413028.hawkr.product.grpc.SkuCartInfo> skuCartInfoMap,
                                long estimatedTotal, int totalItems) {}

    public record SellerGroup(Long storeId, String sellerName, String logoUrl,
                              String city, String district, String storeCode, List<CartItemEntity> items) {}
}

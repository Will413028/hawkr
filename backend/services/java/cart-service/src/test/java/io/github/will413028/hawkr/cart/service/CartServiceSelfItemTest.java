package io.github.will413028.hawkr.cart.service;

import io.github.will413028.hawkr.cart.entity.CartItemEntity;
import io.github.will413028.hawkr.cart.exception.CartBusinessException;
import io.github.will413028.hawkr.cart.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.cart.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.cart.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.cart.repository.CartItemRepository;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.product.grpc.GetProductSimpleResponse;
import io.github.will413028.hawkr.product.grpc.ProductSimpleInfo;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.assertj.core.api.Assertions.assertThatCode;

class CartServiceSelfItemTest {

    // Cart Service node-id-base = 400（application.yml）。同 codebase 慣例（見
    // order-service AutoRefundWorkerTest 等）：跨測試類別重複執行時 init 會拋
    // IllegalStateException，吞掉即可 — 此測試不驗證 TSID 本身，只需讓
    // addItem 的 upsert entity 組建路徑能跑到底。
    @BeforeAll
    static void initTsid() {
        try { TsidUtil.init(400, 0); } catch (IllegalStateException ignored) { }
    }

    private GetProductSimpleResponse productResponse(long storeId) {
        ProductSimpleInfo info = ProductSimpleInfo.newBuilder()
                .setIsActive(true)
                .setStoreId(String.valueOf(storeId))
                .setPrice("100")
                .build();
        return GetProductSimpleResponse.newBuilder().setCode(0).setData(info).build();
    }

    private CartService newService(ProductGrpcClient productClient, SellerGrpcClient sellerClient,
                                    CartItemRepository repo, CartCacheService cache) {
        return newService(productClient, sellerClient, repo, cache, identityBuyerClient());
    }

    private CartService newService(ProductGrpcClient productClient, SellerGrpcClient sellerClient,
                                    CartItemRepository repo, CartCacheService cache, BuyerGrpcClient buyerClient) {
        return new CartService(repo, cache, productClient, sellerClient, buyerClient);
    }

    /** 買家軸無分岔的預設 mock：任何呼叫者身份原封不動 resolve 回自己（比照 grandfathered 案例）。 */
    private BuyerGrpcClient identityBuyerClient() {
        BuyerGrpcClient buyerClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerClient.resolvePrincipals(Mockito.anyList())).thenAnswer(invocation -> {
            List<Long> ids = invocation.getArgument(0);
            return PrincipalResolution.allResolved(
                    ids.stream().collect(java.util.stream.Collectors.toMap(id -> id, id -> id)));
        });
        return buyerClient;
    }

    private CartItemEntity guestItem(long id, String guestId, long storeId, long skuId) {
        CartItemEntity item = new CartItemEntity();
        item.setId(id);
        item.setGuestId(guestId);
        item.setStoreId(storeId);
        item.setSkuId(skuId);
        item.setQuantity(1);
        return item;
    }

    @Test
    void grandfathered_differentSellerAndBuyer_allowed() {
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(200L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.upsertMemberItem(Mockito.any())).thenReturn(1);
        CartCacheService cache = Mockito.mock(CartCacheService.class);

        CartService service = newService(productClient, sellerClient, repo, cache);

        assertThatCode(() -> service.addItem(100L, null, 1L, 1)).doesNotThrowAnyException();
    }

    @Test
    void grandfathered_sameIdBothSides_blocked() {
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(100L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(100L))).thenReturn(PrincipalResolution.allResolved(Map.of(100L, 100L)));
        CartService service = newService(productClient, sellerClient,
                Mockito.mock(CartItemRepository.class), Mockito.mock(CartCacheService.class));

        assertThatThrownBy(() -> service.addItem(100L, null, 1L, 1))
                .isInstanceOf(CartBusinessException.class);
    }

    @Test
    void diverged_storeIdResolvesToBuyerPrincipal_stillBlocked() {
        // Guard-regression case: product's storeId (999) is a diverged
        // store_id whose principal is 42 — same human as the buyer.
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(999L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(999L))).thenReturn(PrincipalResolution.allResolved(Map.of(999L, 42L)));
        CartService service = newService(productClient, sellerClient,
                Mockito.mock(CartItemRepository.class), Mockito.mock(CartCacheService.class));

        assertThatThrownBy(() -> service.addItem(42L, null, 1L, 1))
                .isInstanceOf(CartBusinessException.class);
    }

    @Test
    void resolvePrincipalsRpcFails_throwsServiceUnavailable_failClosed() {
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(200L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(Mockito.anyList()))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));
        CartService service = newService(productClient, sellerClient,
                Mockito.mock(CartItemRepository.class), Mockito.mock(CartCacheService.class));

        assertThatThrownBy(() -> service.addItem(100L, null, 1L, 1))
                .isInstanceOf(CartBusinessException.class);
    }

    @Test
    void diverged_buyerPrincipalResolvesToSameHuman_stillBlocked() {
        // Buyer-axis mirror of diverged_storeIdResolvesToBuyerPrincipal_stillBlocked (0b0d3d0bd
        // was the seller-axis fix; gateway PR#1585 means the caller-side accountId param can
        // now itself be a diverged buyer_id). Caller's accountId (777, a buyer_id) resolves to
        // principal 42; the product's storeId (200) resolves straight through to 42 too —
        // same human, must still be blocked even though 777 != 200 on the raw ids.
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(200L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 42L)));
        BuyerGrpcClient buyerClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerClient.resolvePrincipals(List.of(777L))).thenReturn(PrincipalResolution.allResolved(Map.of(777L, 42L)));
        CartService service = newService(productClient, sellerClient,
                Mockito.mock(CartItemRepository.class), Mockito.mock(CartCacheService.class), buyerClient);

        assertThatThrownBy(() -> service.addItem(777L, null, 1L, 1))
                .isInstanceOf(CartBusinessException.class);
    }

    @Test
    void resolveBuyerPrincipalsRpcFails_throwsServiceUnavailable_failClosed() {
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(200L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerClient.resolvePrincipals(Mockito.anyList()))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));
        CartService service = newService(productClient, sellerClient,
                Mockito.mock(CartItemRepository.class), Mockito.mock(CartCacheService.class), buyerClient);

        assertThatThrownBy(() -> service.addItem(100L, null, 1L, 1))
                .isInstanceOf(CartBusinessException.class);
    }

    // ==================== MergeCart resolve-to-principal (Task 3 review follow-up) ====================

    @Test
    void mergeCart_grandfathered_differentSellerAndBuyer_notSelfOwned() {
        CartItemEntity item = guestItem(1L, "guest-1", 200L, 10L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-1")).thenReturn(List.of(item));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-1", 100L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache);

        CartService.MergeCartResult result = service.mergeCart(100L, "guest-1");

        assertThat(result.removedSelfOwnedCount()).isEqualTo(0);
        Mockito.verify(repo, Mockito.never()).deleteSelfOwnedGuestItems(Mockito.any(), Mockito.any());
        Mockito.verify(repo, Mockito.times(1)).transferOwnerToMember(1L, 100L);
    }

    @Test
    void mergeCart_grandfathered_sameIdBothSides_selfOwnedRemoved() {
        CartItemEntity item = guestItem(2L, "guest-2", 100L, 20L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-2")).thenReturn(List.of(item));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(100L))).thenReturn(PrincipalResolution.allResolved(Map.of(100L, 100L)));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-2", 100L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache);

        CartService.MergeCartResult result = service.mergeCart(100L, "guest-2");

        assertThat(result.removedSelfOwnedCount()).isEqualTo(1);
        Mockito.verify(repo, Mockito.times(1)).deleteSelfOwnedGuestItems("guest-2", List.of(100L));
        Mockito.verify(repo, Mockito.never()).transferOwnerToMember(Mockito.any(), Mockito.any());
    }

    @Test
    void mergeCart_diverged_storeIdResolvesToBuyerPrincipal_selfOwnedRemoved() {
        // Guard-regression case: guest item's store_id (999) is a diverged id
        // whose principal (42) is the same human as the merging accountId (42) — the
        // pre-fix raw-id comparison (999 != 42) would have wrongly let this merge through.
        CartItemEntity item = guestItem(3L, "guest-3", 999L, 30L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-3")).thenReturn(List.of(item));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(999L))).thenReturn(PrincipalResolution.allResolved(Map.of(999L, 42L)));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-3", 42L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache);

        CartService.MergeCartResult result = service.mergeCart(42L, "guest-3");

        assertThat(result.removedSelfOwnedCount()).isEqualTo(1);
        Mockito.verify(repo, Mockito.times(1)).deleteSelfOwnedGuestItems("guest-3", List.of(999L));
        Mockito.verify(repo, Mockito.never()).transferOwnerToMember(Mockito.any(), Mockito.any());
    }

    @Test
    void mergeCart_selfOwnedGuestItems_deletedByResolvedStoreId_notRawAccountId() {
        // Regression guard for the store_id-rekey bug: guest item's store_id (9999) is a
        // re-onboarded store surrogate whose principal resolves to the SAME accountId as
        // the caller (self-owned) — but storeId != accountId. The pre-fix DELETE compared
        // store_id directly against the raw accountId and matched 0 rows (stale item lingers).
        long accountId = 1001L;
        long selfOwnedStoreId = 9999L;
        CartItemEntity item = guestItem(6L, "guest-6", selfOwnedStoreId, 60L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-6")).thenReturn(List.of(item));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(selfOwnedStoreId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(selfOwnedStoreId, accountId)));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-6", accountId)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache);

        CartService.MergeCartResult result = service.mergeCart(accountId, "guest-6");

        assertThat(result.removedSelfOwnedCount()).isEqualTo(1);
        Mockito.verify(repo, Mockito.times(1)).deleteSelfOwnedGuestItems("guest-6", List.of(selfOwnedStoreId));
        Mockito.verify(repo, Mockito.never()).deleteSelfOwnedGuestItems("guest-6", List.of(accountId));
    }

    @Test
    void mergeCart_diverged_buyerPrincipalResolvesToSameHuman_selfOwnedRemoved() {
        // Buyer-axis mirror of mergeCart_diverged_storeIdResolvesToBuyerPrincipal_selfOwnedRemoved.
        // The merging accountId (777) is itself a diverged buyer_id resolving to principal 42;
        // the guest item's storeId (100) resolves straight through to 42 — same human, must be
        // treated as self-owned even though the raw ids (777 vs 100) never match.
        CartItemEntity item = guestItem(7L, "guest-7", 100L, 70L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-7")).thenReturn(List.of(item));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(100L))).thenReturn(PrincipalResolution.allResolved(Map.of(100L, 42L)));
        BuyerGrpcClient buyerClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerClient.resolvePrincipals(List.of(777L))).thenReturn(PrincipalResolution.allResolved(Map.of(777L, 42L)));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-7", 777L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache, buyerClient);

        CartService.MergeCartResult result = service.mergeCart(777L, "guest-7");

        assertThat(result.removedSelfOwnedCount()).isEqualTo(1);
        Mockito.verify(repo, Mockito.times(1)).deleteSelfOwnedGuestItems("guest-7", List.of(100L));
        Mockito.verify(repo, Mockito.never()).transferOwnerToMember(Mockito.any(), Mockito.any());
    }

    @Test
    void mergeCart_batchesResolvePrincipalsOncePerMergeAcrossMultipleSellers() {
        CartItemEntity itemA = guestItem(4L, "guest-4", 200L, 40L);
        CartItemEntity itemB = guestItem(5L, "guest-4", 300L, 50L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-4")).thenReturn(List.of(itemA, itemB));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(Mockito.anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L, 300L, 300L)));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-4", 100L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache);

        service.mergeCart(100L, "guest-4");

        Mockito.verify(sellerClient, Mockito.times(1)).resolvePrincipals(Mockito.anyList());
    }

    // ==================== 未解析 principal ⇒ fail-closed（issue #2789 階段 B1） ====================
    //
    // seller-service / buyer-service 對「查無在架列」的 id（軟刪除的 stint 或不存在）
    // 回傳 code=0 並把輸入<b>原樣回吐</b>。舊寫法 getOrDefault(id, id) 看不出差別，
    // 於是「無法確認是不是同一個人」被靜默判成「不是同一個人」⇒ 自買防護對這些 id
    // 完全不生效。以下每一支都刻意用「server 真實會回的形狀」——map 帶 echo-back
    // 值、unresolved_ids 帶同一個 id——把舊行為（放行／照常合併）與新行為（拒絕）
    // 分開；把產品碼的 requireResolved/requireAllResolved 換回 getOrDefault，這幾支
    // 全部轉紅。

    private PrincipalResolution echoedBack(long... ids) {
        java.util.Map<Long, Long> echo = new java.util.LinkedHashMap<>();
        for (long id : ids) {
            echo.put(id, id); // server 的 COALESCE(s.account_id, input.id) 行為
        }
        return PrincipalResolution.of(echo, java.util.Arrays.stream(ids).boxed().toList());
    }

    @Test
    void addItem_storeUnresolved_failsClosedWithInvalidSku() {
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(200L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        // 200 查無在架賣家：map 仍有 entry（echo-back），unresolved_ids 才是真話
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L))).thenReturn(echoedBack(200L));
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        CartService service = newService(productClient, sellerClient, repo,
                Mockito.mock(CartCacheService.class));

        assertThatThrownBy(() -> service.addItem(100L, null, 1L, 1))
                .isInstanceOf(CartBusinessException.class)
                .extracting(e -> ((CartBusinessException) e).code())
                .isEqualTo(GrpcResponseCode.INVALID_SKU.code());
        // 舊行為是「200 != 100 ⇒ 不是自買 ⇒ 照常往下走」，會摸到 repo（max_limit 預檢
        // 的 findByBuyerIdAndSkuId）；fail-closed 之後一個 repo 呼叫都不該發生
        Mockito.verifyNoInteractions(repo);
    }

    @Test
    void addItem_buyerUnresolved_failsClosedWithInvalidSku() {
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.getProductSimple("1")).thenReturn(productResponse(200L));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerClient.resolvePrincipals(List.of(100L))).thenReturn(echoedBack(100L));
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        CartService service = newService(productClient, sellerClient, repo,
                Mockito.mock(CartCacheService.class), buyerClient);

        assertThatThrownBy(() -> service.addItem(100L, null, 1L, 1))
                .isInstanceOf(CartBusinessException.class)
                .extracting(e -> ((CartBusinessException) e).code())
                .isEqualTo(GrpcResponseCode.INVALID_SKU.code());
        Mockito.verifyNoInteractions(repo);
    }

    @Test
    void mergeCart_oneStoreUnresolved_rejectsWithoutDeletingOrMerging() {
        CartItemEntity resolvable = guestItem(11L, "guest-u1", 200L, 110L);
        CartItemEntity unresolvable = guestItem(12L, "guest-u1", 999L, 120L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-u1"))
                .thenReturn(List.of(resolvable, unresolvable));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L, 999L)))
                .thenReturn(PrincipalResolution.of(Map.of(200L, 200L, 999L, 999L), Set.of(999L)));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-u1", 100L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache);

        assertThatThrownBy(() -> service.mergeCart(100L, "guest-u1"))
                .isInstanceOf(CartBusinessException.class)
                .extracting(e -> ((CartBusinessException) e).code())
                .isEqualTo(GrpcResponseCode.INVALID_SKU.code());
        // fail-closed 的實質：一個副作用都不能發生（刪除尤其）
        Mockito.verify(repo, Mockito.never()).deleteSelfOwnedGuestItems(Mockito.any(), Mockito.any());
        Mockito.verify(repo, Mockito.never()).transferOwnerToMember(Mockito.any(), Mockito.any());
    }

    @Test
    void mergeCart_bothStoresUnresolved_deletesNothing() {
        // 兩個 store 同時未解析：這正是「回吐值被當成真 principal」最極端的形狀。
        // 買家 principal 100 對上兩個 echo-back（200 / 300）都不相等，舊行為是
        // 「都不是自買 ⇒ 兩筆全部合併進會員車」；本測試釘住新行為＝整筆拒絕、
        // 既不刪也不併。
        CartItemEntity a = guestItem(13L, "guest-u2", 200L, 130L);
        CartItemEntity b = guestItem(14L, "guest-u2", 300L, 140L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-u2")).thenReturn(List.of(a, b));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L, 300L)))
                .thenReturn(echoedBack(200L, 300L));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-u2", 100L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache);

        assertThatThrownBy(() -> service.mergeCart(100L, "guest-u2"))
                .isInstanceOf(CartBusinessException.class);

        Mockito.verify(repo, Mockito.never()).deleteSelfOwnedGuestItems(Mockito.any(), Mockito.any());
        Mockito.verify(repo, Mockito.never()).transferOwnerToMember(Mockito.any(), Mockito.any());
    }

    @Test
    void mergeCart_buyerUnresolved_rejectsWithoutDeletingOrMerging() {
        CartItemEntity item = guestItem(15L, "guest-u3", 200L, 150L);
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findNormalByGuestIdForMerge("guest-u3")).thenReturn(List.of(item));
        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.resolvePrincipals(List.of(200L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerClient.resolvePrincipals(List.of(777L))).thenReturn(echoedBack(777L));
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.tryMergeLock("guest-u3", 777L)).thenReturn(true);
        CartService service = newService(Mockito.mock(ProductGrpcClient.class), sellerClient, repo, cache, buyerClient);

        assertThatThrownBy(() -> service.mergeCart(777L, "guest-u3"))
                .isInstanceOf(CartBusinessException.class);

        Mockito.verify(repo, Mockito.never()).deleteSelfOwnedGuestItems(Mockito.any(), Mockito.any());
        Mockito.verify(repo, Mockito.never()).transferOwnerToMember(Mockito.any(), Mockito.any());
    }
}

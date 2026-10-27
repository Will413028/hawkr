package io.github.will413028.hawkr.cart.service;

import io.github.will413028.hawkr.cart.entity.CartItemEntity;
import io.github.will413028.hawkr.cart.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.cart.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.cart.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.cart.repository.CartItemRepository;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.Mockito;

import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * GetCart 帶出 store_code 的路徑（backend#2098）。
 *
 * <p>store_code 是店面連結的 canonical key；購物車每一個賣家分組的標題都是一個連往店面的
 * 連結，缺值時前端 {@code sellerPath()} 會退回 store_id、兩者都空就渲染 {@code href=""}。
 */
class CartServiceSellerMetaTest {

    private static final long BUYER_ID = 2623900520L;
    private static final long STORE_ID = 2685914682L;
    private static final String STORE_CODE = "2612345678";

    private CartItemEntity item() {
        CartItemEntity entity = new CartItemEntity();
        entity.setId(1L);
        entity.setBuyerId(BUYER_ID);
        entity.setStoreId(STORE_ID);
        entity.setSkuId(90001L);
        entity.setQuantity(1);
        entity.setAddedUnitPrice(100L);
        entity.setAddedListPrice(120L);
        return entity;
    }

    private CartService newService(CartCacheService cache, SellerGrpcClient sellerClient,
                                   CartItemRepository repo, ProductGrpcClient productClient) {
        return new CartService(repo, cache, productClient, sellerClient, Mockito.mock(BuyerGrpcClient.class));
    }

    private CartItemRepository repoWithSingleNormalItem() {
        CartItemRepository repo = Mockito.mock(CartItemRepository.class);
        Mockito.when(repo.findLockedByBuyerId(BUYER_ID)).thenReturn(List.of());
        return repo;
    }

    private ProductGrpcClient emptySkuInfoClient() {
        ProductGrpcClient productClient = Mockito.mock(ProductGrpcClient.class);
        Mockito.when(productClient.batchGetSkuCartInfo(Mockito.anyList())).thenReturn(Collections.emptyMap());
        return productClient;
    }

    @Test
    @DisplayName("賣家快取 Miss 時回源 seller-service，store_code 照樣帶出且回填快取")
    void cacheMiss_fallsBackToSellerService_andCarriesStoreCode() {
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.getCartCache(BUYER_ID, null)).thenReturn(List.of(item()));
        // Miss：Redis 沒有這個賣家（或條目殘缺被判為 Miss）
        Mockito.when(cache.getSellerMetaBatch(List.of(STORE_ID))).thenReturn(new HashMap<>());

        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.batchGetSellerMeta(List.of(STORE_ID))).thenReturn(Map.of(
                STORE_ID, SellerMeta.newBuilder()
                        .setSellerName("陳記蔬果")
                        .setCity("台北市")
                        .setDistrict("大安區")
                        .setStoreCode(STORE_CODE)
                        .build()));

        CartService service = newService(cache, sellerClient, repoWithSingleNormalItem(), emptySkuInfoClient());
        CartService.GetCartResult result = service.getCart(BUYER_ID, null);

        assertThat(result.groups()).hasSize(1);
        assertThat(result.groups().get(0).storeCode())
                .as("快取 Miss 是常態（TTL 到期 / profile evict），不能因此吐出空的 store_code")
                .isEqualTo(STORE_CODE);

        ArgumentCaptor<CartCacheService.SellerMetaCacheEntry> captor =
                ArgumentCaptor.forClass(CartCacheService.SellerMetaCacheEntry.class);
        Mockito.verify(cache).setSellerMeta(Mockito.eq(STORE_ID), captor.capture());
        assertThat(captor.getValue().storeCode()).isEqualTo(STORE_CODE);
    }

    @Test
    @DisplayName("賣家快取 Hit 時不必回源，store_code 直接來自快取")
    void cacheHit_usesCachedStoreCode_withoutRpc() {
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.getCartCache(BUYER_ID, null)).thenReturn(List.of(item()));
        Map<Long, CartCacheService.SellerMetaCacheEntry> cached = new HashMap<>();
        cached.put(STORE_ID, new CartCacheService.SellerMetaCacheEntry(
                "陳記蔬果", "seller/logo/a.jpg", "台北市", "大安區", STORE_CODE));
        Mockito.when(cache.getSellerMetaBatch(List.of(STORE_ID))).thenReturn(cached);

        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);

        CartService service = newService(cache, sellerClient, repoWithSingleNormalItem(), emptySkuInfoClient());
        CartService.GetCartResult result = service.getCart(BUYER_ID, null);

        assertThat(result.groups().get(0).storeCode()).isEqualTo(STORE_CODE);
        Mockito.verify(sellerClient, Mockito.never()).batchGetSellerMeta(Mockito.anyList());
    }

    @Test
    @DisplayName("seller-service 重試後仍失敗才降級：購物車照回，但 store_code 是空字串（最後手段）")
    void sellerServiceFullyDown_stillReturnsCart_withBlankStoreCode() {
        CartCacheService cache = Mockito.mock(CartCacheService.class);
        Mockito.when(cache.getCartCache(BUYER_ID, null)).thenReturn(List.of(item()));
        Mockito.when(cache.getSellerMetaBatch(List.of(STORE_ID))).thenReturn(new HashMap<>());

        SellerGrpcClient sellerClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerClient.batchGetSellerMeta(List.of(STORE_ID))).thenReturn(Collections.emptyMap());

        CartService service = newService(cache, sellerClient, repoWithSingleNormalItem(), emptySkuInfoClient());
        CartService.GetCartResult result = service.getCart(BUYER_ID, null);

        assertThat(result.groups()).hasSize(1);
        assertThat(result.groups().get(0).storeCode()).isEmpty();
        // 殘缺的 meta 不得寫回快取，否則會被續命 24h
        Mockito.verify(cache, Mockito.never()).setSellerMeta(Mockito.anyLong(), Mockito.any());
    }
}

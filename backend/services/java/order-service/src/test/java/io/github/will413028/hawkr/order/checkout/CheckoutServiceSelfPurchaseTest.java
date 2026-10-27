package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ResolvedItem;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.SkuShippingOption;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 結帳自買防護。
 *
 * <p><b>兩側都要解析</b>（issue #2789 階段 B2）：{@code buyerId} 參數是 <b>buyer_id 代理鍵</b>，
 * seller 軸解出來的是 <b>account_id</b>。舊碼直接拿 raw {@code buyerId} 比 account_id ——
 * 今天只因 grandfathered {@code buyer_id == account_id} 而碰巧成立，買家 re-key 後就永遠比不中
 * ⇒ 防護靜默關閉。{@link #rekeyedBuyer_resolvesToSellerPrincipal_blocked} 是那個 root cause
 * 的鑑別測試（把買家軸解析拿掉就轉紅）。
 */
class CheckoutServiceSelfPurchaseTest {

    // ResolvedItem field order (CheckoutValidator.java:260-277): skuId, skuName,
    // quantity, unitPrice, listPrice, stockQty, lengthCm, widthCm, heightCm,
    // weightGram, storeId, productId, productName, shippingOptions,
    // imageObjectKey, isActive — storeId is position 11, NOT 6
    // (verify against the record before reusing this helper elsewhere).
    private ResolvedItem item(long skuId, long storeId) {
        return new ResolvedItem(
                skuId, "sku-" + skuId, 1, 100L, 100L, 999L,
                10, 10, 10, 1000L, storeId, "p-" + skuId, "product-" + skuId,
                List.of(new SkuShippingOption("3001", "3201", 0L, "3102")),
                "img", true);
    }

    @Test
    void grandfathered_differentSellerAndBuyer_allowed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(100L, 100L));

        service.enforceNoSelfPurchaseForTest(100L, List.of(item(1L, 200L)));
        // no exception = allowed
    }

    @Test
    void grandfathered_sameIdBothSides_blocked() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(100L))).thenReturn(PrincipalResolution.allResolved(Map.of(100L, 100L)));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(100L, 100L));

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(100L, List.of(item(1L, 100L))))
                .isInstanceOf(OrderBusinessException.class);
    }

    @Test
    void diverged_storeIdResolvesToBuyerPrincipal_stillBlocked() {
        // The guard-regression case: item's storeId (999) is a diverged
        // store_id whose principal is 42 — the same human as the buyer.
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(999L))).thenReturn(PrincipalResolution.allResolved(Map.of(999L, 42L)));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(42L, 42L));

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(42L, List.of(item(1L, 999L))))
                .isInstanceOf(OrderBusinessException.class);
    }

    /**
     * <b>本波新增的 root cause 鑑別測試（issue #2789 階段 B2）</b>。
     *
     * <p>買家已 re-key：{@code buyer_id=7001} 的登入 principal 是 {@code account_id=42}
     * （兩者<b>不相等</b>，正是 grandfather 結束後的常態）。賣家 {@code store_id=999} 的
     * principal 同樣是 42 ⇒ <b>同一個人</b>在自己的店買自己的東西，必須擋。
     *
     * <p>舊碼拿 raw {@code 7001} 去比 {@code 42}：比不中 ⇒ <b>靜默放行</b>。
     * ⇒ 把 {@code buyerGrpcClient.resolvePrincipals} 那一步拿掉、改回用 raw buyerId 比較，
     * 這支就會轉紅；其他幾支因為 {@code buyer_id == account_id} 全都照過。
     */
    @Test
    void rekeyedBuyer_resolvesToSellerPrincipal_blocked() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(999L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(999L, 42L)));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(7001L, 42L));

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(7001L, List.of(item(1L, 999L))))
                .isInstanceOf(OrderBusinessException.class);
    }

    /**
     * 反面（陰性）：買家 re-key 之後解出來的 principal 與賣家 principal <b>不同</b> ⇒ 必須放行。
     * 少了這支，上一支可以被「一律拒絕」這種假修法騙過去。
     */
    @Test
    void rekeyedBuyer_differentPrincipal_allowed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(999L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(999L, 42L)));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(7001L, 43L));

        service.enforceNoSelfPurchaseForTest(7001L, List.of(item(1L, 999L)));
        // no exception = allowed
    }

    @Test
    void resolvePrincipalsRpcFails_throwsServiceUnavailable_failClosed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(Mockito.anyList()))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(100L, 100L));

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(100L, List.of(item(1L, 200L))))
                .isInstanceOf(OrderBusinessException.class);
    }

    /** 買家軸 RPC 失敗同樣 fail-closed（賣家軸正常，單獨釘住買家軸那一次呼叫）。 */
    @Test
    void buyerResolvePrincipalsRpcFails_throwsServiceUnavailable_failClosed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(Mockito.anyList()))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));
        CheckoutService service = newService(sellerGrpcClient, buyerGrpcClient);

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(100L, List.of(item(1L, 200L))))
                .isInstanceOf(OrderBusinessException.class);
    }


    // ==================== 未解析 principal ⇒ fail-closed（issue #2789 階段 B1/B2） ====================

    @Test
    void storeUnresolved_failsClosed_notSilentlyAllowed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        // 999 查無在架賣家：map 仍帶 echo-back（server 的 COALESCE 行為），unresolved_ids 才是真話。
        // 舊碼 getOrDefault(999, 999) ⇒ 999 != 42 ⇒「不是自買」⇒ 放行；本測試釘住新行為＝拒絕。
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(999L)))
                .thenReturn(PrincipalResolution.of(Map.of(999L, 999L), List.of(999L)));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(42L, 42L));

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(42L, List.of(item(1L, 999L))))
                .isInstanceOf(OrderBusinessException.class);
    }

    @Test
    void oneOfManyStoresUnresolved_failsClosedForWholeCheckout() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L, 999L)))
                .thenReturn(PrincipalResolution.of(Map.of(200L, 200L, 999L, 999L), List.of(999L)));
        CheckoutService service = newService(sellerGrpcClient, buyerResolving(42L, 42L));

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(
                42L, List.of(item(1L, 200L), item(2L, 999L))))
                .isInstanceOf(OrderBusinessException.class);
    }

    /**
     * 買家側未解析（map 帶 echo-back、unresolved_ids 說了實話）同樣 fail-closed。
     * 兩軸必須同批處理：只修賣家軸，買家軸的 echo-back 仍會讓「無法確認」被當成「不是同一個人」。
     */
    @Test
    void buyerUnresolved_failsClosed_notSilentlyAllowed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(7001L)))
                .thenReturn(PrincipalResolution.of(Map.of(7001L, 7001L), List.of(7001L)));
        CheckoutService service = newService(sellerGrpcClient, buyerGrpcClient);

        assertThatThrownBy(() -> service.enforceNoSelfPurchaseForTest(7001L, List.of(item(1L, 200L))))
                .isInstanceOf(OrderBusinessException.class);
    }

    /** 買家軸解析成功的 stub（{@code buyerId → principal}）。 */
    private BuyerGrpcClient buyerResolving(long buyerId, long principal) {
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, principal)));
        return buyerGrpcClient;
    }

    private CheckoutService newService(SellerGrpcClient sellerGrpcClient, BuyerGrpcClient buyerGrpcClient) {
        return new CheckoutService(null, sellerGrpcClient, buyerGrpcClient, null, null, null, null, null, new OrderPolicyResolver(), null);
    }
}

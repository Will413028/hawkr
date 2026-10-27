package io.github.will413028.hawkr.coupon.grpc;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.coupon.exception.CouponBusinessException;
import io.github.will413028.hawkr.coupon.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.coupon.grpc.client.SellerGrpcClient;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class CouponGrpcServiceSelfIssueTest {

    @Test
    void grandfathered_differentSellerAndBuyer_allowed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(100L, 101L))).thenReturn(PrincipalResolution.allResolved(Map.of(100L, 100L, 101L, 101L)));

        assertThatCode(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 200L, List.of(100L, 101L))).doesNotThrowAnyException();
    }

    @Test
    void grandfathered_sellerTargetsSelf_blocked() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(200L, 101L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L, 101L, 101L)));

        assertThatThrownBy(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 200L, List.of(200L, 101L)))
                .isInstanceOf(CouponBusinessException.class);
    }

    @Test
    void diverged_sellerSideOnly_storeIdResolvesToOwnPrincipal_stillBlocked() {
        // Guard-regression case: the acting seller's store_id (999, what
        // sellerCreateTemplate reads off SellerCreateTemplateRequest.store_id
        // after gateway resolution) resolves to principal 42, and the
        // designated buyer_id 42 happens to already equal that principal
        // (grandfathered buyer side, no divergence there) — same human,
        // still blocked.
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(999L))).thenReturn(PrincipalResolution.allResolved(Map.of(999L, 42L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(42L, 101L))).thenReturn(PrincipalResolution.allResolved(Map.of(42L, 42L, 101L, 101L)));

        assertThatThrownBy(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 999L, List.of(42L, 101L)))
                .isInstanceOf(CouponBusinessException.class);
    }

    /**
     * account_id/store_id 稽核 2026-08-01 Bug 4 的 core regression case: a seller
     * whose account_id (200) diverges from their own current buyer_id (777) —
     * before the fix, buyerIds ([777, 101]) was compared directly
     * against sellerPrincipal (200) without ever resolving the buyer side, so
     * 777 never matched 200 and the self-issue guard silently let the seller
     * designate a DESIGNATED coupon to their own current buyer_id. After the
     * fix, buyerGrpcClient.resolvePrincipals([777, 101]) resolves 777 -> 200
     * (same underlying account), matching sellerPrincipal and blocking it.
     */
    @Test
    void diverged_buyerSideOnly_buyerIdResolvesToSellerOwnAccountId_stillBlocked() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(777L, 101L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(777L, 200L, 101L, 101L)));

        assertThatThrownBy(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 200L, List.of(777L, 101L)))
                .isInstanceOf(CouponBusinessException.class);
    }

    @Test
    void diverged_bothSidesButDifferentAccounts_allowed() {
        // Both sides diverge from their raw ids, but resolve to *different*
        // underlying accounts — must not be blocked (proves the fix compares
        // resolved principals, not raw ids naively).
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(999L))).thenReturn(PrincipalResolution.allResolved(Map.of(999L, 42L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(777L, 101L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(777L, 200L, 101L, 101L)));

        assertThatCode(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 999L, List.of(777L, 101L))).doesNotThrowAnyException();
    }

    @Test
    void sellerResolvePrincipalsRpcFails_propagatesForOuterCatchAllToHandle() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(Mockito.anyList()))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);

        assertThatThrownBy(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 200L, List.of(101L)))
                .isInstanceOf(StatusRuntimeException.class);
    }

    @Test
    void buyerResolvePrincipalsRpcFails_propagatesForOuterCatchAllToHandle() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L))).thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        Mockito.when(buyerGrpcClient.resolvePrincipals(Mockito.anyList()))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));

        assertThatThrownBy(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 200L, List.of(101L)))
                .isInstanceOf(StatusRuntimeException.class);
    }

    // ==================== 未解析 principal ⇒ fail-closed（issue #2789 階段 B1） ====================
    //
    // seller/buyer service 對查無在架列的 id 回 code=0 並把輸入原樣回吐。舊寫法
    // getOrDefault(id, id) 讓「無法確認」變成「不是同一個人」⇒ 自發自用防護對這些 id
    // 靜默失效。把產品碼的 requireResolved / requireAllResolved 換回 getOrDefault，
    // 以下兩支會轉紅（回到「不拋例外」）。

    private static PrincipalResolution echoedBack(java.util.Map<Long, Long> principals, Long... unresolved) {
        return PrincipalResolution.of(principals, List.of(unresolved));
    }

    @Test
    void sellerStoreUnresolved_failsClosed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L)))
                .thenReturn(echoedBack(Map.of(200L, 200L), 200L));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);

        assertThatThrownBy(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 200L, List.of(100L, 101L)))
                .isInstanceOf(CouponBusinessException.class);
        // 賣家側都還沒確認就擋掉，根本不該去問買家側
        Mockito.verifyNoInteractions(buyerGrpcClient);
    }

    @Test
    void designatedBuyerUnresolved_failsClosed() {
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.resolvePrincipals(List.of(200L)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(200L, 200L)));
        BuyerGrpcClient buyerGrpcClient = Mockito.mock(BuyerGrpcClient.class);
        // 101 查無在架買家：回吐 101，舊碼判 101 != 200 ⇒ 放行
        Mockito.when(buyerGrpcClient.resolvePrincipals(List.of(100L, 101L)))
                .thenReturn(echoedBack(Map.of(100L, 100L, 101L, 101L), 101L));

        assertThatThrownBy(() -> CouponGrpcService.enforceNoSelfIssueForTest(
                sellerGrpcClient, buyerGrpcClient, 200L, List.of(100L, 101L)))
                .isInstanceOf(CouponBusinessException.class);
    }
}

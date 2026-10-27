package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 賣家停權收斂點 backstop 測試（03 §5.1a.1 / 46 §6.2 §11，首次接線 A5）。
 *
 * <p>驗證 {@code CheckoutService.requireSellerAcceptingOrders(...)}：validate()/preview()/confirm()
 * 三處呼叫點共用同一條規則，故只需對規則本身 + 其與 sellerGrpcClient 的接線各驗一次
 * （三個 caller 的呼叫序完全相同，見 CheckoutService.java 內三處 requireSellerAcceptingOrders 呼叫）。
 */
class CheckoutServiceSellerSuspensionTest {

    private static final long STORE_ID = 200L;

    @Test
    void suspendedSeller_canAcceptOrdersFalse_throws9241() {
        CheckoutService service = newServiceWithSellerClient(Mockito.mock(SellerGrpcClient.class));
        SellerMeta suspended = SellerMeta.newBuilder().setCanAcceptOrders(false).build();

        assertThatThrownBy(() -> service.requireSellerAcceptingOrdersForTest(suspended))
                .isInstanceOf(OrderBusinessException.class)
                .satisfies(e -> {
                    OrderBusinessException biz = (OrderBusinessException) e;
                    org.assertj.core.api.Assertions.assertThat(biz.code()).isEqualTo(9241);
                });
    }

    @Test
    void activeSeller_canAcceptOrdersTrue_allowed() {
        CheckoutService service = newServiceWithSellerClient(Mockito.mock(SellerGrpcClient.class));
        SellerMeta active = SellerMeta.newBuilder().setCanAcceptOrders(true).build();

        assertThatCode(() -> service.requireSellerAcceptingOrdersForTest(active))
                .doesNotThrowAnyException();
    }

    @Test
    void canManageProductsButNotAcceptOrders_throws9241() {
        // 店家還不可公開曝光（KYC 或 PayUni 未核准、缺 logo）時賣家可以先上架商品，但結帳只看 can_accept_orders。
        CheckoutService service = newServiceWithSellerClient(Mockito.mock(SellerGrpcClient.class));
        SellerMeta notYetExposable = SellerMeta.newBuilder().setCanManageProducts(true).setCanAcceptOrders(false).build();

        assertThatThrownBy(() -> service.requireSellerAcceptingOrdersForTest(notYetExposable))
                .isInstanceOf(OrderBusinessException.class)
                .satisfies(e -> org.assertj.core.api.Assertions.assertThat(((OrderBusinessException) e).code()).isEqualTo(9241));
    }

    @Test
    void sellerMetaQueryFails_exceptionPropagates_failClosed() {
        // 鏡射 validate()/preview()/confirm() 的實際呼叫序：
        // sellerGrpcClient.getSellerMeta(storeId) 失敗時，例外必須原樣往外拋，
        // 不可被靜默吞掉導致 checkout 誤放行停權賣家（fail-closed，46 §6.1 對稱互補）。
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.getSellerMeta(STORE_ID))
                .thenThrow(new StatusRuntimeException(Status.UNAVAILABLE));
        CheckoutService service = newServiceWithSellerClient(sellerGrpcClient);

        assertThatThrownBy(() -> service.requireSellerAcceptingOrdersByStoreIdForTest(STORE_ID))
                .isInstanceOf(StatusRuntimeException.class);
    }

    @Test
    void sellerMetaNotFound_exceptionPropagates_failClosed() {
        // getSellerMeta 查無資料時拋 OrderBusinessException(NOT_FOUND)（既有硬依賴語意），
        // 同樣不可被吞掉。
        SellerGrpcClient sellerGrpcClient = Mockito.mock(SellerGrpcClient.class);
        Mockito.when(sellerGrpcClient.getSellerMeta(STORE_ID))
                .thenThrow(new OrderBusinessException(io.github.will413028.hawkr.common.utils.code.GrpcResponseCode.NOT_FOUND));
        CheckoutService service = newServiceWithSellerClient(sellerGrpcClient);

        assertThatThrownBy(() -> service.requireSellerAcceptingOrdersByStoreIdForTest(STORE_ID))
                .isInstanceOf(OrderBusinessException.class);
    }

    private CheckoutService newServiceWithSellerClient(SellerGrpcClient sellerGrpcClient) {
        return new CheckoutService(null, sellerGrpcClient, null, null, null, null, null, null, new OrderPolicyResolver(), null);
    }
}

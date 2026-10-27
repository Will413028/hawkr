package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.buyer.grpc.AddressInfo;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ValidationResult;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Assessment;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Check;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.CheckOutcome;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Decision;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Probe;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Status;
import io.github.will413028.hawkr.order.checkout.client.CheckoutCouponGrpcClient;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.grpc.CheckoutItem;
import io.github.will413028.hawkr.order.grpc.OrderUberQuoteData;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.InvoiceGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.service.OrderService;
import io.github.will413028.hawkr.product.grpc.BatchGetProductForCheckoutResponse;
import io.github.will413028.hawkr.product.grpc.ProductForCheckoutInfo;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.data.redis.core.StringRedisTemplate;

import java.util.ArrayList;
import java.util.List;
import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@code POST /app/checkout/uber_quote} 的判定<b>來自閘門</b>，不是自己打一次 shipping。
 *
 * <h2>為什麼這一支必須存在</h2>
 * 「買家看得到的選項」與「按下去會不會過」若來自兩次獨立求值，就會出現
 * 報價說 {@code available=true}、結帳鏈回 {@code 9503} 的組合——那正是本波要消滅的形狀。
 * {@code UberOrderEligibilityGuardTest} 證明「閘門會判」，本支證明「報價端點<b>去問它</b>」，
 * 兩者缺一都會留下一個寫得很好卻沒人用的 guard（{@code can_accept_orders} 踩過的形狀，46 §11）。
 *
 * <h2>🔴 也斷言 shipping 只被打一次</h2>
 * 「走閘門」若寫成「先問閘門、再自己打一次 shipping 拿 quote_id」，功能看起來完全正常，
 * 但每次結帳都對 Uber 多送一次報價（E1 有 100 req/hr 上限）。只驗回傳值的測試看不到它。
 */
class CheckoutUberQuoteWiringTest {

    private static final long BUYER_ID = 2623900520L;
    private static final long STORE_ID = 2685914682L;
    private static final String SKU_ID = "5001";
    private static final String ADDRESS_ID = "7788";
    /** 商品沒有任何 shipping option ⇒ inferredTemperature 落在最冷的 3203。硬寫字面值。 */
    private static final String FROZEN = "3203";

    private ProductGrpcClient productGrpcClient;
    private BuyerGrpcClient buyerGrpcClient;
    private ShippingGrpcClient shippingGrpcClient;
    private CheckoutValidator checkoutValidator;
    private CheckoutService service;

    @BeforeEach
    void setUp() {
        productGrpcClient = mock(ProductGrpcClient.class);
        buyerGrpcClient = mock(BuyerGrpcClient.class);
        shippingGrpcClient = mock(ShippingGrpcClient.class);
        checkoutValidator = mock(CheckoutValidator.class);

        when(productGrpcClient.batchGetProductForCheckout(anyList()))
                .thenReturn(BatchGetProductForCheckoutResponse.newBuilder()
                        .putData(SKU_ID, ProductForCheckoutInfo.newBuilder()
                                .setSkuName("蘋果")
                                .setProductName("蘋果禮盒")
                                .setStoreId(String.valueOf(STORE_ID))
                                .setListPrice("500")
                                .setStockQty("10")
                                .setIsActive(true)
                                .build())
                        .build());

        // 跨賣家／溫層都通過，且交集含本車的溫層——否則 uberQuote 會在打閘門之前就短路
        when(checkoutValidator.validate(anyList(), any()))
                .thenReturn(new ValidationResult(true, Set.of(FROZEN), List.of(), List.of(), 0));

        when(buyerGrpcClient.getAddress(anyLong(), anyLong()))
                .thenReturn(AddressInfo.newBuilder()
                        .setReceiverName("王小明")
                        .setReceiverPhone("0912345678")
                        .setCity("台北市")
                        .setDistrict("中正區")
                        .setDetailAddress("測試路1號3樓")
                        .build());

        service = new CheckoutService(
                productGrpcClient,
                mock(SellerGrpcClient.class),
                buyerGrpcClient,
                shippingGrpcClient,
                mock(CheckoutCouponGrpcClient.class),
                mock(InvoiceGrpcClient.class),
                checkoutValidator,
                mock(OrderService.class),
                new OrderPolicyResolver(),
                mock(StringRedisTemplate.class));
    }

    private static List<CheckoutItem> items() {
        return List.of(CheckoutItem.newBuilder()
                .setSkuId(SKU_ID).setProductId("9001").setQuantity("2").build());
    }

    private static Decision decisionOf(boolean allowed, Status status, Assessment assessment) {
        List<CheckOutcome> outcomes = new ArrayList<>();
        for (Check check : Check.values()) {
            outcomes.add(new CheckOutcome(check, status, "test"));
        }
        return new Decision(true, allowed, List.copyOf(outcomes), assessment);
    }

    @Test
    @DisplayName("🔴 陽性：閘門放行 ⇒ available=true，且 quote_id／運費來自閘門的 assessment")
    void allowedDecisionYieldsQuote() {
        when(checkoutValidator.assessUberQuote(any()))
                .thenReturn(decisionOf(true, Status.PASSED,
                        Assessment.eligible("dqt_abc", "115", "TWD", "2026-09-12T08:30:00Z")));

        OrderUberQuoteData data = service.uberQuote(BUYER_ID, ADDRESS_ID, items());

        assertThat(data.getAvailable()).isTrue();
        assertThat(data.getQuoteId()).isEqualTo("dqt_abc");
        assertThat(data.getFeeAmount()).isEqualTo("115");
        assertThat(data.getCurrency()).isEqualTo("TWD");
        assertThat(data.getExpiresAt()).isEqualTo("2026-09-12T08:30:00Z");

        // 🔴 一次都不自己打 shipping：報價只能從閘門那一次呼叫裡出來
        verify(shippingGrpcClient, never()).getUberQuoteForCheckout(any());
        verify(shippingGrpcClient, never()).validateUberQuoteForCheckout(any());
    }

    @Test
    @DisplayName("🔴 probe 逐欄：買家／賣家 id、溫層、地址三欄、逐件尺寸都擺對位置")
    void probeCarriesEveryFieldInPlace() {
        when(checkoutValidator.assessUberQuote(any()))
                .thenReturn(decisionOf(true, Status.PASSED,
                        Assessment.eligible("dqt_abc", "115", "TWD", "2026-09-12T08:30:00Z")));

        service.uberQuote(BUYER_ID, ADDRESS_ID, items());

        ArgumentCaptor<Probe> captor = ArgumentCaptor.forClass(Probe.class);
        verify(checkoutValidator).assessUberQuote(captor.capture());
        Probe probe = captor.getValue();

        assertThat(probe.buyerId()).isEqualTo(BUYER_ID);
        assertThat(probe.sellerStoreId()).isEqualTo(STORE_ID);
        assertThat(probe.deliveryTemp()).isEqualTo(FROZEN);
        assertThat(probe.receiverName()).isEqualTo("王小明");
        assertThat(probe.receiverPhone()).isEqualTo("0912345678");
        assertThat(probe.county()).isEqualTo("台北市");
        assertThat(probe.district()).isEqualTo("中正區");
        assertThat(probe.detailAddress()).isEqualTo("測試路1號3樓");
        assertThat(probe.items()).hasSize(1);
        assertThat(probe.items().get(0).skuId()).isEqualTo(5001L);
        assertThat(probe.items().get(0).quantity()).isEqualTo(2);
        assertThat(probe.items().get(0).unitPrice()).isEqualTo(500L);
        // 報價階段不落任何狀態
        assertThat(probe.quoteId()).isEmpty();
        assertThat(probe.accept()).isFalse();
    }

    @Test
    @DisplayName("🔴 陰性：閘門拒絕 ⇒ available=false ＋ shipping 的原始理由碼（quote_id／運費一律留空）")
    void deniedDecisionYieldsNamedReason() {
        when(checkoutValidator.assessUberQuote(any()))
                .thenReturn(decisionOf(false, Status.DENIED_BY_PROVIDER,
                        Assessment.unavailable("UBER_STORE_NOT_MAPPED")));

        OrderUberQuoteData data = service.uberQuote(BUYER_ID, ADDRESS_ID, items());

        assertThat(data.getAvailable()).isFalse();
        assertThat(data.getUnavailableReason()).isEqualTo("UBER_STORE_NOT_MAPPED");
        // 用 fee=0 表示不可用會與「0 元運費」（平台補貼）逐位元無法區分
        assertThat(data.getQuoteId()).isEmpty();
        assertThat(data.getFeeAmount()).isEmpty();
    }

    @Test
    @DisplayName("🔴 閘門取不到證據（assessment=null）⇒ 回具名的 ASSESSOR_UNAVAILABLE，不是空字串")
    void deniedWithoutAssessmentStillNamesAReason() {
        when(checkoutValidator.assessUberQuote(any()))
                .thenReturn(decisionOf(false, Status.DENIED_NO_EVIDENCE, null));

        OrderUberQuoteData data = service.uberQuote(BUYER_ID, ADDRESS_ID, items());

        assertThat(data.getAvailable()).isFalse();
        assertThat(data.getUnavailableReason()).isEqualTo("UBER_QUOTE_ASSESSOR_UNAVAILABLE");
    }

    @Test
    @DisplayName("地址查不到 ⇒ ADDRESS_NOT_FOUND，且不浪費一次閘門呼叫")
    void missingAddressShortCircuitsBeforeGuard() {
        when(buyerGrpcClient.getAddress(anyLong(), anyLong())).thenReturn(null);

        OrderUberQuoteData data = service.uberQuote(BUYER_ID, ADDRESS_ID, items());

        assertThat(data.getAvailable()).isFalse();
        assertThat(data.getUnavailableReason()).isEqualTo("ADDRESS_NOT_FOUND");
        verify(checkoutValidator, never()).assessUberQuote(any());
    }
}

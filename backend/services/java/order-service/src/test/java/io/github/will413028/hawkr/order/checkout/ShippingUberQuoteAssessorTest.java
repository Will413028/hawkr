package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Assessment;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Item;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Probe;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.shipping.grpc.GetUberQuoteForCheckoutRequest;
import io.github.will413028.hawkr.shipping.grpc.GetUberQuoteForCheckoutResponse;
import io.github.will413028.hawkr.shipping.grpc.UberQuoteData;
import io.github.will413028.hawkr.shipping.grpc.ValidateUberQuoteData;
import io.github.will413028.hawkr.shipping.grpc.ValidateUberQuoteForCheckoutRequest;
import io.github.will413028.hawkr.shipping.grpc.ValidateUberQuoteForCheckoutResponse;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@link ShippingUberQuoteAssessor}：Probe ↔ wire 的形狀轉換。
 *
 * <h2>🔴 fixture 不是手寫的</h2>
 * 每一個欄位名與型別都來自 {@code shipping.proto} 生成的 builder——
 * 「我覺得對方會收成這樣」的手寫 map 必然自己跟自己對、必然全綠。
 * 欄位錯位（買家與賣家 id 對調、地址三欄擺錯）只有逐欄斷言抓得到。
 */
class ShippingUberQuoteAssessorTest {

    private static final long BUYER_ID = 2623900520L;
    private static final long STORE_ID = 2685914682L;

    private final ShippingGrpcClient shippingGrpcClient = mock(ShippingGrpcClient.class);
    private final ShippingUberQuoteAssessor assessor = new ShippingUberQuoteAssessor(shippingGrpcClient);

    private static Probe quoteProbe() {
        return Probe.forQuote(
                BUYER_ID, STORE_ID, "3202",
                "王小明", "0912345678",
                "台北市", "中正區", "測試路1號3樓",
                List.of(new Item(5001L, "蘋果", 2, 10, 12, 8, 1500L, 500L)));
    }

    private static GetUberQuoteForCheckoutResponse quoteResponse(UberQuoteData data) {
        return GetUberQuoteForCheckoutResponse.newBuilder()
                .setCode(0).setMessage("success").setData(data).build();
    }

    private static ValidateUberQuoteForCheckoutResponse validateResponse(ValidateUberQuoteData data) {
        return ValidateUberQuoteForCheckoutResponse.newBuilder()
                .setCode(0).setMessage("success").setData(data).build();
    }

    // ===== quoteId 空 ⇒ GetUberQuoteForCheckout =====

    @Test
    @DisplayName("🔴 沒有既有 quote ⇒ 打 GetUberQuoteForCheckout，且逐欄擺對位置")
    void blankQuoteIdGoesToGetQuoteWithEveryFieldInPlace() {
        when(shippingGrpcClient.getUberQuoteForCheckout(any()))
                .thenReturn(quoteResponse(UberQuoteData.newBuilder()
                        .setAvailable(true)
                        .setQuoteId("dqt_abc")
                        .setFeeAmount("115")
                        .setCurrency("TWD")
                        .setExpiresAt("2026-09-12T08:30:00Z")
                        .build()));

        Assessment result = assessor.assess(quoteProbe());

        ArgumentCaptor<GetUberQuoteForCheckoutRequest> captor =
                ArgumentCaptor.forClass(GetUberQuoteForCheckoutRequest.class);
        verify(shippingGrpcClient).getUberQuoteForCheckout(captor.capture());
        verify(shippingGrpcClient, never()).validateUberQuoteForCheckout(any());

        GetUberQuoteForCheckoutRequest sent = captor.getValue();
        // 🔴 買家與賣家 id 對調在型別上完全合法（都是 int64），只有逐欄斷言抓得到
        assertThat(sent.getBuyerId()).isEqualTo(BUYER_ID);
        assertThat(sent.getSellerStoreId()).isEqualTo(STORE_ID);
        assertThat(sent.getDeliveryTemp()).isEqualTo("3202");
        assertThat(sent.getDropoff().getReceiverName()).isEqualTo("王小明");
        assertThat(sent.getDropoff().getReceiverPhone()).isEqualTo("0912345678");
        // 🔴 三欄分開，不得攤平（shipping.proto 逐字禁止讓 shipping 反解字串）
        assertThat(sent.getDropoff().getCounty()).isEqualTo("台北市");
        assertThat(sent.getDropoff().getDistrict()).isEqualTo("中正區");
        assertThat(sent.getDropoff().getDetailAddress()).isEqualTo("測試路1號3樓");
        assertThat(sent.getItemsCount()).isEqualTo(1);
        assertThat(sent.getItems(0).getSkuId()).isEqualTo("5001");
        assertThat(sent.getItems(0).getName()).isEqualTo("蘋果");
        assertThat(sent.getItems(0).getQuantity()).isEqualTo("2");
        assertThat(sent.getItems(0).getLengthCm()).isEqualTo("10");
        assertThat(sent.getItems(0).getWidthCm()).isEqualTo("12");
        assertThat(sent.getItems(0).getHeightCm()).isEqualTo("8");
        assertThat(sent.getItems(0).getWeightGram()).isEqualTo("1500");
        assertThat(sent.getItems(0).getUnitPrice()).isEqualTo("500");

        assertThat(result.eligible()).isTrue();
        assertThat(result.quoteId()).isEqualTo("dqt_abc");
        assertThat(result.feeAmount()).isEqualTo("115");
        assertThat(result.currency()).isEqualTo("TWD");
        assertThat(result.expiresAt()).isEqualTo("2026-09-12T08:30:00Z");
    }

    @Test
    @DisplayName("available=false ⇒ 原樣帶回 unavailable_reason（不翻譯、不美化）")
    void unavailableQuoteKeepsProviderReasonVerbatim() {
        when(shippingGrpcClient.getUberQuoteForCheckout(any()))
                .thenReturn(quoteResponse(UberQuoteData.newBuilder()
                        .setAvailable(false)
                        .setUnavailableReason("UBER_STORE_NOT_MAPPED")
                        .build()));

        Assessment result = assessor.assess(quoteProbe());

        assertThat(result.eligible()).isFalse();
        assertThat(result.unavailableReason()).isEqualTo("UBER_STORE_NOT_MAPPED");
        assertThat(result.quoteId()).isEmpty();
        assertThat(result.feeAmount()).isEmpty();
    }

    @Test
    @DisplayName("🔴 下游回非零 code（契約被改過）⇒ 具名的『問不到』，不是猜一個業務理由")
    void nonZeroCodeBecomesAssessorUnavailable() {
        when(shippingGrpcClient.getUberQuoteForCheckout(any()))
                .thenReturn(GetUberQuoteForCheckoutResponse.newBuilder()
                        .setCode(500).setMessage("boom").build());

        Assessment result = assessor.assess(quoteProbe());

        assertThat(result.eligible()).isFalse();
        assertThat(result.unavailableReason()).isEqualTo("UBER_QUOTE_ASSESSOR_UNAVAILABLE");
    }

    @Test
    @DisplayName("🔴 gRPC 例外不在本類別吞——由閘門統一轉成拒絕，兩層各 catch 一次遲早會有一層變 fail-open")
    void grpcExceptionPropagates() {
        when(shippingGrpcClient.getUberQuoteForCheckout(any()))
                .thenThrow(new IllegalStateException("UNAVAILABLE: io exception"));

        assertThatThrownBy(() -> assessor.assess(quoteProbe()))
                .isInstanceOf(IllegalStateException.class);
    }

    // ===== quoteId 非空 ⇒ ValidateUberQuoteForCheckout =====

    @Test
    @DisplayName("🔴 有既有 quote ⇒ 打 ValidateUberQuoteForCheckout，帶 quote_id／accept／order_no")
    void presentQuoteIdGoesToValidate() {
        when(shippingGrpcClient.validateUberQuoteForCheckout(any()))
                .thenReturn(validateResponse(ValidateUberQuoteData.newBuilder()
                        .setValid(true)
                        .setQuoteId("dqt_abc")
                        .setFeeAmount("115")
                        .setExpiresAt("2026-09-12T08:30:00Z")
                        .build()));

        Probe probe = new Probe(BUYER_ID, STORE_ID, "3202",
                "王小明", "0912345678", "台北市", "中正區", "測試路1號3樓",
                List.of(new Item(5001L, "蘋果", 2, 10, 12, 8, 1500L, 500L)),
                "dqt_abc", true, "O20260912001");

        Assessment result = assessor.assess(probe);

        ArgumentCaptor<ValidateUberQuoteForCheckoutRequest> captor =
                ArgumentCaptor.forClass(ValidateUberQuoteForCheckoutRequest.class);
        verify(shippingGrpcClient).validateUberQuoteForCheckout(captor.capture());
        verify(shippingGrpcClient, never()).getUberQuoteForCheckout(any());

        assertThat(captor.getValue().getQuoteId()).isEqualTo("dqt_abc");
        assertThat(captor.getValue().getAccept()).isTrue();
        assertThat(captor.getValue().getOrderNo()).isEqualTo("O20260912001");
        assertThat(captor.getValue().getBuyerId()).isEqualTo(BUYER_ID);
        assertThat(captor.getValue().getSellerStoreId()).isEqualTo(STORE_ID);

        assertThat(result.eligible()).isTrue();
        assertThat(result.quoteId()).isEqualTo("dqt_abc");
        // ValidateUberQuoteData 沒有 currency 欄位 ⇒ 留空，不得編一個 "TWD"
        assertThat(result.currency()).isEmpty();
    }

    @Test
    @DisplayName("🔴 price_changed ⇒ UBER_QUOTE_PRICE_CHANGED（shipping 在這條分支不填 unavailable_reason）")
    void priceChangedBecomesNamedReason() {
        when(shippingGrpcClient.validateUberQuoteForCheckout(any()))
                .thenReturn(validateResponse(ValidateUberQuoteData.newBuilder()
                        .setValid(false)
                        .setPriceChanged(true)
                        .setQuoteId("dqt_new")
                        .setFeeAmount("145")
                        .build()));

        Assessment result = assessor.assess(new Probe(
                BUYER_ID, STORE_ID, "3202", "王小明", "0912345678",
                "台北市", "中正區", "測試路1號3樓",
                List.of(new Item(5001L, "蘋果", 2, 10, 12, 8, 1500L, 500L)),
                "dqt_old", false, ""));

        assertThat(result.eligible()).isFalse();
        // 空 reason 會讓閘門落進「不認得」分支而吐不出原因——這一句就是在擋那個
        assertThat(result.unavailableReason()).isEqualTo("UBER_QUOTE_PRICE_CHANGED");
    }

    @Test
    @DisplayName("valid=false 且非 price_changed ⇒ 原樣帶回 unavailable_reason")
    void invalidRevalidationKeepsProviderReason() {
        when(shippingGrpcClient.validateUberQuoteForCheckout(any()))
                .thenReturn(validateResponse(ValidateUberQuoteData.newBuilder()
                        .setValid(false)
                        .setUnavailableReason("QUOTE_NOT_ACCEPTABLE")
                        .build()));

        Assessment result = assessor.assess(new Probe(
                BUYER_ID, STORE_ID, "3202", "王小明", "0912345678",
                "台北市", "中正區", "測試路1號3樓",
                List.of(new Item(5001L, "蘋果", 2, 10, 12, 8, 1500L, 500L)),
                "dqt_old", true, "O20260912001"));

        assertThat(result.eligible()).isFalse();
        assertThat(result.unavailableReason()).isEqualTo("QUOTE_NOT_ACCEPTABLE");
    }
}

package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.AvailableOption;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.FreeShippingResult;
import io.github.will413028.hawkr.order.checkout.CheckoutValidator.ValidationResult;
import io.github.will413028.hawkr.order.checkout.client.SellerGrpcClient;
import io.github.will413028.hawkr.order.grpc.CheckoutItem;
import io.github.will413028.hawkr.order.grpc.CheckoutPreviewData;
import io.github.will413028.hawkr.order.grpc.OrderGroupPreview;
import io.github.will413028.hawkr.order.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.InvoiceGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.order.service.OrderService;
import io.github.will413028.hawkr.common.utils.code.ShippingFeePayer;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.product.grpc.BatchGetProductForCheckoutResponse;
import io.github.will413028.hawkr.product.grpc.ProductForCheckoutInfo;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.grpc.GetSellerShippingConfigResponse;
import io.github.will413028.hawkr.shipping.grpc.SellerShippingConfigItem;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;

/**
 * #2137 — 結帳預覽的 orderGroups 是買家「下單前最後一眼」看到的賣家區塊，先前只有
 * store_id（內部代理鍵），前端連 /sellers/{store_id} 每次都要多吃一次 middleware 308。
 * 賣家頁 canonical 網址走 store_code，而這個值本來就在同一份 SellerMeta 裡
 * （sellerName 與 payuni_mer_id 就是從那裡抄的），只是組 proto 時被丟掉——補齊零額外 RPC。
 *
 * <p>store_code 刻意設成與 store_id 不同：兩者相等的賣家（由已綁定的 PayUni
 * merchant_code 回填）驗不出有沒有真的讀對欄位。
 *
 * <p>fixture 走 7-11 店到店（3002）：{@code getSenderPostalCode} 對超商短路回空字串、
 * {@code resolveRoute} 固定 DOMESTIC，省掉一層與本案無關的郵遞區號推導。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CheckoutServiceStoreCodeTest {

    private static final long STORE_ID = 2636840881L;
    private static final String STORE_CODE = "2649605557";
    private static final long BUYER_ID = 100L;
    private static final String SKU_ID = "5001";

    private static final String METHOD = ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value();
    private static final String TEMP = ShippingTemperature.CHILLED.value();

    @Mock private ProductGrpcClient productGrpcClient;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private InvoiceGrpcClient invoiceGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;
    @Mock private ShippingGrpcClient shippingGrpcClient;
    @Mock private CheckoutValidator checkoutValidator;
    @Mock private OrderService orderService;
    @Mock private StringRedisTemplate redisTemplate;
    @Mock private ValueOperations<String, String> valueOperations;

    @Test
    void previewOrderGroupCarriesSellerStoreCode() {
        CheckoutService service = newServiceWithSellerMeta(SellerMeta.newBuilder()
                .setSellerName("Seller X")
                .setStoreCode(STORE_CODE)
                .setCanAcceptOrders(true)
                .build());

        OrderGroupPreview group = previewSingleGroup(service);

        assertThat(group.getStoreCode()).isEqualTo(STORE_CODE);
        // store_id 仍要在（結帳鏈後續步驟吃它），只是不再是連結的來源。
        assertThat(group.getStoreId()).isEqualTo(String.valueOf(STORE_ID));
    }

    /**
     * 賣家 store_code 尚未回填時留空，前端據此不渲染連結（而不是退回 store_id 連結）。
     */
    @Test
    void previewLeavesStoreCodeBlankWhenSellerHasNone() {
        CheckoutService service = newServiceWithSellerMeta(SellerMeta.newBuilder()
                .setSellerName("Seller X")
                .setCanAcceptOrders(true)
                .build());

        assertThat(previewSingleGroup(service).getStoreCode()).isEmpty();
    }

    private OrderGroupPreview previewSingleGroup(CheckoutService service) {
        CheckoutPreviewData data = service.preview(
                List.of(CheckoutItem.newBuilder()
                        .setSkuId(SKU_ID)
                        .setProductId("9001")
                        .setQuantity("1")
                        .build()),
                METHOD, TEMP, "", "", "", BUYER_ID, false, "", "");

        assertThat(data.getOrderGroupsList()).hasSize(1);
        return data.getOrderGroups(0);
    }

    private CheckoutService newServiceWithSellerMeta(SellerMeta sellerMeta) {
        // 商品解析：單一可購買 SKU，賣家 = STORE_ID
        when(productGrpcClient.batchGetProductForCheckout(anyList()))
                .thenReturn(BatchGetProductForCheckoutResponse.newBuilder()
                        .putData(SKU_ID, ProductForCheckoutInfo.newBuilder()
                                .setSkuName("Apple")
                                .setProductName("Apple Box")
                                .setStoreId(String.valueOf(STORE_ID))
                                .setListPrice("500")
                                .setStockQty("10")
                                .setIsActive(true)
                                .build())
                        .build());

        // 自買防護：賣家 principal 與買家不同；買家軸同樣要解析（issue #2789 階段 B2）
        when(sellerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(STORE_ID, STORE_ID)));
        when(buyerGrpcClient.resolvePrincipals(anyList()))
                .thenReturn(PrincipalResolution.allResolved(Map.of(BUYER_ID, BUYER_ID)));

        // 賣家已開通 7-11（requireSellerOnboarded 比對 "<method>:" 前綴）
        when(shippingGrpcClient.getSellerShippingConfig(anyLong()))
                .thenReturn(GetSellerShippingConfigResponse.newBuilder()
                        .setCode(0)
                        .addConfigs(SellerShippingConfigItem.newBuilder()
                                .setShippingMethod(METHOD)
                                .setGoodsType(TEMP)
                                .setAvailable(true)
                                .build())
                        .build());

        when(sellerGrpcClient.getSellerMeta(anyLong())).thenReturn(sellerMeta);

        when(checkoutValidator.validate(anyList(), any()))
                .thenReturn(new ValidationResult(true, java.util.Set.of(TEMP), List.of(), List.of(), 0));
        when(checkoutValidator.lookupSingleOption(anyList(), any(), any(), any(), any()))
                .thenReturn(new AvailableOption(
                        ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, ShippingTemperature.CHILLED,
                        "7-11店到店（冷藏）", 60L, 60L, ShippingFeePayer.BUYER.value(), 0L, false));
        when(checkoutValidator.checkFreeShipping(anyList(), anyString(), anyString(), any()))
                .thenReturn(new FreeShippingResult(false, ShippingFeePayer.BUYER.value(), 0L));

        when(redisTemplate.opsForValue()).thenReturn(valueOperations);

        // preview 會呼叫 §6.2.4 order ② 的發票模式解析；本測試不驗發票區，
        // 回 null 走「拿不到就不設 invoice 欄」那條路（與 invoice-service 不可用時同路徑）。
        when(invoiceGrpcClient.resolveOrderInvoiceMode(any())).thenReturn(null);

        return new CheckoutService(productGrpcClient, sellerGrpcClient, buyerGrpcClient, shippingGrpcClient,
                null, invoiceGrpcClient, checkoutValidator, orderService, new OrderPolicyResolver(), redisTemplate);
    }
}

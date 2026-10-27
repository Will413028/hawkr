package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Assessment;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Item;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.Probe;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.ProviderReason;
import io.github.will413028.hawkr.order.checkout.UberOrderEligibilityGuard.QuoteAssessor;
import io.github.will413028.hawkr.order.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.shipping.grpc.GetUberQuoteForCheckoutRequest;
import io.github.will413028.hawkr.shipping.grpc.GetUberQuoteForCheckoutResponse;
import io.github.will413028.hawkr.shipping.grpc.UberQuoteData;
import io.github.will413028.hawkr.shipping.grpc.UberQuoteDropoff;
import io.github.will413028.hawkr.shipping.grpc.UberQuoteItem;
import io.github.will413028.hawkr.shipping.grpc.ValidateUberQuoteData;
import io.github.will413028.hawkr.shipping.grpc.ValidateUberQuoteForCheckoutRequest;
import io.github.will413028.hawkr.shipping.grpc.ValidateUberQuoteForCheckoutResponse;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;

/**
 * {@link UberOrderEligibilityGuard} 的證據來源：打 shipping 的兩支 Uber 報價 RPC。
 *
 * <h2>兩支不是二選一，是兩個時點</h2>
 * <ul>
 *   <li>{@code probe.quoteId()} <b>空</b> ⇒ {@code GetUberQuoteForCheckout}：
 *       「現在有沒有一份可用的報價？」冪等——同一車在 TTL 內回同一個 quote_id，不重打 Uber。</li>
 *   <li>{@code probe.quoteId()} <b>非空</b> ⇒ {@code ValidateUberQuoteForCheckout}：
 *       「買家手上這一份還算數嗎？」capture 前 {@code accept=false} 只重驗，
 *       capture 後 {@code accept=true} 才把它 CAS 成 {@code CHECKOUT_ACCEPTED}。</li>
 * </ul>
 * 🔴 <b>不能只用 validate 並把 quote_id 留空</b>：{@code UberQuoteService.revalidate()} 對
 * {@code current == null} 一律回 {@code priceChanged=true}（原始碼逐字如此），
 * 那會讓「買家從來沒拿過報價」變成「價格變了」——一個沒人看得懂、也修不掉的拒絕。
 *
 * <h2>🔴 本類別只做形狀轉換，不做判斷</h2>
 * 「可不可以用 Uber」的裁決者是 shipping 的 {@code UberQuoteService}；本類別不得在這裡
 * 補任何 if（例如「reason 是 X 的時候其實可以放行」）。要改判準就去改 shipping，
 * 否則買家看到的與建單當下用的會是兩個答案。
 *
 * <p>例外處置刻意<b>不在這裡</b>：{@code StatusRuntimeException}（shipping 掛了／逾時）直接往上
 * 拋給 {@link UberOrderEligibilityGuard#evaluate}，由它統一轉成「取不到證據 ⇒ 拒絕」。
 * 兩層都 catch 會產生兩個 fail-closed 的決定點，而其中一個遲早會被改成 fail-open。
 */
@Component
public class ShippingUberQuoteAssessor implements QuoteAssessor {

    private static final ObsLogger log = ObsLogger.forClass(ShippingUberQuoteAssessor.class);

    private final ShippingGrpcClient shippingGrpcClient;

    public ShippingUberQuoteAssessor(ShippingGrpcClient shippingGrpcClient) {
        this.shippingGrpcClient = shippingGrpcClient;
    }

    @Override
    public Assessment assess(Probe probe) {
        return probe.quoteId().isBlank() ? quote(probe) : revalidate(probe);
    }

    private Assessment quote(Probe probe) {
        GetUberQuoteForCheckoutResponse response = shippingGrpcClient.getUberQuoteForCheckout(
                GetUberQuoteForCheckoutRequest.newBuilder()
                        .setBuyerId(probe.buyerId())
                        .setSellerStoreId(probe.sellerStoreId())
                        .setDeliveryTemp(nullSafe(probe.deliveryTemp()))
                        .setDropoff(dropoff(probe))
                        .addAllItems(items(probe.items()))
                        .build());
        if (response.getCode() != GrpcResponseCode.SUCCESS.code()) {
            // 下游契約是「恆 code=0，業務失敗走 available=false」。收到非零表示契約被改過
            // ⇒ 當成「問不到」，不猜它的意思。
            log.warn("ShippingUberQuoteAssessor.quote.nonZeroCode",
                    "shipping 回非零 code=" + response.getCode() + " message=" + response.getMessage());
            return Assessment.unavailable(ProviderReason.ASSESSOR_UNAVAILABLE);
        }
        UberQuoteData data = response.getData();
        if (!data.getAvailable()) {
            return Assessment.unavailable(data.getUnavailableReason());
        }
        return Assessment.eligible(data.getQuoteId(), data.getFeeAmount(),
                data.getCurrency(), data.getExpiresAt());
    }

    private Assessment revalidate(Probe probe) {
        ValidateUberQuoteForCheckoutResponse response = shippingGrpcClient.validateUberQuoteForCheckout(
                ValidateUberQuoteForCheckoutRequest.newBuilder()
                        .setQuoteId(probe.quoteId())
                        .setBuyerId(probe.buyerId())
                        .setSellerStoreId(probe.sellerStoreId())
                        .setDeliveryTemp(nullSafe(probe.deliveryTemp()))
                        .setDropoff(dropoff(probe))
                        .addAllItems(items(probe.items()))
                        .setAccept(probe.accept())
                        .setOrderNo(probe.orderNo())
                        .build());
        if (response.getCode() != GrpcResponseCode.SUCCESS.code()) {
            log.warn("ShippingUberQuoteAssessor.revalidate.nonZeroCode",
                    "shipping 回非零 code=" + response.getCode() + " message=" + response.getMessage());
            return Assessment.unavailable(ProviderReason.ASSESSOR_UNAVAILABLE);
        }
        ValidateUberQuoteData data = response.getData();
        if (data.getValid()) {
            // ⚠️ ValidateUberQuoteData 沒有 currency 欄位（報價時才有）——留空，不要編一個 "TWD"。
            return Assessment.eligible(data.getQuoteId(), data.getFeeAmount(), "", data.getExpiresAt());
        }
        if (data.getPriceChanged()) {
            // 🔴 金額變了不得 capture（INV-U1）。這裡回 PRICE_CHANGED 而不是 unavailable_reason，
            //    因為 shipping 在這條分支不填 unavailable_reason（原始碼的 else-if 互斥）。
            return Assessment.unavailable(ProviderReason.UBER_QUOTE_PRICE_CHANGED);
        }
        return Assessment.unavailable(data.getUnavailableReason());
    }

    private static UberQuoteDropoff dropoff(Probe probe) {
        return UberQuoteDropoff.newBuilder()
                .setReceiverName(nullSafe(probe.receiverName()))
                .setReceiverPhone(nullSafe(probe.receiverPhone()))
                .setCounty(nullSafe(probe.county()))
                .setDistrict(nullSafe(probe.district()))
                .setDetailAddress(nullSafe(probe.detailAddress()))
                .build();
    }

    private static List<UberQuoteItem> items(List<Item> items) {
        List<UberQuoteItem> out = new ArrayList<>(items.size());
        for (Item item : items) {
            out.add(UberQuoteItem.newBuilder()
                    .setSkuId(String.valueOf(item.skuId()))
                    .setName(nullSafe(item.name()))
                    .setQuantity(String.valueOf(item.quantity()))
                    .setLengthCm(String.valueOf(item.lengthCm()))
                    .setWidthCm(String.valueOf(item.widthCm()))
                    .setHeightCm(String.valueOf(item.heightCm()))
                    .setWeightGram(String.valueOf(item.weightGram()))
                    .setUnitPrice(String.valueOf(item.unitPrice()))
                    .build());
        }
        return out;
    }

    /** proto 的 string setter 不吃 null。 */
    private static String nullSafe(String value) {
        return value == null ? "" : value;
    }
}

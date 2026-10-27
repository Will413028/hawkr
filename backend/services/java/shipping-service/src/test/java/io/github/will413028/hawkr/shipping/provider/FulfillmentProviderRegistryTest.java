package io.github.will413028.hawkr.shipping.provider;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.provider.capability.CancelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.CostCapability;
import io.github.will413028.hawkr.shipping.provider.capability.CreateCapability;
import io.github.will413028.hawkr.shipping.provider.capability.DispatchCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability;
import io.github.will413028.hawkr.shipping.provider.capability.PodCapability;
import io.github.will413028.hawkr.shipping.provider.capability.ReturnCapability;
import io.github.will413028.hawkr.shipping.provider.capability.StoreLocatorCapability;
import io.github.will413028.hawkr.shipping.provider.capability.TrackCapability;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Optional;
import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * provider registry 的 fail-closed 行為。
 *
 * <p>🔴 本測試守的是<b>兩種 empty 的區別</b>：
 * <ul>
 *   <li>「這個配送方式沒有註冊 adapter」＝ <b>我方漏接線</b> ⇒ 必須讓呼叫者看見（拋業務例外）</li>
 *   <li>「這個 provider 不支援某能力」＝ <b>正常狀態</b> ⇒ {@code Optional.empty()}</li>
 * </ul>
 * 把前者也寫成 empty，症狀會是「按鈕靜默消失」而不是「有東西壞了」——那是最貴的一種失效。
 */
@DisplayName("FulfillmentProviderRegistry 的 fail-closed 行為")
class FulfillmentProviderRegistryTest {

    /** 只宣告支援的配送方式、所有 capability 皆 empty 的最小 adapter。 */
    private static FulfillmentProviderAdapter stub(ShippingMethod method) {
        return new FulfillmentProviderAdapter() {
            @Override public ShippingMethod supportedMethod() { return method; }
            @Override public Set<String> pendingMigration() { return Set.of(); }
            @Override public Optional<CreateCapability> create(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<LabelCapability> label(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<DispatchCapability> dispatch(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<CancelCapability> cancel(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<TrackCapability> track(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<ReturnCapability> returns(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<PodCapability> pod(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<CostCapability> cost(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<OnboardCapability> onboard(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<InboundCapability> inbound(FulfillmentContext c) { return Optional.empty(); }
            @Override public Optional<StoreLocatorCapability> storeLocator(FulfillmentContext c) { return Optional.empty(); }
        };
    }

    @Test
    @DisplayName("查無 adapter 是業務例外，不是 empty")
    void unknownMethodThrowsInsteadOfReturningEmpty() {
        var registry = new FulfillmentProviderRegistry(List.of(stub(ShippingMethod.TAQBIN)));
        assertThatThrownBy(() -> registry.require(ShippingMethod.MERCHANT_DELIVERY.value()))
                .isInstanceOf(ShippingBusinessException.class)
                .satisfies(e -> assertThat(((ShippingBusinessException) e).code())
                        .isEqualTo(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED.code()));
    }

    /**
     * 🔴 撞號必須<b>立刻爆開機</b>。兩個 adapter 宣告同一個配送方式時，
     * 「哪一個生效」取決於 bean 掃描順序——那是不可預測、且測試環境與 prod 可能不同的行為。
     */
    @Test
    @DisplayName("兩個 adapter 宣告同一配送方式 → boot fail，不得靜默取其中一個")
    void duplicateMethodFailsFast() {
        assertThatThrownBy(() -> new FulfillmentProviderRegistry(
                List.of(stub(ShippingMethod.TAQBIN), stub(ShippingMethod.TAQBIN))))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("兩個 adapter");
    }

    @Test
    @DisplayName("supportedMethod() 回 null → boot fail（registry 無法建索引）")
    void nullSupportedMethodFailsFast() {
        assertThatThrownBy(() -> new FulfillmentProviderRegistry(List.of(stub(null))))
                .isInstanceOf(IllegalStateException.class);
    }

    @Test
    @DisplayName("find() 對未註冊的方式回 empty（只給非業務路徑用）")
    void findReturnsEmptyForUnknown() {
        var registry = new FulfillmentProviderRegistry(List.of(stub(ShippingMethod.TAQBIN)));
        assertThat(registry.find(ShippingMethod.UBER_INSTANT.value())).isEmpty();
        assertThat(registry.find(ShippingMethod.TAQBIN.value())).isPresent();
    }

    /**
     * {@code FulfillmentContext.method} 為 null 時 fail-fast：
     * capability 解析的第一層分派就是配送方式，null 會讓 registry 落到「查無 adapter」
     * 而不是「不支援」——兩者的處置完全不同。
     */
    @Test
    @DisplayName("context 的配送方式不得為 null")
    void contextRejectsNullMethod() {
        assertThatThrownBy(() -> new FulfillmentContext(1L, null, null))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("capability 解析");
    }
}

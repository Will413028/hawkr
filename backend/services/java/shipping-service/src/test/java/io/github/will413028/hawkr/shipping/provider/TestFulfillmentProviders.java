package io.github.will413028.hawkr.shipping.provider;

import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.shipping.config.ShippingBatchProperties;
import io.github.will413028.hawkr.shipping.provider.payuni.PayUniCvsAdapter;
import io.github.will413028.hawkr.shipping.provider.payuni.PayUniHomeAdapter;
import io.github.will413028.hawkr.shipping.provider.uber.TestUberAdapters;
import io.github.will413028.hawkr.shipping.provider.uber.UberDirectAdapter;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.ShipmentLabelService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import io.github.will413028.hawkr.shipping.service.StoreMapService;
import io.github.will413028.hawkr.shipping.provider.create.CvsShipmentStrategy;
import io.github.will413028.hawkr.shipping.provider.create.HomeShipmentStrategy;
import io.github.will413028.hawkr.shipping.provider.create.UberShipmentStrategy;
import org.springframework.beans.factory.ObjectProvider;

import java.util.List;

import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 測試用的 {@link FulfillmentProviderRegistry} 工廠——<b>裝的是正牌 adapter</b>。
 *
 * <h2>🔴 為什麼不用 stub adapter</h2>
 * 本波把十幾處 {@code shipping_method} 字面分支換成「問 adapter 這個 provider 有沒有這個能力」。
 * 如果測試餵的是自己寫的 stub，斷言驗的就只是「service 有沒有照著我寫的 stub 走」——
 * 而 stub 的 capability 集合是我決定的，正是待驗的那件事（自己跟自己對）。
 * 裝正牌 adapter 之後，改壞 {@code PayUniHomeAdapter.lgsTypeCode()} 或
 * {@code UberDirectAdapter...requiresOnboarding()} 這種變異才會讓測試轉紅。
 *
 * <p>⚠️ 只有 adapter 的<b>協作者</b>是 mock（PayUni client、建單 strategy、OnboardingService）——
 * 那些是 provider 邊界之外的東西，不是被測的判斷。
 */
public final class TestFulfillmentProviders {

    private TestFulfillmentProviders() {
    }

    public static FulfillmentProviderRegistry realRegistry(ShippingClient shippingClient,
                                                           ShipmentStatusMapper statusMapper,
                                                           OnboardingService onboardingService) {
        return realRegistry(shippingClient, statusMapper, onboardingService,
                mock(ShipmentLabelService.class));
    }

    /**
     * 需要盯住標籤 provider 呼叫的測試用這一支——傳進來的 {@code labelService} 就是
     * adapter 實際委派的那一個，斷言才看得到 adapter 有沒有真的打出去。
     */
    public static FulfillmentProviderRegistry realRegistry(ShippingClient shippingClient,
                                                           ShipmentStatusMapper statusMapper,
                                                           OnboardingService onboardingService,
                                                           ShipmentLabelService labelService) {
        return realRegistry(shippingClient, statusMapper, onboardingService, labelService,
                mock(HomeShipmentStrategy.class), mock(CvsShipmentStrategy.class),
                mock(UberShipmentStrategy.class), mock(StoreMapService.class));
    }

    public static FulfillmentProviderRegistry realRegistry(ShippingClient shippingClient,
                                                           ShipmentStatusMapper statusMapper,
                                                           OnboardingService onboardingService,
                                                           ShipmentLabelService labelService,
                                                           HomeShipmentStrategy homeStrategy,
                                                           CvsShipmentStrategy cvsStrategy,
                                                           UberShipmentStrategy uberStrategy,
                                                           StoreMapService storeMapService) {
        ObjectProvider<OnboardingService> onboardingProvider = lazy(onboardingService);
        return new FulfillmentProviderRegistry(List.of(
                new PayUniHomeAdapter(shippingClient, statusMapper, labelService, homeStrategy,
                        onboardingProvider),
                new PayUniCvsAdapter(shippingClient, statusMapper, labelService, cvsStrategy,
                        new ShippingBatchProperties(), storeMapService, onboardingProvider),
                TestUberAdapters.builder().strategy(uberStrategy).build()));
    }

    @SuppressWarnings("unchecked")
    private static ObjectProvider<OnboardingService> lazy(OnboardingService onboardingService) {
        ObjectProvider<OnboardingService> provider = mock(ObjectProvider.class);
        when(provider.getObject()).thenReturn(onboardingService);
        return provider;
    }
}

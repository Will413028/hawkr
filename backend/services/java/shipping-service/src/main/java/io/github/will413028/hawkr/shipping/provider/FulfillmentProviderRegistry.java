package io.github.will413028.hawkr.shipping.provider;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import org.springframework.stereotype.Component;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;

/**
 * {@link FulfillmentProviderAdapter} 的索引。
 *
 * <p>🔴 <b>fail-closed：查無 adapter 是錯誤，不是「沒有能力」。</b>
 * 兩者的差別是本 registry 存在的理由——
 * <ul>
 *   <li>「這個配送方式沒有註冊 adapter」＝ <b>我方漏接線</b>，必須讓呼叫者看見（拋業務例外）</li>
 *   <li>「這個 provider 不支援某能力」＝ <b>正常狀態</b>，由 {@code Optional.empty()} 表達</li>
 * </ul>
 * 把前者也寫成 empty，症狀會是「按鈕靜默消失」而不是「有東西壞了」，而那是最貴的一種失效。
 *
 * <p>⚠️ 本類別<b>刻意不提供</b>「找不到就回預設 adapter」的多載。
 * 那正是把新 provider 靜默導進 PayUni 分支的機制。
 */
@Component
public class FulfillmentProviderRegistry {

    private static final ObsLogger log = ObsLogger.forClass(FulfillmentProviderRegistry.class);

    private final Map<String, FulfillmentProviderAdapter> byMethodValue;

    public FulfillmentProviderRegistry(List<FulfillmentProviderAdapter> adapters) {
        Map<String, FulfillmentProviderAdapter> map = new LinkedHashMap<>();
        for (FulfillmentProviderAdapter adapter : adapters) {
            ShippingMethod method = adapter.supportedMethod();
            if (method == null) {
                throw new IllegalStateException(
                        adapter.getClass().getName() + ".supportedMethod() 回 null——registry 無法建索引");
            }
            FulfillmentProviderAdapter previous = map.put(method.value(), adapter);
            if (previous != null) {
                throw new IllegalStateException("配送方式 " + method.value() + " 有兩個 adapter 宣告支援："
                        + previous.getClass().getName() + " 與 " + adapter.getClass().getName());
            }
        }
        this.byMethodValue = Map.copyOf(map);

        StringBuilder wired = new StringBuilder();
        byMethodValue.forEach((methodValue, adapter) -> {
            if (wired.length() > 0) {
                wired.append(", ");
            }
            wired.append(methodValue).append("=").append(adapter.getClass().getSimpleName());
        });
        log.info("init.ok",
                "履約 provider adapter 註冊完成，共 " + byMethodValue.size() + " 個",
                "wired", wired.toString());
    }

    /**
     * 取得 adapter；查無即拋 {@code FULFILLMENT_PROVIDER_NOT_ENABLED}。
     *
     * @param shippingMethodValue {@code ShippingMethod.value()}（例：{@code "3001"}）
     */
    public FulfillmentProviderAdapter require(String shippingMethodValue) {
        FulfillmentProviderAdapter adapter = byMethodValue.get(shippingMethodValue);
        if (adapter == null) {
            throw new ShippingBusinessException(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED);
        }
        return adapter;
    }

    /** 只在「不確定有沒有」且缺席是合法情境時使用（例如報表投影）。業務路徑一律用 {@link #require}。 */
    public Optional<FulfillmentProviderAdapter> find(String shippingMethodValue) {
        return Optional.ofNullable(byMethodValue.get(shippingMethodValue));
    }

    /** 已註冊的配送方式值域（供 boot 期自檢與測試斷言）。 */
    public java.util.Set<String> registeredMethodValues() {
        return byMethodValue.keySet();
    }
}

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
                // 🔴 撞號必須立刻爆開機，不能靜默取其中一個：
                // 兩個 adapter 宣告同一個配送方式時，「哪一個生效」取決於 bean 掃描順序，
                // 那是一個不可預測、且在測試環境與 prod 可能不同的行為。
                throw new IllegalStateException("配送方式 " + method.value() + " 有兩個 adapter 宣告支援："
                        + previous.getClass().getName() + " 與 " + adapter.getClass().getName());
            }
        }
        this.byMethodValue = Map.copyOf(map);

        // 🔴 開機列出實際註冊了哪些 provider。
        //
        // 為什麼需要這一行：本 registry 的兩道 fail-fast（supportedMethod() 回 null、撞號）
        // 都只在「有東西不對」時出聲，**沒有註冊到任何 adapter 一樣會安靜地開機成功**——
        // 症狀要等到某個賣家按下叫件、拿到 FULFILLMENT_PROVIDER_NOT_ENABLED 才浮現。
        // 「開機成功」證明的是沒有撞號，不是有註冊；這一行才讓後者可查證。
        //
        // ⚠️ 印的是 adapter 的 simple name 不只是配送方式碼：接新 provider 時真正要確認的是
        // 「3001 現在由誰承接」，只印碼值看不出換人了沒。
        StringBuilder wired = new StringBuilder();
        byMethodValue.forEach((methodValue, adapter) -> {
            if (wired.length() > 0) {
                wired.append(", ");
            }
            wired.append(methodValue).append("=").append(adapter.getClass().getSimpleName());
        });
        // ⚠️ ObsLogger 只收單一 bzCol/bzValue 對，所以數量併進訊息本體。
        // ⚠️ 第一個參數是 ObsLogger 的 result 段，不是完整 event 名——它自己會補
        //    ClassName.method 前綴。傳完整名會得到 "X.<init>.X.init.ok" 這種重複。
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

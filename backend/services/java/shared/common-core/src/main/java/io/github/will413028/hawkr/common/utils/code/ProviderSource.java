package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 訂單／售後單在「與第三方對帳」時，`providerStatus` / `providerMessage` 這組欄位究竟來自哪一個第三方環節（[09 §29 異常訂單](../詳細設計書/09-售後微服務（AfterSales%20Service）詳細設計書.md)）。決定 ADM 異常訂單頁要把 provider 回應歸類到金流還是物流，以及要補哪些對帳細節欄位（`providerTradeNo` / `providerRefundNo` / `shipTradeNo`）。**空字串代表「該訂單目前沒有第三方環節可對帳」，不是本 enum 的成員**，前端不得為空字串準備標籤。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProviderSource implements CodeMeta {

    PAYMENT("PAYMENT", "PAYMENT", "付款", "付款收單環節（PayUni 交易建立 / 授權 / 入帳）。同時帶出 `providerTradeNo`"),
    PAYMENT_REFUND("PAYMENT_REFUND", "PAYMENT_REFUND", "付款退款", "金流退款環節（PayUni 退款請求與結果）。同時帶出 `providerRefundNo`"),
    SHIPPING("SHIPPING", "SHIPPING", "物流", "物流環節（建單 / 叫件 / 配送狀態回拋）。同時帶出 `shipTradeNo`");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProviderSource(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, ProviderSource> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProviderSource::code, e -> e));

    private static final Map<String, ProviderSource> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProviderSource::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProviderSource fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProviderSource fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

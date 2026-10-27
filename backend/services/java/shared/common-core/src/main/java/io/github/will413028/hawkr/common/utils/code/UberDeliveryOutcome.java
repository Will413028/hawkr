package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 即時配送未完成的結果原因，5 值。🔴 **值域由 65 定案**——原本 envelope 只寫欄位型別 `String` 而沒有值域，導致 payload 契約測試無值可對。⚠️ 本 enum 是 envelope 欄位的值域，**與 webhook inbox 的 `WebhookProvider` 分類無關**，兩者不得混用。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum UberDeliveryOutcome implements CodeMeta {

    PROVIDER_UNDELIVERABLE("PROVIDER_UNDELIVERABLE", "PROVIDER_UNDELIVERABLE", "物流商無法送達", "provider 回報無法完成配送"),
    PROVIDER_CANCELED("PROVIDER_CANCELED", "PROVIDER_CANCELED", "物流商取消", "provider 側取消"),
    SELLER_CANCELED("SELLER_CANCELED", "SELLER_CANCELED", "賣家取消", "賣家主動取消"),
    ADMIN_CANCELED("ADMIN_CANCELED", "ADMIN_CANCELED", "平台取消", "ADM 判定取消"),
    CREATE_FAILED("CREATE_FAILED", "CREATE_FAILED", "建單失敗", "建單重試耗盡後確定失敗");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    UberDeliveryOutcome(String code, String value, String label, String description) {
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

    private static final Map<String, UberDeliveryOutcome> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(UberDeliveryOutcome::code, e -> e));

    private static final Map<String, UberDeliveryOutcome> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(UberDeliveryOutcome::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static UberDeliveryOutcome fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static UberDeliveryOutcome fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

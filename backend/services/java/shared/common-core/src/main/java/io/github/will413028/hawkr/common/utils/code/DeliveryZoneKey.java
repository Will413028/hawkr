package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 運費表（fee_tier）的「費率分組鍵」。**獨立於 ShippingRoute** —— ShippingRoute 是業務語意（4 值：本島互寄/寄離島/離島寄出/離島互寄），DeliveryZoneKey 是費率分組（3 值，因 TO_OFFSHORE 與 FROM_OFFSHORE 共用同一費率而合併為 OFFSHORE）。對映關係由 `ShippingFeeCacheClient.toDeliveryZone(ShippingRoute)` 處理。Code 與 Value 設計上一致以維持與既有 DB 字面相容。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum DeliveryZoneKey implements CodeMeta {

    DOMESTIC("DOMESTIC", "8101", "本島", "本島區域內運送（對應 ShippingRoute.DOMESTIC）"),
    OFFSHORE("OFFSHORE", "8102", "本島↔離島", "本島與離島之間運送（對應 ShippingRoute.TO_OFFSHORE / FROM_OFFSHORE）"),
    INTER_ISLAND("INTER_ISLAND", "8103", "離島互寄", "離島之間運送（對應 ShippingRoute.OFFSHORE_TO_OFFSHORE）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    DeliveryZoneKey(String code, String value, String label, String description) {
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

    private static final Map<String, DeliveryZoneKey> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DeliveryZoneKey::code, e -> e));

    private static final Map<String, DeliveryZoneKey> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DeliveryZoneKey::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static DeliveryZoneKey fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static DeliveryZoneKey fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

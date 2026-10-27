package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 履約模式——**誰負責把貨送到**。訂單快照六欄之一。⚠️ 與 `ShippingMethod` **不完全正交**：`MERCHANT_DELIVERY`(3005) 必然是 `SELLER_MANAGED`，由 `chk_orders_fulfillment_mode_consistent` 保證兩欄一致，否則兩份真相必然漂移。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum FulfillmentMode implements CodeMeta {

    SELLER_MANAGED("SELLER_MANAGED", "SELLER_MANAGED", "賣家自行履約", "賣家自己出貨或自行配送。3005 必然落在本值"),
    PLATFORM_LOGISTICS("PLATFORM_LOGISTICS", "PLATFORM_LOGISTICS", "平台物流履約", "由平台安排的物流通道履約（3001／3002／3003／3004）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    FulfillmentMode(String code, String value, String label, String description) {
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

    private static final Map<String, FulfillmentMode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentMode::code, e -> e));

    private static final Map<String, FulfillmentMode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentMode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static FulfillmentMode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static FulfillmentMode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

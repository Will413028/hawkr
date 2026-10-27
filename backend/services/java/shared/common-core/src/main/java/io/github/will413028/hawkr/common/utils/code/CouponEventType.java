package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 優惠券事件 MQ 類型（Value 同時作為 routing key），由 `CouponEventPublisher` 發布到 `hawkr.marketing` exchange。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CouponEventType implements CodeMeta {

    COUPON_ISSUED("COUPON_ISSUED", "coupon.issued", "優惠券發放", "發放優惠券給買家，通知下游推播"),
    COUPON_EXPIRING("COUPON_EXPIRING", "coupon.expiring", "優惠券即將到期", "買家未使用券即將到期前預警推播（每日掃描 fan-out）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CouponEventType(String code, String value, String label, String description) {
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

    private static final Map<String, CouponEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponEventType::code, e -> e));

    private static final Map<String, CouponEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CouponEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CouponEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CouponEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

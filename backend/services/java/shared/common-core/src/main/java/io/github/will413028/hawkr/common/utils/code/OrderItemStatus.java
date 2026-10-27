package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 訂單明細退款狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum OrderItemStatus implements CodeMeta {

    NORMAL("NORMAL", "0", "正常", "正常狀態"),
    REFUND_PROCESSING("REFUND_PROCESSING", "10", "退款流程中", "申請/審核/選擇/執行中"),
    REFUNDED_ORIGINAL("REFUNDED_ORIGINAL", "20", "已退款（原路）", "退款至原支付管道"),
    REFUNDED_COUPON("REFUNDED_COUPON", "21", "已退款（優惠券）", "退款為優惠券");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    OrderItemStatus(String code, String value, String label, String description) {
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

    private static final Map<String, OrderItemStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderItemStatus::code, e -> e));

    private static final Map<String, OrderItemStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderItemStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static OrderItemStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static OrderItemStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

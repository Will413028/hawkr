package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家取消出貨狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerCancelStatus implements CodeMeta {

    PENDING("PENDING", "0", "待處理", "已建立，待退款"),
    REFUND_PROCESSING("REFUND_PROCESSING", "10", "退款處理中", "退款執行中"),
    COMPLETED("COMPLETED", "20", "已完成", "退款完成，訂單已取消"),
    FAILED("FAILED", "30", "失敗", "退款失敗");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerCancelStatus(String code, String value, String label, String description) {
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

    private static final Map<String, SellerCancelStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerCancelStatus::code, e -> e));

    private static final Map<String, SellerCancelStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerCancelStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerCancelStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerCancelStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

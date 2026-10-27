package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後賣家決策處理方式
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleResolutionType implements CodeMeta {

    RETURN_REFUND("RETURN_REFUND", "1", "同意退貨退款", "買家退貨後退全額"),
    PARTIAL_REFUND("PARTIAL_REFUND", "2", "部分退款", "賣家自訂退款金額"),
    FULL_REFUND_NO_RETURN("FULL_REFUND_NO_RETURN", "3", "全額退款（免退貨）", "免退貨直接全額退款"),
    COUPON_COMPENSATION("COUPON_COMPENSATION", "4", "發放補償優惠券", "發放賣家補償優惠券"),
    REJECT("REJECT", "5", "拒絕售後", "賣家拒絕售後申請"),
    INSPECTION_DEDUCTION("INSPECTION_DEDUCTION", "6", "驗貨扣款", "賣家確認收貨時依驗貨結果調整退款金額（僅接續 RETURN_REFUND 使用）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleResolutionType(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleResolutionType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleResolutionType::code, e -> e));

    private static final Map<String, AfterSaleResolutionType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleResolutionType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleResolutionType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleResolutionType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

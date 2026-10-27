package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 物流商帳單明細的費用類型，5 值。⚠️ 唯一寫入者是 ADM **人工匯入**——本 repo 明令不做自動比對排程，本表不是自動對帳的一部分。未登記本 enum 則 `CodeMetaValidator` 會擋下寫入。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProviderFeeType implements CodeMeta {

    DELIVERY("DELIVERY", "DELIVERY", "配送費", "正常配送的費用"),
    CANCELLATION("CANCELLATION", "CANCELLATION", "取消費", "取消產生的費用"),
    WAITING("WAITING", "WAITING", "等候費", "外送員等候費"),
    RETURN("RETURN", "RETURN", "退回費", "逆向配送費用"),
    ADJUSTMENT("ADJUSTMENT", "ADJUSTMENT", "調整", "provider 帳單的調整列");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProviderFeeType(String code, String value, String label, String description) {
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

    private static final Map<String, ProviderFeeType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProviderFeeType::code, e -> e));

    private static final Map<String, ProviderFeeType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProviderFeeType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProviderFeeType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProviderFeeType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

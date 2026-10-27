package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家這一版回價的種類（issue #2538）。⛔ **兩個值的必填欄位完全相同**——「接受非價格條款」不是少填金額的捷徑，v6.0 逐字要求不論哪一種都必須建立含實際品相、unit basis、goods／shipping／tax／total 與 validUntil 的 immutable 版本。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementSellerResponseType implements CodeMeta {

    MANUAL_QUOTE("MANUAL_QUOTE", "MANUAL_QUOTE", "自行報價", "賣家提出自己的商務條件"),
    ACCEPT_NON_PRICE_TERMS_WITH_QUOTE("ACCEPT_NON_PRICE_TERMS_WITH_QUOTE", "ACCEPT_NON_PRICE_TERMS_WITH_QUOTE", "接受非價格條款並報價", "接受買家的非價格條款，但仍須自行填齊品相與全部金額");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementSellerResponseType(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementSellerResponseType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementSellerResponseType::code, e -> e));

    private static final Map<String, ProcurementSellerResponseType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementSellerResponseType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementSellerResponseType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementSellerResponseType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家的營業主體類型。⚠️ **正式值域待會計師／法務覆核**（需求書明標）；本表為 Phase 1 的最小可用集合，補值時只改本檔再 generate.sh，不動 schema（欄位為 varchar(32)，無 CHECK）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerTaxEntityType implements CodeMeta {

    FARMER("FARMER", "FARMER", "農漁民／農戶", "個人從事農林漁牧生產"),
    SOLE_PROPRIETOR("SOLE_PROPRIETOR", "SOLE_PROPRIETOR", "獨資商號", "已辦商業登記的獨資事業"),
    PARTNERSHIP("PARTNERSHIP", "PARTNERSHIP", "合夥事業", "已辦商業登記的合夥事業"),
    COMPANY("COMPANY", "COMPANY", "公司", "依公司法設立"),
    OTHER("OTHER", "OTHER", "其他", "上述以外；ADM 審核時必須在 review_note 註明實際型態");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerTaxEntityType(String code, String value, String label, String description) {
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

    private static final Map<String, SellerTaxEntityType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxEntityType::code, e -> e));

    private static final Map<String, SellerTaxEntityType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerTaxEntityType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerTaxEntityType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerTaxEntityType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

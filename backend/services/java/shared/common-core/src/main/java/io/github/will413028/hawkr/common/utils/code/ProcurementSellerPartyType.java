package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家比較清單上那一列回價來自哪一種賣家身分（#2538 wire 契約 §6.3）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementSellerPartyType implements CodeMeta {

    PLATFORM_STORE("PLATFORM_STORE", "PLATFORM_STORE", "平台賣家", "有平台 store 身分，`storeCode` 可用於賣家頁連結"),
    EXTERNAL_VENDOR("EXTERNAL_VENDOR", "EXTERNAL_VENDOR", "站外業者", "來自站外名單，無平台 store；`displayName`／`storeCode` 皆為空字串");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementSellerPartyType(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementSellerPartyType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementSellerPartyType::code, e -> e));

    private static final Map<String, ProcurementSellerPartyType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementSellerPartyType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementSellerPartyType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementSellerPartyType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

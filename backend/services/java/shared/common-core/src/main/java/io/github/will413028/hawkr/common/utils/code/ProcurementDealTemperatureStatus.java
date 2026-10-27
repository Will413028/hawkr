package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 成交溫度計估計的可用性（見 `docs/詳細設計書/64` §2-5）。`AVAILABLE` 必須帶齊機率上下界、band 與四個版本欄位；`UNKNOWN` 不得帶任何機率或 band，只帶 `ProcurementDealTemperatureReason`。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementDealTemperatureStatus implements CodeMeta {

    AVAILABLE("AVAILABLE", "AVAILABLE", "可用", "估計成立，機率上下界、band 與版本欄位齊全"),
    UNKNOWN("UNKNOWN", "UNKNOWN", "未知", "估計不成立，原因見 reasonCode；不得顯示 0%、百分比或色帶");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementDealTemperatureStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementDealTemperatureStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementDealTemperatureStatus::code, e -> e));

    private static final Map<String, ProcurementDealTemperatureStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementDealTemperatureStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementDealTemperatureStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementDealTemperatureStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

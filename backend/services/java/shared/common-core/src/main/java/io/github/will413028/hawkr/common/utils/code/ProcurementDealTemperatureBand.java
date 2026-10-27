package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 成交溫度計的色帶，由 server 依 `probabilityUpperBps`（買家上限落在行情分布的第幾百分位）決定，**client 不得自行由 bps 推導**（v6.0 §V.13 第 18 條）。`BANDS_V1` 曾依 `probabilityLowerBps`（嚴格界），但那個量對買家是**反指標**——出價遠高於行情時它反而下降——已於 `BANDS_V2` 改正。門檻屬 `bandVersion` 管轄，改門檻不動計算層。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementDealTemperatureBand implements CodeMeta {

    COLD("COLD", "COLD", "冷", "買家上限落在行情第 25 百分位以下（`BANDS_V2`）"),
    WARM("WARM", "WARM", "溫", "買家上限落在行情第 25–75 百分位（`BANDS_V2`）"),
    HOT("HOT", "HOT", "熱", "買家上限落在行情第 75 百分位以上（`BANDS_V2`）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementDealTemperatureBand(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementDealTemperatureBand> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementDealTemperatureBand::code, e -> e));

    private static final Map<String, ProcurementDealTemperatureBand> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementDealTemperatureBand::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementDealTemperatureBand fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementDealTemperatureBand fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

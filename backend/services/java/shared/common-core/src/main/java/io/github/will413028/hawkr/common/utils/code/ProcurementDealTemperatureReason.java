package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 成交溫度計為何是 `UNKNOWN` 的原因碼。`MAPPING_MISSING` 與 `ProcurementErrorReason` 的同名值**刻意共用同一個字面值**（同一種「沒有可靠 mapping 可參考」語意，見該 enum 的既有說明），不另創同義詞。v6.0 V.7.3 的 `NO_RELIABLE_PRICEBOOK_MAPPING` 與 issue #2537 allowlist 的 `MAPPING_UNRELIABLE` 是同一概念的另外兩個名字，**不採用**——`MAPPING_MISSING` 已在 prod 的 wire 上。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementDealTemperatureReason implements CodeMeta {

    MAPPING_MISSING("MAPPING_MISSING", "MAPPING_MISSING", "缺乏可靠映射", "沒有可靠的 Pricebook 映射可參考，因此沒有預測分布可算涵蓋率；`BUYER_ENTERED_NO_REFERENCE` 恆為此值"),
    BUYER_RANGE_NOT_SET("BUYER_RANGE_NOT_SET", "BUYER_RANGE_NOT_SET", "買家尚未設定區間", "四種情形共用此碼：①買家未動過希望區間、server 以參考上下限預填（涵蓋率恆為 80%，與品項、城市、價格無關，零資訊量，故不顯示）②下限金額解析失敗 ③上限金額解析失敗 ④上下限顛倒。②③④ 出現在 PRICEBOOK_GUIDED 路徑——它不走 `validateBuyerRequestedRange`，故金額仍可能解不出來；對溫度計而言「讀不到買家區間」與「買家沒設區間」是同一件事"),
    RANGE_OUTSIDE_REFERENCE("RANGE_OUTSIDE_REFERENCE", "RANGE_OUTSIDE_REFERENCE", "區間落在參考之外", "買家希望區間完全高於或低於參考區間；結果將全由未知的尾部形狀決定，不給假精確的數字"),
    QUANTILES_NOT_MONOTONIC("QUANTILES_NOT_MONOTONIC", "QUANTILES_NOT_MONOTONIC", "分位數非嚴格遞增", "編譯取整後三個參考分位數未維持嚴格遞增，CDF 無定義。名稱沿用 pricebook-service `internal/domain/forecast/gate.go` 的同義原因碼");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementDealTemperatureReason(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementDealTemperatureReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementDealTemperatureReason::code, e -> e));

    private static final Map<String, ProcurementDealTemperatureReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementDealTemperatureReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementDealTemperatureReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementDealTemperatureReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

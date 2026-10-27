package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Pricebook 服務（P4）推論期的 7 態預測狀態機。既有欄位
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PredictionStatus implements CodeMeta {

    READY("READY", "ready", "已就緒", "通過品質閘門，分位數可信"),
    PARTIAL("PARTIAL", "partial", "部分覆蓋", "大類聚合下部分子品項有效"),
    MARKET_CLOSED("MARKET_CLOSED", "market_closed", "休市", "該日該市場休市"),
    INSUFFICIENT_DATA("INSUFFICIENT_DATA", "insufficient_data", "資料不足", "品質閘門未過，或休市日曆未驗證"),
    NOT_PREDICTED("NOT_PREDICTED", "not_predicted", "近期無成交", "該 series 近期完全沒有成交紀錄"),
    STALE("STALE", "stale", "已過期", "新鮮度落在 stale 區間；目前程式碼尚無實際指派此值的路徑，保留位"),
    UNAVAILABLE("UNAVAILABLE", "unavailable", "無法取得", "名稱未解析、模型未覆蓋，或新鮮度超過 unavailable 上限");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PredictionStatus(String code, String value, String label, String description) {
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

    private static final Map<String, PredictionStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PredictionStatus::code, e -> e));

    private static final Map<String, PredictionStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PredictionStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PredictionStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PredictionStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家端「這一版可不可以按接受」的 typed 判定（#2538 wire 契約 §6.3）。v6.0 V.11.5 逐字要求「withdrawn／expired／superseded 的 response 留在 audit 但不可接受，UI 取得 typed reason」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementQuoteAcceptability implements CodeMeta {

    ACCEPTABLE("ACCEPTABLE", "ACCEPTABLE", "可接受", "唯一能按下接受的狀態"),
    SUPERSEDED("SUPERSEDED", "SUPERSEDED", "已被改版取代", "賣家已提交更新的版本"),
    WITHDRAWN("WITHDRAWN", "WITHDRAWN", "已撤回", "賣家主動撤回"),
    EXPIRED("EXPIRED", "EXPIRED", "已逾有效期", "已過 `validUntil`"),
    INELIGIBLE("INELIGIBLE", "INELIGIBLE", "該賣家已停止回應", "賣家已拒絕，或該 publication 對他已結案停止寫入");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementQuoteAcceptability(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementQuoteAcceptability> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementQuoteAcceptability::code, e -> e));

    private static final Map<String, ProcurementQuoteAcceptability> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementQuoteAcceptability::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementQuoteAcceptability fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementQuoteAcceptability fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

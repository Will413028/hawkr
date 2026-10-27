package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * ADM 審核賣家稅務資格時的風險提示。**封閉三值，沒有「其他」**——沒有定義的風險標記等於沒有標記（58 §6.1.12 逐字）。⚠️ 這些是**提示不是駁回理由**（`58A` §B-3.5 逐字：`TAX_ID_MISMATCH_KYC` 只是「提醒有差異」）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceTaxRiskFlag implements CodeMeta {

    TAX_ID_MISMATCH_KYC("TAX_ID_MISMATCH_KYC", "TAX_ID_MISMATCH_KYC", "統編與實名資料不符", "申請填寫的統編與賣家 KYC 認證單上的統編不同；提示 ADM 追問，不是駁回理由"),
    REPEAT_APPLICATION("REPEAT_APPLICATION", "REPEAT_APPLICATION", "90 天內重複送審", "90 天內第 3 次以上送審（與決策 C3 的 7 天冷卻是兩件事：冷卻擋的是頻率，本旗標提示的是模式）"),
    OPEN_COMPLAINTS("OPEN_COMPLAINTS", "OPEN_COMPLAINTS", "有未結案發票申訴", "該賣家有 `invoice_complaints.status ∈ {OPEN, OVERDUE, SELLER_RESPONDED, ESCALATED_TO_ADM}` 的案件");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceTaxRiskFlag(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceTaxRiskFlag> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceTaxRiskFlag::code, e -> e));

    private static final Map<String, InvoiceTaxRiskFlag> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceTaxRiskFlag::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceTaxRiskFlag fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceTaxRiskFlag fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

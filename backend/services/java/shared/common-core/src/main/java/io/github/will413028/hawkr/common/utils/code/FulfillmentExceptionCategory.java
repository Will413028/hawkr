package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 維運例外案件的分類。🔴 **`RESPONSIBILITY_UNRESOLVED` 這一格與其他格的性質不同**：它不只是分類，還是一組 DB 不變式的觸發條件——該 category 的案件受 lifetime uniqueness（`uq_exc_responsibility_subject`）拘束、**不得** `DISMISSED`、結案碼只能是四個 resolved 的 `ResponsibilityCause`（`chk_exc_responsibility_no_dismiss` 與 `chk_exc_responsibility_resolved_cause`，V12）。⚠️ **本 enum 的值同時被寫進 V12 的索引述詞與兩道 CHECK**（schema 事實的複製），漂移由 `LogisticsSchemaContractTest` 逐字對拍守住。⚠️ 本表**不含** 69 號預約的六個商家整合類 category——那是該書實作階段的範圍，此處不預先佔位。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum FulfillmentExceptionCategory implements CodeMeta {

    RESPONSIBILITY_UNRESOLVED("RESPONSIBILITY_UNRESOLVED", "RESPONSIBILITY_UNRESOLVED", "責任判定未決", "🔴 配送失敗但成因證據不足，責任歸屬待 ADM 裁決。同一 fulfillment leg **終其一生只能有一張**；**不得** `DISMISSED`（責任判定沒有「不處理」這個結局——Order 正卡在 `35` 等它）"),
    LOGISTICS_STATEMENT_UNMATCHED("LOGISTICS_STATEMENT_UNMATCHED", "LOGISTICS_STATEMENT_UNMATCHED", "帳單列對不到應收", "provider 帳單列找不到唯一對應的應收（0 筆或多筆），或應收已結清後才到的費用"),
    LOGISTICS_PROVIDER_COST_DISPUTE("LOGISTICS_PROVIDER_COST_DISPUTE", "LOGISTICS_PROVIDER_COST_DISPUTE", "物流商成本爭議", "`PROVIDER_CAUSED` 的超額成本 ⇒ 先進爭議，🔴 **不得自動向賣家收**");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    FulfillmentExceptionCategory(String code, String value, String label, String description) {
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

    private static final Map<String, FulfillmentExceptionCategory> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentExceptionCategory::code, e -> e));

    private static final Map<String, FulfillmentExceptionCategory> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentExceptionCategory::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static FulfillmentExceptionCategory fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static FulfillmentExceptionCategory fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

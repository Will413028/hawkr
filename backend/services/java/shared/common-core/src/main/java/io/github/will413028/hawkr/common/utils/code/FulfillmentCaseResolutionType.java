package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 維運例外案件的處置提案型別。🔴 **型別與案件 category 綁定**：`RESPONSIBILITY_ATTRIBUTE` 只能用在 `RESPONSIBILITY_UNRESOLVED` 的案件上，其餘六值只能用在非責任判定案件上——**雙向**（65 §5.5.2 E0.2 的「第二扇門」）。拿 `CLAIM_DISMISS` 去結一張責任案件，會讓案件進入 `RESOLVED` 而 finality 條件永遠不成立 ⇒ `N16` 永不發出 ⇒ Order 永遠停在 `35`，且每一步都「照規則成功」、沒有任何錯誤訊號。⚠️ `chk_fcr_review_required` 逐字 `requires_review = (resolution_type <> 'ANOMALY_RESUME')` ⇒ **新型別自動就是 `requires_review = true`**，不得為了省一個人而放行單人結案（`APPROVED` 的雙人保證正是靠它）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum FulfillmentCaseResolutionType implements CodeMeta {

    RESPONSIBILITY_ATTRIBUTE("RESPONSIBILITY_ATTRIBUTE", "RESPONSIBILITY_ATTRIBUTE", "責任歸屬判定", "🔴 **唯一能結掉責任判定案件的型別**；`resolution_code` 必須是四個 resolved 的 `ResponsibilityCause` 之一（`RESPONSIBILITY_UNRESOLVED` 不是可結案的碼）。`APPROVED` ＋ 案件 `RESOLVED` 是 `fulfillment.responsibility.finalized` 的唯一產生點"),
    CLAIM_SETTLE("CLAIM_SETTLE", "CLAIM_SETTLE", "理賠結案", "物流理賠成立，依核定金額結案"),
    CLAIM_DISMISS("CLAIM_DISMISS", "CLAIM_DISMISS", "理賠駁回", "物流理賠不成立。🔴 **不得用於責任判定案件**"),
    SHIPMENT_CANCEL("SHIPMENT_CANCEL", "SHIPMENT_CANCEL", "取消物流單", "取消尚未取件的物流單"),
    SHIPMENT_CANCEL_CONFIRM("SHIPMENT_CANCEL_CONFIRM", "SHIPMENT_CANCEL_CONFIRM", "確認取消", "取消需二次確認的情境"),
    SHIPMENT_CANCEL_AND_REFUND("SHIPMENT_CANCEL_AND_REFUND", "SHIPMENT_CANCEL_AND_REFUND", "取消並退款", "取消物流單並觸發退款鏈"),
    ANOMALY_RESUME("ANOMALY_RESUME", "ANOMALY_RESUME", "異常恢復", "⚠️ 六值中唯一 `requires_review = false`（單人即可結案）⇒ 🔴 **絕不可用於責任判定案件**，那會繞過雙人程序要件");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    FulfillmentCaseResolutionType(String code, String value, String label, String description) {
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

    private static final Map<String, FulfillmentCaseResolutionType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentCaseResolutionType::code, e -> e));

    private static final Map<String, FulfillmentCaseResolutionType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(FulfillmentCaseResolutionType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static FulfillmentCaseResolutionType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static FulfillmentCaseResolutionType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

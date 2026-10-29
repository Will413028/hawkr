package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 管理員建立代行工作階段時必選的事由。Value 是小寫 snake_case（不同於多數 CodeMeta enum 的 UPPER_SNAKE）：ADM 以 opaque id 規則驗證並原字串回音比對，值一旦大寫化就 502。caseReferenceRequired 不是本 enum 的欄位，由 delegation 模組的純函式決定後隨 preflight 下發；CodeMeta enum 內禁止放業務輔助邏輯。Label 是後端唯一來源（preflight 的 reason.label 與 admin_operation_sessions.reason_label 都取自這裡），改這一格即全站生效。詳見 [56 §6.5.3](../詳細設計書/56-ADM%20代行營運（Delegated%20Operations）詳細設計書.md)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum DelegationReasonCode implements CodeMeta {

    SELLER_REQUEST("SELLER_REQUEST", "seller_request", "賣家請求（管理員記錄）", "需附案件參考。本平台沒有工單系統、賣家也沒有向平台提出請求並留痕的管道，這個事由是管理員的自我宣告，措辭不得暗示它是委託證據"),
    PLATFORM_MAINTENANCE("PLATFORM_MAINTENANCE", "platform_maintenance", "平台維護", "需附案件參考（#2749 起）。原本是三者中唯一的豁免，而全 repo 找不到任何理由；同一份設計在授權層對 `platform_policy` 並不豁免（`source_reference` 為 NOT NULL）"),
    CONTENT_GOVERNANCE("CONTENT_GOVERNANCE", "content_governance", "內容治理", "需附案件參考");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    DelegationReasonCode(String code, String value, String label, String description) {
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

    private static final Map<String, DelegationReasonCode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationReasonCode::code, e -> e));

    private static final Map<String, DelegationReasonCode> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationReasonCode::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static DelegationReasonCode fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static DelegationReasonCode fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

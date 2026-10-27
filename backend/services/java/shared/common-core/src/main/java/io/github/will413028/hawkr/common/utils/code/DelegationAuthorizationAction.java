package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * ADM 代行**授權列本身**的治理動作類型（誰、在什麼時候、把哪家店的代行授權開通／復權／改範圍／撤回）。與 `DelegationAuditAction`（工作階段內對賣家資源的操作）是兩個不同的稽核事實，落在兩張不同的表，不得互相取代。Code 與 Value 一致（UPPER_SNAKE_CASE，直接存讀可辨識文字），對齊 EventType 系列字串式 value 設計；寫入前以 `CodeMetaValidator.validate` 驗證。Label 由後端隨稽核列下發（前端不得自建對照表），故本 enum 列入 `scripts/generate.sh` 的 `GO_LABEL_ENUMS`。「哪些 action 必須帶 `source_reference`」這類判定不是本 enum 的欄位，由 account-service `internal/delegation/rules` 的純函式決定；CodeMeta enum 內禁止放業務輔助邏輯。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum DelegationAuthorizationAction implements CodeMeta {

    GRANT("GRANT", "GRANT", "開通授權", "該店此前沒有任何授權列，首次建立"),
    REINSTATE("REINSTATE", "REINSTATE", "重新開通授權", "對已撤回的授權列復權；`uq_sda_store` 一店終生一列，復權是 UPDATE 同一列而非新增第二列"),
    UPDATE("UPDATE", "UPDATE", "調整授權範圍", "對有效授權列全量取代 capability／依據／期限；打到已撤回的列一律拒絕（9264），不得靜默復權"),
    REVOKE("REVOKE", "REVOKE", "撤回授權", "撤回授權，並在同一 transaction 內連帶中止該店進行中的代行工作階段");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    DelegationAuthorizationAction(String code, String value, String label, String description) {
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

    private static final Map<String, DelegationAuthorizationAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationAuthorizationAction::code, e -> e));

    private static final Map<String, DelegationAuthorizationAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationAuthorizationAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static DelegationAuthorizationAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static DelegationAuthorizationAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

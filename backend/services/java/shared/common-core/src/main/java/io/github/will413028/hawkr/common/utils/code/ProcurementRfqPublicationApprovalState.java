package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * publication approval 的生命週期狀態（issue #2450 Scope 3）。approval 的**內容不可變、狀態可變**；每次狀態轉移一律伴隨 `version + 1`，`version` 是狀態轉移的 CAS token 而不是 data schema 版本（後者是 `schema_version` 欄）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementRfqPublicationApprovalState implements CodeMeta {

    PENDING("PENDING", "PENDING", "待執行", "preview 已建立核准意圖，尚未 execute"),
    EXECUTING("EXECUTING", "EXECUTING", "執行中", "execute 交易已 claim，尚未 commit"),
    SUCCEEDED("SUCCEEDED", "SUCCEEDED", "已發布（終態）", "execute 成功，publication 已建立；不代表已投遞或已被看到"),
    REJECTED("REJECTED", "REJECTED", "已拒絕（終態）", "買家主動放棄這次核准"),
    EXPIRED("EXPIRED", "EXPIRED", "已逾期（終態）", "超過 `expiresAt` 未 execute"),
    INVALIDATED("INVALIDATED", "INVALIDATED", "已失效（終態）", "買家改動了任一 authoritative fact，本核准不再對應現況"),
    FAILED("FAILED", "FAILED", "執行失敗（終態）", "保留值，目前無任何寫入端：單一交易設計下 execute rollback 會把 approval 還原成 `PENDING`（整合測試已證），不會留下 `FAILED`。要有此狀態需先有獨立於該交易的失敗落庫路徑");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementRfqPublicationApprovalState(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementRfqPublicationApprovalState> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRfqPublicationApprovalState::code, e -> e));

    private static final Map<String, ProcurementRfqPublicationApprovalState> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRfqPublicationApprovalState::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementRfqPublicationApprovalState fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementRfqPublicationApprovalState fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

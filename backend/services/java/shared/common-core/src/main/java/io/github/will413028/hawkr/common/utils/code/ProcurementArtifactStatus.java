package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Hawkr Agent typed artifact（候選結果、RFQ 預覽、報價批次結果等）的可用性狀態。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementArtifactStatus implements CodeMeta {

    DRAFT("DRAFT", "DRAFT", "草稿", "產製中或尚未完整，不可據以做決策"),
    READY("READY", "READY", "可用", "內容完整且在有效期內"),
    STALE("STALE", "STALE", "已過時", "依據的事實已更新，需重新產製後才可據以行動"),
    CONSUMED("CONSUMED", "CONSUMED", "已使用", "已被下游動作採用（例如據以寄出報價邀請）"),
    FAILED("FAILED", "FAILED", "產製失敗", "產製過程失敗，無可用內容");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementArtifactStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementArtifactStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementArtifactStatus::code, e -> e));

    private static final Map<String, ProcurementArtifactStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementArtifactStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementArtifactStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementArtifactStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

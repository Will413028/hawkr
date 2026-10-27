package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 需要人類確認的中斷點（interrupt／confirmation）狀態。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementConfirmationStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待確認", "等待使用者決策"),
    APPROVED("APPROVED", "APPROVED", "已核准（非終態）", "使用者已核准，尚未執行"),
    REJECTED("REJECTED", "REJECTED", "已拒絕（終態）", "使用者拒絕"),
    EXPIRED("EXPIRED", "EXPIRED", "已逾期（終態）", "超過 `expiresAt` 未決策"),
    INVALIDATED("INVALIDATED", "INVALIDATED", "已失效（終態）", "依據的 artifact 已過時，此確認不再可用"),
    EXECUTING("EXECUTING", "EXECUTING", "執行中", "核准後的副作用執行中"),
    SUCCEEDED("SUCCEEDED", "SUCCEEDED", "已完成（終態）", "副作用執行成功"),
    FAILED("FAILED", "FAILED", "執行失敗（終態）", "副作用執行失敗");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementConfirmationStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementConfirmationStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementConfirmationStatus::code, e -> e));

    private static final Map<String, ProcurementConfirmationStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementConfirmationStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementConfirmationStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementConfirmationStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

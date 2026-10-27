package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 單一步驟（`StepCode`，見 `services/ai-service/internal/procurement/domain/steps.go`）的執行狀態。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementStepStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待執行", "尚未開始，且無 work queue 語意（與 Run 的 QUEUED 區分）"),
    RUNNING("RUNNING", "RUNNING", "執行中", "進行中"),
    WAITING_FOR_INPUT("WAITING_FOR_INPUT", "WAITING_FOR_INPUT", "等待使用者輸入", "本步驟停在 interrupt"),
    SUCCEEDED("SUCCEEDED", "SUCCEEDED", "已完成", "正常結束（取代舊值 DONE）"),
    FAILED("FAILED", "FAILED", "失敗", "本步驟失敗"),
    SKIPPED("SKIPPED", "SKIPPED", "已略過", "條件不成立而未執行"),
    CANCELLED("CANCELLED", "CANCELLED", "已取消", "隨 run 取消而中止");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementStepStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementStepStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementStepStatus::code, e -> e));

    private static final Map<String, ProcurementStepStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementStepStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementStepStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementStepStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

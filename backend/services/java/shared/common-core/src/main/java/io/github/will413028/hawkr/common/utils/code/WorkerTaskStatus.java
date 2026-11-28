package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 通用 Worker 任務執行狀態。所有非同步重試任務（退款、清車、券核銷、撥款等）共用同一套狀態流：待執行 → 執行中 → 完成 / 失敗 / 人工介入。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/dev-guidelines/java-microservices-development-guidelines.md §6）。
 */
public enum WorkerTaskStatus implements CodeMeta {

    PENDING("PENDING", "0", "待執行", "任務已建立，等待 Worker 處理"),
    PROCESSING("PROCESSING", "10", "執行中", "Worker 已 Claim，執行任務中"),
    DONE("DONE", "20", "已完成", "任務成功完成"),
    FAILED("FAILED", "30", "失敗", "任務失敗（Worker retry 後仍失敗）"),
    MANUAL_INTERVENTION("MANUAL_INTERVENTION", "40", "人工介入", "業務策略明確轉人工（Phase 1 退款任務目前不寫此值，既有歷史資料保留）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    WorkerTaskStatus(String code, String value, String label, String description) {
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

    private static final Map<String, WorkerTaskStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WorkerTaskStatus::code, e -> e));

    private static final Map<String, WorkerTaskStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WorkerTaskStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static WorkerTaskStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static WorkerTaskStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

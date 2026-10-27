package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 候選搜尋的單一通道（PLATFORM／EXTERNAL）狀態。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementLaneStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待執行", "尚未開始"),
    RUNNING("RUNNING", "RUNNING", "搜尋中", "本通道搜尋進行中"),
    SUCCEEDED("SUCCEEDED", "SUCCEEDED", "已完成", "本通道正常結束（取代舊值 OK）"),
    FAILED("FAILED", "FAILED", "失敗", "本通道失敗；另一通道可獨立成功"),
    SKIPPED("SKIPPED", "SKIPPED", "已略過", "條件不成立而未執行本通道"),
    CANCELLED("CANCELLED", "CANCELLED", "已取消", "隨 run 取消而中止");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementLaneStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementLaneStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementLaneStatus::code, e -> e));

    private static final Map<String, ProcurementLaneStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementLaneStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementLaneStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementLaneStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

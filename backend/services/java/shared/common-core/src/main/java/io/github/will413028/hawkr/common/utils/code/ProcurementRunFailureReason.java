package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * run 進入 `FAILED`／`EXPIRED` 時的機器可讀原因。**只在這兩個狀態下非空**。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementRunFailureReason implements CodeMeta {

    WORKER_LOST("WORKER_LOST", "WORKER_LOST", "執行程序中斷", "執行 run 的程序消失（lease 過期／pod 重啟），由讀取路徑終態化"),
    LANE_ALL_FAILED("LANE_ALL_FAILED", "LANE_ALL_FAILED", "全部通道失敗", "PLATFORM 與 EXTERNAL 兩條 lane 皆 FAILED"),
    TURN_PIPELINE_ERROR("TURN_PIPELINE_ERROR", "TURN_PIPELINE_ERROR", "Turn pipeline 執行失敗", "TURN run 的對話 pipeline（抽取／驗證／持久化）中途回錯，run 以 FAILED 終態化。刻意只有這一個粗粒度的值：pipeline 內部的失敗原因不得展開成自由文字或高基數分類，否則 `procurement_run_total` 的 label cardinality 會爆（見本組「值域為 open」那條）；細節走 log／trace，不上這個維度");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementRunFailureReason(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementRunFailureReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRunFailureReason::code, e -> e));

    private static final Map<String, ProcurementRunFailureReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRunFailureReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementRunFailureReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementRunFailureReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

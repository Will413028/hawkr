package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Hawkr Agent（採購助手）**一次 agent run 的生命週期狀態**。issue #2446 對外契約凍結的一部分。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementRunStatus implements CodeMeta {

    QUEUED("QUEUED", "QUEUED", "排隊中", "命令已接受並落地，尚未取得執行資源。與 Step/Lane 的 PENDING 區分：QUEUED 代表背後有真實 work queue"),
    RUNNING("RUNNING", "RUNNING", "執行中", "執行器已開始處理"),
    WAITING_FOR_INPUT("WAITING_FOR_INPUT", "WAITING_FOR_INPUT", "等待使用者輸入", "停在 interrupt，等待買家回答釐清問題或確認"),
    CANCEL_REQUESTED("CANCEL_REQUESTED", "CANCEL_REQUESTED", "取消處理中", "已收到取消命令，尚未收斂到終態"),
    SUCCEEDED("SUCCEEDED", "SUCCEEDED", "已完成（終態）", "正常結束"),
    FAILED("FAILED", "FAILED", "已失敗（終態）", "非正常結束，原因見 ProcurementRunFailureReason"),
    CANCELLED("CANCELLED", "CANCELLED", "已取消（終態）", "取消命令已生效"),
    RESUMED("RESUMED", "RESUMED", "已接續（終態）", "本 run 交棒給 child run，接續者見 `resumedIntoRunId`"),
    EXPIRED("EXPIRED", "EXPIRED", "已逾期（終態）", "超過存活上限被終態化");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementRunStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementRunStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRunStatus::code, e -> e));

    private static final Map<String, ProcurementRunStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementRunStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementRunStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementRunStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

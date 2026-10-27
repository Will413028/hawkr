package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 通用 Worker 任務執行狀態。所有非同步重試任務（退款、清車、券核銷、撥款等）共用同一套狀態流：待執行 → 執行中 → 完成 / 失敗 / 人工介入。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum WorkerTaskStatus implements CodeMeta {

    PENDING("PENDING", "0", "待執行", "任務已建立，等待 Worker 處理"),
    AWAITING_BUYER_REFUND_PAGE("AWAITING_BUYER_REFUND_PAGE", "5", "等待買家操作退款頁面", "ATM 退款，等待買家填寫銀行帳號（僅退款任務使用）"),
    PROCESSING("PROCESSING", "10", "執行中", "Worker 已 Claim，執行任務中"),
    AWAITING_WEBHOOK("AWAITING_WEBHOOK", "15", "等待回調", "ATM 退款已受理，等待 PayUni Webhook 匯款結果（僅退款任務使用）"),
    DONE("DONE", "20", "已完成", "任務成功完成"),
    FAILED("FAILED", "30", "失敗", "任務失敗（信用卡路徑經 Worker retry 1 次後仍失敗 / ATM 路徑 webhook 回報匯款失敗）"),
    MANUAL_INTERVENTION("MANUAL_INTERVENTION", "40", "人工介入", "業務策略明確轉人工（Phase 1 退款任務目前不寫此值，既有歷史資料保留）"),
    PROVIDER_AUTO("PROVIDER_AUTO", "50", "提供方主動撥款", "PayUni 已對退款交易接管撥款；唯一進入 = GrantPayment 回應 `grant_outcome=PROVIDER_TOOK_OVER` 後由 Worker 收斂，不再重試，任務僅作對帳記錄（終態，僅撥款任務使用；03 §3.11.2 / 05 §5.6.1）");

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

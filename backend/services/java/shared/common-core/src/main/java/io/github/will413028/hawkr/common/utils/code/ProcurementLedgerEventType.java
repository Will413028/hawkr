package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * run ledger 單筆事件的類型（doc 57 §2.2「三層分離」的 **Ledger event type** 層），是 browser replay 與 public audit projection 的 SSOT，比 wire carrier（AG-UI frame）豐富。**不等同**於 browser 實際收到的 frame type——原生 AG-UI 型別與 `CUSTOM name=<slug>` 的對應關係、以及本值域與 wire carrier 的完整對照見 doc 57 §4.2。Event ledger 是 browser replay 與 public audit projection，**不是** order／payment／RFQ 或財務 ledger（doc 57 §1.1-6）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementLedgerEventType implements CodeMeta {

    RUN_ACCEPTED("RUN_ACCEPTED", "RUN_ACCEPTED", "Run 已接受", "command／buyer message／private run input／run row／首筆 event 已同一 transaction commit，worker 尚未 claim；`CUSTOM` 承載"),
    RUN_STARTED("RUN_STARTED", "RUN_STARTED", "Run 開始執行", "executor 已 claim 並開始處理；wire carrier 為原生 `RUN_STARTED`"),
    RUN_WAITING("RUN_WAITING", "RUN_WAITING", "Run 等待輸入", "停在 interrupt，等待買家回答釐清問題或確認；`CUSTOM` 承載"),
    RUN_CANCEL_REQUESTED("RUN_CANCEL_REQUESTED", "RUN_CANCEL_REQUESTED", "取消已請求", "已收到取消命令，尚未收斂到終態；`CUSTOM` 承載"),
    RUN_FINISHED("RUN_FINISHED", "RUN_FINISHED", "Run 正常結束", "終態；wire carrier 為原生 `RUN_FINISHED`，`outcome` 為嚴格 discriminated union（`success`／`interrupt`），三個消費端（gateway／website／未來 replay）皆須讀 `outcome`"),
    RUN_FAILED("RUN_FAILED", "RUN_FAILED", "Run 失敗", "終態 ledger event；`CUSTOM` 承載。與 run status `FAILED`、AG-UI wire 的 `RUN_ERROR` frame 型別三者不同軸，不得混稱（doc 57 §4.3）"),
    RUN_CANCELLED("RUN_CANCELLED", "RUN_CANCELLED", "Run 已取消", "終態；取消命令已生效；`CUSTOM` 承載"),
    RUN_RESUMED("RUN_RESUMED", "RUN_RESUMED", "Run 已接續（handoff）", "終態；parent run 交棒給 child run，接續者見 `resumedIntoRunId`；`CUSTOM` 承載"),
    RUN_EXPIRED("RUN_EXPIRED", "RUN_EXPIRED", "Run 已逾期", "終態；超過存活上限被終態化；`CUSTOM` 承載"),
    STEP_STARTED("STEP_STARTED", "STEP_STARTED", "步驟開始", "`CUSTOM name=step_detail` 承載；原生 AG-UI `STEP_STARTED` 可並送但 adapter 的 26 個 case 不含它，不可依賴它顯示任何東西（doc 57 §2.3）"),
    STEP_FINISHED("STEP_FINISHED", "STEP_FINISHED", "步驟完成", "同上，`CUSTOM name=step_detail`"),
    TOOL_STARTED("TOOL_STARTED", "TOOL_STARTED", "工具呼叫開始", "wire carrier 為原生 `TOOL_CALL_START`／`TOOL_CALL_ARGS`"),
    TOOL_FINISHED("TOOL_FINISHED", "TOOL_FINISHED", "工具呼叫完成", "wire carrier 為原生 `TOOL_CALL_RESULT`／`TOOL_CALL_END`"),
    AGENT_ACTIVITY_SNAPSHOT("AGENT_ACTIVITY_SNAPSHOT", "AGENT_ACTIVITY_SNAPSHOT", "Activity 全量快照", "`CUSTOM name=activity`；刻意加 `AGENT_` 前綴避免與 AG-UI 原生 `ACTIVITY_SNAPSHOT` 同名不同義——後者已被 doc 57 §2.3 明確禁用（官方 adapter 只對 A2UI surface 與 MCP apps 特例處理，其餘一律 `break` 丟棄）"),
    AGENT_ACTIVITY_DELTA("AGENT_ACTIVITY_DELTA", "AGENT_ACTIVITY_DELTA", "Activity 增量更新", "同上，`CUSTOM name=activity`；避免與 AG-UI 原生 `ACTIVITY_DELTA` 同名不同義"),
    MESSAGE_COMPLETED("MESSAGE_COMPLETED", "MESSAGE_COMPLETED", "訊息完成", "wire carrier 為原生 `TEXT_MESSAGE_START`／`TEXT_MESSAGE_CONTENT`／`TEXT_MESSAGE_END`"),
    ARTIFACT_UPSERTED("ARTIFACT_UPSERTED", "ARTIFACT_UPSERTED", "Artifact 已建立或更新", "`CUSTOM name=artifact`；型別值域見 `ProcurementArtifactType`（業務類型碼.md）"),
    CONFIRMATION_REQUIRED("CONFIRMATION_REQUIRED", "CONFIRMATION_REQUIRED", "需要人類確認", "`CUSTOM name=confirmation`；對應 `ProcurementConfirmationStatus.PENDING`"),
    CONFIRMATION_RESOLVED("CONFIRMATION_RESOLVED", "CONFIRMATION_RESOLVED", "確認已解決", "同上，`CUSTOM name=confirmation`；對應 `ProcurementConfirmationStatus` 的非 `PENDING` 值");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementLedgerEventType(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementLedgerEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementLedgerEventType::code, e -> e));

    private static final Map<String, ProcurementLedgerEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementLedgerEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementLedgerEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementLedgerEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

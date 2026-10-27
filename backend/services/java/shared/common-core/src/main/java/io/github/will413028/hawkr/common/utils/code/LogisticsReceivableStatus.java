package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家物流應收的生命週期（承 70 §3.2）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum LogisticsReceivableStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待收款", "應收成立，尚未送出 `agent_fee`。進←送達事件 consumer／ADM 授權重送；走←提交 worker"),
    SUBMITTED("SUBMITTED", "SUBMITTED", "已送出待入帳", "已送出 `agent_fee`，**銀行次日執行**。⚠️ 前端文案要寫「銀行次日執行」不是「處理中」——後者讓賣家以為是我方在跑"),
    SETTLED("SETTLED", "SETTLED", "已收款", "`agent_fee_query` 回 `FundDate` 有值 ⇒ 確認入帳。🔴 **不是終態**，still 要 true-up"),
    UNCONFIRMED("UNCONFIRMED", "UNCONFIRMED", "收款結果未確認", "🔴 超過期望撥款日＋寬限，`FundDate` 仍為 `0000-00-00`。**不自動重送，一律進 ADM 人工**。出邊三選一：查證收到→`SETTLED`／確認未受理→`PENDING`（開新 attempt ＋ 新 `trade_no`）／查不出來→`WRITTEN_OFF`"),
    FAILED("FAILED", "FAILED", "送出失敗", "**送出當下** `Status ≠ SUCCESS`（明確失敗，不存在「可能已經扣了」的窗口）⇒ 可自動退避重送。⚠️ 與 `UNCONFIRMED` 的差別是「有沒有不確定窗口」，不是「失敗嚴不嚴重」"),
    CLOSED("CLOSED", "CLOSED", "已結清", "true-up 完成（終態）"),
    WRITTEN_OFF("WRITTEN_OFF", "WRITTEN_OFF", "已核銷", "人工核銷（終態），強制填理由");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    LogisticsReceivableStatus(String code, String value, String label, String description) {
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

    private static final Map<String, LogisticsReceivableStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LogisticsReceivableStatus::code, e -> e));

    private static final Map<String, LogisticsReceivableStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(LogisticsReceivableStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static LogisticsReceivableStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static LogisticsReceivableStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

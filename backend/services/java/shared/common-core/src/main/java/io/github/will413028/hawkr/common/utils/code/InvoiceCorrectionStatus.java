package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家付款後修改發票資料的請求狀態（**6 值**）。與 provider 層的 InvoiceOperationStatus 分開：本值域是「買家看得到的處理進度」，operation 才是「對 AMEGO 的一次呼叫」。⚠️ **沒有 VOID_FAILED**（2026-08-25 依 58 §7.3.6 自值域刪除）：它是陷阱狀態，R05 進得去而兩條出邊 R06／R07 全書沒有任何端點、排程或 MQ consumer 走得到 ⇒ 作廢被 AMEGO 拒絕的請求會永久卡在非終態，而 uq_invcorr_open_per_document 會讓買家再也提不出第二次修改。定版改為 **R05 直接落 REJECTED ＋ reject_reason_code**（三個 f0501 回應碼各自對應一個已存在的拒絕理由，可自動判定、不需人來裁決）。**不要因為「作廢失敗與預檢不通過是兩件事」就把它加回來**——買家看到的答案是同一句，差別只在發現的時點。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceCorrectionStatus implements CodeMeta {

    REQUESTED("REQUESTED", "REQUESTED", "已送出", "買家送出，等待預檢"),
    REJECTED("REJECTED", "REJECTED", "已婉拒", "預檢不通過或確認無法修改；reject_reason_code 必填（終態）"),
    UPDATED_BEFORE_ISSUE("UPDATED_BEFORE_ISSUE", "UPDATED_BEFORE_ISSUE", "開立前已更新", "尚未送出 AMEGO，直接改快照並遞增 snapshot_version（終態）"),
    VOID_REISSUE_IN_PROGRESS("VOID_REISSUE_IN_PROGRESS", "VOID_REISSUE_IN_PROGRESS", "作廢重開處理中", "已開立，預檢通過，正在作廢並重開"),
    REISSUE_FAILED("REISSUE_FAILED", "REISSUE_FAILED", "重開失敗（買家手上無憑證）", "**最高優先級**：原發票已作廢、新發票不存在。獨立 ADM 佇列 ＋ 15 分鐘告警。⚠️ 與已刪除的 VOID_FAILED 的根本差別：本值買家手上一張憑證都沒有，作廢被拒時原發票仍然有效"),
    COMPLETED("COMPLETED", "COMPLETED", "已完成", "作廢與重開皆成功，原件已轉 SUPERSEDED（終態）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceCorrectionStatus(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceCorrectionStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCorrectionStatus::code, e -> e));

    private static final Map<String, InvoiceCorrectionStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCorrectionStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceCorrectionStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceCorrectionStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

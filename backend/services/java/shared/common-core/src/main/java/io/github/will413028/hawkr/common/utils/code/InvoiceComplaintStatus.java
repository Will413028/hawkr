package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家「沒收到發票」申訴案件的狀態，含 3 個工作天 SLA。**UI 的分頁是視圖不是狀態**：S09 三分頁與 A07 五分頁都是對本值域的查詢定義，不各自維護狀態欄。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceComplaintStatus implements CodeMeta {

    OPEN("OPEN", "OPEN", "待賣家處理", "建案或被 ADM 退回重辦；SLA 倒數中"),
    SELLER_RESPONDED("SELLER_RESPONDED", "SELLER_RESPONDED", "賣家已回覆", "買家有 72 小時表示異議，逾期視為接受"),
    OVERDUE("OVERDUE", "OVERDUE", "已逾期", "超過 3 個工作天未回覆；24 小時後自動升級 ADM"),
    ESCALATED_TO_ADM("ESCALATED_TO_ADM", "ESCALATED_TO_ADM", "待平台判定", "賣家請求協助／買家不接受／逾期自動升級"),
    RESOLVED("RESOLVED", "RESOLVED", "已解決", "ADM 判定已解決、買家確認、或系統偵測發票已送達。**非終態**（可於 24 小時後重開，最多 2 次）"),
    VIOLATION_CONFIRMED("VIOLATION_CONFIRMED", "VIOLATION_CONFIRMED", "判定賣家違規", "計入 90 天違規窗口；發票模組只發建議，停權處分由 seller-service 執行（終態）"),
    WITHDRAWN("WITHDRAWN", "WITHDRAWN", "買家已撤回", "不計入違規計數（終態）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceComplaintStatus(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceComplaintStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceComplaintStatus::code, e -> e));

    private static final Map<String, InvoiceComplaintStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceComplaintStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceComplaintStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceComplaintStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

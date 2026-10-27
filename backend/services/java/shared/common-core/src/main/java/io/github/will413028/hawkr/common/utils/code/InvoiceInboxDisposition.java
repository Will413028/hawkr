package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一則入向事件「不處理」時的機器可讀理由（**5 值**）。它是 `SKIPPED` 這個狀態的必填配對——沒有它，「這則事件為什麼沒有下文」在事後只能靠猜。🔴 **本 enum 的存在理由是 runbook 指名要它**：`docs/runbooks/invoice-inbox-stranded-runbook.md` §4 逐字寫「`disposition` 值要先在 CodeMeta 登記再用，不要直接寫字面值進 DB」，而在本次登記之前那個區塊**並不存在** ⇒ 照 runbook 施工的人會找不到要登記的地方，然後直接寫字面值。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceInboxDisposition implements CodeMeta {

    IGNORED_PRE_CUTOVER("IGNORED_PRE_CUTOVER", "IGNORED_PRE_CUTOVER", "服務上線前的積壓", "對應訂單的 ORDER_PAID 從未進過本服務 inbox ⇒ 沒有 document 可作廢或折讓，且**永遠不可認領**（反向認領等的是一則不會來的事件）。退款本身不受影響。寫入端是 runbook 的一次性維運 SQL"),
    IGNORED_DUPLICATE_PAYMENT("IGNORED_DUPLICATE_PAYMENT", "IGNORED_DUPLICATE_PAYMENT", "重複付款事件", "同一筆訂單的付款事件重覆抵達且已有 document；去重鍵已擋下，本列只記錄處置"),
    NOT_AMEGO_AUTO("NOT_AMEGO_AUTO", "NOT_AMEGO_AUTO", "非平台自動開立", "該 document 的 fulfillment_mode 不是 AMEGO_AUTO，或 document_kind 是 RECEIPT ⇒ 調整不由本服務執行（賣家自行處理／只重開收據）。**退款本身不受影響**"),
    NO_ADJUSTABLE_DOCUMENT("NO_ADJUSTABLE_DOCUMENT", "NO_ADJUSTABLE_DOCUMENT", "沒有可調整的發票", "document 已在 VOIDED／SUPERSEDED／CANCELLED_BEFORE_ISSUE／NOT_APPLICABLE／RESOLVED_EXTERNALLY 之一。細節（是哪一個狀態）寫在 disposition_reason"),
    ALLOWANCE_CAP_REACHED("ALLOWANCE_CAP_REACHED", "ALLOWANCE_CAP_REACHED", "累計折讓已達發票金額", "這張發票的累計折讓已經等於 total_amount（法定不變式的上限），可折讓餘額為 0 ⇒ 這一筆退款沒有東西可以開。**退款本身不受影響**，而且這是**真實業務異常**不是技術故障：成因通常是同一張訂單開了兩張售後單而 refundable_amount 沒有互扣（58 §6.3.2.3）。同時刻必有 `invoice.allowance.refund_exceeds_invoiced` 的 Error log 與 metric `invoice_refund_exceeds_invoiced_total`，「是哪一家、哪一張」看那則 log");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceInboxDisposition(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceInboxDisposition> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceInboxDisposition::code, e -> e));

    private static final Map<String, InvoiceInboxDisposition> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceInboxDisposition::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceInboxDisposition fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceInboxDisposition fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

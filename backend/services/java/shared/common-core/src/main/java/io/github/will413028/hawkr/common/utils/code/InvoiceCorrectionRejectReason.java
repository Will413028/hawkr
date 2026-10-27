package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家申請修改發票資料被婉拒的理由（58 §7.3.5 的 **R02** 與 **R05**）。兩條邊共用同一組值域，差別只在**發現的時點**——R02 是送出當下的預檢就擋掉，R05 是送出後 AMEGO 才回報。⚠️ **前三個值同時是 AMEGO `f0501` 三個回應碼的一一對映**：`3050123`→ALREADY_REPORTED、`3050126`→PAST_VOID_DEADLINE、`3050141`→ALLOWANCE_EXISTS，改動值域前先確認那個對映還成立。B06 顯示的是**後端給的具體理由**，不由前端拼字串。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum InvoiceCorrectionRejectReason implements CodeMeta {

    ALREADY_REPORTED("ALREADY_REPORTED", "ALREADY_REPORTED", "發票已申報", "已隨期別申報，依規定不得作廢。🔴 **由 R02 的預檢寫入**（`ReportedStatus` 是純函式，我方自己算得出期別）——它**沒有**對應的 AMEGO 錯誤碼。2026-09-01（DEC-20260831-234750）前本欄誤記為對應 `3050123`，而該碼的官方語意是「已存在註銷」，已改對應 `PROVIDER_STATE_CONFLICT`"),
    PAST_VOID_DEADLINE("PAST_VOID_DEADLINE", "PAST_VOID_DEADLINE", "已逾可作廢期限", "超過法定可作廢期間；對應 AMEGO f0501 的 3050126"),
    ALLOWANCE_EXISTS("ALLOWANCE_EXISTS", "ALLOWANCE_EXISTS", "已存在折讓", "該張發票已有折讓單，作廢會使折讓失去依據；對應 AMEGO f0501 的 3050141"),
    TRACK_INSUFFICIENT("TRACK_INSUFFICIENT", "TRACK_INSUFFICIENT", "字軌餘量不足", "重開需要一個新號碼而目前配號不足；先確保可開再作廢，故在作廢前就擋下"),
    CARRIER_INVALID("CARRIER_INVALID", "CARRIER_INVALID", "載具無效", "更正後的載具驗證不通過，重開會失敗"),
    PROVIDER_UNAVAILABLE("PROVIDER_UNAVAILABLE", "PROVIDER_UNAVAILABLE", "服務商暫時無法連線", "電子發票服務商當下不可用；買家稍後可再申請一次（REJECTED 是終態且不佔 uq_invcorr_open_per_document 的名額）"),
    PROVIDER_STATE_CONFLICT("PROVIDER_STATE_CONFLICT", "PROVIDER_STATE_CONFLICT", "發票狀態衝突，請聯繫客服", "AMEGO f0501 回 `3050123`「指定發票號碼**已存在註銷**」——那張發票在服務商端已有一筆註銷紀錄，我方的作廢因此走不下去。⚠️ 這**不是**「已申報」：註銷與作廢是兩種不同動作（AMEGO 另有 `3050122`「已存在開立作廢」，我方視為成功等價）。**買家手上的原發票不受影響**，只是內容改不了");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    InvoiceCorrectionRejectReason(String code, String value, String label, String description) {
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

    private static final Map<String, InvoiceCorrectionRejectReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCorrectionRejectReason::code, e -> e));

    private static final Map<String, InvoiceCorrectionRejectReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(InvoiceCorrectionRejectReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static InvoiceCorrectionRejectReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static InvoiceCorrectionRejectReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

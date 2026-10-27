package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * **定期配送軸**（`recurring_contract`／`recurring_cycle`）的領域動作值域，6 值。🔴 **與 `FulfillmentAction` 是兩個 enum、兩個資源軸**——兩者都有 `cancel` 但對象不同（配送單 vs 訂閱契約），前端的通用 CTA 元件**必須以資源軸為第一層分派**，否則兩軸的同名動作會互相覆蓋。⚠️ `approve_amount` 統一為 snake_case（原設計文字為 kebab-case）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RecurringAction implements CodeMeta {

    PAUSE("PAUSE", "pause", "暫停訂閱", "暫停後續週期"),
    RESUME("RESUME", "resume", "恢復訂閱", "恢復已暫停的訂閱"),
    CANCEL("CANCEL", "cancel", "取消訂閱", "🔴 對象是**契約**不是配送單，文案不得與 FulfillmentAction.CANCEL 共用"),
    SKIP("SKIP", "skip", "跳過本期", "跳過單一週期"),
    RESCHEDULE("RESCHEDULE", "reschedule", "改期", "調整本期配送日"),
    APPROVE_AMOUNT("APPROVE_AMOUNT", "approve_amount", "同意超額", "本期金額超出授權額度時的買家同意");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RecurringAction(String code, String value, String label, String description) {
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

    private static final Map<String, RecurringAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RecurringAction::code, e -> e));

    private static final Map<String, RecurringAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RecurringAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RecurringAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RecurringAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni 互動稽核紀錄的方向判別。`OUTBOUND` = 我方 → PayUni（API 呼叫）；`INBOUND` = PayUni → 我方（webhook / return 導回解密後）。Code 與 Value 一致（UPPER_SNAKE_CASE，直接存讀可辨識文字），對齊 EventType 系列字串式 value 設計；寫入前以 `CodeMetaValidator.validate(PayUniAuditDirection.class, ...)` 驗證。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum PayUniAuditDirection implements CodeMeta {

    OUTBOUND("OUTBOUND", "OUTBOUND", "出向", "我方 → PayUni 的 API 呼叫（含成功與各類失敗；由 payuni-java-client `onExchange` hook 或 ShipmentLabelService 手動補寫落庫）"),
    INBOUND("INBOUND", "INBOUND", "入向", "PayUni → 我方（webhook / return 導回）解密後的稽核紀錄");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    PayUniAuditDirection(String code, String value, String label, String description) {
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

    private static final Map<String, PayUniAuditDirection> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayUniAuditDirection::code, e -> e));

    private static final Map<String, PayUniAuditDirection> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(PayUniAuditDirection::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static PayUniAuditDirection fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static PayUniAuditDirection fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

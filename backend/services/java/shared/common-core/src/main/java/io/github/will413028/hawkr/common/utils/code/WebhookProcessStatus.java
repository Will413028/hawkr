package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni Webhook 訊息處理結果。獨立於 PaymentStatus / StockTxStatus 等其他 SUCCESS/FAILED 命名空間——本枚舉表達「webhook 訊息處理結果」，後者是「付款交易狀態」。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum WebhookProcessStatus implements CodeMeta {

    PENDING("PENDING", "8201", "待處理", "webhook 已收到，尚未處理"),
    SUCCESS("SUCCESS", "8202", "處理成功", "webhook 處理成功"),
    FAILED("FAILED", "8203", "處理失敗", "webhook 處理失敗"),
    SKIPPED("SKIPPED", "8204", "跳過處理", "交易已終態等情況"),
    VERIFY_FAILED("VERIFY_FAILED", "8205", "簽章驗證失敗", "PayUni 簽章驗證未通過"),
    DECRYPT_FAILED("DECRYPT_FAILED", "8206", "解密失敗", "PayUni payload 解密失敗");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    WebhookProcessStatus(String code, String value, String label, String description) {
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

    private static final Map<String, WebhookProcessStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookProcessStatus::code, e -> e));

    private static final Map<String, WebhookProcessStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookProcessStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static WebhookProcessStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static WebhookProcessStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

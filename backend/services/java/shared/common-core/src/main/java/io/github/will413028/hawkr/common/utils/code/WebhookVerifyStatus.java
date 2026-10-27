package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * PayUni Webhook 簽章驗證結果。獨立於 WebhookProcessStatus.VERIFY_FAILED（後者是處理整體結果，本枚舉只表達簽章驗證階段的二元結果）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum WebhookVerifyStatus implements CodeMeta {

    PASSED("PASSED", "8231", "驗證通過", "簽章驗證通過"),
    FAILED("FAILED", "8232", "驗證失敗", "簽章驗證失敗");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    WebhookVerifyStatus(String code, String value, String label, String description) {
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

    private static final Map<String, WebhookVerifyStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookVerifyStatus::code, e -> e));

    private static final Map<String, WebhookVerifyStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookVerifyStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static WebhookVerifyStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static WebhookVerifyStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

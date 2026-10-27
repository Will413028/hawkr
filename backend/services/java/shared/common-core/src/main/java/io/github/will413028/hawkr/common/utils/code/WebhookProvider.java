package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Webhook 收件來源（**服務商分類，不限金流商**）。表級分類，回答「這筆 webhook 來自哪個服務商」。獨立於 PayUniEventType（後者表達 PayUni 內部事件子類別），抽象層級不同。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum WebhookProvider implements CodeMeta {

    PAYUNI("PAYUNI", "8241", "統一金流", "PayUni（統一金流）所有 webhook"),
    UBER("UBER", "8242", "Uber Direct", "Uber Direct 即時配送所有 webhook（`delivery_status`／`courier_update`／`refund_request`）；66 §5.1.6／§6.3.4");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    WebhookProvider(String code, String value, String label, String description) {
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

    private static final Map<String, WebhookProvider> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookProvider::code, e -> e));

    private static final Map<String, WebhookProvider> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookProvider::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static WebhookProvider fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static WebhookProvider fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * payuni-webhook-service 的 inbox 收件箱狀態。表達「收件流程的階段」，與 WebhookProcessStatus（payment-service 處理結果）為**不同層級**：前者紀錄 webhook-service 自身的收件→去重→入隊流程，後者紀錄下游 payment-service 的處理結果。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum WebhookInboxStatus implements CodeMeta {

    RECEIVED("RECEIVED", "8211", "已接收", "webhook 已寫入 inbox，尚未處理"),
    DEDUPED("DEDUPED", "8212", "已去重", "同 event_key 已存在，跳過處理"),
    ENQUEUED("ENQUEUED", "8213", "已入隊", "已寫入 outbox 待 MQ 投遞"),
    FAILED("FAILED", "8214", "接收失敗", "入庫或解析過程失敗"),
    IGNORED("IGNORED", "8215", "已忽略", "UNKNOWN webhookType 已稽核，故意不投遞 MQ（terminal 狀態，不參與 replay）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    WebhookInboxStatus(String code, String value, String label, String description) {
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

    private static final Map<String, WebhookInboxStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookInboxStatus::code, e -> e));

    private static final Map<String, WebhookInboxStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(WebhookInboxStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static WebhookInboxStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static WebhookInboxStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

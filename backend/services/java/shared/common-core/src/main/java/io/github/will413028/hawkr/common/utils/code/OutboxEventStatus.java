package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Outbox pattern 投遞狀態，跨服務共用（payuni-webhook 與 payment-service 的 outbox_events、order-service 的 order_outbox 皆使用同一組值）。獨立於 WebhookInboxStatus（後者表達收件階段）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum OutboxEventStatus implements CodeMeta {

    PENDING("PENDING", "8221", "待投遞", "待 MQ 投遞"),
    SENT("SENT", "8222", "已投遞", "已成功投遞至 MQ"),
    FAILED("FAILED", "8223", "投遞失敗", "投遞失敗，可重試"),
    DEAD("DEAD", "8224", "已死信", "重試上限後進入死信");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    OutboxEventStatus(String code, String value, String label, String description) {
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

    private static final Map<String, OutboxEventStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OutboxEventStatus::code, e -> e));

    private static final Map<String, OutboxEventStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OutboxEventStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static OutboxEventStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static OutboxEventStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

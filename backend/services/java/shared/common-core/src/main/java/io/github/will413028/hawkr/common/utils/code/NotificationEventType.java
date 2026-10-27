package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 通知派發領域 MQ 事件類型，由 notification-service publish 到 `hawkr.notification` exchange，僅供 gateway SSE consumer 訂閱用於 web 推播。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式 `notification.inbox.new` / `notification.unread_changed`，寫於 `pkg/mq/exchanges.go`。對齊 OrderEventType / BuyerEventType / SocialEventType 設計。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum NotificationEventType implements CodeMeta {

    NOTIFICATION_INBOX_NEW("NOTIFICATION_INBOX_NEW", "NOTIFICATION_INBOX_NEW", "新 inbox 項目", "Routing key `notification.inbox.new`；notification-service `template_handler::handleTemplate` visible 模式寫完 inbox 後 publish；payload 含完整 InboxItem + unreadCount"),
    NOTIFICATION_UNREAD_CHANGED("NOTIFICATION_UNREAD_CHANGED", "NOTIFICATION_UNREAD_CHANGED", "未讀數變化", "Routing key `notification.unread_changed`；mark-read / mark-all-read / delete 後 publish；payload 僅含 userId + unreadCount");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    NotificationEventType(String code, String value, String label, String description) {
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

    private static final Map<String, NotificationEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(NotificationEventType::code, e -> e));

    private static final Map<String, NotificationEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(NotificationEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static NotificationEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static NotificationEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

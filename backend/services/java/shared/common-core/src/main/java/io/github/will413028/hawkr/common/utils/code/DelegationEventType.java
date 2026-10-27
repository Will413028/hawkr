package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 代行營運領域 MQ 事件類型，由 account-service 的 outbox dispatcher 發布到 `hawkr.delegation` exchange。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式 `delegation.operation.completed`，寫於 `pkg/mq/exchanges.go`；event_type 僅作 metadata。對齊 OrderEventType / SocialEventType 設計。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum DelegationEventType implements CodeMeta {

    DELEGATION_OPERATION_COMPLETED("DELEGATION_OPERATION_COMPLETED", "DELEGATION_OPERATION_COMPLETED", "代行寫入完成", "Routing key `delegation.operation.completed`；稽核列 confirm 為 SUCCEEDED 且 notify_seller 為 true 後由 outbox 發出；notification-service consume 後以站內訊息通知賣家（inbox 加 App 推播，push_mode 為 visible；平台無 Email 通道），以 audit_log_id 保冪等");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    DelegationEventType(String code, String value, String label, String description) {
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

    private static final Map<String, DelegationEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationEventType::code, e -> e));

    private static final Map<String, DelegationEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(DelegationEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static DelegationEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static DelegationEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

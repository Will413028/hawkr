package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Buyer／seller Membership 的 canonical event type。Value 使用 UPPER_SNAKE_CASE；domain routing key 由 `pkg/mq`／服務 topology 管理，不放進 enum。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum MembershipEventType implements CodeMeta {

    MEMBERSHIP_ENTITLEMENTS_CHANGED("MEMBERSHIP_ENTITLEMENTS_CHANGED", "MEMBERSHIP_ENTITLEMENTS_CHANGED", "會員權益變更", "current membership period 或 immediate feature overlay 改變"),
    MEMBERSHIP_ENTITLEMENT_REVISION_PUBLISHED("MEMBERSHIP_ENTITLEMENT_REVISION_PUBLISHED", "MEMBERSHIP_ENTITLEMENT_REVISION_PUBLISHED", "權益版本發布", "plan 的 immutable entitlement revision 發布"),
    MEMBERSHIP_PERIOD_STARTED("MEMBERSHIP_PERIOD_STARTED", "MEMBERSHIP_PERIOD_STARTED", "會員週期開始", "新的 canonical membership period 建立"),
    MEMBERSHIP_CHANGE_REQUESTED("MEMBERSHIP_CHANGE_REQUESTED", "MEMBERSHIP_CHANGE_REQUESTED", "會員變更申請", "NEXT_PERIOD 或待付款 membership change 建立"),
    MEMBERSHIP_CANCELED("MEMBERSHIP_CANCELED", "MEMBERSHIP_CANCELED", "會員取消", "membership 或 provider 取消完成");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    MembershipEventType(String code, String value, String label, String description) {
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

    private static final Map<String, MembershipEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(MembershipEventType::code, e -> e));

    private static final Map<String, MembershipEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(MembershipEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static MembershipEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static MembershipEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

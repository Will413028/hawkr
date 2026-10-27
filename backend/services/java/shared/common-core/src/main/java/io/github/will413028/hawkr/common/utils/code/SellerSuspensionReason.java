package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家停權原因碼。Admin 手動停權（`SuspendSellerForAdmin`）時指定，賣家端可見（用於停權公告頁顯示原因）。**已定裁決三值，不含 FOOD_SAFETY**（食安通報走 32 號治理路徑，非本 admin 手動停權原因）。詳見 [46 §5.1](../詳細設計書/46-賣家能力權限層（Seller%20Capability%20Authorization）詳細設計書.md#51-sellersuspensionreason已定裁決三值不含-food_safety)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum SellerSuspensionReason implements CodeMeta {

    POLICY_VIOLATION("POLICY_VIOLATION", "9601", "違反平台規範", "賣家行為違反平台使用規範"),
    REPEATED_WARNING_IGNORED("REPEATED_WARNING_IGNORED", "9602", "屢勸不改", "已多次警告仍未改善"),
    OTHER("OTHER", "9603", "其他", "其他停權原因，搭配 admin 備註顯示");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    SellerSuspensionReason(String code, String value, String label, String description) {
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

    private static final Map<String, SellerSuspensionReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerSuspensionReason::code, e -> e));

    private static final Map<String, SellerSuspensionReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(SellerSuspensionReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static SellerSuspensionReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static SellerSuspensionReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

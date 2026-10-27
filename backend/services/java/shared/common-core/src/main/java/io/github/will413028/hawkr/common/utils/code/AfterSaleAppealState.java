package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 售後單申訴狀態軸（與主 status 正交）；詳見設計書 §10
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum AfterSaleAppealState implements CodeMeta {

    NONE("NONE", "0", "無申訴", "預設值，案件未申訴"),
    APPEALED("APPEALED", "10", "已申訴", "買家已提交申訴，案件凍結等待 Admin 裁決"),
    INVESTIGATING("INVESTIGATING", "20", "平台調查中", "Admin 已接手但尚未裁決（Phase 2 保留，Phase 1 不設定此值）"),
    RESOLVED("RESOLVED", "30", "已裁決", "Admin 已做出裁決");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    AfterSaleAppealState(String code, String value, String label, String description) {
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

    private static final Map<String, AfterSaleAppealState> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAppealState::code, e -> e));

    private static final Map<String, AfterSaleAppealState> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(AfterSaleAppealState::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static AfterSaleAppealState fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static AfterSaleAppealState fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

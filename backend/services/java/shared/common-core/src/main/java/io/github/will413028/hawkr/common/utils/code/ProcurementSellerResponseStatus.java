package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一個 immutable 回價版本的生命週期（issue #2538 wire 契約 §5.5）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementSellerResponseStatus implements CodeMeta {

    ACTIVE("ACTIVE", "ACTIVE", "有效", "目前最新版、未撤回、未過期"),
    SUPERSEDED("SUPERSEDED", "SUPERSEDED", "已被改版取代", "`version < latest_version`；留在 audit，不可接受"),
    WITHDRAWN("WITHDRAWN", "WITHDRAWN", "已撤回", "`version = withdrawn_version`；賣家主動撤回，payload 不改寫"),
    EXPIRED("EXPIRED", "EXPIRED", "已逾有效期", "`now() > valid_until`");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementSellerResponseStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementSellerResponseStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementSellerResponseStatus::code, e -> e));

    private static final Map<String, ProcurementSellerResponseStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementSellerResponseStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementSellerResponseStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementSellerResponseStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

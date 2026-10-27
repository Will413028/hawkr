package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品上架狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductStatus implements CodeMeta {

    DRAFT("DRAFT", "4001", "草稿", "尚未上架，僅建立者可查看"),
    PENDING_REVIEW("PENDING_REVIEW", "4002", "審核中", "等待後台審核，僅建立者可查看"),
    INACTIVE("INACTIVE", "4003", "未上架", "尚未上架販售，涵蓋從未上架、賣家下架與審核未通過三種來源；本欄只答「能不能賣」，是否曾公開由 ProductPublicationState 表達"),
    ACTIVE("ACTIVE", "4004", "已上架", "已上架販售中，任何人可查看");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ProductStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductStatus::code, e -> e));

    private static final Map<String, ProductStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

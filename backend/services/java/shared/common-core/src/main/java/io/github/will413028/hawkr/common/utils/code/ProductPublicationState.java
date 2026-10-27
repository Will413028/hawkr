package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品公開生命週期衍生值；儲存不可變的首次公開事實，避免與 ProductStatus 維護兩套可變狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductPublicationState implements CodeMeta {

    NEVER_PUBLISHED("NEVER_PUBLISHED", "4011", "從未公開", "草稿、審核中或既有資料無法證明曾公開，匿名使用者不可查看"),
    PUBLIC("PUBLIC", "4012", "公開", "商品內容可公開；是否可購買仍由 ProductStatus 與庫存共同決定"),
    TEMPORARILY_UNAVAILABLE("TEMPORARILY_UNAVAILABLE", "4013", "暫時不可購買", "曾公開但目前下架，保留公開內容與穩定網址"),
    REMOVED("REMOVED", "4014", "已移除", "商品已明確刪除，匿名使用者不可查看");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductPublicationState(String code, String value, String label, String description) {
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

    private static final Map<String, ProductPublicationState> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductPublicationState::code, e -> e));

    private static final Map<String, ProductPublicationState> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductPublicationState::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductPublicationState fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductPublicationState fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

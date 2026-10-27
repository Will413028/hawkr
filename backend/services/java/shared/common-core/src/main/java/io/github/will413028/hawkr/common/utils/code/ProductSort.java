package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品列表排序方式。對應 `/api/v1/public/products` 的 `sort` 參數。Value 採字串（同 ClientType / AppType / OrderCancelByRole 慣例），便於前端 `switch` 與 URL round-trip。空字串視為 LATEST。詳見 [35 商品列表排序與銷量反正規化設計書](../詳細設計書/35-商品列表排序與銷量反正規化%EF%BC%88Product%20Sort%20&%20Sales%20Count%EF%BC%89詳細設計書.md)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductSort implements CodeMeta {

    LATEST("LATEST", "LATEST", "最新上架", "依 `created_at DESC`（不送 sort 也視為此值）"),
    PRICE_ASC("PRICE_ASC", "PRICE_ASC", "價格低到高", "依 `price_min ASC`"),
    PRICE_DESC("PRICE_DESC", "PRICE_DESC", "價格高到低", "依 `price_max DESC`"),
    RATING("RATING", "RATING", "評分最高", "依 `rating DESC NULLS LAST`，無評分商品排後"),
    SALES("SALES", "SALES", "銷量最高", "依 `sales_count DESC`，付款成功時累計");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductSort(String code, String value, String label, String description) {
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

    private static final Map<String, ProductSort> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductSort::code, e -> e));

    private static final Map<String, ProductSort> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductSort::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductSort fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductSort fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 一筆認證申請是從哪條寫入路徑產生的。19 §6.1 定義五條寫入路徑收斂到單一收斂點，本值域是那五條路徑在資料上留下的唯一區分——缺了它，事後查不出「這筆是賣家自己送的、還是 ADM 代行送的、還是 migration 造出來的」，而三者的追責對象完全不同。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductCertificationSource implements CodeMeta {

    SELLER("SELLER", "SELLER", "賣家送出", "賣家自行透過商品建立／編輯或專用申請端點送出"),
    PRODUCT_IMPORT("PRODUCT_IMPORT", "PRODUCT_IMPORT", "商品匯入", "product-import-service 的 CSV 匯入 commit 路徑帶入"),
    DELEGATED_ADMIN("DELEGATED_ADMIN", "DELEGATED_ADMIN", "管理員代行", "ADM 在代行工作階段中代替賣家送出；`submitted_by_account_id` 是代行管理員本人"),
    SEED("SEED", "SEED", "種子資料", "`infra/seed-data/` 建立的測試資料，不代表任何真實賣家宣稱"),
    LEGACY_MIGRATION("LEGACY_MIGRATION", "LEGACY_MIGRATION", "存量遷移", "V18 migration 由既有 `product.certifications` JSONB 回填；`submitted_by_account_id` 必為 NULL（migration 沒有送出者，**不得**拿 `store_id` 頂替）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductCertificationSource(String code, String value, String label, String description) {
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

    private static final Map<String, ProductCertificationSource> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationSource::code, e -> e));

    private static final Map<String, ProductCertificationSource> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductCertificationSource::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductCertificationSource fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductCertificationSource fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

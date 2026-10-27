package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 暫存圖片的用途分類，區分商品主圖與描述內嵌圖。詳見 [40 號設計書 §3.4](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#34-product_import_item_asset)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ImportAssetUsage implements CodeMeta {

    GALLERY("GALLERY", "GALLERY", "主圖", "CSV `product_image_1`~`product_image_5` 對應"),
    DESCRIPTION("DESCRIPTION", "DESCRIPTION", "描述內嵌圖", "description.blocks 內 image block 對應");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ImportAssetUsage(String code, String value, String label, String description) {
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

    private static final Map<String, ImportAssetUsage> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetUsage::code, e -> e));

    private static final Map<String, ImportAssetUsage> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetUsage::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ImportAssetUsage fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ImportAssetUsage fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 暫存圖片資產（Import 端視角）的 6 態狀態機。終態為 LINKED / ORPHAN（ORPHAN 等待 GC delete）。詳見 [40 號設計書 §2.3](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#23-importassetlocal-狀態機import-端-product_import_item_assetstatus)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ImportAssetLocalStatus implements CodeMeta {

    PENDING("PENDING", "9301", "待排程", "剛建立，尚未推 MQ 排下載"),
    DOWNLOADING("DOWNLOADING", "9302", "下載中", "已推 MQ，Media 處理中"),
    UPLOADED("UPLOADED", "9303", "已上傳", "Media 回報下載成功"),
    FAILED("FAILED", "9304", "下載失敗", "可重試"),
    LINKED("LINKED", "9305", "已綁定", "轉正完成，終態"),
    ORPHAN("ORPHAN", "9306", "待清理", "賣家編輯後棄置，等待 GC delete");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ImportAssetLocalStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ImportAssetLocalStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetLocalStatus::code, e -> e));

    private static final Map<String, ImportAssetLocalStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportAssetLocalStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ImportAssetLocalStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ImportAssetLocalStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

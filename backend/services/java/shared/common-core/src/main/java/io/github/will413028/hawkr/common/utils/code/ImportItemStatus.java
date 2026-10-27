package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 暫存商品（CSV row 聚合後的 product_key 群組）的 7 態狀態機。終態為 COMMITTED。row 結構錯誤已在 L1/L2 整檔 reject 不建 item；業務錯誤走 READY + blocking_error_count > 0。詳見 [40 號設計書 §2.2](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#22-importitem-狀態機)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ImportItemStatus implements CodeMeta {

    PARSED("PARSED", "9201", "已解析", "row 解析完成，等待圖片下載排程"),
    ASSET_DOWNLOADING("ASSET_DOWNLOADING", "9202", "下載中", "gallery / description 圖片下載中"),
    READY("READY", "9203", "已就緒", "圖片下載完成；含 blocking_error_count>0 的需修才能 commit"),
    REMOVED("REMOVED", "9205", "已移除", "賣家手動移除，可恢復"),
    COMMITTING("COMMITTING", "9206", "轉正中", "commit worker 處理中"),
    COMMITTED("COMMITTED", "9207", "已轉正", "已建立正式商品，終態"),
    COMMIT_FAILED("COMMIT_FAILED", "9208", "轉正失敗", "可重試或編輯後重試");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ImportItemStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ImportItemStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportItemStatus::code, e -> e));

    private static final Map<String, ImportItemStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportItemStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ImportItemStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ImportItemStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

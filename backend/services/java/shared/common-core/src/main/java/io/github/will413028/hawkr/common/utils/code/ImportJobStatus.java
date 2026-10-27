package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品匯入任務（CSV 上傳→L1+L2 同步驗證→PARSING→READY→commit→COMMITTED）的 9 態狀態機。終態為 PARSE_FAILED / COMMITTED / EXPIRED / CANCELLED。L2 失敗則進 CSV_INVALID 等賣家重傳。詳見 [40 號設計書 §2.1](../詳細設計書/40-商品匯入精靈服務（Product%20Import%20Service）詳細設計書.md#21-importjob-狀態機)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ImportJobStatus implements CodeMeta {

    CREATED("CREATED", "9101", "已建立", "L1+L2 同步驗證執行中的瞬間態"),
    PARSING("PARSING", "9102", "解析中", "csv.parse 背景 worker 處理中"),
    PARSE_FAILED("PARSE_FAILED", "9103", "解析失敗", "worker 級系統失敗，終態（罕見）"),
    READY("READY", "9104", "已就緒", "解析完成，可預覽/編輯/commit"),
    COMMITTING("COMMITTING", "9105", "轉正中", "commit 進行中，鎖定編輯"),
    COMMITTED("COMMITTED", "9106", "已完成轉正", "全部 eligible item 已 COMMITTED，終態"),
    EXPIRED("EXPIRED", "9107", "已過期", "過期清理，終態"),
    CANCELLED("CANCELLED", "9108", "已中止", "賣家中止，終態"),
    CSV_INVALID("CSV_INVALID", "9111", "CSV 內容有誤", "L2 同步驗證失敗，等賣家重傳 CSV；獨立 TTL 3 天");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ImportJobStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ImportJobStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportJobStatus::code, e -> e));

    private static final Map<String, ImportJobStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ImportJobStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ImportJobStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ImportJobStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

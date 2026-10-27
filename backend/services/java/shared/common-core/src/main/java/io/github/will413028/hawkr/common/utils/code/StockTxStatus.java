package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 庫存交易執行狀態
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum StockTxStatus implements CodeMeta {

    INIT("INIT", "8911", "初始化", "交易已建立"),
    SUCCESS("SUCCESS", "8912", "成功", "交易執行成功"),
    FAILED("FAILED", "8913", "失敗", "交易執行失敗"),
    MANUAL_INTERVENTION("MANUAL_INTERVENTION", "8914", "需人工介入", "異常需人工處理"),
    COMPENSATED("COMPENSATED", "8915", "已補償", "DEDUCT 已回滾並補償完成（終態，不再被補償 Worker 重撈）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    StockTxStatus(String code, String value, String label, String description) {
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

    private static final Map<String, StockTxStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(StockTxStatus::code, e -> e));

    private static final Map<String, StockTxStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(StockTxStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static StockTxStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static StockTxStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

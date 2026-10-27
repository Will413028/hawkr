package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Product Service 庫存操作結果狀態，供 Order Service / Worker 依值分支判斷
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum StockOperationStatus implements CodeMeta {

    SUCCESS("SUCCESS", "SUCCESS", "成功", "庫存操作成功"),
    INSUFFICIENT_STOCK("INSUFFICIENT_STOCK", "INSUFFICIENT_STOCK", "庫存不足", "SKU 庫存不足，附帶不足明細"),
    ALREADY_APPLIED("ALREADY_APPLIED", "ALREADY_APPLIED", "已重複套用", "相同 stock_tx_id 已處理，視為冪等成功"),
    NOOP_NOT_FOUND("NOOP_NOT_FOUND", "NOOP_NOT_FOUND", "無需處理", "找不到對應 stock_tx_id，視為補償成功"),
    SYSTEM_ERROR("SYSTEM_ERROR", "SYSTEM_ERROR", "系統錯誤", "非業務錯誤，需重試或人工介入");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    StockOperationStatus(String code, String value, String label, String description) {
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

    private static final Map<String, StockOperationStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(StockOperationStatus::code, e -> e));

    private static final Map<String, StockOperationStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(StockOperationStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static StockOperationStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static StockOperationStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

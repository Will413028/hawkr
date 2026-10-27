package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 賣家對單一配送異常的處置動作。**僅紀錄使用者操作**，不參與資料一致性 — 異常本身狀態仍以 shipments 表欄位為唯一真實來源（見[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md) §六）。Dashboard 查詢時 LEFT JOIN ack 表，已 ack 的隱藏。shipment 狀態恢復後 dashboard 自動不顯示（無需動 ack 表）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShipmentAnomalyAckAction implements CodeMeta {

    ACKNOWLEDGED("ACKNOWLEDGED", "ACKNOWLEDGED", "已處理", "賣家確認已採取處理動作（聯絡客戶 / 重列印 / 重叫件等）"),
    IGNORED("IGNORED", "IGNORED", "已忽略", "賣家確認不需 dashboard 提示，但異常仍存在（如已透過其他管道處理）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShipmentAnomalyAckAction(String code, String value, String label, String description) {
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

    private static final Map<String, ShipmentAnomalyAckAction> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShipmentAnomalyAckAction::code, e -> e));

    private static final Map<String, ShipmentAnomalyAckAction> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShipmentAnomalyAckAction::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShipmentAnomalyAckAction fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShipmentAnomalyAckAction fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 收貨確認軸（E 軸）的狀態。**獨立於 `OrderStatus`**：訂單軸會繼續往 50／60／70 走，而收貨確認是一條平行的軸，`confirmed_at`／`confirm_type` 降為附屬事實欄。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ReceiptStatus implements CodeMeta {

    PENDING("PENDING", "PENDING", "待確認收貨", "已送達，等待買家確認。**入邊是送達事件**（`delivered_at` 寫入的同一句）"),
    BUYER_CONFIRMED("BUYER_CONFIRMED", "BUYER_CONFIRMED", "買家已確認", "買家主動確認收貨（對應既有 `confirm_type = MANUAL`）。撥款資格條件之一"),
    AUTO_CONFIRMED("AUTO_CONFIRMED", "AUTO_CONFIRMED", "逾期自動確認", "逾期未確認，系統自動完成（對應既有 `confirm_type = AUTO`）。⚠️ **不**滿足撥款資格的收貨確認條件——那一條只認 `BUYER_CONFIRMED` 與 `ADMIN_CONFIRMED`"),
    REVIEW_PENDING("REVIEW_PENDING", "REVIEW_PENDING", "人工裁決中", "送人工裁決，等待 ADM 判定"),
    ADMIN_CONFIRMED("ADMIN_CONFIRMED", "ADMIN_CONFIRMED", "平台判定已收貨", "ADM 裁定收貨成立。撥款資格條件之一"),
    ADMIN_REJECTED("ADMIN_REJECTED", "ADMIN_REJECTED", "平台判定未收貨", "ADM 裁定收貨不成立"),
    DISPUTED("DISPUTED", "DISPUTED", "爭議中", "有處理中的爭議。🔴 撥款一律 fail-closed 擋下——不知道有沒有爭議時也當作有");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ReceiptStatus(String code, String value, String label, String description) {
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

    private static final Map<String, ReceiptStatus> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReceiptStatus::code, e -> e));

    private static final Map<String, ReceiptStatus> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ReceiptStatus::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ReceiptStatus fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ReceiptStatus fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

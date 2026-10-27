package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 訂單取消者角色。前端依此 role + cancel_reason 渲染差異化文案（見 [03 §5.3.3](../詳細設計書/03-訂單微服務（Order%20Service）詳細設計書.md#533-前端文案對照buyer-視角)）。Value 直接使用字串便於前端 switch case，與 AfterSaleAuditActor 的 numeric value 慣例不同（後者前端不可見）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum OrderCancelByRole implements CodeMeta {

    SYSTEM("SYSTEM", "SYSTEM", "系統", "系統排程自動取消（payment timeout / shipment timeout）"),
    SELLER("SELLER", "SELLER", "賣家", "賣家透過售後申請取消（aftersales SellerCancel）"),
    BUYER("BUYER", "BUYER", "買家", "買家自願取消未付款訂單");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    OrderCancelByRole(String code, String value, String label, String description) {
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

    private static final Map<String, OrderCancelByRole> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderCancelByRole::code, e -> e));

    private static final Map<String, OrderCancelByRole> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderCancelByRole::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static OrderCancelByRole fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static OrderCancelByRole fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

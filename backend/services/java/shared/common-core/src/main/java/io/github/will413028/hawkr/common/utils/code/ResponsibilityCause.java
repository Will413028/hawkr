package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 物流額外成本的**責任成因**（承 `ADR-004`）。🔴 **依成因判定，不依費用名稱**——同一個「取消費」在賣家改地址與外送員失聯兩種情況下歸屬完全不同。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ResponsibilityCause implements CodeMeta {

    SELLER_CAUSED("SELLER_CAUSED", "SELLER_CAUSED", "賣家原因", "由賣家負擔（備貨逾時、地址填錯、商品不符等）"),
    BUYER_CAUSED("BUYER_CAUSED", "BUYER_CAUSED", "買家原因", "🔴 **仍歸賣家的物流成本**；平台**不得**向買家二次收款（買家已完成一次付款，ADR-004）"),
    PLATFORM_CAUSED("PLATFORM_CAUSED", "PLATFORM_CAUSED", "平台或系統原因", "🔴 平台負擔，**不得轉嫁賣家**。這一格是唯一會 post `COLLECTION_FEE_ACCRUAL` 的情況。⚠️ **系統原因也歸這一格**——`SYSTEM_CAUSED` 不存在，不得新增"),
    PROVIDER_CAUSED("PROVIDER_CAUSED", "PROVIDER_CAUSED", "物流商原因", "🔴 **不得自動向賣家收**，先進爭議／對帳。⚠️ **不再是「判定原料不足」的 fail-closed 落點**（2026-09-17 廢除，見上方說明）——證據不足時案件留在 `IN_PROGRESS`，不落任何成因");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ResponsibilityCause(String code, String value, String label, String description) {
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

    private static final Map<String, ResponsibilityCause> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ResponsibilityCause::code, e -> e));

    private static final Map<String, ResponsibilityCause> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ResponsibilityCause::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ResponsibilityCause fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ResponsibilityCause fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

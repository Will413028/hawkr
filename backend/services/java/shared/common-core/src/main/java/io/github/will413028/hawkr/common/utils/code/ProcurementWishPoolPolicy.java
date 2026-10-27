package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Wish Pool 的受眾政策身分。**v1 是常數不是資料表**：範圍就是「全站所有未停權的賣家」，沒有 category／region／tier 維度。刻意不預留 scope 欄位——防止 `audienceHash` preimage 破壞性變更的機制是 policy 版本本身：v2 引入篩選維度時 preimage 定義隨版本走，舊 publication 依它自己的版本解讀。預先放空欄位等於預先發明未經業務設計驗證的欄位（doc 57 §4.5 禁止）。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementWishPoolPolicy implements CodeMeta {

    ALL_ACTIVE_SELLERS_V1("WISH_POOL_ALL_ACTIVE_SELLERS_V1", "WISH_POOL_ALL_ACTIVE_SELLERS_V1", "全站未停權賣家 v1", "Wish Pool 受眾政策 v1：全站所有未停權的賣家，無分類、地區或分級限制");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementWishPoolPolicy(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementWishPoolPolicy> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementWishPoolPolicy::code, e -> e));

    private static final Map<String, ProcurementWishPoolPolicy> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementWishPoolPolicy::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementWishPoolPolicy fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementWishPoolPolicy fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

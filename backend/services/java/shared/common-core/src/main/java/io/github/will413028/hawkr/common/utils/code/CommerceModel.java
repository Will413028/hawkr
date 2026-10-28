package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商流模式——**這筆錢誰收、誰開票**，決定整條帳務軸。訂單快照六欄之一：付款後鎖定，改政策不得回溯既有訂單。取**短名**（不把 provider 綁進商流維度），與 `provider` 維度正交。🔴 **本值域是三條商流帳務軸互斥的分界線**，65 定版、66／68／69 只引用。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CommerceModel implements CodeMeta {

    SELLER_DIRECT("SELLER_DIRECT", "SELLER_DIRECT", "賣家直營", "賣家為收款主體與開票人。**既有全部訂單皆為本值**（merId = 'HAWK' ‖ store_id）。撥款資格述詞對本值回 NOT_APPLICABLE——不是「不合格」，是不在撥款軸上"),
    PLATFORM_CONSIGNMENT("PLATFORM_CONSIGNMENT", "PLATFORM_CONSIGNMENT", "平台受託代銷", "平台以代理商戶收款、對買家開票，再結算撥付賣家。**必須帶 `consignment_agreement_version`**（`chk_orders_consignment_agreement`），且 `collection_model` 必須是 PLATFORM_AGENT_MERCHANT");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CommerceModel(String code, String value, String label, String description) {
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

    private static final Map<String, CommerceModel> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CommerceModel::code, e -> e));

    private static final Map<String, CommerceModel> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CommerceModel::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CommerceModel fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CommerceModel fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

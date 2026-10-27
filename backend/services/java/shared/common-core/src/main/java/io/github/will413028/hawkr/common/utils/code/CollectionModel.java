package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 收款模式——**款項實際經過誰的手**。訂單快照六欄之一。🔴 與 `commerce_model` 由 `chk_orders_policy_matrix` 綁成雙條件式（兩者必須同時是平台側或同時是賣家側），與 `invoice_model` 由 `chk_orders_invoice_collection` 綁死（收款主體與開票 issuer 必須一致）。**這是通用維度，不得把 provider 名稱寫進值域**——下一個 provider 代收會需要第四個值，而 provider 已由 `ShippingMethod` 承載。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum CollectionModel implements CodeMeta {

    SELLER_MERCHANT("SELLER_MERCHANT", "SELLER_MERCHANT", "賣家子商店收款", "款項直接進賣家的 PayUni 子商店（merId = 'HAWK' ‖ store_id）。**現況全部訂單皆為本值**"),
    PLATFORM_AGENT_MERCHANT("PLATFORM_AGENT_MERCHANT", "PLATFORM_AGENT_MERCHANT", "平台代理商戶收款", "款項進平台代理商戶（`PAYUNI_AGENT_MER_ID`）。🔴 `resolvePaymentMerId` 必須由本值驅動；env 未設定時**結帳當下 fail-closed**（9503 群），不得建單後才發現"),
    PROVIDER_COLLECTED("PROVIDER_COLLECTED", "PROVIDER_COLLECTED", "物流商代收", "由物流商在交付時代收貨款（如超商取貨付款）。⚠️ 值域內但**尚無寫入路徑**——書面契約到位前由 flag 擋住，這是刻意的"),
    EXTERNAL_PREPAID("EXTERNAL_PREPAID", "EXTERNAL_PREPAID", "外部預付", "款項在平台之外已完成收取。⚠️ 同上，值域內但尚無寫入路徑");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    CollectionModel(String code, String value, String label, String description) {
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

    private static final Map<String, CollectionModel> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CollectionModel::code, e -> e));

    private static final Map<String, CollectionModel> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(CollectionModel::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static CollectionModel fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static CollectionModel fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

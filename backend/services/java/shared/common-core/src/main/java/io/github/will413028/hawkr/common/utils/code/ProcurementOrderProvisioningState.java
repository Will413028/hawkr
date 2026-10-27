package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 買家接受報價之後，那張 order shell 建到哪一步（#2538 決策 D3、doc 64 §6.2 決策 B）。acceptance 寫在 Go `ai-service` 的 DB、order shell 建在 Java `order-service` 的 DB，兩者之間**必然有一個可觀察的窗口**——這個欄位就是把那個窗口講出來，而不是假裝它不存在。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementOrderProvisioningState implements CodeMeta {

    CREATING_ORDER("CREATING_ORDER", "CREATING_ORDER", "訂單建立中", "已接受、價格已凍結、**尚未付款**；`orderNo` 與 `paymentDeadline` 為空字串"),
    ORDER_CREATED("ORDER_CREATED", "ORDER_CREATED", "訂單已建立", "order shell 已建；`orderNo` 與 `paymentDeadline` 此時才有值"),
    ORDER_FAILED("ORDER_FAILED", "ORDER_FAILED", "建單失敗", "需要前滾或補償；acceptance 本身仍成立");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementOrderProvisioningState(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementOrderProvisioningState> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementOrderProvisioningState::code, e -> e));

    private static final Map<String, ProcurementOrderProvisioningState> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementOrderProvisioningState::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementOrderProvisioningState fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementOrderProvisioningState fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

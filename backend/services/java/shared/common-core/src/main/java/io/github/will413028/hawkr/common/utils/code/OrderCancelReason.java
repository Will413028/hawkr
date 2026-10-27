package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 訂單取消原因。涵蓋系統 / 賣家 / 買家三種觸發來源，編碼前綴區分：91xx 系統觸發、92xx 賣家觸發、93xx 預留買家觸發。買家自願取消未付款訂單時不寫此欄位（NULL），僅以 OrderCancelByRole=BUYER 區分。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum OrderCancelReason implements CodeMeta {

    PAYMENT_TIMEOUT("PAYMENT_TIMEOUT", "9101", "付款逾時自動取消", "系統排程：訂單超過付款期限未付款"),
    SHIPMENT_TIMEOUT("SHIPMENT_TIMEOUT", "9102", "出貨逾時自動取消", "系統排程：賣家未在 §5.9 處理期限內安排出貨"),
    SHIPMENT_CREATE_TIMEOUT("SHIPMENT_CREATE_TIMEOUT", "9103", "建單逾時自動取消", "系統排程：付款後超過寬限期仍無法建立物流單，重試上限後自動取消並退款（見 03 §8.11）"),
    SELLER_SUSPENDED("SELLER_SUSPENDED", "9104", "賣家停權", "賣家被停權時系統批次取消其未付款訂單（連動 saga，見 46 §5）"),
    FORCE_MAJEURE("FORCE_MAJEURE", "9201", "天災因素無法配送", "天災或不可抗力因素"),
    OUT_OF_STOCK("OUT_OF_STOCK", "9202", "商品缺貨", "庫存不足無法出貨"),
    QUALITY_ISSUE("QUALITY_ISSUE", "9203", "商品品質問題", "商品有品質問題不宜出貨"),
    SUGGEST_SPLIT("SUGGEST_SPLIT", "9204", "建議買家拆單重下", "建議買家拆成多張較小訂單重新下單（生鮮跨區常見場景）"),
    OTHER("OTHER", "9299", "其他原因", "賣家自填原因，搭配 cancel_remark 顯示");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    OrderCancelReason(String code, String value, String label, String description) {
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

    private static final Map<String, OrderCancelReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderCancelReason::code, e -> e));

    private static final Map<String, OrderCancelReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(OrderCancelReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static OrderCancelReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static OrderCancelReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

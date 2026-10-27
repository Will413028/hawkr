package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 訂單層權威退款事件的來源分類，供 invoice-service 判定應走作廢還是折讓。三個值對應 order-service 的三個發布點，全部在既有交易內落 order_outbox。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum RefundSource implements CodeMeta {

    AFTERSALE_CASE("AFTERSALE_CASE", "AFTERSALE_CASE", "售後案件退款", "AfterSalesRefundEventHandler.handleRefundCompleted（僅 duplicate == false 分支）"),
    SELLER_CANCEL("SELLER_CANCEL", "SELLER_CANCEL", "賣家取消訂單", "AfterSalesEventConsumer.onCancelCompleted 新抽的 tx handler"),
    AUTO_REFUND("AUTO_REFUND", "AUTO_REFUND", "系統自動退款", "AutoRefundTxHandler.completeAutoRefund");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    RefundSource(String code, String value, String label, String description) {
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

    private static final Map<String, RefundSource> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundSource::code, e -> e));

    private static final Map<String, RefundSource> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(RefundSource::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static RefundSource fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static RefundSource fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

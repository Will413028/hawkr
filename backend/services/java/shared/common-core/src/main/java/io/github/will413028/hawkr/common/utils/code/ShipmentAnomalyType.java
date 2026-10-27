package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 配送異常類型。由 shipping-service 之既有 scheduler 與 webhook consumer 偵測 shipments / seller_shipment_settings 既有狀態欄位（status / tcat_call_status / label_print_status / payuni_submit_status）異常時發布 `shipment.anomaly.detected` event 用。**異常本身的狀態仍以 shipments 既有欄位為唯一真實來源**，本 enum 僅標示「賣家收到通知時看到的異常分類」。詳見 [06 §5.21](../詳細設計書/06-物流微服務（Shipping%20Service）詳細設計書.md#521-配送異常賣家通知)。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShipmentAnomalyType implements CodeMeta {

    TCAT_CALL_FAILED("TCAT_CALL_FAILED", "TCAT_CALL_FAILED", "黑貓叫件未完成", "賣家呼叫黑貓後 24h 內 tcat_call_status 仍為 REQUESTED（4202）"),
    CVS_LABEL_UNPRINTED("CVS_LABEL_UNPRINTED", "CVS_LABEL_UNPRINTED", "超商標籤未列印", "賣家列印超商標籤後 48h 仍未交件（label_print_status 卡 4102）"),
    PROVIDER_STATUS_ABNORMAL("PROVIDER_STATUS_ABNORMAL", "PROVIDER_STATUS_ABNORMAL", "物流商回報異常", "shipments.status=33（異常）。41 CANCELLED 為賣家自取消（不算物流商異常），44 走 PACKAGE_LOST"),
    PACKAGE_LOST("PACKAGE_LOST", "PACKAGE_LOST", "包裹遺失（卡待取貨）", "shipments.status 卡 32 待取貨超過 7 天"),
    DELIVERY_OVERDUE("DELIVERY_OVERDUE", "DELIVERY_OVERDUE", "配送逾期", "超過 expected_delivery_date 且物流商超過 3 天無狀態更新（軌跡近期仍活躍視為仍在正常移動，不打異常）"),
    PAYUNI_LGS_NO_CODE("PAYUNI_LGS_NO_CODE", "PAYUNI_LGS_NO_CODE", "PayUni 回 SUCCESS 但無三段碼", "PayUni 列印 Webhook SUCCESS 但 partnerId / odno / validationNo 為空（B-143 重現）"),
    RETURN_STUCK("RETURN_STUCK", "RETURN_STUCK", "退貨流程卡住", "aftersale_case 退貨流程超過 14 天無狀態進展"),
    UBER_COURIER_UNASSIGNED("UBER_COURIER_UNASSIGNED", "UBER_COURIER_UNASSIGNED", "Uber 外送員未指派", "建單成功後停在 UNPROCESSED(91) 超過 30 分鐘仍未指派外送員（66 §5.1.11）"),
    UBER_PICKUP_STUCK("UBER_PICKUP_STUCK", "UBER_PICKUP_STUCK", "Uber 取件停滯", "停在 COURIER_ASSIGNED(933) 超過 45 分鐘仍未取件（66 §5.1.11）"),
    UBER_CREATE_UNKNOWN_UNRESOLVED("UBER_CREATE_UNKNOWN_UNRESOLVED", "UBER_CREATE_UNKNOWN_UNRESOLVED", "Uber 建單結果未知且未收斂", "CREATE_UNKNOWN(991) 超過回查窗仍未知（66 §5.1.11）"),
    UBER_POD_MISSING("UBER_POD_MISSING", "UBER_POD_MISSING", "Uber 送達憑證缺漏", "delivered 後 7 天內未取得任何 POD（66 §5.1.11／INV-U5）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShipmentAnomalyType(String code, String value, String label, String description) {
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

    private static final Map<String, ShipmentAnomalyType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShipmentAnomalyType::code, e -> e));

    private static final Map<String, ShipmentAnomalyType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShipmentAnomalyType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShipmentAnomalyType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShipmentAnomalyType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

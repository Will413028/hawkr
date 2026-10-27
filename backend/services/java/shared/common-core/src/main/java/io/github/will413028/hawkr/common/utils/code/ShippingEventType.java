package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 物流事件 MQ 類型，由 `ShippingEventPublisher` 與 `ShipmentAnomalyEventPublisher` 發布到 `hawkr.shipping` exchange。🔴 **本 enum 的 `Value` 欄是點分式 routing key**（同 `OrderEventType` 的 Java 領域慣例），`Code` 才是 UPPER_SNAKE。**物流原本是唯一沒有專屬 EventType enum 的領域**——9 條 routing key 散落 `MqConstants` 的字面字串常數，違反禁硬編碼。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ShippingEventType implements CodeMeta {

    SHIPMENT_CREATED("SHIPMENT_CREATED", "shipment.created", "物流單已建立", "建單成功，order 回填 shipment_no 與 processing_deadline"),
    SHIPMENT_LABEL_UPDATED("SHIPMENT_LABEL_UPDATED", "shipment.label.updated", "標籤已更新", "託運單／超商三段碼取得，order 與 aftersales 各有 consumer"),
    SHIPMENT_DELIVERED("SHIPMENT_DELIVERED", "shipment.delivered", "已送達", "推進 order 至 DELIVERED(40) 並落 order.delivered outbox"),
    SHIPMENT_PHOTO_UPDATED("SHIPMENT_PHOTO_UPDATED", "shipment.photo.updated", "出貨照片已更新", "只存 objectKey，不存 URL"),
    SHIPMENT_STATUS_CHANGED("SHIPMENT_STATUS_CHANGED", "shipment.status.changed", "貨態已變更", "⚠️ payload 的 `status` 是 `ShippingStatus` value，**跨服務契約**：改值域即破壞性變更"),
    RETURN_SHIPMENT_STATUS_CHANGED("RETURN_SHIPMENT_STATUS_CHANGED", "shipment.return.status_changed", "逆向貨態已變更", "逆向 leg 專屬；aftersales 有第二張映射表（同一個值在正逆向語意不同）"),
    SHIPMENT_PROCESSING_TIMEOUT("SHIPMENT_PROCESSING_TIMEOUT", "shipment.processing.timeout", "賣家處理逾時", "逾期未處理，aftersales 接手"),
    SHIPMENT_PROCESSING_CLEARED("SHIPMENT_PROCESSING_CLEARED", "shipment.processing.cleared", "處理期限已解除", "order 清 processing_deadline"),
    SHIPMENT_ANOMALY_DETECTED("SHIPMENT_ANOMALY_DETECTED", "shipment.anomaly.detected", "物流異常已偵測", "由 `ShipmentAnomalyEventPublisher` 發出；⚠️ topology 無專屬 binding，靠 `notification.shipping` 的 `#` 收");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ShippingEventType(String code, String value, String label, String description) {
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

    private static final Map<String, ShippingEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingEventType::code, e -> e));

    private static final Map<String, ShippingEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ShippingEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ShippingEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ShippingEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}

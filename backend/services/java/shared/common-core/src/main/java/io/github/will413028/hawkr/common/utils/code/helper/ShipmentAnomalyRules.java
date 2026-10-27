package io.github.will413028.hawkr.common.utils.code.helper;

import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalySeverity;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;

import java.util.Set;

/**
 * §5.21 配送異常賣家通知 — Severity 與異常推導規則 helper（跨服務共用）。
 * <p>
 * CodeMeta enum 為跨語言公共型別，禁止包含業務輔助邏輯（docs/開發規範/Java 微服務開發規範.md §6）。
 * 本 helper 集中放在 common-core，因為 shipping-service（發送 5 種 anomaly）
 * 與 aftersales-service（B-143 即時偵測 PAYUNI_LGS_NO_CODE、RETURN_STUCK 排程）
 * 都需要相同 severity 對應，避免複製造成不一致。
 * 此 helper 為純函式，不引入任何外部依賴。
 *
 * 設計書：06 §5.21.3
 */
public final class ShipmentAnomalyRules {

    private ShipmentAnomalyRules() {
    }

    /**
     * 異常類型對應 severity。
     * HIGH 觸發即時 FCM；MEDIUM 早晚摘要；LOW 僅 inbox 不推 push（推送節奏由 notification-service 端策略決定）。
     *
     * <p><b>⚠ Dashboard SQL 同步點</b>：本方法的對應規則同時硬編於
     * {@code ShipmentAnomalyQueryMapper} 的 {@code SEVERITY_HIGH_TYPES} /
     * {@code SEVERITY_MEDIUM_TYPES} / {@code SEVERITY_LOW_TYPES} 三個具名常數
     * （由 {@code CASE_SEVERITY} 組成 SQL 片段），供 stats 端 GROUP BY severity 與
     * list 端 severity filter 使用。修改此方法時必須同步更新那三個常數，
     * 否則 Dashboard 顯示的 severity 與 FCM 推送 / inbox 寫入的 severity 會脫節。
     *
     * <p>✅ <b>2026-09-12 已收斂</b>（架構裁決 DEC-20260912-184025 第 17 條）：
     * Uber 四值已逐值列入那三個常數，{@code CASE_SEVERITY} 改以 {@code ELSE 'HIGH'} 收尾
     * （未知值<b>吵</b>而不是靜默降級——分不出來的時候，吵比靜默安全）。
     * 🔴 <b>兩份 SSOT 的同步現在由測試守著</b>：
     * {@code ShipmentAnomalySeverityMappingTest} 對 {@code ShipmentAnomalyType} 逐值斷言
     * 「恰好落在一組」＋「ELSE 不得是 LOW」。新增碼值卻忘了同步常數 ⇒ build 紅。
     * ⚠️ 它是被<b>變異證明過需要</b>才寫的：在它存在之前，同時注入
     * 「拿掉一個 HIGH 值」＋「ELSE 改回 LOW」兩個變異，shipping-service 的 375 支測試
     * <b>全綠</b>。
     *
     * <p>⚠️ 兩份 SSOT 無法整併成一份的原因：{@code @Select} 需要編譯期常數，
     * 而本 helper 是 runtime 方法。那個限制論證的是「不做 {@code @SelectProvider} 重寫」，
     * <b>不是「不設 guard」</b>——所以 guard 有設。
     *
     * <p><b>Uber 四值的 severity 取值理由</b>（即時配送的整段生命週期只有約 1 小時，
     * MEDIUM 的「早晚摘要」對它等於沒有通知，所以這裡只用 HIGH / LOW 兩端）：
     * <ul>
     *   <li>{@code UBER_COURIER_UNASSIGNED}（30 分未指派）→ HIGH：買家正在等，賣家可取消或改配送方式</li>
     *   <li>{@code UBER_PICKUP_STUCK}（45 分未取件）→ HIGH：貨已打包擺著，賣家要決定是否改派</li>
     *   <li>{@code UBER_CREATE_UNKNOWN_UNRESOLVED} → HIGH：這張單究竟有沒有建成不明，
     *       賣家在釐清前不應把貨交給任何人</li>
     *   <li>{@code UBER_POD_MISSING}（delivered 後 7 天）→ LOW：純證據面，賣家當下無事可做</li>
     * </ul>
     * ⚠️ 66 未指定這四個值的 severity；上表是本波取的預設值，屬功能擁有者可調整的產品判斷
     * （零 emitter 期間改動無任何線上影響）。
     */
    public static ShipmentAnomalySeverity severityOf(ShipmentAnomalyType type) {
        return switch (type) {
            case PAYUNI_LGS_NO_CODE, PROVIDER_STATUS_ABNORMAL, PACKAGE_LOST -> ShipmentAnomalySeverity.HIGH;
            case TCAT_CALL_FAILED, CVS_LABEL_UNPRINTED, DELIVERY_OVERDUE -> ShipmentAnomalySeverity.MEDIUM;
            case UBER_COURIER_UNASSIGNED, UBER_PICKUP_STUCK, UBER_CREATE_UNKNOWN_UNRESOLVED
                    -> ShipmentAnomalySeverity.HIGH;
            case RETURN_STUCK, UBER_POD_MISSING -> ShipmentAnomalySeverity.LOW;
        };
    }

    /**
     * Dashboard 查詢條件：哪些 shipments.status 直接屬於「目前有異常」。
     * 用於 ListSellerShipmentAnomalies 推導 PROVIDER_STATUS_ABNORMAL / PACKAGE_LOST 異常 row。
     *
     * <p><b>不含 CANCELLED (41)</b>：41 為「訂單已被商店取消」(賣家主動)，
     * 非物流商異常；由訂單列表呈現，不重複 list 至 anomaly dashboard
     * （stage E2E 抓到的 false positive — 37 筆 anomaly 多數為 status=CANCELLED）。
     */
    public static Set<String> abnormalShipmentStatusValues() {
        return Set.of(ShippingStatus.ABNORMAL.value(), ShippingStatus.PACKAGE_LOST.value());
    }

    /**
     * 由 shipments.status 推導對應的 PROVIDER_STATUS_ABNORMAL 子分類。
     * PACKAGE_LOST → PACKAGE_LOST；ABNORMAL → PROVIDER_STATUS_ABNORMAL。
     */
    public static ShipmentAnomalyType providerStatusAnomalyOf(String shipmentStatus) {
        return ShippingStatus.PACKAGE_LOST.value().equals(shipmentStatus)
                ? ShipmentAnomalyType.PACKAGE_LOST
                : ShipmentAnomalyType.PROVIDER_STATUS_ABNORMAL;
    }
}

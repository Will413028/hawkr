package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

import java.time.LocalDateTime;

/**
 * 主動回查能力。狀態同步（使用者觸發）與對帳排程（背景）共用同一支 provider 呼叫，
 * 不另立 capability——那是同一個 provider 能力的兩個觸發者。
 */
public interface TrackCapability {

    /**
     * 向 provider 查詢當前狀態。
     *
     * <p>🔴 回傳<b>已正規化</b>的結果 ＋ provider 原生值。
     * {@code LgsType}／{@code ShipType} 等 provider 專屬參數的推導必須留在 adapter 內部
     * ——它們是「只有這個 provider 才需要的參數」，由平台維度欄位即時推導正是最典型的洩漏。
     */
    TrackResult query(ShipmentEntity shipment);

    /**
     * @param normalizedStatus  正規化後的 {@code ShippingStatus.value()}；null 表示 provider 沒有可用狀態
     * @param providerRawStatus provider 原生值，原樣保存到 {@code shipments.provider_status}
     * @param providerMessage   provider 說明
     * @param providerStatusAt  provider 回報的狀態時間
     */
    record TrackResult(String normalizedStatus,
                       String providerRawStatus,
                       String providerMessage,
                       LocalDateTime providerStatusAt) {}
}

package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

/**
 * 取消能力。
 *
 * <p>🔴 <b>本 capability 為 {@code Optional.empty()} 不等於「不能取消」。</b>
 * 四個 provider 中目前只有 Uber 有真正的 cancel API；其餘三條線的取消是
 * {@code requiresProviderCall = false} 的 domain action——寫 {@code 992 CANCEL_PENDING}
 * 或本地終態，由回調／人工補上 provider 側確認。
 * 把 empty 直接接到「按鈕不可用」會讓那些取消按鈕整條消失。
 */
public interface CancelCapability {

    CancelOutcome cancel(ShipmentEntity shipment, String reason);

    /**
     * @param accepted        provider 是否受理
     * @param terminal        provider 是否已給出終局答案（true ＝ 可直接落終態；false ＝ 落 992 等回調）
     * @param providerMessage 原始訊息，僅供稽核
     */
    record CancelOutcome(boolean accepted, boolean terminal, String providerMessage) {}
}

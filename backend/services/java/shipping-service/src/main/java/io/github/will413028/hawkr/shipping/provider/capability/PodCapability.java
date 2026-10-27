package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

import java.time.LocalDateTime;
import java.util.Optional;

/**
 * 送達證明（Proof of Delivery）能力。
 *
 * <p>⚠️ 有窗口限制的 provider（例如取件後 N 天內才查得到）必須在 {@link #retentionWindowHours()}
 * 揭露，呼叫端據此決定何時抓回自存——過了窗口才去抓，拿到的會是「查無此件」而不是錯誤。
 */
public interface PodCapability {

    /** 向 provider 取得送達證明。窗口已過或尚未產生時回 empty。 */
    Optional<PodResult> fetchPod(ShipmentEntity shipment);

    /** provider 保留 POD 的時數；0 表示無限制。 */
    int retentionWindowHours();

    /**
     * @param imageObjectKey 已轉存到我方儲存的 objectKey。🔴 只存 objectKey，禁止存完整 URL 或 presigned URL
     * @param signedBy       簽收人
     */
    record PodResult(String imageObjectKey, String signedBy, LocalDateTime deliveredAt) {}
}

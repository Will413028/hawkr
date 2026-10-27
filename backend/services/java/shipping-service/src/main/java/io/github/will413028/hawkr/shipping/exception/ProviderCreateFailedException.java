package io.github.will413028.hawkr.shipping.exception;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * PayUni 建單呼叫 retry 後仍失敗（收斂規範 §8.1）。
 *
 * <p>Strategy 層在 retry 耗盡後拋出此例外，ShippingService 層捕獲後負責將 provider_status /
 * provider_message 落地到 shipments 表（status=CREATE_FAILED），然後再把 retriable 原例外
 * 重新拋出給呼叫端，完成人工介入閉環（收斂規範 §8.4）。
 */
public class ProviderCreateFailedException extends ShippingBusinessException {

    private final String providerStatus;
    private final String providerMessage;

    public ProviderCreateFailedException(GrpcResponseCode code, String providerStatus,
                                         String providerMessage) {
        super(code, providerMessage != null ? providerMessage : "");
        this.providerStatus = providerStatus;
        this.providerMessage = providerMessage;
    }

    public String providerStatus() {
        return providerStatus;
    }

    public String providerMessage() {
        return providerMessage;
    }
}

package io.github.will413028.hawkr.common.obs.grpc;

/**
 * gRPC 業務異常介面。
 *
 * <p>實作此介面的 RuntimeException 子類別，會被 {@link GrpcErrorHandler} 識別為業務異常，
 * 直接回傳 {@link #code()} + {@link RuntimeException#getMessage()} 給前端，
 * 而非被 catch-all 吞掉為「系統錯誤」。</p>
 *
 * <p>各微服務的 BusinessException（如 ShippingBusinessException、OrderBusinessException）
 * 應實作此介面。</p>
 */
public interface GrpcBusinessException {

    /**
     * 業務錯誤碼（對應 GrpcResponseCode.code()）。
     */
    int code();
}

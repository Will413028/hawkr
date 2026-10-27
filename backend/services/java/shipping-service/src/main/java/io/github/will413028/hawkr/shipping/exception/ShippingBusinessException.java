package io.github.will413028.hawkr.shipping.exception;

import io.github.will413028.hawkr.common.obs.grpc.GrpcBusinessException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * Shipping Service 業務異常。
 * 攜帶 GrpcResponseCode，由 GrpcErrorHandler 的 catch-all 自動識別並回傳業務碼 + 訊息。
 *
 * <p>用法：
 * <pre>
 * // 無參數 → 使用 CodeMeta 預設訊息
 * throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
 *
 * // 有參數 → String.format(message, args)
 * throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_DATE_HOLIDAY, "出貨日期", "20260327", "端午節");
 * </pre>
 */
public class ShippingBusinessException extends RuntimeException implements GrpcBusinessException {

    private final GrpcResponseCode responseCode;

    public ShippingBusinessException(GrpcResponseCode responseCode, Object... args) {
        super(responseCode.formatMessage(args));
        this.responseCode = responseCode;
    }

    @Override
    public int code() {
        return responseCode.code();
    }

    public GrpcResponseCode responseCode() {
        return responseCode;
    }
}

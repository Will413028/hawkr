package io.github.will413028.hawkr.payment.exception;

import io.github.will413028.hawkr.common.obs.grpc.GrpcBusinessException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * Payment Service 業務異常。
 * 攜帶 GrpcResponseCode，由 GrpcErrorHandler 的 catch-all 自動識別並回傳業務碼 + 訊息。
 *
 * <p>用法：
 * <pre>
 * // 無參數 → 使用 CodeMeta 預設訊息
 * throw new PaymentBusinessException(GrpcResponseCode.PAYMENT_ALREADY_EXISTS);
 *
 * // 有參數 → String.format(message, args)
 * throw new PaymentBusinessException(GrpcResponseCode.INVALID_PAYMENT_TYPE, "未知支付方式");
 * </pre>
 */
public class PaymentBusinessException extends RuntimeException implements GrpcBusinessException {

    private final GrpcResponseCode responseCode;

    public PaymentBusinessException(GrpcResponseCode responseCode, Object... args) {
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

package io.github.will413028.hawkr.order.exception;

import io.github.will413028.hawkr.common.obs.grpc.GrpcBusinessException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * Order Service 業務異常。
 * 攜帶 GrpcResponseCode，由 GrpcErrorHandler 的 catch-all 自動識別並回傳業務碼 + 訊息。
 *
 * <p>用法：
 * <pre>
 * // 無參數 → 使用 CodeMeta 預設訊息
 * throw new OrderBusinessException(GrpcResponseCode.SHIPPING_METHOD_UNAVAILABLE);
 *
 * // 有參數 → String.format(message, args)
 * throw new OrderBusinessException(GrpcResponseCode.CHECKOUT_VALIDATION_FAILED, "商品已下架");
 * </pre>
 */
public class OrderBusinessException extends RuntimeException implements GrpcBusinessException {

    private final GrpcResponseCode responseCode;

    public OrderBusinessException(GrpcResponseCode responseCode, Object... args) {
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

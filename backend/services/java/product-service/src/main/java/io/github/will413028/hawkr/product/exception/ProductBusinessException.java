package io.github.will413028.hawkr.product.exception;

import io.github.will413028.hawkr.common.obs.grpc.GrpcBusinessException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * Product Service 業務異常。
 * 攜帶 GrpcResponseCode，由 GrpcErrorHandler 的 catch-all 自動識別並回傳業務碼 + 訊息。
 *
 * <p>必須保留 (GrpcResponseCode, String) 建構子以支援
 * {@link io.github.will413028.hawkr.common.utils.grpc.GrpcClientErrorTranslator#translate}.
 */
public class ProductBusinessException extends RuntimeException implements GrpcBusinessException {

    private final GrpcResponseCode responseCode;

    public ProductBusinessException(GrpcResponseCode responseCode) {
        super(responseCode.message());
        this.responseCode = responseCode;
    }

    public ProductBusinessException(GrpcResponseCode responseCode, String message) {
        super(message);
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

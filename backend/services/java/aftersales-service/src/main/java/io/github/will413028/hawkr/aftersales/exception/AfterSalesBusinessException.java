package io.github.will413028.hawkr.aftersales.exception;

import io.github.will413028.hawkr.common.obs.grpc.GrpcBusinessException;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * AfterSales Service 業務異常。
 * 攜帶 GrpcResponseCode，由 GrpcErrorHandler 的 catch-all 自動識別並回傳業務碼 + 訊息。
 */
public class AfterSalesBusinessException extends RuntimeException implements GrpcBusinessException {

    private final GrpcResponseCode responseCode;

    public AfterSalesBusinessException(GrpcResponseCode responseCode, Object... args) {
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

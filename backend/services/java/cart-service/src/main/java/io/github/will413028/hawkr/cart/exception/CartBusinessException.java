package io.github.will413028.hawkr.cart.exception;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * 購物車業務例外
 *
 * 攜帶 GrpcResponseCode，由 CartGrpcService 統一轉為 Response.code 回傳，
 * 對齊 Product / Order Service 的 onNext(code) 模式。
 */
public class CartBusinessException extends RuntimeException {

    private final GrpcResponseCode responseCode;

    public CartBusinessException(GrpcResponseCode responseCode) {
        super(responseCode.message());
        this.responseCode = responseCode;
    }

    public CartBusinessException(GrpcResponseCode responseCode, String message) {
        super(message);
        this.responseCode = responseCode;
    }

    public GrpcResponseCode getResponseCode() {
        return responseCode;
    }

    public int code() {
        return responseCode.code();
    }
}

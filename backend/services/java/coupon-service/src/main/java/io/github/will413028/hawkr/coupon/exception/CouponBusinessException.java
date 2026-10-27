package io.github.will413028.hawkr.coupon.exception;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/**
 * 優惠券業務例外
 *
 * 攜帶 GrpcResponseCode，由 CouponGrpcService 統一轉為 Response.code 回傳。
 */
public class CouponBusinessException extends RuntimeException {

    private final GrpcResponseCode responseCode;

    public CouponBusinessException(GrpcResponseCode responseCode) {
        super(responseCode.message());
        this.responseCode = responseCode;
    }

    public CouponBusinessException(GrpcResponseCode responseCode, String message) {
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

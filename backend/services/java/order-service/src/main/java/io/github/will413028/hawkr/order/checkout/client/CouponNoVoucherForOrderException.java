package io.github.will413028.hawkr.order.checkout.client;

/**
 * 核銷時 coupon 回 code=7009：這筆訂單沒有持有任何券。
 *
 * <p>訂單已取消且券已釋放時這是預期結果（核銷任務是付款前排的），其他情況表示券遺失，呼叫端要保留為失敗。
 */
public class CouponNoVoucherForOrderException extends IllegalStateException {

    public static final int CODE = 7009;

    public CouponNoVoucherForOrderException(String message) {
        super(message);
    }
}

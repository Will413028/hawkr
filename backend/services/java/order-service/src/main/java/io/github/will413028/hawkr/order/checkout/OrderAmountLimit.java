package io.github.will413028.hawkr.order.checkout;

/**
 * 訂單金額上限（新台幣元，含）。這是訂單的業務規則，與付款方式無關：
 * confirm 對超過者回 ORDER_AMOUNT_EXCEEDS_LIMIT。
 * invoice-service 的不變量 A20 與 {@code allocate.go} 的 {@code MaxAmount} 與本值一致，
 * 並依賴結帳已擋下超額訂單。
 */
public final class OrderAmountLimit {

    public static final long ORDER_MAX_AMOUNT = 199_999L;

    private OrderAmountLimit() {
    }
}

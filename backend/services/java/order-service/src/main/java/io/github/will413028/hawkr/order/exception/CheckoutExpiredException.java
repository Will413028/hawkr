package io.github.will413028.hawkr.order.exception;

/**
 * 結帳已過期（checkoutId TTL 失效）
 */
public class CheckoutExpiredException extends RuntimeException {
    public CheckoutExpiredException() {
        super("結帳已過期，請重新預覽");
    }
}

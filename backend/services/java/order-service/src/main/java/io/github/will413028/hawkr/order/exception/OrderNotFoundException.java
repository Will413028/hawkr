package io.github.will413028.hawkr.order.exception;

/**
 * 訂單不存在
 */
public class OrderNotFoundException extends RuntimeException {
    public OrderNotFoundException(String orderNo) {
        super("訂單不存在: " + orderNo);
    }
}

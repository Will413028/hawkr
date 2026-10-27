package io.github.will413028.hawkr.order.exception;

/**
 * 狀態不合法（不允許的狀態轉換）
 */
public class InvalidStatusException extends RuntimeException {
    public InvalidStatusException(String message) {
        super(message);
    }
}

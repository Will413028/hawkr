package io.github.will413028.hawkr.product.exception;

/**
 * 冪等性衝突例外
 * 當請求正在處理中時拋出（IN_PROGRESS 狀態）
 */
public class IdempotencyConflictException extends RuntimeException {

    private final String idempotencyKey;
    private final String status;

    public IdempotencyConflictException(String idempotencyKey, String status) {
        super("冪等性衝突，請求正在處理中: " + idempotencyKey);
        this.idempotencyKey = idempotencyKey;
        this.status = status;
    }

    public IdempotencyConflictException(String idempotencyKey, String status, String message) {
        super(message);
        this.idempotencyKey = idempotencyKey;
        this.status = status;
    }

    public String getIdempotencyKey() {
        return idempotencyKey;
    }

    public String getStatus() {
        return status;
    }
}

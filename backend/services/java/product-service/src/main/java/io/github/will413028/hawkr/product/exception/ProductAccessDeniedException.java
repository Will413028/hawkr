package io.github.will413028.hawkr.product.exception;

/**
 * 商品存取權限不足例外
 * 當使用者嘗試存取非 ACTIVE 狀態的商品且不是 store_id owner 時拋出
 */
public class ProductAccessDeniedException extends RuntimeException {

    private final String productId;
    private final String reason;

    public ProductAccessDeniedException(String productId, String reason) {
        super("無權存取商品: " + productId + " - " + reason);
        this.productId = productId;
        this.reason = reason;
    }

    public String getProductId() {
        return productId;
    }

    public String getReason() {
        return reason;
    }
}

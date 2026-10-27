package io.github.will413028.hawkr.product.exception;

/**
 * 商品不存在例外
 */
public class ProductNotFoundException extends RuntimeException {

    private final String productId;

    public ProductNotFoundException(String productId) {
        super("商品不存在: " + productId);
        this.productId = productId;
    }

    public ProductNotFoundException(String productId, String message) {
        super(message);
        this.productId = productId;
    }

    public String getProductId() {
        return productId;
    }
}

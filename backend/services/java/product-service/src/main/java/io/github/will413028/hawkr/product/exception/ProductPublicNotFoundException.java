package io.github.will413028.hawkr.product.exception;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;

/** Stable public-visibility 404 carrying a CodeMeta business code. */
public class ProductPublicNotFoundException extends ProductNotFoundException {

    private final GrpcResponseCode responseCode;

    public ProductPublicNotFoundException(String productId, GrpcResponseCode responseCode) {
        super(productId, responseCode.message());
        this.responseCode = responseCode;
    }

    public GrpcResponseCode responseCode() {
        return responseCode;
    }
}

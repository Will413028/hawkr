package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;

import java.util.Optional;

/** Maps public product existence/publication state to stable CodeMeta errors. */
public final class ProductPublicVisibilityPolicy {

    private ProductPublicVisibilityPolicy() {}

    public static Optional<GrpcResponseCode> notFoundCode(
            boolean exists, ProductPublicationState publicationState) {
        if (!exists) {
            return Optional.of(GrpcResponseCode.PRODUCT_NOT_FOUND);
        }
        if (publicationState == null || publicationState == ProductPublicationState.NEVER_PUBLISHED) {
            return Optional.of(GrpcResponseCode.PRODUCT_NOT_PUBLISHED);
        }
        if (publicationState == ProductPublicationState.REMOVED) {
            return Optional.of(GrpcResponseCode.PRODUCT_REMOVED);
        }
        return Optional.empty();
    }
}

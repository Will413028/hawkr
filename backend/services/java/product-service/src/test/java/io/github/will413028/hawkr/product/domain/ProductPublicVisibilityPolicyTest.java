package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class ProductPublicVisibilityPolicyTest {

    @Test
    void distinguishesAbsentNeverPublishedAndRemovedProducts() {
        assertThat(ProductPublicVisibilityPolicy.notFoundCode(false, null))
                .contains(GrpcResponseCode.PRODUCT_NOT_FOUND);
        assertThat(ProductPublicVisibilityPolicy.notFoundCode(true, ProductPublicationState.NEVER_PUBLISHED))
                .contains(GrpcResponseCode.PRODUCT_NOT_PUBLISHED);
        assertThat(ProductPublicVisibilityPolicy.notFoundCode(true, ProductPublicationState.REMOVED))
                .contains(GrpcResponseCode.PRODUCT_REMOVED);
    }

    @Test
    void publicAndTemporarilyUnavailableProductsHaveNoVisibilityError() {
        assertThat(ProductPublicVisibilityPolicy.notFoundCode(true, ProductPublicationState.PUBLIC)).isEmpty();
        assertThat(ProductPublicVisibilityPolicy.notFoundCode(true, ProductPublicationState.TEMPORARILY_UNAVAILABLE)).isEmpty();
    }

    @Test
    void missingDerivedStateFailsClosed() {
        assertThat(ProductPublicVisibilityPolicy.notFoundCode(true, null))
                .contains(GrpcResponseCode.PRODUCT_NOT_PUBLISHED);
    }
}

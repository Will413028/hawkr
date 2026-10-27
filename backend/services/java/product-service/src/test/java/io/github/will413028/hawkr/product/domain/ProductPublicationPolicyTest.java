package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ProductPublicationPolicyTest {

    @Test
    void firstActivationRecordsImmutablePublicationFact() {
        LocalDateTime now = LocalDateTime.of(2026, 8, 9, 12, 0);
        assertEquals(now, ProductPublicationPolicy.firstPublishedAtOnActivation(
                null, ProductStatus.ACTIVE.value(), now));

        LocalDateTime original = now.minusDays(10);
        assertEquals(original, ProductPublicationPolicy.firstPublishedAtOnActivation(
                original, ProductStatus.ACTIVE.value(), now));
    }

    @Test
    void nonActiveProductDoesNotInventPublicationHistory() {
        LocalDateTime now = LocalDateTime.of(2026, 8, 9, 12, 0);
        assertNull(ProductPublicationPolicy.firstPublishedAtOnActivation(
                null, ProductStatus.DRAFT.value(), now));
        assertEquals(ProductPublicationState.NEVER_PUBLISHED,
                ProductPublicationPolicy.deriveState(
                        null, ProductStatus.INACTIVE.value(), null, null, true));
    }

    @Test
    void sellerOffShelfPreservesPublicPageAsTemporarilyUnavailable() {
        assertEquals(ProductPublicationState.TEMPORARILY_UNAVAILABLE,
                ProductPublicationPolicy.deriveState(
                        LocalDateTime.now(), ProductStatus.INACTIVE.value(), null, null, true));
    }

    @Test
    void activeProductWithPublicationFactIsPublic() {
        assertEquals(ProductPublicationState.PUBLIC,
                ProductPublicationPolicy.deriveState(
                        LocalDateTime.now(), ProductStatus.ACTIVE.value(), null, null, true));
    }

    @Test
    void explicitRemovalWinsOverStatusAndStock() {
        assertFalse(ProductPublicationPolicy.isPublic(ProductPublicationState.REMOVED));
        assertFalse(ProductPublicationPolicy.isPurchasable(
                LocalDateTime.now(), ProductStatus.ACTIVE.value(), 10L, LocalDateTime.now(), null, true));
    }

    @Test
    void publicAvailabilitySeparatesStockFromUrlVisibility() {
        assertTrue(ProductPublicationPolicy.isPublic(ProductPublicationState.PUBLIC));
        assertTrue(ProductPublicationPolicy.isPublic(ProductPublicationState.TEMPORARILY_UNAVAILABLE));
        LocalDateTime firstPublishedAt = LocalDateTime.now();
        assertTrue(ProductPublicationPolicy.isPurchasable(
                firstPublishedAt, ProductStatus.ACTIVE.value(), 1L, null, null, true));
        assertFalse(ProductPublicationPolicy.isPurchasable(
                firstPublishedAt, ProductStatus.ACTIVE.value(), 0L, null, null, true));
        assertFalse(ProductPublicationPolicy.isPurchasable(
                firstPublishedAt, ProductStatus.INACTIVE.value(), 10L, null, null, true));
    }

    @Test
    void nonExposableStoreHidesPurchaseButKeepsPublishedProductPage() {
        LocalDateTime firstPublishedAt = LocalDateTime.now();
        assertEquals(ProductPublicationState.TEMPORARILY_UNAVAILABLE,
                ProductPublicationPolicy.deriveState(
                        firstPublishedAt, ProductStatus.ACTIVE.value(), null, null, false));
        assertTrue(ProductPublicationPolicy.isPublic(
                ProductPublicationPolicy.deriveState(
                        firstPublishedAt, ProductStatus.ACTIVE.value(), null, null, false)));
        assertFalse(ProductPublicationPolicy.isPurchasable(
                firstPublishedAt, ProductStatus.ACTIVE.value(), 10L, null, null, false));
    }

    @Test
    void storeExposableStateTable() {
        LocalDateTime published = LocalDateTime.now();
        String active = ProductStatus.ACTIVE.value();
        String inactive = ProductStatus.INACTIVE.value();
        for (boolean exposable : new boolean[] {true, false}) {
            // 移除永遠優先於店家可曝光
            assertEquals(ProductPublicationState.REMOVED,
                    ProductPublicationPolicy.deriveState(published, active, published, null, exposable));
            assertEquals(ProductPublicationState.REMOVED,
                    ProductPublicationPolicy.deriveState(published, active, null, published, exposable));
            // 從未公開不因店家可曝光而改變
            assertEquals(ProductPublicationState.NEVER_PUBLISHED,
                    ProductPublicationPolicy.deriveState(null, active, null, null, exposable));
            // 下架本來就是暫時不可用
            assertEquals(ProductPublicationState.TEMPORARILY_UNAVAILABLE,
                    ProductPublicationPolicy.deriveState(published, inactive, null, null, exposable));
        }
        assertEquals(ProductPublicationState.PUBLIC,
                ProductPublicationPolicy.deriveState(published, active, null, null, true));
        assertEquals(ProductPublicationState.TEMPORARILY_UNAVAILABLE,
                ProductPublicationPolicy.deriveState(published, active, null, null, false));
        assertTrue(ProductPublicationPolicy.isPurchasable(published, active, 1L, null, null, true));
        assertFalse(ProductPublicationPolicy.isPurchasable(published, active, 1L, null, null, false));
    }
}

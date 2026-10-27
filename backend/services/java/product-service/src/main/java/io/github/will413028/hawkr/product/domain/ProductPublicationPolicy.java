package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.common.utils.code.ProductPublicationState;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;

import java.time.LocalDateTime;

/**
 * Keeps the public lifecycle independent from the product review/sale status.
 *
 * <p>This policy covers a single product. Lists and counts apply the same store-exposure rule in SQL
 * ({@code store_exposure}) so that pagination and totals stay exact.
 */
public final class ProductPublicationPolicy {

    private ProductPublicationPolicy() {
    }

    public static LocalDateTime firstPublishedAtOnActivation(
            LocalDateTime currentFirstPublishedAt,
            String nextStatus,
            LocalDateTime now) {
        if (currentFirstPublishedAt != null) {
            return currentFirstPublishedAt;
        }
        return ProductStatus.ACTIVE.value().equals(nextStatus) ? now : null;
    }

    public static ProductPublicationState deriveState(
            LocalDateTime firstPublishedAt,
            String status,
            LocalDateTime deletedAt,
            LocalDateTime sellerDeletedAt,
            boolean storeExposable) {
        if (deletedAt != null || sellerDeletedAt != null) {
            return ProductPublicationState.REMOVED;
        }
        if (firstPublishedAt == null) {
            return ProductPublicationState.NEVER_PUBLISHED;
        }
        if (!storeExposable) {
            return ProductPublicationState.TEMPORARILY_UNAVAILABLE;
        }
        if (ProductStatus.ACTIVE.value().equals(status)) {
            return ProductPublicationState.PUBLIC;
        }
        return ProductPublicationState.TEMPORARILY_UNAVAILABLE;
    }

    public static boolean isPublic(ProductPublicationState state) {
        return state == ProductPublicationState.PUBLIC
                || state == ProductPublicationState.TEMPORARILY_UNAVAILABLE;
    }

    public static boolean isPurchasable(
            LocalDateTime firstPublishedAt,
            String status,
            Long stockQty,
            LocalDateTime deletedAt,
            LocalDateTime sellerDeletedAt,
            boolean storeExposable) {
        return deriveState(firstPublishedAt, status, deletedAt, sellerDeletedAt, storeExposable)
                == ProductPublicationState.PUBLIC
                && ProductStatus.ACTIVE.value().equals(status)
                && stockQty != null
                && stockQty > 0;
    }
}

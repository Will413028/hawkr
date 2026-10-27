package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.seller.grpc.PublicCityStores;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/** Joins seller-owned city membership with product-owned public counts. */
public final class ProductSellerCitySummaryPolicy {

    /**
     * How many stores the preview carries. Mirrors the bound seller-service already
     * applies to {@code preview_store_ids}; re-derived here because the preview is
     * now taken from the filtered subset rather than passed through.
     */
    private static final int PREVIEW_LIMIT = 6;

    private ProductSellerCitySummaryPolicy() {}

    /**
     * @param sellerCount     stores in this city with at least one publicly listed
     *                        product — NOT the city's approved-seller count
     * @param previewStoreIds up to {@value #PREVIEW_LIMIT} of those same stores
     */
    public record Summary(
            String cityCode,
            String cityName,
            long productCount,
            long sellerCount,
            List<String> previewStoreIds) {}

    /**
     * Aggregates a city's sellers into the public summary.
     *
     * <p><b>Every field is scoped to stores that actually have something on the
     * shelf.</b> {@code productCount} always was (an empty store contributes 0),
     * but {@code sellerCount} used to be {@code storeIds.size()} and
     * {@code previewStoreIds} used to be seller-service's unfiltered preview, so
     * both counted and showed empty storefronts. The website renders these as
     * merchant cards, where an empty store degrades into a placeholder — no
     * avatar, the literal label "查看商家" — and on dev that was every card in
     * 台北市 (sellerCount 6, public products 0).
     *
     * <p>No extra query is involved: {@code publicProductCountsByStore} is already
     * in hand, and it is the same {@code findPublicProductCountsByStore} aggregate
     * the recommendation rail filters on (02 §5.13c), so "publicly listed" has one
     * definition across both surfaces.
     *
     * <p>Cities are never dropped, however empty: the endpoint's contract is a
     * fixed 22-row list of Taiwan's counties, so a city with no qualifying seller
     * reports zeros rather than disappearing.
     */
    public static List<Summary> aggregate(
            List<PublicCityStores> cities, Map<Long, Long> publicProductCountsByStore) {
        List<Summary> result = new ArrayList<>(cities.size());
        for (PublicCityStores city : cities) {
            long productCount = 0;
            // Order is seller-service's (`ORDER BY city, store_id`), and filtering
            // preserves it, so the preview stays deterministic across calls.
            List<String> storesWithProducts = new ArrayList<>();
            for (String rawStoreId : city.getStoreIdsList()) {
                long count;
                try {
                    count = publicProductCountsByStore.getOrDefault(Long.parseLong(rawStoreId), 0L);
                } catch (NumberFormatException e) {
                    throw new IllegalArgumentException(
                            "seller-service returned non-numeric store_id: " + rawStoreId, e);
                }
                if (count <= 0) {
                    continue;
                }
                productCount += count;
                storesWithProducts.add(rawStoreId);
            }
            List<String> preview = List.copyOf(
                    storesWithProducts.subList(0, Math.min(PREVIEW_LIMIT, storesWithProducts.size())));
            result.add(new Summary(
                    city.getCityCode(), city.getCityName(), productCount,
                    storesWithProducts.size(), preview));
        }
        return result;
    }
}

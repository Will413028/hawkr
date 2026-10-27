package io.github.will413028.hawkr.product.domain;

import io.github.will413028.hawkr.seller.grpc.PublicCityStores;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class ProductSellerCitySummaryPolicyTest {

    @Test
    void countsOnlyStoresWithPublicProductsAndKeepsPreviewBounded() {
        // 8 approved sellers in the city; 106 and 107 have nothing on the shelf.
        var taipei = PublicCityStores.newBuilder()
                .setCityCode("TW-TPE")
                .setCityName("台北市")
                .addAllStoreIds(List.of("101", "102", "103", "104", "105", "106", "107", "108"))
                .addAllPreviewStoreIds(List.of("101", "102", "103", "104", "105", "106"))
                .build();

        var result = ProductSellerCitySummaryPolicy.aggregate(
                List.of(taipei),
                Map.of(101L, 60L, 102L, 50L, 103L, 1L, 104L, 1L, 105L, 1L, 108L, 5L));

        assertThat(result).hasSize(1);
        assertThat(result.getFirst().cityCode()).isEqualTo("TW-TPE");
        assertThat(result.getFirst().productCount()).isEqualTo(118L);
        assertThat(result.getFirst().sellerCount()).isEqualTo(6L);
        // 106/107 are dropped, so 108 is pulled into the 6-slot preview: the
        // preview is the filtered subset's head, NOT seller-service's pass-through
        // (which would have ended "...105, 106" and shown an empty storefront).
        assertThat(result.getFirst().previewStoreIds())
                .containsExactly("101", "102", "103", "104", "105", "108");
    }

    /**
     * The discriminating case, kept deliberately small: three sellers, one of them
     * empty. Every assertion below flips if the product filter is removed —
     * sellerCount becomes 3 and the preview carries the empty store — so this test
     * cannot pass on the pre-fix behaviour.
     *
     * <p>Why it matters: the website renders previewStoreIds as merchant cards and
     * sellerCount as the city's merchant tally. An empty storefront has no avatar
     * and no usable name, so it degrades into a placeholder card — which is what
     * 台北市 looked like on dev (sellerCount 6, zero public products between them).
     */
    @Test
    @DisplayName("有 0 件在架商品的店不計入 sellerCount，也不進 preview")
    void emptyStorefrontIsNeitherCountedNorPreviewed() {
        var city = PublicCityStores.newBuilder()
                .setCityCode("TW-TPE")
                .setCityName("台北市")
                .addAllStoreIds(List.of("201", "202", "203"))
                .addAllPreviewStoreIds(List.of("201", "202", "203"))
                .build();

        var result = ProductSellerCitySummaryPolicy.aggregate(
                List.of(city), Map.of(201L, 3L, 203L, 4L)); // 202 has none

        assertThat(result.getFirst().sellerCount()).isEqualTo(2L);
        assertThat(result.getFirst().previewStoreIds()).containsExactly("201", "203");
        assertThat(result.getFirst().previewStoreIds()).doesNotContain("202");
        assertThat(result.getFirst().productCount()).isEqualTo(7L);
    }

    /**
     * A store present in the count map with an explicit zero is the same answer as
     * one that is absent. Guards a {@code containsKey}-style rewrite, which would
     * read as "has products" for a row that says the opposite.
     */
    @Test
    void explicitZeroCountsAsNoPublicProduct() {
        var city = PublicCityStores.newBuilder()
                .setCityCode("TW-TPE").setCityName("台北市")
                .addAllStoreIds(List.of("301", "302")).build();

        var result = ProductSellerCitySummaryPolicy.aggregate(
                List.of(city), Map.of(301L, 0L, 302L, 2L));

        assertThat(result.getFirst().sellerCount()).isEqualTo(1L);
        assertThat(result.getFirst().previewStoreIds()).containsExactly("302");
    }

    /**
     * The endpoint's contract is a fixed 22-county list, so a city whose sellers
     * are all empty must still appear — with zeros, not omitted.
     */
    @Test
    void cityWhoseSellersAreAllEmptyStaysInSummaryWithZeroCounts() {
        var city = PublicCityStores.newBuilder()
                .setCityCode("TW-TPE").setCityName("台北市")
                .addAllStoreIds(List.of("401", "402")).build();

        var result = ProductSellerCitySummaryPolicy.aggregate(List.of(city), Map.of());

        assertThat(result).hasSize(1);
        assertThat(result.getFirst().cityCode()).isEqualTo("TW-TPE");
        assertThat(result.getFirst().sellerCount()).isZero();
        assertThat(result.getFirst().productCount()).isZero();
        assertThat(result.getFirst().previewStoreIds()).isEmpty();
    }

    @Test
    void cityWithNoSellersRemainsInSummaryWithZeroCounts() {
        var city = PublicCityStores.newBuilder()
                .setCityCode("TW-LIE").setCityName("連江縣").build();

        var result = ProductSellerCitySummaryPolicy.aggregate(List.of(city), Map.of());

        assertThat(result.getFirst().productCount()).isZero();
        assertThat(result.getFirst().sellerCount()).isZero();
    }

    @Test
    void malformedSellerStoreIdFailsClosedInsteadOfSilentlyUndercounting() {
        var city = PublicCityStores.newBuilder()
                .setCityCode("TW-TPE").setCityName("台北市").addStoreIds("not-a-number").build();

        assertThatThrownBy(() -> ProductSellerCitySummaryPolicy.aggregate(List.of(city), Map.of()))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("not-a-number");
    }
}

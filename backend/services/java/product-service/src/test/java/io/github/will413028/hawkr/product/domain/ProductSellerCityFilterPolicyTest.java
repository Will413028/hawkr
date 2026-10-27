package io.github.will413028.hawkr.product.domain;

import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ProductSellerCityFilterPolicyTest {

    @Test
    void usesAllCityStoresWhenNoOtherStoreFilterExists() {
        assertThat(ProductSellerCityFilterPolicy.intersect(null, List.of(101L, 102L)))
                .containsExactly(101L, 102L);
    }

    @Test
    void intersectsExplicitStoreFilterWithCompleteCityMembership() {
        assertThat(ProductSellerCityFilterPolicy.intersect(List.of(102L, 999L), List.of(101L, 102L)))
                .containsExactly(102L);
    }

    @Test
    void returnsEmptyWhenCityHasNoMatchingStores() {
        assertThat(ProductSellerCityFilterPolicy.intersect(List.of(999L), List.of(101L, 102L))).isEmpty();
        assertThat(ProductSellerCityFilterPolicy.intersect(null, List.of())).isEmpty();
    }
}

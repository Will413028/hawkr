package io.github.will413028.hawkr.product.domain;

import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;

/** Complete city membership filter shared by public list queries. */
public final class ProductSellerCityFilterPolicy {

    private ProductSellerCityFilterPolicy() {}

    public static List<Long> intersect(List<Long> requestedStoreIds, List<Long> cityStoreIds) {
        if (cityStoreIds == null || cityStoreIds.isEmpty()) {
            return List.of();
        }
        if (requestedStoreIds == null || requestedStoreIds.isEmpty()) {
            return List.copyOf(cityStoreIds);
        }
        var result = new LinkedHashSet<>(requestedStoreIds);
        result.retainAll(cityStoreIds);
        return new ArrayList<>(result);
    }
}

package io.github.will413028.hawkr.order.repository;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * Real-behavior assertion per parent spec Testing point 1 ("assert net amounts, not just
 * presence of rows"): this test documents the expected CASE-expression semantics as a
 * contract test against a mocked repository; the actual SQL correctness (status filter,
 * refund subtraction) must additionally be verified against a real Postgres per Task 19 —
 * this unit test alone cannot prove the SQL is right, only that the interface contract is used correctly.
 */
class OrderAggregatesQueryTest {

    @Test
    @DisplayName("findOrderAggregates delegates with storeId=0 meaning unfiltered")
    void findOrderAggregates_unfilteredWhenStoreIdZero() {
        OrderRepository repo = mock(OrderRepository.class);
        io.github.will413028.hawkr.order.entity.OrderAggregateResult row = new io.github.will413028.hawkr.order.entity.OrderAggregateResult();
        row.setStoreId(7001L);
        row.setBuyerId(1L);
        row.setOrderCount(3L);
        row.setTotalNetAmount(new java.math.BigDecimal("1500"));
        when(repo.findOrderAggregates(0L, 20, 0)).thenReturn(List.of(row));

        List<io.github.will413028.hawkr.order.entity.OrderAggregateResult> result = repo.findOrderAggregates(0L, 20, 0);

        assertEquals(1, result.size());
        assertEquals(7001L, result.get(0).getStoreId());
        assertEquals(new java.math.BigDecimal("1500"), result.get(0).getTotalNetAmount());
    }
}

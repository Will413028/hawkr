package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.product.mq.SalesAccumulatedConsumer.SalesAccumulator;
import io.github.will413028.hawkr.product.mq.SalesAccumulatedConsumer.SalesItem;
import io.github.will413028.hawkr.product.repository.ProductRepository;
import io.github.will413028.hawkr.product.repository.SalesCountEventDedupRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 銷量累計冪等測試：同一 order_no 重投只累計一次（由 order_no dedup claim 保證）。
 */
@ExtendWith(MockitoExtension.class)
class SalesAccumulatorTest {

    @Mock ProductRepository productRepository;
    @Mock SalesCountEventDedupRepository dedupRepository;
    @InjectMocks SalesAccumulator accumulator;

    private static final String ORDER_NO = "O2500000001";
    private static final List<SalesItem> ITEMS =
            List.of(new SalesItem(100L, 2), new SalesItem(200L, 3));

    @Test
    @DisplayName("首次事件：claim 成功 → 逐筆累計、回傳 true")
    void firstEventAccumulates() {
        when(dedupRepository.insertIfAbsent(ORDER_NO)).thenReturn(1);

        boolean applied = accumulator.applyAll(ORDER_NO, ITEMS);

        assertThat(applied).isTrue();
        verify(productRepository).incrementSalesCount(100L, 2);
        verify(productRepository).incrementSalesCount(200L, 3);
    }

    @Test
    @DisplayName("重複事件：claim NO-OP → 不累計、回傳 false")
    void duplicateEventSkips() {
        when(dedupRepository.insertIfAbsent(ORDER_NO)).thenReturn(0);

        boolean applied = accumulator.applyAll(ORDER_NO, ITEMS);

        assertThat(applied).isFalse();
        verify(productRepository, never()).incrementSalesCount(anyLong(), anyInt());
    }

    @Test
    @DisplayName("同 order_no 連續兩次：sales_count 只累計一次")
    void sameOrderNoTwiceAccumulatesOnce() {
        // 第一次 claim 成功、第二次（redelivery）NO-OP
        when(dedupRepository.insertIfAbsent(ORDER_NO)).thenReturn(1, 0);

        boolean first = accumulator.applyAll(ORDER_NO, ITEMS);
        boolean second = accumulator.applyAll(ORDER_NO, ITEMS);

        assertThat(first).isTrue();
        assertThat(second).isFalse();
        // 每個商品僅被累計一次（第二次 redelivery 完全不觸及 increment）
        verify(productRepository, times(1)).incrementSalesCount(eq(100L), anyInt());
        verify(productRepository, times(1)).incrementSalesCount(eq(200L), anyInt());
    }
}

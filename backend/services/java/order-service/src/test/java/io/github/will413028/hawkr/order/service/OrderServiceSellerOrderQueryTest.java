package io.github.will413028.hawkr.order.service;

import io.github.will413028.hawkr.order.entity.SellerOrderQuery;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.same;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;

/**
 * 賣家訂單列表查詢條件的正規化（backend#2478，56 號設計書 §6.1.10）。
 *
 * <p>SQL 端的形狀由 {@code OrderRepositorySellerFilterSqlTest} 守，這裡守的是進 SQL 之前
 * 那一層：keyword 的跳脫、日期的解析與邊界補值，以及「列表與筆數拿到的是同一個條件實例」。
 */
class OrderServiceSellerOrderQueryTest {

    private static final long STORE_ID = 2685914682L;

    private final OrderRepository orderRepository = mock(OrderRepository.class);
    private final OrderService orderService = OrderServiceTestHarness.withOrderRepository(orderRepository);

    @Test
    @DisplayName("空 keyword 不篩選；一般 keyword 前後補萬用字元")
    void keywordPattern() {
        assertThat(query("", "", "").getKeywordPattern()).isNull();
        assertThat(query("   ", "", "").getKeywordPattern()).isNull();
        assertThat(query(" 蘋果 ", "", "").getKeywordPattern()).isEqualTo("%蘋果%");
        assertThat(query("O123456789", "", "").getKeywordPattern()).isEqualTo("%O123456789%");
    }

    @Test
    @DisplayName("keyword 內的 LIKE 萬用字元先跳脫，避免「100%」變成撈全店")
    void keywordWildcardsAreEscaped() {
        assertThat(query("100%", "", "").getKeywordPattern()).isEqualTo("%100\\%%");
        assertThat(query("a_b", "", "").getKeywordPattern()).isEqualTo("%a\\_b%");
        // 跳脫字元本身要先處理，否則使用者輸入的 "\%" 會被還原成萬用字元
        assertThat(query("\\%", "", "").getKeywordPattern()).isEqualTo("%\\\\\\%%");
    }

    @Test
    @DisplayName("日期：純日期的上界補到當日最後一微秒，下界補 00:00:00")
    void dateOnlyBoundsCoverTheWholeDay() {
        SellerOrderQuery q = query("", "2026-08-13", "2026-08-13");
        assertThat(q.getDateFrom()).isEqualTo(LocalDateTime.of(2026, 8, 13, 0, 0, 0));
        assertThat(q.getDateTo()).isEqualTo(LocalDateTime.of(2026, 8, 13, 23, 59, 59, 999_999_000));
    }

    @Test
    @DisplayName("日期：帶時間的形式逐字採用，Z 尾綴視為 UTC")
    void instantBoundsAreTakenLiterally() {
        SellerOrderQuery q = query("", "2026-08-13T04:00:00Z", "2026-08-13T16:30:00Z");
        assertThat(q.getDateFrom()).isEqualTo(LocalDateTime.of(2026, 8, 13, 4, 0, 0));
        assertThat(q.getDateTo()).isEqualTo(LocalDateTime.of(2026, 8, 13, 16, 30, 0));
    }

    @Test
    @DisplayName("日期空字串＝不篩選；格式不合法則明確拒絕而非靜默忽略")
    void invalidDateIsRejected() {
        SellerOrderQuery none = query("", "", "");
        assertThat(none.getDateFrom()).isNull();
        assertThat(none.getDateTo()).isNull();

        assertThatThrownBy(() -> query("", "2026/08/13", ""))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("date_from");
        assertThatThrownBy(() -> query("", "", "yesterday"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("date_to");
    }

    @Test
    @DisplayName("狀態沿用逗號分隔多值，空字串＝全部")
    void statusFilterStillParsed() {
        assertThat(query("", "", "").getStatusList()).isEmpty();
        assertThat(orderService.sellerOrderQuery(STORE_ID, 0L, "20, 30", "", "", "").getStatusList())
                .containsExactly("20", "30");
    }

    @Test
    @DisplayName("列表與筆數收到的是同一個條件實例（條件無從分岔）")
    void listAndCountReceiveTheSameQueryInstance() {
        SellerOrderQuery q = query("蘋果", "2026-08-01", "2026-08-13");

        orderService.getSellerOrders(q, 2, 20);
        orderService.countSellerOrders(q);

        verify(orderRepository).findSellerOrders(same(q), eq(20), eq(20));
        verify(orderRepository).countSellerOrders(same(q));
    }

    @Test
    @DisplayName("分頁：page 從 1 起算，offset =(page-1)*size")
    void offsetIsZeroBased() {
        SellerOrderQuery q = query("", "", "");
        orderService.getSellerOrders(q, 1, 20);
        verify(orderRepository).findSellerOrders(same(q), eq(20), eq(0));
    }

    private SellerOrderQuery query(String keyword, String dateFrom, String dateTo) {
        return orderService.sellerOrderQuery(STORE_ID, 0L, "", keyword, dateFrom, dateTo);
    }
}

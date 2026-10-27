package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.entity.StoreOrderStatsResult;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.autoconfigure.ImportAutoConfiguration;
import org.springframework.boot.autoconfigure.flyway.FlywayAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceTransactionManagerAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.JdbcTemplateAutoConfiguration;
import org.springframework.boot.autoconfigure.sql.init.SqlInitializationAutoConfiguration;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * {@code findStoreOrderStats}（ListStoreOrderStats）在真實 PostgreSQL 上的語意：成立訂單口徑、不重複買家、
 * 賣家取消、時間窗（成立訂單以付款時間、賣家取消以取消時間計，不是下單時間）、空店排除與 keyset 分頁。Mockito 驗不到 FILTER／COUNT(DISTINCT) 的結果。
 */
@SpringBootTest(classes = ListStoreOrderStatsIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class ListStoreOrderStatsIT {

    private static final List<String> QUALIFYING = List.of(
            OrderStatus.PAID.value(), OrderStatus.PENDING_SHIPMENT.value(),
            OrderStatus.IN_TRANSIT.value(), OrderStatus.WAITING_PICKUP.value(),
            OrderStatus.SHIPPING_ABNORMAL.value(), OrderStatus.DELIVERED.value(),
            OrderStatus.COMPLETED.value(), OrderStatus.PARTIALLY_REFUNDED.value());
    private static final String CANCELLED = OrderStatus.CANCELLED.value();

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_store_stats_test")
            .withUsername("test")
            .withPassword("test")
            .withEnv("TZ", "UTC")
            .withEnv("PGTZ", "UTC");

    @DynamicPropertySource
    static void registerProps(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
        registry.add("spring.datasource.driver-class-name", () -> "org.postgresql.Driver");
        registry.add("spring.flyway.url", POSTGRES::getJdbcUrl);
        registry.add("spring.flyway.user", POSTGRES::getUsername);
        registry.add("spring.flyway.password", POSTGRES::getPassword);
        registry.add("spring.flyway.enabled", () -> "true");
        registry.add("spring.flyway.locations", () -> "classpath:db/migration");
        registry.add("spring.flyway.baseline-on-migrate", () -> "false");
    }

    @SpringBootConfiguration
    @ImportAutoConfiguration({
            DataSourceAutoConfiguration.class,
            DataSourceTransactionManagerAutoConfiguration.class,
            JdbcTemplateAutoConfiguration.class,
            SqlInitializationAutoConfiguration.class,
            FlywayAutoConfiguration.class,
            MybatisPlusAutoConfiguration.class,
            MyBatisPlusAutoConfiguration.class
    })
    @MapperScan("io.github.will413028.hawkr.order.repository")
    static class MinimalConfig {
    }

    @Autowired
    private OrderRepository orderRepository;

    @Autowired
    private JdbcTemplate jdbc;

    private long nextId;

    @BeforeEach
    void clean() {
        jdbc.update("DELETE FROM order_items");
        jdbc.update("DELETE FROM orders");
        nextId = 1;
    }

    private List<StoreOrderStatsResult> query(int days, long after, int limit) {
        return orderRepository.findStoreOrderStats(days, after, limit, QUALIFYING, CANCELLED, "SELLER");
    }

    @Test
    @DisplayName("成交口徑、不重複買家、賣家取消與空店排除")
    void countsQualifyingBuyersAndSellerCancellations() {
        // 店 100：買家 1 兩單（PAID、COMPLETED）、買家 2 一單（PARTIALLY_REFUNDED）→ 3 單 2 人；
        // 另有待付款(0)、全額退款(70)、買家取消、系統取消各一單，都不算成交也不算賣家取消；賣家取消一單。
        order("100", 1, "10", "1 day");
        order("100", 1, "50", "1 day");
        order("100", 2, "60", "1 day");
        order("100", 3, "0", "1 day");
        order("100", 3, "70", "1 day");
        order("100", 4, CANCELLED, "1 day", "BUYER");
        order("100", 5, CANCELLED, "1 day", "SYSTEM");
        order("100", 6, CANCELLED, "1 day", "SELLER");
        // 店 200：只有賣家取消 → 出現，成交為 0。
        order("200", 7, CANCELLED, "1 day", "SELLER");
        // 店 300：只有待付款與買家取消 → 不出現。
        order("300", 8, "0", "1 day");
        order("300", 9, CANCELLED, "1 day", "BUYER");
        // 店 400：成交單在時間窗之外 → 不出現。
        order("400", 10, "50", "40 days");

        List<StoreOrderStatsResult> rows = query(30, 0L, 100);

        assertThat(rows).extracting(StoreOrderStatsResult::getStoreId).containsExactly(100L, 200L);
        StoreOrderStatsResult s100 = rows.get(0);
        assertThat(s100.getQualifyingOrders()).isEqualTo(3);
        assertThat(s100.getDistinctBuyers()).isEqualTo(2);
        assertThat(s100.getSellerCancelledOrders()).isEqualTo(1);
        StoreOrderStatsResult s200 = rows.get(1);
        assertThat(s200.getQualifyingOrders()).isZero();
        assertThat(s200.getDistinctBuyers()).isZero();
        assertThat(s200.getSellerCancelledOrders()).isEqualTo(1);
    }

    @Test
    @DisplayName("days 決定時間窗；窗外的訂單不計")
    void windowDaysBoundsOrders() {
        order("100", 1, "50", "5 days");
        order("100", 2, "50", "20 days");

        assertThat(query(7, 0L, 10).get(0).getQualifyingOrders()).isEqualTo(1);
        assertThat(query(30, 0L, 10).get(0).getQualifyingOrders()).isEqualTo(2);
    }

    @Test
    @DisplayName("keyset 分頁：store_id 升冪、游標之後、limit 生效，頁與頁之間不重不漏")
    void keysetPagingWalksAllStoresOnce() {
        for (String store : List.of("300", "100", "500", "200", "400")) {
            order(store, 1, "50", "1 day");
        }

        List<StoreOrderStatsResult> first = query(30, 0L, 2);
        List<StoreOrderStatsResult> second = query(30, first.get(1).getStoreId(), 2);
        List<StoreOrderStatsResult> third = query(30, second.get(1).getStoreId(), 2);

        assertThat(first).extracting(StoreOrderStatsResult::getStoreId).containsExactly(100L, 200L);
        assertThat(second).extracting(StoreOrderStatsResult::getStoreId).containsExactly(300L, 400L);
        assertThat(third).extracting(StoreOrderStatsResult::getStoreId).containsExactly(500L);
        assertThat(query(30, 500L, 2)).isEmpty();
    }

    @Test
    @DisplayName("成立訂單的時間窗以付款時間計：40 天前下單、2 天前付款算；5 天前下單但付款時間在窗外不算")
    void qualifyingOrdersAreWindowedByPayTime() {
        order("100", 1, "50", "40 days", null, "2 days", null);   // 下單在窗外、付款在窗內 → 算
        order("100", 2, "50", "10 days", null, "9 days", null);   // 付款在 7 天窗外、30 天窗內
        order("200", 3, "50", "5 days", null, "40 days", null);   // 付款時間在窗外（資料上不合理的下單晚於付款，仍以付款時間為準）→ 不算
        order("300", 4, "10", "5 days", null, null, null);        // 已付款狀態但沒有 pay_time → 不算（沒有付款時間就無從落入時間窗）

        assertThat(query(7, 0L, 10)).extracting(StoreOrderStatsResult::getStoreId).containsExactly(100L);
        assertThat(query(7, 0L, 10).get(0).getQualifyingOrders()).isEqualTo(1);
        assertThat(query(30, 0L, 10)).extracting(StoreOrderStatsResult::getStoreId).containsExactly(100L);
        assertThat(query(30, 0L, 10).get(0).getQualifyingOrders()).isEqualTo(2);
        assertThat(query(30, 0L, 10).get(0).getDistinctBuyers()).isEqualTo(2);
    }

    @Test
    @DisplayName("賣家取消的時間窗以取消時間計：40 天前下單、2 天前取消算；近期下單但取消在窗外不算；沒有 cancelled_at 不算")
    void sellerCancellationsAreWindowedByCancelTime() {
        order("100", 1, CANCELLED, "40 days", "SELLER", null, "2 days");
        order("200", 2, CANCELLED, "3 days", "SELLER", null, "40 days");
        order("300", 3, CANCELLED, "3 days", "SELLER", null, null);

        List<StoreOrderStatsResult> rows = query(30, 0L, 10);

        assertThat(rows).extracting(StoreOrderStatsResult::getStoreId).containsExactly(100L);
        assertThat(rows.get(0).getSellerCancelledOrders()).isEqualTo(1);
        assertThat(rows.get(0).getQualifyingOrders()).isZero();
    }

    /** 沒有指定 createdAge 時，下單時間比相關的時間戳再早一天（付款或取消一定在下單之後）。 */
    private void order(String storeId, long buyerId, String status, String age) {
        order(storeId, buyerId, status, age, null);
    }

    private void order(String storeId, long buyerId, String status, String age, String cancelByRole) {
        boolean cancelled = CANCELLED.equals(status);
        boolean paid = QUALIFYING.contains(status);
        String created = jdbc.queryForObject("SELECT (?::interval + INTERVAL '1 day')::text", String.class, age);
        order(storeId, buyerId, status, created, cancelByRole, paid ? age : null, cancelled ? age : null);
    }

    private void order(String storeId, long buyerId, String status, String createdAge, String cancelByRole,
                       String payAge, String cancelAge) {
        long id = nextId++;
        jdbc.update("""
                INSERT INTO orders (id, order_no, buyer_id, store_id, total_original_amount,
                                    total_actual_amount, status, cancel_by_role, created_at, pay_time, cancelled_at)
                VALUES (?, ?, ?, ?, 100, 100, ?, ?, NOW() - ?::interval,
                        CASE WHEN ?::text IS NULL THEN NULL ELSE NOW() - ?::interval END,
                        CASE WHEN ?::text IS NULL THEN NULL ELSE NOW() - ?::interval END)
                """, id, "O" + id, buyerId, Long.parseLong(storeId), status, cancelByRole, createdAge,
                payAge, payAge, cancelAge, cancelAge);
    }
}

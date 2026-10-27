package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.entity.StorePeriodOrderStatsResult;
import io.github.will413028.hawkr.order.service.OrderService;
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
 * {@code findStorePeriodOrderStats}（GetStorePeriodOrderStats）在真實 PostgreSQL 上的語意：成立訂單口徑（與
 * ListStoreOrderStats 同一組狀態）、不重複買家、營收扣除已退款金額、時間窗 [start, end) 以付款時間計、只算指定的店。
 */
@SpringBootTest(classes = StorePeriodOrderStatsIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class StorePeriodOrderStatsIT {

    private static final List<String> QUALIFYING = OrderService.QUALIFYING_ORDER_STATUSES;
    private static final String CANCELLED = OrderStatus.CANCELLED.value();

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_store_period_stats_test")
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

    private StorePeriodOrderStatsResult query(long storeId, String start, String end) {
        long startMs = jdbc.queryForObject("SELECT (EXTRACT(EPOCH FROM ?::timestamptz) * 1000)::bigint", Long.class, start);
        long endMs = jdbc.queryForObject("SELECT (EXTRACT(EPOCH FROM ?::timestamptz) * 1000)::bigint", Long.class, end);
        return orderRepository.findStorePeriodOrderStats(storeId, startMs, endMs, QUALIFYING);
    }

    @Test
    @DisplayName("成立訂單口徑：只算八個狀態；待付款、全額退款、取消不算；買家重複只算一人；其他店不算")
    void countsOnlyQualifyingOrdersOfTheStore() {
        order(100, 1, "10", 1000, 0, "2026-09-28 12:00:00+00");
        order(100, 1, "50", 2000, 0, "2026-09-29 12:00:00+00");
        order(100, 2, "60", 3000, 500, "2026-09-30 12:00:00+00");
        order(100, 3, "0", 4000, 0, "2026-09-30 12:00:00+00");
        order(100, 3, "70", 4000, 0, "2026-09-30 12:00:00+00");
        order(100, 4, CANCELLED, 4000, 0, "2026-09-30 12:00:00+00");
        order(200, 5, "50", 9000, 0, "2026-09-30 12:00:00+00");

        StorePeriodOrderStatsResult r = query(100, "2026-09-27 16:00:00+00", "2026-10-03 16:00:00+00");

        assertThat(r.getQualifyingOrders()).isEqualTo(3);
        assertThat(r.getDistinctBuyers()).isEqualTo(2);
        assertThat(r.getRevenue()).isEqualTo(1000 + 2000 + 3000 - 500);
    }

    @Test
    @DisplayName("進行中狀態的部分退款也從營收扣除（refund_total_amount 在 10 到 40 也會累加）")
    void refundOnAnInFlightOrderIsDeducted() {
        order(100, 1, "40", 1000, 300, "2026-09-30 12:00:00+00");

        assertThat(query(100, "2026-09-27 16:00:00+00", "2026-10-03 16:00:00+00").getRevenue()).isEqualTo(700);
    }

    @Test
    @DisplayName("時間窗左閉右開，以付款時間計：剛好在起點算、剛好在終點不算；沒有付款時間不算")
    void windowIsHalfOpenOnPayTime() {
        order(100, 1, "50", 100, 0, "2026-09-27 16:00:00+00");
        order(100, 2, "50", 200, 0, "2026-10-03 15:59:59.999+00");
        order(100, 3, "50", 400, 0, "2026-10-03 16:00:00+00");
        order(100, 4, "50", 800, 0, "2026-09-27 15:59:59.999+00");
        order(100, 5, "10", 1600, 0, null);

        StorePeriodOrderStatsResult r = query(100, "2026-09-27 16:00:00+00", "2026-10-03 16:00:00+00");

        assertThat(r.getQualifyingOrders()).isEqualTo(2);
        assertThat(r.getDistinctBuyers()).isEqualTo(2);
        assertThat(r.getRevenue()).isEqualTo(300);
    }

    @Test
    @DisplayName("窗內沒有成立訂單：回一列全為 0，不是沒有列")
    void emptyWindowIsAllZeros() {
        order(100, 1, "50", 100, 0, "2026-01-01 00:00:00+00");

        StorePeriodOrderStatsResult r = query(100, "2026-09-27 16:00:00+00", "2026-10-03 16:00:00+00");

        assertThat(r.getQualifyingOrders()).isZero();
        assertThat(r.getDistinctBuyers()).isZero();
        assertThat(r.getRevenue()).isZero();
    }

    @Test
    @DisplayName("自買單（buyer_id = store_id）與 ListStoreOrderStats 一致：計入")
    void selfPurchasesAreCountedLikeListStoreOrderStats() {
        order(100, 100, "50", 500, 0, "2026-09-30 12:00:00+00");

        StorePeriodOrderStatsResult r = query(100, "2026-09-27 16:00:00+00", "2026-10-03 16:00:00+00");

        assertThat(r.getQualifyingOrders()).isEqualTo(1);
        assertThat(r.getDistinctBuyers()).isEqualTo(1);
    }

    private void order(long storeId, long buyerId, String status, long actual, long refunded, String payTime) {
        long id = nextId++;
        jdbc.update("""
                INSERT INTO orders (id, order_no, buyer_id, store_id, total_original_amount,
                                    total_actual_amount, refund_total_amount, status, created_at, pay_time)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW() - INTERVAL '30 days', ?::timestamptz)
                """, id, "O" + id, buyerId, storeId, actual, actual, refunded, status, payTime);
    }
}

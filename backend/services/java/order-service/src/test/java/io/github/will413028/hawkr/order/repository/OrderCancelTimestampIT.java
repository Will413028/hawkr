package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.code.OrderCancelByRole;
import io.github.will413028.hawkr.common.utils.code.OrderCancelReason;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.entity.OrderEntity;
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

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 三條取消路徑在**真實 PostgreSQL** 上確實寫下 {@code cancelled_at}（issue #2471）。
 *
 * <p>為什麼要真 DB：{@link OrderCancelTimestampSqlTest} 只看得到 SQL 的形狀，看不到
 * 「V11 這一欄到底存不存在」「MyBatis 有沒有把它映回 {@link OrderEntity}」。這三件事任一沒對上，
 * 前端就是拿不到結案時間，而 SQL 形狀測試會全綠——所以這裡跑 Flyway 全套 migration 再打實際的
 * mapper 方法，從 DDL 到 Entity 一路串起來驗。
 *
 * <p>另外兩個案例守的是 V11 刻意的取捨：**沒有 DEFAULT**（未取消的訂單必須是 NULL，不能被
 * migration 執行時刻填滿）、以及 **CAS 未命中時不得留下時間戳**（時間戳與狀態同句原子落地）。
 */
@SpringBootTest(classes = OrderCancelTimestampIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class OrderCancelTimestampIT {

    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_cancel_timestamp_test")
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
        // application.yml 的 spring.flyway.* 指向本機真實 hawkr_order；不覆寫會讓 migration
        // 跑到那裡去，而 testcontainers 裡的測試 DB 是空的。
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
            MyBatisPlusAutoConfiguration.class  // common-core：TimestampTz TypeHandler + MetaObjectHandler
    })
    @MapperScan("io.github.will413028.hawkr.order.repository")
    static class MinimalConfig {
    }

    @Autowired
    private OrderRepository orderRepository;

    @Autowired
    private JdbcTemplate jdbc;

    @BeforeEach
    void reset() {
        jdbc.update("DELETE FROM orders");
    }

    @Test
    @DisplayName("售後／賣家取消（cancelOrderInternalCas）寫下 cancelled_at")
    void internalCancelStampsCancelledAt() {
        order(1L, "O100000001", OrderStatus.PENDING_SHIPMENT.value());

        int rows = orderRepository.cancelOrderInternalCas(
                "O100000001",
                OrderStatus.CANCELLED.value(),
                OrderCancelReason.SELLER_SUSPENDED.value(),
                "賣家停權",
                OrderCancelByRole.SELLER.value());

        assertThat(rows).isEqualTo(1);
        assertCancelled("O100000001");
    }

    @Test
    @DisplayName("買家自願取消（cancelOrderByBuyerCas）寫下 cancelled_at")
    void buyerCancelStampsCancelledAt() {
        order(2L, "O200000002", OrderStatus.PENDING_PAYMENT.value());

        int rows = orderRepository.cancelOrderByBuyerCas(
                "O200000002", OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value());

        assertThat(rows).isEqualTo(1);
        assertCancelled("O200000002");
    }

    @Test
    @DisplayName("系統逾時／停權批次取消（cancelOrderByPaymentExpireCas）寫下 cancelled_at")
    void systemCancelStampsCancelledAt() {
        order(3L, "O300000003", OrderStatus.PENDING_PAYMENT.value());

        int rows = orderRepository.cancelOrderByPaymentExpireCas(
                "O300000003", OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value(),
                OrderCancelReason.PAYMENT_TIMEOUT.value());

        assertThat(rows).isEqualTo(1);
        assertCancelled("O300000003");
    }

    @Test
    @DisplayName("未取消的訂單 cancelled_at 為 NULL（V11 刻意不設 DEFAULT，既有訂單不得被填時刻）")
    void untouchedOrdersKeepNullCancelledAt() {
        order(4L, "O400000004", OrderStatus.PAID.value());

        assertThat(orderRepository.findByOrderNo("O400000004").getCancelledAt())
                .as("設了 DEFAULT NOW() 的話每一列都會有值，那是憑空捏造的業務時間")
                .isNull();
    }

    @Test
    @DisplayName("CAS 未命中時不寫 cancelled_at（時間戳與狀態同句原子落地）")
    void failedCasLeavesNoTimestamp() {
        order(5L, "O500000005", OrderStatus.PAID.value());

        // 期望舊狀態給 PENDING_PAYMENT，實際是 PAID ⇒ CAS 不命中
        int rows = orderRepository.cancelOrderByBuyerCas(
                "O500000005", OrderStatus.PENDING_PAYMENT.value(), OrderStatus.CANCELLED.value());

        assertThat(rows).isZero();
        OrderEntity order = orderRepository.findByOrderNo("O500000005");
        assertThat(order.getStatus()).isEqualTo(OrderStatus.PAID.value());
        assertThat(order.getCancelledAt())
                .as("狀態沒變卻留下取消時間 ⇒ 時間戳被寫在 CAS 之外")
                .isNull();
    }

    // ==================== helpers ====================

    private void assertCancelled(String orderNo) {
        OrderEntity order = orderRepository.findByOrderNo(orderNo);
        assertThat(order.getStatus()).isEqualTo(OrderStatus.CANCELLED.value());
        assertThat(order.getCancelledAt())
                .as("取消了卻沒有結案時間 ⇒ 訂單時間軸最後一格永遠是空的（不報錯、不轉紅）")
                .isNotNull();

        // 用 DB 自己的時鐘判斷「是不是當下的時間」：epoch／1970 這種假值同樣是非 NULL，
        // 只斷言 isNotNull 擋不住。窗口取寬，測試不因機器慢而閃爍。
        assertThat(jdbc.queryForObject("""
                SELECT cancelled_at BETWEEN NOW() - INTERVAL '5 minutes' AND NOW() + INTERVAL '5 minutes'
                FROM orders WHERE order_no = ?
                """, Boolean.class, orderNo))
                .as("cancelled_at 不在當下附近 ⇒ 寫進去的不是取消發生的時間")
                .isTrue();
    }

    private void order(long id, String orderNo, String status) {
        jdbc.update("""
                INSERT INTO orders (id, order_no, buyer_id, store_id, total_original_amount,
                                    total_actual_amount, status, recipient_name, created_at)
                VALUES (?, ?, ?, ?, 100, 100, ?, '陳小明', NOW())
                """, id, orderNo, BUYER_ID, STORE_ID, status);
    }
}

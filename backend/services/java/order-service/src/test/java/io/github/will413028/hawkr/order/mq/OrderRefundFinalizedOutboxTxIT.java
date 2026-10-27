package io.github.will413028.hawkr.order.mq;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.repository.FinancialLogRepository;
import io.github.will413028.hawkr.order.repository.OrderOutboxMapper;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import org.junit.jupiter.api.BeforeAll;
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
import org.springframework.context.annotation.Bean;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Service;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.transaction.annotation.EnableTransactionManagement;
import org.springframework.transaction.annotation.Transactional;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * {@code order.refund.finalized} 真的落在<b>呼叫端既有的交易裡</b>（58 §6.3.2.3 硬性約束 1）。
 *
 * <p>為什麼要真 DB：Mockito 測得出「有沒有呼叫 {@code outboxMapper.insert}」，測不出
 * 「那一列會不會跟著 rollback 一起消失」。而這正是本約束的全部內容——直接 publish 或
 * {@code REQUIRES_NEW} 都會讓「交易 rollback 但事件已發出」變成可能，
 * 那在退款上等於一張<b>無法撤回</b>的折讓單。
 *
 * <p><b>兩支測試合起來才有鑑別力，缺一不可</b>：
 * <ul>
 *   <li>{@link #commitPath_persistsOutboxRow} —— 改成直接 publish（不寫 outbox）⇒ 轉紅</li>
 *   <li>{@link #rollbackPath_leavesNoOutboxRow} —— 把寫入移出交易（{@code REQUIRES_NEW}／
 *       另開連線／afterCommit 之外的任何逃逸）⇒ 轉紅</li>
 * </ul>
 * 只留前者，「把 outbox 換成 publish 之後再補一行 insert」照樣全綠。
 */
@SpringBootTest(classes = OrderRefundFinalizedOutboxTxIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class OrderRefundFinalizedOutboxTxIT {

    private static final String ORDER_NO = "O9900000001";
    private static final long STORE_ID = 2685914682L;
    private static final long BUYER_ID = 2623900520L;
    private static final long TOTAL_ACTUAL = 1200L;

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_refund_finalized_tx_test")
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
        // application.yml 的 spring.flyway.* 指向本機真實 hawkr_order；不覆寫會跑到那裡去
        registry.add("spring.flyway.url", POSTGRES::getJdbcUrl);
        registry.add("spring.flyway.user", POSTGRES::getUsername);
        registry.add("spring.flyway.password", POSTGRES::getPassword);
        registry.add("spring.flyway.enabled", () -> "true");
        registry.add("spring.flyway.locations", () -> "classpath:db/migration");
        registry.add("spring.flyway.baseline-on-migrate", () -> "false");
    }

    @SpringBootConfiguration
    @EnableTransactionManagement
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

        @Bean
        OrderRefundFinalizedOutboxWriter refundFinalizedOutboxWriter(
                OrderRepository orderRepository,
                FinancialLogRepository financialLogRepository,
                OrderOutboxMapper orderOutboxMapper) {
            return new OrderRefundFinalizedOutboxWriter(
                    orderRepository, financialLogRepository, orderOutboxMapper,
                    new io.github.will413028.hawkr.common.obs.context.ObsContext());
        }

        @Bean
        SellerCancelRefundTxHandler sellerCancelRefundTxHandler(
                OrderRepository orderRepository,
                FinancialLogRepository financialLogRepository,
                OrderRefundFinalizedOutboxWriter writer) {
            return new SellerCancelRefundTxHandler(orderRepository, financialLogRepository, writer);
        }

        @Bean
        FailingCaller failingCaller(SellerCancelRefundTxHandler handler) {
            return new FailingCaller(handler);
        }
    }

    /**
     * 模擬「同一交易的後續步驟炸掉」：REQUIRED 傳播讓 handler 併入本交易，
     * 這裡一拋例外，handler 寫下的每一列都必須消失。
     */
    @Service
    static class FailingCaller {
        private final SellerCancelRefundTxHandler handler;

        FailingCaller(SellerCancelRefundTxHandler handler) {
            this.handler = handler;
        }

        @Transactional
        public void cancelThenBlowUp(String orderNo, String cancelId) {
            handler.handleCancelCompleted(orderNo, cancelId);
            throw new IllegalStateException("模擬交易後段失敗");
        }
    }

    @BeforeAll
    static void initTsid() {
        try {
            TsidUtil.init(300, 0);
        } catch (IllegalStateException ignored) {
            // 已初始化
        }
    }

    @Autowired private SellerCancelRefundTxHandler handler;
    @Autowired private FailingCaller failingCaller;
    @Autowired private JdbcTemplate jdbc;

    @BeforeEach
    void reset() {
        jdbc.update("DELETE FROM order_outbox");
        jdbc.update("DELETE FROM financial_logs");
        jdbc.update("DELETE FROM orders");
        jdbc.update("""
                INSERT INTO orders (id, order_no, buyer_id, store_id, total_original_amount,
                                    total_actual_amount, status, payment_method, recipient_name, created_at)
                VALUES (1, ?, ?, ?, ?, ?, ?, 'CREDIT_CARD', '陳小明', NOW())
                """, ORDER_NO, BUYER_ID, STORE_ID, TOTAL_ACTUAL, TOTAL_ACTUAL,
                OrderStatus.CANCELLED.value());
    }

    @Test
    @DisplayName("commit：order_outbox 真的多一列 order.refund.finalized（改成直接 publish ⇒ 轉紅）")
    void commitPath_persistsOutboxRow() {
        handler.handleCancelCompleted(ORDER_NO, "CXL_IT_1");

        assertThat(outboxCount()).isEqualTo(1);
        assertThat(jdbc.queryForObject(
                "SELECT routing_key FROM order_outbox WHERE aggregate_id = ?", String.class, ORDER_NO))
                .isEqualTo("order.refund.finalized");
        assertThat(jdbc.queryForObject(
                "SELECT payload FROM order_outbox WHERE aggregate_id = ?", String.class, ORDER_NO))
                .contains("\"refund_ref\":\"AS_CANCEL_CXL_IT_1\"")
                .contains("\"refund_source\":\"SELLER_CANCEL\"");
        // E3 同時補上了那筆從來沒被寫過的資金流水
        assertThat(financialLogCount()).isEqualTo(1);
    }

    @Test
    @DisplayName("rollback：order_outbox 一列都沒有（寫入逃出交易 ⇒ 轉紅）")
    void rollbackPath_leavesNoOutboxRow() {
        assertThatThrownBy(() -> failingCaller.cancelThenBlowUp(ORDER_NO, "CXL_IT_2"))
                .isInstanceOf(IllegalStateException.class);

        assertThat(outboxCount())
                .as("交易 rollback 了卻留下 outbox 列 ⇒ 事件會被派送 ⇒ 一張退不掉的折讓單")
                .isZero();
        assertThat(financialLogCount()).isZero();
    }

    @Test
    @DisplayName("rollback 之後重試，仍然恰好一列（不會因為前一次逃逸而變兩列）")
    void retryAfterRollback_yieldsExactlyOneRow() {
        assertThatThrownBy(() -> failingCaller.cancelThenBlowUp(ORDER_NO, "CXL_IT_3"))
                .isInstanceOf(IllegalStateException.class);
        handler.handleCancelCompleted(ORDER_NO, "CXL_IT_3");

        assertThat(outboxCount()).isEqualTo(1);
    }

    private int outboxCount() {
        return jdbc.queryForObject(
                "SELECT COUNT(*) FROM order_outbox WHERE routing_key = 'order.refund.finalized'",
                Integer.class);
    }

    private int financialLogCount() {
        return jdbc.queryForObject(
                "SELECT COUNT(*) FROM financial_logs WHERE source_ref_no = ?", Integer.class, ORDER_NO);
    }
}

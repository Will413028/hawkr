package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.order.TestIds;
import io.github.will413028.hawkr.order.entity.OrderItemEntity;
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
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;

/** UUID 型的 sku_id / product_id 經 MyBatis 寫入、讀回，以及 {@code <foreach>} 展開 List&lt;UUID&gt;。 */
@SpringBootTest(classes = OrderItemUuidMappingIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class OrderItemUuidMappingIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_item_uuid_test")
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

    private static final long BUYER_ID = 2623900520L;

    @Autowired
    private OrderRepository orderRepository;

    @Autowired
    private OrderItemRepository orderItemRepository;

    @Autowired
    private JdbcTemplate jdbc;

    @BeforeEach
    void reset() {
        jdbc.update("DELETE FROM order_items");
        jdbc.update("DELETE FROM orders");
    }

    @Test
    @DisplayName("order_items 的 UUID product_id / sku_id 經 mapper 寫入後讀回同一個值")
    void orderItemUuidIdsRoundTrip() {
        order(1L, "O100000001", "0");
        OrderItemEntity item = item(11L, "O100000001", TestIds.id(1), TestIds.id(2));

        orderItemRepository.insert(item);

        OrderItemEntity read = orderItemRepository.findById(11L);
        assertThat(read.getProductId()).isEqualTo(TestIds.id(1));
        assertThat(read.getSkuId()).isEqualTo(TestIds.id(2));
    }

    @Test
    @DisplayName("non-catalog 明細的 NULL product_id / sku_id 讀回 null")
    void nullIdsRoundTrip() {
        order(2L, "O200000002", "0");
        orderItemRepository.insert(item(12L, "O200000002", null, null));

        OrderItemEntity read = orderItemRepository.findById(12L);
        assertThat(read.getProductId()).isNull();
        assertThat(read.getSkuId()).isNull();
    }

    @Test
    @DisplayName("findLatestPendingPaymentByBuyerAndSkus：<foreach> 展開 List<UUID> 並命中含該 SKU 的待付款訂單")
    void findLatestPendingPaymentByBuyerAndSkusExpandsUuidList() {
        order(3L, "O300000003", "0");
        orderItemRepository.insert(item(13L, "O300000003", TestIds.id(1), TestIds.id(3)));

        assertThat(orderRepository.findLatestPendingPaymentByBuyerAndSkus(
                BUYER_ID, List.of(TestIds.id(9), TestIds.id(3)))).isNotNull()
                .extracting(o -> o.getOrderNo()).isEqualTo("O300000003");
        assertThat(orderRepository.findLatestPendingPaymentByBuyerAndSkus(
                BUYER_ID, List.of(TestIds.id(9)))).isNull();
    }

    private OrderItemEntity item(long id, String orderNo, UUID productId, UUID skuId) {
        OrderItemEntity e = new OrderItemEntity();
        e.setId(id);
        e.setOrderNo(orderNo);
        e.setProductId(productId);
        e.setProductName("文旦柚");
        e.setSkuId(skuId);
        e.setSkuName("文旦柚 200 公斤");
        e.setQuantity(1);
        e.setListPrice(100L);
        e.setUnitPrice(100L);
        e.setShareDiscount(0L);
        e.setActualPayAmount(100L);
        return e;
    }

    private void order(long id, String orderNo, String status) {
        jdbc.update("""
                INSERT INTO orders (id, order_no, buyer_id, store_id, total_original_amount,
                                    total_actual_amount, status, recipient_name, created_at)
                VALUES (?, ?, ?, ?, 100, 100, ?, '陳小明', NOW())
                """, id, orderNo, BUYER_ID, 2685914682L, status);
    }
}

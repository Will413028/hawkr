package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.order.checkout.OrderPolicyResolver;
import io.github.will413028.hawkr.order.checkout.OrderPolicySnapshot;
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

import java.time.LocalDateTime;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 訂單政策五欄的<b>可執行契約</b>：resolver 算出來的值必須逐欄等於最後那一列 DB row。
 *
 * <h2>為什麼這支測試必須用真 DB</h2>
 * 這個 bug 的失效機制<b>只存在於 DB 層</b>：MyBatis-Plus 的 {@code insertStrategy=NOT_NULL}
 * 讓沒有被 setter 設過的欄位<b>根本不出現在 INSERT 語句裡</b>，於是 PostgreSQL 套用
 * {@code DEFAULT 'SELLER_MANAGED'} / {@code DEFAULT 'v0-legacy'}。
 * 結果是：Java 端沒有例外、沒有警告、entity 上那六個欄位是 null 而 DB 上是「看起來合理的值」。
 * <b>任何 mock 或 SQL 形狀測試對這個失效模式都是全綠的</b>——V14 部署後 dev 上
 * 每一筆新訂單都寫錯，而 211 支既有測試沒有一支變紅。
 *
 * <h2>鑑別力（M1 變異）</h2>
 * 拿掉 {@code OrderCreationTxHandler.applyPolicySnapshot} 的任何一行，
 * 對應那一欄就會落回 DB DEFAULT，本檔對應斷言轉紅。
 * 🔴 斷言的期望值<b>不從被測物取得</b>：{@code fulfillment_mode} 的期望值是硬寫的字面字串
 * {@code "PLATFORM_LOGISTICS"}，不是 {@code resolver.resolve(...).fulfillmentMode()}
 * ——後者會跟著被測物一起變，把 resolver 改壞也照樣綠。
 */
@SpringBootTest(classes = OrderPolicySnapshotPersistenceIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class OrderPolicySnapshotPersistenceIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_policy_snapshot_test")
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

    private final OrderPolicyResolver resolver = new OrderPolicyResolver();

    @BeforeEach
    void reset() {
        jdbc.update("DELETE FROM orders WHERE order_no LIKE 'ITPOL%'");
    }

    // ==================== 契約 ====================

    @Test
    @DisplayName("3001 黑貓：五欄逐欄等於 resolver 的輸出，且 fulfillment_mode 不是 DB DEFAULT")
    void taqbinOrderPersistsResolvedPolicy() {
        assertPersisted(ShippingMethod.TAQBIN.value(), "ITPOL0001", "PLATFORM_LOGISTICS");
    }

    @Test
    @DisplayName("3002 超商：同上——這兩條是本波唯一啟用的配送方式，也是 V14 回填的那 1093 列")
    void cvsOrderPersistsResolvedPolicy() {
        assertPersisted(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(), "ITPOL0002", "PLATFORM_LOGISTICS");
    }

    @Test
    @DisplayName("3005 商家自送：fulfillment_mode = SELLER_MANAGED（與 DB DEFAULT 同值，但來源是 resolver 不是 DEFAULT）")
    void merchantDeliveryPersistsSellerManaged() {
        // ⚠️ 這一格的值恰好等於 DB DEFAULT ⇒ 它單獨無鑑別力，
        //    真正的鑑別力來自上面兩支（值與 DEFAULT 不同）。列在這裡是為了覆蓋
        //    chk_orders_fulfillment_mode_consistent：3005 若被寫成 PLATFORM_LOGISTICS，DB 會擋。
        assertPersisted(ShippingMethod.MERCHANT_DELIVERY.value(), "ITPOL0005", "SELLER_MANAGED");
    }

    @Test
    @DisplayName("沒有人寫政策欄時 DB 會落 DEFAULT——這是本測試要防的失效本身，先證明它真的會發生")
    void withoutExplicitWriteTheRowFallsBackToDbDefault() {
        // 🔴 這支是「量測有鑑別力」的對照組：它刻意重現修復前的行為。
        //    若哪天 DB DEFAULT 被移除或改掉，這支會先紅，提醒上面三支的前提變了。
        OrderEntity order = baseOrder("ITPOL9999", ShippingMethod.TAQBIN.value());
        orderRepository.insert(order);

        Map<String, Object> row = readPolicy("ITPOL9999");
        assertThat(row.get("fulfillment_mode")).isEqualTo("SELLER_MANAGED");
        assertThat(row.get("policy_version")).isEqualTo("v0-legacy");
    }

    // ==================== helpers ====================

    private void assertPersisted(String shippingMethod, String orderNo, String expectedFulfillmentMode) {
        OrderPolicySnapshot policy = resolver.resolve(shippingMethod);

        OrderEntity order = baseOrder(orderNo, shippingMethod);
        // 與 OrderCreationTxHandler.applyPolicySnapshot 同一組 setter。
        order.setCommerceModel(policy.commerceModel());
        order.setCollectionModel(policy.collectionModel());
        order.setFulfillmentMode(policy.fulfillmentMode());
        order.setInvoiceModel(policy.invoiceModel());
        order.setPolicyVersion(policy.policyVersion());
        orderRepository.insert(order);

        Map<String, Object> row = readPolicy(orderNo);

        // 期望值一律硬寫字面值，不從 resolver 取——否則 resolver 改壞了測試也不會紅。
        assertThat(row.get("commerce_model")).isEqualTo("SELLER_DIRECT");
        assertThat(row.get("collection_model")).isEqualTo("SELLER_MERCHANT");
        assertThat(row.get("fulfillment_mode")).isEqualTo(expectedFulfillmentMode);
        assertThat(row.get("invoice_model")).isEqualTo("SELLER_TO_BUYER");
        assertThat(row.get("policy_version")).isEqualTo("v1-foundation");

        // 第二層：DB row 與 resolver 的輸出**逐欄相同**（BLOCKER 1 的驗收條件原話）。
        assertThat(row.get("commerce_model")).isEqualTo(policy.commerceModel());
        assertThat(row.get("collection_model")).isEqualTo(policy.collectionModel());
        assertThat(row.get("fulfillment_mode")).isEqualTo(policy.fulfillmentMode());
        assertThat(row.get("invoice_model")).isEqualTo(policy.invoiceModel());
        assertThat(row.get("policy_version")).isEqualTo(policy.policyVersion());
    }

    private Map<String, Object> readPolicy(String orderNo) {
        return jdbc.queryForMap("""
                SELECT commerce_model, collection_model, fulfillment_mode,
                       invoice_model, policy_version
                  FROM orders WHERE order_no = ?
                """, orderNo);
    }

    private static OrderEntity baseOrder(String orderNo, String shippingMethod) {
        OrderEntity order = new OrderEntity();
        order.setId(Math.abs(orderNo.hashCode()) + 1_000_000L);
        order.setOrderNo(orderNo);
        order.setBuyerId(2623900520L);
        order.setStoreId(2675237784L);
        order.setTotalOriginalAmount(1000L);
        order.setTotalActualAmount(1060L);
        order.setStatus("3001");
        order.setDeliveryTemp("3201");
        order.setShippingMethod(shippingMethod);
        order.setShippingFee(60L);
        order.setShippingFeePayer("BUYER");
        order.setPaymentDeadline(LocalDateTime.now().plusDays(3));
        order.setPaymentAttempts(0);
        return order;
    }
}

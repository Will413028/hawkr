package io.github.will413028.hawkr.order.repository;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.autoconfigure.ImportAutoConfiguration;
import org.springframework.boot.autoconfigure.flyway.FlywayAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceTransactionManagerAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.JdbcTemplateAutoConfiguration;
import org.springframework.boot.autoconfigure.sql.init.SqlInitializationAutoConfiguration;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * non-catalog line（Wave A）的 <b>schema 層可執行契約</b>：V17 放寬了什麼、
 * 以及<b>刻意沒有放寬什麼</b>。
 *
 * <p>🔴 為什麼要用真 DB 而不是單元測試：這四條全是 Postgres 的約束語意
 * （NOT NULL、UNIQUE 對 NULL 的處理），mock 不出來；而 A-7 的整個推論
 *  「Postgres 視 NULL 相異 ⇒ UNIQUE 與索引都不用動」只有真 DB 驗得了。
 */
@Testcontainers
@SpringBootTest(classes = NonCatalogOrderItemConstraintIT.MinimalConfig.class)
@Tag("integration")
class NonCatalogOrderItemConstraintIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("non_catalog_order_item_test")
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
            FlywayAutoConfiguration.class
    })
    static class MinimalConfig {
    }

    @Autowired
    private JdbcTemplate jdbc;

    private static final String INSERT = """
            INSERT INTO order_items
              (id, order_no, sku_id, sku_name, quantity, list_price, unit_price, actual_pay_amount)
            VALUES (?, ?, ?, ?, 1, 30000, 30000, 30000)
            """;

    @Test
    @DisplayName("V17 生效：sku_id 可為 NULL（non-catalog line 的整個前提）")
    void skuIdAcceptsNull() {
        assertThatCode(() -> jdbc.update(INSERT, 9001L, "O_NC_001", null, "文旦柚 200 公斤"))
                .doesNotThrowAnyException();

        Map<String, Object> row = jdbc.queryForMap(
                "SELECT sku_id, sku_name FROM order_items WHERE id = 9001");
        assertThat(row.get("sku_id")).isNull();
        // ⚠️ 品名必須原樣存在——它才是 non-catalog line 的識別依據。
        assertThat(row.get("sku_name")).isEqualTo("文旦柚 200 公斤");
    }

    @Test
    @DisplayName("⛔ sku_name 仍是 NOT NULL：放寬的只有 sku_id，品名不得缺（doc 64 §6.4.9）")
    void skuNameStillNotNull() {
        assertThatThrownBy(() -> jdbc.update(INSERT, 9002L, "O_NC_002", null, null))
                .isInstanceOf(DataIntegrityViolationException.class);
    }

    @Test
    @DisplayName("A-7 前半：同一張訂單的兩筆 NULL sku_id 不撞 UNIQUE（Postgres 視 NULL 相異）")
    void twoNullSkuIdsDoNotCollide() {
        jdbc.update(INSERT, 9003L, "O_NC_003", null, "文旦柚 200 公斤");
        assertThatCode(() -> jdbc.update(INSERT, 9004L, "O_NC_003", null, "柳丁 100 公斤"))
                .doesNotThrowAnyException();
    }

    @Test
    @DisplayName("A-7 後半：catalog 訂單的 uk_order_items_order_sku 原樣成立，沒被放寬波及")
    void sameSkuIdInSameOrderStillCollides() {
        jdbc.update(INSERT, 9005L, "O_NC_004", 5150L, "文旦柚 200 公斤");
        assertThatThrownBy(() -> jdbc.update(INSERT, 9006L, "O_NC_004", 5150L, "文旦柚 200 公斤"))
                .isInstanceOf(DataIntegrityViolationException.class);
    }
}

package io.github.will413028.hawkr.product.integration;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import org.junit.jupiter.api.Tag;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.autoconfigure.ImportAutoConfiguration;
import org.springframework.boot.autoconfigure.flyway.FlywayAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceTransactionManagerAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.JdbcTemplateAutoConfiguration;
import org.springframework.boot.autoconfigure.sql.init.SqlInitializationAutoConfiguration;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;

/**
 * product-service 在真實 PostgreSQL 上的整合測試基底：Flyway 套用全部 migration，
 * 掛上 repository 的 MyBatis mapper，不啟動 MQ、gRPC、Redis。
 */
@SpringBootTest(classes = AbstractProductPostgresIT.MinimalConfig.class)
@Tag("integration")
abstract class AbstractProductPostgresIT {

    /**
     * 所有 IT 共用一個容器：Spring 會依 DynamicPropertySource 快取 context，容器若每個類別重啟，
     * 後面的類別會拿到指向已關閉容器的 context。容器由 Testcontainers 的 Ryuk 在 JVM 結束時回收。
     */
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withInitScript("product-it/extensions.sql")   // 正式環境由 DB 初始化於 Flyway 之前安裝 pg_trgm
            .withDatabaseName("product_it")
            .withUsername("test")
            .withPassword("test")
            .withEnv("TZ", "UTC")
            .withEnv("PGTZ", "UTC");

    static {
        POSTGRES.start();
    }

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
    @MapperScan("io.github.will413028.hawkr.product.repository")
    static class MinimalConfig {
    }
}

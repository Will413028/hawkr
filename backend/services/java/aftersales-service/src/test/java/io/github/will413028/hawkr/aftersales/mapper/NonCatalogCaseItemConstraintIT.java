package io.github.will413028.hawkr.aftersales.mapper;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseItemEntity;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.autoconfigure.EnableAutoConfiguration;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.annotation.Import;
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
 * non-catalog line 的 aftersales 半邊（V10）：<b>放寬了什麼、以及刻意沒放寬什麼</b>。
 *
 * <p>🔴 本檔是 {@code aftersale_case_items} 的<b>第一個</b> IT 守門面——在此之前該表零 IT 覆蓋
 * （{@code AfterSaleCaseMapperIT} 460 行但完全不碰 case items）。
 *
 * <p>⚠️ 表名是 {@code aftersale_case_items}（<b>無中間底線</b>）。doc 64 §6.4.9 與 :476 的 SQL
 * 逐字寫的是 {@code after_sale_case_items}，那是錯的——照抄會 relation does not exist。
 */
@Testcontainers
@SpringBootTest(classes = NonCatalogCaseItemConstraintIT.TestBoot.class)
@Tag("integration")
class NonCatalogCaseItemConstraintIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:15-alpine")
            .withDatabaseName("aftersales_non_catalog_test")
            .withUsername("test")
            .withPassword("test");

    @DynamicPropertySource
    static void registerProps(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
        registry.add("spring.datasource.driver-class-name", () -> "org.postgresql.Driver");
        // 以下四組與 AfterSaleCaseMapperIT 同因：bootstrap 需要 DB_NAME、application.yml 的
        // flyway 指向本機真實 DB、TSID 與 gRPC port 會撞本機 dev container、測試不需服務註冊。
        registry.add("hawkr.db.bootstrap.enabled", () -> "false");
        registry.add("spring.flyway.enabled", () -> "false");
        registry.add("hawkr.tsid.enabled", () -> "false");
        // -1 關掉對外 gRPC server（本測試用不到）。0 是「先找空 port 再綁」，
        // 中間可能被 testcontainers 的 port 映射搶走（CI 實測 Address already in use）。
        registry.add("grpc.server.port", () -> "-1");
        registry.add("spring.cloud.nacos.discovery.enabled", () -> "false");
        registry.add("spring.cloud.discovery.enabled", () -> "false");
        registry.add("spring.cloud.service-registry.auto-registration.enabled", () -> "false");
    }

    @BeforeAll
    static void runFlywayMigration() {
        Flyway.configure()
                .dataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword())
                .locations("classpath:db/migration")
                .load()
                .migrate();
    }

    @SpringBootConfiguration
    @EnableAutoConfiguration
    @Import(MyBatisPlusAutoConfiguration.class)
    @MapperScan("io.github.will413028.hawkr.aftersales.mapper")
    static class TestBoot {}

    @Autowired private AfterSaleCaseItemMapper itemMapper;
    @Autowired private JdbcTemplate jdbc;

    private static final String INSERT = """
            INSERT INTO aftersale_case_items
              (id, case_no, order_item_id, sku_id, sku_name, quantity, original_price, actual_pay_amount)
            VALUES (?, ?, 7001, ?, ?, 1, 30000, 30000)
            """;

    @Test
    @DisplayName("V10 生效：sku_id 可為 NULL（non-catalog 訂單開得了售後單的前提）")
    void skuIdAcceptsNull() {
        assertThatCode(() -> jdbc.update(INSERT, 9101L, "AS_NC_001", null, "文旦柚 200 公斤"))
                .doesNotThrowAnyException();

        Map<String, Object> row = jdbc.queryForMap(
                "SELECT sku_id, sku_name FROM aftersale_case_items WHERE id = 9101");
        assertThat(row.get("sku_id")).isNull();
        assertThat(row.get("sku_name")).isEqualTo("文旦柚 200 公斤");
    }

    @Test
    @DisplayName("⛔ sku_name 仍是 NOT NULL：放寬的只有 sku_id，品名不得缺")
    void skuNameStillNotNull() {
        assertThatThrownBy(() -> jdbc.update(INSERT, 9102L, "AS_NC_002", null, null))
                .isInstanceOf(DataIntegrityViolationException.class);
    }

    /**
     * A-4 的真驗收：建案時 {@code OrderItemForAfterSales.sku_id} 是<b>空字串</b>
     * （order-service 對 NULL 走 {@code formatLong} 輸出 ""），這一串必須不拋錯且落成 NULL。
     *
     * <p>逐字重現 {@code AfterSaleCaseService} 那一行的轉換 ＋ 真的經 MyBatis 寫進 DB，
     * 兩層一起驗：只驗 parse 不驗寫入的話，entity 型別若是 primitive 仍會炸而測試看不到。
     */
    @Test
    @DisplayName("建案路徑：空字串 sku_id 不拋錯，且經 MyBatis 落成 NULL")
    void blankSkuIdFromOrderServiceBecomesNull() {
        AfterSaleCaseItemEntity item = new AfterSaleCaseItemEntity();
        item.setId(9103L);
        item.setCaseNo("AS_NC_003");
        item.setOrderItemId(7001L);
        item.setSkuId(ProtoConversionUtils.parseLongOrNull(""));   // ← 與產線同一支 helper
        item.setSkuName("文旦柚 200 公斤");
        item.setQuantity(1);
        item.setOriginalPrice(30000L);
        item.setActualPayAmount(30000L);

        assertThatCode(() -> itemMapper.insert(item)).doesNotThrowAnyException();

        assertThat(jdbc.queryForMap(
                "SELECT sku_id FROM aftersale_case_items WHERE id = 9103").get("sku_id")).isNull();
    }

    @Test
    @DisplayName("catalog 明細不受影響：有 SKU 就原樣寫入")
    void catalogSkuIdStillPersisted() {
        jdbc.update(INSERT, 9104L, "AS_NC_004", 5150L, "文旦柚 200 公斤");
        assertThat(jdbc.queryForMap(
                "SELECT sku_id FROM aftersale_case_items WHERE id = 9104").get("sku_id"))
                .isEqualTo(5150L);
    }
}

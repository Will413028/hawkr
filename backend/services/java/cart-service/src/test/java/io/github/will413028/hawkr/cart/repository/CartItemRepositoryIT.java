package io.github.will413028.hawkr.cart.repository;

import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.autoconfigure.EnableAutoConfiguration;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Task 7（group-b-storeid-bugfixes）：CartItemRepository 整合測試（TestContainers PostgreSQL + Flyway）。
 *
 * <p>Task 2 修好 {@code deleteSelfOwnedGuestItemsInternal} 的 MyBatis {@code <foreach>} IN 子句 SQL
 * （自買過濾：MergeCart 時刪除訪客車中由已解析 storeIds 自己上架的 NORMAL 商品），但整條分支審查發現
 * cart-service 模組完全沒有 repository 層真資料庫測試 —— 之前只有 Mockito-mocked 單元測試，看不到
 * 真實 SQL 在 PostgreSQL 上的 affected rows / IN 子句展開行為。本檔補這個缺口，鏡射
 * {@code aftersales-service}（AfterSaleCaseMapperIT）已建立的 testcontainers + Flyway 慣例。
 *
 * <p>Image 選用 postgres:16-alpine（非 aftersales 前例用的 15-alpine）：僅 16-alpine 有鏡像到 ghcr
 * 供 self-hosted CI runner 拉取（見 .github/workflows/mirror-testcontainers-images.yml），匿名拉
 * Docker Hub 其他 tag 在此 repo 的 runner 上會跨境逾時。
 *
 * <p>覆蓋：
 * <ul>
 *   <li>deleteSelfOwnedGuestItems：只刪「同 guestId + storeId 屬於給定清單 + status=NORMAL」的列，
 *       不同 store / LOCKED / 不同 guestId 的列必須存活</li>
 *   <li>deleteSelfOwnedGuestItems：storeIds 為空 list 時是 no-op（repository 層 default method 在
 *       進 SQL 前短路，不會送出格式錯誤的空 {@code IN ()}）</li>
 * </ul>
 */
@Testcontainers
@SpringBootTest(classes = CartItemRepositoryIT.TestBoot.class)
@Tag("integration")
class CartItemRepositoryIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("cart_test")
            .withUsername("test")
            .withPassword("test");

    @DynamicPropertySource
    static void registerProps(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
        registry.add("spring.datasource.driver-class-name", () -> "org.postgresql.Driver");
        // common-core DatabaseBootstrapAutoConfiguration 預設 enabled=true 且
        // hawkr.db.bootstrap.target-database=${DB_NAME}（無預設值，本機/CI 未
        // export DB_NAME 時直接 IllegalStateException）。本測試用 testcontainers 全新
        // container，DB 已存在、不需要「確保 DB 存在」的 bootstrap 邏輯，關掉即可。
        registry.add("hawkr.db.bootstrap.enabled", () -> "false");
        // application.yml 的 spring.flyway.url/user/password 是獨立於 spring.datasource.*
        // 的硬編碼預設值（指向本機真實 hawkr_cart DB）。本測試已在 @BeforeAll 手動
        // 對 testcontainers 跑過一次 migration，這裡索性關掉 Spring 自己的 FlywayAutoConfiguration，
        // 避免它改用真實本機 DB 的帳密連線（會因密碼不符而噴 BeanCreationException）。
        registry.add("spring.flyway.enabled", () -> "false");
        registry.add("hawkr.tsid.enabled", () -> "false");
        // gRPC server port=0 = 隨機 port，避免與本機 dev container（9927）衝突
        // -1 關掉對外 gRPC server（本測試用不到）。0 是「先找空 port 再綁」，
        // 中間可能被 testcontainers 的 port 映射搶走（CI 實測 Address already in use）。
        registry.add("grpc.server.port", () -> "-1");
        // 跳過 Nacos / Spring Cloud（測試不需要服務註冊）
        registry.add("spring.cloud.nacos.discovery.enabled", () -> "false");
        registry.add("spring.cloud.discovery.enabled", () -> "false");
        registry.add("spring.cloud.service-registry.auto-registration.enabled", () -> "false");
    }

    /**
     * Flyway 手動執行（在 Spring 啟動之前）：application.yml 的 ${FLYWAY_ENABLED:true}
     * 預設為 true 且指向本機真實 DB，DynamicPropertySource 對其覆蓋在 Spring Boot 3.2 + Spring Cloud
     * 屬性解析時有時序問題（aftersales-service 前例已 observed locally），所以乾脆繞過 Spring 的
     * Flyway 整合，直接在容器啟動後跑一次 migration。
     */
    @BeforeAll
    static void runFlywayMigration() {
        Flyway.configure()
                .dataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword())
                .locations("classpath:db/migration")
                .load()
                .migrate();
    }

    /**
     * 最小化測試 boot：只啟用 DataSource + MyBatis-Plus，Flyway 已在 @BeforeAll 跑完。
     */
    @SpringBootConfiguration
    @EnableAutoConfiguration
    @Import(MyBatisPlusAutoConfiguration.class)
    @MapperScan("io.github.will413028.hawkr.cart.repository")
    static class TestBoot {}

    @Autowired private CartItemRepository repository;
    @Autowired private JdbcTemplate jdbc;

    @BeforeEach
    void cleanCartItems() {
        jdbc.execute("TRUNCATE TABLE cart_items RESTART IDENTITY CASCADE");
    }

    // ==================== deleteSelfOwnedGuestItems（MergeCart 自買過濾，§5.3）====================

    @Test
    @DisplayName("deleteSelfOwnedGuestItems: deletes only rows matching guestId + one of the given storeIds + NORMAL status")
    void deleteSelfOwnedGuestItems_deletesOnlyMatchingRows() {
        insertCartItem(1001L, "guest-A", 9001L, "0"); // 自買 store 9001, NORMAL -> 應刪除
        insertCartItem(1002L, "guest-A", 9002L, "0"); // 自買 store 9002, NORMAL -> 應刪除
        insertCartItem(1003L, "guest-A", 7777L, "0"); // 非自買（不同 store）-> 必須存活
        insertCartItem(1004L, "guest-A", 9001L, "1"); // 自買但 LOCKED -> 必須存活
        insertCartItem(1005L, "guest-B", 9001L, "0"); // 不同 guestId -> 必須存活

        int affected = repository.deleteSelfOwnedGuestItems("guest-A", List.of(9001L, 9002L));

        assertEquals(2, affected);
        assertEquals(0, countById(1001L), "item 1001 (self-owned store 9001, NORMAL) should be deleted");
        assertEquals(0, countById(1002L), "item 1002 (self-owned store 9002, NORMAL) should be deleted");
        assertEquals(1, countById(1003L), "item 1003 (different store) must survive");
        assertEquals(1, countById(1004L), "item 1004 (LOCKED) must survive");
        assertEquals(1, countById(1005L), "item 1005 (different guestId) must survive");
        assertEquals(3, jdbc.queryForObject("SELECT COUNT(*) FROM cart_items", Integer.class),
                "items 1003/1004/1005 must survive");
    }

    @Test
    @DisplayName("deleteSelfOwnedGuestItems: empty storeIds list is a no-op, never sends a malformed empty IN ()")
    void deleteSelfOwnedGuestItems_emptyStoreIds_isNoOp() {
        insertCartItem(2001L, "guest-A", 9001L, "0");

        int affected = repository.deleteSelfOwnedGuestItems("guest-A", List.of());

        assertEquals(0, affected);
        assertEquals(1, jdbc.queryForObject("SELECT COUNT(*) FROM cart_items", Integer.class));
    }

    // ==================== helpers ====================

    /**
     * 直接 SQL insert（避免走 MyBatis-Plus auto-fill，方便精確控制 status / store_id）。
     * 對應 V1（+ V2 rename）schema 的 NOT NULL 欄位給預設值；sku_id 用 id 本身頂替，確保同一
     * guest_id 底下每列 sku_id 相異（uk_cart_guest_sku 部分唯一索引：guest_id, sku_id）。
     */
    private void insertCartItem(long id, String guestId, long storeId, String status) {
        jdbc.update("""
                INSERT INTO cart_items
                    (id, guest_id, store_id, sku_id, quantity,
                     added_unit_price, added_list_price, max_limit_snapshot, sku_name_snapshot, status)
                VALUES (?, ?, ?, ?, 1, 100, 100, 10, 'test sku', ?)
                """,
                id, guestId, storeId, id, status);
    }

    private int countById(long id) {
        Integer count = jdbc.queryForObject("SELECT COUNT(*) FROM cart_items WHERE id = ?", Integer.class, id);
        return count == null ? 0 : count;
    }
}

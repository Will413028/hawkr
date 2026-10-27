package io.github.will413028.hawkr.coupon.migration;

import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Types;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * V6 migration（issuer 身份欄位拆分，expand 階段限定）驗證：migration 能乾淨套用、
 * 型別匹配的 issuer_account_id / issuer_store_id 寫入正常成立、既有髒資料的 backfill
 * 正確搬值。
 *
 * <p>本檔刻意*不*驗證「issuer_type 與非 null 欄位不匹配」會被拒絕——V6 這次只做
 * expand（加 nullable 欄位 + 放寬 NOT NULL + 回填），CHECK constraint
 * （{@code chk_tpl_issuer_identity}）依 docs/開發規範/DB Migration 相容性規範
 * （expand-contract）.md §4 延後到 contract release 才加，此檔尚未真的用 DB 層擋
 * 互斥違規。等 contract release 補上 CHECK constraint 時，這裡要補回對應的
 * reject-path 測試。
 *
 * docs/superpowers/specs/2026-08-06-coupon-issuer-identity-schema-split-design.md §3
 */
@Tag("integration")
@Testcontainers
class IssuerIdentityMigrationIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("coupon_migration_test")
            .withUsername("test")
            .withPassword("test");

    private static Connection connection;

    @BeforeAll
    static void migrateAndConnect() throws SQLException {
        Flyway.configure()
                .dataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword())
                .locations("classpath:db/migration")
                .load()
                .migrate();
        connection = DriverManager.getConnection(
                POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
    }

    @AfterAll
    static void closeConnection() throws SQLException {
        if (connection != null) {
            connection.close();
        }
    }

    private void insertTemplate(long templateId, String issuerType, Long issuerAccountId, Long issuerStoreId)
            throws SQLException {
        String sql = """
            INSERT INTO coupon_templates
              (template_id, title, type, issuer_type, issuer_account_id, issuer_store_id,
               threshold_amount, target_type, start_at, end_at)
            VALUES (?, 'test', '1', ?, ?, ?, 0, '1', NOW(), NOW() + INTERVAL '1 day')
            """;
        try (PreparedStatement ps = connection.prepareStatement(sql)) {
            ps.setLong(1, templateId);
            ps.setString(2, issuerType);
            if (issuerAccountId != null) {
                ps.setLong(3, issuerAccountId);
            } else {
                ps.setNull(3, Types.BIGINT);
            }
            if (issuerStoreId != null) {
                ps.setLong(4, issuerStoreId);
            } else {
                ps.setNull(4, Types.BIGINT);
            }
            ps.executeUpdate();
        }
    }

    @Test
    void adminRow_withIssuerAccountId_isAccepted() {
        assertDoesNotThrow(() -> insertTemplate(1001L, "1", 777L, null));
    }

    @Test
    void sellerRow_withIssuerStoreId_isAccepted() {
        assertDoesNotThrow(() -> insertTemplate(1002L, "2", null, 888L));
    }

    /**
     * 上面所有測試都共用 {@link #POSTGRES}，該容器在 {@link #migrateAndConnect()} 就已經
     * 從空 DB 一路跑完 V1~V6，所以 V6 的 backfill UPDATE 永遠是對 0 筆資料執行——從未真正
     * 驗證過它在既有髒資料（V1~V5 舊 schema 下 issuer_store_id 身兼兩種語意）上的行為，而
     * 那正是這支 UPDATE 存在的理由（真實 dev/prod migration 執行時的場景）。
     *
     * 這裡用另一個獨立的 Testcontainers Postgres 容器（同樣是 Testcontainers-only 驗證，
     * 不是對共用 DB 手動下 migration）：先只 migrate 到 V5（拆分前、`issuer_store_id`
     * 仍是 NOT NULL 的混合語意欄位），用 JDBC 直接插入模擬「舊資料」的兩列，再繼續
     * migrate 到底套用 V6，驗證 backfill 真的把 ADMIN 列的值搬到新欄位、SELLER 列則
     * 原封不動。
     */
    @Test
    void v6Backfill_migratesPreExistingDirtyDataFromV5Schema() throws SQLException {
        try (PostgreSQLContainer<?> backfillContainer = new PostgreSQLContainer<>("postgres:16-alpine")
                .withDatabaseName("coupon_migration_backfill_test")
                .withUsername("test")
                .withPassword("test")) {
            backfillContainer.start();
            String jdbcUrl = backfillContainer.getJdbcUrl();
            String username = backfillContainer.getUsername();
            String password = backfillContainer.getPassword();

            // Step 1：只 migrate 到 V5（拆分前的最後一版），此時 coupon_templates 只有
            // 單一 issuer_store_id BIGINT NOT NULL 欄位，尚無 issuer_account_id。
            Flyway.configure()
                    .dataSource(jdbcUrl, username, password)
                    .locations("classpath:db/migration")
                    .target("5")
                    .load()
                    .migrate();

            // Step 2：用 JDBC 直接插入「V1~V5 舊 schema 下」的既有資料，模擬 V3 rename
            // 之後仍在線上、尚未經過本次拆分的真實模板列：
            //   - ADMIN 列：issuer_store_id 存的是管理員 account_id（舊混合語意）。
            //   - SELLER 列：issuer_store_id 存的本來就是 store_id（語意本來就對）。
            try (Connection conn = DriverManager.getConnection(jdbcUrl, username, password)) {
                String insertOldSchemaSql = """
                    INSERT INTO coupon_templates
                      (template_id, title, type, issuer_type, issuer_store_id,
                       threshold_amount, target_type, start_at, end_at)
                    VALUES (?, 'test', '1', ?, ?, 0, '1', NOW(), NOW() + INTERVAL '1 day')
                    """;
                try (PreparedStatement ps = conn.prepareStatement(insertOldSchemaSql)) {
                    ps.setLong(1, 2001L);
                    ps.setString(2, "1"); // ADMIN
                    ps.setLong(3, 777L); // 舊語意：其實是 admin account_id
                    ps.executeUpdate();

                    ps.setLong(1, 2002L);
                    ps.setString(2, "2"); // SELLER
                    ps.setLong(3, 888L); // 語意本來就對：store_id
                    ps.executeUpdate();
                }
            }

            // Step 3：繼續 migrate 到底（套用 V6），這是唯一會讓 backfill UPDATE
            // 對非空、既有髒資料真正執行的路徑。
            Flyway.configure()
                    .dataSource(jdbcUrl, username, password)
                    .locations("classpath:db/migration")
                    .load()
                    .migrate();

            try (Connection conn = DriverManager.getConnection(jdbcUrl, username, password);
                 PreparedStatement ps = conn.prepareStatement(
                         "SELECT issuer_account_id, issuer_store_id FROM coupon_templates WHERE template_id = ?")) {

                ps.setLong(1, 2001L);
                try (ResultSet rs = ps.executeQuery()) {
                    assertTrue(rs.next(), "ADMIN 列應該存在");
                    long issuerAccountId = rs.getLong("issuer_account_id");
                    assertTrue(!rs.wasNull(), "ADMIN 列的 issuer_account_id 應該被 backfill 成有值");
                    assertEquals(777L, issuerAccountId, "ADMIN 列的 issuer_account_id 應等於原本 issuer_store_id 存的值");
                    rs.getObject("issuer_store_id");
                    assertTrue(rs.wasNull(), "ADMIN 列的 issuer_store_id 應該被 backfill 成 NULL");
                }

                ps.setLong(1, 2002L);
                try (ResultSet rs = ps.executeQuery()) {
                    assertTrue(rs.next(), "SELLER 列應該存在");
                    rs.getObject("issuer_account_id");
                    assertTrue(rs.wasNull(), "SELLER 列不在 backfill 的 WHERE issuer_type='1' 範圍內，issuer_account_id 應維持 NULL");
                    long issuerStoreId = rs.getLong("issuer_store_id");
                    assertTrue(!rs.wasNull(), "SELLER 列的 issuer_store_id 應維持原值");
                    assertEquals(888L, issuerStoreId, "SELLER 列的 issuer_store_id 不應被 backfill 動到");
                }
            }
        }
    }
}

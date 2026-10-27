package io.github.will413028.hawkr.aftersales.mapper;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
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

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

/**
 * 2026-05-22 逆物流退貨政策修訂 — AfterSaleCaseMapper 整合測試（TestContainers PostgreSQL + Flyway V1）。
 *
 * <p>驗證實際 SQL 在 PostgreSQL 上的行為，避免 Mockito unit test 看不到 affected rows 真相。
 *
 * <p>覆蓋（對應 §5.3.10 / §5.3.11 / §5.3.12 與 V1 migration）：
 * <ul>
 *   <li>markReturnAtStore：from 20/25 OK / from 27 fail (affected=0)</li>
 *   <li>markReturnDelivered：from 20/25/26 OK（退貨跳號 / 送達事件先到時自 RETURN_PENDING 直達 27）</li>
 *   <li>markReturnFailed：from 20/25/26/27 OK</li>
 *   <li>markAutoCompletedFromAtStore：26→31（STORE_PICKUP_TIMEOUT，原因由 status 區分）</li>
 *   <li>markAutoCompletedFromDelivered：27→32（CONFIRM_RECEIPT_TIMEOUT，原因由 status 區分）</li>
 *   <li>markReturnReceivedByAdmin：from 26 允許（2026-05-22 擴增）</li>
 *   <li>stampReturnProviderInfo（碼 55 路徑）：任意 status 都 stamp 成功、不變 status</li>
 *   <li>stamp*24hWarned CAS 條件：第一次 affected=1，第二次 affected=0（issue #2294 A1–A3 起
 *       這個 affected 值**不再是系統的去重保證**——順序已翻轉為 publish → stamp，
 *       去重由 notification 端 notifications_inbox_dedup_idx 承擔；SQL 未變故斷言仍成立）</li>
 *   <li>6 個新部分索引存在（migration 完整性）</li>
 * </ul>
 */
@Testcontainers
@SpringBootTest(classes = AfterSaleCaseMapperIT.TestBoot.class)
@Tag("integration")
class AfterSaleCaseMapperIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:15-alpine")
            .withDatabaseName("aftersales_test")
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
        // 的硬編碼預設值（指向本機真實 hawkr_aftersales DB）。本測試已在 @BeforeAll 手動
        // 對 testcontainers 跑過一次 migration，這裡索性關掉 Spring 自己的 FlywayAutoConfiguration，
        // 避免它改用真實本機 DB 的帳密連線（會因密碼不符而噴 BeanCreationException）。
        registry.add("spring.flyway.enabled", () -> "false");
        registry.add("hawkr.tsid.enabled", () -> "false");
        // gRPC server port=0 = 隨機 port，避免與本機 dev container（9923）衝突
        // -1 關掉對外 gRPC server（本測試用不到）。0 是「先找空 port 再綁」，
        // 中間可能被 testcontainers 的 port 映射搶走（CI 實測 Address already in use）。
        registry.add("grpc.server.port", () -> "-1");
        // 跳過 Nacos / Spring Cloud（測試不需要服務註冊）
        registry.add("spring.cloud.nacos.discovery.enabled", () -> "false");
        registry.add("spring.cloud.discovery.enabled", () -> "false");
        registry.add("spring.cloud.service-registry.auto-registration.enabled", () -> "false");
    }

    /**
     * Flyway 手動執行（在 Spring 啟動之前）：application.yml 的 ${FLYWAY_ENABLED:false}
     * 預設為 false，且 DynamicPropertySource 對其覆蓋在 Spring Boot 3.2 + Spring Cloud
     * 屬性解析時有時序問題（observed locally），所以乾脆繞過 Spring 的 Flyway
     * 整合，直接在容器啟動後跑一次 migration。
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
    @MapperScan("io.github.will413028.hawkr.aftersales.mapper")
    static class TestBoot {}

    @Autowired private AfterSaleCaseMapper mapper;
    @Autowired private JdbcTemplate jdbc;

    @BeforeEach
    void cleanCases() {
        jdbc.execute("TRUNCATE TABLE aftersale_cases RESTART IDENTITY CASCADE");
    }

    // ==================== markReturnAtStore（20/25→26）====================

    @Test
    @DisplayName("markReturnAtStore: 25→26 成功 + stamp store_arrived_at / store_pickup_deadline_at")
    void markReturnAtStore_fromStatus25_succeeds() {
        insertCase("A001", "O100", "25", "3002");
        LocalDateTime arrivedAt = LocalDateTime.of(2026, 5, 22, 0, 0);
        LocalDateTime deadlineAt = arrivedAt.plusHours(72);

        int affected = mapper.markReturnAtStore("A001", arrivedAt, deadlineAt);

        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A001");
        assertEquals("26", loaded.getStatus());
        assertNotNull(loaded.getStoreArrivedAt());
        assertNotNull(loaded.getStorePickupDeadlineAt());
    }

    @Test
    @DisplayName("markReturnAtStore: from 27 失敗 (affected=0)")
    void markReturnAtStore_fromStatus27_fails() {
        insertCase("A002", "O200", "27", "3002");
        int affected = mapper.markReturnAtStore("A002",
                LocalDateTime.now(), LocalDateTime.now().plusHours(72));
        assertEquals(0, affected);
        assertEquals("27", selectByCaseNo("A002").getStatus());
    }

    @Test
    @DisplayName("markReturnAtStore: from 20（退貨跳號到店，到店事件先於在途）成功 → 26 + stamp store_pickup_deadline_at")
    void markReturnAtStore_fromStatus20_succeeds() {
        insertCase("A0S20", "O120", "20", "3002");
        LocalDateTime arrivedAt = LocalDateTime.of(2026, 5, 22, 0, 0);
        LocalDateTime deadlineAt = arrivedAt.plusHours(72);

        int affected = mapper.markReturnAtStore("A0S20", arrivedAt, deadlineAt);

        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A0S20");
        assertEquals("26", loaded.getStatus());
        assertNotNull(loaded.getStoreArrivedAt());
        assertNotNull(loaded.getStorePickupDeadlineAt());
    }

    // ==================== markReturnDelivered（20/25/26→27）====================

    @Test
    @DisplayName("markReturnDelivered: from 25（宅配）成功")
    void markReturnDelivered_fromStatus25_succeeds() {
        insertCase("A003", "O300", "25", "3001");
        LocalDateTime now = LocalDateTime.of(2026, 5, 22, 1, 0);
        int affected = mapper.markReturnDelivered("A003", now, now.plusHours(48), null);
        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A003");
        assertEquals("27", loaded.getStatus());
        assertNotNull(loaded.getReturnDeliveredAt());
        assertNotNull(loaded.getAutoConfirmAt());
    }

    @Test
    @DisplayName("markReturnDelivered: from 26（超商）成功 + stamp seller_picked_up_at")
    void markReturnDelivered_fromStatus26_succeeds() {
        insertCase("A004", "O400", "26", "3002");
        LocalDateTime now = LocalDateTime.of(2026, 5, 22, 1, 0);
        int affected = mapper.markReturnDelivered("A004", now, now.plusHours(48), now);
        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A004");
        assertEquals("27", loaded.getStatus());
        assertNotNull(loaded.getSellerPickedUpAt());
    }

    @Test
    @DisplayName("markReturnDelivered: from 20（退貨跳號 / 送達事件先到）成功 → 27")
    void markReturnDelivered_fromStatus20_succeeds() {
        insertCase("A005", "O500", "20", "3001");
        LocalDateTime now = LocalDateTime.of(2026, 5, 22, 1, 0);
        int affected = mapper.markReturnDelivered("A005", now, now.plusHours(48), null);
        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A005");
        assertEquals("27", loaded.getStatus());
        assertNotNull(loaded.getReturnDeliveredAt());
        assertNotNull(loaded.getAutoConfirmAt());
    }

    // ==================== markReturnFailed（20/25/26/27→28）====================

    @Test
    @DisplayName("markReturnFailed: from 20/25/26/27 都成功")
    void markReturnFailed_fromAllAllowedStatuses_succeeds() {
        for (String fromStatus : List.of("20", "25", "26", "27")) {
            insertCase("F" + fromStatus, "OF" + fromStatus, fromStatus, "3001");
            int affected = mapper.markReturnFailed("F" + fromStatus, "33", "配送異常");
            assertEquals(1, affected, "from status " + fromStatus + " 應該成功");
            AfterSaleCaseEntity loaded = selectByCaseNo("F" + fromStatus);
            assertEquals("28", loaded.getStatus());
            assertEquals("33", loaded.getReturnProviderStatus());
            assertEquals("配送異常", loaded.getReturnProviderMessage());
        }
    }

    @Test
    @DisplayName("markReturnFailed: from 28（終態）失敗（防 webhook 重投）")
    void markReturnFailed_fromTerminalStatus28_fails() {
        insertCase("A006", "O600", "28", "3001");
        int affected = mapper.markReturnFailed("A006", "44", "其他異常");
        assertEquals(0, affected);
    }

    // ==================== markAutoCompletedFromAtStore（26→31，STORE_PICKUP_TIMEOUT）====================

    @Test
    @DisplayName("markAutoCompletedFromAtStore: 26→31 + stamp auto_complete_at_stamp（原因由 status 區分）")
    void markAutoCompletedFromAtStore_transitionsTo31() {
        insertCase("A007", "O700", "26", "3002");
        LocalDateTime now = LocalDateTime.of(2026, 5, 22, 1, 0);
        int affected = mapper.markAutoCompletedFromAtStore("A007", now);
        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A007");
        assertEquals("31", loaded.getStatus());
        assertNotNull(loaded.getAutoCompleteAtStamp());
    }

    @Test
    @DisplayName("markAutoCompletedFromAtStore: from 27 失敗（CAS 樂觀鎖）")
    void markAutoCompletedFromAtStore_fromStatus27_fails() {
        insertCase("A008", "O800", "27", "3002");
        int affected = mapper.markAutoCompletedFromAtStore("A008", LocalDateTime.now());
        assertEquals(0, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A008");
        assertEquals("27", loaded.getStatus());
    }

    // ==================== markAutoCompletedFromDelivered（27→32，CONFIRM_RECEIPT_TIMEOUT）====================

    @Test
    @DisplayName("markAutoCompletedFromDelivered: 27→32 + stamp auto_complete_at_stamp（原因由 status 區分）")
    void markAutoCompletedFromDelivered_transitionsTo32() {
        insertCase("A009", "O900", "27", "3001");
        LocalDateTime now = LocalDateTime.of(2026, 5, 22, 1, 0);
        int affected = mapper.markAutoCompletedFromDelivered("A009", now);
        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A009");
        assertEquals("32", loaded.getStatus());
        assertNotNull(loaded.getAutoCompleteAtStamp());
    }

    @Test
    @DisplayName("markReturnReceived（賣家手動）：27→30")
    void markReturnReceived_manualSeller_transitionsTo30() {
        insertCase("A010", "O1000", "27", "3001");
        int affected = mapper.markReturnReceived("A010");
        assertEquals(1, affected);
        AfterSaleCaseEntity loaded = selectByCaseNo("A010");
        assertEquals("30", loaded.getStatus());
    }

    // ==================== markReturnReceivedByAdmin（擴增 26 允許）====================

    @Test
    @DisplayName("markReturnReceivedByAdmin: from 26 允許（2026-05-22 擴增）")
    void markReturnReceivedByAdmin_fromStatus26_succeeds() {
        insertCase("A011", "O1100", "26", "3002");
        int affected = mapper.markReturnReceivedByAdmin("A011");
        assertEquals(1, affected);
        assertEquals("30", selectByCaseNo("A011").getStatus());
    }

    @Test
    @DisplayName("markReturnReceivedByAdmin: from 20/25/27/28 皆允許")
    void markReturnReceivedByAdmin_fromAllAllowedStatuses_succeeds() {
        for (String fromStatus : List.of("20", "25", "27", "28")) {
            insertCase("AD" + fromStatus, "OAD" + fromStatus, fromStatus, "3001");
            int affected = mapper.markReturnReceivedByAdmin("AD" + fromStatus);
            assertEquals(1, affected, "from status " + fromStatus + " 應允許");
            assertEquals("30", selectByCaseNo("AD" + fromStatus).getStatus());
        }
    }

    // ==================== stampReturnProviderInfo（碼 55 路徑，不變狀態）====================

    @Test
    @DisplayName("stampReturnProviderInfo: 任意 status 都成功 stamp 訊息、不變 status")
    void stampReturnProviderInfo_anyStatus_stampsWithoutStateChange() {
        for (String status : List.of("20", "25", "26", "27", "28", "30", "50")) {
            insertCase("M" + status, "OM" + status, status, "3002");
            int affected = mapper.stampReturnProviderInfo("M" + status, "55", "賣家未取");
            assertEquals(1, affected, "status " + status + " 應允許 stamp");
            AfterSaleCaseEntity loaded = selectByCaseNo("M" + status);
            assertEquals(status, loaded.getStatus(), "stamp 不應變更 status");
            assertEquals("55", loaded.getReturnProviderStatus());
            assertEquals("賣家未取", loaded.getReturnProviderMessage());
        }
    }

    // ==================== stamp*24hWarned CAS 條件（非系統去重保證，見 issue #2294 A1–A3） ====================

    @Test
    @DisplayName("stampStorePickup24hWarned: 第一次=1，第二次=0（CAS 條件，非系統去重保證）")
    void stampStorePickup24hWarned_isIdempotent() {
        insertCase("A020", "O2000", "26", "3002");
        LocalDateTime now1 = LocalDateTime.of(2026, 5, 22, 1, 0);
        LocalDateTime now2 = LocalDateTime.of(2026, 5, 22, 2, 0);

        assertEquals(1, mapper.stampStorePickup24hWarned("A020", now1));
        assertEquals(0, mapper.stampStorePickup24hWarned("A020", now2),
                "第二次 stamp 的 CAS 條件不成立 affected=0");
    }

    @Test
    @DisplayName("stampConfirmReceipt24hWarned: 第一次=1，第二次=0（CAS 條件，非系統去重保證）")
    void stampConfirmReceipt24hWarned_isIdempotent() {
        insertCase("A021", "O2100", "27", "3001");
        LocalDateTime now1 = LocalDateTime.of(2026, 5, 22, 1, 0);
        LocalDateTime now2 = LocalDateTime.of(2026, 5, 22, 2, 0);

        assertEquals(1, mapper.stampConfirmReceipt24hWarned("A021", now1));
        assertEquals(0, mapper.stampConfirmReceipt24hWarned("A021", now2));
    }

    @Test
    @DisplayName("stampStorePickup24hWarned: status≠26 affected=0（CAS 條件）")
    void stampStorePickup24hWarned_statusNot26_fails() {
        insertCase("A022", "O2200", "27", "3002");
        assertEquals(0, mapper.stampStorePickup24hWarned("A022", LocalDateTime.now()));
    }

    // ==================== findBuyerIdByCaseNo（issue #1626）====================

    @Test
    @DisplayName("findBuyerIdByCaseNo: 查得到 case 回傳 buyer_id")
    void findBuyerIdByCaseNo_existingCase_returnsBuyerId() {
        insertCase("B001", "O900", "20", "3002");

        Long buyerId = mapper.findBuyerIdByCaseNo("B001");

        assertEquals(2001L, buyerId);
    }

    @Test
    @DisplayName("findBuyerIdByCaseNo: 查無 case 回傳 null")
    void findBuyerIdByCaseNo_missingCase_returnsNull() {
        Long buyerId = mapper.findBuyerIdByCaseNo("NOT_EXIST");

        assertNull(buyerId);
    }

    // ==================== 部分索引存在（V1 migration 完整性）====================

    @Test
    @DisplayName("6 個新部分索引存在於 PostgreSQL（V1 migration 結構驗證）")
    void newPartialIndexesExist() {
        List<String> indexes = jdbc.queryForList("""
                SELECT indexname FROM pg_indexes
                 WHERE tablename = 'aftersale_cases'
                """, String.class);
        // 2026-05-22 新增的 3 個部分索引
        assertTrue(indexes.contains("idx_aftersale_cases_store_pickup_deadline"));
        assertTrue(indexes.contains("idx_aftersale_cases_warn_store_24h"));
        assertTrue(indexes.contains("idx_aftersale_cases_warn_confirm_24h"));
        // 既有的 auto_confirm 索引仍存在
        assertTrue(indexes.contains("idx_aftersale_cases_auto_confirm"));
        // 既有的退貨逾時/處理逾時索引仍存在
        assertTrue(indexes.contains("idx_aftersale_cases_return_timeout"));
        assertTrue(indexes.contains("idx_aftersale_cases_process_deadline"));
    }

    // ==================== V6: seller_account_id → store_id rename（Plan C Phase 3）====================

    @Test
    @DisplayName("V6 migration: aftersale_cases.store_id 存在、seller_account_id 真的消失、值健在、索引改名")
    void v6Migration_aftersaleCasesRekeysSellerAccountIdToStoreId() {
        insertCase("V6T1", "OV6T1", "0", "3001");

        AfterSaleCaseEntity loaded = selectByCaseNo("V6T1");
        assertEquals(1001L, loaded.getStoreId(), "既有值必須原封不動存活在新欄名下");

        List<String> columns = jdbc.queryForList("""
                SELECT column_name FROM information_schema.columns
                 WHERE table_name = 'aftersale_cases'
                """, String.class);
        assertTrue(columns.contains("store_id"), "store_id 欄位應存在");
        assertFalse(columns.contains("seller_account_id"), "seller_account_id 不應再存在（非相容 view，是真的 rename）");

        List<String> indexes = jdbc.queryForList("""
                SELECT indexname FROM pg_indexes
                 WHERE tablename = 'aftersale_cases'
                """, String.class);
        assertTrue(indexes.contains("idx_aftersale_cases_store"), "索引應改名為 idx_aftersale_cases_store");
        assertFalse(indexes.contains("idx_aftersale_cases_seller"), "舊索引名不應存在");
    }

    @Test
    @DisplayName("V6 migration: seller_cancel_requests.store_id 存在、seller_account_id 真的消失、索引改名")
    void v6Migration_sellerCancelRequestsRekeysSellerAccountIdToStoreId() {
        jdbc.update("""
                INSERT INTO seller_cancel_requests
                    (id, order_no, store_id, buyer_id, reason, status,
                     refund_amount, transaction_id, payment_method, mer_id, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
                """,
                9001L, "OV6T2", 1001L, 2001L, "測試原因", "0", 500L, "TXN-V6T2", "CREDIT_CARD", "HAWK9");

        Long storeId = jdbc.queryForObject(
                "SELECT store_id FROM seller_cancel_requests WHERE id = ?", Long.class, 9001L);
        assertEquals(1001L, storeId, "既有值必須原封不動存活在新欄名下");

        List<String> columns = jdbc.queryForList("""
                SELECT column_name FROM information_schema.columns
                 WHERE table_name = 'seller_cancel_requests'
                """, String.class);
        assertTrue(columns.contains("store_id"), "store_id 欄位應存在");
        assertFalse(columns.contains("seller_account_id"), "seller_account_id 不應再存在");

        List<String> indexes = jdbc.queryForList("""
                SELECT indexname FROM pg_indexes
                 WHERE tablename = 'seller_cancel_requests'
                """, String.class);
        assertTrue(indexes.contains("idx_seller_cancel_store"), "索引應改名為 idx_seller_cancel_store");
        assertFalse(indexes.contains("idx_seller_cancel_seller"), "舊索引名不應存在");
    }

    // ==================== helpers ====================

    /**
     * 直接 SQL insert（避免走 MyBatis-Plus auto-fill，方便精確控制 status / 欄位）。
     * 對應 V1 schema 的 NOT NULL 欄位給預設值。
     */
    private void insertCase(String caseNo, String orderNo, String status, String returnShippingMethod) {
        jdbc.update("""
                INSERT INTO aftersale_cases
                    (id, case_no, order_no, buyer_id, store_id, status,
                     reason, description, refundable_amount, return_shipping_method,
                     payment_method, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, 'BROKEN', '測試', 1000, ?, 'CREDIT_CARD', NOW(), NOW())
                """,
                Math.abs((long) caseNo.hashCode() * 100000L),
                caseNo, orderNo, 2001L, 1001L, status, returnShippingMethod);
    }

    private AfterSaleCaseEntity selectByCaseNo(String caseNo) {
        return mapper.selectOne(new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<AfterSaleCaseEntity>()
                .eq(AfterSaleCaseEntity::getCaseNo, caseNo));
    }
}

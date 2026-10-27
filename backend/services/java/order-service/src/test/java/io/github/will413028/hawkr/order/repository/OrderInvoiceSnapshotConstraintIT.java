package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.order.entity.OrderInvoiceSnapshotEntity;
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
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Flyway {@code V12} 的 {@code order_invoice_snapshots} 在<b>真實 PostgreSQL</b> 上跑一次
 * （58 號 §5.4.3）。
 *
 * <p><b>為什麼需要真 DB</b>：CHECK 約束在 PostgreSQL 上「運算式為 NULL 時放行」
 * （§5.5 通則）——一條寫錯的 CHECK 會<b>看起來在擋、實際對最該擋的那一組輸入永遠放行</b>，
 * 而那在任何 mock 測試上都是全綠。§5.5 逐字要求：每一條新增的 CHECK 都要有一筆
 * 「應該被擋下」的 INSERT 測試，且該測試要真的看到 {@code 23514}。本檔就是那些測試。
 *
 * <p>同時驗兩件只有真 DB 看得到的事：V12 這張表真的存在（Flyway 跑得過），
 * 以及 MyBatis 有沒有把每一欄映回 {@link OrderInvoiceSnapshotEntity}
 * ——SQL 形狀測試對這兩件事全盲。
 */
@SpringBootTest(classes = OrderInvoiceSnapshotConstraintIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class OrderInvoiceSnapshotConstraintIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_invoice_snapshot_test")
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
    private OrderInvoiceSnapshotRepository repository;

    @Autowired
    private JdbcTemplate jdbc;

    @BeforeEach
    void reset() {
        jdbc.update("DELETE FROM order_invoice_snapshots");
    }

    // ==================== helpers ====================

    /** 直接走 SQL 而非 Entity：要測的是 DB 層的 CHECK，不能被 Java 端的驗證擋在前面。 */
    private void insertRaw(long id, String orderNo, String kind, String mode,
                           Long prefId, String taxId, String title, String donation, String email) {
        jdbc.update("""
                INSERT INTO order_invoice_snapshots
                  (id, order_no, snapshot_version, invoice_kind, carrier_selection_mode,
                   buyer_invoice_preference_id, buyer_tax_id, buyer_title, donation_code, notify_email)
                VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?)
                """, id, orderNo, kind, mode, prefId, taxId, title, donation, email);
    }

    /**
     * 斷言這一筆被指名的約束以指定 SQLSTATE 擋下。
     *
     * <p>⚠️ 不能只看訊息字串：Spring 會把 {@code PSQLException} 翻譯成
     * {@code DataIntegrityViolationException}，翻譯後的訊息<b>不含 SQLSTATE</b>
     * ⇒ 斷言 {@code hasStackTraceContaining("23514")} 會對「約束確實生效」的情況誤判成失敗。
     * 要拿 SQLSTATE 只能往 cause 鏈裡找真正的 {@link java.sql.SQLException}。
     */
    private static void assertSqlState(org.junit.jupiter.api.function.Executable e,
                                       String sqlState, String constraint) {
        Throwable thrown = org.assertj.core.api.Assertions.catchThrowable(e::execute);
        assertThat(thrown)
                .as("這一筆應該被 %s 擋下（CHECK 在運算式為 NULL 時會放行，寫錯的 CHECK 長得跟對的一樣）",
                        constraint)
                .isNotNull();

        java.sql.SQLException sql = null;
        for (Throwable t = thrown; t != null; t = t.getCause()) {
            if (t instanceof java.sql.SQLException se) {
                sql = se;
                break;
            }
        }
        assertThat((Object) sql).as("cause 鏈裡找不到 SQLException：%s", thrown).isNotNull();
        assertThat(sql.getSQLState()).as("SQLSTATE").isEqualTo(sqlState);
        if (constraint != null) {
            assertThat(thrown).hasStackTraceContaining(constraint);
        }
    }

    private static void assertCheckViolation(org.junit.jupiter.api.function.Executable e, String constraint) {
        assertSqlState(e, "23514", constraint);
    }

    /** 只斷言 SQLSTATE，不指名約束——用在「多條 CHECK 都可能先命中」的情境。 */
    private static void assertSqlStateOnly(org.junit.jupiter.api.function.Executable e, String sqlState) {
        assertSqlState(e, sqlState, null);
    }

    // ==================== 合法輸入必須進得去 ====================

    @Test
    @DisplayName("缺省快照（B2C ＋ USE_BUYER_DEFAULT，三個目標全 NULL）寫得進去，且每一欄映得回 Entity")
    void defaultSnapshotRoundTrips() {
        OrderInvoiceSnapshotEntity row = new OrderInvoiceSnapshotEntity();
        row.setId(1L);
        row.setOrderNo("O100000001");
        row.setSnapshotVersion(1);
        row.setInvoiceKind(InvoiceDocumentKind.B2C.value());
        row.setCarrierSelectionMode(InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value());
        row.setNotifyEmail("buyer@example.com");

        assertThat(repository.insert(row)).isEqualTo(1);

        OrderInvoiceSnapshotEntity got = repository.findLatestByOrderNo("O100000001");
        assertThat(got).isNotNull();
        assertThat(got.getId()).isEqualTo(1L);
        assertThat(got.getSnapshotVersion()).isEqualTo(1);
        assertThat(got.getInvoiceKind()).isEqualTo(InvoiceDocumentKind.B2C.value());
        assertThat(got.getCarrierSelectionMode())
                .isEqualTo(InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value());
        assertThat(got.getBuyerInvoicePreferenceId()).isNull();
        assertThat(got.getBuyerTaxId()).isNull();
        assertThat(got.getBuyerTitle()).isNull();
        assertThat(got.getDonationCode()).isNull();
        assertThat(got.getNotifyEmail())
                .as("notify_email 是本案補進 V12 的欄位；沒有它 invoice_documents.recipient_email 恆空")
                .isEqualTo("buyer@example.com");
        assertThat(got.getCreatedAt()).as("DEFAULT NOW() ／ MetaObjectHandler 兩層之一必須填到").isNotNull();
    }

    @Test
    @DisplayName("B2B（統編＋抬頭＋NONE）、EXPLICIT_PREFERENCE、DONATION 三種合法組合都寫得進去")
    void allLegalShapesAreAccepted() {
        insertRaw(2L, "O200000002", "B2B", "NONE", null, "12345678", "範例生鮮股份有限公司", null, null);
        insertRaw(3L, "O300000003", "B2C", "EXPLICIT_PREFERENCE", 2649605557L, null, null, null, null);
        insertRaw(4L, "O400000004", "B2C", "DONATION", null, null, null, "5238", "a@b.co");

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM order_invoice_snapshots", Integer.class))
                .isEqualTo(3);
    }

    // ==================== 每一條 CHECK 都要有一筆被擋下的 INSERT ====================

    @Test
    @DisplayName("chk_ois_kind：invoice_kind 只有 B2C／B2B（RECEIPT 是 invoice_documents 的值域，不是本表的）")
    void rejectsUnknownInvoiceKind() {
        assertCheckViolation(
                () -> insertRaw(10L, "O1", "RECEIPT", "USE_BUYER_DEFAULT", null, null, null, null, null),
                "chk_ois_kind");
    }

    @Test
    @DisplayName("chk_ois_mode：carrier_selection_mode 只有四值")
    void rejectsUnknownCarrierSelectionMode() {
        assertCheckViolation(
                () -> insertRaw(11L, "O1", "B2C", "MOBILE_BARCODE", null, null, null, null, null),
                "chk_ois_mode");
    }

    @Test
    @DisplayName("chk_ois_b2b：B2B 少了抬頭就進不去（統編有值但 buyer_title 為 NULL）")
    void rejectsB2bWithoutTitle() {
        assertCheckViolation(
                () -> insertRaw(12L, "O1", "B2B", "NONE", null, "12345678", null, null, null),
                "chk_ois_b2b");
    }

    @Test
    @DisplayName("chk_ois_b2b：B2B 的統編非 8 碼數字進不去——⚠️ COALESCE 那一段就是為了這一筆（統編為 NULL 時 CHECK 會放行）")
    void rejectsB2bWithNullTaxId() {
        assertCheckViolation(
                () -> insertRaw(13L, "O1", "B2B", "NONE", null, null, "抬頭", null, null),
                "chk_ois_b2b");
    }

    @Test
    @DisplayName("chk_ois_b2c_no_taxid：B2C 不得帶統編")
    void rejectsB2cWithTaxId() {
        assertCheckViolation(
                () -> insertRaw(14L, "O1", "B2C", "NONE", null, "12345678", null, null, null),
                "chk_ois_b2c_no_taxid");
    }

    @Test
    @DisplayName("chk_ois_pref_pair：EXPLICIT_PREFERENCE 必須有 id、有 id 必須是 EXPLICIT_PREFERENCE（雙向）")
    void rejectsPreferenceModePairMismatch() {
        assertCheckViolation(
                () -> insertRaw(15L, "O1", "B2C", "EXPLICIT_PREFERENCE", null, null, null, null, null),
                "chk_ois_pref_pair");
        assertCheckViolation(
                () -> insertRaw(16L, "O1", "B2C", "USE_BUYER_DEFAULT", 100L, null, null, null, null),
                "chk_ois_pref_pair");
    }

    @Test
    @DisplayName("chk_ois_donation_pair：DONATION 必須有捐贈碼、有捐贈碼必須是 DONATION（雙向）")
    void rejectsDonationModePairMismatch() {
        assertCheckViolation(
                () -> insertRaw(17L, "O1", "B2C", "DONATION", null, null, null, null, null),
                "chk_ois_donation_pair");
        assertCheckViolation(
                () -> insertRaw(18L, "O1", "B2C", "USE_BUYER_DEFAULT", null, null, null, "5238", null),
                "chk_ois_donation_pair");
    }

    @Test
    @DisplayName("chk_ois_donation_fmt：捐贈碼非 3–7 碼數字")
    void rejectsMalformedDonationCode() {
        assertCheckViolation(
                () -> insertRaw(19L, "O1", "B2C", "DONATION", null, null, null, "52", null),
                "chk_ois_donation_fmt");
    }

    @Test
    @DisplayName("三選一：任兩個目標同時有值一律進不去（AMEGO 不擋，DB 這一層擋）")
    void rejectsTwoTargetsAtOnce() {
        // ⚠️ 這裡刻意**不指名 chk_ois_exclusive**。實測發現：pref_pair 與 donation_pair 都是
        // 「mode ⟺ 該欄非空」的雙條件式，而 mode 只有一個值 ⇒ 只要有兩個目標同時非空，
        // 必然先違反其中一條 pair 約束（本例回的是 chk_ois_donation_pair）。
        // 換句話說 **chk_ois_exclusive 是一條到不了的備援**——它單獨不可能是第一個被報出來的
        // 約束，而 PostgreSQL 也不保證多條 CHECK 的評估順序。把它寫進斷言等於斷言一個
        // 實作細節，會在 PostgreSQL 換順序時假紅。要守的不變式是「兩個目標寫不進去」。
        assertSqlStateOnly(
                () -> insertRaw(20L, "O1", "B2B", "NONE", null, "12345678", "抬頭", "5238", null),
                "23514");
        assertSqlStateOnly(
                () -> insertRaw(21L, "O1", "B2C", "EXPLICIT_PREFERENCE", 100L, null, null, "5238", null),
                "23514");
    }

    @Test
    @DisplayName("chk_ois_notify_email：形狀不成立的信箱進不去，NULL 則放行（本欄選填）")
    void rejectsMalformedNotifyEmail() {
        assertCheckViolation(
                () -> insertRaw(31L, "O1", "B2C", "USE_BUYER_DEFAULT", null, null, null, null, "not-an-email"),
                "chk_ois_notify_email");
        // NULL 必須放行——否則沒填信箱的買家一律結不了帳
        insertRaw(32L, "O2", "B2C", "USE_BUYER_DEFAULT", null, null, null, null, null);
    }

    @Test
    @DisplayName("chk_ois_version：snapshot_version 下界為 1")
    void rejectsVersionBelowOne() {
        assertCheckViolation(() -> jdbc.update("""
                INSERT INTO order_invoice_snapshots
                  (id, order_no, snapshot_version, invoice_kind, carrier_selection_mode)
                VALUES (23, 'O1', 0, 'B2C', 'USE_BUYER_DEFAULT')
                """), "chk_ois_version");
    }

    @Test
    @DisplayName("uk_order_invoice_snapshots：同一 (order_no, snapshot_version) 只能有一列（重送結帳不得產生第二份快照）")
    void rejectsDuplicateOrderNoAndVersion() {
        insertRaw(24L, "ODUP", "B2C", "USE_BUYER_DEFAULT", null, null, null, null, null);
        assertSqlState(() ->
                        insertRaw(25L, "ODUP", "B2C", "USE_BUYER_DEFAULT", null, null, null, null, null),
                "23505", "uk_order_invoice_snapshots");
    }

    // ==================== 索引取捨（§5.4.3 刻意不建單欄索引）====================

    @Test
    @DisplayName("刻意不建 (order_no) 單欄索引：唯一鍵的前綴已足夠，多一個窄索引只是結帳 hot path 的寫入放大")
    void hasNoRedundantSingleColumnIndexOnOrderNo() {
        // 🔴 **改成逐名比對，不是數個數**（2026-09-17）。
        //    原本 pin 的是「恰好 2 個索引」，而 V13 合法新增了 uk_ois_revision_idem
        //    （B06／R03 更正的冪等鍵）之後這個數字就過期了——而它**沒有被任何人發現**，
        //    因為 @Tag("integration") 讓這支測試從來沒有在 CI 跑過。
        //    ⇒ 純計數的 pin 有兩個問題：① 合法新增索引時它會紅，而修法只是「把 2 改成 3」，
        //    那等於把 gate 關掉 ② 它**分不出**新增的是正當索引還是本測試要禁的
        //    (order_no) 單欄索引——兩者都只是讓數字 +1。
        //    逐名比對讓「新增索引」變成一個必須在 diff 裡寫清楚的動作，
        //    而下面那條 order_no 單欄索引的禁令則獨立成立、與索引總數無關。
        List<String> indexes = jdbc.queryForList("""
                SELECT indexname FROM pg_indexes
                WHERE tablename = 'order_invoice_snapshots'
                ORDER BY indexname
                """, String.class);

        // 對照組：查詢真的看得到這張表（空清單時下面兩條斷言都會空過）。
        assertThat(indexes)
                .as("量測本身失效：order_invoice_snapshots 一個索引都查不到 ⇒ 查詢或 schema 不對")
                .isNotEmpty();

        assertThat(indexes)
                .as("索引清單變了。新增索引請在這裡具名登記，並在 PR 說明它的讀取端收益；"
                        + "這張表在結帳 hot path 上，每個索引都是寫入放大")
                .containsExactlyInAnyOrder(
                        // PK
                        "order_invoice_snapshots_pkey",
                        // 業務唯一鍵（order_no 是它的前綴 ⇒ 單欄查詢已經走得到）
                        "uk_order_invoice_snapshots",
                        // V13：B06／R03 更正的冪等鍵（partial，NULL 不參與）
                        "uk_ois_revision_idem");

        // 🔴 本測試 DisplayName 的那條禁令，獨立於索引總數成立：
        //    uk_order_invoice_snapshots 的前綴已經是 order_no ⇒ 再建一個 (order_no) 單欄索引
        //    對讀取零收益、對結帳寫入是純成本。
        List<String> onOrderNoOnly = jdbc.queryForList("""
                SELECT indexname FROM pg_indexes
                WHERE tablename = 'order_invoice_snapshots'
                  AND indexdef LIKE '%(order_no)%'
                """, String.class);
        assertThat(onOrderNoOnly)
                .as("(order_no) 單欄索引：唯一鍵的前綴已足夠，多一個窄索引只是結帳 hot path 的寫入放大")
                .isEmpty();
    }
}

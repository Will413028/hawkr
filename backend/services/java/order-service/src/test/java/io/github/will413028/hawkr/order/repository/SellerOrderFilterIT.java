package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.SellerOrderQuery;
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
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 賣家訂單 keyword ／日期範圍篩選在**真實 PostgreSQL** 上的行為（backend#2478，56 號設計書 §6.1.10）。
 *
 * <p>為什麼要真 DB：ILIKE 的大小寫與萬用字元語意、EXISTS 半連接會不會讓一單多品的訂單重複、
 * LIKE 跳脫有沒有真的擋住使用者輸入的 {@code %}、以及 {@code LocalDateTime} 綁進 timestamptz
 * 欄位的日界邊界——這四件事 Mockito 一件也驗不到，SQL 渲染測試（
 * {@link OrderRepositorySellerFilterSqlTest}）也只看得到條件的形狀而非結果。
 *
 * <p>每個案例都同時斷言 list 與 count：兩句是獨立 SQL，只驗其中一句等於放掉一半。
 */
@SpringBootTest(classes = SellerOrderFilterIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class SellerOrderFilterIT {

    private static final long STORE_ID = 2685914682L;
    private static final long OTHER_STORE_ID = 2632049831L;

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_seller_filter_test")
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
        // application.yml 的 spring.flyway.* 指向本機真實 hawkr_order；不覆寫會讓 migration
        // 跑到那裡去，而 testcontainers 裡的測試 DB 是空的。
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
            MyBatisPlusAutoConfiguration.class  // common-core：TimestampTz TypeHandler + MetaObjectHandler
    })
    @MapperScan("io.github.will413028.hawkr.order.repository")
    static class MinimalConfig {
    }

    @Autowired
    private OrderRepository orderRepository;

    @Autowired
    private JdbcTemplate jdbc;

    /**
     * 固定資料：同店四筆一般訂單 ＋ 一筆自購（buyer_id = store_id，列表本來就排除）
     * ＋ 一筆他店訂單（跨店隔離）。
     */
    @BeforeEach
    void seed() {
        jdbc.update("DELETE FROM order_items");
        jdbc.update("DELETE FROM orders");

        order(1L, "O100000001", STORE_ID, 501L, "林小明", "2026-08-01 02:00:00+00");
        item(11L, "O100000001", "有機蘋果", "一箱 5kg");

        order(2L, "O200000002", STORE_ID, 502L, "陳大文", "2026-08-10 02:00:00+00");
        item(21L, "O200000002", "溫室番茄", "500g 盒裝");

        // recipient_name 刻意含字面 % ：用來分辨「LIKE 萬用字元有沒有被跳脫」
        order(3L, "O300000003", STORE_ID, 503L, "100% 王", "2026-08-13 23:59:59+00");
        item(31L, "O300000003", "蜜蘋果禮盒", "大");

        // 一單兩品且兩品都會命中 "蘋果"：半連接寫成 JOIN 的話這筆會重複出現
        order(4L, "O400000004", STORE_ID, 504L, "李四", "2026-08-20 02:00:00+00");
        item(41L, "O400000004", "有機蘋果", "散裝 1kg");
        item(42L, "O400000004", "蘋果醋", "隨手瓶");

        order(5L, "O500000005", STORE_ID, STORE_ID, "自購", "2026-08-10 03:00:00+00");
        item(51L, "O500000005", "有機蘋果", "一箱 5kg");

        order(9L, "O900000009", OTHER_STORE_ID, 509L, "他店買家", "2026-08-10 04:00:00+00");
        item(91L, "O900000009", "有機蘋果", "一箱 5kg");
    }

    @Test
    @DisplayName("無 keyword：只回本店非自購訂單，count 與筆數一致")
    void noKeywordReturnsWholeStore() {
        assertMatches(query(null, null, null), "O400000004", "O300000003", "O200000002", "O100000001");
    }

    @Test
    @DisplayName("keyword 命中 order_no")
    void keywordMatchesOrderNo() {
        assertMatches(query("%O200000002%", null, null), "O200000002");
        // 訂單編號不含日期也不遞增，只能子字串匹配；取中段仍要命中
        assertMatches(query("%0000000%", null, null),
                "O400000004", "O300000003", "O200000002", "O100000001");
    }

    @Test
    @DisplayName("keyword 命中 recipient_name（列表本來就未遮罩顯示，故可搜）")
    void keywordMatchesRecipientName() {
        assertMatches(query("%林%", null, null), "O100000001");
        assertMatches(query("%陳大文%", null, null), "O200000002");
    }

    @Test
    @DisplayName("keyword 命中商品名／規格名，且一單多品不重複出現")
    void keywordMatchesItemNames() {
        // product_name：O100000001「有機蘋果」、O300000003「蜜蘋果禮盒」、O400000004 兩品皆含「蘋果」
        assertMatches(query("%蘋果%", null, null), "O400000004", "O300000003", "O100000001");
        // sku_name
        assertMatches(query("%5kg%", null, null), "O100000001");
        assertMatches(query("%隨手瓶%", null, null), "O400000004");
    }

    @Test
    @DisplayName("keyword 大小寫不敏感（ILIKE）")
    void keywordIsCaseInsensitive() {
        assertMatches(query("%o200000002%", null, null), "O200000002");
        assertMatches(query("%5KG%", null, null), "O100000001");
    }

    @Test
    @DisplayName("使用者輸入的 % 被當字面字元：只命中名字裡真的有 % 的那筆")
    void wildcardInUserInputIsEscaped() {
        // OrderService.toKeywordPattern("%") 的結果；未跳脫的話 pattern 會是 "%%%"＝全部命中
        assertMatches(query("%\\%%", null, null), "O300000003");
    }

    @Test
    @DisplayName("日期範圍以 created_at 篩選，上界含當日最後一刻")
    void dateRangeFiltersOnCreatedAt() {
        // OrderService 對純日期 date_to 的補值：2026-08-13 → 當日最後一微秒。
        // 若上界誤補成 00:00:00，23:59:59 那筆會掉出去。
        assertMatches(query(null, LocalDateTime.of(2026, 8, 10, 0, 0),
                        LocalDateTime.of(2026, 8, 13, 23, 59, 59, 999_999_000)),
                "O300000003", "O200000002");

        // 只給下界
        assertMatches(query(null, LocalDateTime.of(2026, 8, 13, 0, 0), null),
                "O400000004", "O300000003");

        // 只給上界，且邊界值恰為某筆的 created_at ⇒ 閉區間必須含它（也順帶證明測試 JVM 跑在 UTC，
        // 否則這個時刻會位移，見 pom surefire 的 -Duser.timezone=UTC）
        assertMatches(query(null, null, LocalDateTime.of(2026, 8, 1, 2, 0)), "O100000001");
    }

    @Test
    @DisplayName("keyword 與日期範圍同時生效（AND）")
    void keywordAndDateRangeCombine() {
        assertMatches(query("%蘋果%", LocalDateTime.of(2026, 8, 13, 0, 0), null),
                "O400000004", "O300000003");
    }

    @Test
    @DisplayName("空集合：條件無命中時 list 與 count 都是 0")
    void emptyResult() {
        assertMatches(query("%不存在的商品%", null, null));
    }

    // ==================== helpers ====================

    private SellerOrderQuery query(String keywordPattern, LocalDateTime dateFrom, LocalDateTime dateTo) {
        return new SellerOrderQuery(STORE_ID, 0L, List.of(), keywordPattern, dateFrom, dateTo);
    }

    /** 同時斷言 list 的內容順序與 count 的數字——兩句是獨立 SQL，只驗一句等於放掉一半。 */
    private void assertMatches(SellerOrderQuery query, String... expectedOrderNosNewestFirst) {
        List<OrderEntity> rows = orderRepository.findSellerOrders(query, 50, 0);
        assertThat(rows).extracting(OrderEntity::getOrderNo)
                .containsExactly(expectedOrderNosNewestFirst);
        assertThat(orderRepository.countSellerOrders(query))
                .as("count 必須與 list 同條件（否則前端 total 與 records 對不上）")
                .isEqualTo(expectedOrderNosNewestFirst.length);
    }

    private void order(long id, String orderNo, long storeId, long buyerId,
                       String recipientName, String createdAt) {
        jdbc.update("""
                INSERT INTO orders (id, order_no, buyer_id, store_id, total_original_amount,
                                    total_actual_amount, status, recipient_name, created_at)
                VALUES (?, ?, ?, ?, 100, 100, '20', ?, ?::timestamptz)
                """, id, orderNo, buyerId, storeId, recipientName, createdAt);
    }

    private void item(long id, String orderNo, String productName, String skuName) {
        jdbc.update("""
                INSERT INTO order_items (id, order_no, product_id, product_name, sku_id, sku_name,
                                         quantity, list_price, unit_price, actual_pay_amount)
                VALUES (?, ?, ?, ?, ?, ?, 1, 100, 100, 100)
                """, id, orderNo, id, productName, id, skuName);
    }
}

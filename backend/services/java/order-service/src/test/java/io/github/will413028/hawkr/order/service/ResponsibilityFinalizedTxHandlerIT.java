package io.github.will413028.hawkr.order.service;

import static org.assertj.core.api.Assertions.assertThat;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.order.mq.OrderEventPublisher;
import io.github.will413028.hawkr.order.repository.GrantPaymentTaskRepository;
import io.github.will413028.hawkr.order.repository.OrderRepository;
import java.util.List;
import java.util.Map;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;
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
import org.springframework.context.annotation.Bean;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.transaction.annotation.EnableTransactionManagement;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

/**
 * {@code 35 → DELIVERY_FAILED_NO_REFUND(54)} 與撥款任務建立在<b>真實 PostgreSQL</b> 上的行為
 * （03 §5.7.3／70 §3.8.1）。
 *
 * <p><b>為什麼非真 DB 不可</b>——這三件事 mock 一件都證明不了：
 * <ul>
 *   <li><b>交易原子性</b>：撥款任務 INSERT 失敗時，狀態轉換必須<b>不存在</b>。
 *       mock 只能證明「我呼叫了 insert」，證明不了 rollback 之後那一列回到了 {@code 35}。</li>
 *   <li><b>CAS 的 allowed-from</b>：{@code WHERE status='35'} 對其他狀態真的 rows=0。
 *       mock repository 沒有述詞，每一次都會「成功」。</li>
 *   <li><b>唯一約束</b>：{@code uk_grant_payment_order} 在第二筆任務上真的擋下來。</li>
 * </ul>
 *
 * <p>🔴 <b>handler 必須是真的 Spring bean</b>（不是手動 {@code new}）：{@code @Transactional}
 * 靠 proxy，手動 new 的實例上它<b>完全不生效</b> ⇒ 測到的會是「沒有交易」那條路徑，
 * 而那正是本測試要防的東西。本檔因此在 {@link MinimalConfig} 內以 {@code @Bean} 註冊
 * 並開 {@code @EnableTransactionManagement}。
 */
@SpringBootTest(classes = ResponsibilityFinalizedTxHandlerIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
@DisplayName("責任判定 BUYER_CAUSED ⇒ 35→54 ＋ 撥款任務的真 DB 行為")
class ResponsibilityFinalizedTxHandlerIT {

    private static final String ORDER_NO = "OWP3IT000001";
    private static final long ORDER_ID = 910000000000001L;

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("order_responsibility_test")
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
        // ⚠️ application.yml 的 spring.flyway.* 有指向本機真實 DB 的硬編碼預設值；
        //    不覆寫會讓 Flyway 對本機 no-op、容器內的測試 DB 從未跑過 migration
        //    ⇒ 測試會對著空 schema「全綠」。
        registry.add("spring.flyway.url", POSTGRES::getJdbcUrl);
        registry.add("spring.flyway.user", POSTGRES::getUsername);
        registry.add("spring.flyway.password", POSTGRES::getPassword);
        registry.add("spring.flyway.enabled", () -> "true");
        registry.add("spring.flyway.locations", () -> "classpath:db/migration");
        registry.add("spring.flyway.baseline-on-migrate", () -> "false");
    }

    @SpringBootConfiguration
    @EnableTransactionManagement
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

        /**
         * emit 是 afterCommit 的 realtime 推播，與本檔的不變式無關 ⇒ mock。
         * ⚠️ 但它<b>不能</b>是 null：handler 在 CAS 成功後一定會呼叫它，
         * null 會讓成功路徑變成 NPE 而測試以為「交易失敗了」。
         */
        @Bean
        OrderEventPublisher orderEventPublisher() {
            return Mockito.mock(OrderEventPublisher.class);
        }

        @Bean
        ResponsibilityFinalizedTxHandler handler(OrderRepository orderRepository,
                                                 GrantPaymentTaskRepository grantRepo,
                                                 OrderEventPublisher publisher) {
            return new ResponsibilityFinalizedTxHandler(orderRepository, grantRepo, publisher);
        }
    }

    @Autowired
    private ResponsibilityFinalizedTxHandler handler;

    @Autowired
    private JdbcTemplate jdbc;

    /**
     * 🔴 {@code TsidUtil} 是靜態單例，正式啟動時由 common-core 的 {@code TsidConfiguration}
     * 依 {@code node-id-base} 初始化；本檔的 MinimalConfig 刻意不載那一整包，所以要自己初始化。
     *
     * <p>⚠️ 少了它的症狀是 {@code IllegalStateException: TsidUtil 尚未初始化}，而它會在
     * <b>交易內</b>拋出 ⇒ 看起來像「建撥款任務失敗所以 rollback」，
     * 與本檔真正要測的原子性失效<b>長得一模一樣</b>。
     */
    @BeforeAll
    static void initTsid() {
        // 🔴 **必須先問 isInitialized()**：TsidUtil 是靜態單例，而 surefire 預設
        //    reuseForks ⇒ 同一個 JVM 會跑完整個模組的測試。本模組已有別的測試
        //    （例如 GrantPaymentPayoutHoldIT 那一系）會先初始化它，重複呼叫會拋
        //    「TsidUtil 已經初始化，不可重複初始化」。
        // ⚠️ 這個 bug 在**單獨跑本檔時完全看不到**（沒有別人先初始化）——
        //    它只在整個模組或 reactor 一起跑時才現形，而那正是 CI 的跑法。
        //    ⇒ 單檔綠不足以宣稱這支測試是好的。
        if (!TsidUtil.isInitialized()) {
            // node-id-base 300 ＝ order-service（10 檔 §7.2）；pod index 0。
            TsidUtil.init(300, 0);
        }
    }

    @BeforeEach
    void reset() {
        jdbc.update("DELETE FROM grant_payment_tasks");
        jdbc.update("DELETE FROM orders");
    }

    /**
     * 建一筆處於指定狀態的訂單。
     *
     * <p>⚠️ {@code orders.id} 是 TSID PK（<b>沒有 DEFAULT</b>）⇒ 必須明給，
     * 漏了會撞 not-null constraint 而不是靜默用 0。
     */
    private void orderAt(String status) {
        jdbc.update("""
                INSERT INTO orders (id, order_no, buyer_id, store_id, status,
                                    total_original_amount, total_actual_amount, recipient_name, created_at)
                VALUES (?, ?, 1, 2, ?, 1000, 1000, '陳小明', NOW())
                """, ORDER_ID, ORDER_NO, status);
    }

    private String statusOf() {
        return jdbc.queryForObject("SELECT status FROM orders WHERE order_no = ?", String.class, ORDER_NO);
    }

    private List<Map<String, Object>> tasks() {
        return jdbc.queryForList("SELECT * FROM grant_payment_tasks WHERE order_no = ?", ORDER_NO);
    }

    // ==================== 正向：唯一入邊 ====================

    @Test
    @DisplayName("35 → 54：狀態轉換與撥款任務在同一交易內都成立")
    void buyerCausedAppliesTerminalAndGrantTask() {
        orderAt(OrderStatus.SHIPPING_ABNORMAL.value());

        assertThat(handler.applyBuyerCausedFinality(ORDER_NO))
                .isEqualTo(ResponsibilityFinalizedTxHandler.Outcome.APPLIED);

        assertThat(statusOf()).isEqualTo(OrderStatus.DELIVERY_FAILED_NO_REFUND.value());
        assertThat(tasks()).hasSize(1);
        assertThat(tasks().get(0).get("status")).isEqualTo(WorkerTaskStatus.PENDING.value());
        // 🔴 confirm_type 必須是 SYSTEM，不得借用 MANUAL／AUTO：
        //    後兩者的語意是「買家收到貨了」，而這條路徑上買家從頭到尾沒有收到貨。
        assertThat(tasks().get(0).get("confirm_type")).isEqualTo("SYSTEM");
    }

    // ==================== 陰性：allowed-from 的實質內容 ====================
    //
    // 🔴 這一組才是「唯一入邊」這個保證的本體（`02-judgment.md` §2.0 第 3 問）：
    //    只驗「35 會進 54」而不驗「非 35 不會進 54」，等於沒有驗到那個保證本身。

    @Test
    @DisplayName("🔴 已送達(40) 不得進 54，且不得建撥款任務")
    void deliveredIsNotAnEntryEdge() {
        assertNotEntryEdge(OrderStatus.DELIVERED.value());
    }

    @Test
    @DisplayName("🔴 已完成(50) 不得進 54（它已經有自己的撥款任務了）")
    void completedIsNotAnEntryEdge() {
        assertNotEntryEdge(OrderStatus.COMPLETED.value());
    }

    @Test
    @DisplayName("🔴 全額退款(70) 不得進 54 —— 那會把「已退款」翻成「不退款」")
    void fullyRefundedIsNotAnEntryEdge() {
        assertNotEntryEdge(OrderStatus.FULLY_REFUNDED.value());
    }

    @Test
    @DisplayName("🔴 已取消(80) 不得進 54")
    void cancelledIsNotAnEntryEdge() {
        assertNotEntryEdge(OrderStatus.CANCELLED.value());
    }

    @Test
    @DisplayName("🔴 配送中(30) 不得進 54 —— 責任判定的前置是訂單已在物流異常")
    void inTransitIsNotAnEntryEdge() {
        assertNotEntryEdge(OrderStatus.IN_TRANSIT.value());
    }

    private void assertNotEntryEdge(String from) {
        orderAt(from);

        assertThat(handler.applyBuyerCausedFinality(ORDER_NO))
                .isEqualTo(ResponsibilityFinalizedTxHandler.Outcome.INCONSISTENT);

        assertThat(statusOf()).isEqualTo(from);
        // 🔴 絕對不得建立撥款任務：那是不可逆的錢。
        assertThat(tasks()).isEmpty();
    }

    // ==================== 冪等與不一致的分辨 ====================

    @Test
    @DisplayName("重送：已是 54 且撥款任務存在 ⇒ ALREADY_FINAL，不建第二筆")
    void replayIsIdempotent() {
        orderAt(OrderStatus.SHIPPING_ABNORMAL.value());
        assertThat(handler.applyBuyerCausedFinality(ORDER_NO))
                .isEqualTo(ResponsibilityFinalizedTxHandler.Outcome.APPLIED);

        assertThat(handler.applyBuyerCausedFinality(ORDER_NO))
                .isEqualTo(ResponsibilityFinalizedTxHandler.Outcome.ALREADY_FINAL);

        assertThat(tasks()).hasSize(1);
    }

    @Test
    @DisplayName("🔴 已是 54 但撥款任務不見了 ⇒ INCONSISTENT，不得當成正常重送靜默放行")
    void terminalWithoutGrantTaskIsInconsistent() {
        orderAt(OrderStatus.SHIPPING_ABNORMAL.value());
        handler.applyBuyerCausedFinality(ORDER_NO);
        // 模擬「狀態轉了但任務沒建」的失效形狀——那正是本 handler 用單一交易在防的事，
        // 不該由它自己的冪等路徑掩蓋掉。
        jdbc.update("DELETE FROM grant_payment_tasks");

        assertThat(handler.applyBuyerCausedFinality(ORDER_NO))
                .isEqualTo(ResponsibilityFinalizedTxHandler.Outcome.INCONSISTENT);
        // 🔴 而且不得「順手補一筆」：那會讓一個已知壞掉的不變式被靜默修掉，沒有人會發現。
        assertThat(tasks()).isEmpty();
    }

    @Test
    @DisplayName("查無訂單 ⇒ INCONSISTENT（不是靜默成功）")
    void unknownOrderIsInconsistent() {
        assertThat(handler.applyBuyerCausedFinality("ONOTEXIST001"))
                .isEqualTo(ResponsibilityFinalizedTxHandler.Outcome.INCONSISTENT);
    }

    // ==================== 原子性 ====================

    @Test
    @DisplayName("🔴 撥款任務 INSERT 失敗 ⇒ 整個交易 rollback，訂單必須留在 35")
    void grantTaskFailureRollsBackTheTransition() {
        orderAt(OrderStatus.SHIPPING_ABNORMAL.value());
        // 讓 uk_grant_payment_order 在 INSERT 當下衝突：先塞一筆同 order_no 的任務。
        // ⚠️ 這是一個「不該發生但發生了」的情境（訂單在 35 卻已有撥款任務）。
        //    重點不在它怎麼來的，而在於**此時整個交易必須 rollback**——
        //    否則訂單會進入無出邊的財務終態，而撥款任務是別人的那一筆。
        jdbc.update("""
                INSERT INTO grant_payment_tasks (task_id, order_no, confirm_type, status,
                                                 retry_count, max_retry_count)
                VALUES (999000000000001, ?, 'MANUAL', ?, 0, 10)
                """, ORDER_NO, WorkerTaskStatus.PENDING.value());

        try {
            handler.applyBuyerCausedFinality(ORDER_NO);
        } catch (RuntimeException expected) {
            // 唯一鍵衝突會讓交易 rollback 並上拋；consumer 會把它送進 DLQ。
        }

        // 🔴 這一行是本測試的實質內容：狀態**沒有**被推進終態。
        assertThat(statusOf()).isEqualTo(OrderStatus.SHIPPING_ABNORMAL.value());
        assertThat(tasks()).hasSize(1);
        assertThat(tasks().get(0).get("task_id")).isEqualTo(999000000000001L);
    }
}

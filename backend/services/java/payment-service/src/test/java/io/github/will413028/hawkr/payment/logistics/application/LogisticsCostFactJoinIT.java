package io.github.will413028.hawkr.payment.logistics.application;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeGrossUp;
import io.github.will413028.hawkr.payment.agentfee.AgentFeeService;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCostFactJoinService.Outcome;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LedgerRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCollectionAttemptRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCostFactRepository;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsReceivableRepository;
import io.github.will413028.hawkr.payment.logistics.repository.SellerLogisticsAccountRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCostFactEntity;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.FulfillmentExceptionCaseMapper;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LedgerEntryMapper;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LedgerTransactionMapper;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsCollectionAttemptMapper;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsCostFactMapper;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.LogisticsReceivableMapper;
import io.github.will413028.hawkr.payment.logistics.repository.mapper.SellerLogisticsAccountMapper;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;
import javax.sql.DataSource;
import org.junit.jupiter.api.AfterEach;
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
 * durable join 收斂點（70 §3.1.4）的 <b>真 DB</b> 行為。
 *
 * <h2>為什麼非真 DB 不可（fake 一件都證明不了）</h2>
 * <ul>
 *   <li><b>{@code uq_costfact_delivery_ref}／{@code uq_logrecv_idem}</b>：重複事實與
 *       重複應收由 DB 的 UNIQUE 擋，不是由應用層的「先查再寫」。fake 的 UNIQUE 是
 *       我自己寫的——它驗的是我對 DDL 的理解，不是 DDL 本身。</li>
 *   <li><b>{@code chk_logrecv_idem_anchor}（V15）</b>：冪等鍵錨錯了會在 INSERT 當下 23514。
 *       這道 CHECK 在 fake 上不存在，而它正是「MQ 至少一次投遞不會重複計費」的保證。</li>
 *   <li><b>並行</b>：兩個 pod 同時判定「齊備」時，只有一筆寫得進去。fake 沒有兩個交易，
 *       兩個 thread 會同時通過。</li>
 * </ul>
 *
 * <p>🔴 <b>service 必須是真的 Spring bean</b>（不是手動 new）：
 * {@link LogisticsCollectionTxService#createReceivableInTx} 靠 {@code @Transactional} proxy，
 * 手動 new 的實例上它完全不生效 ⇒ 測到的會是「沒有交易」那條路徑。
 * 本檔因此在 {@link MinimalConfig} 內以 {@code @Bean} 註冊並開 {@code @EnableTransactionManagement}。
 *
 * <p>⚠️ 本檔<b>只讀不改</b> {@code src/main}：每一組的期望值都是硬寫的字面值，
 * 不從被測物讀回來（{@code 03-delegation-templates.md} §0.3 第二形態）。
 */
@SpringBootTest(classes = LogisticsCostFactJoinIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
@DisplayName("durable join 收斂點的真 DB 行為（A–H）")
class LogisticsCostFactJoinIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("payment_costfact_join_test")
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
        //    不覆寫會讓 Flyway 對本機 no-op、容器內的測試 DB 從未跑過 migration（測試會對著空 schema 綠）。
        registry.add("spring.flyway.url", POSTGRES::getJdbcUrl);
        registry.add("spring.flyway.user", POSTGRES::getUsername);
        registry.add("spring.flyway.password", POSTGRES::getPassword);
        registry.add("spring.flyway.enabled", () -> "true");
        registry.add("spring.flyway.locations", () -> "classpath:db/migration");
        registry.add("spring.flyway.baseline-on-migrate", () -> "false");
    }

    // ── 測試常數（全部字面值，不從被測物推導） ──────────────────────────────────

    /** 賣家業務鍵（🔴 不是 account_id）。 */
    private static final long STORE_ID = 2685914682L;

    /** {@code ShippingMethod} 碼值。冪等鍵與 ledger 的 provider owner_ref 都用它。 */
    private static final String PROVIDER = "3003";

    private static final LocalDateTime OCCURRED_AT = LocalDateTime.of(2026, 9, 17, 3, 0, 0);

    // ── id 產生器 ＋ 建立嘗試計數 ＋（E 組專用）起跑對齊柵欄 ─────────────────────

    private static final AtomicLong RECEIVABLE_IDS = new AtomicLong(760_000_000_000L);
    private static final AtomicLong LEDGER_IDS = new AtomicLong(770_000_000_000L);
    private static final AtomicLong COST_FACT_IDS = new AtomicLong(780_000_000_000L);
    private static final AtomicLong CASE_IDS = new AtomicLong(790_000_000_000L);
    private static final AtomicLong CASE_NO_SEQ = new AtomicLong(1L);

    /**
     * {@code LogisticsCollectionService.createReceivable} 真正被呼叫了幾次。
     *
     * <p>🔴 <b>E 組的鑑別力就在這個數字</b>：它 ＝ 2 而 DB 只有 1 筆，才證明擋住重複的是
     * <b>DB 的 UNIQUE</b>；若它 ＝ 1，代表第二個 thread 在應用層的「先查再寫」就被擋掉了
     * ——那條路徑在真實的兩個 pod 上<b>不存在</b>（各自有各自的快照）。
     */
    private static final AtomicInteger CREATE_ATTEMPTS = new AtomicInteger();

    /**
     * 🔴 E 組專用：讓兩個 thread <b>都通過應用層的存在性檢查之後</b>才各自開交易。
     *
     * <p>柵欄擺在 {@code ids.get()}（在 {@code @Transactional} 之外、在任何寫入之前）
     * ⇒ 沒有任何鎖被持有，不會死結；而「兩者都判定齊備」從機率變成<b>確定</b>。
     * 非 E 組時為 {@code null}，{@link #nextReceivableId()} 直接放行。
     */
    private static final AtomicReference<CyclicBarrier> CREATE_BARRIER = new AtomicReference<>();

    private static long nextReceivableId() {
        CREATE_ATTEMPTS.incrementAndGet();
        CyclicBarrier barrier = CREATE_BARRIER.get();
        if (barrier != null) {
            try {
                barrier.await(15, TimeUnit.SECONDS);
            } catch (Exception ex) {
                throw new IllegalStateException(
                        "E 組柵欄逾時：只有一個 thread 走到建立應收 ⇒ 另一個在應用層就被擋掉了", ex);
            }
        }
        return RECEIVABLE_IDS.incrementAndGet();
    }

    @SpringBootConfiguration
    @EnableTransactionManagement
    @ImportAutoConfiguration({
            DataSourceAutoConfiguration.class,
            DataSourceTransactionManagerAutoConfiguration.class,
            SqlInitializationAutoConfiguration.class,
            FlywayAutoConfiguration.class,
            MybatisPlusAutoConfiguration.class,
            MyBatisPlusAutoConfiguration.class
    })
    @MapperScan("io.github.will413028.hawkr.payment.logistics.repository.mapper")
    static class MinimalConfig {

        @Bean
        LogisticsCostFactRepository costFactRepository(LogisticsCostFactMapper m) {
            return new LogisticsCostFactRepository(m);
        }

        @Bean
        FulfillmentExceptionCaseRepository caseRepository(FulfillmentExceptionCaseMapper m) {
            return new FulfillmentExceptionCaseRepository(m);
        }

        @Bean
        LogisticsReceivableRepository receivableRepository(LogisticsReceivableMapper m) {
            return new LogisticsReceivableRepository(m);
        }

        @Bean
        LogisticsCollectionAttemptRepository attemptRepository(LogisticsCollectionAttemptMapper m) {
            return new LogisticsCollectionAttemptRepository(m);
        }

        @Bean
        SellerLogisticsAccountRepository accountRepository(SellerLogisticsAccountMapper m) {
            return new SellerLogisticsAccountRepository(m);
        }

        @Bean
        LedgerRepository ledgerRepository(LedgerTransactionMapper t, LedgerEntryMapper e) {
            return new LedgerRepository(t, e);
        }

        /** 🔴 必須是 bean：{@code @Transactional} 靠 proxy，手動 new 的實例上完全不生效。 */
        @Bean
        LogisticsCollectionTxService collectionTxService(LogisticsReceivableRepository receivables,
                                                         LogisticsCollectionAttemptRepository attempts,
                                                         SellerLogisticsAccountRepository accounts,
                                                         LedgerRepository ledger) {
            // 🔴 帳本 id 用**另一個**產生器：與 receivable id 共用的話，E 組的柵欄會在交易內
            //    被單邊觸發而卡死（只有真的建了應收的那一方才會 post 帳本）。
            return new LogisticsCollectionTxService(receivables, attempts, accounts, ledger,
                    LEDGER_IDS::incrementAndGet, () -> LocalDateTime.now(ZoneOffset.UTC));
        }

        @Bean
        LogisticsCollectionService collectionService(LogisticsReceivableRepository receivables,
                                                     LogisticsCollectionAttemptRepository attempts,
                                                     LogisticsCollectionTxService txService) {
            // autoCollectionEnabled=false：本檔只走 createReceivable，不碰送出／查詢路徑。
            return new LogisticsCollectionService(receivables, attempts, txService,
                    mock(AgentFeeService.class), mock(AgentFeeGrossUp.class),
                    false, 24,
                    LogisticsCostFactJoinIT::nextReceivableId,
                    () -> LocalDateTime.now(ZoneOffset.UTC));
        }

        @Bean
        LogisticsCostFactJoinService joinService(LogisticsCostFactRepository costFacts,
                                                 FulfillmentExceptionCaseRepository cases,
                                                 LogisticsReceivableRepository receivables,
                                                 LogisticsCollectionService collection) {
            return new LogisticsCostFactJoinService(costFacts, cases, receivables, collection);
        }
    }

    @Autowired
    private LogisticsCostFactJoinService join;

    @Autowired
    private LogisticsCostFactRepository costFacts;

    @Autowired
    private DataSource dataSource;

    private JdbcTemplate jdbc;

    @BeforeEach
    void setUp() {
        jdbc = new JdbcTemplate(dataSource);
        truncate();
        CREATE_ATTEMPTS.set(0);
        CREATE_BARRIER.set(null);
    }

    @AfterEach
    void tearDown() {
        CREATE_BARRIER.set(null);
        truncate();
    }

    /** FK 順序：子表先刪。每個 @Test 前後各一次 ⇒ 執行順序無關。 */
    private void truncate() {
        jdbc.update("DELETE FROM logistics_collection_attempts");
        jdbc.update("DELETE FROM logistics_adjustments");
        jdbc.update("DELETE FROM provider_settlement_lines");
        jdbc.update("DELETE FROM logistics_receivables");
        jdbc.update("DELETE FROM logistics_cost_facts");
        jdbc.update("DELETE FROM ledger_entries");
        jdbc.update("DELETE FROM ledger_transactions");
        jdbc.update("DELETE FROM fulfillment_case_resolutions");
        jdbc.update("DELETE FROM fulfillment_exception_cases");
        jdbc.update("DELETE FROM seller_logistics_accounts");
    }

    // =========================================================================
    // A — 成本先到 → 責任後到
    // =========================================================================

    @Test
    @DisplayName("A. 成本先到 → 責任後到：先 AWAITING_RESPONSIBILITY 且 DB 0 筆，後 RECEIVABLE_CREATED 且恰好 1 筆")
    void a_costFirstThenResponsibility() {
        String ref = "del-a-cost-first";

        // ① 成本先到：事實落庫，責任未定 ⇒ 🔴 不得建應收（70 §3.1.1 未 financially final 期間）
        assertThat(putCostFact(ref, 115L, "evt-a-1")).isTrue();
        assertThat(join.converge(ref)).isEqualTo(Outcome.AWAITING_RESPONSIBILITY);
        assertThat(countReceivables(ref))
                .as("🔴 責任未 final 期間不得建立 Seller receivable —— 直接查 DB，不看回傳值")
                .isEqualTo(0);

        // ② 責任後到
        insertResolvedResponsibilityCase(ref, "BUYER_CAUSED");
        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_CREATED);

        assertThat(countReceivables(ref)).isEqualTo(1);
        Map<String, Object> row = theOnlyReceivable(ref);
        assertThat(((Number) row.get("estimated_cost")).longValue())
                .as("金額必須取自 cost fact，不得由責任側推導")
                .isEqualTo(115L);
        assertThat(row.get("responsibility_cause")).isEqualTo("BUYER_CAUSED");
        assertThat(row.get("status")).isEqualTo("PENDING");
    }

    // =========================================================================
    // B — 責任先到 → 成本後到
    // =========================================================================

    @Test
    @DisplayName("B. 責任先到 → 成本後到：先 AWAITING_COST_FACT 且 DB 0 筆，後 RECEIVABLE_CREATED 且恰好 1 筆")
    void b_responsibilityFirstThenCost() {
        String ref = "del-b-resp-first";

        // ① 責任先到：案件已 RESOLVED，成本未到 ⇒ 🔴 不得在這裡憑空估一個 estimated_cost（§14.1）
        insertResolvedResponsibilityCase(ref, "BUYER_CAUSED");
        assertThat(join.converge(ref)).isEqualTo(Outcome.AWAITING_COST_FACT);
        assertThat(countReceivables(ref))
                .as("🔴 成本未到不得建應收，也不得先建一筆「等成本」的 PENDING")
                .isEqualTo(0);

        // ② 成本後到
        assertThat(putCostFact(ref, 242L, "evt-b-1")).isTrue();
        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_CREATED);

        assertThat(countReceivables(ref)).isEqualTo(1);
        Map<String, Object> row = theOnlyReceivable(ref);
        assertThat(((Number) row.get("estimated_cost")).longValue()).isEqualTo(242L);
        assertThat(row.get("responsibility_cause")).isEqualTo("BUYER_CAUSED");
    }

    // =========================================================================
    // C — 重複成本事實
    // =========================================================================

    @Test
    @DisplayName("C. 重複成本事實：uq_costfact_delivery_ref 擋下第二筆，DB 仍 1 筆且金額是先寫入的那個")
    void c_duplicateCostFactBlockedByUnique() {
        String ref = "del-c-dup-cost";

        assertThat(putCostFact(ref, 300L, "evt-c-1"))
                .as("第一筆是新建")
                .isTrue();

        // 🔴 不同 event_id、不同金額 ⇒ uq_costfact_event 攔不到，只有
        //    uq_costfact_delivery_ref 擋得住。這正是「同一條 leg 被不同事件重複回報」。
        assertThat(putCostFact(ref, 999L, "evt-c-2"))
                .as("🔴 同一條 leg 的第二筆事實必須被 uq_costfact_delivery_ref 擋下（回 false 而非拋）")
                .isFalse();

        assertThat(countCostFacts(ref)).isEqualTo(1);
        assertThat(costFactAmount(ref))
                .as("🔴 事實 append-only：先寫入的金額不得被後到的覆蓋")
                .isEqualTo(300L);

        // 後續收斂拿到的必須是先寫入的那個金額。
        insertResolvedResponsibilityCase(ref, "SELLER_CAUSED");
        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_CREATED);
        assertThat(((Number) theOnlyReceivable(ref).get("estimated_cost")).longValue()).isEqualTo(300L);
    }

    // =========================================================================
    // D — 重複收斂
    // =========================================================================

    @Test
    @DisplayName("D. 重複收斂：converge 連呼三次，DB 內始終恰好 1 筆 receivable")
    void d_repeatedConvergeIsIdempotent() {
        String ref = "del-d-repeat";

        putCostFact(ref, 180L, "evt-d-1");
        insertResolvedResponsibilityCase(ref, "SELLER_CAUSED");

        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_CREATED);
        assertThat(countReceivables(ref)).isEqualTo(1);
        long idAfterFirst = ((Number) theOnlyReceivable(ref).get("id")).longValue();

        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_ALREADY_EXISTS);
        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_ALREADY_EXISTS);

        assertThat(countReceivables(ref))
                .as("🔴 重投三次仍只有一筆 —— 同一趟配送只能收一次錢")
                .isEqualTo(1);
        Map<String, Object> row = theOnlyReceivable(ref);
        assertThat(((Number) row.get("id")).longValue())
                .as("而且是同一列，不是刪掉重建")
                .isEqualTo(idAfterFirst);
        assertThat(((Number) row.get("estimated_cost")).longValue()).isEqualTo(180L);
        assertThat(row.get("responsibility_cause")).isEqualTo("SELLER_CAUSED");
    }

    // =========================================================================
    // E — 並行（本檔最重要的一組）
    // =========================================================================

    @Test
    @DisplayName("E. 並行：兩個真 thread 同時 converge ⇒ 兩者都走到建立、但 DB 內恰好 1 筆（擋住的是 UNIQUE 不是先查再寫）")
    void e_concurrentConvergeCreatesExactlyOne() throws Exception {
        // 🔴 跑 3 輪不同 leg：並行屬時序類，單一樣本只能宣稱「這一次沒事」。
        for (int round = 1; round <= 3; round++) {
            String ref = "del-e-race-" + round;
            putCostFact(ref, 120L + round, "evt-e-" + round);
            insertResolvedResponsibilityCase(ref, "BUYER_CAUSED");

            int attemptsBefore = CREATE_ATTEMPTS.get();
            // 🔴 柵欄在 ids.get()（交易之外）⇒ 兩個 thread 都通過了應用層的存在性檢查
            //    才各自開交易。這讓「兩個 pod 同時判定齊備」從機率變成確定。
            CREATE_BARRIER.set(new CyclicBarrier(2));

            CountDownLatch ready = new CountDownLatch(2);
            CountDownLatch start = new CountDownLatch(1);
            ExecutorService pool = Executors.newFixedThreadPool(2);
            try {
                Callable<Outcome> task = () -> {
                    ready.countDown();
                    start.await(15, TimeUnit.SECONDS);
                    return join.converge(ref);
                };
                Future<Outcome> f1 = pool.submit(task);
                Future<Outcome> f2 = pool.submit(task);

                assertThat(ready.await(15, TimeUnit.SECONDS)).isTrue();
                start.countDown();

                List<Outcome> outcomes = new ArrayList<>();
                outcomes.add(f1.get(60, TimeUnit.SECONDS));
                outcomes.add(f2.get(60, TimeUnit.SECONDS));

                assertThat(CREATE_ATTEMPTS.get() - attemptsBefore)
                        .as("🔴 round %d：兩個 thread 都必須真的走到建立應收 —— "
                                + "否則這一輪驗的是應用層的先查再寫，不是 DB 的 UNIQUE", round)
                        .isEqualTo(2);

                assertThat(countReceivables(ref))
                        .as("🔴 round %d：兩個並行建立，DB 內必須恰好 1 筆（uq_logrecv_idem）", round)
                        .isEqualTo(1);

                assertThat(outcomes)
                        .as("round %d：恰好一個 CREATED、一個認出已存在", round)
                        .containsExactlyInAnyOrder(
                                Outcome.RECEIVABLE_CREATED, Outcome.RECEIVABLE_ALREADY_EXISTS);

                Map<String, Object> row = theOnlyReceivable(ref);
                assertThat(((Number) row.get("estimated_cost")).longValue()).isEqualTo(120L + round);
                assertThat(row.get("responsibility_cause")).isEqualTo("BUYER_CAUSED");
            } finally {
                CREATE_BARRIER.set(null);
                pool.shutdownNow();
            }
        }

        // 🔴 帳本不得被記兩次：LOGISTICS_RECEIVABLE_ACCRUAL 一條 leg 一筆。
        assertThat(jdbc.queryForObject(
                "SELECT count(*) FROM ledger_transactions WHERE txn_type = 'LOGISTICS_RECEIVABLE_ACCRUAL'",
                Integer.class))
                .as("🔴 三輪各一筆應收 ⇒ 帳本恰好三筆認列，不得因並行而重複認列收入")
                .isEqualTo(3);
    }

    // =========================================================================
    // F — counterparty ≠ SELLER
    // =========================================================================

    @Test
    @DisplayName("F. PROVIDER_CAUSED ＋ 成本已到 ⇒ NO_SELLER_RECEIVABLE，DB 內 0 筆")
    void f_nonSellerCounterpartyCreatesNothing() {
        String ref = "del-f-provider";

        putCostFact(ref, 500L, "evt-f-1");
        // PROVIDER_CAUSED ⇒ Bearer.DISPUTE（ADR-004）⇒ 🔴 不得自動向賣家收。
        insertResolvedResponsibilityCase(ref, "PROVIDER_CAUSED");

        assertThat(join.converge(ref)).isEqualTo(Outcome.NO_SELLER_RECEIVABLE);
        assertThat(countReceivables(ref))
                .as("🔴 陰性保證：counterparty 不是賣家時，一列都不准建")
                .isEqualTo(0);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM logistics_receivables", Integer.class))
                .as("全表也必須是 0（不是「建在別的 ref 上」）")
                .isEqualTo(0);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM ledger_transactions", Integer.class))
                .as("🔴 沒有應收就沒有認列，帳上不得憑空多一筆收入")
                .isEqualTo(0);

        // 成本事實本身仍在（它是事實，與歸屬無關）。
        assertThat(countCostFacts(ref)).isEqualTo(1);
    }

    // =========================================================================
    // G — 衝突
    // =========================================================================

    @Test
    @DisplayName("🔴 I. 案件已 RESOLVED 但 resolution_code 非法 ⇒ CONFLICT，且零收款副作用")
    void i_resolvedWithCorruptCauseIsConflictWithNoFinancialSideEffect() {
        String ref = "del-i-corrupt";

        // ① 成本事實已到（收斂的另一半條件成立）。
        putCostFact(ref, 480L, "evt-i-1");

        // ② 案件宣告 RESOLVED，但成因是一個**不存在**的值。
        //    🔴 這一列是用直接 SQL 造的——V12 的 chk_exc_responsibility_resolved_cause
        //    擋的是**寫入路徑**，而本測試要驗的正是「CHECK 加上之前就存在的舊列」
        //    或「一次繞過應用層的維運 SQL」留下的資料。
        //    ⚠️ 假設所有歷史資料都必然通過 CHECK，就是假設這種列不存在——
        //    而它一旦存在，舊行為會把它當成「還在等」。
        insertResolvedResponsibilityCase(ref, "BUYER_CAUSED");

        // 🔴 **必須暫時卸下 CHECK 才造得出這種列**——而那正是本測試的重點：
        //    V12 的 chk_exc_responsibility_resolved_cause 是**第一道**防線，它擋得住寫入；
        //    但它擋不住「CHECK 加上之前就存在的舊列」。
        //    ⚠️ 假設所有歷史資料都必然通過 CHECK，就是假設這種列不存在——
        //    而它一旦存在，應用層的行為才是最後一道防線。本測試驗的就是那一道。
        //
        //    ⚠️ 這個 DROP/ADD 只在測試容器內、只在本測試的交易外進行，
        //    結束前一定加回來（見下方 finally）。
        jdbc.execute("ALTER TABLE fulfillment_exception_cases"
                + " DROP CONSTRAINT chk_exc_responsibility_resolved_cause");
        try {
            assertThat(jdbc.update(
                    "UPDATE fulfillment_exception_cases SET resolution_code = 'SYSTEM_CAUSED'"
                            + " WHERE subject_ref = ?", ref))
                    .isEqualTo(1);
        } finally {
            // 🔴 立刻加回：後面的斷言要在「CHECK 仍然存在」的世界裡跑，
            //    否則本測試會污染同一個容器裡的其他測試。
            jdbc.execute("ALTER TABLE fulfillment_exception_cases"
                    + " ADD CONSTRAINT chk_exc_responsibility_resolved_cause CHECK ("
                    + "  category <> 'RESPONSIBILITY_UNRESOLVED'"
                    + "  OR status <> 'RESOLVED'"
                    + "  OR resolution_code IN ('BUYER_CAUSED','SELLER_CAUSED',"
                    + "                         'PROVIDER_CAUSED','PLATFORM_CAUSED')"
                    + ") NOT VALID");
        }

        // ③ 🔴 必須是 CONFLICT，**不是** AWAITING_RESPONSIBILITY。
        //    舊行為回 AWAITING ⇒ consumer 走成功路徑 ⇒ 這筆損壞的 financial finality
        //    就此無聲消失，而下游（退款／應收／KPI）全都以為「還在處理中」。
        assertThat(join.converge(ref))
                .as("🔴 案件已宣告定案卻沒有合法成因 ＝ financial finality 已損壞，"
                        + "絕不能當成『還沒判』")
                .isEqualTo(Outcome.CONFLICT);

        // ④ 🔴 陰性：零收款副作用。
        assertThat(countReceivables(ref))
                .as("🔴 不得建立應收——金額與負擔方都還不可知")
                .isZero();
        assertThat(jdbc.queryForObject(
                "SELECT count(*) FROM ledger_transactions WHERE txn_type = ?",
                Integer.class, "LOGISTICS_RECEIVABLE_ACCRUAL"))
                .as("🔴 不得認列任何收入")
                .isZero();

        // ⑤ 成本事實本身仍在（它沒有錯，錯的是責任側）。
        assertThat(jdbc.queryForObject(
                "SELECT count(*) FROM logistics_cost_facts WHERE provider_delivery_ref = ?",
                Integer.class, ref))
                .as("成本事實不該被牽連刪除——它是獨立的事實")
                .isEqualTo(1);

        // ⑥ 重跑仍是 CONFLICT（不會因為重試就變成別的結果）。
        assertThat(join.converge(ref)).isEqualTo(Outcome.CONFLICT);
        assertThat(countReceivables(ref)).isZero();
    }

    @Test
    @DisplayName("G. 已有賣家應收而責任判定為 PLATFORM_CAUSED ⇒ CONFLICT，且該列未被修改")
    void g_existingReceivableWithNonSellerCounterpartyConflicts() {
        String ref = "del-g-conflict";

        // ① 先以正常路徑建出一筆應收（成因 BUYER_CAUSED ⇒ 負擔方是賣家）。
        putCostFact(ref, 640L, "evt-g-1");
        insertResolvedResponsibilityCase(ref, "BUYER_CAUSED");
        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_CREATED);

        Map<String, Object> before = theOnlyReceivable(ref);
        assertThat(before.get("responsibility_cause")).isEqualTo("BUYER_CAUSED");
        long idBefore = ((Number) before.get("id")).longValue();
        Object updatedAtBefore = before.get("updated_at");

        // ② 案件被改判成 PLATFORM_CAUSED（負擔方 ＝ 平台）⇒ 正在向不該付的人收錢。
        assertThat(jdbc.update(
                "UPDATE fulfillment_exception_cases SET resolution_code = 'PLATFORM_CAUSED'"
                        + " WHERE subject_ref = ?", ref))
                .isEqualTo(1);

        assertThat(join.converge(ref))
                .as("🔴 不得靜默 ACK：leg 上已有賣家應收，而責任說這筆不該由賣家承擔")
                .isEqualTo(Outcome.CONFLICT);

        // ③ 🔴 CONFLICT 不得順手「修正」那一列——改判要走 adjustment，不是覆蓋既有快照。
        Map<String, Object> after = theOnlyReceivable(ref);
        assertThat(after.get("responsibility_cause"))
                .as("🔴 既有快照必須原封不動（覆蓋會讓「改判」看起來像正常更新）")
                .isEqualTo("BUYER_CAUSED");
        assertThat(((Number) after.get("id")).longValue()).isEqualTo(idBefore);
        assertThat(((Number) after.get("estimated_cost")).longValue()).isEqualTo(640L);
        assertThat(after.get("status")).isEqualTo("PENDING");
        assertThat(after.get("updated_at"))
                .as("連 updated_at 都不該動 —— 這一次收斂沒有寫入任何東西")
                .isEqualTo(updatedAtBefore);
        assertThat(countReceivables(ref)).isEqualTo(1);
    }

    // =========================================================================
    // H — 冪等鍵錨點
    // =========================================================================

    @Test
    @DisplayName("H. 冪等鍵逐字等於 logistics_receivable:{provider}:{ref}，且別的格式會在 INSERT 當下 23514")
    void h_idempotencyKeyAnchoredOnProviderLeg() {
        String ref = "del-h-idem";

        putCostFact(ref, 77L, "evt-h-1");
        insertResolvedResponsibilityCase(ref, "SELLER_CAUSED");
        assertThat(join.converge(ref)).isEqualTo(Outcome.RECEIVABLE_CREATED);

        // 🔴 期望值是完全硬寫的字面值，不呼叫 CostIncurred.idempotencyKey()
        //    （期望值跟著被測物一起變 ⇒ 它驗不到任何東西）。
        assertThat(theOnlyReceivable(ref).get("idempotency_key"))
                .isEqualTo("logistics_receivable:3003:del-h-idem");

        // 🔴 鑑別力：證明守住這個格式的是 chk_logrecv_idem_anchor（V15），不是巧合。
        //    用舊格式 'logistics_receivable:{shipment_id}:{leg}' 直接 INSERT ⇒ 23514。
        assertThatThrownBy(() -> jdbc.update("""
                INSERT INTO logistics_receivables (
                    id, store_id, order_no, shipment_id, provider, provider_delivery_ref,
                    estimated_cost, platform_subsidy, target_net, carried_residual,
                    status, charge_basis_frozen_at, idempotency_key
                ) VALUES (?, ?, ?, NULL, ?, ?, 77, 0, 77, 0, 'PENDING', NOW(), ?)
                """,
                RECEIVABLE_IDS.incrementAndGet(), STORE_ID, orderNoFor(ref), PROVIDER,
                ref + "-other-leg", "logistics_receivable:12345:FORWARD"))
                .as("🔴 錨錯了必須在 INSERT 當下現形，而不是等到對帳時金額對不上")
                .hasMessageContaining("chk_logrecv_idem_anchor");

        assertThat(jdbc.queryForObject("SELECT count(*) FROM logistics_receivables", Integer.class))
                .as("被擋下的那一筆不得留下任何痕跡")
                .isEqualTo(1);
    }

    // ── helpers ───────────────────────────────────────────────────────────────

    /** 寫一筆成本事實（走 production 的 {@code insertIfAbsent}，不是裸 SQL）。 */
    private boolean putCostFact(String ref, long cost, String eventId) {
        LogisticsCostFactEntity f = new LogisticsCostFactEntity();
        f.setId(COST_FACT_IDS.incrementAndGet());
        f.setProvider(PROVIDER);
        f.setProviderDeliveryRef(ref);
        f.setStoreId(STORE_ID);
        f.setOrderNo(orderNoFor(ref));
        f.setShipmentId(null);
        f.setEstimatedCost(cost);
        f.setOccurredAt(OCCURRED_AT);
        f.setEventId(eventId);
        return costFacts.insertIfAbsent(f);
    }

    /**
     * 建一張 <b>financially final</b> 的責任判定案件（裸 SQL：本檔測的是 join，
     * 不是 maker-checker 那條軸；那條軸的行為在 {@code ResponsibilityCaseFinalityIntegrationTest}）。
     *
     * <p>欄位齊備是 DDL 強制的：{@code chk_exc_owner}（離開 OPEN ⇒ 必有承辦人）、
     * {@code chk_exc_resolved}（結案三件套）、{@code chk_exc_responsibility_context}
     * （V14：責任軸的 order_no／shipping_method 必填）。
     */
    private void insertResolvedResponsibilityCase(String ref, String cause) {
        long id = CASE_IDS.incrementAndGet();
        String caseNo = String.format("EX20260917%08d", CASE_NO_SEQ.getAndIncrement());
        jdbc.update("""
                INSERT INTO fulfillment_exception_cases (
                    id, case_no, source_service, category, severity,
                    subject_type, subject_ref, store_id, status, status_entered_at,
                    owner_account_id, owner_assigned_at, dedup_key,
                    resolution_code, resolution_note, resolved_by_account_id, resolved_at,
                    occurred_at, order_no, shipping_method
                ) VALUES (
                    ?, ?, 'payment', 'RESPONSIBILITY_UNRESOLVED', 'P1',
                    'SHIPMENT', ?, ?, 'RESOLVED', NOW(),
                    9001, NOW(), ?,
                    ?, '整合測試 fixture', 9002, NOW(),
                    NOW(), ?, ?
                )
                """,
                id, caseNo, ref, STORE_ID,
                "RESPONSIBILITY_UNRESOLVED:SHIPMENT:" + ref,
                cause, orderNoFor(ref), PROVIDER);
    }

    /** {@code order_no} 是 varchar(32)；本檔只要求穩定且不撞。 */
    private static String orderNoFor(String ref) {
        return "O" + Integer.toHexString(ref.hashCode()).toUpperCase();
    }

    private int countReceivables(String ref) {
        return jdbc.queryForObject(
                "SELECT count(*) FROM logistics_receivables WHERE provider_delivery_ref = ?",
                Integer.class, ref);
    }

    private int countCostFacts(String ref) {
        return jdbc.queryForObject(
                "SELECT count(*) FROM logistics_cost_facts WHERE provider_delivery_ref = ?",
                Integer.class, ref);
    }

    private long costFactAmount(String ref) {
        return jdbc.queryForObject(
                "SELECT estimated_cost FROM logistics_cost_facts WHERE provider_delivery_ref = ?",
                Long.class, ref);
    }

    /** 🔴 每一組的斷言都直接查 DB —— 回傳值對了而 DB 沒寫進去，正是本檔要抓的東西。 */
    private Map<String, Object> theOnlyReceivable(String ref) {
        List<Map<String, Object>> rows = jdbc.queryForList(
                "SELECT * FROM logistics_receivables WHERE provider_delivery_ref = ?", ref);
        assertThat(rows).as("該 leg 應恰好一筆應收").hasSize(1);
        return rows.get(0);
    }
}

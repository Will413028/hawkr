package io.github.will413028.hawkr.payment.logistics.application;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxPublisher;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityCaseRules;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentCaseResolutionRepository;
import io.github.will413028.hawkr.payment.logistics.repository.FulfillmentExceptionCaseRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentCaseResolutionEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentExceptionCaseEntity;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.SQLException;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.concurrent.Callable;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import javax.sql.DataSource;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
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
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.EnableTransactionManagement;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

/**
 * 責任判定核心 finality 的 <b>真 DB</b> 行為（WP2A）。
 *
 * <p><b>為什麼非真 DB 不可</b>：本檔要證明的三件事，mock 一件都證明不了——
 * <ul>
 *   <li><b>交易原子性</b>：rollback 之後那幾列<b>不存在</b>。mock 只能證明「我呼叫了 insert」。</li>
 *   <li><b>並發序列化</b>：{@code SELECT … FOR UPDATE} 真的把兩個交易排成一列。
 *       mock 的 repository 沒有鎖，兩個 thread 會同時通過。</li>
 *   <li><b>唯一約束</b>：lifetime UNIQUE 在兩個並行 INSERT 下只讓一個成功。</li>
 * </ul>
 *
 * <p>🔴 <b>service 必須是真的 Spring bean</b>（不是手動 new）：{@code @Transactional} 靠 proxy，
 * 手動 new 的實例上它完全不生效 ⇒ 測到的會是「沒有交易」那條路徑，而那正是我們要防的東西。
 * 本檔因此在 {@link MinimalConfig} 內以 {@code @Bean} 註冊並開 {@code @EnableTransactionManagement}。
 */
@SpringBootTest(classes = ResponsibilityCaseFinalityIntegrationTest.MinimalConfig.class)
@Testcontainers
@Tag("integration")
@DisplayName("責任判定 finality 的真 DB 行為")
class ResponsibilityCaseFinalityIntegrationTest {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("payment_responsibility_test")
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

    /** 測試用的 id 產生器：遞增即可，重點是唯一（TSID 的實際實作不是本檔的被測物）。 */
    private static final AtomicLong IDS = new AtomicLong(700_000_000_000L);

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
    @MapperScan({"io.github.will413028.hawkr.payment.infrastructure.persistence.mapper",
            "io.github.will413028.hawkr.payment.logistics.repository.mapper"})
    static class MinimalConfig {

        @Bean
        FulfillmentExceptionCaseRepository caseRepository(
                io.github.will413028.hawkr.payment.logistics.repository.mapper.FulfillmentExceptionCaseMapper m) {
            return new FulfillmentExceptionCaseRepository(m);
        }

        @Bean
        FulfillmentCaseResolutionRepository resolutionRepository(
                io.github.will413028.hawkr.payment.logistics.repository.mapper.FulfillmentCaseResolutionMapper m) {
            return new FulfillmentCaseResolutionRepository(m);
        }

        @Bean
        PaymentOutboxWriter outboxWriter(OutboxEventMapper m) {
            return new PaymentOutboxWriter(m, new ObjectMapper(),
                    mock(PaymentOutboxPublisher.class), new SimpleMeterRegistry());
        }

        @Bean
        ResponsibilityCaseService responsibilityCaseService(FulfillmentExceptionCaseRepository cases,
                                                            FulfillmentCaseResolutionRepository resolutions,
                                                            PaymentOutboxWriter writer) {
            return new ResponsibilityCaseService(cases, resolutions, writer,
                    mock(PaymentOutboxPublisher.class), new ObjectMapper(),
                    IDS::incrementAndGet, () -> LocalDateTime.now(ZoneOffset.UTC));
        }
    }

    @Autowired
    private ResponsibilityCaseService service;

    @Autowired
    private DataSource dataSource;

    @Autowired
    private PlatformTransactionManager txManager;

    @Autowired
    private PaymentOutboxWriter outboxWriter;

    @Autowired
    private FulfillmentExceptionCaseRepository cases;

    private JdbcTemplate jdbc;
    private TransactionTemplate tx;

    private static final String LEG = "leg-integration-1";
    private static final String ORDER_NO = "O20260916000001";
    private static final long MAKER = 9001L;
    private static final long CHECKER = 9002L;

    @BeforeEach
    void setUp() {
        jdbc = new JdbcTemplate(dataSource);
        tx = new TransactionTemplate(txManager);
        truncate();
    }

    @AfterEach
    void tearDown() {
        truncate();
    }

    private void truncate() {
        jdbc.update("DELETE FROM fulfillment_case_resolutions");
        jdbc.update("DELETE FROM fulfillment_exception_cases");
        jdbc.update("DELETE FROM outbox_events");
    }

    // ── helpers ───────────────────────────────────────────────────────────

    /**
     * 🔴 超過 {@code 2^53}（9,007,199,254,740,992）的 TSID —— wire 序列化測試的核心輸入。
     *
     * <p>用小數字（例如 12345）測 {@code shipment_id} 的型別<b>零鑑別力</b>：
     * JSON number 與 quoted string 在那個範圍內往返都不會失真，
     * 而真正的 TSID 正好落在 float64 精確整數範圍之外——那才是 65 要防的東西。
     */
    private static final long BIG_TSID = 7_200_000_000_000_000_001L;

    /**
     * 權威全額退款基準 ＝ {@code orders.total_actual_amount}（單位「元」）。
     *
     * <p>🔴 測試裡它是<b>由呼叫端在交易外讀好後傳入</b>的，與生產路徑同形
     * （Phase A 在 WP2B 的 Admin handler，Phase B 在本 service）。
     */
    private static final long FULL_REFUND = 1_200L;

    private ResponsibilityCaseService.OpenUnresolvedCommand openCmd(String leg) {
        return openCmd(leg, 12345L);
    }

    private ResponsibilityCaseService.OpenUnresolvedCommand openCmd(String leg, Long shipmentId) {
        return new ResponsibilityCaseService.OpenUnresolvedCommand(
                leg, "payment", 2685914682L, ORDER_NO, shipmentId, "3003", "{}",
                LocalDateTime.now(ZoneOffset.UTC));
    }

    /** 預設提案：maker 主張 {@code BUYER_CAUSED}。 */
    private ResponsibilityCaseService.ProposeCommand proposeCmd(long caseId) {
        return proposeCmd(caseId, "BUYER_CAUSED");
    }

    /**
     * 🔴 <b>成因在這裡給，不在 finalize</b>（V13）：它是 maker 的主張，
     * 覆核者只能 APPROVE／REJECT 它。
     */
    private ResponsibilityCaseService.ProposeCommand proposeCmd(long caseId, String proposedCause) {
        return new ResponsibilityCaseService.ProposeCommand(
                caseId, ResponsibilityCaseRules.RESOLUTION_TYPE, proposedCause,
                "外送員回報收件人不在", MAKER);
    }

    /**
     * 🔴 <b>刻意只有 caseId 與 resolutionId</b>（V13 ＋ V14）。
     *
     * <p>覆核者提供不了成因（V13：來自提案列），也提供不了 order_no／shipment_id／
     * shipping_method（V14：來自建案當下凍結的 case row）。
     * 想改這些值只能回頭改 {@link #proposeCmd(long, String)} 或 {@link #openCmd(String, Long)}
     * ——而那正是本測試檔要守住的東西。
     */
    private ResponsibilityCaseService.FinalizeCommand finalizeCmd(long caseId, long resolutionId) {
        return new ResponsibilityCaseService.FinalizeCommand(
                caseId, resolutionId, "證據充分", CHECKER);
    }

    /** 某一筆提案上，maker 主張的成因（V13 持久化欄位）。 */
    private String proposedCauseOf(long resolutionId) {
        return jdbc.queryForObject(
                "SELECT proposed_responsibility_cause FROM fulfillment_case_resolutions WHERE id = ?",
                String.class, resolutionId);
    }

    /** 案件的結案碼。 */
    private String resolutionCodeOf(long caseId) {
        return jdbc.queryForObject(
                "SELECT resolution_code FROM fulfillment_exception_cases WHERE id = ?",
                String.class, caseId);
    }

    /** outbox payload 的某個欄位值（字串）。 */
    private String payloadFieldOf(String routingKey, String field) {
        return jdbc.queryForObject(
                "SELECT payload ->> ? FROM outbox_events WHERE routing_key = ?",
                String.class, field, routingKey);
    }

    /** 直接問 PostgreSQL「這個 jsonb 值是什麼型別」——比字串比對更難被格式差異糊弄。 */
    private String jsonTypeOf(String routingKey, String field) {
        return jdbc.queryForObject(
                "SELECT jsonb_typeof(payload -> ?) FROM outbox_events WHERE routing_key = ?",
                String.class, field, routingKey);
    }

    private long countCases() {
        return jdbc.queryForObject("SELECT count(*) FROM fulfillment_exception_cases", Long.class);
    }

    private long countResolutions() {
        return jdbc.queryForObject("SELECT count(*) FROM fulfillment_case_resolutions", Long.class);
    }

    private long countOutbox(String routingKey) {
        return jdbc.queryForObject(
                "SELECT count(*) FROM outbox_events WHERE routing_key = ?", Long.class, routingKey);
    }

    private String caseStatus(long caseId) {
        return jdbc.queryForObject(
                "SELECT status FROM fulfillment_exception_cases WHERE id = ?", String.class, caseId);
    }

    private String resolutionStatus(long resolutionId) {
        return jdbc.queryForObject(
                "SELECT status FROM fulfillment_case_resolutions WHERE id = ?", String.class, resolutionId);
    }

    /**
     * 直接下 SQL 建案（{@link ResponsibilityCaseService} 只管責任 category，generic 對照組得自己建）。
     *
     * <p>⚠️ {@code chk_exc_owner}：離開 {@code OPEN} ⇒ 必須有承辦人與指派時間；
     * {@code chk_exc_resolved}：結案三件套必須同時有值。兩者在同一句寫完。
     */
    private void insertCase(long id, String caseNo, String category, String status,
                            String subjectRef, String resolutionCode, String dedupKey) {
        boolean closed = "RESOLVED".equals(status) || "DISMISSED".equals(status);
        // 🔴 責任 category 才補 event context（V14 的 chk_exc_responsibility_context）。
        //    ⚠️ 少了這個條件，責任探針會被那道 CHECK **先一步**擋下，於是測試問不到它
        //    真正想問的那一道（例如 e5 的 chk_exc_responsibility_no_dismiss）
        //    ——探針必須先滿足前一道約束，才問得到後一道。
        //    ⚠️ 反過來也不能無條件補：generic 案件三欄必須維持 null，那是 Ctx9 要守的東西。
        boolean responsibility = ResponsibilityCaseRules.CATEGORY.equals(category);
        jdbc.update("INSERT INTO fulfillment_exception_cases "
                        + "(id, case_no, source_service, category, severity, subject_type, subject_ref, status,"
                        + " owner_account_id, owner_assigned_at, dedup_key, detail, proposed_action_codes,"
                        + " resolution_code, resolved_by_account_id, resolved_at, occurred_at,"
                        + " order_no, shipping_method) VALUES "
                        + "(?, ?, 'payment', ?, 'P1', 'SHIPMENT', ?, ?, ?, "
                        + ("OPEN".equals(status) ? "NULL" : "NOW()")
                        + ", ?, '{}', '[]', ?, ?, "
                        + (closed ? "NOW()" : "NULL") + ", NOW(), ?, ?)",
                id, caseNo, category, subjectRef, status,
                "OPEN".equals(status) ? null : 9001L,
                dedupKey,
                closed ? resolutionCode : null,
                closed ? 9002L : null,
                responsibility ? ORDER_NO : null,
                responsibility ? "3003" : null);
    }

    /**
     * 用<b>獨立的 connection</b> 嘗試以 {@code FOR UPDATE NOWAIT} 鎖住某一列。
     *
     * <p>🔴 必須繞過 Spring 的 connection 綁定（{@code dataSource.getConnection()} 而不是
     * {@code JdbcTemplate}）：同一個交易內再鎖一次自己已持有的列一定會成功，那樣測不到任何東西。
     *
     * @return {@code true} = 拿到鎖；{@code false} = 被 {@code 55P03}（lock_not_available）拒絕
     * @throws SQLException 其他任何 SQL 錯誤 —— 🔴 <b>不吞</b>，否則「SQL 寫錯」會被誤讀成「鎖住了」
     */
    private boolean tryLockNoWait(long caseId) throws SQLException {
        try (Connection c = dataSource.getConnection()) {
            c.setAutoCommit(false);
            try (PreparedStatement ps = c.prepareStatement(
                    "SELECT id FROM fulfillment_exception_cases WHERE id = ? FOR UPDATE NOWAIT")) {
                ps.setLong(1, caseId);
                ps.execute();
                c.rollback();
                return true;
            } catch (SQLException e) {
                c.rollback();
                if ("55P03".equals(e.getSQLState())) {
                    return false;
                }
                throw e;
            }
        }
    }

    /** {@code exception_case_no_seq} 的 {@code last_value} —— sequence 不受 rollback 影響，是「碰過 DB」的痕跡。 */
    private long caseNoSeqLastValue() {
        return jdbc.queryForObject("SELECT last_value FROM exception_case_no_seq", Long.class);
    }

    /** payload 欄是 jsonb，取回時鍵序與空白已被 PostgreSQL 正規化 ⇒ 一律解析後比對。 */
    @SuppressWarnings("unchecked")
    private static Map<String, Object> readJson(String json) {
        try {
            return new ObjectMapper().readValue(json, Map.class);
        } catch (Exception e) {
            throw new IllegalStateException("payload 不是合法 JSON：" + json, e);
        }
    }

    /** 兩個動作在各自的執行緒、各自的交易裡同時起跑。 */
    private <T> List<T> runConcurrently(Callable<T> a, Callable<T> b) throws Exception {
        ExecutorService pool = Executors.newFixedThreadPool(2);
        try {
            CountDownLatch start = new CountDownLatch(1);
            Future<T> fa = pool.submit(() -> {
                start.await();
                return a.call();
            });
            Future<T> fb = pool.submit(() -> {
                start.await();
                return b.call();
            });
            start.countDown();
            return List.of(fa.get(30, TimeUnit.SECONDS), fb.get(30, TimeUnit.SECONDS));
        } finally {
            pool.shutdownNow();
        }
    }

    /** 把 callable 的例外收斂成一個標記字串，讓並行測試可以斷言「兩者之一失敗」。 */
    private static Callable<String> quiet(Callable<String> c) {
        return () -> {
            try {
                return c.call();
            } catch (Exception e) {
                return "EX:" + e.getClass().getSimpleName();
            }
        };
    }

    // ═══════════════════ T 群：交易原子性 ═══════════════════

    @Nested
    @DisplayName("T 群：交易原子性")
    class Atomicity {

        /**
         * T1：建案 ＋ N17 同生共死。
         *
         * <p>以外層交易 {@code setRollbackOnly} 模擬「業務交易在後續步驟失敗」——
         * service 的 {@code @Transactional} 是 {@code REQUIRED}，會加入外層交易
         * ⇒ 外層 rollback，case 與 outbox 必須一起消失。
         * <p>⚠️ 刻意<b>不</b>在 production service 上加測試專用的 fail hook：那會讓
         * 被測物與正式路徑不同，而這支測試要證明的正是<b>正式路徑</b>的原子性。
         */
        @Test
        @DisplayName("T1 建案交易 rollback ⇒ case 與 N17 都不存在")
        void t1_openRollbackLeavesNothing() {
            tx.execute(status -> {
                var opened = service.openUnresolved(openCmd(LEG));
                assertTrue(opened.created(), "前提：這一次真的建立了案件");
                status.setRollbackOnly();
                return null;
            });

            assertEquals(0, countCases(), "case 不該留下");
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED),
                    "🔴 N17 不該留下——否則會發出一個「沒有發生過的事」");
        }

        /** T2：finalization 四步同生共死。 */
        @Test
        @DisplayName("T2 定案交易 rollback ⇒ resolution 仍 AWAITING_REVIEW、case 未 RESOLVED、N16 不存在")
        void t2_finalizeRollbackRestoresEverything() {
            var opened = service.openUnresolved(openCmd(LEG));
            long resolutionId = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);

            tx.execute(status -> {
                var r = service.finalizeResponsibility(
                        finalizeCmd(opened.caseId(), resolutionId), FULL_REFUND);
                assertEquals(ResponsibilityCaseService.Outcome.APPLIED, r.outcome(),
                        "前提：這一次真的定案了（四步都成功）");
                status.setRollbackOnly();
                return null;
            });

            assertEquals(ResponsibilityCaseRules.RESOLUTION_AWAITING_REVIEW, resolutionStatus(resolutionId),
                    "resolution 應回到 AWAITING_REVIEW");
            assertEquals(ResponsibilityCaseRules.STATUS_IN_PROGRESS, caseStatus(opened.caseId()),
                    "case 不該停在 RESOLVED");
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED),
                    "🔴 N16 不該留下");
        }

        /**
         * T3：outbox enqueue 失敗 ⇒ domain transition 不得 commit。
         *
         * <p>以「不在交易內呼叫」觸發 {@code PaymentOutboxWriter} 的 fail-fast 閘門
         * ——那是它對 caller 唯一的硬性要求，也是 transactional outbox 的存在意義本身。
         */
        @Test
        @DisplayName("T3 沒有 active transaction 時 enqueue 直接 fail-fast，任何副作用之前")
        void t3_enqueueWithoutTransactionFailsFast() {
            IllegalStateException ex = assertThrows(IllegalStateException.class,
                    () -> outboxWriter.enqueue(MqConstants.EXCHANGE_SHIPPING,
                            MqConstants.EVENT_TYPE_FULFILLMENT_RESPONSIBILITY_UNRESOLVED,
                            MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED, "{}"));
            assertTrue(ex.getMessage().contains("active transaction"),
                    "應該是 outbox 的交易閘門擋下的，實際：" + ex.getMessage());
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED),
                    "閘門必須在 insert 之前擋下");
        }
    }

    // ═══════════════════ C 群：並發 ═══════════════════

    @Nested
    @DisplayName("C 群：並發序列化")
    class Concurrency {

        /** C1：兩個交易同時對同一 leg 建案。 */
        @Test
        @DisplayName("C1 並行建案 ⇒ 只有一張 case、只有一個 N17")
        void c1_concurrentOpenProducesOneCaseOneEvent() throws Exception {
            List<String> r = runConcurrently(
                    quiet(() -> String.valueOf(service.openUnresolved(openCmd(LEG)).created())),
                    quiet(() -> String.valueOf(service.openUnresolved(openCmd(LEG)).created())));

            assertEquals(1, countCases(), "🔴 同一 leg 只能有一張責任案件（lifetime UNIQUE）");
            assertEquals(1, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED),
                    "🔴 只有真的寫進一列的那一次才發 N17；另一次必須是 no-op");
            assertEquals(1, r.stream().filter("true"::equals).count(),
                    "恰好一個 created=true，實際：" + r);
        }

        /** C2：兩個交易同時 finalize 同一個 resolution。 */
        @Test
        @DisplayName("C2 並行定案 ⇒ 一次 APPLIED、一次 STALE，只有一個 N16")
        void c2_concurrentFinalizeProducesOneEvent() throws Exception {
            var opened = service.openUnresolved(openCmd(LEG));
            long resolutionId = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);

            List<String> r = runConcurrently(
                    quiet(() -> service.finalizeResponsibility(
                            finalizeCmd(opened.caseId(), resolutionId), FULL_REFUND).outcome().name()),
                    quiet(() -> service.finalizeResponsibility(
                            finalizeCmd(opened.caseId(), resolutionId), FULL_REFUND).outcome().name()));

            assertEquals(1, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED),
                    "🔴 只有兩道 CAS 都 rows=1 的那一次才 INSERT outbox");
            assertEquals(1, r.stream().filter("APPLIED"::equals).count(), "恰好一次 APPLIED，實際：" + r);
            assertEquals(ResponsibilityCaseRules.STATUS_RESOLVED, caseStatus(opened.caseId()));
            assertEquals(ResponsibilityCaseRules.RESOLUTION_APPROVED, resolutionStatus(resolutionId));
        }

        /**
         * C3：提案與定案競爭。
         *
         * <p>🔴 要證明的是<b>不存在</b>「case 已 RESOLVED 卻還掛著一筆 AWAITING_REVIEW」這個矛盾狀態
         * ——那正是 snapshot-based finality 會產生的東西。兩種順序都必須收斂到一致的結果。
         */
        @Test
        @DisplayName("C3 提案與定案競爭 ⇒ 不會出現「已定案卻仍有待覆核提案」")
        void c3_proposeVsFinalizeNeverLeavesContradiction() throws Exception {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);

            runConcurrently(
                    quiet(() -> service.finalizeResponsibility(
                            finalizeCmd(opened.caseId(), r1), FULL_REFUND).outcome().name()),
                    quiet(() -> String.valueOf(service.propose(proposeCmd(opened.caseId()), FULL_REFUND))));

            long pending = jdbc.queryForObject(
                    "SELECT count(*) FROM fulfillment_case_resolutions r "
                            + "JOIN fulfillment_exception_cases c ON c.id = r.case_id "
                            + "WHERE c.status = 'RESOLVED' AND r.status = 'AWAITING_REVIEW'", Long.class);
            assertEquals(0, pending,
                    "🔴 出現了「case RESOLVED ＋ pending AWAITING_REVIEW」的矛盾狀態——"
                            + "那代表 FOR UPDATE 沒有把提案與定案序列化");
            assertEquals(1, countCases(), "全程只能有一張 case");
            assertTrue(countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED) <= 1,
                    "N16 至多一筆");
        }

        /**
         * 🔴 <b>{@code FOR UPDATE} 真的鎖住了那一列——用 {@code NOWAIT} 證明，零 sleep、零時序依賴。</b>
         *
         * <p><b>為什麼需要這一支</b>：WP2A 實測發現，把 {@code FOR UPDATE} 拿掉之後
         * C1～C4 <b>連跑 5 次全綠</b>（每條競態都先被 resolution 的 CAS 與
         * {@code uq_fcr_open_per_case} 接住了）⇒ 沒有任何測試在證明「那一列真的被鎖住」。
         * {@code LogisticsSchemaContractTest} 釘的是 <b>SQL 文字</b>，本支釘的是 <b>執行期行為</b>。
         *
         * <p>🔴 <b>用 {@code NOWAIT} 而不是等待或逾時</b>：PostgreSQL 的 {@code FOR UPDATE NOWAIT}
         * 在拿不到鎖時<b>立刻</b>丟 {@code 55P03}（{@code lock_not_available}），不等待。
         * ⇒ 測試是<b>確定性</b>的：沒有 sleep、沒有「等多久算 block」的猜測，也就不會 flaky。
         *
         * <p>三段：① 持鎖者未 commit 時，第二個 connection 的 {@code NOWAIT} 必須被拒
         * ② 持鎖者 commit 後必須拿得到 ③ 對照組——同一句 {@code NOWAIT} 在無人持鎖時本來就會成功
         * （否則「被拒」可能只是 SQL 寫錯）。
         */
        @Test
        @DisplayName("C5 FOR UPDATE 真的鎖住該列：持鎖期間第二個交易的 NOWAIT 被拒（55P03）")
        void c5_forUpdateActuallyLocksTheRow() throws Exception {
            var opened = service.openUnresolved(openCmd(LEG));
            long caseId = opened.caseId();

            // ③ 對照組先跑：無人持鎖時 NOWAIT 必須成功。
            //    少了它，①的「被拒」可能只是因為 SQL 本身有問題。
            assertTrue(tryLockNoWait(caseId), "前提失敗：無人持鎖時 NOWAIT 竟然拿不到鎖 ⇒ 本測試零鑑別力");

            ExecutorService pool = Executors.newSingleThreadExecutor();
            CountDownLatch lockAcquired = new CountDownLatch(1);
            CountDownLatch releaseLock = new CountDownLatch(1);
            try {
                Future<?> holder = pool.submit(() -> tx.execute(status -> {
                    // 這一句就是被測物：它必須真的鎖住該列
                    cases.findByIdForUpdate(caseId).orElseThrow();
                    lockAcquired.countDown();
                    try {
                        // 等主執行緒驗完再 commit（bounded，避免測試掛死）
                        releaseLock.await(20, TimeUnit.SECONDS);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    }
                    return null;
                }));

                assertTrue(lockAcquired.await(20, TimeUnit.SECONDS), "持鎖者未能在時限內取得鎖");

                // ① 持鎖期間：第二個 connection 的 NOWAIT 必須被拒
                assertFalse(tryLockNoWait(caseId),
                        "🔴 FOR UPDATE 沒有鎖住該列——第二個交易照樣拿得到鎖 ⇒ "
                                + "提案與定案不再序列化，而每一步單獨看都會成功、沒有任何錯誤訊息");

                releaseLock.countDown();
                holder.get(20, TimeUnit.SECONDS);
            } finally {
                releaseLock.countDown();
                pool.shutdownNow();
            }

            // ② 釋放之後必須拿得到（證明①的拒絕是「鎖」造成的，不是那一列壞了）
            assertTrue(tryLockNoWait(caseId), "持鎖者 commit 後仍拿不到鎖 ⇒ 鎖沒有被正確釋放");
        }

        /** C4：駁回之後在同一張 case 重提並定案。 */
        @Test
        @DisplayName("C4 REJECTED → 同一張 case 重提 → 定案；lifetime 仍只有一張 case")
        void c4_rejectThenRepropose() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);

            var rejected = service.reject(new ResponsibilityCaseService.RejectCommand(
                    opened.caseId(), r1, "證據不足，請補外送員回報", CHECKER));
            assertEquals(ResponsibilityCaseService.Outcome.APPLIED, rejected);

            // 🔴 駁回不動案件：維持 IN_PROGRESS、owner 保留
            assertEquals(ResponsibilityCaseRules.STATUS_IN_PROGRESS, caseStatus(opened.caseId()),
                    "🔴 駁回不得把案件推離可提案狀態——那會讓它永遠到不了終態");
            assertNotNull(jdbc.queryForObject(
                            "SELECT owner_account_id FROM fulfillment_exception_cases WHERE id = ?",
                            Long.class, opened.caseId()),
                    "owner 必須保留");
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED),
                    "🔴 駁回不得發 N16");

            // 同一張 case 重提
            long r2 = service.propose(proposeCmd(opened.caseId(), "PROVIDER_CAUSED"), FULL_REFUND);
            assertTrue(r2 != r1, "重提應產生新的 resolution");

            var result = service.finalizeResponsibility(finalizeCmd(opened.caseId(), r2), FULL_REFUND);
            assertEquals(ResponsibilityCaseService.Outcome.APPLIED, result.outcome());
            assertEquals(1, countCases(), "🔴 全程只有一張 case——不得靠另開第二張繞過 lifetime UNIQUE");
            assertEquals(1, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED));
            assertEquals(ResponsibilityCaseRules.RESOLUTION_REJECTED, resolutionStatus(r1),
                    "第一筆提案仍是 REJECTED");

            // 🔴 M7：最終成因必須來自<b>第二筆提案</b>（PROVIDER_CAUSED），
            //    不是第一筆（BUYER_CAUSED），更不是覆核者臨時輸入的值。
            //    這是「重提可以換成因」與「覆核者不能換成因」同時成立的證明。
            assertEquals("BUYER_CAUSED", proposedCauseOf(r1),
                    "第一筆提案的成因不得被後續動作改寫——它是稽核軌跡");
            assertEquals("PROVIDER_CAUSED", proposedCauseOf(r2),
                    "第二筆提案的成因");
            assertEquals("PROVIDER_CAUSED", resolutionCodeOf(opened.caseId()),
                    "🔴 結案碼必須＝被批准的那一筆提案的成因，而不是第一筆或覆核者給的值");
            assertEquals("PROVIDER_CAUSED",
                    payloadFieldOf("fulfillment.responsibility.finalized", "responsibility_cause"),
                    "🔴 N16 的成因必須＝被批准的那一筆提案的成因");
        }
    }

    // ═══════════════════ E 群：lifecycle 閉環（65 的八項）═══════════════════

    @Nested
    @DisplayName("E 群：lifecycle 閉環")
    class Lifecycle {

        /** E0.2 測試 1／D3 的 service 層對應：責任案件不得 DISMISSED。 */
        @Test
        @DisplayName("E1 責任案件不得 DISMISSED（DB ＋ 應用層各一道）")
        void e1_dismissForbidden() {
            var opened = service.openUnresolved(openCmd(LEG));

            // 應用層守衛（日後新增 dismiss 入口時必須呼叫它）
            assertThrows(IllegalStateException.class,
                    () -> ResponsibilityCaseRules.assertDismissAllowed(ResponsibilityCaseRules.CATEGORY));

            // DB 那道：直接下 SQL 也擋得住（應用層的 if 會在下一次重構被抽掉，DB 不會）
            Exception ex = assertThrows(Exception.class, () -> jdbc.update(
                    "UPDATE fulfillment_exception_cases SET status='DISMISSED', resolution_code='BUYER_CAUSED',"
                            + " resolved_by_account_id=1, resolved_at=NOW(), owner_account_id=1,"
                            + " owner_assigned_at=NOW() WHERE id=?", opened.caseId()));
            assertTrue(ex.getMessage().contains("chk_exc_responsibility_no_dismiss"),
                    "應由 DB CHECK 擋下，實際：" + ex.getMessage());
        }

        /** E0.2 測試 7：責任案件不接受非責任 resolution_type。 */
        @Test
        @DisplayName("E2 責任案件不接受非責任 resolution_type（雙向）")
        void e2_typeCategoryBindingBothWays() {
            var opened = service.openUnresolved(openCmd(LEG));

            assertThrows(IllegalArgumentException.class, () -> service.propose(
                    new ResponsibilityCaseService.ProposeCommand(
                            opened.caseId(), "CLAIM_DISMISS", null, "r", MAKER), FULL_REFUND),
                    "責任案件不得掛 CLAIM_DISMISS");
            assertThrows(IllegalArgumentException.class, () -> service.propose(
                    new ResponsibilityCaseService.ProposeCommand(
                            opened.caseId(), "ANOMALY_RESUME", null, "r", MAKER), FULL_REFUND),
                    "🔴 ANOMALY_RESUME 免覆核 ⇒ 一個人就能結掉責任案件，繞過雙人制");

            // 反向：一般案件不得掛責任型別（DB 層）
            jdbc.update("INSERT INTO fulfillment_exception_cases "
                    + "(id, case_no, source_service, category, severity, subject_type, subject_ref, status,"
                    + " dedup_key, detail, proposed_action_codes, occurred_at) "
                    + "VALUES (?, 'EXGEN0001', 'payment', 'LOGISTICS_STATEMENT_UNMATCHED', 'P1', 'SHIPMENT',"
                    + " 'leg-generic', 'OPEN', 'g:SHIPMENT:leg-generic', '{}', '[]', NOW())", 990001L);
            Exception ex = assertThrows(Exception.class, () -> jdbc.update(
                    "INSERT INTO fulfillment_case_resolutions "
                            + "(id, case_id, case_category, resolution_type,"
                            + " proposed_responsibility_cause, reason, impact_preview,"
                            + " impact_computed_at, requires_review, status, proposed_by_account_id) "
                            + "VALUES (?, ?, 'LOGISTICS_STATEMENT_UNMATCHED', 'RESPONSIBILITY_ATTRIBUTE',"
                            + " 'BUYER_CAUSED', 'r', '{}', NOW(), true, 'AWAITING_REVIEW', 1)",
                    990002L, 990001L));
            // 🔴 這一列刻意帶合法成因：V13 的 chk_fcr_proposed_cause_binding 會對「責任型別 + 無成因」
            //    先一步丟出來，那樣本測試就驗不到它真正要驗的 type↔category 綁定了。
            //    ⚠️ 這是 V12／V13 兩道各自正確的約束疊在同一列上的交互——探針必須先滿足前一道，
            //    才問得到後一道。
            assertTrue(ex.getMessage().contains("chk_fcr_responsibility_type_binding"),
                    "反向應由雙向等式擋下，實際：" + ex.getMessage());
        }

        /**
         * E0.2 測試 8：{@code RESPONSIBILITY_UNRESOLVED} 不是合法成因。
         *
         * <p>🔴 <b>V13 起這道守門從「結案時」前移到「提案時」</b>：成因是 maker 主張的東西，
         * 覆核者提供不了它 ⇒ 不合法的成因根本提不出來，而不是提得出來、等到覆核才被擋。
         * <p>⚠️ 前移之後<b>更強</b>：舊形狀會先落一列 AWAITING_REVIEW 的提案再擋，
         * 那一列會佔住 {@code uq_fcr_open_per_case} 的唯一名額（要有人去駁回它才能重提）；
         * 現在它連落地都不會。
         */
        @Test
        @DisplayName("E3 RESPONSIBILITY_UNRESOLVED 不得作為提案成因，且提案不落地、不發 N16")
        void e3_unresolvedIsNotAProposableCause() {
            var opened = service.openUnresolved(openCmd(LEG));

            assertThrows(IllegalArgumentException.class,
                    () -> service.propose(proposeCmd(opened.caseId(), ResponsibilityCaseRules.CATEGORY), FULL_REFUND),
                    "RESPONSIBILITY_UNRESOLVED 是未結案期間的 category，不是可提出的成因");

            assertEquals(0L, countResolutions(), "🔴 被拒的提案不得落地——否則會佔住唯一的待覆核名額");
            // 🔴 比舊形狀更強：守門在 assignOwner 之前 ⇒ 被拒的提案<b>零副作用</b>，
            //    案件連 OPEN 都沒離開（舊形狀會先把它推成 IN_PROGRESS 並指派承辦人）。
            assertEquals(ResponsibilityCaseRules.STATUS_OPEN, caseStatus(opened.caseId()),
                    "被拒的提案不得推動案件狀態");
            assertNull(jdbc.queryForObject(
                            "SELECT owner_account_id FROM fulfillment_exception_cases WHERE id = ?",
                            Long.class, opened.caseId()),
                    "被拒的提案不得指派承辦人");
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED),
                    "🔴 outbox 必須零新增列");
        }

        /**
         * 🔴 <b>M2：覆核端在型別上就沒有成因這個參數。</b>
         *
         * <p>這是整個 R2 的核心不變式，而它<b>擋不住「有人把欄位加回去」</b>——
         * 所以用反射把 {@code FinalizeCommand} 的元件名稱釘死：任何人想讓覆核者重新
         * 提供成因，都必須先讓這支測試變紅。
         *
         * <p>⚠️ 用 {@code contains} 判斷而不是比對完整清單：本測試要守的是
         * 「沒有成因欄位」這一件事，不該在每次新增無關欄位時都跟著紅。
         */
        @Test
        @DisplayName("M2 FinalizeCommand 不得有任何成因欄位（覆核者改不了 maker 的主張）")
        void e3b_finalizeCommandHasNoCauseComponent() {
            List<String> components = java.util.Arrays.stream(
                            ResponsibilityCaseService.FinalizeCommand.class.getRecordComponents())
                    .map(java.lang.reflect.RecordComponent::getName)
                    .toList();
            // 對照組：真的讀到了元件，證明這個斷言不是因為反射落空而空轉。
            assertFalse(components.isEmpty(), "量測本身失效：讀到 0 個 record component");
            assertTrue(components.contains("resolutionId"),
                    "量測本身失效：連 resolutionId 都讀不到 ⇒ 反射抓錯類別");

            assertFalse(components.stream().anyMatch(n -> n.toLowerCase().contains("cause")),
                    "🔴 FinalizeCommand 出現了成因欄位 " + components + "——"
                            + "覆核者一旦能提供成因，chk_fcr_two_person 就退化成「有兩個人簽過名」，"
                            + "而不是「第二個人批准了第一個人提的那一件事」。"
                            + "成因必須留在 ProposeCommand。");

            // 反向對照：提案端<b>必須</b>有成因欄位，否則等於整條軸被拆掉。
            List<String> proposeComponents = java.util.Arrays.stream(
                            ResponsibilityCaseService.ProposeCommand.class.getRecordComponents())
                    .map(java.lang.reflect.RecordComponent::getName)
                    .toList();
            assertTrue(proposeComponents.contains("proposedResponsibilityCause"),
                    "🔴 ProposeCommand 少了 proposedResponsibilityCause ⇒ maker 無處主張成因");
        }

        /**
         * {@code source_service} 的 fail-closed guard。
         *
         * <p>🔴 <b>語意</b>：它表達「這個例外狀況的<u>來源服務</u>」，<b>不是</b>「哪個服務擁有這張表」。
         * 兩張表由 payment-service 擁有、INSERT 也由它執行，但觸發來源可能是 {@code shipping}
         * （provider 終局處置格判不出成因）或 {@code order}。
         *
         * <p>🔴 <b>「在碰 DB 之前拒絕」怎麼證明</b>：用 {@code exception_case_no_seq} 的 {@code last_value}。
         * 只數 case 列數不夠——guard 若放在 {@code nextCaseNo()} 之後，列數一樣不會增加
         * （交易 rollback），但 <b>sequence 已經被消耗掉了</b>（sequence 不受 rollback 影響）。
         * ⇒ 那才是「碰過 DB」的可指認痕跡。
         */
        @Test
        @DisplayName("E6 source_service：三個合法值接受；null／blank／unknown 在碰 DB 之前就被拒")
        void e6_sourceServiceFailClosed() {
            // ① 三個合法值都接受（各用不同 leg——同一 leg 會撞 lifetime UNIQUE）
            int i = 0;
            for (String src : List.of("shipping", "order", "payment")) {
                i++;
                var r = service.openUnresolved(new ResponsibilityCaseService.OpenUnresolvedCommand(
                        "leg-src-" + i, src, 2685914682L, ORDER_NO, 12345L, "3003", "{}",
                        LocalDateTime.now(ZoneOffset.UTC)));
                assertTrue(r.created(), "合法來源服務 '" + src + "' 應被接受");
            }
            assertEquals(3, countCases());

            // ② 非法值：拒絕，且**完全沒碰 DB**
            long casesBefore = countCases();
            long seqBefore = caseNoSeqLastValue();
            long outboxBefore = countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED);

            List<String> invalid = new ArrayList<>();
            invalid.add(null);
            invalid.add("");
            invalid.add("   ");
            invalid.add("settlement");   // 曾經存在的服務，已隨 V9 併入並刪除
            invalid.add("PAYMENT");      // 大小寫不符
            invalid.add("payment ");     // 尾隨空白

            for (String bad : invalid) {
                int n = invalid.indexOf(bad);
                assertThrows(IllegalArgumentException.class,
                        () -> service.openUnresolved(new ResponsibilityCaseService.OpenUnresolvedCommand(
                                "leg-bad-" + n, bad, 2685914682L, ORDER_NO, 12345L, "3003", "{}",
                                LocalDateTime.now(ZoneOffset.UTC))),
                        "非法來源服務應被拒：" + (bad == null ? "null" : "'" + bad + "'"));
            }

            assertEquals(casesBefore, countCases(), "被拒的輸入不得留下任何 case 列");
            assertEquals(outboxBefore,
                    countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED),
                    "被拒的輸入不得留下任何 outbox 列");
            assertEquals(seqBefore, caseNoSeqLastValue(),
                    "🔴 case_no sequence 被消耗了 ⇒ guard 放在 nextCaseNo() 之後才擋。"
                            + "sequence 不受 rollback 影響，所以它是「碰過 DB」的可指認痕跡");
        }

        /**
         * 🔴 <b>generic category 的對照組</b>——責任規則<b>只套</b> {@code RESPONSIBILITY_UNRESOLVED}。
         *
         * <p><b>沒有這一支，上面每一條「責任 invariant」測試都只證明了一半</b>：它們證明責任案件被擋住了，
         * 證明不了「不相干的異常流沒有被一起鎖死」。而後者的失效同樣是靜默的——
         * 既有的 {@code LogisticsReconciliationService.openCase} 寫的正是 generic category，
         * 它被誤傷時不會有人知道，直到某天對帳案件開不出來。
         *
         * <p>三格各驗一次，對應 V12 的三道責任約束：
         * <ul>
         *   <li>{@code chk_exc_responsibility_no_dismiss} ⇒ generic 仍可 {@code DISMISSED}</li>
         *   <li>{@code chk_exc_responsibility_resolved_cause} ⇒ generic 仍可用 generic 結案碼</li>
         *   <li>{@code uq_exc_responsibility_subject} ⇒ generic 同一 subject 仍可開第二張</li>
         * </ul>
         */
        @Test
        @DisplayName("E5 generic category 不受責任規則影響（DISMISSED／結案碼／第二張案件）")
        void e5_genericCategoryUnaffected() {
            String generic = "LOGISTICS_STATEMENT_UNMATCHED";

            // ① generic 仍可 DISMISSED（責任案件不行，見 e1）
            insertCase(990101L, "EXGEN1", generic, "DISMISSED", "leg-gen-1",
                    "CLAIM_DISMISS", "gen1:SHIPMENT:leg-gen-1");
            assertEquals("DISMISSED", caseStatus(990101L),
                    "🔴 chk_exc_responsibility_no_dismiss 誤傷了 generic category");

            // ② generic 仍可用 generic 結案碼（責任案件只能用四個 resolved cause，見 e3）
            insertCase(990102L, "EXGEN2", generic, "RESOLVED", "leg-gen-2",
                    "CLAIM_SETTLE", "gen2:SHIPMENT:leg-gen-2");
            assertEquals("RESOLVED", caseStatus(990102L),
                    "🔴 chk_exc_responsibility_resolved_cause 誤傷了 generic category");

            // ③ generic 同一 subject 可開第二張（責任案件 lifetime 只能一張，見 e4）
            //    ⚠️ dedup_key 刻意不同——相同 dedup_key 會撞 uq_exc_open_dedup，
            //    那是另一條約束，會讓這一格測不到 uq_exc_responsibility_subject 的射程。
            insertCase(990103L, "EXGEN3", generic, "RESOLVED", "leg-gen-shared",
                    "CLAIM_SETTLE", "gen3:SHIPMENT:leg-gen-shared");
            insertCase(990104L, "EXGEN4", generic, "RESOLVED", "leg-gen-shared",
                    "CLAIM_SETTLE", "gen4:SHIPMENT:leg-gen-shared");
            long sameSubject = jdbc.queryForObject(
                    "SELECT count(*) FROM fulfillment_exception_cases WHERE subject_ref = ?",
                    Long.class, "leg-gen-shared");
            assertEquals(2, sameSubject,
                    "🔴 uq_exc_responsibility_subject 的射程外溢到 generic category —— "
                            + "它的述詞必須含 WHERE category = 責任判定");

            // 對照組：同樣的三件事，責任 category 一件都不行（證明上面三格不是因為約束整個失效才通過）
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED),
                    "generic 案件不得產生責任判定事件");
            Exception ex = assertThrows(Exception.class, () -> insertCase(990105L, "EXGEN5",
                    ResponsibilityCaseRules.CATEGORY, "DISMISSED", "leg-gen-9",
                    "BUYER_CAUSED", "r:SHIPMENT:leg-gen-9"));
            assertTrue(ex.getMessage().contains("chk_exc_responsibility_no_dismiss"),
                    "🔴 量測本身失效：責任案件的 DISMISSED 竟然沒被擋 ⇒ 上面三格的 PASS 不代表任何事。實際："
                            + ex.getMessage());
        }

        /** RESOLVED 是終局：不得 reopen、不得再提案。 */
        @Test
        @DisplayName("E4 RESOLVED 之後不得再提案、不得另開第二張 case")
        void e4_resolvedIsTerminal() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);
            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);

            assertThrows(IllegalStateException.class, () -> service.propose(proposeCmd(opened.caseId()), FULL_REFUND),
                    "RESOLVED 之後 allowed-from 不含它 ⇒ 提案必須被擋");

            var second = service.openUnresolved(openCmd(LEG));
            assertFalse(second.created(), "🔴 不得另開第二張責任案件（lifetime UNIQUE）");
            assertEquals(1, countCases());
            assertEquals(1, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED),
                    "第二次建案是 no-op ⇒ 不得再發 N17");
        }
    }

    // ═══════════════════ O 群：outbox producer 契約 ═══════════════════

    @Nested
    @DisplayName("O 群：outbox producer 契約")
    class OutboxContract {

        @Test
        @DisplayName("O1 N17／N16 都落在 hawkr.shipping，routing key 與 event_type 逐字正確")
        void o1_exchangeAndRoutingKey() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);
            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);

            List<Map<String, Object>> rows = jdbc.queryForList(
                    "SELECT exchange, routing_key, event_type, payload FROM outbox_events ORDER BY created_at");
            assertEquals(2, rows.size(), "應有 N17 ＋ N16 各一筆");

            // 🔴 期望值硬寫字面值，不引用 MqConstants：契約測試要對拍的正是那個常數有沒有被改壞。
            assertEquals("hawkr.shipping", rows.get(0).get("exchange"),
                    "🔴 發錯 exchange 會 unroutable ⇒ 三個 consumer 一個都收不到，而且沒有任何錯誤說「發錯了」");
            assertEquals("hawkr.shipping", rows.get(1).get("exchange"));
            assertEquals("fulfillment.responsibility.unresolved", rows.get(0).get("routing_key"));
            assertEquals("fulfillment.responsibility.finalized", rows.get(1).get("routing_key"));
            assertEquals("FULFILLMENT_RESPONSIBILITY_UNRESOLVED", rows.get(0).get("event_type"));
            assertEquals("FULFILLMENT_RESPONSIBILITY_FINALIZED", rows.get(1).get("event_type"));
        }

        @Test
        @DisplayName("O2 payload 是 snake_case，且 N16 帶 financially_final=true、N17 帶 false")
        void o2_payloadShape() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "SELLER_CAUSED"), FULL_REFUND);
            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);

            String n17 = jdbc.queryForObject(
                    "SELECT payload FROM outbox_events WHERE routing_key = ?", String.class,
                    "fulfillment.responsibility.unresolved");
            String n16 = jdbc.queryForObject(
                    "SELECT payload FROM outbox_events WHERE routing_key = ?", String.class,
                    "fulfillment.responsibility.finalized");

            // ⚠️ payload 欄是 jsonb ⇒ PostgreSQL 會正規化鍵序與空白，
            //    所以斷言必須解析 JSON，不能比對字串片段（那會綁在一個不屬於契約的格式細節上）。
            Map<String, Object> m16 = readJson(n16);
            Map<String, Object> m17 = readJson(n17);

            // 🔴 欄位集合逐欄釘死（deny-by-default）：只驗「該有的有」擋不住上游長胖時
            //    悄悄多出來的欄位，而 wire 上多一個欄位就是契約變更。
            assertEquals(Set.of("event_id", "event_type", "event_version", "occurred_at", "trace_id",
                            "correlation_id", "order_no", "shipment_id", "shipping_method",
                            "provider_delivery_ref", "responsibility_cause", "financially_final",
                            "case_no", "resolution_id", "resolved_at"),
                    new TreeSet<>(m16.keySet()),
                    "N16 的 wire 欄位集合與 36 §3.3 的 15 欄逐欄表不符");
            assertEquals(Set.of("event_id", "event_type", "event_version", "occurred_at", "trace_id",
                            "correlation_id", "order_no", "shipment_id", "shipping_method",
                            "provider_delivery_ref", "case_no", "financially_final"),
                    new TreeSet<>(m17.keySet()),
                    "N17 的 wire 欄位集合與 WP2A 定案的 12 欄不符");

            assertEquals("SELLER_CAUSED", m16.get("responsibility_cause"));
            assertEquals(Boolean.TRUE, m16.get("financially_final"),
                    "🔴 N16 的 financially_final 必須恆 true 且斷言得到，不是靠 routing key 的紀律");
            assertEquals(LEG, m16.get("provider_delivery_ref"));
            assertNotNull(m16.get("case_no"));
            assertTrue(m16.get("resolution_id") instanceof String,
                    "🔴 resolution_id 是 TSID，必須是字串（JSON 數字會在 consumer 端被 float64 截斷）");

            assertEquals(Boolean.FALSE, m17.get("financially_final"),
                    "🔴 N17 的 financially_final 必須恆 false——「退不退」此時尚未決定");
            assertFalse(m17.containsKey("responsibility_cause"),
                    "🔴 N17 不得攜帶任何 financially-final 語意");
            assertFalse(m17.containsKey("resolution_id"), "N17 尚無 resolution");
        }

        @Test
        @DisplayName("O3 重複建案不重發 N17、重複定案不重發 N16")
        void o3_producerIdempotency() {
            var opened = service.openUnresolved(openCmd(LEG));
            service.openUnresolved(openCmd(LEG));
            service.openUnresolved(openCmd(LEG));
            assertEquals(1, countOutbox("fulfillment.responsibility.unresolved"),
                    "🔴 L2 冪等 ＝ guarded case insert 本身");

            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);
            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);
            var again = service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);
            assertEquals(ResponsibilityCaseService.Outcome.STALE, again.outcome(),
                    "已 final 的案件再 finalize ⇒ STALE");
            assertNull(again.outboxId(), "STALE 不得產生 outbox 列");
            assertEquals(1, countOutbox("fulfillment.responsibility.finalized"),
                    "🔴 L2 冪等 ＝ guarded finalization transition 本身");
        }

        /**
         * 🔴 <b>{@code shipment_id} 在 wire 上必須是 quoted string，不得是 JSON 數字</b>
         * （65 的跨語言通則）。
         *
         * <p><b>為什麼一定要用 &gt; 2^53 的 TSID</b>：notification-service 把 payload 解進
         * {@code map[string]interface{}}，{@code encoding/json} 對 {@code any} 用 float64
         * ⇒ 超過 2^53 的整數會被<b>靜默截斷</b>。用小數字測這件事零鑑別力——
         * 那個範圍內 number 與 string 往返都不會失真。
         *
         * <p>⚠️ <b>不得只驗 Java record 的欄位型別</b>：那證明不了 Jackson 真的寫出了引號。
         * 本測試兩路都驗：① 問 PostgreSQL {@code jsonb_typeof}（型別事實）
         * ② 比對 raw JSON 文字（引號事實）。
         */
        @Test
        @DisplayName("O4 shipment_id 是 quoted string —— 用 > 2^53 的 TSID 驗，且不得是 JSON number")
        void o4_shipmentIdIsQuotedStringBeyond2To53() {
            var opened = service.openUnresolved(openCmd(LEG, BIG_TSID));
            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);
            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);

            // ① 型別事實：PostgreSQL 自己說這是 string
            assertEquals("string", jsonTypeOf("fulfillment.responsibility.unresolved", "shipment_id"),
                    "🔴 N17 的 shipment_id 是 JSON number ⇒ notification-service 解進 "
                            + "map[string]interface{} 時會被 float64 截斷");
            assertEquals("string", jsonTypeOf("fulfillment.responsibility.finalized", "shipment_id"),
                    "🔴 N16 的 shipment_id 是 JSON number ⇒ 同上");

            // ② 引號事實 ＋ 值未失真
            String expected = Long.toString(BIG_TSID);
            assertTrue(expected.length() == 19, "前提：BIG_TSID 是 19 位數，確實超過 2^53");
            for (String rk : List.of("fulfillment.responsibility.unresolved",
                    "fulfillment.responsibility.finalized")) {
                String raw = jdbc.queryForObject(
                        "SELECT payload::text FROM outbox_events WHERE routing_key = ?", String.class, rk);
                assertTrue(raw.matches("(?s).*\"shipment_id\"\\s*:\\s*\"" + expected + "\".*"),
                        rk + " 的 shipment_id 不是 quoted string。raw=" + raw);
                assertFalse(raw.matches("(?s).*\"shipment_id\"\\s*:\\s*" + expected + "[,}\\s].*"),
                        "🔴 " + rk + " 的 shipment_id 是未加引號的 JSON 數字。raw=" + raw);
                // 值本身不得失真（截斷會讓尾數變 0）
                assertTrue(raw.contains(expected),
                        rk + " 的 shipment_id 值已失真（期望 " + expected + "）。raw=" + raw);
            }

            // ③ 其他 wire 欄位不受影響
            Map<String, Object> m16 = readJson(jdbc.queryForObject(
                    "SELECT payload FROM outbox_events WHERE routing_key = ?", String.class,
                    "fulfillment.responsibility.finalized"));
            assertEquals("BUYER_CAUSED", m16.get("responsibility_cause"));
            assertEquals(Boolean.TRUE, m16.get("financially_final"));
            assertEquals(expected, m16.get("shipment_id"), "解析後應是字串形式的原值");
        }

        /**
         * {@code shipmentId = null}（取消發生在 shipment 建立之前）⇒ wire 上是 JSON {@code null}。
         *
         * <p>🔴 特別擋 {@code String.valueOf(Long)} 那個寫法：它會把 {@code null} 變成字面字串
         * {@code "null"}，而那在 wire 上是一個<b>看起來像值的東西</b>，consumer 分不出
         * 「沒有 shipment」與「有一個叫 null 的 shipment」。
         */
        @Test
        @DisplayName("O5 shipmentId 為 null ⇒ wire 是 JSON null，不是字串 \"null\"")
        void o5_nullShipmentIdSerializesAsJsonNull() {
            var opened = service.openUnresolved(openCmd(LEG, null));
            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);
            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);

            for (String rk : List.of("fulfillment.responsibility.unresolved",
                    "fulfillment.responsibility.finalized")) {
                assertEquals("null", jsonTypeOf(rk, "shipment_id"),
                        rk + " 的 shipment_id 應為 JSON null");
                String raw = jdbc.queryForObject(
                        "SELECT payload::text FROM outbox_events WHERE routing_key = ?", String.class, rk);
                assertFalse(raw.matches("(?s).*\"shipment_id\"\\s*:\\s*\"null\".*"),
                        "🔴 " + rk + " 把 null 序列化成字面字串 \"null\"。raw=" + raw);
            }
        }
    }

    // ═══════════ M 群：maker-checker 語意綁定（WP2A-R2，65）═══════════

    /**
     * 🔴 <b>這一群守的是「覆核者批准的，正是提案者提出的那一個」。</b>
     *
     * <p>{@code chk_fcr_two_person} 只保證「覆核者 ≠ 提案者」。少了本群，
     * 一個人可以獨自決定 {@code BUYER_CAUSED}（＝買家拿不到退款），
     * 而帳面上雙人制完全成立——那正是 maker-checker 要防的事情本身。
     */
    @Nested
    @DisplayName("M 群：maker-checker 語意綁定")
    class MakerCheckerBinding {

        /** M1：maker 的成因一路落到 case.resolution_code 與 N16，全程沒有第二個輸入點。 */
        @Test
        @DisplayName("M1 maker 主張的成因 → 提案列 → 結案碼 → N16，四處一致")
        void m1_makerCauseFlowsThroughUnchanged() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "SELLER_CAUSED"), FULL_REFUND);

            // ① 提案當下就落地在提案列上
            assertEquals("SELLER_CAUSED", proposedCauseOf(r1),
                    "🔴 maker 的主張必須在提案時就持久化——否則覆核時無值可用，只能回頭跟 checker 要");

            var result = service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);
            assertEquals(ResponsibilityCaseService.Outcome.APPLIED, result.outcome());

            // ②③④ 結案碼、N16、提案列三者必須是同一個值
            assertEquals("SELLER_CAUSED", resolutionCodeOf(opened.caseId()), "結案碼");
            assertEquals("SELLER_CAUSED",
                    payloadFieldOf("fulfillment.responsibility.finalized", "responsibility_cause"), "N16");
            assertEquals("SELLER_CAUSED", proposedCauseOf(r1), "提案列不得被覆核動作改寫");
        }

        /**
         * M3：責任提案不得沒有成因（DB 層）。
         *
         * <p>⚠️ 走裸 SQL 而不是 service——service 的 {@code assertProposableCause} 已經先擋了。
         * 本測試要證明的是<b>繞過應用層也擋得住</b>：DB 那道是最後防線。
         */
        @Test
        @DisplayName("M3 責任提案 + NULL 成因 ⇒ 被 chk_fcr_proposed_cause_binding 擋下")
        void m3_responsibilityProposalRequiresCause() {
            var opened = service.openUnresolved(openCmd(LEG));
            var ex = assertThrows(Exception.class, () -> insertRawResolution(
                    opened.caseId(), ResponsibilityCaseRules.CATEGORY,
                    ResponsibilityCaseRules.RESOLUTION_TYPE, null));
            assertTrue(ex.getMessage().contains("chk_fcr_proposed_cause_binding"),
                    "應由 binding 擋下，實際：" + ex.getMessage());
            assertEquals(0L, countResolutions(), "被拒的列不得落地");
        }

        /** M4（DB 層）：成因必須是四個 resolved 值之一。 */
        @Test
        @DisplayName("M4 責任提案 + 非法成因 ⇒ 被 chk_fcr_proposed_cause_domain 擋下")
        void m4_causeMustBeInDomain() {
            var opened = service.openUnresolved(openCmd(LEG));
            for (String bad : List.of("SYSTEM_CAUSED", "RESPONSIBILITY_UNRESOLVED", "buyer_caused", "X")) {
                var ex = assertThrows(Exception.class, () -> insertRawResolution(
                        opened.caseId(), ResponsibilityCaseRules.CATEGORY,
                        ResponsibilityCaseRules.RESOLUTION_TYPE, bad),
                        "應拒絕成因 " + bad);
                assertTrue(ex.getMessage().contains("chk_fcr_proposed_cause_domain"),
                        bad + " 應由 domain CHECK 擋下，實際：" + ex.getMessage());
            }
            assertEquals(0L, countResolutions(), "被拒的列不得落地");
        }

        /**
         * M5：一般處置不得夾帶責任成因（binding 的反向）。
         *
         * <p>🔴 單向的 CHECK 會讓這個方向靜默通過，結果是稽核看到一個
         * 「有責任主張」的理賠處置——而那個主張沒有任何人會讀、也沒有任何流程會執行它。
         */
        @Test
        @DisplayName("M5 一般 resolution 夾帶責任成因 ⇒ 被 binding 反向擋下")
        void m5_genericResolutionMustNotCarryCause() {
            jdbc.update("INSERT INTO fulfillment_exception_cases "
                    + "(id, case_no, source_service, category, severity, subject_type, subject_ref, status,"
                    + " dedup_key, detail, proposed_action_codes, occurred_at) "
                    + "VALUES (?, 'EXGEN0009', 'payment', 'LOGISTICS_STATEMENT_UNMATCHED', 'P1', 'SHIPMENT',"
                    + " 'leg-m5', 'OPEN', 'g:SHIPMENT:leg-m5', '{}', '[]', NOW())", 990009L);

            var ex = assertThrows(Exception.class, () -> insertRawResolution(
                    990009L, "LOGISTICS_STATEMENT_UNMATCHED", "CLAIM_SETTLE", "BUYER_CAUSED"));
            assertTrue(ex.getMessage().contains("chk_fcr_proposed_cause_binding"),
                    "應由 binding 反向擋下，實際：" + ex.getMessage());

            // 對照組：同一筆但不帶成因 ⇒ 必須成功，證明上面的拒絕來自成因而不是別的欄位
            assertDoesNotThrow(() -> insertRawResolution(
                            990009L, "LOGISTICS_STATEMENT_UNMATCHED", "CLAIM_SETTLE", null),
                    "不帶成因的一般 resolution 必須能建立——否則上面那個斷言證明不了是成因造成的");
        }

        /**
         * M6：駁回不動任何業務狀態，且<b>保留</b> maker 原本的主張。
         *
         * <p>🔴 保留是稽核要求：被駁回的提案主張過什麼，是「為什麼要重提」的唯一紀錄。
         */
        @Test
        @DisplayName("M6 REJECT 保留原提案成因、案件仍 IN_PROGRESS、N16 為 0")
        void m6_rejectPreservesProposedCause() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "PLATFORM_CAUSED"), FULL_REFUND);

            var rejected = service.reject(new ResponsibilityCaseService.RejectCommand(
                    opened.caseId(), r1, "證據不足", CHECKER));
            assertEquals(ResponsibilityCaseService.Outcome.APPLIED, rejected);

            assertEquals("PLATFORM_CAUSED", proposedCauseOf(r1),
                    "🔴 駁回不得抹掉 maker 的主張——它是稽核軌跡");
            assertEquals(ResponsibilityCaseRules.RESOLUTION_REJECTED, resolutionStatus(r1));
            assertEquals(ResponsibilityCaseRules.STATUS_IN_PROGRESS, caseStatus(opened.caseId()),
                    "駁回不得推動案件狀態");
            assertNull(resolutionCodeOf(opened.caseId()), "駁回不得寫結案碼");
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED),
                    "🔴 駁回不得發 N16");
        }

        /**
         * Ctx6／Ctx7：{@code order_no}／{@code shipping_method} 在<b>建案</b>時 fail-closed。
         *
         * <p>🔴 <b>守門從 finalize 前移到 open</b>（V14）：這三欄現在是案件自己的 immutable 事實，
         * 所以唯一該檢查它們的時機就是它們進入系統的那一刻。
         * 36 §3.3.1 把兩欄標為不可 null，而在 V14 之前實作端一行檢查都沒有。
         *
         * <p>🔴 {@code order_no} 是 order-service 把訂單從 {@code 35} 推到 {@code 54} 的 CAS 錨
         * ⇒ 它為空時那道 CAS 永遠 {@code rows=0}，Order 停在 35，<b>全鏈路無訊號</b>。
         *
         * <p>⚠️ 用 {@code exception_case_no_seq} 的 {@code last_value} 證明<b>零 DB 互動</b>：
         * 只數案件列數不夠——guard 若放在 {@code nextCaseNo()} 之後，列數一樣不會增加（交易 rollback），
         * 但 <b>sequence 已經被消耗掉了</b>（sequence 不受 rollback 影響）。
         * 這一點與 {@code assertSourceService} 的 E6 同構，而且<b>這次真的測得到</b>
         * （R2 第一版把同一個守衛放在 finalize，那裡不消耗 sequence ⇒ 擺放位置無從驗證）。
         */
        @Test
        @DisplayName("Ctx6/Ctx7 建案時 orderNo／shippingMethod 為空 ⇒ 碰 DB 之前就拒絕，sequence 不被消耗")
        void ctx67_openEventContextFailsClosed() {
            long seqBefore = caseNoSeqLastValue();

            record Bad(String orderNo, String shippingMethod, String why) {
            }
            List<Bad> bads = List.of(
                    new Bad(null, "3003", "orderNo null"),
                    new Bad("", "3003", "orderNo 空字串"),
                    new Bad("   ", "3003", "orderNo 全空白"),
                    new Bad(ORDER_NO, null, "shippingMethod null"),
                    new Bad(ORDER_NO, "", "shippingMethod 空字串"),
                    new Bad(ORDER_NO, "   ", "shippingMethod 全空白"));

            for (Bad bad : bads) {
                assertThrows(IllegalArgumentException.class,
                        () -> service.openUnresolved(new ResponsibilityCaseService.OpenUnresolvedCommand(
                                LEG, "payment", 2685914682L, bad.orderNo(), 12345L,
                                bad.shippingMethod(), "{}", LocalDateTime.now(ZoneOffset.UTC))),
                        "應拒絕：" + bad.why());
            }

            assertEquals(0, countCases(), "被拒的建案不得落地");
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_UNRESOLVED),
                    "🔴 被拒的建案不得發 N17");
            assertEquals(seqBefore, caseNoSeqLastValue(),
                    "🔴 sequence 被消耗 ⇒ guard 跑在 nextCaseNo() 之後，不是「任何 DB 互動之前」");

            // 對照組：兩欄合法時必須成功。
            // 🔴 少了它，上面六個 assertThrows 可能是「建案根本不會成功」造成的假陽性。
            assertTrue(service.openUnresolved(openCmd(LEG)).created(),
                    "兩欄合法時必須建得起來——否則上面的拒絕證明不了是這兩欄造成的");
        }

        /** 提案成因缺漏時，service 層在碰 DB 之前就拒絕（與 assertSourceService 同形）。 */
        @Test
        @DisplayName("M8 service 層：責任提案未主張成因 ⇒ 提案不落地")
        void m8_serviceRejectsMissingCause() {
            var opened = service.openUnresolved(openCmd(LEG));
            assertThrows(IllegalArgumentException.class,
                    () -> service.propose(proposeCmd(opened.caseId(), null), FULL_REFUND),
                    "責任提案必須主張一個成因");
            assertEquals(0L, countResolutions(), "被拒的提案不得落地");
        }
    }

    /**
     * 繞過應用層直接 INSERT 一列 resolution，用來證明<b>DB 那道</b>擋得住。
     *
     * <p>⚠️ {@code case_category} 刻意由參數給，才能構造出「一般 case 掛責任成因」這種
     * 應用層根本組不出來的組合。
     */
    private void insertRawResolution(long caseId, String caseCategory, String type, String cause) {
        jdbc.update("INSERT INTO fulfillment_case_resolutions "
                        + "(id, case_id, case_category, resolution_type, proposed_responsibility_cause,"
                        + " reason, impact_preview, impact_computed_at, requires_review, status,"
                        + " proposed_by_account_id) "
                        + "VALUES (?, ?, ?, ?, ?, 'r', '{}', NOW(), true, 'AWAITING_REVIEW', ?)",
                // ⚠️ 不用 TsidUtil：本測試的 MinimalConfig 沒載入 TsidConfiguration
                //    ⇒ 它會丟「尚未初始化」而不是我們要驗的 constraint violation，
                //    那會讓斷言看起來失敗，實際上是探針根本沒打到 DB。
                IDS.incrementAndGet(), caseId, caseCategory, type, cause, MAKER);
    }

    // ═══════════ Ctx 群：immutable event context（WP2A-R2 PM Decision，V14）═══════════

    /**
     * 🔴 <b>這一群守的是「N17 與 N16 講的是同一張訂單」。</b>
     *
     * <p>V14 之前，這三欄由呼叫端在 open 與 finalize <b>各傳一次</b>，中間隔著數小時～數天
     * 與一次人工覆核，而 server 端<b>沒有任何一行</b>比對
     * ⇒ 兩筆事件可以帶不同的 {@code order_no}，而那是 order-service 把訂單從
     * {@code 35} 推到 {@code 54} 的 CAS 錨 ⇒ 對不上時 Order 永遠停在 35，全鏈路無訊號。
     */
    @Nested
    @DisplayName("Ctx 群：immutable event context")
    class ImmutableContext {

        /** Ctx1：建案時三欄落到 case row。 */
        @Test
        @DisplayName("Ctx1 open 把 order_no／shipment_id／shipping_method 寫進 case row")
        void ctx1_openPersistsContext() {
            var opened = service.openUnresolved(openCmd(LEG));
            Map<String, Object> row = caseContextOf(opened.caseId());
            assertEquals(ORDER_NO, row.get("order_no"));
            assertEquals(12345L, ((Number) row.get("shipment_id")).longValue());
            assertEquals("3003", row.get("shipping_method"));
        }

        /**
         * Ctx2：N17 與 case row 講同一件事。
         *
         * <p>⚠️ <b>本測試擋不住什麼</b>（誠實標明）：`e` 是兩行前才由 cmd 建出並原樣 INSERT 的實體，
         * 所以 {@code e.getOrderNo()} 與 {@code cmd.orderNo()} 在這個時點<b>必然相等</b>
         * ⇒ 本測試<b>分辨不出</b>payload 讀的是哪一個。它釘住的是真正重要的那條不變式
         * 「**寫進 DB 的與發出去的一致**」，而 V14 要消滅的跨時間漂移在 finalize 側（見 Ctx4）。
         */
        @Test
        @DisplayName("Ctx2 N17 的三欄 = case row 的三欄")
        void ctx2_n17MatchesPersistedRow() {
            var opened = service.openUnresolved(openCmd(LEG));
            Map<String, Object> row = caseContextOf(opened.caseId());
            String rk = "fulfillment.responsibility.unresolved";
            assertEquals(row.get("order_no"), payloadFieldOf(rk, "order_no"));
            assertEquals(String.valueOf(((Number) row.get("shipment_id")).longValue()),
                    payloadFieldOf(rk, "shipment_id"));
            assertEquals(row.get("shipping_method"), payloadFieldOf(rk, "shipping_method"));
        }

        /**
         * 🔴 <b>Ctx3：覆核端在型別上就沒有 event facts 這些參數。</b>
         *
         * <p>與 V13 的成因欄位同一個形狀：這條不變式擋不住「有人把欄位加回去」，
         * 所以用反射把 {@code FinalizeCommand} 的元件名稱釘死。
         */
        @Test
        @DisplayName("Ctx3 FinalizeCommand 不得有任何 event fact 欄位（反射釘死）")
        void ctx3_finalizeCommandHasNoEventFacts() {
            List<String> components = java.util.Arrays.stream(
                            ResponsibilityCaseService.FinalizeCommand.class.getRecordComponents())
                    .map(java.lang.reflect.RecordComponent::getName)
                    .toList();
            // 對照組：真的讀到了元件。
            assertFalse(components.isEmpty(), "量測本身失效：讀到 0 個 record component");
            assertTrue(components.contains("resolutionId"),
                    "量測本身失效：連 resolutionId 都讀不到 ⇒ 反射抓錯類別");

            for (String banned : List.of("orderNo", "shipmentId", "shippingMethod",
                    "providerDeliveryRef", "sourceService", "responsibilityCause")) {
                assertFalse(components.contains(banned),
                        "🔴 FinalizeCommand 出現了 " + banned + "——覆核者不得指定 event facts。"
                                + "它的權威來源是建案當下凍結的 case row（V14）／提案列（V13），"
                                + "由呼叫端再傳一次就會讓 N17 與 N16 漂開。實際元件：" + components);
            }
            assertEquals(4, components.size(),
                    "FinalizeCommand 應只有 caseId／resolutionId／reviewComment／reviewedByAccountId，"
                            + "實際：" + components);
        }

        /**
         * 🔴 <b>Ctx4：N16 取自 locked case row，不是任何 in-memory 或 request 的值。</b>
         *
         * <p>做法：建案之後<b>直接改 DB 那一列</b>的 {@code order_no}，再 finalize。
         * N16 若帶的是<b>改過</b>的值 ⇒ 證明它真的從那一列讀；
         * 若帶的是原值 ⇒ 它讀的是別的地方。這是本群唯一有完整鑑別力的測試。
         */
        @Test
        @DisplayName("Ctx4 N16 的三欄取自 locked case row（改 DB 後 N16 跟著變）")
        void ctx4_n16ReadsFromLockedCaseRow() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "SELLER_CAUSED"), FULL_REFUND);

            // 繞過應用層直接改那一列（模擬「case row 才是權威」）
            jdbc.update("UPDATE fulfillment_exception_cases SET order_no = ?, shipping_method = ?"
                    + " WHERE id = ?", "O-REWRITTEN", "3004", opened.caseId());

            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);

            String rk = "fulfillment.responsibility.finalized";
            assertEquals("O-REWRITTEN", payloadFieldOf(rk, "order_no"),
                    "🔴 N16 的 order_no 不是取自 case row ⇒ 呼叫端仍有機會讓它與 N17 漂開");
            assertEquals("3004", payloadFieldOf(rk, "shipping_method"),
                    "🔴 N16 的 shipping_method 不是取自 case row");
        }

        /** Ctx5：shipment_id 為 null 合法（shipment 建立前也可能判責任）。 */
        @Test
        @DisplayName("Ctx5 shipment_id 可為 null，N17／N16 都是 JSON null")
        void ctx5_nullShipmentIdIsLegal() {
            var opened = service.openUnresolved(openCmd(LEG, null));
            assertNull(caseContextOf(opened.caseId()).get("shipment_id"),
                    "shipment_id 應可為 null——建單失敗(990)也是一種履約失敗，那時還沒有 shipment");
            long r1 = service.propose(proposeCmd(opened.caseId()), FULL_REFUND);
            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND);
            for (String rk : List.of("fulfillment.responsibility.unresolved",
                    "fulfillment.responsibility.finalized")) {
                assertEquals("null", jsonTypeOf(rk, "shipment_id"), rk + " 的 shipment_id 應為 JSON null");
            }
        }

        /**
         * 🔴 <b>Ctx8：REJECT → 重提 → 定案，全程 context 一個字都不變。</b>
         *
         * <p>這是 immutability 的實質驗證：中間經過了 propose／reject／re-propose／finalize
         * 四個會寫 DB 的動作，沒有任何一個該碰這三欄。
         */
        @Test
        @DisplayName("Ctx8 proposal／reject／re-proposal／finalize 之後 context 完全不變")
        void ctx8_contextSurvivesFullLifecycle() {
            var opened = service.openUnresolved(openCmd(LEG));
            Map<String, Object> before = caseContextOf(opened.caseId());

            long r1 = service.propose(proposeCmd(opened.caseId(), "BUYER_CAUSED"), FULL_REFUND);
            assertEquals(before, caseContextOf(opened.caseId()), "propose 後 context 不得變");

            service.reject(new ResponsibilityCaseService.RejectCommand(
                    opened.caseId(), r1, "證據不足", CHECKER));
            assertEquals(before, caseContextOf(opened.caseId()), "reject 後 context 不得變");

            long r2 = service.propose(proposeCmd(opened.caseId(), "PROVIDER_CAUSED"), FULL_REFUND);
            assertEquals(before, caseContextOf(opened.caseId()), "re-proposal 後 context 不得變");

            service.finalizeResponsibility(finalizeCmd(opened.caseId(), r2), FULL_REFUND);
            assertEquals(before, caseContextOf(opened.caseId()), "finalize 後 context 不得變");

            // 對照組：這一輪確實有東西在動（否則上面四個 assertEquals 可能只是「什麼都沒發生」）
            assertEquals(ResponsibilityCaseRules.STATUS_RESOLVED, caseStatus(opened.caseId()),
                    "量測本身失效：整個 lifecycle 跑完案件卻沒結案");
            assertEquals("PROVIDER_CAUSED", resolutionCodeOf(opened.caseId()),
                    "量測本身失效：結案碼沒有跟著第二筆提案走");
        }

        /**
         * 🔴 <b>Ctx10：繞過應用層也擋得住</b>（V14 的 {@code chk_exc_responsibility_context}）。
         *
         * <p>⚠️ <b>為什麼需要這一支</b>：實測發現拿掉那道 CHECK 之後<b>整組測試仍然全綠</b>
         * ——因為 {@code assertEventContext} 在應用層就先擋了，沒有任何測試路徑會讓一筆
         * 缺 context 的責任案件走到 DB。那讓 DB 那道成了<b>沒有人守著的裸約束</b>。
         *
         * <p>🔴 兩道的分工不同、不互相取代：應用層那道把它變成可回業務碼的正常拒絕；
         * DB 這道守的是「<b>繞過應用層的寫入路徑</b>」——維運 SQL、未來新增的 repository 方法、
         * 或哪天有人直接用 MyBatis-Plus 的通用 insert。本測試用裸 jdbc 模擬那條路。
         */
        @Test
        @DisplayName("Ctx10 繞過應用層的責任案件 INSERT 缺 context ⇒ 被 DB CHECK 擋下")
        void ctx10_dbCheckCatchesBypass() {
            // order_no 缺
            var e1 = assertThrows(Exception.class, () -> insertRawCase(
                    990201L, "EXBP0001", "leg-bp-1", null, "3003"));
            assertTrue(e1.getMessage().contains("chk_exc_responsibility_context"),
                    "應由 chk_exc_responsibility_context 擋下，實際：" + e1.getMessage());

            // shipping_method 缺
            var e2 = assertThrows(Exception.class, () -> insertRawCase(
                    990202L, "EXBP0002", "leg-bp-2", ORDER_NO, null));
            assertTrue(e2.getMessage().contains("chk_exc_responsibility_context"),
                    "應由 chk_exc_responsibility_context 擋下，實際：" + e2.getMessage());

            assertEquals(0, countCases(), "被拒的列不得落地");

            // 🔴 對照組：兩欄齊全時必須成功（且 shipment_id 仍可為 null）。
            //    少了它，上面兩個 assertThrows 可能是別的約束造成的。
            assertDoesNotThrow(() -> insertRawCase(
                            990203L, "EXBP0003", "leg-bp-3", ORDER_NO, "3003"),
                    "兩欄齊全時必須建得起來——否則上面的拒絕證明不了是 context 造成的");
            assertEquals(1, countCases());
        }

        /** Ctx9：generic exception case 三欄全 null 仍合法（不得被責任軸的新需求破壞）。 */
        @Test
        @DisplayName("Ctx9 generic exception case 三欄皆 null 仍可建立")
        void ctx9_genericCaseNeedsNoContext() {
            assertDoesNotThrow(() -> jdbc.update("INSERT INTO fulfillment_exception_cases "
                    + "(id, case_no, source_service, category, severity, subject_type, subject_ref,"
                    + " status, dedup_key, detail, proposed_action_codes, occurred_at) "
                    + "VALUES (?, 'EXGEN0099', 'payment', 'LOGISTICS_STATEMENT_UNMATCHED', 'P2',"
                    + " 'SHIPMENT', 'leg-ctx9', 'OPEN', 'g:SHIPMENT:leg-ctx9', '{}', '[]', NOW())",
                    990099L),
                    "🔴 generic 案件不該因為責任軸的新欄位而建不出來");
            Map<String, Object> row = caseContextOf(990099L);
            assertNull(row.get("order_no"));
            assertNull(row.get("shipment_id"));
            assertNull(row.get("shipping_method"));
        }
    }

    /**
     * 繞過應用層直接 INSERT 一筆<b>責任</b>案件，用來證明 DB 那道擋得住。
     *
     * <p>⚠️ {@code shipment_id} 刻意不給——它本來就不在必填內，
     * 這樣上面的拒絕才能歸因到 {@code order_no}／{@code shipping_method}。
     */
    private void insertRawCase(long id, String caseNo, String subjectRef,
                               String orderNo, String shippingMethod) {
        jdbc.update("INSERT INTO fulfillment_exception_cases "
                        + "(id, case_no, source_service, category, severity, subject_type, subject_ref,"
                        + " status, dedup_key, detail, proposed_action_codes, occurred_at,"
                        + " order_no, shipping_method) "
                        + "VALUES (?, ?, 'shipping', ?, 'P1', 'SHIPMENT', ?, 'OPEN', ?, '{}', '[]',"
                        + " NOW(), ?, ?)",
                id, caseNo, ResponsibilityCaseRules.CATEGORY, subjectRef,
                ResponsibilityCaseRules.dedupKey(subjectRef), orderNo, shippingMethod);
    }

    /** case row 上的三個 immutable context 欄位。 */
    private Map<String, Object> caseContextOf(long caseId) {
        return jdbc.queryForMap(
                "SELECT order_no, shipment_id, shipping_method"
                        + " FROM fulfillment_exception_cases WHERE id = ?", caseId);
    }

    // ═══════════ P 群：ResponsibilityImpactPreview 與 9575 stale guard（WP2A-R2）═══════════

    /**
     * 🔴 <b>這一群守的是 65 硬約束 2</b>：
     * 「覆核者看到的金額與實際執行的金額不同，就是 maker-checker 在保護的那件事本身失效」。
     *
     * <p>⚠️ 預覽<b>只有 refundAmount 一個金額</b>。{@code platformFeeDelta}／{@code sellerNetDelta}
     * 不是 0，而是<b>不屬於這個 contract</b>——它們的唯一權威公式在 65（已裁決 dormant），
     * 且 {@code SELLER_DIRECT} 下平台沒有代收 ⇒ 訂單層平台費語意不成立。
     * 填 0 會讓覆核者看到「平台與賣家都不受影響」的假結論，比不顯示更糟。
     */
    @Nested
    @DisplayName("P 群：影響預覽與 stale guard")
    class ImpactPreviewGuard {

        /** P1：BUYER_CAUSED ⇒ 退款 0（走 54 DELIVERY_FAILED_NO_REFUND）。 */
        @Test
        @DisplayName("P1 BUYER_CAUSED ⇒ refundAmount = 0")
        void p1_buyerCausedRefundsNothing() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "BUYER_CAUSED"), FULL_REFUND);
            assertEquals("0", previewFieldOf(r1, "refundAmount"),
                    "🔴 BUYER_CAUSED 是唯一不退款的成因");
        }

        /** P2–P4：其餘三個 resolved 成因 ⇒ 全額（＝ orders.total_actual_amount）。 */
        @Test
        @DisplayName("P2/P3/P4 SELLER／PROVIDER／PLATFORM_CAUSED ⇒ refundAmount = 全額")
        void p234_otherCausesRefundFull() {
            for (String cause : List.of("SELLER_CAUSED", "PROVIDER_CAUSED", "PLATFORM_CAUSED")) {
                var opened = service.openUnresolved(openCmd("leg-" + cause));
                long r = service.propose(proposeCmd(opened.caseId(), cause), FULL_REFUND);
                assertEquals(String.valueOf(FULL_REFUND), previewFieldOf(r, "refundAmount"),
                        cause + " 應全額退款");
            }
        }

        /** P5：快照與覆核當下重算一致 ⇒ 放行。 */
        @Test
        @DisplayName("P5 refundAmount 一致 ⇒ 可以定案")
        void p5_matchingPreviewAllowsApproval() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "SELLER_CAUSED"), FULL_REFUND);
            var result = service.finalizeResponsibility(
                    finalizeCmd(opened.caseId(), r1), FULL_REFUND);
            assertEquals(ResponsibilityCaseService.Outcome.APPLIED, result.outcome());
            assertEquals(1, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED));
        }

        /**
         * 🔴 <b>P6：refundAmount 變了 ⇒ 擋下，且零副作用。</b>
         *
         * <p>以覆核時傳入<b>不同的權威退款基準</b>模擬「金額前提變了」。
         * 依 {@code orders.total_actual_amount} 的 immutability 這在生產上不該發生——
         * 但守衛的價值正在於<b>它不假設那件事永遠成立</b>：
         * 哪天有人在 order-service 加了寫入路徑，這裡會擋下來而不是安靜地退錯錢。
         */
        @Test
        @DisplayName("P6 refundAmount 不一致 ⇒ STALE_PREVIEW，不進 CAS、不發 N16")
        void p6_staleRefundAmountBlocksApproval() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "SELLER_CAUSED"), FULL_REFUND);

            var result = service.finalizeResponsibility(
                    finalizeCmd(opened.caseId(), r1), FULL_REFUND + 1);

            assertEquals(ResponsibilityCaseService.Outcome.STALE_PREVIEW, result.outcome(),
                    "🔴 金額前提變了必須擋下（WP2B 映 9575 EXCEPTION_CASE_IMPACT_PREVIEW_STALE）");
            assertNull(result.outboxId(), "不得排入任何事件");
            assertEquals(ResponsibilityCaseRules.RESOLUTION_AWAITING_REVIEW, resolutionStatus(r1),
                    "🔴 提案必須留在 AWAITING_REVIEW——被擋下不等於被駁回");
            assertEquals(ResponsibilityCaseRules.STATUS_IN_PROGRESS, caseStatus(opened.caseId()),
                    "🔴 案件不得被結案");
            assertNull(resolutionCodeOf(opened.caseId()), "不得寫結案碼");
            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED),
                    "🔴 N16 必須零筆");
            assertEquals(String.valueOf(FULL_REFUND), previewFieldOf(r1, "refundAmount"),
                    "被擋下時不得偷偷改寫快照——那會讓下一次覆核直接通過");
        }

        /**
         * 🔴 <b>P7：computedAt 不同不算 stale。</b>
         *
         * <p>重算必然產生新的時間戳。把 {@code computedAt} 納入 equality
         * 會讓<b>每一次</b>覆核都判定 stale ⇒ 那道守衛永遠是狼來了，
         * 而狼來了的守衛比沒有守衛更糟（它會被當成雜訊忽略）。
         */
        @Test
        @DisplayName("P7 只有 computedAt 不同 ⇒ 不算 stale")
        void p7_computedAtDifferenceIsNotStale() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "PLATFORM_CAUSED"), FULL_REFUND);

            // 把快照的 computedAt 改成一個明顯不同的時間，金額不動
            jdbc.update("UPDATE fulfillment_case_resolutions"
                    + " SET impact_preview = jsonb_set(impact_preview, '{computedAt}',"
                    + " '\"2020-01-01T00:00:00Z\"'::jsonb) WHERE id = ?", r1);
            assertEquals("2020-01-01T00:00:00Z", previewFieldOf(r1, "computedAt"),
                    "量測本身失效：computedAt 沒有真的被改掉");

            assertEquals(ResponsibilityCaseService.Outcome.APPLIED,
                    service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND)
                            .outcome(),
                    "🔴 computedAt 不同不得判 stale——否則每一次覆核都會被擋");
        }

        /**
         * 🔴 <b>P8：呼叫端沒有任何路徑可以提供金額。</b>
         *
         * <p>65 的 A-4 原本把 {@code impactPreview} 放在 <b>request body</b> 由前端送，
         * PM 已裁決撤銷：對責任判定，預覽必須是 backend 算出來的。
         * 這裡用反射釘死 {@code ProposeCommand} 沒有任何金額／預覽欄位。
         */
        @Test
        @DisplayName("P8 ProposeCommand 沒有任何 impact／金額欄位（反射釘死）")
        void p8_proposeCommandHasNoClientSuppliedPreview() {
            List<String> components = java.util.Arrays.stream(
                            ResponsibilityCaseService.ProposeCommand.class.getRecordComponents())
                    .map(java.lang.reflect.RecordComponent::getName)
                    .toList();
            assertFalse(components.isEmpty(), "量測本身失效：讀到 0 個 record component");
            assertTrue(components.contains("proposedResponsibilityCause"),
                    "量測本身失效：反射抓錯類別");
            for (String banned : List.of("impactPreviewJson", "impactPreview", "refundAmount",
                    "platformFeeDelta", "sellerNetDelta")) {
                assertFalse(components.contains(banned),
                        "🔴 ProposeCommand 出現了 " + banned + "——責任判定的影響預覽必須由 backend 算，"
                                + "不得信任前端金額。實際元件：" + components);
            }
        }

        /**
         * 🔴 快照必須能被 {@code ResponsibilityImpactPreview} <b>嚴格</b> parse。
         *
         * <p>寬鬆 parse 會讓一份壞掉的快照在覆核當下「看起來還能用」，而那時錢正要退出去。
         * 這裡把快照改成舊世界的 {@code "{}"}（V13 之前的預設值），覆核必須<b>拒絕</b>而不是當成 0。
         */
        @Test
        @DisplayName("P9 快照壞掉（空 object／多欄）⇒ 覆核拒絕，不得當成 0")
        void p9_corruptSnapshotIsRejected() {
            var opened = service.openUnresolved(openCmd(LEG));
            long r1 = service.propose(proposeCmd(opened.caseId(), "SELLER_CAUSED"), FULL_REFUND);

            jdbc.update("UPDATE fulfillment_case_resolutions SET impact_preview = '{}'::jsonb"
                    + " WHERE id = ?", r1);
            assertThrows(IllegalStateException.class,
                    () -> service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND),
                    "空 object 不是合法快照——V13 之前的 \"{}\" 預設值必須被拒絕");

            // 多一個沒有 canonical 算法的欄位也要拒絕（否則它會看起來像被驗過了）
            jdbc.update("UPDATE fulfillment_case_resolutions SET impact_preview ="
                    + " '{\"refundAmount\":\"1200\",\"computedAt\":\"2026-01-01T00:00:00Z\","
                    + "\"platformFeeDelta\":\"0\"}'::jsonb WHERE id = ?", r1);
            assertThrows(IllegalStateException.class,
                    () -> service.finalizeResponsibility(finalizeCmd(opened.caseId(), r1), FULL_REFUND),
                    "🔴 platformFeeDelta 沒有 canonical 算法，出現在快照裡就是有人在假裝它被算過");

            assertEquals(0, countOutbox(MqConstants.ROUTING_KEY_FULFILLMENT_RESPONSIBILITY_FINALIZED),
                    "全程不得發 N16");
        }
    }

    /** 提案列上 {@code impact_preview} 快照的某個欄位。 */
    private String previewFieldOf(long resolutionId, String field) {
        return jdbc.queryForObject(
                "SELECT impact_preview ->> ? FROM fulfillment_case_resolutions WHERE id = ?",
                String.class, field, resolutionId);
    }
}

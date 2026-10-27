package io.github.will413028.hawkr.payment.scheduler;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.payment.config.ReconcileProperties;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentTransactionMapper;
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
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.List;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Reconcile 邊界 SQL 整合測試（H7-H9）
 *
 * 對應審計報告 §6 + Phase 6：驗證 PaymentTransactionMapper.findReconcileCandidates
 * 的 SQL where 條件，純單元測試 mock 不到 SQL，必須真實 PostgreSQL。
 */
@SpringBootTest(classes = ReconcileBoundaryIntegrationTest.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class ReconcileBoundaryIntegrationTest {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("payment_reconcile_test")
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
        // spring.flyway.url/user/password 在 application.yml 有獨立硬編碼預設值（指向本機真實
        // hawkr_payment DB），FlywayAutoConfiguration 優先使用這組獨立設定而非
        // spring.datasource.*；不覆寫會導致 Flyway 對本機真實 DB no-op、testcontainers 內的
        // 測試 DB 卻從未跑過 migration（mapper 因此撞 "relation does not exist"）。
        registry.add("spring.flyway.url", POSTGRES::getJdbcUrl);
        registry.add("spring.flyway.user", POSTGRES::getUsername);
        registry.add("spring.flyway.password", POSTGRES::getPassword);
        registry.add("spring.flyway.enabled", () -> "true");
        registry.add("spring.flyway.locations", () -> "classpath:db/migration");
        registry.add("spring.flyway.baseline-on-migrate", () -> "false");
    }

    /**
     * 最小 Spring config：只載 DataSource + Flyway + MybatisPlus + Mapper，
     * 完全避開 RabbitMQ / Web / TSID / gRPC / 業務 service。
     */
    @SpringBootConfiguration
    @ImportAutoConfiguration({
            DataSourceAutoConfiguration.class,
            DataSourceTransactionManagerAutoConfiguration.class,
            SqlInitializationAutoConfiguration.class,
            FlywayAutoConfiguration.class,
            MybatisPlusAutoConfiguration.class,
            MyBatisPlusAutoConfiguration.class  // common-core 提供 TimestampTzTypeHandler
    })
    @MapperScan("io.github.will413028.hawkr.payment.infrastructure.persistence.mapper")
    static class MinimalConfig {
    }

    @Autowired
    private PaymentTransactionMapper mapper;

    @BeforeEach
    void cleanDb() {
        mapper.delete(null);
    }

    @AfterEach
    void cleanAfter() {
        mapper.delete(null);
    }

    private PaymentTransactionEntity newTx(
            String tradeNo,
            String status,
            LocalDateTime createdAt,
            LocalDateTime expireAt,
            LocalDateTime lastReconciledAt,
            LocalDateTime rescuedAt
    ) {
        PaymentTransactionEntity e = new PaymentTransactionEntity();
        e.setId(Math.abs(UUID.randomUUID().getMostSignificantBits() % 1_000_000_000L) + 100_000L);
        e.setTradeNo(tradeNo);
        e.setOrderId(tradeNo);
        e.setMerId("HAWK9");
        e.setBuyerId("2001");
        e.setPaymentType("2004");
        e.setAmount(100L);
        e.setProductName("test");
        e.setStatus(status);
        e.setCreatedAt(createdAt);
        e.setUpdatedAt(createdAt);
        e.setExpireAt(expireAt);
        e.setLastReconciledAt(lastReconciledAt);
        e.setRescuedAt(rescuedAt);
        e.setVersion(0);
        return e;
    }

    private LocalDateTime now() {
        return LocalDateTime.now(ZoneOffset.UTC);
    }

    private List<PaymentTransactionEntity> findCandidates(int initialDelayMin, int graceMin, int retryMin) {
        LocalDateTime n = now();
        return mapper.findReconcileCandidates(
                n.minusMinutes(initialDelayMin),
                n.minusMinutes(graceMin),
                n.minusMinutes(retryMin),
                100
        );
    }

    // ========================= H7: grace 視窗 =========================

    @Test
    @DisplayName("H7-1: EXPIRED + expire_at 在 grace 內 → 應被撈")
    void h7_expiredInsideGrace_shouldBePicked() {
        var tx = newTx("H7_INSIDE", "3",
                now().minusHours(1),
                now().minusMinutes(25),
                null,
                null);
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(1, candidates.size());
        assertEquals("H7_INSIDE", candidates.get(0).getTradeNo());
    }

    @Test
    @DisplayName("H7-2: EXPIRED + expire_at 超出 grace → 不應被撈")
    void h7_expiredOutsideGrace_shouldNotBePicked() {
        var tx = newTx("H7_OUTSIDE", "3",
                now().minusHours(1),
                now().minusMinutes(31),
                null,
                null);
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(0, candidates.size(), "grace 視窗外的 EXPIRED 不該被撈");
    }

    // ========================= H8: initial-delay 內 =========================

    @Test
    @DisplayName("H8-1: INIT + created_at 在 initial-delay 內 → 不應被撈")
    void h8_initInsideInitialDelay_shouldNotBePicked() {
        var tx = newTx("H8_TOO_NEW", "0",
                now().minusMinutes(1),
                now().plusMinutes(60),
                null,
                null);
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(0, candidates.size(), "initial-delay 內的 INIT 不該被撈");
    }

    @Test
    @DisplayName("H8-2: INIT + created_at 超過 initial-delay → 應被撈")
    void h8_initBeyondInitialDelay_shouldBePicked() {
        var tx = newTx("H8_OK", "0",
                now().minusMinutes(4),
                now().plusMinutes(60),
                null,
                null);
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(1, candidates.size());
    }

    // ========================= H9: retry interval =========================

    @Test
    @DisplayName("H9-1: last_reconciled_at IS NULL → 應被撈")
    void h9_lastReconciledNull_shouldBePicked() {
        var tx = newTx("H9_NEVER", "0",
                now().minusMinutes(10),
                now().plusMinutes(60),
                null,
                null);
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(1, candidates.size());
    }

    @Test
    @DisplayName("H9-2: last_reconciled_at 在 retry interval 內 → 不應被撈")
    void h9_lastReconciledInsideRetry_shouldNotBePicked() {
        var tx = newTx("H9_TOO_RECENT", "0",
                now().minusMinutes(10),
                now().plusMinutes(60),
                now().minusMinutes(2),
                null);
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(0, candidates.size(), "retry interval 內的訂單不該被撈");
    }

    @Test
    @DisplayName("H9-3: last_reconciled_at 超過 retry interval → 應被撈")
    void h9_lastReconciledBeyondRetry_shouldBePicked() {
        var tx = newTx("H9_OLD", "0",
                now().minusMinutes(20),
                now().plusMinutes(60),
                now().minusMinutes(6),
                null);
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(1, candidates.size());
    }

    // ========================= 補充邊界 =========================

    @Test
    @DisplayName("補充: EXPIRED + rescued_at 已標記 → 不應被撈")
    void rescuedExpired_shouldNotBePicked() {
        var tx = newTx("RESCUED", "3",
                now().minusHours(1),
                now().minusMinutes(20),
                now().minusMinutes(10),
                now().minusMinutes(10));
        mapper.insert(tx);

        var candidates = findCandidates(3, 30, 5);
        assertEquals(0, candidates.size());
    }

    @Test
    @DisplayName("補充: PAID 不該被撈")
    void paid_shouldNotBePicked() {
        var tx = newTx("PAID_DONE", "1",
                now().minusHours(1),
                now().plusMinutes(60),
                null,
                null);
        mapper.insert(tx);

        assertEquals(0, findCandidates(3, 30, 5).size());
    }

    @Test
    @DisplayName("補充: CLOSED 不該被撈")
    void closed_shouldNotBePicked() {
        var tx = newTx("CLOSED_DONE", "5",
                now().minusHours(1),
                now().plusMinutes(60),
                null,
                null);
        mapper.insert(tx);

        assertEquals(0, findCandidates(3, 30, 5).size());
    }

    @Test
    @DisplayName("ReconcileProperties 預設值符合文件（grace=30, retry=5, initialDelay=3）")
    void reconcileProperties_defaults() {
        var p = new ReconcileProperties();
        assertEquals(3, p.getInitialDelayMinutes());
        assertEquals(5, p.getRetryIntervalMinutes());
        assertEquals(30, p.getGraceMinutes());
        assertTrue(p.isEnabled());
    }
}

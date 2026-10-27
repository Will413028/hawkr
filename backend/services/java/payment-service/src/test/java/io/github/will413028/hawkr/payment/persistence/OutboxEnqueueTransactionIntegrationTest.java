package io.github.will413028.hawkr.payment.persistence;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxPublisher;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
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
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;

/**
 * generic {@code enqueue} 的 <b>真 DB</b> 交易語意（WP1-R1，T3／T4）。
 *
 * <p><b>為什麼必須用真 DB 而不是 mock mapper</b>：本檔要證明的是
 * 「outbox INSERT 真的跟外層業務交易一起 rollback」。mock mapper 只能證明
 * 「我呼叫了 insert」，證明不了那一列在 rollback 之後**不存在**——
 * 而那正是 transactional outbox 的全部意義。
 *
 * <p>形狀照抄 {@code ReconcileBoundaryIntegrationTest}：Testcontainers ＋ 最小 Spring config
 * （只載 DataSource／Flyway／MyBatis-Plus／Mapper，完全避開 RabbitMQ／Web／gRPC）。
 * {@code PaymentOutboxWriter} 手動 new 並注入**真的** mapper ⇒ 它的 INSERT 走 Spring 管理的
 * SqlSession，與 {@code TransactionTemplate} 開的交易是同一個。
 */
@SpringBootTest(classes = OutboxEnqueueTransactionIntegrationTest.MinimalConfig.class)
@Testcontainers
@Tag("integration")
@DisplayName("generic enqueue 的真 DB 交易語意")
class OutboxEnqueueTransactionIntegrationTest {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("payment_outbox_tx_test")
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
        // application.yml 的 spring.flyway.* 有指向本機真實 DB 的硬編碼預設值，
        // 不覆寫會讓 Flyway 對本機 no-op、容器內的測試 DB 從未跑過 migration。
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
            SqlInitializationAutoConfiguration.class,
            FlywayAutoConfiguration.class,
            MybatisPlusAutoConfiguration.class,
            MyBatisPlusAutoConfiguration.class
    })
    @MapperScan("io.github.will413028.hawkr.payment.infrastructure.persistence.mapper")
    static class MinimalConfig {
    }

    @Autowired
    private OutboxEventMapper outboxMapper;

    @Autowired
    private PlatformTransactionManager txManager;

    private PaymentOutboxWriter writer;
    private TransactionTemplate tx;

    @BeforeEach
    void setUp() {
        writer = new PaymentOutboxWriter(
                outboxMapper, new ObjectMapper(), mock(PaymentOutboxPublisher.class), new SimpleMeterRegistry());
        tx = new TransactionTemplate(txManager);
        outboxMapper.delete(null);
    }

    @AfterEach
    void tearDown() {
        outboxMapper.delete(null);
    }

    private long countByRoutingKey(String routingKey) {
        return outboxMapper.selectCount(new QueryWrapper<OutboxEventEntity>().eq("routing_key", routingKey));
    }

    // ───────────────── T3：真交易 → 接受並落庫 ─────────────────

    @Test
    @DisplayName("T3｜真 TransactionTemplate 交易內 → enqueue 成功且 commit 後列真的在 DB 裡")
    void t3_realTransactionIsAcceptedAndRowPersists() {
        OutboxEventEntity written = tx.execute(status -> writer.enqueue(
                "hawkr.shipping", "fulfillment.responsibility.finalized", "t3.rk", "{\"a\":1}"));

        assertNotNull(written);
        assertNotNull(written.getId(), "id 應由 DB 回填");
        assertEquals(1L, countByRoutingKey("t3.rk"), "commit 之後那一列必須存在");

        OutboxEventEntity reread = outboxMapper.selectById(written.getId());
        assertEquals("hawkr.shipping", reread.getExchange(),
                "exchange 必須原樣落庫（read-back，不是相信寫入當下的物件）");
        assertEquals("8221", reread.getStatus());
    }

    // ───────────────── T4：rollback → 列必須不存在 ─────────────────

    /**
     * 🔴 本檔的核心：機械證明 generic enqueue 真的跟外層業務交易一起 rollback。
     *
     * <p>失敗的形狀會是「outbox 列留在 DB 裡」——而它描述的那件事已經被 rollback 掉了，
     * scheduler 仍會照常把它發出去，下游收到一個**從未發生過的事件**。
     */
    @Test
    @DisplayName("T4｜交易 rollback → outbox 列必須不存在（真 DB 原子性）")
    void t4_rollbackRemovesTheOutboxRow() {
        RuntimeException boom = new RuntimeException("business failure after enqueue");

        RuntimeException thrown = assertThrows(RuntimeException.class, () -> tx.execute(status -> {
            writer.enqueue("payment.events", "payment.success", "t4.rk", "{\"b\":2}");
            // 對照：交易**還沒**結束時它是看得見的（同一個交易內讀得到自己的寫入）
            assertEquals(1L, countByRoutingKey("t4.rk"),
                    "交易內應該讀得到自己剛寫的列——讀不到代表這個測試根本沒寫進去，後面的斷言就沒有意義");
            throw boom;
        }));

        assertEquals(boom, thrown);
        assertEquals(0L, countByRoutingKey("t4.rk"),
                "🔴 rollback 之後那一列必須消失。它若還在，代表 outbox INSERT 沒有參與外層交易，"
                        + "下游會收到一個從未發生過的事件");
    }

    /**
     * 對照組：證明上一支的 0 不是因為「這個測試從頭到尾都寫不進去」。
     * 同一支測試內先 rollback 一筆、再 commit 一筆，兩者結果必須相反。
     */
    @Test
    @DisplayName("對照組｜同一環境下 rollback 的消失、commit 的留下（證明查詢有鑑別力）")
    void controlGroup_rollbackVsCommitInSameEnvironment() {
        assertThrows(RuntimeException.class, () -> tx.execute(status -> {
            writer.enqueue("payment.events", "payment.success", "ctl.rollback", "{}");
            throw new RuntimeException("rollback this one");
        }));

        tx.execute(status -> writer.enqueue("payment.events", "payment.success", "ctl.commit", "{}"));

        assertEquals(0L, countByRoutingKey("ctl.rollback"), "rollback 的那筆應消失");
        assertEquals(1L, countByRoutingKey("ctl.commit"), "commit 的那筆應留下");
    }

    // ───────────────── 真環境下的閘門（T1 的 integration 對照） ─────────────────

    @Test
    @DisplayName("交易外呼叫（真 DataSource 在場）→ 仍然 fail-fast，DB 零新增列")
    void outsideTransaction_failsFastEvenWithRealDataSource() {
        long before = outboxMapper.selectCount(null);

        IllegalStateException ex = assertThrows(IllegalStateException.class,
                () -> writer.enqueue("payment.events", "payment.success", "no.tx.rk", "{}"));

        assertTrue(ex.getMessage().contains("requires an active transaction"), ex.getMessage());
        assertEquals(before, outboxMapper.selectCount(null),
                "閘門在 insert 之前 ⇒ DB 的列數不得改變");
        assertEquals(0L, countByRoutingKey("no.tx.rk"));
    }
}

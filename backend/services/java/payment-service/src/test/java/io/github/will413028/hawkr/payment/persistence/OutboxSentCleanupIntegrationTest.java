package io.github.will413028.hawkr.payment.persistence;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.payment.scheduler.DataCleanupScheduler;
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
import org.springframework.context.annotation.Import;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.test.context.TestPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.time.LocalDateTime;
import java.time.ZoneOffset;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * outbox_events 的 SENT retention 清理（WP1-R1 支線 F1）。
 *
 * <p><b>要守住的不變式</b>：<b>只有 SENT 會被刪</b>。
 * PENDING／FAILED 仍在派送候選集合內（刪了等於事件蒸發）；
 * 🔴 <b>DEAD 一列都不能刪</b>——它是 permanent delivery failure 的證據，也是
 * {@code payment_outbox_dead_rows} 告警的來源，自動刪掉會讓「事故沒解決但告警自己恢復綠」。
 *
 * <p><b>為什麼要真 DB</b>：本檔驗的是 {@code @Delete} 的述詞本身。mock mapper 只能證明
 * 「我呼叫了它」，證明不了那句 SQL 刪掉了哪些列、留下了哪些列。
 *
 * <p>retention 設為 <b>3 天</b>（非預設的 7）是刻意的：它讓 {@link #c7_schedulerUsesConfiguredRetention()}
 * 能分辨「讀設定」與「硬寫 7」——若 scheduler 把天數寫死在 Java 裡，那一支會轉紅。
 */
@SpringBootTest(classes = OutboxSentCleanupIntegrationTest.MinimalConfig.class)
@Testcontainers
@Tag("integration")
@TestPropertySource(properties = "payment.cleanup.outbox-sent-retention-days=3")
@DisplayName("outbox SENT retention 清理")
class OutboxSentCleanupIntegrationTest {

    private static final String SENT = "8222";
    private static final String PENDING = "8221";
    private static final String FAILED = "8223";
    private static final String DEAD = "8224";

    /** 與 @TestPropertySource 的值一致，硬寫字面值（不從被測物讀，否則期望值會跟著被測物一起變）。 */
    private static final int RETENTION_DAYS = 3;

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("payment_outbox_cleanup_test")
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
            SqlInitializationAutoConfiguration.class,
            FlywayAutoConfiguration.class,
            MybatisPlusAutoConfiguration.class,
            MyBatisPlusAutoConfiguration.class
    })
    @MapperScan("io.github.will413028.hawkr.payment.infrastructure.persistence.mapper")
    @Import(DataCleanupScheduler.class)
    static class MinimalConfig {
    }

    @Autowired
    private OutboxEventMapper outboxMapper;

    @Autowired
    private DataCleanupScheduler scheduler;

    private static LocalDateTime now() {
        return LocalDateTime.now(ZoneOffset.UTC);
    }

    @BeforeEach
    void clean() {
        outboxMapper.delete(null);
    }

    @AfterEach
    void cleanAfter() {
        outboxMapper.delete(null);
    }

    /** 造一列。sentAt 為 null 代表「尚未送出」。 */
    private void row(String routingKey, String status, LocalDateTime createdAt, LocalDateTime sentAt) {
        OutboxEventEntity e = new OutboxEventEntity();
        e.setExchange("payment.events");
        e.setEventType("payment.success");
        e.setRoutingKey(routingKey);
        e.setPayload("{}");
        e.setStatus(status);
        e.setRetryCount(0);
        e.setCreatedAt(createdAt);
        e.setSentAt(sentAt);
        outboxMapper.insert(e);
    }

    private long count(String routingKey) {
        return outboxMapper.selectCount(new QueryWrapper<OutboxEventEntity>().eq("routing_key", routingKey));
    }

    /** 直接用 mapper 的述詞（retention 界線由本測試給，與 scheduler 的 config 無關）。 */
    private int deleteWithCutoff(LocalDateTime cutoff) {
        return outboxMapper.deleteSentBefore(cutoff);
    }

    // ───────────────── C1～C6：述詞本身 ─────────────────

    @Test
    @DisplayName("C1～C6｜只有超過保留期的 SENT 被刪，PENDING／FAILED／DEAD 與新的 SENT 全部留下")
    void c1ToC6_onlyStaleSentRowsAreDeleted() {
        LocalDateTime cutoff = now().minusDays(RETENTION_DAYS);
        LocalDateTime old = cutoff.minusDays(1);      // 界線之前
        LocalDateTime recent = cutoff.plusDays(1);    // 界線之後

        row("c1.old-sent", SENT, old, old);                 // C1 → 應刪
        row("c2.recent-sent", SENT, recent, recent);        // C2 → 不刪
        row("c3.old-pending", PENDING, old, null);          // C3 → 不刪
        row("c4.old-failed", FAILED, old, null);            // C4 → 不刪
        row("c5.old-dead", DEAD, old, null);                // C5 → 🔴 不刪
        row("c6.boundary-sent", SENT, cutoff, cutoff);      // C6 → 界線上，`<` ⇒ 不刪

        int deleted = deleteWithCutoff(cutoff);

        assertEquals(1, deleted, "只有 C1 那一列該被刪");
        assertEquals(0L, count("c1.old-sent"), "C1 過期 SENT 應被刪除");
        assertEquals(1L, count("c2.recent-sent"), "C2 保留期內的 SENT 不得刪");
        assertEquals(1L, count("c3.old-pending"), "C3 PENDING 仍在派送候選集合內，刪它等於事件蒸發");
        assertEquals(1L, count("c4.old-failed"), "C4 FAILED 仍在派送候選集合內，刪它等於事件蒸發");
        assertEquals(1L, count("c5.old-dead"),
                "🔴 C5 DEAD 一列都不能刪：它是事故證據且是 payment_outbox_dead_rows 的來源，"
                        + "自動刪掉會讓事故沒解決但告警自己恢復綠");
        assertEquals(1L, count("c6.boundary-sent"),
                "C6 界線上的列不刪（述詞是嚴格小於 `<`，與同服務既有三支 cleanup 一致）");
    }

    /**
     * C5 的加強版：即使 DEAD 已經非常久（遠超過 outbox-dead-retention-days 的 30 天），
     * 本清理路徑仍然一列都不能碰。
     */
    @Test
    @DisplayName("C5+｜DEAD 即使超過 30 天（dead retention 設定值）也不得被 SENT 清理刪到")
    void c5plus_veryOldDeadIsStillKept() {
        row("c5p.ancient-dead", DEAD, now().minusDays(400), now().minusDays(400));

        assertEquals(0, deleteWithCutoff(now().minusDays(3)), "不該刪到任何列");
        assertEquals(1L, count("c5p.ancient-dead"),
                "outbox-dead-retention-days 刻意未接線；DEAD 只能由人工處置後清除");
    }

    /**
     * sent_at 為 NULL 的 SENT 列（理論上不存在——唯一寫 SENT 的路徑同句必設 sent_at，
     * 且 dev 實查 155/155 非 NULL）。若真的出現，方向必須是 fail-safe：**不刪**。
     */
    @Test
    @DisplayName("SENT 但 sent_at 為 NULL → 不刪（fail-safe：拿不準的列留著）")
    void sentWithNullSentAt_isNotDeleted() {
        row("nullsent.rk", SENT, now().minusDays(400), null);

        assertEquals(0, deleteWithCutoff(now().minusDays(3)));
        assertEquals(1L, count("nullsent.rk"), "NULL 比較不成立 ⇒ 不會被刪，方向正確");
    }

    // ───────────────── C7：scheduler 用設定值，不是硬寫 ─────────────────

    /**
     * 🔴 本測試把 retention 設成 <b>3</b>（非預設的 7）。
     * 造兩列 SENT：一列 5 天前、一列 2 天前。
     * <ul>
     *   <li>讀設定（3 天）⇒ 刪 5 天前那列、留 2 天前那列 ✅</li>
     *   <li>硬寫 7 天 ⇒ 兩列都留下 ⇒ 本測試轉紅</li>
     * </ul>
     */
    @Test
    @DisplayName("C7｜scheduler 用既有 retention config（設 3 天時，5 天前的刪、2 天前的留）")
    void c7_schedulerUsesConfiguredRetention() {
        row("c7.five-days", SENT, now().minusDays(5), now().minusDays(5));
        row("c7.two-days", SENT, now().minusDays(2), now().minusDays(2));

        scheduler.cleanup();

        assertEquals(0L, count("c7.five-days"),
                "5 天 > 設定的 3 天 ⇒ 應被刪。若它還在，代表 scheduler 用的不是 "
                        + "payment.cleanup.outbox-sent-retention-days（可能硬寫了 7）");
        assertEquals(1L, count("c7.two-days"),
                "2 天 < 設定的 3 天 ⇒ 不得刪。若它消失了，代表 cutoff 算錯或述詞不對");
    }
}

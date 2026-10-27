package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.WorkerTaskStatus;
import io.github.will413028.hawkr.order.entity.GrantPaymentTaskEntity;
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
import java.util.concurrent.Callable;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * backend#3262 撥款 hold 在<b>真實 PostgreSQL</b> 上的行為。
 *
 * <p>為什麼要真 DB：{@link GrantPaymentTaskRepositoryPayoutHoldSqlTest} 只看得到 SQL 的形狀，
 * 看不到「推遲之後這一列到底還會不會被 {@code findPendingTasks} 撈出來」——而那正是本修法
 * 「撥款被擋住」與「窗過之後自動放行」兩個保證的機制本體。SQL 形狀對、候選述詞沒把時間算進去，
 * 形狀測試照樣全綠而撥款照撥。
 *
 * <p>覆蓋（對應 issue Regression Case）：
 * <ul>
 *   <li><b>Case C/F</b> 兩個 replica 同時處理同一筆：claim 只有一個成功；推遲 CAS 只有一個成功</li>
 *   <li><b>Case E</b> 推遲時點到了之後，任務<b>必然</b>重新進候選（不永久 block）</li>
 *   <li><b>Case G</b> crash 後任務停在 PROCESSING：租約到期即可重新 claim，重新過閘</li>
 *   <li>推遲不動 {@code retry_count}（等待不得被誤計為失敗）</li>
 * </ul>
 */
@SpringBootTest(classes = GrantPaymentPayoutHoldIT.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class GrantPaymentPayoutHoldIT {

    private static final String ORDER_NO = "O3262IT0001";
    private static final long TASK_ID = 900000000000001L;

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("grant_payout_hold_test")
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
    private GrantPaymentTaskRepository repo;

    @Autowired
    private JdbcTemplate jdbc;

    @BeforeEach
    void reset() {
        jdbc.update("DELETE FROM grant_payment_tasks");
    }

    /** 建一筆全新的 PENDING 撥款任務（＝訂單完成同事務建立的形狀：next_retry_time 為 NULL）。 */
    private void freshTask() {
        jdbc.update("""
                INSERT INTO grant_payment_tasks (task_id, order_no, confirm_type, status, retry_count, max_retry_count)
                VALUES (?, ?, 'MANUAL', ?, 0, 10)
                """, TASK_ID, ORDER_NO, WorkerTaskStatus.PENDING.value());
    }

    private GrantPaymentTaskEntity row() {
        return repo.selectById(TASK_ID);
    }

    private List<Long> candidateIds() {
        return repo.findPendingTasks(20).stream().map(GrantPaymentTaskEntity::getTaskId).toList();
    }

    // ==================== 核心保證：推遲之後撈不到，時間到又撈得到 ====================

    @Test
    @DisplayName("新建任務立即是候選（＝修復前 30 秒內撥款的那條路徑，本測試錨定它沒被改壞）")
    void freshTaskIsImmediatelyCandidate() {
        freshTask();
        assertThat(candidateIds()).containsExactly(TASK_ID);
    }

    @Test
    @DisplayName("Case C：推遲到 72h 後 ⇒ 立刻退出候選集合（撥款真的被擋住，不只是 SQL 形狀對）")
    void deferredTaskLeavesCandidateSet() {
        freshTask();
        assertThat(repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120))).isEqualTo(1);

        LocalDateTime holdUntil = DateTimeUtils.nowUtc().plusHours(72);
        assertThat(repo.deferForPayoutHold(TASK_ID, holdUntil, "payout hold: APPEAL_WINDOW_OPEN")).isEqualTo(1);

        assertThat(candidateIds()).isEmpty();
        GrantPaymentTaskEntity t = row();
        assertThat(t.getStatus()).isEqualTo(WorkerTaskStatus.PENDING.value());
        assertThat(t.getRetryCount()).isZero();
        assertThat(t.getLastError()).contains("APPEAL_WINDOW_OPEN");
    }

    @Test
    @DisplayName("Case E：推遲時點一過，任務必然重新進候選 ⇒ 賣家款項不會永久卡住")
    void deferredTaskReturnsAfterHoldExpires() {
        freshTask();
        repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120));
        // 推到「1 秒前」＝模擬 hold 已到期（不 sleep，直接把時點放在過去）
        repo.deferForPayoutHold(TASK_ID, DateTimeUtils.nowUtc().minusSeconds(1), "payout hold: expired");

        assertThat(candidateIds())
                .as("hold 到期後必須回到候選集合，否則就是 issue comment #4 說的『撥款永久卡死』")
                .containsExactly(TASK_ID);
    }

    @Test
    @DisplayName("推遲不消耗重試額度：retry_count 維持原值（等待不得被誤計為失敗而推進人工介入）")
    void deferDoesNotBurnRetryCount() {
        freshTask();
        jdbc.update("UPDATE grant_payment_tasks SET retry_count = 7 WHERE task_id = ?", TASK_ID);
        repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120));

        repo.deferForPayoutHold(TASK_ID, DateTimeUtils.nowUtc().plusHours(1), "payout hold: ACTIVE_CASE");

        assertThat(row().getRetryCount()).isEqualTo(7);
    }

    // ==================== Case F：多副本競態 ====================

    @Test
    @DisplayName("Case F：兩個 replica 同時 claim ⇒ 只有一個拿到租約（既有 lease CAS，本修法未破壞）")
    void concurrentClaimOnlyOneWins() throws Exception {
        freshTask();
        LocalDateTime lease = DateTimeUtils.nowUtc().plusSeconds(120);
        List<Integer> results = runConcurrently(
                () -> repo.claimTask(TASK_ID, lease),
                () -> repo.claimTask(TASK_ID, lease));
        assertThat(results.stream().filter(r -> r == 1).count()).isEqualTo(1);
    }

    @Test
    @DisplayName("Case F：租約已易主後遲到的推遲 ⇒ CAS 回 0，不得覆蓋新持有者的租約")
    void lateDeferAfterClaimLostIsRejected() {
        freshTask();
        repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120));
        // pod A 推遲成功 → 列回到 PENDING
        assertThat(repo.deferForPayoutHold(TASK_ID, DateTimeUtils.nowUtc().plusHours(1), "A")).isEqualTo(1);
        // pod B（同一輪的落後者）再推一次 → status 已非 PROCESSING ⇒ 0
        assertThat(repo.deferForPayoutHold(TASK_ID, DateTimeUtils.nowUtc().plusHours(9), "B")).isZero();
        assertThat(row().getLastError()).isEqualTo("A");
    }

    @Test
    @DisplayName("Case F：兩個 replica 同時推遲 ⇒ 只有一個 CAS 成功（不重複記帳）")
    void concurrentDeferOnlyOneWins() throws Exception {
        freshTask();
        repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120));
        LocalDateTime until = DateTimeUtils.nowUtc().plusHours(1);
        List<Integer> results = runConcurrently(
                () -> repo.deferForPayoutHold(TASK_ID, until, "X"),
                () -> repo.deferForPayoutHold(TASK_ID, until, "Y"));
        assertThat(results.stream().filter(r -> r == 1).count()).isEqualTo(1);
    }

    // ==================== Case G：crash / restart ====================

    @Test
    @DisplayName("Case G：pod 在閘門與撥款之間 crash（列停在 PROCESSING）⇒ 租約到期後可重新 claim，重新過閘")
    void crashedTaskIsReclaimableAfterLeaseExpiry() {
        freshTask();
        // 模擬 crash：claim 過、租約已過期、狀態停在 PROCESSING
        jdbc.update("""
                UPDATE grant_payment_tasks
                   SET status = ?, next_retry_time = NOW() - INTERVAL '1 second'
                 WHERE task_id = ?
                """, WorkerTaskStatus.PROCESSING.value(), TASK_ID);

        assertThat(candidateIds())
                .as("PROCESSING ＋ 租約過期必須回到候選，否則 crash 一次就永遠不撥款")
                .containsExactly(TASK_ID);
        assertThat(repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120)))
                .as("重新 claim 成功 ⇒ worker 會再跑一次閘門（資格在執行點重算）")
                .isEqualTo(1);
    }

    @Test
    @DisplayName("Case G：crash 發生在推遲之後（列已是 PENDING + 未來時點）⇒ 重啟不會提前撥款")
    void crashAfterDeferKeepsHold() {
        freshTask();
        repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120));
        repo.deferForPayoutHold(TASK_ID, DateTimeUtils.nowUtc().plusHours(72), "payout hold");

        // 重啟＝重新掃候選，沒有任何記憶體狀態參與判定
        assertThat(candidateIds()).isEmpty();
    }

    // ==================== admin 救濟與閘門的關係 ====================

    @Test
    @DisplayName("admin resetForRetry 收不到被 hold 住的 PENDING 任務（不會意外把 hold 解除）")
    void adminResetDoesNotBypassHold() {
        freshTask();
        repo.claimTask(TASK_ID, DateTimeUtils.nowUtc().plusSeconds(120));
        repo.deferForPayoutHold(TASK_ID, DateTimeUtils.nowUtc().plusHours(72), "payout hold");

        assertThat(repo.resetForRetry(ORDER_NO)).isZero();
        assertThat(candidateIds()).isEmpty();
    }

    @Test
    @DisplayName("admin 對 FAILED 任務重發 ⇒ 任務回候選，但 worker 下一輪仍會重新過閘（閘門在執行點）")
    void adminResetOnFailedTaskStillPassesThroughGate() {
        freshTask();
        jdbc.update("UPDATE grant_payment_tasks SET status = ?, next_retry_time = NOW() + INTERVAL '1 hour' WHERE task_id = ?",
                WorkerTaskStatus.FAILED.value(), TASK_ID);
        assertThat(candidateIds()).isEmpty();

        assertThat(repo.resetForRetry(ORDER_NO)).isEqualTo(1);
        assertThat(candidateIds()).containsExactly(TASK_ID);
        // 回到候選只代表 worker 會處理它；worker 的第一件事是 claim → 反查資格
        // （順序由 GrantPaymentWorkerPayoutHoldTest.gateSitsBetweenClaimAndPayment 守）
    }

    private List<Integer> runConcurrently(Callable<Integer> a, Callable<Integer> b) throws Exception {
        ExecutorService pool = Executors.newFixedThreadPool(2);
        try {
            Future<Integer> fa = pool.submit(a);
            Future<Integer> fb = pool.submit(b);
            return List.of(fa.get(), fb.get());
        } finally {
            pool.shutdownNow();
        }
    }
}

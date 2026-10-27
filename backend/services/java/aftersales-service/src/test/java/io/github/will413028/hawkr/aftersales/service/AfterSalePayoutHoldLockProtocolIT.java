package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.AfterEach;
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
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.PreparedStatement;
import java.sql.Statement;
import java.time.LocalDateTime;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * backend#3262 P0 修復的<b>語義證明</b>：撥款放行與「一筆已取得申訴資格的在途交易」
 * 在真實 PostgreSQL 上<b>不可能同時成立</b>。
 *
 * <h3>它取代了什麼</h3>
 * 獨立 QA 的 {@code Qa3262PayoutGraceRaceReproIT} 證明了舊版的缺口：deadline 已過 28 小時、
 * 遠超 15 分鐘 grace，{@code getPayoutHold} 回 {@code hold=false}（撥款發動）的<b>同一時刻</b>，
 * {@code markAppealed} 仍回 {@code UPDATE 1} ⇒ 錢撥出去了、申訴還能成立、全 repo 無扣回機制。
 * 那個缺口的根因是「用交易耗時的上界證明資金安全」，而該交易<b>沒有</b>可證明的上界。
 *
 * <h3>新的證明形狀</h3>
 * 改成<b>順序保證</b>：{@code submitAppeal} 在做任何資格判定之前先
 * {@code SELECT … FOR UPDATE} 鎖住該 case（持有到 commit/rollback）；撥款閘門在因
 * 「窗已過」而放行之前先以 {@code FOR UPDATE SKIP LOCKED} 探測同一列。於是：
 * <ul>
 *   <li>鎖被持有期間 ⇒ 閘門 fail-closed，撥款不可能發動（{@link #gateNeverReleasesWhileAppealTransactionIsInFlight}）</li>
 *   <li>鎖釋放後只有兩種結果 ⇒ 申訴已 commit（閘門看得到 {@code appeal_state='10'}，無時間界 hold）
 *       或已 rollback（deadline 已過，A3 會擋下重試 ⇒ 申訴永遠不可能成立）</li>
 * </ul>
 * <b>兩條都不依賴任何耗時假設</b>——這正是與舊版的本質差異。
 *
 * <p>⚠️ 本測試需要真實 PostgreSQL 的 row lock 語義（{@code FOR UPDATE} / {@code SKIP LOCKED}），
 * 無法以 mock 取代 ⇒ {@code @Tag("integration")}，CI 預設不跑（見 pom 的 surefire 註解）。
 * CI 跑得到的等價守衛是 {@code AfterSalePayoutHoldServiceTest} 與
 * {@code AfterSalePayoutHoldBoundaryTest}（探測邏輯層），本檔守的是「PostgreSQL 真的這樣行為」。
 */
@Testcontainers
@SpringBootTest(classes = AfterSalePayoutHoldLockProtocolIT.TestBoot.class)
@Tag("integration")
class AfterSalePayoutHoldLockProtocolIT {

    private static final String CASE_NO = "AS-P0-LOCK";
    private static final String ORDER_NO = "O-P0-LOCK";

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:15-alpine")
            .withDatabaseName("aftersales_test").withUsername("test").withPassword("test")
            .withEnv("TZ", "UTC").withEnv("PGTZ", "UTC");

    @DynamicPropertySource
    static void props(DynamicPropertyRegistry r) {
        r.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        r.add("spring.datasource.username", POSTGRES::getUsername);
        r.add("spring.datasource.password", POSTGRES::getPassword);
        r.add("spring.datasource.driver-class-name", () -> "org.postgresql.Driver");
        r.add("hawkr.db.bootstrap.enabled", () -> "false");
        r.add("spring.flyway.enabled", () -> "false");
        r.add("hawkr.tsid.enabled", () -> "false");
        // -1 關掉對外 gRPC server（本測試用不到）。0 是「先找空 port 再綁」，
        // 中間可能被 testcontainers 的 port 映射搶走（CI 實測 Address already in use）。
        r.add("grpc.server.port", () -> "-1");
        r.add("spring.cloud.nacos.discovery.enabled", () -> "false");
        r.add("spring.cloud.discovery.enabled", () -> "false");
        r.add("spring.cloud.service-registry.auto-registration.enabled", () -> "false");
    }

    @BeforeAll
    static void migrate() {
        Flyway.configure()
                .dataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword())
                .locations("classpath:db/migration").load().migrate();
    }

    @SpringBootConfiguration
    @EnableAutoConfiguration
    @Import(MyBatisPlusAutoConfiguration.class)
    @MapperScan("io.github.will413028.hawkr.aftersales.mapper")
    static class TestBoot {}

    @Autowired private AfterSaleCaseMapper mapper;
    @Autowired private JdbcTemplate jdbc;
    @Autowired private TransactionTemplate txTemplate;

    private AfterSalePayoutHoldService payoutHold;
    /** 模擬「另一個 pod 上正在跑 submitAppeal 的那個交易」，必須是獨立連線。 */
    private Connection appealTx;

    @BeforeEach
    void setUp() throws Exception {
        jdbc.execute("TRUNCATE TABLE aftersale_cases RESTART IDENTITY CASCADE");
        // 生產預設值：deadlineHours=72、payoutHoldGraceMinutes=15
        payoutHold = new AfterSalePayoutHoldService(mapper, new AfterSalesProperties());
        appealTx = DriverManager.getConnection(
                POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
        appealTx.setAutoCommit(false);
    }

    @AfterEach
    void tearDown() throws Exception {
        if (appealTx != null && !appealTx.isClosed()) {
            appealTx.rollback();
            appealTx.close();
        }
    }

    // ==================== P0 主證明 ====================

    @Test
    @DisplayName("P0：申訴交易在途（已通過 A3、持鎖未 commit）期間，撥款閘門一律不放行——不論過了多久")
    void gateNeverReleasesWhileAppealTransactionIsInFlight() throws Exception {
        // 駁回於 100 小時前：deadline 已過 28 小時，遠超 15 分鐘 grace。
        // 這正是 QA 重現用的時序——舊版在這一刻無條件放行。
        insertRejectedCase(DateTimeUtils.nowUtc().minusHours(100));

        // 前提：無人持鎖時，這一刻本來就是「該放行」的 ⇒ 證明後面的 hold 不是恆真
        assertFalse(payoutHold.getPayoutHold(ORDER_NO).hold(),
                "前提：窗（含 grace）早已過，無在途申訴時必須放行");

        // === 另一個 pod 開始跑 submitAppeal：先持鎖，才做資格判定（A1/A2/A3）===
        lockCaseRowInAppealTransaction();

        // 撥款閘門在這一刻被 GrantPaymentWorker 呼叫：必須 fail-closed
        AfterSalePayoutHoldService.PayoutHold duringAppeal = payoutHold.getPayoutHold(ORDER_NO);
        assertTrue(duringAppeal.hold(),
                "P0 核心：有交易正壓在這一列上（可能已通過 A3）⇒ 撥款絕對不可放行");
        assertEquals(AfterSalePayoutHoldService.HoldReason.APPEAL_IN_FLIGHT, duringAppeal.reason());
        assertNull(duringAppeal.holdUntil(), "在途申訴無可預期結束時點 ⇒ 無時間界，推遲後重問");

        // === 申訴交易繼續跑（media gRPC / DB / MQ 都在這之間），最後 commit ===
        assertEquals(1, markAppealedInAppealTransaction(),
                "買家權利不得縮減：通過 A3 之後不論交易跑多久，申訴都必須寫得進去");
        appealTx.commit();

        // commit 之後閘門看得到申訴 ⇒ 轉為無時間界 hold，等 Admin 裁決
        AfterSalePayoutHoldService.PayoutHold afterCommit = payoutHold.getPayoutHold(ORDER_NO);
        assertTrue(afterCommit.hold(), "申訴已成立 ⇒ 撥款仍不得放行");
        assertEquals(AfterSalePayoutHoldService.HoldReason.ACTIVE_CASE, afterCommit.reason());
        assertNull(afterCommit.holdUntil());

        // 全程沒有任何一個時點同時滿足「閘門放行」與「申訴仍可成立」——這就是要證的不變式
    }

    @Test
    @DisplayName("P0 另一半：在途申訴 rollback 之後，閘門恢復放行（不會因為探測而永久卡死）")
    void gateResumesReleaseAfterAppealTransactionRollsBack() throws Exception {
        insertRejectedCase(DateTimeUtils.nowUtc().minusHours(100));
        lockCaseRowInAppealTransaction();
        assertTrue(payoutHold.getPayoutHold(ORDER_NO).hold(), "持鎖期間擋住");

        appealTx.rollback();

        // rollback ⇒ 申訴沒落地，而 deadline 早就過了 ⇒ 買家重試會被 A3 擋下
        // ⇒ 這張訂單再也不可能生出退款 ⇒ 必須放行，不得讓賣家款項卡死
        assertFalse(payoutHold.getPayoutHold(ORDER_NO).hold(),
                "鎖釋放且申訴未落地 ⇒ 必須恢復放行");
    }

    @Test
    @DisplayName("探測不得等待：持鎖者存在時閘門必須立刻返回（否則耗時會灌進撥款 worker 的租約下界）")
    void probeIsNonBlocking() throws Exception {
        insertRejectedCase(DateTimeUtils.nowUtc().minusHours(100));
        lockCaseRowInAppealTransaction();

        long startNanos = System.nanoTime();
        assertTrue(payoutHold.getPayoutHold(ORDER_NO).hold());
        long elapsedMillis = (System.nanoTime() - startNanos) / 1_000_000L;

        // SKIP LOCKED 立即返回；若誤用 FOR UPDATE（等待）這裡會掛到 appealTx 結束為止。
        // 取 2 秒是寬鬆上界——真值是毫秒級，而 5 秒是 PAYOUT_HOLD_DEADLINE_SEC 的預算。
        assertTrue(elapsedMillis < 2_000,
                "閘門必須非阻塞（實際 " + elapsedMillis + "ms）：等待會讓持鎖者的耗時進入"
                        + "《多副本排程搶佔規範》§三第 5 步的租約下界算式，兩個 pod 就可能同時撥款");
    }

    @Test
    @DisplayName("對照：窗內的 case 本來就擋（有時間界），與持鎖探測無關")
    void withinWindow_stillBoundedHold() {
        insertRejectedCase(DateTimeUtils.nowUtc().minusHours(71));
        AfterSalePayoutHoldService.PayoutHold h = payoutHold.getPayoutHold(ORDER_NO);
        assertTrue(h.hold(), "窗內必須擋");
        assertEquals(AfterSalePayoutHoldService.HoldReason.APPEAL_WINDOW_OPEN, h.reason());
        assertTrue(h.holdUntil() != null, "窗內的 hold 有到期日");
    }

    @Test
    @DisplayName("P0 端到端：兩端都走 production mapper 方法——submitAppeal 的持鎖讀真的鎖得住，閘門的探測真的偵測得到")
    void productionMapperMethodsFormTheProtocol() throws Exception {
        insertRejectedCase(DateTimeUtils.nowUtc().minusHours(100));

        CountDownLatch locked = new CountDownLatch(1);
        CountDownLatch release = new CountDownLatch(1);
        ExecutorService pool = Executors.newSingleThreadExecutor();
        try {
            // 另一條「submitAppeal」：在真實交易內呼叫 production 的持鎖讀，然後把交易掛著
            Future<AfterSaleCaseEntity> appealTxTask = pool.submit(() -> txTemplate.execute(status -> {
                AfterSaleCaseEntity held = mapper.selectByCaseNoForUpdate(CASE_NO);
                locked.countDown();
                try {
                    release.await(30, TimeUnit.SECONDS);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                }
                return held;
            }));

            assertTrue(locked.await(10, TimeUnit.SECONDS), "持鎖讀應在 10 秒內完成");

            // 閘門此刻被呼叫：探測（production 的 tryLockCaseForPayoutRelease）必須偵測到
            AfterSalePayoutHoldService.PayoutHold duringAppeal = payoutHold.getPayoutHold(ORDER_NO);
            assertTrue(duringAppeal.hold(),
                    "production 的 selectByCaseNoForUpdate 必須真的持有 row lock，"
                            + "且 production 的 tryLockCaseForPayoutRelease 必須偵測得到");
            assertEquals(AfterSalePayoutHoldService.HoldReason.APPEAL_IN_FLIGHT, duringAppeal.reason());

            release.countDown();
            AfterSaleCaseEntity held = appealTxTask.get(30, TimeUnit.SECONDS);

            // 順帶驗欄位映射：SELECT * → entity 的三個判定用欄位都要正確
            // （映射壞掉時 status 會是 null ⇒ payoutBlock 走 deny-by-default，
            //   症狀是「永遠 hold」而不是報錯，很難察覺）
            assertEquals(CASE_NO, held.getCaseNo());
            assertEquals("60", held.getStatus());
            assertEquals("0", held.getAppealState());
            assertTrue(held.getLastRejectedAt() != null, "last_rejected_at 必須映射得到（申訴截止的起算點）");
        } finally {
            release.countDown();
            pool.shutdownNow();
        }

        // 交易結束、鎖釋放、申訴未落地 ⇒ 恢復放行
        assertFalse(payoutHold.getPayoutHold(ORDER_NO).hold(), "鎖釋放後必須恢復放行");
    }

    // ==================== helper ====================

    /** 模擬 submitAppeal 的第一個動作：持鎖讀（之後才做 A1/A2/A3）。 */
    private void lockCaseRowInAppealTransaction() throws Exception {
        try (Statement st = appealTx.createStatement()) {
            st.execute("SELECT * FROM aftersale_cases WHERE case_no = '" + CASE_NO + "' FOR UPDATE");
        }
    }

    /** 模擬 submitAppeal 的 CAS（與 AfterSaleCaseMapper#markAppealed 同述詞）。 */
    private int markAppealedInAppealTransaction() throws Exception {
        try (PreparedStatement ps = appealTx.prepareStatement("""
                UPDATE aftersale_cases
                   SET appeal_state = '10', appeal_submitted_at = now(),
                       appeal_reason = 'P0', appeal_description = 'lock protocol IT'
                 WHERE case_no = ? AND status IN ('60', '33') AND appeal_state = '0'
                """)) {
            ps.setString(1, CASE_NO);
            return ps.executeUpdate();
        }
    }

    private void insertRejectedCase(LocalDateTime rejectedAt) {
        jdbc.update("""
                INSERT INTO aftersale_cases
                  (id, case_no, order_no, buyer_id, store_id, status, reason, description,
                   refundable_amount, appeal_state, last_rejected_at, created_at)
                VALUES (?, ?, ?, 1, 1, '60', '1101', 'p0 lock protocol', 100, '0', ?, now())
                """, Math.abs(CASE_NO.hashCode()), CASE_NO, ORDER_NO, rejectedAt);
    }
}

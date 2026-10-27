package io.github.will413028.hawkr.aftersales.mapper;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.aftersales.service.AfterSaleLabelRetryService;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import org.flywaydb.core.Flyway;
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
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>L7</b>：7-11 退貨代碼自動重試的 <b>lease claim</b> 在真實 PostgreSQL 上的行為。
 *
 * <p>為什麼要真 DB：Mockito 看不到 {@code affected rows} 的真相，而搶佔的全部語意就是
 * 「兩個 pod 同時打這句 UPDATE，只有一個拿到 1」。租約會不會過期回來、CAS 述詞會不會
 * 誤傷別人的租約，也只有真的跑一次 SQL 才知道。
 *
 * <p>對應規範 {@code docs/開發規範/多副本排程搶佔規範.md}：
 * §一 ①⑦（閘門在副作用前、述詞非純 PK）、§二 第 3–4 步（是 lease 不是永久旗標）、§二 第 6 步
 *（ops 救濟路徑收得到這個狀態）。
 */
@Testcontainers
@SpringBootTest(classes = AfterSaleLabelRetryLeaseIT.TestBoot.class)
@Tag("integration")
class AfterSaleLabelRetryLeaseIT {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:15-alpine")
            .withDatabaseName("aftersales_lease_test")
            .withUsername("test")
            .withPassword("test");

    @DynamicPropertySource
    static void registerProps(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", POSTGRES::getJdbcUrl);
        registry.add("spring.datasource.username", POSTGRES::getUsername);
        registry.add("spring.datasource.password", POSTGRES::getPassword);
        registry.add("spring.datasource.driver-class-name", () -> "org.postgresql.Driver");
        registry.add("hawkr.db.bootstrap.enabled", () -> "false");
        registry.add("spring.flyway.enabled", () -> "false");
        registry.add("hawkr.tsid.enabled", () -> "false");
        // -1 關掉對外 gRPC server（本測試用不到）。0 是「先找空 port 再綁」，
        // 中間可能被 testcontainers 的 port 映射搶走（CI 實測 Address already in use）。
        registry.add("grpc.server.port", () -> "-1");
        registry.add("spring.cloud.nacos.discovery.enabled", () -> "false");
        registry.add("spring.cloud.discovery.enabled", () -> "false");
        registry.add("spring.cloud.service-registry.auto-registration.enabled", () -> "false");
    }

    @BeforeAll
    static void runFlywayMigration() {
        Flyway.configure()
                .dataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword())
                .locations("classpath:db/migration")
                .load()
                .migrate();
    }

    @SpringBootConfiguration
    @EnableAutoConfiguration
    @Import(MyBatisPlusAutoConfiguration.class)
    @MapperScan("io.github.will413028.hawkr.aftersales.mapper")
    static class TestBoot {}

    @Autowired private AfterSaleCaseMapper mapper;
    @Autowired private JdbcTemplate jdbc;

    @BeforeEach
    void cleanCases() {
        jdbc.execute("TRUNCATE TABLE aftersale_cases RESTART IDENTITY CASCADE");
    }

    // ==================== claimLabelRetry：跨副本互斥 ====================

    /**
     * <b>兩個 pod 同一 tick</b>：第一句拿到 1，第二句拿到 0。
     *
     * <p>這是搶佔的全部意義。修復前的閘門（{@code label_retry_count} 0→1 CAS）在這一項上
     * <b>也是對的</b>——所以光有這一支不足以證明修復；下面的租約到期測試才是分水嶺。
     */
    @Test
    @DisplayName("claimLabelRetry：兩個副本同時搶，只有一個拿到 affected=1")
    void claim_isMutuallyExclusiveAcrossReplicas() {
        insertRetryCandidate("L001");
        LocalDateTime now = LocalDateTime.now();

        int first = mapper.claimLabelRetry("L001", now, now.plusMinutes(6));
        int second = mapper.claimLabelRetry("L001", now, now.plusMinutes(6));

        assertEquals(1, first, "第一個副本必須搶到");
        assertEquals(0, second, "第二個副本必須落空——否則兩個 pod 各觸發一次 7-11 列印");
    }

    /**
     * <b>本 issue 的分水嶺</b>：租約到期後該案件會自己回到可搶佔狀態。
     *
     * <p>修復前的閘門是 {@code label_retry_count} 0→1 的永久旗標：pod 在 CAS 與 gRPC 之間消失
     *（或 gRPC 失敗被 catch 吞掉）之後，<b>沒有任何機制</b>能讓這個案件再被自動重試一次。
     * 本測試用「租約已過期」的持有狀態證明現在有：述詞看的是時間，不是旗標。
     */
    @Test
    @DisplayName("claimLabelRetry：租約過期後可重新搶佔（證明是 lease 不是永久旗標）")
    void claim_expiredLeaseIsClaimableAgain() {
        insertRetryCandidate("L002");
        LocalDateTime now = LocalDateTime.now();

        // 前一個持有者寫了一個「已經到期」的租約（＝它 claim 完就消失了）
        assertEquals(1, mapper.claimLabelRetry("L002", now, now.minusMinutes(1)));

        int reclaimed = mapper.claimLabelRetry("L002", now, now.plusMinutes(6));

        assertEquals(1, reclaimed,
                "租約已到期就必須能被重新搶佔；回 0 代表它其實是永久旗標，"
                        + "持有者崩潰後這個案件永遠不會再被自動重試");
    }

    /** 預算用完（count>0）之後不得再被搶佔——那是業務規則「自動只重試一次」。 */
    @Test
    @DisplayName("claimLabelRetry：預算已消耗（count=1）→ 搶不到")
    void claim_budgetConsumed_notClaimable() {
        insertRetryCandidate("L003");
        jdbc.update("UPDATE aftersale_cases SET label_retry_count = 1 WHERE case_no = 'L003'");
        LocalDateTime now = LocalDateTime.now();

        assertEquals(0, mapper.claimLabelRetry("L003", now, now.plusMinutes(6)));
    }

    // ==================== 候選述詞：租約條件真的有生效 ====================

    @Test
    @DisplayName("selectRetryCandidates：租約未到期的案件不在候選集合，到期後回來")
    void candidates_excludeLiveLease_includeExpiredLease() {
        insertRetryCandidate("L004");
        LocalDateTime now = LocalDateTime.now();

        assertTrue(caseNos(mapper.selectRetryCandidates(50)).contains("L004"),
                "尚未被搶佔（label_retry_at IS NULL）時必須是候選");

        assertEquals(1, mapper.claimLabelRetry("L004", now, now.plusMinutes(6)));
        assertTrue(!caseNos(mapper.selectRetryCandidates(50)).contains("L004"),
                "租約未到期時必須退出候選集合——否則同一個 pod 下一 tick 又會重打一次");

        jdbc.update("UPDATE aftersale_cases SET label_retry_at = NOW() - INTERVAL '1 minute' "
                + "WHERE case_no = 'L004'");
        assertTrue(caseNos(mapper.selectRetryCandidates(50)).contains("L004"),
                "租約到期後必須自動回到候選集合");
    }

    // ==================== releaseLabelRetryLease：守衛式還原 ====================

    @Test
    @DisplayName("releaseLabelRetryLease：租約值相符才釋放，不相符（已被他人重新 claim）回 0")
    void releaseLease_onlyWhenLeaseStillMine() {
        insertRetryCandidate("L005");
        LocalDateTime now = LocalDateTime.now().withNano(0);
        LocalDateTime mine = now.plusMinutes(6);
        assertEquals(1, mapper.claimLabelRetry("L005", now, mine));

        // 別人的租約值 → 不得釋放（不能只用 case_no 當條件）
        assertEquals(0, mapper.releaseLabelRetryLease("L005", now.plusMinutes(99), now));
        // 自己的租約值 → 釋放成一個過去的時間
        assertEquals(1, mapper.releaseLabelRetryLease("L005", mine, now));

        assertTrue(caseNos(mapper.selectRetryCandidates(50)).contains("L005"),
                "釋放之後應立刻回到候選集合（下一 tick 就重試，不必等滿租約）");
    }

    /**
     * 🚨 <b>本次修復的核心測試：跨「釋放」與「耗盡掃描」兩支。</b>
     *
     * <p>兩個各自正確的修改組合起來會互相抵銷：
     * <ul>
     *   <li><b>(A)</b> {@code label_retry_count} 降級為「已確認觸發的預算」（只在 gRPC 成功後才 +1）
     *       ⇒ 持續打不通的案件停在 {@code count = 0}；為了讓告警仍撈得到它們，耗盡述詞放寬成
     *       {@code count = 1 OR (count = 0 AND label_retry_at IS NOT NULL)}</li>
     *   <li><b>(B)</b> 失敗時釋放租約以加速恢復</li>
     * </ul>
     * 若 (B) 把租約「還原成 claim 之前的值」，對從未成功觸發過的案件那個值<b>就是 {@code NULL}</b>
     * ⇒ (A) 的第二個分支只在租約被持有的那幾秒成立，而 {@code checkExhausted} 每 10 分鐘才取樣
     * ⇒ <b>取樣幾乎永遠命中不了</b>，{@code LABEL_RETRY_EXHAUSTED} 與「請聯繫客服」的 ops 動線
     * 在「shipping 長時間打不通」時形同不存在 —— 正是 (A) 想避免的結果。
     *
     * <p>既有的兩支測試各自驗 release（回到候選集合）與 exhausted（有租約值就撈得到），
     * <b>都是綠的</b>，缺的正是「release <b>之後</b>還撈不撈得到」這一步。
     *
     * <p>⚠️ <b>本測試刻意走真正的 {@link AfterSaleLabelRetryService#retryAutomatic} 而不是直接打
     * mapper</b>：缺陷同時橫跨「mapper 的 SET 值」與「service 傳哪個引數給它」兩個決定，
     * 直接打 mapper 只能鑑別前者（測試自己把非 null 的值傳進去，就把要驗的東西假設掉了）。
     * 這裡只 mock 掉 shipping gRPC（讓它拋例外＝shipping 打不通），DB 全程是真的。
     */
    @Test
    @DisplayName("claim → gRPC 失敗 → release → 耗盡掃描仍撈得到（釋放不得抹掉偵測證據）")
    void exhausted_stillDetectedAfterLeaseRelease() {
        insertRetryCandidate("L009");
        agedTenMinutes("L009");

        // 案件目前「從未成功觸發過」＝ label_retry_at IS NULL，這正是缺陷成立的前提
        AfterSaleCaseEntity before = loadCase("L009");
        assertTrue(before.getLabelRetryAt() == null, "前提：本案件從未被搶佔過");

        ShippingGrpcClient shipping = mock(ShippingGrpcClient.class);
        when(shipping.retryReturnPrintLabel(anyString()))
                .thenThrow(new RuntimeException("UNAVAILABLE: shipping down"));
        AfterSaleLabelRetryService service =
                new AfterSaleLabelRetryService(mapper, shipping, mock(AfterSaleAuditService.class));

        // 完整失敗路徑：claim → gRPC 失敗 → release
        assertTrue(!service.retryAutomatic(before), "shipping 打不通必須回 false");

        AfterSaleCaseEntity after = loadCase("L009");
        assertEquals((short) 0, after.getLabelRetryCount(),
                "gRPC 失敗不得消耗預算——否則本測試驗的不是 count=0 那條分支");
        assertTrue(after.getLabelRetryAt() != null,
                "🚨 釋放後 label_retry_at 不得回到 NULL——NULL 的語意是「從未嘗試過」，"
                        + "會抹掉耗盡掃描唯一的「試過」證據");

        // 恢復速度沒有被犧牲：下一 tick 仍立刻可重試（這是 release 存在的理由）
        assertTrue(caseNos(mapper.selectRetryCandidates(50)).contains("L009"),
                "釋放後必須立刻回到候選集合，否則等於退回「等滿一個租約」");

        // 偵測條件仍成立：耗盡掃描撈得到，且 CAS 標得動（兩支述詞逐字同步）
        assertTrue(caseNos(mapper.selectExhaustedCandidates(50)).contains("L009"),
                "🚨 release 之後耗盡掃描必須仍撈得到——否則 shipping 長時間打不通時"
                        + "案件每 2 分鐘靜靜重試、只留 error log，LABEL_RETRY_EXHAUSTED 永遠不發，"
                        + "「請聯繫客服」的 ops 動線不會被啟動");
        assertEquals(1, mapper.markLabelRetryExhausted("L009"),
                "撈得到卻標不動 = 每 10 分鐘重複寫一次 audit log");
    }

    private AfterSaleCaseEntity loadCase(String caseNo) {
        return mapper.selectOne(
                new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getCaseNo, caseNo));
    }

    // ==================== 耗盡掃描：兩支述詞必須逐字同步 ====================

    /**
     * 「試過但沒能確認觸發」（count=0 且 {@code label_retry_at IS NOT NULL}）必須被告警撈到。
     * 修復把 count 移到 gRPC 成功之後，這類案件會一直停在 count=0；
     * 若耗盡掃描仍只收 count=1，{@code LABEL_RETRY_EXHAUSTED} 告警永遠不會發。
     */
    @Test
    @DisplayName("selectExhaustedCandidates：試過但未確認觸發（count=0 + 有租約）算耗盡")
    void exhausted_includesAttemptedButUnconfirmed() {
        insertRetryCandidate("L006");
        agedTenMinutes("L006");
        jdbc.update("UPDATE aftersale_cases SET label_retry_at = NOW() - INTERVAL '1 minute' "
                + "WHERE case_no = 'L006'");

        assertTrue(caseNos(mapper.selectExhaustedCandidates(50)).contains("L006"));
        assertEquals(1, mapper.markLabelRetryExhausted("L006"),
                "markLabelRetryExhausted 的述詞必須與 selectExhaustedCandidates 一致，"
                        + "否則案件每 10 分鐘被撈到卻標不動，audit log 重複寫");
    }

    /**
     * <b>反向</b>：從未被搶佔過（{@code label_retry_at IS NULL}）的案件<b>不算</b>耗盡。
     *
     * <p>沒有這個條件，服務停機 10 分鐘期間堆積的案件會在恢復後直接被判耗盡丟給 ops，
     * <b>一次自動重試都沒做</b>——比修復前更差。
     */
    @Test
    @DisplayName("selectExhaustedCandidates：從未被搶佔過（label_retry_at IS NULL）不算耗盡")
    void exhausted_excludesNeverClaimed() {
        insertRetryCandidate("L007");
        agedTenMinutes("L007");

        assertTrue(!caseNos(mapper.selectExhaustedCandidates(50)).contains("L007"),
                "一次都還沒搶佔過的案件不是「耗盡」，它只是還沒輪到——"
                        + "判它耗盡等於剝奪它唯一一次自動重試");
        assertEquals(0, mapper.markLabelRetryExhausted("L007"));
    }

    // ==================== ops 救濟：規範 §二 第 6 步 ====================

    @Test
    @DisplayName("resetLabelRetryCount：同時清掉租約，救濟立即生效而不是等租約到期")
    void reset_clearsLeaseToo() {
        insertRetryCandidate("L008");
        LocalDateTime now = LocalDateTime.now();
        assertEquals(1, mapper.claimLabelRetry("L008", now, now.plusMinutes(6)));
        jdbc.update("UPDATE aftersale_cases SET label_retry_count = 2 WHERE case_no = 'L008'");

        assertEquals(1, mapper.resetLabelRetryCount("L008"));

        AfterSaleCaseEntity after = mapper.selectOne(
                new com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getCaseNo, "L008"));
        assertEquals((short) 0, after.getLabelRetryCount());
        assertTrue(after.getLabelRetryAt() == null,
                "只歸零 count 卻留著未到期的租約，ops 的救濟要等租約到期才生效");
        assertTrue(caseNos(mapper.selectRetryCandidates(50)).contains("L008"));
    }

    // ==================== helpers ====================

    private List<String> caseNos(List<AfterSaleCaseEntity> cases) {
        return cases.stream().map(AfterSaleCaseEntity::getCaseNo).toList();
    }

    /** status=25(RETURN_SHIPPING) + 7-11(3002) + 有 ship_trade_no + 已逾 5 分鐘 = 重試候選。 */
    private void insertRetryCandidate(String caseNo) {
        jdbc.update("""
                INSERT INTO aftersale_cases
                    (id, case_no, order_no, buyer_id, store_id, status,
                     reason, description, refundable_amount, return_shipping_method,
                     payment_method, ship_trade_no, label_retry_count,
                     created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, '25', 'BROKEN', '測試', 1000, '3002',
                        'CREDIT_CARD', ?, 0, NOW() - INTERVAL '6 minutes', NOW())
                """,
                Math.abs((long) caseNo.hashCode() * 100000L),
                caseNo, "O" + caseNo, 2001L, 1001L, "SHIP_" + caseNo);
    }

    /** 把 created_at 推到 10 分鐘之前，讓 selectExhaustedCandidates 撈得到。 */
    private void agedTenMinutes(String caseNo) {
        jdbc.update("UPDATE aftersale_cases SET created_at = NOW() - INTERVAL '11 minutes' "
                + "WHERE case_no = ?", caseNo);
    }
}

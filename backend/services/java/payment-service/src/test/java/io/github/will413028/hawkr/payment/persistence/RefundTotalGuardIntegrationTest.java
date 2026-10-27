package io.github.will413028.hawkr.payment.persistence;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.RefundTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentTransactionMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.RefundTransactionMapper;
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
import java.util.concurrent.atomic.AtomicLong;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * §5.5.5 退款超額三層防護 L2 / L3 的 SQL 整合測試（純 Mockito 驗不到 SQL / trigger，必須真實 PostgreSQL）。
 *
 * 驗證：
 * - L2 sumRefundedAmount 狀態集合 = ('0','1','2')（在途 + 成功），不含 FAILED
 * - L2 hasProcessingRefund 狀態集合 = ('0','1')（REQUESTED + PROCESSING），<b>不含 SUCCEEDED</b>
 * - L3 V2 trigger：同 trade_no 退款總額（在途 + 成功）超過交易金額的 INSERT 被 DB 拒絕
 */
@SpringBootTest(classes = RefundTotalGuardIntegrationTest.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class RefundTotalGuardIntegrationTest {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("payment_refund_guard_test")
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
        // hawkr_payment DB），不覆寫會讓 Flyway 對本機真實 DB no-op、testcontainers 內的
        // 測試 DB 卻從未跑過 migration。
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
            MyBatisPlusAutoConfiguration.class  // common-core：TimestampTzTypeHandler + MetaObjectHandler
    })
    @MapperScan("io.github.will413028.hawkr.payment.infrastructure.persistence.mapper")
    static class MinimalConfig {
    }

    @Autowired
    private PaymentTransactionMapper txMapper;
    @Autowired
    private RefundTransactionMapper refundMapper;

    private final AtomicLong idSeq = new AtomicLong(1_000_000L);

    @BeforeEach
    void clean() {
        refundMapper.delete(null);
        txMapper.delete(null);
    }

    @AfterEach
    void cleanAfter() {
        refundMapper.delete(null);
        txMapper.delete(null);
    }

    private LocalDateTime now() {
        return LocalDateTime.now(ZoneOffset.UTC);
    }

    /** 插入一筆 PAID 交易，回傳 payment id。 */
    private long insertTx(String tradeNo, long amount) {
        long id = idSeq.getAndIncrement();
        PaymentTransactionEntity e = new PaymentTransactionEntity();
        e.setId(id);
        e.setTradeNo(tradeNo);
        e.setOrderId(tradeNo);
        e.setMerId("HAWK9");
        e.setBuyerId("2001");
        e.setPaymentType("2001");   // 信用卡（非 2900，amount > 0）
        e.setAmount(amount);
        e.setProductName("test");
        e.setStatus("1");           // PAID
        e.setCreatedAt(now());
        e.setUpdatedAt(now());
        e.setVersion(0);
        txMapper.insert(e);
        return id;
    }

    private RefundTransactionEntity refund(String tradeNo, long paymentId, long amount, String status) {
        long id = idSeq.getAndIncrement();
        RefundTransactionEntity r = new RefundTransactionEntity();
        r.setId(id);
        r.setRefundNo("RFD" + id);
        r.setPaymentId(paymentId);
        r.setTradeNo(tradeNo);
        r.setOrderId(tradeNo);
        r.setAmount(amount);
        r.setStatus(status);
        r.setReason("test");
        r.setCreatedAt(now());
        r.setUpdatedAt(now());
        r.setVersion(0);
        return r;
    }

    // ==================== L2: sumRefundedAmount 狀態集合 ====================

    @Test
    @DisplayName("L2: sumRefundedAmount 含在途(0/1)+成功(2)，不含 FAILED(3)")
    void sumRefundedAmount_includesInFlightAndSucceeded_excludesFailed() {
        String tradeNo = "T_SUM";
        long pid = insertTx(tradeNo, 10_000L);

        refundMapper.insert(refund(tradeNo, pid, 100L, "0"));  // REQUESTED
        refundMapper.insert(refund(tradeNo, pid, 200L, "1"));  // PROCESSING
        refundMapper.insert(refund(tradeNo, pid, 300L, "2"));  // SUCCEEDED
        refundMapper.insert(refund(tradeNo, pid, 400L, "3"));  // FAILED（不計）

        assertEquals(600L, refundMapper.sumRefundedAmount(tradeNo),
                "應為 100+200+300=600，FAILED 400 不計入");
    }

    // ==================== L2: 排除本筆（getRefundPageUrl 回歸修復）====================

    @Test
    @DisplayName("L2 排除本筆: ATM 全額退款預建 REQUESTED row 後，排除本筆 → alreadyRefunded=0（不誤擋）")
    void sumExcluding_atmFullRefund_excludesOwnRow() {
        String tradeNo = "T_EXCL_FULL";
        long pid = insertTx(tradeNo, 1_000L);

        // ATM 預建：本筆全額退款 REQUESTED('0')，amount == 交易金額
        RefundTransactionEntity own = refund(tradeNo, pid, 1_000L, "0");
        refundMapper.insert(own);

        // 沿用不排除版：把本筆算進去 → 1000（會使 getRefundPageUrl remaining=0 誤擋）
        assertEquals(1_000L, refundMapper.sumRefundedAmount(tradeNo));

        // 排除本筆 → 其他退款已用額度=0 → remaining=1000-0=1000 ≥ 1000，全額退款可取頁
        assertEquals(0L,
                refundMapper.sumRefundedAmountByTradeNoExcluding(tradeNo, own.getRefundNo()),
                "排除本筆後應為 0，全額 ATM 退款不被誤擋");
    }

    @Test
    @DisplayName("L2 排除本筆: 排除本筆後仍計入其他在途+成功退款（不放寬 REF-02）")
    void sumExcluding_stillCountsOtherRefunds() {
        String tradeNo = "T_EXCL_OTHER";
        long pid = insertTx(tradeNo, 1_000L);

        refundMapper.insert(refund(tradeNo, pid, 200L, "2"));  // 其他已成功
        refundMapper.insert(refund(tradeNo, pid, 100L, "1"));  // 其他在途
        RefundTransactionEntity own = refund(tradeNo, pid, 300L, "0");  // 本筆
        refundMapper.insert(own);

        // 排除本筆(300) → 仍計其他 200+100=300（含在途，保住 REF-02 防護）
        assertEquals(300L,
                refundMapper.sumRefundedAmountByTradeNoExcluding(tradeNo, own.getRefundNo()));
    }

    @Test
    @DisplayName("L2 排除本筆: excludeRefundNo=null → 退化為含全部（等同 sumRefundedAmount）")
    void sumExcluding_nullExclude_includesAll() {
        String tradeNo = "T_EXCL_NULL";
        long pid = insertTx(tradeNo, 1_000L);
        refundMapper.insert(refund(tradeNo, pid, 250L, "0"));
        refundMapper.insert(refund(tradeNo, pid, 150L, "2"));

        assertEquals(400L, refundMapper.sumRefundedAmountByTradeNoExcluding(tradeNo, null));
    }

    // ==================== L2: hasProcessingRefund 狀態集合 ====================

    @Test
    @DisplayName("L2: hasProcessingRefund 對 REQUESTED(0) / PROCESSING(1) 為 true")
    void hasProcessingRefund_trueForRequestedAndProcessing() {
        String t0 = "T_REQ";
        long p0 = insertTx(t0, 10_000L);
        refundMapper.insert(refund(t0, p0, 100L, "0"));  // REQUESTED
        assertTrue(refundMapper.hasProcessingRefund(t0), "REQUESTED 應算未終結申請");

        String t1 = "T_PROC";
        long p1 = insertTx(t1, 10_000L);
        refundMapper.insert(refund(t1, p1, 100L, "1"));  // PROCESSING
        assertTrue(refundMapper.hasProcessingRefund(t1), "PROCESSING 應算未終結申請");
    }

    @Test
    @DisplayName("L2: hasProcessingRefund 對 SUCCEEDED(2) 為 false（否則部分退款被永久擋死）")
    void hasProcessingRefund_falseForSucceededOnly() {
        String tradeNo = "T_SUCC";
        long pid = insertTx(tradeNo, 10_000L);
        refundMapper.insert(refund(tradeNo, pid, 300L, "2"));  // 僅 SUCCEEDED

        assertFalse(refundMapper.hasProcessingRefund(tradeNo),
                "SUCCEEDED 不得算未終結申請，否則同交易後續部分退款被永久擋死");
    }

    @Test
    @DisplayName("L2: hasProcessingRefund 對 FAILED(3) 為 false")
    void hasProcessingRefund_falseForFailed() {
        String tradeNo = "T_FAIL";
        long pid = insertTx(tradeNo, 10_000L);
        refundMapper.insert(refund(tradeNo, pid, 300L, "3"));  // FAILED

        assertFalse(refundMapper.hasProcessingRefund(tradeNo));
    }

    // ==================== L3: DB trigger 兜底 ====================

    @Test
    @DisplayName("L3: 退款總額（在途+成功）超過交易金額的 INSERT 被 trigger 拒絕")
    void trigger_rejectsInsertExceedingTransactionAmount() {
        String tradeNo = "T_GUARD";
        long pid = insertTx(tradeNo, 100L);

        // 首筆 REQUESTED 100 恰好用滿額度 → 允許
        refundMapper.insert(refund(tradeNo, pid, 100L, "0"));

        // 再退 1 元 → 100(在途) + 1 > 100 → 應被 trigger 拒絕
        RefundTransactionEntity over = refund(tradeNo, pid, 1L, "0");
        assertThrows(Exception.class, () -> refundMapper.insert(over),
                "超額退款 INSERT 應被 L3 trigger 拒絕");
    }

    @Test
    @DisplayName("L3: 多筆部分退款累計未超額 → 允許；再超一元 → 拒絕")
    void trigger_allowsPartialUpToLimit_rejectsBeyond() {
        String tradeNo = "T_PARTIAL";
        long pid = insertTx(tradeNo, 100L);

        refundMapper.insert(refund(tradeNo, pid, 60L, "2"));  // 成功 60
        refundMapper.insert(refund(tradeNo, pid, 40L, "0"));  // 在途 40 → 累計 100，恰好

        RefundTransactionEntity over = refund(tradeNo, pid, 1L, "0");
        assertThrows(Exception.class, () -> refundMapper.insert(over),
                "累計已達交易金額，再退任何金額都應被拒絕");
    }

    @Test
    @DisplayName("L3: FAILED 退款不佔額度，不阻擋後續在途退款")
    void trigger_failedRefundDoesNotConsumeQuota() {
        String tradeNo = "T_FAILED_QUOTA";
        long pid = insertTx(tradeNo, 100L);

        refundMapper.insert(refund(tradeNo, pid, 100L, "3"));  // FAILED 100（不佔額度）
        // 仍可再建立一筆 100 的在途退款（FAILED 不計）
        refundMapper.insert(refund(tradeNo, pid, 100L, "0"));

        assertEquals(100L, refundMapper.sumRefundedAmount(tradeNo));
    }
}

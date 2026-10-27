package io.github.will413028.hawkr.payment.application.service;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.payment.adapter.grpc.client.OrderGrpcClient;
import io.github.will413028.hawkr.payment.application.dto.GrantPaymentResult;
import io.github.will413028.hawkr.payment.config.PayUniConfig;
import io.github.will413028.hawkr.payment.constant.PayUniConstants;
import io.github.will413028.hawkr.payment.domain.repository.PaymentTransactionRepository;
import io.github.will413028.hawkr.payment.domain.service.PaymentDomainService;
import io.github.will413028.hawkr.payment.infrastructure.mq.PaymentOutboxWriter;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.PaymentTransactionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentLogMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.PaymentTransactionMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.repository.PaymentTransactionRepositoryImpl;
import io.github.will413028.hawkr.payuni.http.PayUniApiException;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.payment.PaymentClient;
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
import org.springframework.boot.autoconfigure.transaction.TransactionAutoConfiguration;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.context.annotation.Bean;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.Map;
import java.util.concurrent.atomic.AtomicLong;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.when;

/**
 * §5.6 去重硬化 A（P0-1）「哨兵值/失敗終態必須隨交易 commit 存活、不被 rollback」的<b>真實 Spring
 * transaction</b> 驗證（純 Mockito 測不出 commit / rollback，必須真實 PostgreSQL + 交易管理員）。
 *
 * <p>驗證：grantPayment（@Transactional）於撥款模稜兩可/失敗時採 write-then-return（不 throw），
 * grant_status（PENDING_CONFIRM / PayUni 錯誤碼）<b>真的 commit 存活</b>（讀回仍在，未被回滾），
 * 且失敗路徑 <b>granted_at 保持 null</b>（里程碑僅 SUCCESS 寫入）。
 * 舊實作（寫入後 throw / 例外穿透）此處讀回會是 null → 本測試即為修復的回歸鎖。
 */
@SpringBootTest(classes = GrantSentinelSurvivalIntegrationTest.MinimalConfig.class)
@Testcontainers
@Tag("integration")
class GrantSentinelSurvivalIntegrationTest {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("payment_grant_sentinel_test")
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
            MyBatisPlusAutoConfiguration.class,
            TransactionAutoConfiguration.class   // 讓 @Transactional 真正生效（commit/rollback）
    })
    @MapperScan("io.github.will413028.hawkr.payment.infrastructure.persistence.mapper")
    static class MinimalConfig {

        @Bean
        PaymentTransactionRepository transactionRepository(PaymentTransactionMapper mapper) {
            return new PaymentTransactionRepositoryImpl(mapper);
        }

        @Bean
        PaymentApplicationService paymentApplicationService(
                PaymentTransactionRepository repo,
                io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository refundRepo,
                PaymentOutboxWriter pub, PaymentDomainService dom,
                PaymentClient client, PayUniConfig cfg, OrderGrpcClient order, PaymentLogMapper logMapper,
                AtmReactivationService atm, PaymentCloseService close) {
            return new PaymentApplicationService(repo, refundRepo, pub, dom, client, cfg, order, logMapper, atm, close);
        }
    }

    @MockBean private io.github.will413028.hawkr.payment.domain.repository.RefundTransactionRepository refundRepository;
    @MockBean private PaymentOutboxWriter outboxWriter;
    @MockBean private PaymentDomainService domainService;
    @MockBean private PaymentClient paymentClient;
    @MockBean private PayUniConfig payUniConfig;
    @MockBean private OrderGrpcClient orderGrpcClient;
    @MockBean private AtmReactivationService atmReactivationService;
    @MockBean private PaymentCloseService paymentCloseService;
    @MockBean private PaymentLogMapper paymentLogMapper;   // 撥款 audit log no-op，聚焦 grant_status 落地

    @Autowired private PaymentApplicationService service;
    @Autowired private PaymentTransactionMapper txMapper;

    private final AtomicLong idSeq = new AtomicLong(2_000_000L);

    @BeforeEach
    void setUp() {
        txMapper.delete(null);
        lenient().when(payUniConfig.getAgentId()).thenReturn("HAWK");
    }

    @AfterEach
    void cleanAfter() {
        txMapper.delete(null);
    }

    private LocalDateTime nowUtc() {
        return LocalDateTime.now(ZoneOffset.UTC);
    }

    /** 插入一筆首撥 PAID 交易（grant_status=null），回傳 row id。 */
    private long insertPaidTx(String orderId) {
        long id = idSeq.getAndIncrement();
        PaymentTransactionEntity e = new PaymentTransactionEntity();
        e.setId(id);
        e.setTradeNo(orderId + "001");
        e.setOrderId(orderId);
        e.setAttemptSeq(1);
        e.setMerId("HAWK9");
        e.setBuyerId("2001");
        e.setPaymentType("2001");    // CREDIT_CARD
        e.setAmount(500L);
        e.setProductName("test");
        e.setStatus("1");            // PAID
        e.setProviderTradeNo("UNI_TEST");
        e.setIsDuplicate(false);
        e.setCreatedAt(nowUtc());
        e.setUpdatedAt(nowUtc());
        e.setVersion(0);
        txMapper.insert(e);
        return id;
    }

    @Test
    @DisplayName("(a) 撥款 timeout（PayUniApiException 模稜兩可）→ grant_status=PENDING_CONFIRM 真的 commit 存活，granted_at 保持 null")
    void ambiguousTimeout_pendingConfirmCommittedAndSurvives() {
        long id = insertPaidTx("OSENT1");
        when(paymentClient.grant(any(), any(), any(), anyLong()))
                .thenThrow(new PayUniApiException("HTTP_ERROR", "connect timed out", 504));

        GrantPaymentResult result = service.grantPayment("OSENT1");
        assertEquals(PayUniConstants.PENDING_CONFIRM, result.grantStatus());

        // 讀回已 commit 的資料庫狀態（服務的 @Transactional 已結束並 commit）
        PaymentTransactionEntity reloaded = txMapper.selectById(id);
        assertEquals(PayUniConstants.PENDING_CONFIRM, reloaded.getGrantStatus(),
                "PENDING_CONFIRM 必須隨交易 commit 存活（未被 rollback）");
        assertNull(reloaded.getGrantedAt(), "失敗路徑不寫里程碑 granted_at");
    }

    @Test
    @DisplayName("(a') 撥款明確拒絕碼（retry 後仍失敗）→ grant_status=錯誤碼 commit 存活，granted_at 保持 null")
    void rejectCode_errorStatusCommittedAndSurvives() {
        long id = insertPaidTx("OSENT2");
        when(paymentClient.grant(any(), any(), any(), anyLong()))
                .thenReturn(new PayUniApiResponse("GRANT99999", "餘額不足", Map.of()));

        GrantPaymentResult result = service.grantPayment("OSENT2");
        assertEquals("GRANT99999", result.grantStatus());

        PaymentTransactionEntity reloaded = txMapper.selectById(id);
        assertEquals("GRANT99999", reloaded.getGrantStatus(),
                "PayUni 錯誤碼必須隨交易 commit 存活（未被 rollback）");
        assertNull(reloaded.getGrantedAt(), "失敗路徑不寫里程碑 granted_at");
    }

    @Test
    @DisplayName("(正控) 撥款成功 → grant_status=SUCCESS commit 存活，granted_at 已寫入")
    void success_committedWithGrantedAt() {
        long id = insertPaidTx("OSENT3");
        when(paymentClient.grant(any(), any(), any(), anyLong()))
                .thenReturn(new PayUniApiResponse("SUCCESS", "撥款成功", Map.of()));

        GrantPaymentResult result = service.grantPayment("OSENT3");
        assertEquals(PayUniConstants.RESPONSE_SUCCESS, result.grantStatus());

        PaymentTransactionEntity reloaded = txMapper.selectById(id);
        assertEquals(PayUniConstants.RESPONSE_SUCCESS, reloaded.getGrantStatus());
        org.junit.jupiter.api.Assertions.assertNotNull(reloaded.getGrantedAt(), "SUCCESS 須寫里程碑 granted_at");
    }
}

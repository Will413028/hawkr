package io.github.will413028.hawkr.webhook.service;

import com.baomidou.mybatisplus.autoconfigure.MybatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.config.MyBatisPlusAutoConfiguration;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.OutboxEventStatus;
import io.github.will413028.hawkr.common.utils.code.PayUniEventType;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.common.utils.code.WebhookVerifyStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.OutboxEvent;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.OutboxEventMapper;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.AfterEach;
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
import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.transaction.annotation.EnableTransactionManagement;
import org.springframework.transaction.annotation.Transactional;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * webhook_inbox dedup + FailedInboxRecorder 整合測試。
 *
 * 對應設計書 10 §2.4 dedup 行為 + §2.5.2 saveFailedInbox 必須走獨立事務。
 *
 * 涵蓋：
 * 1. DB 層 UNIQUE 約束驗證（PM Case 2/4：相同 eventKey 第二次必拋 DuplicateKeyException）
 * 2. FailedInboxRecorder 基本功能（驗簽失敗時 audit 寫入）
 * 3. §2.5.2 修正驗證：FailedInboxRecorder REQUIRES_NEW 真的隔離外層事務
 *    （外層 rollback 時 inbox 紀錄仍 commit）
 *
 * 採用最小 Spring config（不載 RabbitMQ / gRPC / Web / TSID / 業務 service）。
 */
@SpringBootTest(classes = WebhookInboxDedupIntegrationTest.MinimalConfig.class)
@Testcontainers
@Tag("integration")
@DisplayName("webhook_inbox dedup + FailedInboxRecorder 整合測試")
class WebhookInboxDedupIntegrationTest {

    @Container
    static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:17-alpine")
            .withDatabaseName("webhook_dedup_test")
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
        // hawkr_webhook DB），不覆寫會讓 Flyway 對本機真實 DB no-op（或密碼不符直接
        // BeanCreationException）、testcontainers 內的測試 DB 卻從未跑過 migration。
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
    @MapperScan("io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper")
    @EnableTransactionManagement
    static class MinimalConfig {

        @Bean
        ObjectMapper objectMapper() {
            return new ObjectMapper();
        }

        @Bean
        FailedInboxRecorder failedInboxRecorder(WebhookInboxMapper inboxMapper, ObjectMapper objectMapper) {
            return new FailedInboxRecorder(inboxMapper, objectMapper);
        }

        /**
         * 測試專用 outer service：模擬「外層 @Transactional 在呼叫 FailedInboxRecorder
         * 後拋例外」的情境，驗證 §2.5.2 REQUIRES_NEW 真的隔離。
         */
        @Bean
        OuterTransactionalCaller outerTransactionalCaller(FailedInboxRecorder failedInboxRecorder) {
            return new OuterTransactionalCaller(failedInboxRecorder);
        }

        @Bean
        WebhookInboxTxOps webhookInboxTxOps(WebhookInboxMapper inboxMapper) {
            return new WebhookInboxTxOps(inboxMapper);
        }

        @Bean
        WebhookOutboxTxOps webhookOutboxTxOps(OutboxEventMapper outboxMapper, WebhookInboxMapper inboxMapper) {
            return new WebhookOutboxTxOps(outboxMapper, inboxMapper);
        }
    }

    @Service
    static class OuterTransactionalCaller {
        private final FailedInboxRecorder failedInboxRecorder;

        OuterTransactionalCaller(FailedInboxRecorder failedInboxRecorder) {
            this.failedInboxRecorder = failedInboxRecorder;
        }

        @Transactional
        public void callFailedInboxThenThrow(String encryptInfo, String hashInfo, String error) {
            failedInboxRecorder.record(encryptInfo, hashInfo, error);
            throw new RuntimeException("simulated outer failure");
        }
    }

    @Autowired
    private WebhookInboxMapper inboxMapper;

    @Autowired
    private OutboxEventMapper outboxMapper;

    @Autowired
    private FailedInboxRecorder failedInboxRecorder;

    @Autowired
    private OuterTransactionalCaller outerCaller;

    @Autowired
    private WebhookInboxTxOps inboxTxOps;

    @Autowired
    private WebhookOutboxTxOps outboxTxOps;

    @AfterEach
    void cleanup() {
        outboxMapper.delete(null);
        inboxMapper.delete(null);
    }

    @Test
    @DisplayName("DB UNIQUE 約束：相同 eventKey 第二次 INSERT 必拋 DuplicateKeyException（PM Case 2/4 驗證）")
    void duplicateEventKey_throwsDuplicateKeyException() {
        WebhookInbox first = buildInbox("HAWK9:O123456789001:1776493168257421093:1");
        inboxMapper.insert(first);

        WebhookInbox second = buildInbox("HAWK9:O123456789001:1776493168257421093:1");
        assertThrows(DuplicateKeyException.class,
                () -> inboxMapper.insert(second),
                "相同 (provider, webhook_type, event_key) 第二次 INSERT 必須拋 DuplicateKeyException — "
              + "這是 dedup 機制的最外層守門");
    }

    @Test
    @DisplayName("不同 eventKey 可正常 INSERT 多筆（PM Case 1/3 驗證）")
    void differentEventKey_allInserted() {
        inboxMapper.insert(buildInbox("HAWK9:O123456789001:1776493168257421093:0"));
        inboxMapper.insert(buildInbox("HAWK9:O123456789001:1776493168257421093:1"));

        Long count = inboxMapper.selectCount(null);
        assertEquals(2L, count, "不同 eventKey 兩筆都應寫入");
    }

    @Test
    @DisplayName("FailedInboxRecorder.record() 基本功能：驗簽失敗時寫入 inbox audit")
    void failedInboxRecorder_writesInboxRow() {
        failedInboxRecorder.record("encrypt-data", "hash-data", "驗簽失敗測試");

        Long count = inboxMapper.selectCount(null);
        assertEquals(1L, count);

        WebhookInbox row = inboxMapper.selectList(null).get(0);
        assertEquals(WebhookInboxStatus.FAILED.value(), row.getStatus());
        assertEquals(WebhookVerifyStatus.FAILED.value(), row.getVerifyStatus());
        assertEquals(PayUniEventType.UNKNOWN.value(), row.getWebhookType());
        assertTrue(row.getEventKey().startsWith("FAILED:"),
                "失敗 inbox 的 eventKey 應以 FAILED: 開頭");
        assertEquals("驗簽失敗測試", row.getLastError());
        assertNotNull(row.getRawEnvelope());
    }

    @Test
    @DisplayName("§2.5.2 修正驗證：外層 @Transactional rollback 時 FailedInboxRecorder 紀錄仍 commit（REQUIRES_NEW）")
    void failedInbox_persistsEvenWhenOuterTransactionRollsBack() {
        // 模擬：外層 @Transactional 呼叫 FailedInboxRecorder.record() 後拋例外
        // 預期：外層 rollback，但 FailedInboxRecorder 因 REQUIRES_NEW 已獨立 commit
        assertThrows(RuntimeException.class,
                () -> outerCaller.callFailedInboxThenThrow("enc-x", "hash-x", "外層失敗測試"));

        Long count = inboxMapper.selectCount(null);
        assertEquals(1L, count,
                "§2.5.2：FailedInboxRecorder 必須走 REQUIRES_NEW 獨立事務 — "
              + "即使外層 @Transactional rollback，audit 紀錄仍應持久化。"
              + "若此 assertion 失敗，代表 FailedInboxRecorder 沒有正確隔離外層事務 "
              + "(可能是漏了 @Transactional(propagation = REQUIRES_NEW)，"
              + "或被 self-invoke 而未經 Spring AOP proxy)。");

        WebhookInbox row = inboxMapper.selectList(null).get(0);
        assertEquals("外層失敗測試", row.getLastError());
    }

    @Test
    @DisplayName("§4.5.3 OutboxEventMapper.insertIfAbsent：第一次 INSERT 回 1，第二次相同 (aggregate_id, exchange, topic) 因 UNIQUE 命中回 0 不報錯")
    void insertIfAbsent_replaySafeViaOnConflict() {
        String eventKey = "HAWK9:OREPLAY001:UNI001:1";
        String exchange = "payuni.events";
        String topic = "payuni.payment.uop";
        var now = DateTimeUtils.nowUtc();

        // 第一次：應寫入
        int first = outboxMapper.insertIfAbsent(
                WebhookProvider.PAYUNI.value(), eventKey, exchange, topic, eventKey,
                "{\"v\":\"1\"}", OutboxEventStatus.PENDING.value(), 0, now);
        assertEquals(1, first, "第一次應 INSERT 成功（return 1）");

        // 第二次：UNIQUE 命中應跳過（不拋例外、不污染 transaction）
        int second = outboxMapper.insertIfAbsent(
                WebhookProvider.PAYUNI.value(), eventKey, exchange, topic, eventKey,
                "{\"v\":\"2\"}", OutboxEventStatus.PENDING.value(), 0, now);
        assertEquals(0, second, "第二次應 UNIQUE 命中跳過（return 0）— 這是 §4.5.3 replay-safe 的核心保證");

        // 不同 exchange 仍允許（雙通道前提）
        int agentNotify = outboxMapper.insertIfAbsent(
                WebhookProvider.PAYUNI.value(), eventKey, "seller.events", "seller.review.result", eventKey,
                "{\"v\":\"1\"}", OutboxEventStatus.PENDING.value(), 0, now);
        assertEquals(1, agentNotify,
                "同 aggregate_id 但不同 (exchange, topic) 應允許 — uq_outbox_aggregate_route 粒度保證");

        Long count = outboxMapper.selectCount(null);
        assertEquals(2L, count, "結果應有 2 筆（同 eventKey 兩個路由），第二次重放被跳過");
    }

    @Test
    @DisplayName("§4.5.3 WebhookInboxTxOps.markIgnored：UNKNOWN 終態必須變 IGNORED + processedAt")
    void markIgnored_setsIgnoredStatusAndProcessedAt() {
        WebhookInbox inbox = buildInbox("HAWK9:UNKNOWN:" + System.currentTimeMillis());
        inboxMapper.insert(inbox);

        inboxTxOps.markIgnored(inbox.getId());

        WebhookInbox after = inboxMapper.selectById(inbox.getId());
        assertEquals(WebhookInboxStatus.IGNORED.value(), after.getStatus(),
                "markIgnored 必須把 status 設為 IGNORED（terminal 狀態，不參與 replay）");
        assertNotNull(after.getProcessedAt(), "markIgnored 必須設 processedAt");
    }

    @Test
    @DisplayName("§4.5.3 WebhookInboxTxOps.markFailed：T2 失敗時必須變 FAILED + lastError")
    void markFailed_setsFailedStatusAndLastError() {
        WebhookInbox inbox = buildInbox("HAWK9:OFAIL001:UNI001:1");
        inboxMapper.insert(inbox);

        inboxTxOps.markFailed(inbox.getId(), "simulated T2 failure");

        WebhookInbox after = inboxMapper.selectById(inbox.getId());
        assertEquals(WebhookInboxStatus.FAILED.value(), after.getStatus(),
                "markFailed 必須把 status 設為 FAILED（PayUni 重送 → T1 dedup hit FAILED → REPLAY 路徑接手）");
        assertEquals("simulated T2 failure", after.getLastError());
    }

    @Test
    @DisplayName("§4.5.3 WebhookInboxTxOps.markFailed：超長 error 訊息應截斷至 4000 字元防爆 TEXT 欄位")
    void markFailed_truncatesLongError() {
        WebhookInbox inbox = buildInbox("HAWK9:OFAIL002:UNI002:1");
        inboxMapper.insert(inbox);

        String longError = "x".repeat(5000);
        inboxTxOps.markFailed(inbox.getId(), longError);

        WebhookInbox after = inboxMapper.selectById(inbox.getId());
        assertEquals(4000, after.getLastError().length(), "lastError 應被截斷至 4000 字元");
    }

    @Test
    @DisplayName("§4.5.3 WebhookInboxMapper.findByEventKey：依三元組（provider, webhookType, eventKey）查 existing inbox")
    void findByEventKey_returnsExistingRow() {
        WebhookInbox inbox = buildInbox("HAWK9:OFIND001:UNI001:1");
        inboxMapper.insert(inbox);

        WebhookInbox found = inboxMapper.findByEventKey(
                WebhookProvider.PAYUNI.value(),
                PayUniEventType.PAYMENT_UOP.value(),
                "HAWK9:OFIND001:UNI001:1");
        assertNotNull(found, "findByEventKey 應能查到 existing row");
        assertEquals(inbox.getId(), found.getId());

        WebhookInbox notFound = inboxMapper.findByEventKey(
                WebhookProvider.PAYUNI.value(),
                PayUniEventType.PAYMENT_UOP.value(),
                "HAWK9:NOTEXIST:UNI001:1");
        assertEquals(null, notFound, "查不到應回 null（用於 dedup.lookupMissing 防呆判斷）");
    }

    @Test
    @DisplayName("§4.5.3 WebhookOutboxTxOps.enqueueAndMark：T2 完成後 inbox status=ENQUEUED + outbox 1 筆 PENDING")
    void enqueueAndMark_singleRoute_setsEnqueuedAndCreatesOutbox() {
        WebhookInbox inbox = buildInbox("HAWK9:OENQ001:UNI001:1");
        inboxMapper.insert(inbox);

        outboxTxOps.enqueueAndMark(inbox.getId(),
                java.util.List.of(new WebhookOutboxTxOps.OutboxRoute(
                        "HAWK9:OENQ001:UNI001:1",
                        "payuni.events",
                        "payuni.payment.uop",
                        "{\"e\":\"1\"}")));

        WebhookInbox after = inboxMapper.selectById(inbox.getId());
        assertEquals(WebhookInboxStatus.ENQUEUED.value(), after.getStatus());
        assertNotNull(after.getProcessedAt());

        Long outboxCount = outboxMapper.selectCount(null);
        assertEquals(1L, outboxCount);
    }

    @Test
    @DisplayName("§4.5.3 WebhookOutboxTxOps.enqueueAndMark：AGENT_NOTIFY 單通道僅發 seller.review.result（#1757 收斂）")
    void enqueueAndMark_agentNotifySingleRoute() {
        WebhookInbox inbox = buildInbox("HAWK9:1:20260426143000");
        inbox.setWebhookType(PayUniEventType.AGENT_NOTIFY.value());
        inboxMapper.insert(inbox);

        // #1757：AGENT_NOTIFY 已移除無 consumer 的 payuni.events/payuni.agent.notify 冗餘路由，
        // 僅保留 seller.events/seller.review.result 單通道。
        outboxTxOps.enqueueAndMark(inbox.getId(),
                java.util.List.of(
                        new WebhookOutboxTxOps.OutboxRoute(
                                "HAWK9:1:20260426143000",
                                "seller.events",
                                "seller.review.result",
                                "{\"e\":\"1\"}")));

        Long outboxCount = outboxMapper.selectCount(null);
        assertEquals(1L, outboxCount, "AGENT_NOTIFY 應僅建立 1 筆 outbox（seller.review.result 單通道）");
    }

    @Test
    @DisplayName("§4.5.3 WebhookOutboxTxOps.enqueueAndMark：replay 重跑時 outbox UNIQUE 命中跳過，inbox 仍變 ENQUEUED")
    void enqueueAndMark_replaySkipsExistingOutbox() {
        WebhookInbox inbox = buildInbox("HAWK9:OREP001:UNI001:1");
        inbox.setStatus(WebhookInboxStatus.FAILED.value());
        inboxMapper.insert(inbox);

        WebhookOutboxTxOps.OutboxRoute route = new WebhookOutboxTxOps.OutboxRoute(
                "HAWK9:OREP001:UNI001:1",
                "payuni.events",
                "payuni.payment.uop",
                "{\"e\":\"1\"}");

        // 第一次（模擬前次 T2 commit 後在某處中斷）
        outboxTxOps.enqueueAndMark(inbox.getId(), java.util.List.of(route));

        // 第二次（replay 重跑）— 不應拋例外、outbox 不應重複
        outboxTxOps.enqueueAndMark(inbox.getId(), java.util.List.of(route));

        Long outboxCount = outboxMapper.selectCount(null);
        assertEquals(1L, outboxCount, "replay 重跑時 ON CONFLICT 應跳過，outbox 仍 1 筆");

        WebhookInbox after = inboxMapper.selectById(inbox.getId());
        assertEquals(WebhookInboxStatus.ENQUEUED.value(), after.getStatus(),
                "即使 outbox 命中 UNIQUE 跳過，inbox 仍應正確收斂為 ENQUEUED");
    }

    private WebhookInbox buildInbox(String eventKey) {
        WebhookInbox inbox = new WebhookInbox();
        inbox.setProvider(WebhookProvider.PAYUNI.value());
        inbox.setMerId("HAWK9");
        inbox.setWebhookType(PayUniEventType.PAYMENT_UOP.value());
        inbox.setEventKey(eventKey);
        inbox.setStatus(WebhookInboxStatus.RECEIVED.value());
        inbox.setVerifyStatus(WebhookVerifyStatus.PASSED.value());
        inbox.setRawEnvelope("{}");
        inbox.setDecryptedPayload(toJson(Map.of("test", "payload")));
        inbox.setReceivedAt(DateTimeUtils.nowUtc());
        return inbox;
    }

    private String toJson(Object obj) {
        try {
            return new ObjectMapper().writeValueAsString(obj);
        } catch (Exception e) {
            return "{}";
        }
    }
}

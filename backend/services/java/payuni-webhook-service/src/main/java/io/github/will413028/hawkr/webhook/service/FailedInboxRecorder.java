package io.github.will413028.hawkr.webhook.service;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.PayUniEventType;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.common.utils.code.WebhookProvider;
import io.github.will413028.hawkr.common.utils.code.WebhookVerifyStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

import java.util.Map;

/**
 * 驗簽/解析失敗時的 audit inbox 寫入器。
 * <p>
 * 設計書 §2.5.2：必須走獨立事務（REQUIRES_NEW）。
 * 因 Spring AOP self-invoke 限制，從 WebhookReceiveService 拆出獨立 bean，
 * 確保 @Transactional 真的生效。
 * <p>
 * 若與外層共用同一 @Transactional，內部吞例外的 INSERT 失敗會讓
 * PostgreSQL connection 進入 aborted state，外層 commit 變 ROLLBACK，
 * audit 紀錄寫不進去。
 */
@Service
public class FailedInboxRecorder {

    private static final ObsLogger log = ObsLogger.forClass(FailedInboxRecorder.class);

    private final WebhookInboxMapper inboxMapper;
    private final ObjectMapper objectMapper;

    public FailedInboxRecorder(WebhookInboxMapper inboxMapper, ObjectMapper objectMapper) {
        this.inboxMapper = inboxMapper;
        this.objectMapper = objectMapper;
    }

    @Transactional(propagation = Propagation.REQUIRES_NEW)
    public void record(String encryptInfo, String hashInfo, String error) {
        try {
            WebhookInbox inbox = new WebhookInbox();
            inbox.setProvider(WebhookProvider.PAYUNI.value());
            inbox.setMerId("UNKNOWN");
            inbox.setWebhookType(PayUniEventType.UNKNOWN.value());
            inbox.setEventKey("FAILED:" + System.currentTimeMillis());
            inbox.setStatus(WebhookInboxStatus.FAILED.value());
            inbox.setVerifyStatus(WebhookVerifyStatus.FAILED.value());
            inbox.setRawEnvelope(toJson(Map.of("EncryptInfo", encryptInfo, "HashInfo", hashInfo)));
            inbox.setReceivedAt(DateTimeUtils.nowUtc());
            inbox.setLastError(error);
            inboxMapper.insert(inbox);
        } catch (Exception e) {
            log.error("FailedInboxRecorder.record.error", "記錄失敗 inbox 時發生錯誤", e);
        }
    }

    private String toJson(Object obj) {
        try {
            return objectMapper.writeValueAsString(obj);
        } catch (Exception e) {
            return "{}";
        }
    }
}

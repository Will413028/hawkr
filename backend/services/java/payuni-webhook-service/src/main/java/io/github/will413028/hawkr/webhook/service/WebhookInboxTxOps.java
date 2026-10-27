package io.github.will413028.hawkr.webhook.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.WebhookInboxStatus;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.WebhookInboxMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;

/**
 * Inbox 的事務操作（拆獨立 bean，避免 §4.5.3 self-invocation 失效）。
 * <p>
 * 設計書 §4.5.3：T1 為 inbox INSERT 的真實 dedup commitment 邊界，
 * 必須走 REQUIRES_NEW 獨立 commit。
 */
@Service
public class WebhookInboxTxOps {

    private static final int MAX_LAST_ERROR_LEN = 4000;

    private final WebhookInboxMapper inboxMapper;

    public WebhookInboxTxOps(WebhookInboxMapper inboxMapper) {
        this.inboxMapper = inboxMapper;
    }

    /**
     * T1：INSERT inbox。不 catch 任何例外 — DuplicateKeyException 與其他 DB 例外都向上拋。
     * §2.5.1：catch 內禁止再執行 SQL（PostgreSQL aborted state），DupKey 後的 SELECT existing
     * 由呼叫端在 T1 之外執行（直接呼叫 {@link WebhookInboxMapper#findByEventKey}）。
     */
    @Transactional(propagation = Propagation.REQUIRES_NEW)
    public void insertInbox(WebhookInbox inbox) {
        inboxMapper.insert(inbox);
    }

    /**
     * UNKNOWN webhookType 的終態：標 IGNORED，避免 inbox 卡 RECEIVED 被 replay 邏輯誤撈。
     */
    @Transactional(propagation = Propagation.REQUIRES_NEW)
    public void markIgnored(Long inboxId) {
        WebhookInbox patch = new WebhookInbox();
        patch.setId(inboxId);
        patch.setStatus(WebhookInboxStatus.IGNORED.value());
        patch.setProcessedAt(DateTimeUtils.nowUtc());
        inboxMapper.updateById(patch);
    }

    /**
     * T2 失敗時標記 inbox 為 FAILED + last_error，REQUIRES_NEW 確保不被 T2 rollback 牽連。
     * 自身失敗只 log，不再嘗試（best-effort）；最終由 controller 回 5xx + PayUni 重送觸發 replay 補救。
     */
    @Transactional(propagation = Propagation.REQUIRES_NEW)
    public void markFailed(Long inboxId, String error) {
        WebhookInbox patch = new WebhookInbox();
        patch.setId(inboxId);
        patch.setStatus(WebhookInboxStatus.FAILED.value());
        patch.setLastError(truncate(error));
        inboxMapper.updateById(patch);
    }

    private String truncate(String s) {
        if (s == null) return null;
        return s.length() <= MAX_LAST_ERROR_LEN ? s : s.substring(0, MAX_LAST_ERROR_LEN);
    }
}

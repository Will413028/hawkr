package io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.baomidou.mybatisplus.core.toolkit.Wrappers;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.WebhookInbox;
import org.apache.ibatis.annotations.Mapper;

@Mapper
public interface WebhookInboxMapper extends BaseMapper<WebhookInbox> {

    /**
     * 依 (provider, webhookType, eventKey) 查 inbox row。
     * 用於 T1 INSERT 命中 UNIQUE 後讀 existing 判斷狀態（dedup hit vs replay）。
     * 必須在 T1 transaction 之外呼叫（§2.5.1：catch 內禁止再 SQL）。
     */
    default WebhookInbox findByEventKey(String provider, String webhookType, String eventKey) {
        return selectOne(Wrappers.<WebhookInbox>lambdaQuery()
                .eq(WebhookInbox::getProvider, provider)
                .eq(WebhookInbox::getWebhookType, webhookType)
                .eq(WebhookInbox::getEventKey, eventKey));
    }
}

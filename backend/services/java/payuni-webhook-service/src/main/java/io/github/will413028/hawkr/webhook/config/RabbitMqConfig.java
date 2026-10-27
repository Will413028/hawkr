package io.github.will413028.hawkr.webhook.config;

import io.github.will413028.hawkr.common.mq.MqConstants;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * RabbitMQ 配置
 *
 * webhook-service 負責：
 * 1. 接收外部 PayUni Webhook（HTTP）
 * 2. 驗簽、解密、去重
 * 3. 透過 Outbox 模式發布事件到 payuni.events Exchange
 */
@Configuration
public class RabbitMqConfig {

    /**
     * PayUni Webhook 事件 Exchange（Topic 模式）
     * 下游服務綁定此 Exchange 消費對應 routing key
     */
    @Bean
    public TopicExchange payuniWebhookExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_PAYUNI_WEBHOOK, true, false);
    }

    /**
     * Seller 事件 Exchange（Topic 模式）
     * AGENT_NOTIFY 中的 MerStatus 事件發布至此，保持 seller-service 消費端不變
     */
    @Bean
    public TopicExchange sellerExchange() {
        return new TopicExchange(MqConstants.EXCHANGE_SELLER, true, false);
    }
}

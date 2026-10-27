package io.github.will413028.hawkr.webhook.infrastructure.mq;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.entity.OutboxEvent;
import io.github.will413028.hawkr.webhook.infrastructure.persistence.mapper.OutboxEventMapper;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageBuilder;
import org.springframework.amqp.core.MessagePropertiesBuilder;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.stereotype.Component;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;

import java.nio.charset.StandardCharsets;

/**
 * Outbox 事件發布器
 *
 * 將 outbox_events 記錄發布到 RabbitMQ
 */
@Component
public class OutboxPublisher {

    private static final ObsLogger log = ObsLogger.forClass(OutboxPublisher.class);

    private final RabbitTemplate rabbitTemplate;
    private final OutboxEventMapper outboxMapper;

    public OutboxPublisher(RabbitTemplate rabbitTemplate, OutboxEventMapper outboxMapper) {
        this.rabbitTemplate = rabbitTemplate;
        this.outboxMapper = outboxMapper;
    }

    /**
     * 發布單筆 Outbox 事件到 MQ
     *
     * @return true=發送成功，false=發送失敗
     */
    public boolean publish(OutboxEvent event) {
        try {
            Message message = MessageBuilder
                    .withBody(event.getPayload().getBytes(StandardCharsets.UTF_8))
                    .andProperties(MessagePropertiesBuilder.newInstance()
                            .setContentType("application/json")
                            .setMessageId(String.valueOf(event.getId()))
                            .setHeader("aggregateType", event.getAggregateType())
                            .setHeader("aggregateId", event.getAggregateId())
                            .setHeader("messageKey", event.getMessageKey())
                            .build())
                    .build();

            rabbitTemplate.send(
                    event.getExchange(),
                    event.getTopic(),
                    message);

            // 更新為 SENT（issue #2294 H3：具名守衛 UPDATE，避免 updateById 全欄覆寫掉
            // 另一個 pod 剛寫入的 claim 租約／狀態）
            outboxMapper.markSent(event.getId(), DateTimeUtils.nowUtc());

            log.info("OutboxPublisher.publish.success",
                    "事件已發布", "exchange", event.getExchange() + "/" + event.getTopic());
            return true;

        } catch (Exception e) {
            log.error("OutboxPublisher.publish.failed",
                    "事件發布失敗: " + event.getMessageKey(), e);
            markFailed(event, e.getMessage());
            return false;
        }
    }

    /**
     * 標記為失敗，設定下次重試時間（指數退避）。
     *
     * <p>issue #2294 H3：具名守衛 UPDATE（{@code status <> SENT}），取代 {@code updateById}
     * 全欄覆寫——後者會把「另一個 pod 已標 SENT」的結果回寫成 FAILED，造成重複投遞一輪。
     */
    private void markFailed(OutboxEvent event, String error) {
        int retryCount = event.getRetryCount() + 1;
        // 指數退避：5s, 15s, 45s, 135s, 405s
        long delaySec = (long) (5 * Math.pow(3, retryCount - 1));
        outboxMapper.markFailedGuarded(event.getId(), retryCount, error,
                DateTimeUtils.nowUtc().plusSeconds(delaySec));
    }
}

package io.github.will413028.hawkr.productimport.listener;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.CommitItemEvent;
import io.github.will413028.hawkr.productimport.service.CommitItemService;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;

@Component
public class CommitItemListener {

    private static final ObsLogger log = ObsLogger.forClass(CommitItemListener.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final CommitItemService commitItemService;

    public CommitItemListener(CommitItemService commitItemService) {
        this.commitItemService = commitItemService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_IMPORT_COMMIT,
                    containerFactory = "manualAckContainerFactory")
    public void onCommit(Message message, Channel channel,
                          @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        try {
            CommitItemEvent event = MAPPER.readValue(message.getBody(), CommitItemEvent.class);
            log.info("CommitItemListener.received", "itemId=" + event.importItemId());
            commitItemService.commitOne(event);
            channel.basicAck(deliveryTag, false);
            log.info("CommitItemListener.success", "itemId=" + event.importItemId());
        } catch (Exception e) {
            log.error("CommitItemListener.error", "Failed commit", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "CommitItemListener", e);
        }
    }
}

package io.github.will413028.hawkr.productimport.listener;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.HtmlFetchEvent;
import io.github.will413028.hawkr.productimport.service.HtmlFetchService;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;

@Component
public class HtmlFetchListener {

    private static final ObsLogger log = ObsLogger.forClass(HtmlFetchListener.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final HtmlFetchService htmlFetchService;

    public HtmlFetchListener(HtmlFetchService htmlFetchService) {
        this.htmlFetchService = htmlFetchService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_IMPORT_HTML_FETCH,
                    containerFactory = "manualAckContainerFactory")
    public void onHtmlFetch(Message message, Channel channel,
                             @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        try {
            HtmlFetchEvent event = MAPPER.readValue(message.getBody(), HtmlFetchEvent.class);
            log.info("HtmlFetchListener.received", "itemId=" + event.importItemId());
            htmlFetchService.fetchAndPersist(event);
            channel.basicAck(deliveryTag, false);
            log.info("HtmlFetchListener.success", "itemId=" + event.importItemId());
        } catch (Exception e) {
            log.error("HtmlFetchListener.error", "Failed html.fetch", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "HtmlFetchListener", e);
        }
    }
}

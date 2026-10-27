package io.github.will413028.hawkr.productimport.listener;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.productimport.publisher.ImportEvents.CsvParseEvent;
import io.github.will413028.hawkr.productimport.service.CsvParseService;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;

@Component
public class CsvParseListener {

    private static final ObsLogger log = ObsLogger.forClass(CsvParseListener.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final CsvParseService csvParseService;

    public CsvParseListener(CsvParseService csvParseService) {
        this.csvParseService = csvParseService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_IMPORT_CSV_PARSE,
                    containerFactory = "manualAckContainerFactory")
    public void onCsvParse(Message message, Channel channel,
                            @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag) throws IOException {
        try {
            CsvParseEvent event = MAPPER.readValue(message.getBody(), CsvParseEvent.class);
            log.info("CsvParseListener.received", "jobId=" + event.importJobId());
            csvParseService.parseAndPersist(event.importJobId(), event.storeId());
            channel.basicAck(deliveryTag, false);
            log.info("CsvParseListener.success", "jobId=" + event.importJobId());
        } catch (Exception e) {
            log.error("CsvParseListener.error", "Failed to process csv.parse event", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, "CsvParseListener", e);
        }
    }
}

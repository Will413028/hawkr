package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.product.service.StoreExposureService;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class SellerExposureChangedConsumerTest {

    @Mock StoreExposureService storeExposureService;
    @Mock Channel channel;
    @InjectMocks SellerExposureChangedConsumer consumer;

    private static final String CONTRACT_BODY = "{\"event_id\":\"<uuid>\",\"event_type\":\"SELLER_EXPOSURE_CHANGED\","
            + "\"version\":\"1.0\",\"timestamp\":\"2026-10-05T01:02:03.123456789Z\",\"source\":\"seller-service\","
            + "\"store_id\":123,\"exposable\":true,\"exposure_version\":7}";

    private Message msg(String body) {
        MessageProperties props = new MessageProperties();
        props.setDeliveryTag(9L);
        return new Message(body.getBytes(StandardCharsets.UTF_8), props);
    }

    private static String body(String storeId, String exposable, String version) {
        StringBuilder sb = new StringBuilder("{\"event_type\":\"SELLER_EXPOSURE_CHANGED\"");
        if (storeId != null) sb.append(",\"store_id\":").append(storeId);
        if (exposable != null) sb.append(",\"exposable\":").append(exposable);
        if (version != null) sb.append(",\"exposure_version\":").append(version);
        return sb.append("}").toString();
    }

    @Test
    @DisplayName("契約的 JSON 逐字：解析成功，寫入 (123, true, 7) 並 ack")
    void contractBodyIsApplied() throws IOException {
        consumer.onSellerExposureChanged(msg(CONTRACT_BODY), channel);
        verify(storeExposureService).apply(123L, true, 7L);
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("store_id 與 exposure_version 為 string 也可")
    void stringIdsAreAccepted() throws IOException {
        consumer.onSellerExposureChanged(msg(body("\"7412345678901234567\"", "false", "\"3\"")), channel);
        verify(storeExposureService).apply(7412345678901234567L, false, 3L);
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("舊版本或重送（apply 回 false）仍 ack")
    void staleEventIsAcked() throws IOException {
        when(storeExposureService.apply(123L, true, 7L)).thenReturn(false);
        consumer.onSellerExposureChanged(msg(CONTRACT_BODY), channel);
        verify(channel).basicAck(9L, false);
        verify(channel, never()).basicNack(anyLong(), anyBoolean(), anyBoolean());
    }

    @Test
    @DisplayName("缺 exposure_version → nack 進 DLQ，不寫入")
    void missingVersionGoesToDlq() throws IOException {
        consumer.onSellerExposureChanged(msg(body("123", "true", null)), channel);
        verify(channel).basicNack(9L, false, false);
        verify(storeExposureService, never()).apply(anyLong(), anyBoolean(), anyLong());
    }

    @Test
    @DisplayName("缺 exposable → nack 進 DLQ")
    void missingExposableGoesToDlq() throws IOException {
        consumer.onSellerExposureChanged(msg(body("123", null, "7")), channel);
        verify(channel).basicNack(9L, false, false);
        verify(storeExposureService, never()).apply(anyLong(), anyBoolean(), anyLong());
    }

    @Test
    @DisplayName("缺 store_id、exposable 非 boolean、版本非數字、store_id 非正數、event_type 不符、非 JSON → 全部 nack")
    void malformedBodiesGoToDlq() throws IOException {
        String[] bad = {
                body(null, "true", "7"),
                body("123", "\"true\"", "7"),
                body("123", "true", "\"abc\""),
                body("0", "true", "7"),
                body("123", "true", "0"),
                body("123", "true", "7.5"),
                "{\"event_type\":\"SELLER_DELETED\",\"store_id\":123,\"exposable\":true,\"exposure_version\":7}",
                "not json",
                "[]"
        };
        for (String b : bad) {
            consumer.onSellerExposureChanged(msg(b), channel);
        }
        verify(channel, org.mockito.Mockito.times(bad.length)).basicNack(9L, false, false);
        verify(channel, never()).basicAck(anyLong(), anyBoolean());
        verify(storeExposureService, never()).apply(anyLong(), anyBoolean(), anyLong());
    }

    @Test
    @DisplayName("暫時性失敗（DB 例外）：第一次失敗不進 DLQ，ack 原訊息並重新發布重試（x-retry-count=1）")
    void transientFailureIsRetriedNotDeadLettered() throws IOException {
        when(storeExposureService.apply(123L, true, 7L)).thenThrow(new IllegalStateException("db down"));

        consumer.onSellerExposureChanged(msg(CONTRACT_BODY), channel);

        verify(channel, never()).basicNack(anyLong(), anyBoolean(), anyBoolean());
        verify(channel, never()).basicReject(anyLong(), anyBoolean());
        verify(channel).basicAck(9L, false);
        org.mockito.ArgumentCaptor<com.rabbitmq.client.AMQP.BasicProperties> props =
                org.mockito.ArgumentCaptor.forClass(com.rabbitmq.client.AMQP.BasicProperties.class);
        verify(channel).basicPublish(org.mockito.ArgumentMatchers.eq(""), org.mockito.ArgumentMatchers.any(),
                props.capture(), org.mockito.ArgumentMatchers.any(byte[].class));
        org.junit.jupiter.api.Assertions.assertEquals(1, ((Number) props.getValue().getHeaders().get("x-retry-count")).intValue());
    }

    @Test
    @DisplayName("重試用盡（x-retry-count=3）：basicReject 進 DLQ")
    void exhaustedRetriesGoToDlq() throws IOException {
        when(storeExposureService.apply(123L, true, 7L)).thenThrow(new IllegalStateException("db down"));
        MessageProperties props = new MessageProperties();
        props.setDeliveryTag(9L);
        props.setHeader("x-retry-count", 3);

        consumer.onSellerExposureChanged(new Message(CONTRACT_BODY.getBytes(StandardCharsets.UTF_8), props), channel);

        verify(channel).basicReject(9L, false);
        verify(channel, never()).basicAck(anyLong(), anyBoolean());
    }
}

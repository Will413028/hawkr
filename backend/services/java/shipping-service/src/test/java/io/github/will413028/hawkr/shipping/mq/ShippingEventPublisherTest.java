package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;

/**
 * 單元測試：驗證 {@link ShippingEventPublisher#publishProcessingTimeout} 手搭的
 * JSON {@code Map.of(...)} 真的把買家欄位的 wire key 從 {@code buyerAccountId}
 * 改成 {@code buyerId}（issue #2174，2026-08-05 正名）。
 *
 * <p>這個方法用 {@code objectMapper.writeValueAsString(Map.of(...))} 手動組 JSON，
 * 不是走 record + {@code @JsonNaming}，所以任何只捕捉呼叫端傳入的 {@code Long}
 * 參數（例如 {@link io.github.will413028.hawkr.shipping.scheduler.ProcessingDeadlineSchedulerTest}
 * 對 mocked {@code eventPublisher} 的位置參數斷言）都測不到序列化後的實際 key 名稱
 * ——必須直接對這個 publisher 發出的位元組斷言，才能防止改名 regress 或被之後的
 * 改動悄悄改回 {@code buyerAccountId}。
 */
class ShippingEventPublisherTest {

    private ObsRabbitPublisher rabbitPublisher;
    private ShippingEventPublisher publisher;

    @BeforeEach
    void setUp() {
        rabbitPublisher = mock(ObsRabbitPublisher.class);
        publisher = new ShippingEventPublisher(rabbitPublisher, new ObjectMapper());
    }

    @Test
    void publishProcessingTimeout_emitsBuyerId_notBuyerAccountId() throws Exception {
        publisher.publishProcessingTimeout("O100", 11L, 555L);

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(rabbitPublisher).publish(
                eq(MqConstants.EXCHANGE_SHIPPING),
                eq(MqConstants.ROUTING_KEY_SHIPMENT_PROCESSING_TIMEOUT),
                bodyCaptor.capture(),
                eq(MqConstants.EVENT_TYPE_SHIPMENT_PROCESSING_TIMEOUT));

        JsonNode wire = new ObjectMapper().readTree(bodyCaptor.getValue());
        assertEquals("O100", wire.get("orderNo").asText());
        // sellerAccountId 保留原名：這裡裝的確實是已解析的登入 account_id，不動。
        assertEquals("11", wire.get("sellerAccountId").asText());

        assertTrue(wire.has("buyerId"), "shipment.processing.timeout 必須發 buyerId");
        assertEquals("555", wire.get("buyerId").asText());
        assertFalse(wire.has("buyerAccountId"), "shipment.processing.timeout 不得再發 buyerAccountId");
    }
}

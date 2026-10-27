package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.product.service.SellerProductOffShelfService;
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

import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * ② re-key：seller.deleted 的 owner 識別優先用 store_id（store 代理鍵，enqueue 時
 * snapshot；Phase 2 account_id 命名債 campaign Task 3 起，proto/wire key 從
 * sellerAccountId 正名為 store_id）。仍相容舊鍵 sellerAccountId 一個 rollout 窗口
 * （尚未部署完成的舊版 seller-service 送來的事件）；再退一步 fallback accountId
 * — pre-incarnation surrogate == account_id，安全。
 */
@ExtendWith(MockitoExtension.class)
class SellerDeletedConsumerTest {

    @Mock SellerProductOffShelfService offShelfService;
    @Mock Channel channel;
    @InjectMocks SellerDeletedConsumer consumer;

    private Message msg(String body) {
        MessageProperties props = new MessageProperties();
        props.setDeliveryTag(7L);
        return new Message(body.getBytes(), props);
    }

    @Test
    @DisplayName("新格式：帶 store_id → 以 surrogate 下架 + ack")
    void newFormatUsesStoreId() throws IOException {
        consumer.onSellerDeleted(msg(
                "{\"event_type\":\"SELLER_DELETED\",\"accountId\":2619961976,\"store_id\":7412345678901234567}"),
                channel);
        verify(offShelfService).offShelfAllByMerchant(7412345678901234567L);
        verify(channel).basicAck(7L, false);
    }

    @Test
    @DisplayName("rollout-window 相容格式：無 store_id 但帶舊鍵 sellerAccountId → 仍以 surrogate 下架 + ack")
    void rolloutWindowFallsBackToLegacySellerAccountId() throws IOException {
        consumer.onSellerDeleted(msg(
                "{\"event_type\":\"SELLER_DELETED\",\"accountId\":2619961976,\"sellerAccountId\":8523456789012345678}"),
                channel);
        verify(offShelfService).offShelfAllByMerchant(8523456789012345678L);
        verify(channel).basicAck(7L, false);
    }

    @Test
    @DisplayName("舊格式：store_id 與 sellerAccountId 皆無 → fallback accountId（in-flight 舊事件）")
    void oldFormatFallsBackToAccountId() throws IOException {
        consumer.onSellerDeleted(msg(
                "{\"event_type\":\"SELLER_DELETED\",\"accountId\":2619961976}"), channel);
        verify(offShelfService).offShelfAllByMerchant(2619961976L);
        verify(channel).basicAck(7L, false);
    }

    @Test
    @DisplayName("兩者皆缺 → WARN + ack，不下架")
    void missingBothIdsAcksWithoutOffShelf() throws IOException {
        consumer.onSellerDeleted(msg("{\"event_type\":\"SELLER_DELETED\"}"), channel);
        verify(offShelfService, never()).offShelfAllByMerchant(anyLong());
        verify(channel).basicAck(7L, false);
    }
}

package io.github.will413028.hawkr.coupon.mq;

import io.github.will413028.hawkr.coupon.service.CouponInstanceService;
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
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;

/**
 * W2/W3 設計書 §3.2 D6：coupon-service 的買家刪除清理 consumer 從
 * account.deleted 遷移到 buyer.deleted。
 *
 * <p>此測試檔為新建（2026-07-16 隨遷移新增）——遷移前的 AccountDeletedConsumer
 * 沒有對應測試檔，全 repo（含所有 .worktrees/）搜尋確認為 0 個既有測試可沿用，
 * 因此本檔案本身就是本次遷移唯一的測試基準，非「更新既有測試」。
 *
 * <p>比照 product-service {@code SellerDeletedConsumerTest} 的慣例：純 JUnit5 +
 * MockitoExtension，直接呼叫 consumer 方法（不經 Spring context / testcontainers），
 * 驗證對下游 service 的呼叫參數 + channel ack 行為。
 */
@ExtendWith(MockitoExtension.class)
class BuyerDeletedConsumerTest {

    @Mock CouponInstanceService instanceService;
    @Mock Channel channel;
    @InjectMocks BuyerDeletedConsumer consumer;

    private Message msg(String body) {
        return new Message(body.getBytes(), new MessageProperties());
    }

    @Test
    @DisplayName("buyerId 有效 → 以 buyerId（非 accountId）批量作廢 + ack")
    void validBuyerIdVoidsAndAcks() throws IOException {
        consumer.onBuyerDeleted(
                msg("{\"accountId\":2619961976,\"buyerId\":7412345678901234567}"),
                channel, 9L);

        verify(instanceService).voidAllUnusedByOwner(7412345678901234567L, "BUYER_DELETED");
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("buyerId 欄位缺失（僅有 accountId）→ WARN + ack，不作廢")
    void missingBuyerIdAcksWithoutVoiding() throws IOException {
        consumer.onBuyerDeleted(
                msg("{\"accountId\":2619961976}"),
                channel, 9L);

        verify(instanceService, never()).voidAllUnusedByOwner(anyLong(), anyString());
        verify(channel).basicAck(9L, false);
    }

    @Test
    @DisplayName("buyerId = 0 → WARN + ack，不作廢")
    void zeroBuyerIdAcksWithoutVoiding() throws IOException {
        consumer.onBuyerDeleted(
                msg("{\"accountId\":2619961976,\"buyerId\":0}"),
                channel, 9L);

        verify(instanceService, never()).voidAllUnusedByOwner(anyLong(), anyString());
        verify(channel).basicAck(9L, false);
    }
}

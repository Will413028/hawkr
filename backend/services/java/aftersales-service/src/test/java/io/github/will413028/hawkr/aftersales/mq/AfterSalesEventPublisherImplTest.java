package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class AfterSalesEventPublisherImplTest {

    @Mock private ObsRabbitPublisher obsRabbitPublisher;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;

    /**
     * 賣家工作區 realtime 的 sellerId 是 store_id；domain event 的 seller_account_id 仍是反解後的
     * account_id —— 兩條線的消費端不同，身分空間也就不同，同一次呼叫要同時驗到。
     *
     * <p>本測試的前身斷言 realtime 的 sellerId「必須是反解後的 account_id」。那在 2026-08-30 之前
     * 是對的：gateway 的 SSE 訂閱 topic 當時用 login account_id 建。commit {@code 57cefff9f} 把
     * 訂閱端改成 current store_id（讓買賣兩端對稱），發布端與這支測試都沒跟著回灌 ⇒ 對
     * {@code store_id != account_id} 的賣家，兩個 topic 永不相等、事件 100% 送不到，而 Hub 以
     * 精確鍵 fan-out ⇒ 查無訂閱者不產生任何錯誤或 log。
     */
    @Test
    void publishRefundCompleted_realtimeCarriesStoreIdWhileDomainEventCarriesAccountId() throws Exception {
        // 分岔賣家：store_id != login account_id。兩者相等時本測試對「有沒有反解析」零鑑別力，
        // 而那正是這個 bug 活到部署的原因。
        long storeId = 7001L;
        long resolvedAccountId = 42L;
        assertNotEquals(storeId, resolvedAccountId, "fixture 必須用分岔身分，否則本測試無鑑別力");
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(resolvedAccountId);

        AfterSalesEventPublisherImpl publisher = new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        publisher.publishRefundCompleted("A001", "O100", storeId, 2001L,
                AfterSalePlatformDecisionOutcome.FULL_REFUND, 1000L, "CREDIT_CARD", "TXN001");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(2)).publish(anyString(), anyString(), bodyCaptor.capture(), anyString());

        // 第二次 publish 呼叫是 seller.after_sale.updated（第一次是 aftersales.events domain event）
        String json = new String(bodyCaptor.getAllValues().get(1));
        assertTrue(json.contains("\"sellerId\":" + storeId),
                "realtime payload 的 sellerId 必須是 store_id(" + storeId + ")——gateway 的訂閱 topic 用 current store_id 建：" + json);
        assertFalse(json.contains("\"sellerId\":" + resolvedAccountId),
                "反解析成 login account_id(" + resolvedAccountId + ") 會讓事件落在沒有人訂閱的 topic 上：" + json);

        // domain event（notification-service 消費）維持 account_id 空間，不受本次修復影響
        String domainEventJson = new String(bodyCaptor.getAllValues().get(0));
        assertTrue(domainEventJson.contains("\"store_id\":\"" + storeId + "\""),
                "domain event payload 必須額外帶未反解的 store_id(" + storeId + ")：" + domainEventJson);
    }

    /**
     * 身分反解失敗只該影響 domain event 的 seller_account_id（降級帶 0），
     * 不該連帶讓賣家工作區 realtime 漏推 —— realtime 不再需要那個反解值。
     */
    @Test
    void publishRefundCompleted_realtimeStillPublishedWhenPrincipalResolutionFails() {
        long storeId = 7001L;
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(null);

        AfterSalesEventPublisherImpl publisher = new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        publisher.publishRefundCompleted("A001", "O100", storeId, 2001L,
                AfterSalePlatformDecisionOutcome.FULL_REFUND, 1000L, "CREDIT_CARD", "TXN001");

        // domain event ＋ realtime 兩則都要發出。修復前 realtime 會因為反解失敗被跳過，
        // 也就是說 seller-service 抖一下、賣家的訂單頁就靜靜地不更新。
        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(2)).publish(anyString(), anyString(), bodyCaptor.capture(), anyString());
        String realtimeJson = new String(bodyCaptor.getAllValues().get(1));
        assertTrue(realtimeJson.contains("\"sellerId\":" + storeId),
                "realtime 仍以 store_id 發出，與身分反解是否成功無關：" + realtimeJson);
    }

    @Test
    void publishCaseTransitionWithContext_resolvesStoreIdToAccountIdInDomainEventPayload() throws Exception {
        long storeId = 7001L;
        long resolvedAccountId = 42L; // 不同於 storeId：模擬已重新上架的賣家
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(resolvedAccountId);

        AfterSalesEventPublisherImpl publisher = new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        publisher.publishCaseTransitionWithContext("aftersales.case.some_transition", "some.event.type",
                "A001", "O100", 2001L, storeId, "buyer", null);

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(2)).publish(anyString(), anyString(), bodyCaptor.capture(), anyString());

        // 第一次 publish 呼叫是 domain event（aftersales.events），本測試斷言對象
        byte[] domainEventBody = bodyCaptor.getAllValues().get(0);
        String json = new String(domainEventBody);
        assertTrue(json.contains("\"seller_account_id\":\"" + resolvedAccountId + "\""),
                "domain event payload 的 seller_account_id 必須是反解後的 account_id(" + resolvedAccountId + ")，不是 store_id(" + storeId + ")：" + json);
        assertFalse(json.contains("\"seller_account_id\":\"" + storeId + "\""),
                "domain event payload 不應直接帶未反解的 store_id：" + json);
        assertTrue(json.contains("\"store_id\":\"" + storeId + "\""),
                "domain event payload 必須額外帶未反解的 store_id(" + storeId + ")：" + json);
    }
}

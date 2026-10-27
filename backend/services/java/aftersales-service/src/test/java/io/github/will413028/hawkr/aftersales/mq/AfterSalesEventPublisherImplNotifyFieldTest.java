package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * buyer_id 8-service rekey Phase 7: discriminating test for the
 * notify_buyer_account_id reverse-resolve fix. Uses divergent buyer_id/account_id
 * values (buyer_id != account_id) — a same-value test cannot distinguish the bug
 * from the fix.
 */
class AfterSalesEventPublisherImplNotifyFieldTest {

    @Test
    @DisplayName("publishCaseAutoRejected resolves buyer_id to login account_id in notify_buyer_account_id (discriminating)")
    void publishCaseAutoRejected_resolvesBuyerIdForNotification() throws Exception {
        long buyerId = 9001L;
        long resolvedAccountId = 42L;

        ObsRabbitPublisher obsRabbitPublisher = mock(ObsRabbitPublisher.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(sellerGrpcClient.resolveAccountIdForStore(7001L)).thenReturn(7001L);
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(buyerId, resolvedAccountId), Set.of()));

        AfterSalesEventPublisherImpl publisher =
                new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        publisher.publishCaseAutoRejected("CASE1", "ORD1", buyerId, 7001L, 24);

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher).publish(any(), eq("aftersales.case.auto_rejected"), bodyCaptor.capture(), any());

        Map<?, ?> payload = new ObjectMapper().readValue(bodyCaptor.getValue(), Map.class);
        assertEquals(String.valueOf(buyerId), payload.get("buyer_id"),
                "buyer_id must stay the raw stint-scoped business value (9001)");
        assertEquals(String.valueOf(resolvedAccountId), payload.get("notify_buyer_account_id"),
                "notify_buyer_account_id must be the resolved login account_id (42), not buyer_id (9001)");
        assertNotEquals(payload.get("buyer_id"), payload.get("notify_buyer_account_id"),
                "buyer_id and notify_buyer_account_id must diverge when buyer_id != account_id");
    }

    @Test
    @DisplayName("publishCaseAutoRejected falls back to 0 when buyer_id resolve fails (fail-closed, not silently dropped)")
    void publishCaseAutoRejected_fallsBackToZeroOnResolveFailure() throws Exception {
        long buyerId = 9001L;

        ObsRabbitPublisher obsRabbitPublisher = mock(ObsRabbitPublisher.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(sellerGrpcClient.resolveAccountIdForStore(7001L)).thenReturn(7001L);
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));

        AfterSalesEventPublisherImpl publisher =
                new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        publisher.publishCaseAutoRejected("CASE1", "ORD1", buyerId, 7001L, 24);

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher).publish(any(), eq("aftersales.case.auto_rejected"), bodyCaptor.capture(), any());

        Map<?, ?> payload = new ObjectMapper().readValue(bodyCaptor.getValue(), Map.class);
        assertEquals("0", payload.get("notify_buyer_account_id"),
                "resolve failure must fall back to 0 (safe no-match routing value), not throw or drop the event");
    }

    @Test
    @DisplayName("publishRefundCompleted resolves storeId->sellerAccountId and buyerId->notifyBuyerAccountId (discriminating)")
    void publishRefundCompleted_resolvesIdentityFields() throws Exception {
        long storeId = 7001L;
        long resolvedSellerAccountId = 42L; // 不同於 storeId
        long buyerId = 9001L;
        long resolvedBuyerAccountId = 55L; // 不同於 buyerId

        ObsRabbitPublisher obsRabbitPublisher = mock(ObsRabbitPublisher.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(resolvedSellerAccountId);
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(buyerId, resolvedBuyerAccountId), Set.of()));

        AfterSalesEventPublisherImpl publisher =
                new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        publisher.publishRefundCompleted("CASE1", "ORD1", storeId, buyerId,
                io.github.will413028.hawkr.common.utils.code.AfterSalePlatformDecisionOutcome.FULL_REFUND,
                1000L, "CREDIT_CARD", "EXT1");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher).publish(any(), eq("aftersales.refund.completed"), bodyCaptor.capture(), any());

        Map<?, ?> payload = new ObjectMapper().readValue(bodyCaptor.getValue(), Map.class);
        assertEquals("system", payload.get("actor"));
        assertEquals(String.valueOf(buyerId), payload.get("buyer_id"),
                "buyer_id must stay the raw stint-scoped business value (9001), not the resolved account_id");
        assertEquals(String.valueOf(resolvedSellerAccountId), payload.get("seller_account_id"),
                "seller_account_id 必須是反解後的 account_id(42)，不是 store_id(7001)");
        assertEquals(String.valueOf(resolvedBuyerAccountId), payload.get("notify_buyer_account_id"),
                "notify_buyer_account_id 必須是反解後的 account_id(55)，不是 buyer_id(9001)");
    }

    @Test
    @DisplayName("publishAwaitingBuyerRefundPage resolves buyerId to notify_buyer_account_id (discriminating)")
    void publishAwaitingBuyerRefundPage_resolvesNotifyBuyerAccountId() throws Exception {
        long buyerId = 9001L;
        long resolvedAccountId = 55L;

        ObsRabbitPublisher obsRabbitPublisher = mock(ObsRabbitPublisher.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(buyerId, resolvedAccountId), Set.of()));

        AfterSalesEventPublisherImpl publisher =
                new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        publisher.publishAwaitingBuyerRefundPage("ORD1", "CASE1", 9999L, buyerId);

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher).publish(any(), eq("aftersales.refund.awaiting_buyer_page"), bodyCaptor.capture(), any());

        Map<?, ?> payload = new ObjectMapper().readValue(bodyCaptor.getValue(), Map.class);
        assertEquals(String.valueOf(buyerId), payload.get("buyer_id"),
                "buyer_id must stay the raw stint-scoped business value (9001), not the resolved account_id");
        assertEquals(String.valueOf(resolvedAccountId), payload.get("notify_buyer_account_id"),
                "notify_buyer_account_id 必須是反解後的 account_id(55)，不是 buyer_id(9001)");
    }

    @Test
    @DisplayName("buyer_id 查無存活買家（unresolved_ids）：notify_buyer_account_id 帶 0，不回吐 buyer_id")
    void publishCaseAutoRejected_unresolvedBuyerIsNotEchoedAsAnAccount() throws Exception {
        long buyerId = 9001L;

        ObsRabbitPublisher obsRabbitPublisher = mock(ObsRabbitPublisher.class);
        SellerGrpcClient sellerGrpcClient = mock(SellerGrpcClient.class);
        BuyerGrpcClient buyerGrpcClient = mock(BuyerGrpcClient.class);

        when(sellerGrpcClient.resolveAccountIdForStore(7001L)).thenReturn(7001L);
        // 單體不回填；舊 buyer-service 會把輸入原樣放進 principals，兩種形狀都不得被當成帳號。
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(buyerId, buyerId), Set.of(buyerId)));

        AfterSalesEventPublisherImpl publisher =
                new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        publisher.publishCaseAutoRejected("CASE1", "ORD1", buyerId, 7001L, 24);

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher).publish(any(), eq("aftersales.case.auto_rejected"), bodyCaptor.capture(), any());
        Map<?, ?> payload = new ObjectMapper().readValue(bodyCaptor.getValue(), Map.class);
        assertEquals("0", String.valueOf(payload.get("notify_buyer_account_id")));
    }
}

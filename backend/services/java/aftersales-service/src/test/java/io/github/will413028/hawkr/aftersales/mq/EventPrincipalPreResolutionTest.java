package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.aftersales.service.PrincipalResolutionFailedException;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 P1-2：出路 4 的短 tx 內<b>不得有網路呼叫</b>。
 *
 * <p>兩組斷言：
 * <ul>
 *   <li><b>tx 內零「身分反解」gRPC</b>（限定詞是認真的，見下方 P1-B 註解）：帶 {@link EventPrincipals} 的三支 {@code tryPublishXxx}（＝ B1／B2／B3
 *       在 {@code @Transactional} 內呼叫的那三支）跑完之後，seller／buyer gRPC client
 *       <b>完全沒有被碰過</b>。這是修復前後的鑑別點——修復前每一支都會在 tx 內打兩次無 deadline 的
 *       gRPC（payload 一次 ＋ {@code emitSellerAfterSaleUpdated} 再一次）。</li>
 *   <li><b>strict 解析語意</b>：{@code resolveEventPrincipals} 任一側失敗即拋
 *       {@link PrincipalResolutionFailedException}，<b>不</b>降級成 null／0 把事件照發
 *       （那會讓排程用一則永遠路由不到收件人的通知，換掉一次本來會成功的重試）。</li>
 * </ul>
 */
class EventPrincipalPreResolutionTest {

    private ObsRabbitPublisher obsRabbitPublisher;
    private SellerGrpcClient sellerGrpcClient;
    private BuyerGrpcClient buyerGrpcClient;
    private AfterSalesEventPublisherImpl publisher;

    private static final long STORE_ID = 7001L;
    private static final long BUYER_ID = 9001L;
    /** 刻意與 storeId／buyerId 皆不同：同值測不出「有沒有真的用預解值」。 */
    private static final EventPrincipals PRINCIPALS = new EventPrincipals(42L, 55L);

    @BeforeEach
    void setUp() {
        obsRabbitPublisher = mock(ObsRabbitPublisher.class);
        sellerGrpcClient = mock(SellerGrpcClient.class);
        buyerGrpcClient = mock(BuyerGrpcClient.class);
        publisher = new AfterSalesEventPublisherImpl(obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
    }

    // ==================== tx 內零身分反解 gRPC ====================
    // ⚠️ 範圍限定（issue #2294 P1-B）：本組只 verifyNoInteractions seller／buyer 兩個 client，
    //    量不到 payment。ATM 路徑的 createRefund 仍在 tx 內（ReturnReceiptService
    //    .createRefundTaskAndAdvanceStatus 的 javadoc 有完整說明），不要把本組的綠燈
    //    讀成「tx 內已經沒有任何 gRPC」。

    @Test
    @DisplayName("B2 tryPublishCaseAutoRejected(principals)：tx 內零身分反解 gRPC，payload 用預解值")
    void tryPublishCaseAutoRejected_doesNotCallGrpcInsideTx() throws Exception {
        publisher.tryPublishCaseAutoRejected("A001", "O100", BUYER_ID, STORE_ID, 48, PRINCIPALS);

        verifyNoInteractions(sellerGrpcClient, buyerGrpcClient);
        Map<?, ?> payload = capturePayload("aftersales.case.auto_rejected");
        assertEquals("42", payload.get("seller_account_id"));
        assertEquals("55", payload.get("notify_buyer_account_id"));
    }

    @Test
    @DisplayName("B3 tryPublishCaseAutoEscalated(principals)：tx 內零身分反解 gRPC，payload 用預解值")
    void tryPublishCaseAutoEscalated_doesNotCallGrpcInsideTx() throws Exception {
        publisher.tryPublishCaseAutoEscalated("A001", "O100", BUYER_ID, STORE_ID, 48, PRINCIPALS);

        verifyNoInteractions(sellerGrpcClient, buyerGrpcClient);
        Map<?, ?> payload = capturePayload("aftersales.case.auto_escalated");
        assertEquals("42", payload.get("seller_account_id"));
        assertEquals("55", payload.get("notify_buyer_account_id"));
    }

    @Test
    @DisplayName("B1 tryPublishCaseTransitionWithContext(principals)：tx 內零身分反解 gRPC，payload 用預解值")
    void tryPublishCaseTransition_doesNotCallGrpcInsideTx() throws Exception {
        publisher.tryPublishCaseTransitionWithContext(
                "aftersales.case.auto_completed_return_pending_timeout",
                "CASE_AUTO_COMPLETED_RETURN_PENDING_TIMEOUT",
                "A001", "O100", BUYER_ID, STORE_ID, "system", Map.of(), PRINCIPALS);

        verifyNoInteractions(sellerGrpcClient, buyerGrpcClient);
        Map<?, ?> payload = capturePayload("aftersales.case.auto_completed_return_pending_timeout");
        assertEquals("42", payload.get("seller_account_id"));
        assertEquals("55", payload.get("notify_buyer_account_id"));
    }

    @Test
    @DisplayName("B7 tryPublishAwaitingBuyerRefundPage(principals)：tx 內零身分反解 gRPC，payload 用預解值")
    void tryPublishAwaitingBuyerRefundPage_doesNotCallGrpcInsideTx() throws Exception {
        publisher.tryPublishAwaitingBuyerRefundPage("O100", "A001", 9999L, BUYER_ID, PRINCIPALS);

        verifyNoInteractions(sellerGrpcClient, buyerGrpcClient);
        Map<?, ?> payload = capturePayload("aftersales.refund.awaiting_buyer_page");
        // 55 ＝ 預解值；9001 ＝ 未反解的 raw buyer_id。兩者不同才測得出「有沒有真的用預解值」
        assertEquals("55", payload.get("notify_buyer_account_id"));
        assertEquals(String.valueOf(BUYER_ID), payload.get("buyer_id"));
    }

    // ==================== strict 解析語意 ====================

    @Test
    @DisplayName("resolveEventPrincipals：seller 反解失敗 → 拋，不降級")
    void resolveEventPrincipals_sellerFailure_throws() {
        // SellerGrpcClient 內部 catch 後回 null ＝ 該次 gRPC 失敗（非 null storeId 成功時必回非 null）
        when(sellerGrpcClient.resolveAccountIdForStore(STORE_ID)).thenReturn(null);

        assertThrows(PrincipalResolutionFailedException.class,
                () -> publisher.resolveEventPrincipals(STORE_ID, BUYER_ID));
    }

    @Test
    @DisplayName("resolveEventPrincipals：buyer 反解失敗 → 拋，不退回 0L")
    void resolveEventPrincipals_buyerFailure_throws() {
        when(sellerGrpcClient.resolveAccountIdForStore(STORE_ID)).thenReturn(42L);
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(BUYER_ID)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));

        assertThrows(PrincipalResolutionFailedException.class,
                () -> publisher.resolveEventPrincipals(STORE_ID, BUYER_ID));
    }

    @Test
    @DisplayName("resolveEventPrincipals：兩側都成功 → 回反解後的值（非原始 store_id／buyer_id）")
    void resolveEventPrincipals_happyPath() {
        when(sellerGrpcClient.resolveAccountIdForStore(STORE_ID)).thenReturn(42L);
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(BUYER_ID)))
                .thenReturn(new PrincipalResolution(Map.of(BUYER_ID, 55L), Set.of()));

        EventPrincipals p = publisher.resolveEventPrincipals(STORE_ID, BUYER_ID);

        assertEquals(42L, p.sellerAccountId());
        assertEquals(55L, p.notifyBuyerAccountId());
    }

    private Map<?, ?> capturePayload(String routingKey) throws Exception {
        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher).publish(any(), eq(routingKey), bodyCaptor.capture(), any());
        return new ObjectMapper().readValue(bodyCaptor.getValue(), Map.class);
    }
}

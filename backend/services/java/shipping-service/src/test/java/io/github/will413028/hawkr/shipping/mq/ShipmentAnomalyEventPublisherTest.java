package io.github.will413028.hawkr.shipping.mq;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.mq.payload.ShipmentAnomalyDetectedPayload;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 單元測試：驗證 {@link ShipmentAnomalyEventPublisher#publishAnomalyDetected} 發布的
 * payload 帶「已解析」的賣家 accountId，而非呼叫端傳入的 store_id 原始值（單一 choke point，
 * 修好這裡即可讓 TcatCallStuckAlertScheduler / ShipmentStatusReconcileScheduler /
 * ShipmentLabelReconcileScheduler / ShippingService 共 7 個呼叫點一併正確）。
 * buyerAccountId 對稱補上相同斷言（見 resolveNotifyBuyerAccountId）。
 */
class ShipmentAnomalyEventPublisherTest {

    private ObsRabbitPublisher rabbitPublisher;
    private ObjectMapper objectMapper;
    private SellerGrpcClient sellerGrpcClient;
    private BuyerGrpcClient buyerGrpcClient;
    private ShipmentAnomalyEventPublisher publisher;

    @BeforeEach
    void setUp() {
        rabbitPublisher = mock(ObsRabbitPublisher.class);
        objectMapper = mock(ObjectMapper.class);
        sellerGrpcClient = mock(SellerGrpcClient.class);
        buyerGrpcClient = mock(BuyerGrpcClient.class);
        publisher = new ShipmentAnomalyEventPublisher(rabbitPublisher, objectMapper, sellerGrpcClient, buyerGrpcClient);
    }

    @Test
    void publishAnomalyDetected_resolvesStoreIdToAccountId_notRawStoreId() throws Exception {
        long storeId = 9999L;
        long resolvedAccountId = 1001L; // 刻意 != storeId
        long buyerId = 888L;

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(storeId, SellerMeta.newBuilder().setAccountId(resolvedAccountId).build())
                        .build());
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, buyerId)));

        ArgumentCaptor<Object> payloadCaptor = ArgumentCaptor.forClass(Object.class);
        when(objectMapper.writeValueAsBytes(payloadCaptor.capture())).thenReturn(new byte[0]);

        publisher.publishAnomalyDetected(
                ShipmentAnomalyType.TCAT_CALL_FAILED,
                "O100",
                "1",
                storeId,
                buyerId);

        verify(rabbitPublisher).publish(anyString(), anyString(), any(byte[].class), anyString());
        ShipmentAnomalyDetectedPayload payload = (ShipmentAnomalyDetectedPayload) payloadCaptor.getValue();
        assertEquals(resolvedAccountId, payload.sellerAccountId());
    }

    /**
     * <b>語意已翻轉（issue #2789 階段 B2）</b>：本測試原名
     * {@code publishAnomalyDetected_resolverThrows_fallsBackToRawStoreId}，斷言 fail-open
     * 回吐原始 storeId、且把那個行為當成刻意設計。現在斷言的是相反的事——回 {@code 0L}。
     *
     * <p><b>為什麼翻轉</b>：回吐的 store_id 是<b>另一個身分空間</b>的合法非零值。它送不到任何人
     * （target_user_field = {@code seller_account_id}），卻會在 {@code notifications.user_id}
     * 留下寫錯身分空間的一列，而且完全無聲。帶 0 之後
     * {@code template_handler.extractUserID} 對 {@code id <= 0} 回 error ⇒ 跳過並留 WARN：
     * <b>投遞結果不變（本來就送不到），但從無聲變成看得見</b>。
     *
     * <p>不變的部分：例外不得外洩（本 publisher 是 fire-and-forget，不可影響 scheduler 主流程）。
     */
    @Test
    void publishAnomalyDetected_resolverThrows_publishesZero_notRawStoreId() throws Exception {
        long storeId = 9999L;
        long buyerId = 888L;

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenThrow(new RuntimeException("seller-service unavailable"));
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, buyerId)));

        ArgumentCaptor<Object> payloadCaptor = ArgumentCaptor.forClass(Object.class);
        when(objectMapper.writeValueAsBytes(payloadCaptor.capture())).thenReturn(new byte[0]);

        publisher.publishAnomalyDetected(
                ShipmentAnomalyType.TCAT_CALL_FAILED,
                "O100",
                "1",
                storeId,
                buyerId);

        ShipmentAnomalyDetectedPayload payload = (ShipmentAnomalyDetectedPayload) payloadCaptor.getValue();
        assertEquals(0L, payload.sellerAccountId());
        // issue #2707 Phase A：raw storeId 不受 sellerAccountId 解析失敗降級影響，仍是原始值。
        assertEquals(storeId, payload.storeId());
    }

    /**
     * seller-service 回應成功但 map 裡沒有這個 store_id（{@code GetSellerMetaByStoreIDs} 是
     * PK lookup、<b>不做 COALESCE echo-back</b>，所以 map-miss 就是「查無此 store_id」）。
     * 與上一支的差別：那支是 transport 失敗，這支是<b>成功回應但查無</b>——兩條分支各自釘住，
     * 因為它們在改動前都回吐 storeId、在 log 上也無法區分。
     */
    @Test
    void publishAnomalyDetected_storeNotFound_publishesZero_notRawStoreId() throws Exception {
        long storeId = 9999L;
        long buyerId = 888L;

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder().build());
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, buyerId)));

        ArgumentCaptor<Object> payloadCaptor = ArgumentCaptor.forClass(Object.class);
        when(objectMapper.writeValueAsBytes(payloadCaptor.capture())).thenReturn(new byte[0]);

        publisher.publishAnomalyDetected(
                ShipmentAnomalyType.TCAT_CALL_FAILED,
                "O100",
                "1",
                storeId,
                buyerId);

        ShipmentAnomalyDetectedPayload payload = (ShipmentAnomalyDetectedPayload) payloadCaptor.getValue();
        assertEquals(0L, payload.sellerAccountId());
        // issue #2707 Phase A：raw storeId 不受 sellerAccountId 解析失敗降級影響，仍是原始值。
        assertEquals(storeId, payload.storeId());
    }

    @Test
    void publishAnomalyDetected_resolvesBuyerIdToAccountId_notRawBuyerId() throws Exception {
        long storeId = 9999L;
        long buyerId = 888L;
        long resolvedBuyerAccountId = 2002L; // 刻意 != buyerId

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(storeId, SellerMeta.newBuilder().setAccountId(1001L).build())
                        .build());
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedBuyerAccountId)));

        ArgumentCaptor<Object> payloadCaptor = ArgumentCaptor.forClass(Object.class);
        when(objectMapper.writeValueAsBytes(payloadCaptor.capture())).thenReturn(new byte[0]);

        publisher.publishAnomalyDetected(
                ShipmentAnomalyType.TCAT_CALL_FAILED,
                "O100",
                "1",
                storeId,
                buyerId);

        verify(rabbitPublisher).publish(anyString(), anyString(), any(byte[].class), anyString());
        ShipmentAnomalyDetectedPayload payload = (ShipmentAnomalyDetectedPayload) payloadCaptor.getValue();
        assertEquals(resolvedBuyerAccountId, payload.buyerAccountId());
    }

    /**
     * <b>語意已翻轉（issue #2789 階段 B2）</b>：原名
     * {@code publishAnomalyDetected_buyerResolverThrows_fallsBackToRawBuyerId}，
     * 與賣家側同一理由改為斷言 {@code 0L}（見上方 seller 側翻轉說明）。
     */
    @Test
    void publishAnomalyDetected_buyerResolverThrows_publishesZero_notRawBuyerId() throws Exception {
        long storeId = 9999L;
        long buyerId = 888L;

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(storeId, SellerMeta.newBuilder().setAccountId(1001L).build())
                        .build());
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));

        ArgumentCaptor<Object> payloadCaptor = ArgumentCaptor.forClass(Object.class);
        when(objectMapper.writeValueAsBytes(payloadCaptor.capture())).thenReturn(new byte[0]);

        publisher.publishAnomalyDetected(
                ShipmentAnomalyType.TCAT_CALL_FAILED,
                "O100",
                "1",
                storeId,
                buyerId);

        ShipmentAnomalyDetectedPayload payload = (ShipmentAnomalyDetectedPayload) payloadCaptor.getValue();
        assertEquals(0L, payload.buyerAccountId());
    }

    /**
     * buyer 軸的 {@code ResolvePrincipals} 對查無在架列的 id <b>原樣回吐進 map</b>
     * （{@code buyer.proto} 的 {@code unresolved_ids} 逐字寫明那是 UNVERIFIED echo-back），
     * 所以「map 裡沒有」在買家軸是死碼、{@code getOrDefault} 的 default 永遠打不到。
     * 本測試釘住真正的判別依據：<b>map 帶著 echo-back，但 unresolvedIds 說了實話</b> ⇒ 帶 0。
     */
    @Test
    void publishAnomalyDetected_buyerUnresolvedButEchoedBack_publishesZero() throws Exception {
        long storeId = 9999L;
        long buyerId = 888L;

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(storeId, SellerMeta.newBuilder().setAccountId(1001L).build())
                        .build());
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.of(Map.of(buyerId, buyerId), Set.of(buyerId)));

        ArgumentCaptor<Object> payloadCaptor = ArgumentCaptor.forClass(Object.class);
        when(objectMapper.writeValueAsBytes(payloadCaptor.capture())).thenReturn(new byte[0]);

        publisher.publishAnomalyDetected(
                ShipmentAnomalyType.TCAT_CALL_FAILED,
                "O100",
                "1",
                storeId,
                buyerId);

        ShipmentAnomalyDetectedPayload payload = (ShipmentAnomalyDetectedPayload) payloadCaptor.getValue();
        assertEquals(0L, payload.buyerAccountId());
        // 賣家側正常解析不受影響（釘住「兩軸各自獨立降級」）
        assertEquals(1001L, payload.sellerAccountId());
    }
}

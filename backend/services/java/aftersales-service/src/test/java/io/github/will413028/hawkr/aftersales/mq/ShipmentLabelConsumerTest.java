package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;

import java.lang.reflect.Method;
import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ShipmentLabelConsumerTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private ObsRabbitPublisher obsRabbitPublisher;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;

    @Test
    void publishPayUniLgsNoCodeAnomaly_resolvesStoreIdToAccountIdInShipmentAnomalyPayload() throws Exception {
        long storeId = 7001L;
        long resolvedAccountId = 42L; // 不同於 storeId：模擬已重新上架的賣家
        long buyerId = 2001L;

        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(storeId);
        acase.setBuyerId(buyerId);

        when(caseMapper.selectOne(any())).thenReturn(acase);
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(resolvedAccountId);
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(buyerId, buyerId), Set.of()));

        ShipmentLabelConsumer consumer = new ShipmentLabelConsumer(caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        Method publishPayUniLgsNoCodeAnomaly = ShipmentLabelConsumer.class
                .getDeclaredMethod("publishPayUniLgsNoCodeAnomaly", String.class);
        publishPayUniLgsNoCodeAnomaly.setAccessible(true);
        publishPayUniLgsNoCodeAnomaly.invoke(consumer, "O100");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publish(anyString(), anyString(), bodyCaptor.capture(), anyString());

        String json = new String(bodyCaptor.getValue());
        assertTrue(json.contains("\"seller_account_id\":" + resolvedAccountId),
                "shipment anomaly payload 的 seller_account_id 必須是反解後的 account_id(" + resolvedAccountId + ")，不是 store_id(" + storeId + ")：" + json);
        assertFalse(json.contains("\"seller_account_id\":" + storeId),
                "shipment anomaly payload 不應直接帶未反解的 store_id：" + json);
        // issue #2707 Phase A：新增 storeId 欄位必須是未反解的原始值，與 seller_account_id 分開帶。
        assertTrue(json.contains("\"store_id\":" + storeId),
                "shipment anomaly payload 必須帶原始 store_id(" + storeId + ")：" + json);
    }

    @Test
    void publishPayUniLgsNoCodeAnomaly_resolvesBuyerIdToAccountIdInShipmentAnomalyPayload() throws Exception {
        long storeId = 7001L;
        long buyerId = 2001L;
        long resolvedBuyerAccountId = 88L; // 刻意 != buyerId

        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(storeId);
        acase.setBuyerId(buyerId);

        when(caseMapper.selectOne(any())).thenReturn(acase);
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(42L);
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(buyerId, resolvedBuyerAccountId), Set.of()));

        ShipmentLabelConsumer consumer = new ShipmentLabelConsumer(caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        Method publishPayUniLgsNoCodeAnomaly = ShipmentLabelConsumer.class
                .getDeclaredMethod("publishPayUniLgsNoCodeAnomaly", String.class);
        publishPayUniLgsNoCodeAnomaly.setAccessible(true);
        publishPayUniLgsNoCodeAnomaly.invoke(consumer, "O100");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publish(anyString(), anyString(), bodyCaptor.capture(), anyString());

        String json = new String(bodyCaptor.getValue());
        assertTrue(json.contains("\"buyer_account_id\":" + resolvedBuyerAccountId),
                "shipment anomaly payload 的 buyer_account_id 必須是反解後的 account_id(" + resolvedBuyerAccountId + ")，不是 buyer_id(" + buyerId + ")：" + json);
        assertFalse(json.contains("\"buyer_account_id\":" + buyerId),
                "shipment anomaly payload 不應直接帶未反解的 buyer_id：" + json);
    }

    @Test
    void publishPayUniLgsNoCodeAnomaly_unresolvedBuyerIsNotEchoedAsAnAccountId() throws Exception {
        long buyerId = 2001L;
        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(7001L);
        acase.setBuyerId(buyerId);

        when(caseMapper.selectOne(any())).thenReturn(acase);
        when(sellerGrpcClient.resolveAccountIdForStore(7001L)).thenReturn(42L);
        // 查無存活買家：單體不回填 principals；舊 buyer-service 會回填，但 unresolved_ids 有它。
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(buyerId, buyerId), Set.of(buyerId)));

        ShipmentLabelConsumer consumer = new ShipmentLabelConsumer(caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        Method publish = ShipmentLabelConsumer.class.getDeclaredMethod("publishPayUniLgsNoCodeAnomaly", String.class);
        publish.setAccessible(true);
        publish.invoke(consumer, "O100");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publish(anyString(), anyString(), bodyCaptor.capture(), anyString());
        String json = new String(bodyCaptor.getValue());
        assertFalse(json.contains("\"buyer_account_id\":" + buyerId),
                "未解析的 buyer_id 不得被當成 account_id：" + json);
    }

    @Test
    void publishPayUniLgsNoCodeAnomaly_unresolvedSellerAndBuyerUseTheSameAbsentRepresentation() throws Exception {
        long buyerId = 2001L;
        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(7001L);
        acase.setBuyerId(buyerId);

        when(caseMapper.selectOne(any())).thenReturn(acase);
        when(sellerGrpcClient.resolveAccountIdForStore(7001L)).thenReturn(0L); // 查無存活賣家
        when(buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId)))
                .thenReturn(new PrincipalResolution(Map.of(), Set.of(buyerId)));

        ShipmentLabelConsumer consumer = new ShipmentLabelConsumer(caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        Method publish = ShipmentLabelConsumer.class.getDeclaredMethod("publishPayUniLgsNoCodeAnomaly", String.class);
        publish.setAccessible(true);
        publish.invoke(consumer, "O100");

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publish(anyString(), anyString(), bodyCaptor.capture(), anyString());
        var node = new com.fasterxml.jackson.databind.ObjectMapper().readTree(bodyCaptor.getValue());
        assertTrue(node.path("seller_account_id").isNull(), "seller_account_id 應為 null：" + node);
        assertTrue(node.path("buyer_account_id").isNull(), "buyer_account_id 應為 null：" + node);
    }
}

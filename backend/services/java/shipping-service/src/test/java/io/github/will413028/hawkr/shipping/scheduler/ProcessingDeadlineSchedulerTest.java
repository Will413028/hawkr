package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 單元測試：驗證 {@link ProcessingDeadlineScheduler#scanExpiredShipments} 發布逾期事件時，
 * sellerAccountId 帶「已解析」的賣家 accountId，而非 shipment.storeId 原始值；並驗證
 * 每輪掃描只呼叫一次 batchGetSellerMeta（批次解析，避免逐筆 N+1 gRPC 呼叫）。
 */
class ProcessingDeadlineSchedulerTest {

    private ShipmentMapper shipmentMapper;
    private ShippingEventPublisher eventPublisher;
    private SellerGrpcClient sellerGrpcClient;
    private ProcessingDeadlineScheduler scheduler;

    @BeforeEach
    void setUp() {
        shipmentMapper = mock(ShipmentMapper.class);
        eventPublisher = mock(ShippingEventPublisher.class);
        sellerGrpcClient = mock(SellerGrpcClient.class);
        scheduler = new ProcessingDeadlineScheduler(shipmentMapper, eventPublisher, sellerGrpcClient);
        // issue #2294 L1：加了 lease 閘門之後，Mockito 對 int 預設回 0 ⇒ `if (leased == 0) continue;`
        // ⇒ 一則都不會 publish ⇒ 本檔三支全紅。這裡 stub 成「搶到」，讓原本的斷言回到它們要驗的事；
        // 閘門本身的兩條分支由 ProcessingDeadlineSchedulerClaimTest 覆蓋。
        when(shipmentMapper.leaseProcessingDeadline(anyLong(), any(), any())).thenReturn(1);
    }

    private static ShipmentEntity shipment(long id, String orderNo, long storeId, long buyerId) {
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(id);
        entity.setOrderNo(orderNo);
        entity.setStoreId(storeId);
        entity.setBuyerId(buyerId);
        return entity;
    }

    @Test
    void scanExpiredShipments_publishesResolvedAccountId_batchedOncePerScan_notPerItem() {
        ShipmentEntity s1 = shipment(1L, "O100", 100L, 555L);
        ShipmentEntity s2 = shipment(2L, "O200", 200L, 666L);

        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of(s1, s2));
        when(sellerGrpcClient.batchGetSellerMeta(anyList()))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(100L, SellerMeta.newBuilder().setAccountId(11L).build())
                        .putSellers(200L, SellerMeta.newBuilder().setAccountId(22L).build())
                        .build());

        scheduler.scanExpiredShipments();

        // 批次解析：不論筆數多少，本輪只打一次 gRPC（防 N+1）
        verify(sellerGrpcClient, times(1)).batchGetSellerMeta(anyList());

        ArgumentCaptor<Long> s1AccountIdCaptor = ArgumentCaptor.forClass(Long.class);
        verify(eventPublisher).publishProcessingTimeout(eq("O100"), s1AccountIdCaptor.capture(), eq(555L));
        assertEquals(11L, s1AccountIdCaptor.getValue());

        ArgumentCaptor<Long> s2AccountIdCaptor = ArgumentCaptor.forClass(Long.class);
        verify(eventPublisher).publishProcessingTimeout(eq("O200"), s2AccountIdCaptor.capture(), eq(666L));
        assertEquals(22L, s2AccountIdCaptor.getValue());
    }

    /**
     * <b>語意已翻轉（issue #2789 階段 B2）</b>：原名
     * {@code scanExpiredShipments_batchResolverThrows_fallsBackToRawStoreIds}，
     * 斷言「本輪全數退回原始 store_id」並把 fail-open 當成刻意設計。現在斷言帶 {@code 0L}。
     *
     * <p><b>為什麼翻轉</b>：{@code shipment.processing.timeout} <b>沒有任何 notification
     * template</b>（唯一消費者 aftersales {@code ProcessingTimeoutConsumer} 只讀
     * {@code orderNo}）⇒ 回吐 store_id 的投遞收益是零，代價卻是讓另一個身分空間的
     * 合法非零值繼續在事件裡流動、且完全無聲。
     *
     * <p>不變的部分（本測試同時仍在守）：例外不得中斷這條半關鍵路徑——
     * {@code publishProcessingTimeout} 仍被呼叫，orderNo 與裸 buyer_id 都照舊。
     */
    @Test
    void scanExpiredShipments_batchResolverThrows_publishesZero_notRawStoreIds() {
        ShipmentEntity s1 = shipment(1L, "O100", 9999L, 555L);

        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of(s1));
        when(sellerGrpcClient.batchGetSellerMeta(anyList()))
                .thenThrow(new RuntimeException("seller-service unavailable"));

        scheduler.scanExpiredShipments();

        verify(eventPublisher).publishProcessingTimeout("O100", 0L, 555L);
    }

    /**
     * 批次成功但其中一筆的 store_id 不在回傳 map 裡（{@code GetSellerMetaByStoreIDs} 是
     * PK lookup、不做 echo-back ⇒ map-miss 就是「查無此 store_id」）。
     * 與上一支的差別：那支整批 transport 失敗，這支是<b>部分命中</b>——
     * 釘住「已解析的那筆照常帶解析值、未解析的那筆帶 0」，兩者互不污染。
     */
    @Test
    void scanExpiredShipments_partialMapMiss_zeroesOnlyTheMissingOne() {
        ShipmentEntity s1 = shipment(1L, "O100", 100L, 555L);
        ShipmentEntity s2 = shipment(2L, "O200", 9999L, 666L);

        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of(s1, s2));
        when(sellerGrpcClient.batchGetSellerMeta(anyList()))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(100L, SellerMeta.newBuilder().setAccountId(11L).build())
                        .build());

        scheduler.scanExpiredShipments();

        verify(eventPublisher).publishProcessingTimeout("O100", 11L, 555L);
        verify(eventPublisher).publishProcessingTimeout("O200", 0L, 666L);
    }

    @Test
    void scanExpiredShipments_noExpiredShipments_skipsResolverCallEntirely() {
        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of());

        scheduler.scanExpiredShipments();

        verify(sellerGrpcClient, never()).batchGetSellerMeta(any());
    }
}

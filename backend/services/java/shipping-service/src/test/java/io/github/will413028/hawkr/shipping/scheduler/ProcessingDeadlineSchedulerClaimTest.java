package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>L1</b>：{@link ProcessingDeadlineScheduler} 的 lease 閘門。
 *
 * <p>原本<b>完全沒有搶佔</b>：{@code findExpiredUnprocessedShipments} → 兩則 publish →
 * {@code clearProcessingDeadline}。多副本下同一張逾期單被兩個 pod 各通知一次，
 * AfterSales 端收到兩則系統取消+退款。
 *
 * <p>修法＝lease on 既有的 {@code shipments.processing_deadline}（候選述詞本來就用這一欄）。
 */
class ProcessingDeadlineSchedulerClaimTest {

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
        when(sellerGrpcClient.batchGetSellerMeta(anyList()))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder().build());
    }

    private static ShipmentEntity shipment(long id, String orderNo) {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(id);
        e.setOrderNo(orderNo);
        e.setStoreId(100L);
        e.setBuyerId(555L);
        return e;
    }

    /**
     * 反向斷言：閘門回 0（另一個 pod 已搶走）⇒ <b>一個副作用都不准發生</b>。
     *
     * <p>沒有這一支，「stub 成 1 之後原測試變綠」只證明程式碼還會 publish，
     * 不證明閘門真的擋得住任何東西（Mockito 對 {@code int} 預設回 0，
     * 若閘門被寫成 {@code if (leased < 0)} 這種永遠不成立的條件，正向測試照樣綠）。
     */
    @Test
    @DisplayName("claim 回 0（他人持有租約）⇒ 不 publish、不 clear")
    void claimLost_publishesNothing() {
        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of(shipment(1L, "O100")));
        when(shipmentMapper.leaseProcessingDeadline(anyLong(), any(), any())).thenReturn(0);

        scheduler.scanExpiredShipments();

        verify(eventPublisher, never()).publishProcessingCleared(any());
        verify(eventPublisher, never()).publishProcessingTimeout(any(), any(), any());
        verify(shipmentMapper, never()).clearProcessingDeadline(anyLong());
    }

    @Test
    @DisplayName("claim 回 1 ⇒ 照常兩則 publish，且 clearProcessingDeadline 仍在 publish 之後")
    void claimWon_publishesAndThenClears() {
        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of(shipment(1L, "O100")));
        when(shipmentMapper.leaseProcessingDeadline(anyLong(), any(), any())).thenReturn(1);

        scheduler.scanExpiredShipments();

        var inOrder = org.mockito.Mockito.inOrder(shipmentMapper, eventPublisher);
        inOrder.verify(shipmentMapper).leaseProcessingDeadline(anyLong(), any(), any());
        inOrder.verify(eventPublisher).publishProcessingCleared("O100");
        inOrder.verify(eventPublisher).publishProcessingTimeout(any(), any(), any());
        // clear 是 SET processing_deadline = NULL（永久標記），必須留在 publish **之後**：
        // 上移到 publish 之前 ⇒ publish 拋例外或 pod 崩潰時事件永久遺失且無告警。
        inOrder.verify(shipmentMapper).clearProcessingDeadline(1L);
    }

    /**
     * 租約基準必須在<b>迴圈內、claim 當下</b>取，不能在方法開頭（tick 起點）只算一次。
     *
     * <p>只算一次 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的<b>有效</b>租約
     * = {@code LEASE_MINUTES} − (前 k−1 筆的耗時)，不變式退化成「批次總耗時 &lt; 租約」，
     * 會隨 {@code batch-size}（預設 50）一起破。基準改在 claim 當下取之後，
     * 不變式降為「<b>單筆</b>最壞耗時 &lt; 租約」，與批量無關。
     */
    @Test
    @DisplayName("租約基準取 claim 當下——第二筆的租約嚴格晚於第一筆（迴圈外只算一次會轉紅）")
    void leaseBaselineIsPerItemNotPerTick() {
        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of(shipment(1L, "O100"), shipment(2L, "O200")));
        when(shipmentMapper.leaseProcessingDeadline(anyLong(), any(), any())).thenReturn(1);
        // 讓第一筆 claim 之後的工作耗掉一段可觀測的時間
        org.mockito.Mockito.doAnswer(inv -> {
            Thread.sleep(40L);
            return null;
        }).when(eventPublisher).publishProcessingCleared(any());

        scheduler.scanExpiredShipments();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(shipmentMapper, times(2))
                .leaseProcessingDeadline(anyLong(), any(), leases.capture());
        long deltaMs = Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();

        assertTrue(deltaMs >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                        + "兩者相等 ⇒ 租約基準在迴圈外只算一次 ⇒ 不變式退化成「批次總耗時 < 租約」。");
    }

    /**
     * 租約長度的<b>上界</b>。
     *
     * <p>⚠️ 只驗「租約是未來時間」對任何大值都成立（1440 分照樣綠），所以那種斷言沒有鑑別力。
     * 這裡釘的是業務上界：租約同時是「pod 在 claim 與 publish 之間崩潰」的恢復窗口，
     * 逾期取消+退款是買家在等的錢，恢復窗口不該長於一個 tick（{@code interval-ms} 預設 5 分）。
     */
    @Test
    @DisplayName("租約長度 = 5 分鐘且不超過一個 tick（改大成小時級會轉紅）")
    void leaseLengthIsBoundedByOneTick() {
        when(shipmentMapper.findExpiredUnprocessedShipments(any(), anyInt()))
                .thenReturn(List.of(shipment(1L, "O100")));
        when(shipmentMapper.leaseProcessingDeadline(anyLong(), any(), any())).thenReturn(1);

        LocalDateTime before = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();
        scheduler.scanExpiredShipments();

        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(shipmentMapper).leaseProcessingDeadline(anyLong(), any(), lease.capture());
        long leaseSeconds = Duration.between(before, lease.getValue()).getSeconds();

        assertTrue(leaseSeconds > 0,
                "租約必須是未來時間（0 或負 ⇒ 任何人都搶得到 ⇒ 等於沒有搶佔）");
        // interval-ms 預設 300000 = 5 分鐘（ProcessingDeadlineScheduler 的 @Scheduled 預設值）
        long oneTickSeconds = 300L;
        assertTrue(leaseSeconds <= oneTickSeconds + 5,
                "租約 = pod 崩潰後的恢復窗口，實測 " + leaseSeconds + " 秒 > 一個 tick("
                        + oneTickSeconds + " 秒)。取更大值只會讓逾期取消+退款被延後更久，"
                        + "而候選述詞無時間上界、不會替你擋住這件事。");
        assertTrue(ProcessingDeadlineScheduler.LEASE_MINUTES == 5L,
                "租約長度常數應與 interval-ms 預設值（5 分）對齊，實際 "
                        + ProcessingDeadlineScheduler.LEASE_MINUTES + " 分");
    }
}

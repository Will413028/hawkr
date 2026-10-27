package io.github.will413028.hawkr.shipping.scheduler;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.response.QueryShipmentResponse;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.test.util.ReflectionTestUtils;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>L2 / L3</b>：兩支 §5.19 補收排程的 lease 閘門。
 *
 * <p>兩支原本都<b>完全沒有搶佔</b>，多副本下同一張單被兩個 pod 各打一次 PayUni 查詢、
 * 各發一次異常事件。
 * <ul>
 *   <li><b>L2</b> 貨態補收：lease on <b>既有的</b> {@code last_reconciled_at}（零 migration）</li>
 *   <li><b>L3</b> 列印補收：lease on <b>新增的</b> {@code label_reconcile_next_at}（V10）</li>
 * </ul>
 */
class ShipmentReconcileClaimTest {

    private static ShipmentEntity shipment(long id, String shipmentNo) {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(id);
        e.setShipmentNo(shipmentNo);
        e.setOrderNo("O" + id);
        e.setStoreId(100L);
        e.setBuyerId(555L);
        e.setMerId("HAWK100");
        e.setShipTradeNo("ST" + id);
        e.setLabelPrintStatus("4102");
        e.setShippingMethod("3002");
        return e;
    }

    @Nested
    @DisplayName("L2 ShipmentStatusReconcileScheduler")
    class StatusReconcile {

        private ShipmentMapper shipmentMapper;
        private ShippingService shippingService;
        private ShipmentAnomalyEventPublisher anomalyPublisher;
        private ShipmentStatusReconcileScheduler scheduler;

        @BeforeEach
        void setUp() {
            shipmentMapper = mock(ShipmentMapper.class);
            shippingService = mock(ShippingService.class);
            anomalyPublisher = mock(ShipmentAnomalyEventPublisher.class);
            scheduler = new ShipmentStatusReconcileScheduler(shipmentMapper, shippingService, anomalyPublisher);
            ReflectionTestUtils.setField(scheduler, "batchSize", 100);
            ReflectionTestUtils.setField(scheduler, "maxAgeDays", 7);
            ReflectionTestUtils.setField(scheduler, "backoffMinutes", 15);
            ReflectionTestUtils.setField(scheduler, "packageLostDays", 7);
        }

        /** 反向斷言：claim 回 0 ⇒ 不准打 PayUni、不准 reload、不准發異常事件。 */
        @Test
        @DisplayName("claim 回 0 ⇒ 不打 PayUni 查詢、不發異常事件")
        void claimLost_doesNothing() {
            when(shipmentMapper.findReconcileCandidates(any(), any(), anyInt()))
                    .thenReturn(List.of(shipment(1L, "SH1")));
            when(shipmentMapper.claimForReconcile(anyLong(), any(), any())).thenReturn(0);

            scheduler.reconcile();

            verify(shippingService, never()).queryAndSyncStatus(anyString());
            verify(shipmentMapper, never()).selectById(anyLong());
            verify(anomalyPublisher, never())
                    .publishAnomalyDetected(any(), anyString(), anyString(), anyLong(), anyLong());
        }

        @Test
        @DisplayName("claim 回 1 ⇒ 照常同步貨態")
        void claimWon_syncs() {
            when(shipmentMapper.findReconcileCandidates(any(), any(), anyInt()))
                    .thenReturn(List.of(shipment(1L, "SH1")));
            when(shipmentMapper.claimForReconcile(anyLong(), any(), any())).thenReturn(1);

            scheduler.reconcile();

            verify(shippingService).queryAndSyncStatus("SH1");
        }

        /**
         * 租約基準必須在迴圈內、claim 當下取。
         *
         * <p>L2 的批量預設 100、單筆是一次 PayUni HTTP ⇒ 若基準在 tick 起點只算一次，
         * 不變式就是「100 筆的總耗時 &lt; 15 分」，那是參數巧合不是結構保證。
         */
        @Test
        @DisplayName("租約基準取 claim 當下——第二筆嚴格晚於第一筆")
        void leaseBaselineIsPerItem() {
            when(shipmentMapper.findReconcileCandidates(any(), any(), anyInt()))
                    .thenReturn(List.of(shipment(1L, "SH1"), shipment(2L, "SH2")));
            when(shipmentMapper.claimForReconcile(anyLong(), any(), any())).thenReturn(1);
            org.mockito.Mockito.doAnswer(inv -> {
                Thread.sleep(40L);
                return null;
            }).when(shippingService).queryAndSyncStatus(anyString());

            scheduler.reconcile();

            ArgumentCaptor<LocalDateTime> nows = ArgumentCaptor.forClass(LocalDateTime.class);
            verify(shipmentMapper, times(2)).claimForReconcile(anyLong(), nows.capture(), any());
            long deltaMs = Duration
                    .between(nows.getAllValues().get(0), nows.getAllValues().get(1)).toMillis();
            assertTrue(deltaMs >= 20L,
                    "第二筆的租約基準必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                            + "兩者相等 ⇒ 基準在迴圈外只算一次 ⇒ 不變式退化成「批次總耗時 < 租約」，"
                            + "會被 batch-size=100 直接打破。");
        }
    }

    @Nested
    @DisplayName("L3 ShipmentLabelReconcileScheduler")
    class LabelReconcile {

        private ShipmentMapper shipmentMapper;
        private ShippingClient shippingClient;
        private ShippingService shippingService;
        private ShipmentAnomalyEventPublisher anomalyPublisher;
        private ShipmentLabelReconcileScheduler scheduler;

        @BeforeEach
        void setUp() {
            shipmentMapper = mock(ShipmentMapper.class);
            shippingClient = mock(ShippingClient.class);
            shippingService = mock(ShippingService.class);
            anomalyPublisher = mock(ShipmentAnomalyEventPublisher.class);
            // ⚠️ shippingClient 傳進 registry 而不是 scheduler：provider 呼叫已整段搬進 adapter，
            //    scheduler 不再自己打 PayUni（連帶消滅它自己那份 deriveLgsType）。
            //    下面的 verify(shippingClient) 仍然成立，因為 adapter 用的就是這一個 mock。
            scheduler = new ShipmentLabelReconcileScheduler(
                    shipmentMapper, shippingService, anomalyPublisher,
                    io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders.realRegistry(
                            shippingClient,
                            mock(io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper.class),
                            mock(io.github.will413028.hawkr.shipping.service.OnboardingService.class)));
            ReflectionTestUtils.setField(scheduler, "batchSize", 50);
            ReflectionTestUtils.setField(scheduler, "maxAgeDays", 7);
            ReflectionTestUtils.setField(scheduler, "requestedGraceMinutes", 5);
            ReflectionTestUtils.setField(scheduler, "cvsLabelStuckHours", 48);
        }

        @Test
        @DisplayName("claim 回 0 ⇒ 不打 PayUni 查詢")
        void claimLost_doesNotCallPayUni() {
            when(shipmentMapper.findLabelReconcileCandidates(any(), any(), any(), anyInt()))
                    .thenReturn(List.of(shipment(1L, "SH1")));
            when(shipmentMapper.claimLabelReconcile(anyLong(), anyString(), anyString(), any(), any()))
                    .thenReturn(0);

            scheduler.run();

            verify(shippingClient, never()).queryShipment(anyString(), any());
            verify(shippingService, never())
                    .processWebhookPrint(anyString(), anyString(), anyString(), anyString());
        }

        @Test
        @DisplayName("claim 回 1 ⇒ 照常打 PayUni 查詢")
        void claimWon_callsPayUni() {
            when(shipmentMapper.findLabelReconcileCandidates(any(), any(), any(), anyInt()))
                    .thenReturn(List.of(shipment(1L, "SH1")));
            when(shipmentMapper.claimLabelReconcile(anyLong(), anyString(), anyString(), any(), any()))
                    .thenReturn(1);
            when(shippingClient.queryShipment(anyString(), any())).thenReturn(failedQuery());

            scheduler.run();

            verify(shippingClient).queryShipment(anyString(), any());
        }

        /**
         * 租約長度的<b>上界</b>。只驗「是未來時間」對任何大值都成立，沒有鑑別力；
         * 這裡釘的是「租約 = 一個 tick，且必須 ≪ 候選窗口 7 天」。
         * 租約若被改成天級，補收延遲就跟著變成天級，而候選窗口不會替你擋住。
         */
        @Test
        @DisplayName("租約長度 = 10 分鐘且不超過一個 tick、且 ≪ 候選窗口 7 天")
        void leaseLengthIsBoundedByOneTickAndCandidateWindow() {
            when(shipmentMapper.findLabelReconcileCandidates(any(), any(), any(), anyInt()))
                    .thenReturn(List.of(shipment(1L, "SH1")));
            when(shipmentMapper.claimLabelReconcile(anyLong(), anyString(), anyString(), any(), any()))
                    .thenReturn(1);
            when(shippingClient.queryShipment(anyString(), any())).thenReturn(failedQuery());

            LocalDateTime before = DateTimeUtils.nowUtc();
            scheduler.run();

            ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
            verify(shipmentMapper)
                    .claimLabelReconcile(anyLong(), anyString(), anyString(), any(), lease.capture());
            long leaseSeconds = Duration.between(before, lease.getValue()).getSeconds();

            assertTrue(leaseSeconds > 0, "租約必須是未來時間（0 或負 ⇒ 等於沒有搶佔）");
            long oneTickSeconds = 600L;   // label.interval-ms 預設 600000
            long windowSeconds = 7L * 24 * 3600;  // label.max-age-days 預設 7
            assertTrue(leaseSeconds <= oneTickSeconds + 5,
                    "租約實測 " + leaseSeconds + " 秒 > 一個 tick(" + oneTickSeconds + " 秒)");
            assertTrue(leaseSeconds * 10 <= windowSeconds,
                    "租約必須 ≪ 候選窗口（≤ 1/10），否則窗口內重試次數不足，lease 會退化成永久旗標");
            assertTrue(ShipmentLabelReconcileScheduler.LEASE_MINUTES == 10L,
                    "租約長度常數應與 label.interval-ms 預設值（10 分）對齊，實際 "
                            + ShipmentLabelReconcileScheduler.LEASE_MINUTES + " 分");
        }

        /**
         * PayUni 查不到（{@code isSuccess()==false}）的回應。
         * 兩者都是 {@code record}（final）不能 mock，直接建真值。
         */
        private QueryShipmentResponse failedQuery() {
            PayUniApiResponse api = new PayUniApiResponse("FAIL", "not found", java.util.Map.of());
            return new QueryShipmentResponse(api, "ST1", null, null, "O1");
        }
    }
}

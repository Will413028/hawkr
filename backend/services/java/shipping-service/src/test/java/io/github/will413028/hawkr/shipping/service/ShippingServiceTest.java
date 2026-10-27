package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver;
import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.seller.grpc.BatchGetSellerMetaResponse;
import io.github.will413028.hawkr.seller.grpc.SellerMeta;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import io.github.will413028.hawkr.shipping.client.OrderGrpcClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentStatusLogMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders;
import io.github.will413028.hawkr.shipping.service.dto.ShipmentCreatedPayload;
import io.github.will413028.hawkr.shipping.service.dto.ShippingEventPayload;
import io.github.will413028.hawkr.shipping.provider.create.ShipmentStrategy;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.transaction.PlatformTransactionManager;

import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.time.LocalDate;
import java.util.Collections;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 單元測試：驗證 ShippingService 在發布 shipment.status.changed 事件時
 * 帶上 entity 的 buyerId（issue #626 ORDER_SHIPPED 通知所需）。
 */
class ShippingServiceTest {

    private io.micrometer.core.instrument.simple.SimpleMeterRegistry meterRegistry;
    private ShipmentMapper shipmentMapper;
    private ShipmentStatusLogMapper statusLogMapper;
    private OrderGrpcClient orderGrpcClient;
    private BuyerGrpcClient buyerGrpcClient;
    private MediaGrpcClient mediaGrpcClient;
    private SellerGrpcClient sellerGrpcClient;
    private ShippingClient shippingClient;
    private ShipmentStatusMapper statusMapper;
    private ShipmentLabelService labelService;
    private ShippingEventPublisher eventPublisher;
    private ShipmentAnomalyEventPublisher anomalyEventPublisher;
    private OnboardingService onboardingService;
    private HolidayService holidayService;
    private CvsPrintWaitingRegistry cvsPrintWaitingRegistry;
    private io.github.will413028.hawkr.common.providercall.ProviderCallExecutor providerCallExecutor;
    private PlatformTransactionManager transactionManager;

    private ShippingService shippingService;

    @BeforeEach
    void setUp() {
        shipmentMapper = mock(ShipmentMapper.class);
        statusLogMapper = mock(ShipmentStatusLogMapper.class);
        orderGrpcClient = mock(OrderGrpcClient.class);
        buyerGrpcClient = mock(BuyerGrpcClient.class);
        mediaGrpcClient = mock(MediaGrpcClient.class);
        sellerGrpcClient = mock(SellerGrpcClient.class);
        shippingClient = mock(ShippingClient.class);
        statusMapper = mock(ShipmentStatusMapper.class);
        labelService = mock(ShipmentLabelService.class);
        eventPublisher = mock(ShippingEventPublisher.class);
        anomalyEventPublisher = mock(ShipmentAnomalyEventPublisher.class);
        onboardingService = mock(OnboardingService.class);
        holidayService = mock(HolidayService.class);
        cvsPrintWaitingRegistry = mock(CvsPrintWaitingRegistry.class);
        providerCallExecutor = mock(io.github.will413028.hawkr.common.providercall.ProviderCallExecutor.class);
        transactionManager = mock(PlatformTransactionManager.class);

        meterRegistry = new io.micrometer.core.instrument.simple.SimpleMeterRegistry();

        shippingService = new ShippingService(
                shipmentMapper,
                statusLogMapper,
                orderGrpcClient,
                buyerGrpcClient,
                mediaGrpcClient,
                sellerGrpcClient,
                statusMapper,
                labelService,
                eventPublisher,
                anomalyEventPublisher,
                onboardingService,
                holidayService,
                cvsPrintWaitingRegistry,
                providerCallExecutor,
                transactionManager,
                TestFulfillmentProviders.realRegistry(shippingClient, statusMapper, onboardingService),
                new AllowedActionResolver(
                        TestFulfillmentProviders.realRegistry(shippingClient, statusMapper, onboardingService),
                        statusMapper),
                meterRegistry);
    }

    /**
     * 驗證 IN_TRANSIT 狀態變更 publish 的 payload 帶上 entity.buyerId。
     * <p>
     * Entry point: processWebhookStatusUpdate（由 MQ Consumer 呼叫，內部走 updateStatus
     * → publishShipmentStatusChanged）。
     */
    /**
     * 🔴 <b>「沒有 raw 值」與「raw 值看不懂」是兩件事</b>（2026-09-10 dev 實測抓到的回歸）。
     *
     * <p>接上 canonical ingress 之後，PayUni 的<b>建單結果回報</b>（只有 {@code TradeStatus}、
     * 沒有 {@code ShipStatus}）會流進未知貨態的處置分支。而 {@code updateProviderStatus}
     * 是<b>無條件覆寫</b> ⇒ 照寫下去會把 {@code provider_status} 與 {@code provider_message}
     * 一起清成 null，等於用「保存證據」的名義銷毀證據。
     *
     * <p>dev 逐字證據：{@code eventKey=HAWK2675237784:S0011789042138064014:STATUS:0}，
     * payload 只有 {@code TradeStatus}；shipping log
     * {@code "…（source=WEBHOOK, raw=null, shipmentNo=S726336190, persisted=1）"}。
     *
     * <p><b>本測試的鑑別力來自這一對</b>：空值不得寫、有值必須寫。
     * 只有前者的話，「整條路徑都不寫」也會全綠。
     */
    @Test
    void blankProviderStatusIsNotPersistedButUnknownValueIs() {
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(77L);
        entity.setShipmentNo("S777");
        entity.setOrderNo("O777");
        entity.setShipTradeNo("T777");
        entity.setStatus(ShippingStatus.UNPROCESSED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);

        // (a) 建單結果回報：沒有貨態欄位 ⇒ 一個字都不准寫
        shippingService.processWebhookStatusUpdate("T777", "", null, null);
        verify(shipmentMapper, never()).updateProviderStatus(any(), any(), any());

        // (b) 對照組：有值但映射不出來 ⇒ 必須保存 raw
        when(statusMapper.mapPayUniStatus("9999")).thenReturn(null);
        shippingService.processWebhookStatusUpdate("T777", "9999", "PayUni 新貨態", null);
        verify(shipmentMapper).updateProviderStatus(77L, "9999", "PayUni 新貨態");
    }

    /**
     * R2：未知貨態必須有<b>指標</b>，不能只有 WARN log。
     *
     * <p>Independent QA 的原話：「unknown status 被保存只是第一步；如果沒有任何監控，
     * 實務上仍是 silent failure」。log 沒有訂閱者，而這件事的意義是趨勢
     * （provider 換碼表／adapter 漏 map），不是單筆。
     *
     * <p>🔴 <b>兩個出口都要驗</b>：{@code not_a_status_event}（回調不帶貨態欄位）與
     * {@code persisted}（有值但映射不出來）是同一個函式的兩條未知路徑。只驗一條的話，
     * 另一條被拿掉時測試照樣綠——而那正是 invoice M3/M4/M14 三條告警十個月從未觸發的形狀。
     *
     * <p>🔴 <b>期望值硬寫字面值</b>：metric 名與 tag 值都不從 production 常數讀，
     * 否則改名時斷言會跟著改、測試驗不到任何東西。字面值與
     * {@code monitoring/prometheus/rules/business-alerts.yml} 的 {@code expr} 必須一致。
     */
    @Test
    void unknownProviderStatusIncrementsMetricOnBothExits() {
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(88L);
        entity.setShipmentNo("S888");
        entity.setOrderNo("O888");
        entity.setShipTradeNo("T888");
        entity.setStatus(ShippingStatus.UNPROCESSED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);

        // 🔴 預先展開：告警用 counter 在**任何事發生之前**就必須存在，否則第一筆
        //    未知貨態是「序列從不存在跳到 1」——increase() 不算那一跳，告警靜默。
        assertEquals(0.0, unknownCount(ShippingMethod.TAQBIN.value()),
                "建構子必須把 5 個配送方式的 counter 預先展開成 0");

        // 出口 ①：回調不帶貨態欄位 —— 正常流量，**不得**進告警用的那支
        shippingService.processWebhookStatusUpdate("T888", "", null, null);
        assertEquals(1.0, notStatusEventCount(),
                "回調不帶貨態欄位要有自己的計數（趨勢用）");
        assertEquals(0.0, unknownCount(ShippingMethod.TAQBIN.value()),
                "🔴 它是 PayUni 建單結果回報這種正常流量，混進告警指標會讓每次正常建單都告警");

        // 出口 ②：有值但映射不出來 —— 這才是告警要看的
        when(statusMapper.mapPayUniStatus("9999")).thenReturn(null);
        when(shipmentMapper.updateProviderStatus(anyLong(), any(), any())).thenReturn(1);
        shippingService.processWebhookStatusUpdate("T888", "9999", "PayUni 新貨態", null);
        assertEquals(1.0, unknownCount(ShippingMethod.TAQBIN.value()),
                "真的看不懂的貨態必須計數，否則告警沒有生產者");

        // 🔴 陰性：沒發生的配送方式維持 0（證明上面那個 1.0 不是「一律回 1」）
        assertEquals(0.0, unknownCount(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value()),
                "沒發生的產品線不得被計數");

        // 🔴 label 基數紀律：raw 狀態值不得成為 label（母體由第三方決定，無上界）
        meterRegistry.get("hawkr_shipping_provider_status_unknown").counters()
                .forEach(c -> c.getId().getTags().forEach(tag -> assertFalse(
                        "9999".equals(tag.getValue()),
                        "raw 貨態值被當成 label（tag=" + tag.getKey() + "）——基數無上界")));
    }

    /**
     * R2 第三個缺陷：PayUni 的 {@code "-"} 哨兵不是「看不懂的貨態」，是「還沒有貨態」。
     *
     * <p>dev 實測（2026-09-11）：回查路徑對尚未進入物流流程的單子拿回 {@code "-"}，
     * 25 次／2 小時全部是它 ⇒ ① 告警被正常流量持續觸發
     * ② 無條件覆寫把原本有意義的貨態蓋成 {@code "-"}（10 張 status=91 的單中招）。
     *
     * <p>🔴 本測試的鑑別力來自這一對：哨兵不得寫、真未知必須寫。
     * 只有前者的話，「整條路徑都不寫」也會全綠。
     */
    @Test
    void payUniDashSentinelIsNotAnUnknownStatus() {
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(99L);
        entity.setShipmentNo("S999");
        entity.setOrderNo("O999");
        entity.setShipTradeNo("T999");
        entity.setStatus(ShippingStatus.UNPROCESSED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);

        // (a) 哨兵：一個字都不准寫，也不准進告警指標
        shippingService.processWebhookStatusUpdate("T999", "-", "-", null);
        verify(shipmentMapper, never()).updateProviderStatus(any(), any(), any());
        assertEquals(0.0, unknownCount(ShippingMethod.TAQBIN.value()),
                "🔴 `-` 是「尚無貨態」不是「看不懂的貨態」——計進去會讓告警被正常流量淹沒");

        // (b) 對照組：真的看不懂的值必須寫、必須計數
        when(statusMapper.mapPayUniStatus("9999")).thenReturn(null);
        when(shipmentMapper.updateProviderStatus(anyLong(), any(), any())).thenReturn(1);
        shippingService.processWebhookStatusUpdate("T999", "9999", "新貨態", null);
        verify(shipmentMapper).updateProviderStatus(99L, "9999", "新貨態");
        assertEquals(1.0, unknownCount(ShippingMethod.TAQBIN.value()));
    }

    private double unknownCount(String shippingMethod) {
        return meterRegistry.get("hawkr_shipping_provider_status_unknown")
                .tag("shipping_method", shippingMethod).counter().count();
    }

    private double notStatusEventCount() {
        return meterRegistry.get("hawkr_shipping_provider_not_status_event")
                .tag("shipping_method", ShippingMethod.TAQBIN.value())
                .tag("source", "WEBHOOK").counter().count();
    }

    @Test
    void inTransit_publishCarriesBuyerId() {
        // Arrange — 模擬從 PayUni Webhook 收到 IN_TRANSIT (31) 狀態，當前 entity 在 TCAT_CALLED
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(999L);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.TCAT_CALLED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("31")).thenReturn(ShippingStatus.IN_TRANSIT);
        when(statusMapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.IN_TRANSIT))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        // Act — 走 webhook 公開入口，內部觸發 updateStatus 中央 publish
        shippingService.processWebhookStatusUpdate(
                "T123", "31", "運送中", "2026-04-30 10:00:00");

        // Assert — payload 帶 buyerId、status 為 IN_TRANSIT
        verify(eventPublisher).publishShipmentStatusChanged(captor.capture());
        ShippingEventPayload payload = captor.getValue();
        assertEquals(999L, payload.buyerId());
        assertEquals(ShippingStatus.IN_TRANSIT.value(), payload.status());
    }

    @Test
    void inTransit_notifyBuyerAccountId_resolvesToPrincipalAccountId() {
        long buyerId = 999L;
        long resolvedAccountId = 5001L; // 刻意 != buyerId

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(buyerId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.TCAT_CALLED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("31")).thenReturn(ShippingStatus.IN_TRANSIT);
        when(statusMapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.IN_TRANSIT))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedAccountId)));

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        shippingService.processWebhookStatusUpdate("T123", "31", "運送中", "2026-04-30 10:00:00");

        verify(eventPublisher).publishShipmentStatusChanged(captor.capture());
        assertEquals(resolvedAccountId, captor.getValue().notifyBuyerAccountId());
    }

    /**
     * 驗證 resolver（BuyerGrpcClient）失敗時 fail-closed：notifyBuyerAccountId 落到 0L，
     * 不拋例外、也不誤用原始 buyer_id（0 是 notification-service「查無此人」的安全跳過訊號）。
     */
    @Test
    void inTransit_notifyBuyerAccountId_resolverFailure_fallsBackToZero() {
        long buyerId = 999L;

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(buyerId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.TCAT_CALLED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("31")).thenReturn(ShippingStatus.IN_TRANSIT);
        when(statusMapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.IN_TRANSIT))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        // Act — 不應拋例外
        shippingService.processWebhookStatusUpdate("T123", "31", "運送中", "2026-04-30 10:00:00");

        verify(eventPublisher).publishShipmentStatusChanged(captor.capture());
        assertEquals(0L, captor.getValue().notifyBuyerAccountId());
    }

    /**
     * 驗證 PICKED_UP（shipment.delivered，template id=15）publish 的 payload 帶
     * 「已解析」買家 notifyBuyerAccountId，而非 entity.buyerId(PK) 原始值。
     */
    @Test
    void delivered_notifyBuyerAccountId_resolvesToPrincipalAccountId() {
        long buyerId = 999L;
        long resolvedAccountId = 5002L; // 刻意 != buyerId

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(buyerId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.IN_TRANSIT.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("11")).thenReturn(ShippingStatus.PICKED_UP);
        when(statusMapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.PICKED_UP))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(PrincipalResolution.allResolved(Map.of(buyerId, resolvedAccountId)));

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        shippingService.processWebhookStatusUpdate("T123", "11", "已取貨", "2026-04-30 10:00:00");

        verify(eventPublisher).publishShipmentDelivered(captor.capture());
        assertEquals(resolvedAccountId, captor.getValue().notifyBuyerAccountId());
    }

    /**
     * 驗證 resolver（BuyerGrpcClient）失敗時 fail-closed：PICKED_UP 分支同樣落到 0L，
     * 不拋例外。
     */
    @Test
    void delivered_notifyBuyerAccountId_resolverFailure_fallsBackToZero() {
        long buyerId = 999L;

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(buyerId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.IN_TRANSIT.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("11")).thenReturn(ShippingStatus.PICKED_UP);
        when(statusMapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.PICKED_UP))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        // Act — 不應拋例外
        shippingService.processWebhookStatusUpdate("T123", "11", "已取貨", "2026-04-30 10:00:00");

        verify(eventPublisher).publishShipmentDelivered(captor.capture());
        assertEquals(0L, captor.getValue().notifyBuyerAccountId());
    }

    @Test
    void delivered_publishesResolvedAccountId_notRawStoreId() {
        long storeId = 9999L;
        long resolvedAccountId = 1001L; // 刻意 != storeId

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(999L);
        entity.setStoreId(storeId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.IN_TRANSIT.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("11")).thenReturn(ShippingStatus.PICKED_UP);
        when(statusMapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.PICKED_UP))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(storeId, SellerMeta.newBuilder().setAccountId(resolvedAccountId).build())
                        .build());

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        shippingService.processWebhookStatusUpdate("T123", "11", "已取貨", "2026-04-30 10:00:00");

        verify(eventPublisher).publishShipmentDelivered(captor.capture());
        assertEquals(resolvedAccountId, captor.getValue().sellerAccountId());
        // issue #2707 Phase A：raw storeId 必須無條件透傳，不受 sellerAccountId 解析結果影響。
        assertEquals(storeId, captor.getValue().storeId());
    }

    /**
     * #2288：解除 PDF 引用之後，shipments 上的 PDF 欄位必須一起清掉。
     * <p>
     * 留著就是指向不存在物件的死指標——media-service 的 GC 會在 7 天後刪掉 R2 上的
     * PDF（既定政策），而 buildDetailView 在配送完成後仍會回傳 labelKey，Gateway
     * 照樣組出 URL，前端拿到 404。
     */
    @Test
    void delivered_unlinkSucceeded_clearsLabelColumns() {
        ShipmentEntity entity = deliveredEntityWithLabel();
        when(labelService.unlinkLabel(1L, 777L)).thenReturn(true);

        shippingService.processWebhookStatusUpdate("T123", "11", "已取貨", "2026-04-30 10:00:00");

        verify(labelService).unlinkLabel(1L, 777L);
        verify(shipmentMapper).clearLabelAfterDelivery(1L);
        assertNull(entity.getLabelObjectKey(), "記憶體中的 entity 也要清乾淨，否則同一交易後續讀到的還是舊值");
        assertNull(entity.getLabelAssetId());
        assertNull(entity.getLabelExpireAt());
    }

    /**
     * #2288 的反向：解除失敗就**不能**清欄位。
     * <p>
     * 清了會讓引用留在 media-service 裡，那個 R2 物件就永遠不會被 GC 回收（洩漏），
     * 而且我們再也找不到它的 key。兩者必須同進退。
     */
    @Test
    void delivered_unlinkFailed_keepsLabelColumns() {
        ShipmentEntity entity = deliveredEntityWithLabel();
        when(labelService.unlinkLabel(1L, 777L)).thenReturn(false);

        shippingService.processWebhookStatusUpdate("T123", "11", "已取貨", "2026-04-30 10:00:00");

        verify(labelService).unlinkLabel(1L, 777L);
        verify(shipmentMapper, never()).clearLabelAfterDelivery(anyLong());
        assertEquals("public/shipping/20260805/x/S100.pdf", entity.getLabelObjectKey());
        assertEquals(777L, entity.getLabelAssetId());
    }

    /** 兩個 #2288 案例共用的 fixture：一筆帶 PDF、即將轉入 PICKED_UP 的出貨單。 */
    private ShipmentEntity deliveredEntityWithLabel() {
        long storeId = 9999L;
        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(999L);
        entity.setStoreId(storeId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.IN_TRANSIT.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);
        entity.setLabelObjectKey("public/shipping/20260805/x/S100.pdf");
        entity.setLabelAssetId(777L);
        entity.setLabelExpireAt(java.time.LocalDateTime.of(2026, 8, 6, 0, 0));

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("11")).thenReturn(ShippingStatus.PICKED_UP);
        when(statusMapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.PICKED_UP))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(storeId, SellerMeta.newBuilder().setAccountId(1001L).build())
                        .build());
        return entity;
    }

    // ==================== 賣家身分未解析時的降級（issue #2789 階段 B2） ====================
    //
    // updateStatus 底下分岔成三條 publish，而「解析不到時該不該回吐 store_id」逐條不同。
    // 下面四支測試把那張對照表釘成可執行的斷言 —— 只驗其中一條會讓「兩個入口」退化成
    // 「一個入口 + 一個沒人走的分支」，所以四條分支各要一支。
    //
    //   publish 目標                       | baseline 的賣家向 template          | 未解析時
    //   ----------------------------------|-----------------------------------|--------
    //   shipment.created                   | id=2，push_mode=visible「買家已下單」 | 保守（echo）
    //   shipment.delivered                 | shipment.delivered.seller           | 保守（echo）
    //   shipment.status.changed            | 無（baseline 只有買家向 id=3）        | 0
    //   return.shipment.status.changed     | 無（baseline 完全沒有 template）      | 0

    /**
     * <b>語意已翻轉（issue #2789 階段 B2）</b>：原名
     * {@code statusChanged_resolverFailure_fallsBackToRawStoreId}，其註解把 fail-open
     * 回吐 store_id 寫成刻意設計。現在斷言相反的事——{@code shipment.status.changed} 帶 {@code 0L}。
     *
     * <p><b>為什麼翻轉</b>：baseline 對 {@code shipment.status.changed} <b>只有買家向 template</b>
     * （id=3，{@code target_user_field=buyer_id}，{@code payload_filter status=31}），
     * <b>沒有任何模板讀 {@code seller_account_id}</b>。回吐 store_id 的投遞收益因此是零，
     * 代價卻是讓另一個身分空間的合法非零值繼續在事件裡流動、且完全無聲。
     *
     * <p>不變的部分（本測試同時仍在守）：resolver 失敗<b>不得</b>拋例外中斷 statusUpdate
     * 這條半關鍵路徑 —— 事件照發、只有身分欄位降級。
     */
    @Test
    void statusChanged_resolverFailure_publishesZero_notRawStoreId() {
        long storeId = 9999L;

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(999L);
        entity.setStoreId(storeId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.TCAT_CALLED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("31")).thenReturn(ShippingStatus.IN_TRANSIT);
        when(statusMapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.IN_TRANSIT))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenThrow(new RuntimeException("seller-service unavailable"));

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        // Act — 不應拋例外
        shippingService.processWebhookStatusUpdate("T123", "31", "運送中", "2026-04-30 10:00:00");

        verify(eventPublisher).publishShipmentStatusChanged(captor.capture());
        assertEquals(0L, captor.getValue().sellerAccountId());
        // issue #2707 Phase A：raw storeId 不受 sellerAccountId 解析失敗降級影響，仍是原始值。
        assertEquals(storeId, captor.getValue().storeId());
    }

    /**
     * 退貨物流單（{@code isReturn=true}）走 {@code return.shipment.status.changed}，
     * baseline <b>完全沒有 template</b>（golden 也查無此 event_type）⇒ 未解析時帶 {@code 0L}。
     *
     * <p>它與上一支的差別不在狀態碼而在 {@code isReturn} 旗標：分流條件必須與 publish
     * 分支逐字同構，這支就是釘住 {@code isReturn} 那一段。
     */
    @Test
    void returnShipment_resolverFailure_publishesZero_notRawStoreId() {
        long storeId = 9999L;

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(999L);
        entity.setStoreId(storeId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.TCAT_CALLED.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(true);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("31")).thenReturn(ShippingStatus.IN_TRANSIT);
        when(statusMapper.isValidTransition(ShippingStatus.TCAT_CALLED, ShippingStatus.IN_TRANSIT))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenThrow(new RuntimeException("seller-service unavailable"));

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        shippingService.processWebhookStatusUpdate("T123", "31", "運送中", "2026-04-30 10:00:00");

        verify(eventPublisher).publishReturnShipmentStatusChanged(captor.capture());
        assertEquals(0L, captor.getValue().sellerAccountId());
        // issue #2707 Phase A：raw storeId 不受 sellerAccountId 解析失敗降級影響，仍是原始值。
        assertEquals(storeId, captor.getValue().storeId());
    }

    /**
     * <b>保守版</b>：{@code PICKED_UP} 走 {@code shipment.delivered}，而 baseline 有賣家向的
     * {@code shipment.delivered.seller}（golden 亦列 {@code seller_account_id}）
     * ⇒ 未解析時<b>維持 echo-back</b>，寧可「可能送錯對象」也不要保證跳過賣家的送達通知。
     *
     * <p>⚠️ 這支釘住的是一個<b>刻意保留的 fail-open</b>，不是理想終局：帶出去的值可能不是
     * 解析結果。要拿掉這個取捨，正解是讓 seller 軸的 {@code batchGetSellerMeta} 也回報
     * unresolved，而不是把它改成靜默 echo-back（那就退回本 issue 要修的原狀）。
     */
    @Test
    void delivered_resolverFailure_keepsRawStoreId_conservativeBecauseSellerTemplateExists() {
        long storeId = 9999L;

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setBuyerId(999L);
        entity.setStoreId(storeId);
        entity.setShipTradeNo("T123");
        entity.setStatus(ShippingStatus.IN_TRANSIT.value());
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());
        entity.setIsReturn(false);

        when(shipmentMapper.selectOne(any(LambdaQueryWrapper.class))).thenReturn(entity);
        when(statusMapper.mapPayUniStatus("11")).thenReturn(ShippingStatus.PICKED_UP);
        when(statusMapper.isValidTransition(ShippingStatus.IN_TRANSIT, ShippingStatus.PICKED_UP))
                .thenReturn(true);
        when(shipmentMapper.updateShipmentStatus(any(), any(), any(), any(), any(), any()))
                .thenReturn(1);
        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenThrow(new RuntimeException("seller-service unavailable"));

        ArgumentCaptor<ShippingEventPayload> captor = ArgumentCaptor.forClass(ShippingEventPayload.class);

        shippingService.processWebhookStatusUpdate("T123", "11", "已取貨", "2026-04-30 10:00:00");

        verify(eventPublisher).publishShipmentDelivered(captor.capture());
        assertEquals(storeId, captor.getValue().sellerAccountId());
    }

    /**
     * <b>保守版</b>：{@code shipment.created} 是賣家<b>唯一</b>的新訂單推播
     * （template id=2、{@code push_mode=visible}「買家已下單」），掉了會連鎖到出貨逾時、
     * 系統取消、退款 ⇒ 未解析時維持 echo-back。理由與上一支相同、代價更高。
     */
    @Test
    void shipmentCreated_resolverFailure_keepsRawStoreId_conservative() throws Exception {
        long storeId = 9999L;

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setStoreId(storeId);
        entity.setShipTradeNo("T123");
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());

        OrderShippingInfo orderInfo = OrderShippingInfo.newBuilder()
                .setStoreId(String.valueOf(storeId))
                .setBuyerId("999")
                .setTotalActualAmount("100.00")
                .build();

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenThrow(new RuntimeException("seller-service unavailable"));

        invokePublishShipmentCreatedEvent(shippingService, entity, orderInfo);

        ArgumentCaptor<ShipmentCreatedPayload> captor = ArgumentCaptor.forClass(ShipmentCreatedPayload.class);
        verify(eventPublisher).publishShipmentCreated(captor.capture());
        assertEquals(String.valueOf(storeId), captor.getValue().sellerAccountId());
    }

    /**
     * 驗證 shipment.created（賣家「新訂單」推播）publish 的 payload 帶「已解析」賣家
     * accountId，而非 entity.storeId 原始值 — Task 5 修 updateStatus 時漏掉的同源
     * sibling method（publishShipmentCreatedEvent 是 private，僅能透過
     * createShipmentFromOrder 觸及；直接 reflection 呼叫，比照本檔案 parseFlexibleDate
     * 既有的 private-method 測試慣例，避免為此而搭建 strategyMap/onboarding 整條建單鏈路）。
     */
    @Test
    void publishShipmentCreatedEvent_publishesResolvedAccountId_notRawStoreId() throws Exception {
        long storeId = 9999L;
        long resolvedAccountId = 1001L; // 刻意 != storeId

        ShipmentEntity entity = new ShipmentEntity();
        entity.setId(1L);
        entity.setShipmentNo("S100");
        entity.setOrderNo("O200");
        entity.setStoreId(storeId);
        entity.setShipTradeNo("T123");
        entity.setShippingMethod(ShippingMethod.TAQBIN.value());

        OrderShippingInfo orderInfo = OrderShippingInfo.newBuilder()
                .setStoreId(String.valueOf(storeId))
                .setBuyerId("999")
                .setTotalActualAmount("100.00")
                .build();

        when(sellerGrpcClient.batchGetSellerMeta(List.of(storeId)))
                .thenReturn(BatchGetSellerMetaResponse.newBuilder()
                        .putSellers(storeId, SellerMeta.newBuilder().setAccountId(resolvedAccountId).build())
                        .build());

        invokePublishShipmentCreatedEvent(shippingService, entity, orderInfo);

        ArgumentCaptor<ShipmentCreatedPayload> captor = ArgumentCaptor.forClass(ShipmentCreatedPayload.class);
        verify(eventPublisher).publishShipmentCreated(captor.capture());
        assertEquals(String.valueOf(resolvedAccountId), captor.getValue().sellerAccountId());
        // issue #2707 Phase A：raw storeId 必須無條件透傳，不受 sellerAccountId 解析結果影響。
        assertEquals(String.valueOf(storeId), captor.getValue().storeId());

        // issue #2174：買家側刻意帶裸 buyer_id（本事件無買家向 template），
        // 且 wire key 必須是 buyer_id，不能是會誤導成「已解析登入身分」的 buyer_account_id。
        assertEquals("999", captor.getValue().buyerId());
        com.fasterxml.jackson.databind.JsonNode wire =
                new com.fasterxml.jackson.databind.ObjectMapper().valueToTree(captor.getValue());
        assertTrue(wire.has("buyer_id"), "shipment.created 必須發 buyer_id");
        assertFalse(wire.has("buyer_account_id"), "shipment.created 不得再發 buyer_account_id");
    }

    private static void invokePublishShipmentCreatedEvent(ShippingService service, ShipmentEntity entity,
                                                           OrderShippingInfo orderInfo)
            throws NoSuchMethodException, IllegalAccessException, InvocationTargetException {
        Method m = ShippingService.class.getDeclaredMethod(
                "publishShipmentCreatedEvent", ShipmentEntity.class, OrderShippingInfo.class);
        m.setAccessible(true);
        m.invoke(service, entity, orderInfo);
    }

    // ==================== #1491 shipDate/deliveryDate 雙格式解析 ====================
    //
    // parseFlexibleDate / toPayUniDateFormat 為 private helper（無外部依賴的純函式），
    // 用 reflection 直接測試三態，避免為了觸發它們而搭建 generateHomeLabelPdf 整條鏈路的 mock。

    private static Object invokeParseFlexibleDate(ShippingService service, String rawDate, String fieldLabel)
            throws NoSuchMethodException, IllegalAccessException, InvocationTargetException {
        Method m = ShippingService.class.getDeclaredMethod("parseFlexibleDate", String.class, String.class);
        m.setAccessible(true);
        return m.invoke(service, rawDate, fieldLabel);
    }

    private static String invokeToPayUniDateFormat(ShippingService service, LocalDate date)
            throws NoSuchMethodException, IllegalAccessException, InvocationTargetException {
        Method m = ShippingService.class.getDeclaredMethod("toPayUniDateFormat", LocalDate.class);
        m.setAccessible(true);
        return (String) m.invoke(service, date);
    }

    @Test
    void parseFlexibleDate_acceptsCompactYyyyMMdd() throws Exception {
        LocalDate parsed = (LocalDate) invokeParseFlexibleDate(shippingService, "20260815", "出貨日期");
        assertEquals(LocalDate.of(2026, 8, 15), parsed);
        assertEquals("20260815", invokeToPayUniDateFormat(shippingService, parsed));
    }

    @Test
    void parseFlexibleDate_acceptsIsoYyyyDashMmDashDd() throws Exception {
        LocalDate parsed = (LocalDate) invokeParseFlexibleDate(shippingService, "2026-08-15", "出貨日期");
        assertEquals(LocalDate.of(2026, 8, 15), parsed);
        // 兩種輸入格式 normalize 後必須得到相同的 PayUni 送出格式
        assertEquals("20260815", invokeToPayUniDateFormat(shippingService, parsed));
    }

    @Test
    void parseFlexibleDate_rejectsUnsupportedFormat_asBusinessCodeNotRawParseException() {
        InvocationTargetException ex = assertThrows(InvocationTargetException.class,
                () -> invokeParseFlexibleDate(shippingService, "2026/08/15", "出貨日期"));
        assertEquals(ShippingBusinessException.class, ex.getCause().getClass());
        ShippingBusinessException sbe = (ShippingBusinessException) ex.getCause();
        assertEquals(GrpcResponseCode.SHIPMENT_DATE_INVALID_FORMAT.code(), sbe.code());
    }

    @Test
    void parseFlexibleDate_rejectsBlank_asBusinessCodeNotNpe() {
        InvocationTargetException ex = assertThrows(InvocationTargetException.class,
                () -> invokeParseFlexibleDate(shippingService, "", "出貨日期"));
        assertEquals(ShippingBusinessException.class, ex.getCause().getClass());
        ShippingBusinessException sbe = (ShippingBusinessException) ex.getCause();
        assertEquals(GrpcResponseCode.SHIPMENT_DATE_INVALID_FORMAT.code(), sbe.code());
    }
}

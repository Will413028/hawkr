package io.github.will413028.hawkr.shipping.provider;

import io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver;
import io.github.will413028.hawkr.common.providercall.ProviderCallExecutor;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.order.grpc.GetOrderForShippingResponse;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.QueryShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.response.QueryShipmentResponse;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import io.github.will413028.hawkr.shipping.client.OrderGrpcClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.mapper.OnboardingLogMapper;
import io.github.will413028.hawkr.shipping.mapper.SellerShipmentSettingMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentStatusLogMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability;
import io.github.will413028.hawkr.shipping.service.CvsPrintWaitingRegistry;
import io.github.will413028.hawkr.shipping.service.HolidayService;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.ShipmentLabelService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import io.github.will413028.hawkr.shipping.service.ShippingFeeService;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import io.github.will413028.hawkr.shipping.provider.create.ShipmentStrategy;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.transaction.PlatformTransactionManager;

import java.util.Collections;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 65 判準 1 的行為面守衛：**每一處被消滅的 {@code shipping_method} 字面分支，
 * 換上的 capability 真的在同一個位置做同一件事。**
 *
 * <h2>🔴 這支測試要抓的是什麼</h2>
 * ratchet gate（{@code scripts/check-fulfillment-provider-branching.py}）只數字面值，
 * 它的檔頭自己寫明「把 if 包成 helper 也會讓計數下降，而分支原封不動」。
 * 也就是說 <b>gate 綠燈證明不了行為正確</b>。本測試補的正是那一半：
 * 每個判斷點各驗<b>陽性與陰性兩面</b>（哪個 provider 走進去、哪些 provider 被擋在外），
 * 而 capability 的值來自<b>正牌 adapter</b>（{@link TestFulfillmentProviders}），
 * 所以改壞 adapter 的任一格常數都會讓對應斷言轉紅。
 *
 * <p>⚠️ 陰性面（「不該發生的沒有發生」）才是這些守衛的實質內容——
 * 「超商才要確認出貨」的價值全在「宅配不能確認出貨」那一半。
 */
@DisplayName("provider capability 取代 shipping_method 字面分支後的行為")
class FulfillmentCapabilityWiringTest {

    private ShipmentMapper shipmentMapper;
    private OrderGrpcClient orderGrpcClient;
    private ShippingClient shippingClient;
    private ShipmentStatusMapper statusMapper;
    private OnboardingService onboardingService;
    private ShipmentAnomalyEventPublisher anomalyEventPublisher;
    private ShippingService shippingService;

    @BeforeEach
    void setUp() {
        shipmentMapper = mock(ShipmentMapper.class);
        orderGrpcClient = mock(OrderGrpcClient.class);
        shippingClient = mock(ShippingClient.class);
        statusMapper = mock(ShipmentStatusMapper.class);
        onboardingService = mock(OnboardingService.class);
        anomalyEventPublisher = mock(ShipmentAnomalyEventPublisher.class);

        shippingService = new ShippingService(
                shipmentMapper,
                mock(ShipmentStatusLogMapper.class),
                orderGrpcClient,
                mock(BuyerGrpcClient.class),
                mock(MediaGrpcClient.class),
                mock(SellerGrpcClient.class),
                statusMapper,
                mock(ShipmentLabelService.class),
                mock(ShippingEventPublisher.class),
                anomalyEventPublisher,
                onboardingService,
                mock(HolidayService.class),
                mock(CvsPrintWaitingRegistry.class),
                mock(ProviderCallExecutor.class),
                mock(PlatformTransactionManager.class),
                TestFulfillmentProviders.realRegistry(shippingClient, statusMapper, onboardingService),
                new AllowedActionResolver(
                        TestFulfillmentProviders.realRegistry(shippingClient, statusMapper, onboardingService),
                        statusMapper),
                new io.micrometer.core.instrument.simple.SimpleMeterRegistry());
    }

    private static ShipmentEntity shipment(ShippingMethod method) {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setShipmentNo("SH1");
        e.setOrderNo("O1");
        e.setStoreId(2685914682L);
        e.setBuyerId(2623900520L);
        e.setMerId("HAWK2685914682");
        e.setShipTradeNo("ST1");
        e.setShippingMethod(method.value());
        e.setStatus(ShippingStatus.UNPROCESSED.value());
        return e;
    }

    // ==================================================================
    // ① CreateCapability.requiresOnboarding() 取代 `!UBER_INSTANT → 必須已建店`
    // ==================================================================

    /**
     * 兩個 assertion 是<b>同一個判斷點的兩面</b>，分開看都證明不了什麼：
     * <ul>
     *   <li>Uber：走到「查無 strategy」才失敗 ⇒ 建店閘門<b>放行了</b></li>
     *   <li>黑貓：停在建店閘門 ⇒ 閘門<b>還在</b>（沒有被順手拆掉）</li>
     * </ul>
     * 只留前者，把閘門整個刪掉也會綠；只留後者，把 {@code requiresOnboarding()} 全改成 true 也會綠。
     */
    @Nested
    @DisplayName("① 建單前置：這個 provider 需不需要先建店")
    class RequiresOnboarding {

        @BeforeEach
        void noExistingShipment() {
            when(shipmentMapper.selectOne(any())).thenReturn(null);
        }

        private void orderWithMethod(ShippingMethod method) {
            OrderShippingInfo info = OrderShippingInfo.newBuilder()
                    .setStoreId("2685914682")
                    .setBuyerId("2623900520")
                    .setTotalActualAmount("500")
                    .setShippingMethod(method.value())
                    .setMerId("HAWK2685914682")
                    .build();
            when(orderGrpcClient.getOrderForShipping(anyString()))
                    .thenReturn(GetOrderForShippingResponse.newBuilder().setData(info).build());
            when(onboardingService.findActiveSetting(eq(2685914682L), anyString())).thenReturn(null);
        }

        @Test
        @DisplayName("Uber：沒有建店設定也放行（requiresOnboarding()=false）")
        void uberSkipsOnboardingGate() {
            orderWithMethod(ShippingMethod.UBER_INSTANT);

            // 🔴 斷言改寫（2026-09-10 BLOCKER 5）。
            //    舊版斷言「會收到 SHIPMENT_UNSUPPORTED_METHOD」，並註解「走到查無 strategy＝已越過建店閘門」。
            //    那個訊號來自**測試自己餵的空 strategyMap**（`Collections.emptyList()`），
            //    不是產線行為——建單改走 adapter 的 CreateCapability 之後，Uber 的 adapter
            //    create() 是 present 的，於是它往下走進 mock 的 strategy 而拿到 null。
            //    要驗的事情本身沒變：**Uber 不該被建店閘門擋下**，所以直接斷言那件事。
            Throwable thrown = org.assertj.core.api.Assertions.catchThrowable(
                    () -> shippingService.createShipmentFromOrder("O1"));

            assertThat(thrown).as("mock 的 UberShipmentStrategy 回 null，往下走一定會炸——重點是炸在哪").isNotNull();
            if (thrown instanceof ShippingBusinessException se) {
                assertThat(se.code())
                        .as("Uber 的 requiresOnboarding()=false，不得被建店閘門擋下")
                        .isNotEqualTo(GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED.code());
            }
        }

        @Test
        @DisplayName("黑貓：沒有建店設定就擋下（requiresOnboarding()=true）")
        void taqbinBlockedWithoutOnboarding() {
            orderWithMethod(ShippingMethod.TAQBIN);

            assertThatThrownBy(() -> shippingService.createShipmentFromOrder("O1"))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .isEqualTo(GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED.code()));
        }

        @Test
        @DisplayName("超商：沒有建店設定就擋下（requiresOnboarding()=true）")
        void cvsBlockedWithoutOnboarding() {
            orderWithMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP);

            assertThatThrownBy(() -> shippingService.createShipmentFromOrder("O1"))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .isEqualTo(GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED.code()));
        }
    }

    // ==================================================================
    // ② DispatchCapability.requiresSellerConfirmation() 取代 `!SEVEN_ELEVEN → 硬擋`
    // ==================================================================

    @Nested
    @DisplayName("② 確認出貨：這個 provider 的流程裡有沒有賣家確認這一步")
    class RequiresSellerConfirmation {

        @Test
        @DisplayName("超商：通過 provider 守衛，停在下一道（尚未列印）")
        void cvsPassesProviderGuard() {
            ShipmentEntity e = shipment(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP);
            e.setLabelPrintStatus(LabelPrintStatus.NONE.value());
            when(shipmentMapper.selectOne(any())).thenReturn(e);

            // 🔴 拒絕碼必須就是契約給前端的 disabledReasonCode（9527），不是 400 參數錯誤。
            //    改動前這三支斷言的是 IllegalArgumentException ＋ 中文訊息 ——
            //    而 GrpcErrorHandler 把它翻成 INVALID_PARAM(400)，與契約的 9527 是兩個碼、
            //    兩條 log event，那正是 BLOCKER 3 要消滅的「兩套規則」殘留形態。
            assertThatThrownBy(() ->
                    shippingService.confirmCvsShipment(2685914682L, "O1", List.of()))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .isEqualTo(GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED.code()));
        }

        @Test
        @DisplayName("黑貓：被 provider 守衛擋下（dispatch capability 為 empty ⇒ false）")
        void taqbinRejected() {
            ShipmentEntity e = shipment(ShippingMethod.TAQBIN);
            e.setLabelPrintStatus(LabelPrintStatus.SUCCEEDED.value());
            when(shipmentMapper.selectOne(any())).thenReturn(e);

            assertThatThrownBy(() ->
                    shippingService.confirmCvsShipment(2685914682L, "O1", List.of()))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .as("這條線上不存在「賣家確認出貨」⇒ 9528，與契約 stateGate 給的碼相同")
                            .isEqualTo(GrpcResponseCode.SHIPMENT_PROVIDER_MISMATCH.code()));
        }

        @Test
        @DisplayName("Uber：被 provider 守衛擋下")
        void uberRejected() {
            ShipmentEntity e = shipment(ShippingMethod.UBER_INSTANT);
            e.setLabelPrintStatus(LabelPrintStatus.SUCCEEDED.value());
            when(shipmentMapper.selectOne(any())).thenReturn(e);

            assertThatThrownBy(() ->
                    shippingService.confirmCvsShipment(2685914682L, "O1", List.of()))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .as("這條線上不存在「賣家確認出貨」⇒ 9528，與契約 stateGate 給的碼相同")
                            .isEqualTo(GrpcResponseCode.SHIPMENT_PROVIDER_MISMATCH.code()));
        }
    }

    // ==================================================================
    // ③ TrackCapability 取代 `UBER_INSTANT 早退` ＋ `deriveLgsType`
    // ==================================================================

    @Nested
    @DisplayName("③ 主動回查：有沒有回查能力，以及送出去的 LgsType")
    class TrackCapabilityWiring {

        private void payUniReturnsFailure() {
            when(shippingClient.queryShipment(anyString(), any()))
                    .thenReturn(new QueryShipmentResponse(
                            new PayUniApiResponse("FAIL", "查無此件", null),
                            "ST1", null, null, "O1"));
        }

        private QueryShipmentRequest capturedRequest() {
            ArgumentCaptor<QueryShipmentRequest> captor =
                    ArgumentCaptor.forClass(QueryShipmentRequest.class);
            verify(shippingClient).queryShipment(anyString(), captor.capture());
            return captor.getValue();
        }

        /**
         * capability 這一端：三條線都有回查能力，<b>但走的不是同一支 provider API</b>。
         *
         * <p>⚠️ 本條原本斷言「Uber 無回查能力」。W2-1 把 {@code UberDirectAdapter.track()}
         * 換成 E4 Get Delivery 的真實實作之後那句話不再成立
         * ——而<b>它跟下面那條合起來要證明的事沒有變</b>：
         * 「每條線都查得到狀態」且「Uber 那條<b>不經過 PayUni</b>」。
         *
         * <p>🔴 兩條缺一不可，各自會被某一類變異放過：
         * <ul>
         *   <li>只留這條：把 Uber 的 {@code track()} 實作改成去打 PayUni，它照樣綠
         *       （它只看 {@code isPresent()}）</li>
         *   <li>只留下面那條：把 Uber 的 {@code track()} 改回 {@code Optional.empty()}，
         *       它照樣綠——因為 empty 本來就不打 PayUni</li>
         * </ul>
         */
        @Test
        @DisplayName("capability 形狀：三條線都有回查能力（Uber 走 E4，PayUni 兩條走 query）")
        void trackCapabilityShape() {
            FulfillmentProviderRegistry registry = TestFulfillmentProviders.realRegistry(
                    mock(ShippingClient.class), mock(ShipmentStatusMapper.class),
                    mock(OnboardingService.class));
            assertThat(registry.require(ShippingMethod.UBER_INSTANT.value())
                    .track(FulfillmentContext.forShipment(shipment(ShippingMethod.UBER_INSTANT))))
                    .as("W2-1 之後 Uber 的回查走 E4 Get Delivery")
                    .isPresent();
            assertThat(registry.require(ShippingMethod.TAQBIN.value())
                    .track(FulfillmentContext.forShipment(shipment(ShippingMethod.TAQBIN))))
                    .isPresent();
            assertThat(registry.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                    .track(FulfillmentContext.forShipment(
                            shipment(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP))))
                    .isPresent();
        }

        @Test
        @DisplayName("Uber：回查走自己的 provider API ⇒ 完全不打 PayUni")
        void uberNeverQueriesPayUni() {
            when(shipmentMapper.selectOne(any())).thenReturn(shipment(ShippingMethod.UBER_INSTANT));

            shippingService.getSellerTrackingByOrderNo("O1", 2685914682L, false);

            verify(shippingClient, never()).queryShipment(anyString(), any());
        }

        /**
         * 🔴 硬寫 {@code "HOME"} 而不是 {@code LgsType.HOME.code()}：
         * 期望值若跟著被測物一起變，這個斷言就什麼都驗不到。
         */
        @Test
        @DisplayName("黑貓：LgsType=HOME（adapter 的常數，不是從 shipping_method 即時推導）")
        void taqbinQueriesWithHome() {
            when(shipmentMapper.selectOne(any())).thenReturn(shipment(ShippingMethod.TAQBIN));
            payUniReturnsFailure();

            shippingService.getSellerTrackingByOrderNo("O1", 2685914682L, false);

            assertThat(capturedRequest().lgsType()).isEqualTo("HOME");
        }

        @Test
        @DisplayName("超商：LgsType=C2C")
        void cvsQueriesWithC2c() {
            when(shipmentMapper.selectOne(any())).thenReturn(
                    shipment(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP));
            payUniReturnsFailure();

            shippingService.getSellerTrackingByOrderNo("O1", 2685914682L, false);

            assertThat(capturedRequest().lgsType()).isEqualTo("C2C");
        }
    }

    // ==================================================================
    // ④ LabelCapability.usesThreeSegmentCode() 取代 `SEVEN_ELEVEN → 偵測三段碼`
    // ==================================================================

    @Nested
    @DisplayName("④ B-143 三段碼缺失偵測：這個 provider 的標籤帶不帶三段碼")
    class UsesThreeSegmentCode {

        @BeforeEach
        void webhookWritesNothing() {
            // 讓 markLabelPrintSucceededByWebhook 回 0（冪等跳過），把測試收斂在偵測這一段
            when(shipmentMapper.markLabelPrintSucceededByWebhook(
                    any(), any(), any(), any(), anyString())).thenReturn(0);
        }

        @Test
        @DisplayName("超商 + 三段碼缺失 ⇒ 發 PAYUNI_LGS_NO_CODE")
        void cvsMissingSegmentsPublishesAnomaly() {
            when(shipmentMapper.selectOne(any())).thenReturn(
                    shipment(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP));

            shippingService.processWebhookPrint("ST1", "", "", "");

            verify(anomalyEventPublisher).publishAnomalyDetected(
                    eq(ShipmentAnomalyType.PAYUNI_LGS_NO_CODE), anyString(), anyString(),
                    any(), any());
        }

        /**
         * 陰性面：宅配 by-design 不寫三段碼，誤觸發會讓賣家收到不存在的異常。
         * 把 {@code PayUniHomeAdapter} 的 {@code usesThreeSegmentCode()} 改成 true，這一條就轉紅。
         */
        @Test
        @DisplayName("黑貓 + 三段碼缺失 ⇒ 不發（usesThreeSegmentCode()=false）")
        void taqbinMissingSegmentsPublishesNothing() {
            when(shipmentMapper.selectOne(any())).thenReturn(shipment(ShippingMethod.TAQBIN));

            shippingService.processWebhookPrint("ST1", "", "", "");

            verify(anomalyEventPublisher, never()).publishAnomalyDetected(
                    any(), anyString(), anyString(), any(), any());
        }

        @Test
        @DisplayName("Uber + 三段碼缺失 ⇒ 不發（連 LabelCapability 都沒有）")
        void uberMissingSegmentsPublishesNothing() {
            when(shipmentMapper.selectOne(any())).thenReturn(shipment(ShippingMethod.UBER_INSTANT));

            shippingService.processWebhookPrint("ST1", "", "", "");

            verify(anomalyEventPublisher, never()).publishAnomalyDetected(
                    any(), anyString(), anyString(), any(), any());
        }
    }

    // ==================================================================
    // ⑤ OnboardCapability.shippingTypeValue() 取代 mapShippingMethodToType
    // ==================================================================

    // ==================================================================
    // ⑥ 四份重複的「賣家當下意願防線」收斂成 OnboardingService.requireActiveSetting
    // ==================================================================

    /**
     * 這一組不是 provider 分支，是<b>去重</b>：同樣三行守衛原本在
     * {@code printCvsLabel}／{@code batchPrintCvsLabel}／{@code generateHomeLabelPdf}／
     * {@code BatchHomeLabelService.batchGenerate} 各有一份副本。
     *
     * <p>🔴 收斂之後最怕的失效是「守衛靜默消失」——賣家取消建店後仍列印得出標籤，
     * 而那不會拋任何錯誤。所以陽性（沒建店就擋）與陰性（有建店就放行到下一道）兩面都要盯。
     */
    @Nested
    @DisplayName("⑥ 寫路徑的賣家建店守衛")
    class RequireActiveSetting {

        @Test
        @DisplayName("超商列印：沒有 ACTIVE 設定 ⇒ SHIPMENT_SELLER_NOT_ONBOARDED")
        void printCvsLabelBlockedWithoutSetting() {
            when(onboardingService.requireActiveSetting(anyLong(), any()))
                    .thenThrow(new ShippingBusinessException(
                            GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED, "7-11"));

            assertThatThrownBy(() -> shippingService.printCvsLabel(2685914682L, "O1", "20260930"))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .isEqualTo(GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED.code()));
            // 守衛在最前面：連物流單都還沒查
            verify(shipmentMapper, never()).selectOne(any());
        }

        @Test
        @DisplayName("宅配取號：沒有 ACTIVE 設定 ⇒ SHIPMENT_SELLER_NOT_ONBOARDED")
        void generateHomeLabelBlockedWithoutSetting() {
            when(onboardingService.requireActiveSetting(anyLong(), any()))
                    .thenThrow(new ShippingBusinessException(
                            GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED, "黑貓"));

            assertThatThrownBy(() -> shippingService.generateHomeLabelPdf(
                    2685914682L, "O1", "20260930", "20261001", List.of()))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .isEqualTo(GrpcResponseCode.SHIPMENT_SELLER_NOT_ONBOARDED.code()));
            verify(shipmentMapper, never()).selectOne(any());
        }

        /**
         * 陰性面：守衛通過就要往下走。只驗上面兩條的話，把 {@code requireActiveSetting}
         * 改成「一律拋」也會綠。
         */
        @Test
        @DisplayName("有 ACTIVE 設定 ⇒ 放行到下一道（查無物流單）")
        void printCvsLabelProceedsWhenOnboarded() {
            when(onboardingService.requireActiveSetting(anyLong(), any()))
                    .thenReturn(new SellerShipmentSettingEntity());
            when(shipmentMapper.selectOne(any())).thenReturn(null);

            assertThatThrownBy(() -> shippingService.printCvsLabel(2685914682L, "O1", "20260930"))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(ex -> assertThat(((ShippingBusinessException) ex).code())
                            .isEqualTo(GrpcResponseCode.SHIPMENT_NOT_FOUND.code()));
        }
    }

    @Nested
    @DisplayName("⑤ 建店槽位分類：配送方式 → shipping_type")
    class ShippingTypeValue {

        private final FulfillmentProviderRegistry registry = TestFulfillmentProviders.realRegistry(
                mock(ShippingClient.class), mock(ShipmentStatusMapper.class),
                mock(OnboardingService.class));

        /**
         * 從 registry 走一次完整解析路徑（配送方式值 → adapter → OnboardCapability），
         * 與 {@code OnboardingService.mapShippingMethodToType} 用的是同一條。
         *
         * <p>⚠️ 刻意<b>不</b>直接 new adapter：那樣只驗得到常數本身，
         * 驗不到「哪一個 adapter 服務哪一個配送方式」——把兩個 adapter 的
         * {@code supportedMethod()} 互換，直接 new 的寫法照樣綠。
         */
        private String resolve(ShippingMethod method) {
            return registry.require(method.value())
                    .onboard(FulfillmentContext.forStore(2685914682L, method))
                    .map(OnboardCapability::shippingTypeValue)
                    .orElse(null);
        }

        /** 🔴 硬寫 {@code "3651"}：期望值跟著 {@code ShippingType} 一起變的話，這條就驗不到東西。 */
        @Test
        @DisplayName("超商 → CVS_C2C(3651)")
        void cvsResolvesToCvsC2c() {
            assertThat(resolve(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP)).isEqualTo("3651");
        }

        @Test
        @DisplayName("黑貓 → HOME_DELIVERY(3652)")
        void taqbinResolvesToHomeDelivery() {
            assertThat(resolve(ShippingMethod.TAQBIN)).isEqualTo("3652");
        }

        @Test
        @DisplayName("Uber → 沒有建店概念（OnboardCapability 為 empty）")
        void uberHasNoOnboardingSlot() {
            assertThat(resolve(ShippingMethod.UBER_INSTANT)).isNull();
        }

        /**
         * {@code OnboardingService} 端的接線：解析不出 shipping_type 就<b>不查 DB</b>、回 null。
         *
         * <p>陰性面很重要——舊實作對未登記的配送方式 {@code return null}，
         * 兩個呼叫端都是 {@code if (shippingType == null) return null}，
         * 換成 registry 之後那個 null 必須還在，否則會變成「用 null 當 shipping_type 去查」。
         */
        @Test
        @DisplayName("OnboardingService：未註冊 adapter 的配送方式 → null，且不查 DB")
        void unregisteredMethodResolvesToNullWithoutDbHit() {
            SellerShipmentSettingMapper settingMapper = mock(SellerShipmentSettingMapper.class);
            OnboardingService service = new OnboardingService(
                    settingMapper,
                    mock(OnboardingLogMapper.class),
                    mock(SellerGrpcClient.class),
                    mock(ShippingClient.class),
                    mock(ProviderCallExecutor.class),
                    mock(ShippingFeeService.class),
                    registry);

            assertThat(service.findActiveSetting(
                    2685914682L, ShippingMethod.MERCHANT_DELIVERY.value())).isNull();
            verify(settingMapper, never()).selectOne(any());
        }

        /**
         * 反向查表（{@code getSellerReturnConfig}）：shipping_type → 配送方式。
         *
         * <p>舊實作把它寫成第二份寫死的對照表 ⇒ 與 {@code shippingTypeValue()} 是兩個真實來源。
         * 這一組把來回都釘住：{@code 3651 → 3002 → 3651}。
         */
        @Test
        @DisplayName("反向查表：3651 → 超商、3652 → 黑貓，未知值仍拋 IllegalArgumentException")
        void reverseLookupFromShippingType() {
            when(shipmentMapper.selectOne(any())).thenReturn(null);
            ArgumentCaptor<String> methodCaptor = ArgumentCaptor.forClass(String.class);

            shippingService.getSellerReturnConfig(2685914682L, "3651");
            verify(onboardingService).findSettingByMethod(anyLong(), methodCaptor.capture());
            assertThat(methodCaptor.getValue()).isEqualTo("3002");

            shippingService.getSellerReturnConfig(2685914682L, "3652");
            verify(onboardingService, org.mockito.Mockito.times(2))
                    .findSettingByMethod(anyLong(), methodCaptor.capture());
            assertThat(methodCaptor.getValue()).isEqualTo("3001");

            assertThatThrownBy(() -> shippingService.getSellerReturnConfig(2685914682L, "9999"))
                    .isInstanceOf(IllegalArgumentException.class)
                    .hasMessageContaining("不支援的 shipping_type");
        }

        /** 陽性面：註冊過的配送方式解析得出 shipping_type ⇒ 確實會去查 DB。 */
        @Test
        @DisplayName("OnboardingService：已註冊的配送方式會查 DB")
        void registeredMethodHitsDb() {
            SellerShipmentSettingMapper settingMapper = mock(SellerShipmentSettingMapper.class);
            OnboardingService service = new OnboardingService(
                    settingMapper,
                    mock(OnboardingLogMapper.class),
                    mock(SellerGrpcClient.class),
                    mock(ShippingClient.class),
                    mock(ProviderCallExecutor.class),
                    mock(ShippingFeeService.class),
                    registry);

            service.findActiveSetting(2685914682L, ShippingMethod.TAQBIN.value());
            verify(settingMapper).selectOne(any());
        }
    }
}

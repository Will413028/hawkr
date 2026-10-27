package io.github.will413028.hawkr.shipping.provider;

import io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver;
import io.github.will413028.hawkr.common.providercall.DefaultProviderCallExecutor;
import io.github.will413028.hawkr.common.providercall.ProviderCallBusyException;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.payuni.http.PayUniApiResponse;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.payuni.shipping.request.CallTcatRequest;
import io.github.will413028.hawkr.payuni.shipping.request.CreateCvsShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.request.CreateHomeRefundRequest;
import io.github.will413028.hawkr.payuni.shipping.request.QueryShipmentRequest;
import io.github.will413028.hawkr.payuni.shipping.response.CallTcatResponse;
import io.github.will413028.hawkr.payuni.shipping.response.CreateCvsShipmentResponse;
import io.github.will413028.hawkr.payuni.shipping.response.CreateHomeRefundResponse;
import io.github.will413028.hawkr.payuni.shipping.response.QueryShipmentResponse;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import io.github.will413028.hawkr.shipping.client.OrderGrpcClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.TcatCallStatus;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentStatusLogMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import io.github.will413028.hawkr.shipping.provider.capability.DispatchCapability;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.StoreLocatorCapability;
import io.github.will413028.hawkr.shipping.service.CvsPrintWaitingRegistry;
import io.github.will413028.hawkr.shipping.service.HolidayService;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.ShipmentLabelService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import io.github.will413028.hawkr.shipping.service.ShippingService;
import io.github.will413028.hawkr.shipping.service.StoreMapService;
import io.github.will413028.hawkr.shipping.provider.create.ShipmentStrategy;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.util.Collections;
import java.util.List;
import java.util.Map;

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
 * 第二波搬移（{@code label} / {@code returns} / {@code storeLocator}）之後的行為。
 *
 * <h2>為什麼這一檔要存在</h2>
 * 「capability 已經搬進 adapter」在型別上看不出來——{@code Optional.of(...)} 與
 * {@code Optional.empty()} 都能編譯。真正要盯的是<b>行為有沒有真的改走 adapter</b>：
 * <ul>
 *   <li>provider 請求裡的參數是不是<b>來自 adapter 的常數</b>而不是 entity 上的欄位</li>
 *   <li>「還沒搬」的清單（{@link FulfillmentProviderAdapter#pendingMigration()}）
 *       有沒有被人偷偷加回去——加回去不會有任何東西轉紅</li>
 * </ul>
 */
@DisplayName("label／returns／storeLocator／dispatch 搬進 adapter 之後的行為")
class FulfillmentCapabilityMigrationTest {

    /** 讓 {@code requiresNewTx} 真的跑得起來（mock 的 tx manager 會回 null status 而 NPE）。 */
    private static final class NoopTxManager implements PlatformTransactionManager {
        @Override
        public TransactionStatus getTransaction(TransactionDefinition definition) {
            return new SimpleTransactionStatus(true);
        }

        @Override
        public void commit(TransactionStatus status) {
        }

        @Override
        public void rollback(TransactionStatus status) {
        }
    }

    /**
     * 逆向建單會產 TSID 主鍵（{@code TsidUtil.generate()}），未初始化時直接拋
     * {@code IllegalStateException} ——那會蓋掉本檔真正要斷言的例外。
     * shipping-service 的 node-id-base 見 §7.2（700 是 aftersales，shipping 是 600）。
     */
    @BeforeAll
    static void initTsid() {
        if (!io.github.will413028.hawkr.common.utils.TsidUtil.isInitialized()) {
            io.github.will413028.hawkr.common.utils.TsidUtil.init(600, 0);
        }
    }

    private ShipmentMapper shipmentMapper;
    private ShippingClient shippingClient;
    private OnboardingService onboardingService;
    private ShipmentLabelService labelService;
    private StoreMapService storeMapService;
    private FulfillmentProviderRegistry registry;
    private ShippingService shippingService;

    @BeforeEach
    void setUp() {
        shipmentMapper = mock(ShipmentMapper.class);
        shippingClient = mock(ShippingClient.class);
        onboardingService = mock(OnboardingService.class);
        labelService = mock(ShipmentLabelService.class);
        storeMapService = mock(StoreMapService.class);

        registry = TestFulfillmentProviders.realRegistry(
                shippingClient, mock(ShipmentStatusMapper.class), onboardingService, labelService,
                mock(io.github.will413028.hawkr.shipping.provider.create.HomeShipmentStrategy.class),
                mock(io.github.will413028.hawkr.shipping.provider.create.CvsShipmentStrategy.class),
                mock(io.github.will413028.hawkr.shipping.provider.create.UberShipmentStrategy.class),
                storeMapService);

        shippingService = new ShippingService(
                shipmentMapper,
                mock(ShipmentStatusLogMapper.class),
                mock(OrderGrpcClient.class),
                mock(BuyerGrpcClient.class),
                mock(MediaGrpcClient.class),
                mock(SellerGrpcClient.class),
                mock(ShipmentStatusMapper.class),
                labelService,
                mock(ShippingEventPublisher.class),
                mock(ShipmentAnomalyEventPublisher.class),
                onboardingService,
                mock(HolidayService.class),
                mock(CvsPrintWaitingRegistry.class),
                // 🔴 用 production 實作而不是 mock：callTcat 的整條路徑（acquire → invoke →
                //    markSucceeded／markFailed）就是被測的東西之一，mock 掉等於把它跳過。
                //    其餘既有測試都不經過 executor，換掉不影響它們。
                new DefaultProviderCallExecutor(),
                new NoopTxManager(),
                registry,
                // 🔴 真的 resolver，不是 mock：write path 的守衛（BLOCKER 3）走的就是它，
                //    傳 mock 等於把剛加上的守衛在測試裡關掉。
                new AllowedActionResolver(registry, new ShipmentStatusMapper()),
                new io.micrometer.core.instrument.simple.SimpleMeterRegistry());
    }

    // ==================================================================
    // ⓪ 遷移狀態的誠實標註：pendingMigration() 不得把已搬完的項目加回去
    // ==================================================================

    @Nested
    @DisplayName("⓪ pendingMigration()：已搬完的 capability 不得再列為「還沒搬」")
    class PendingMigrationShrinks {

        /**
         * 🔴 這一條盯的是<b>反向退步</b>。{@code pendingMigration()} 是一份手寫清單，
         * 把 {@code "label"} 加回去不會讓任何編譯或測試失敗——而下一個人讀到它，
         * 就會以為標籤還沒搬，然後在別處重新實作一份。那正是本 adapter 要消滅的東西。
         *
         * <p>⚠️ 這一波（{@code dispatch}）之後，PayUni 兩軸的<b>目標終態是空集合</b>：
         * 3001／3002 在 production 跑的 capability 已全數在 adapter 內。
         * 用 {@code isEmpty()} 而不是逐項 {@code doesNotContain}，是因為後者對
         * 「新增一個沒人注意到的 pending 項」完全沒有鑑別力——而那正是這份清單
         * 會用來擱置工作的方式。
         */
        @Test
        @DisplayName("3001／3002 的清單已是空集合（含本波搬入的 dispatch）")
        void payUniAxesHaveNothingPending() {
            for (String methodValue : List.of(ShippingMethod.TAQBIN.value(),
                    ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())) {
                assertThat(registry.require(methodValue).pendingMigration())
                        .as(methodValue + " 已搬完的 capability 不得再列為 pending"
                                + "（label／returns／storeLocator／dispatch 都已在 adapter 內）")
                        .isEmpty();
            }
        }

        /**
         * 陰性面。
         *
         * <p>⚠️ 本條原本是「3003 Uber 的 dispatch 仍要標著」。W2-1 把 Uber 的七個
         * capability 全部接上之後，<b>三條線的清單都空了</b>，於是那個斷言的前提消失
         * ——但它要守的東西沒有：<b>清空這份清單必須代表真的搬完了，不是宣告勝利</b>。
         *
         * <p>🔴 因此改成直接驗那件事：空集合 ⇒ 每一個「不在永久 empty 清單裡」的
         * capability 都必須真的 {@code isPresent()}。
         * 把 {@code pendingMigration()} 改成 {@code Set.of()} 而不接上實作，本條會轉紅；
         * 而舊寫法（{@code contains("dispatch")}）對那個變異<b>完全沒有鑑別力</b>
         * ——它只看得到清單自己的內容。
         *
         * <p>Uber 的三個永久 empty（66 §4.5.0 逐字「這不是缺口」）：
         * {@code label}（無標籤概念）／{@code returns}（逆向 leg 是另開一筆）／
         * {@code storeLocator}（送到府，沒有門市）。
         */
        @Test
        @DisplayName("清單空了就必須真的搬完：3003 的七個 capability 逐一 present")
        void emptyPendingListMeansCapabilitiesAreReallyThere() {
            FulfillmentProviderAdapter uber = registry.require(ShippingMethod.UBER_INSTANT.value());
            FulfillmentContext ctx = FulfillmentContext.forShipment(uberShipment());

            assertThat(uber.pendingMigration())
                    .as("W2-1 之後 3003 沒有「provider 有能力但我們還沒搬」的 capability")
                    .isEmpty();

            assertThat(uber.create(ctx)).as("E2 Create Delivery").isPresent();
            assertThat(uber.dispatch(ctx)).as("E2（由 order.paid／賣家重試觸發）").isPresent();
            assertThat(uber.cancel(ctx)).as("E6 —— 四個 provider 裡唯一").isPresent();
            assertThat(uber.track(ctx)).as("E4 Get Delivery").isPresent();
            assertThat(uber.pod(ctx)).as("E7 ＋ 轉存 media secure/").isPresent();
            assertThat(uber.onboard(ctx)).as("uber_store_mapping 的狀態").isPresent();
            assertThat(uber.inbound(ctx)).as("webhook 驗簽 ＋ 狀態映射").isPresent();

            assertThat(uber.label(ctx)).as("永久 empty：即時配送無標籤概念").isEmpty();
            assertThat(uber.returns(ctx)).as("永久 empty：逆向 leg 是另開一筆").isEmpty();
            assertThat(uber.storeLocator(ctx)).as("永久 empty：送到府，沒有門市").isEmpty();
        }

        private ShipmentEntity uberShipment() {
            ShipmentEntity e = new ShipmentEntity();
            e.setId(3003L);
            e.setOrderNo("O-UBER-1");
            e.setShipmentNo("S-UBER-1");
            e.setStoreId(2685914682L);
            e.setShippingMethod(ShippingMethod.UBER_INSTANT.value());
            e.setStatus(ShippingStatus.UNPROCESSED.value());
            return e;
        }
    }

    // ==================================================================
    // ⓪-b DispatchCapability：黑貓叫件（本波搬入）
    // ==================================================================

    /**
     * 黑貓叫件（{@code callTcat}）走 adapter 之後的行為。
     *
     * <h2>🔴 這一組是本波唯一有鑑別力的護欄</h2>
     * 搬移前 {@code ShippingService.callTcat} <b>一支測試都沒有</b>（實查：整個 test source tree
     * 對 {@code shippingService.callTcat} 零命中）。也就是說「185 支既有測試全綠」對這次改動的
     * 鑑別力是 <b>0</b>——把溫層對照改錯、把寄件人地址組錯、把 Y/N 反過來，既有測試通通不會轉紅。
     *
     * <p>所以這裡驗的是**參數本身**：用 {@link ArgumentCaptor} 抓 adapter 真正送給 PayUni 的
     * {@link CallTcatRequest}，逐欄比對。fixture 的欄位形狀取自 provider 的 record 定義
     * （{@code CallTcatRequest} 的 9 個 component），不是我對「應該長怎樣」的想像。
     */
    @Nested
    @DisplayName("⓪-b 黑貓叫件走 adapter")
    class Dispatch {

        private static final long STORE_ID = 2685914682L;
        private static final String ORDER_NO = "O400280260";

        @BeforeEach
        void shipmentIsReadyToDispatch() {
            when(shipmentMapper.selectOne(any())).thenReturn(taqbinShipment());
            when(onboardingService.requireActiveSetting(STORE_ID, ShippingMethod.TAQBIN))
                    .thenReturn(senderSetting());
            when(shipmentMapper.acquireTcatCallRequested(anyLong(), any(), any(), any()))
                    .thenReturn(1);
            when(shipmentMapper.markTcatCallSucceeded(anyLong(), any(), anyString()))
                    .thenReturn(1);
        }

        /**
         * 🔴 <b>溫層 → 件數欄位</b>：PayUni 收的是三個件數欄位，一筆單只落在其中一格。
         * 這個對照遷移前是 {@code ShippingService} 裡的 if/else if 鏈。
         *
         * <p>⚠️ 三個溫層分別驗，而且每次都斷言<b>另外兩格是 null</b>——只驗「冷凍那格是 1」
         * 的話，把三格都填 1 也會綠。
         */
        @Test
        @DisplayName("冷凍單：只有 FreezeQuantity 有值，常溫／冷藏兩格必須是 null")
        void frozenGoesToFreezeQuantityOnly() {
            CallTcatRequest req = dispatchWithGoodsType("2");   // GoodsType.FROZEN
            assertThat(req.freezeQuantity()).isEqualTo("1");
            assertThat(req.normalQuantity()).as("冷凍不得同時算成常溫件").isNull();
            assertThat(req.coldQuantity()).as("冷凍不得同時算成冷藏件").isNull();
        }

        @Test
        @DisplayName("冷藏單：只有 ColdQuantity 有值")
        void refrigeratedGoesToColdQuantityOnly() {
            CallTcatRequest req = dispatchWithGoodsType("3");   // GoodsType.REFRIGERATED
            assertThat(req.coldQuantity()).isEqualTo("1");
            assertThat(req.normalQuantity()).isNull();
            assertThat(req.freezeQuantity()).isNull();
        }

        @Test
        @DisplayName("常溫單：只有 NormalQuantity 有值")
        void normalGoesToNormalQuantityOnly() {
            CallTcatRequest req = dispatchWithGoodsType("1");   // GoodsType.NORMAL
            assertThat(req.normalQuantity()).isEqualTo("1");
            assertThat(req.coldQuantity()).isNull();
            assertThat(req.freezeQuantity()).isNull();
        }

        /**
         * 陰性面：未知溫層碼三格都是 null，與遷移前的 if/else if 鏈逐字相同。
         *
         * <p>🔴 這一條擋的是「順手改成預設常溫」——那會把一筆資料有問題的單靜默送成常溫件，
         * 而 PayUni 會回 200。
         */
        @Test
        @DisplayName("未知溫層碼：三格都是 null，不得預設成常溫")
        void unknownGoodsTypeFillsNothing() {
            CallTcatRequest req = dispatchWithGoodsType("9");
            assertThat(req.normalQuantity()).isNull();
            assertThat(req.coldQuantity()).isNull();
            assertThat(req.freezeQuantity()).isNull();
        }

        /**
         * 寄件人資訊來自 {@code seller_shipment_settings}，地址是
         * {@code city + district + 正規化後的 detail}——與逆向建單共用同一支 {@code fullAddress}。
         */
        @Test
        @DisplayName("寄件人：姓名／電話／地址取自賣家設定，地址已串接並正規化")
        void senderComesFromSellerSetting() {
            CallTcatRequest req = dispatchWithGoodsType("1");
            assertThat(req.contactName()).isEqualTo("範例生鮮小舖");
            assertThat(req.contactMobile()).isEqualTo("0912345678");
            assertThat(req.contactAddress())
                    .as("city + district + detail 串成 provider 要的單一字串")
                    .startsWith("台北市大安區")
                    .contains("敦化南路一段");
        }

        /**
         * 🔴 {@code "Y"}／{@code "N"} 是 PayUni 的參數形狀，翻譯必須在 adapter。
         * 兩個旗標<b>取相反值</b>：都送 true 的話，把兩者接反也會綠。
         */
        @Test
        @DisplayName("中性布林翻成 provider 的 Y／N，且兩個旗標沒有接反")
        void booleanOptionsAreTranslatedAndNotSwapped() {
            CallTcatResponse ok = new CallTcatResponse(success(), "2026-09-10 11:22:33");
            when(shippingClient.callTcat(anyString(), any())).thenReturn(ok);

            // 電聯 Y、推車 N（＝ service 的預設值路徑：兩個入參留白）
            shippingService.callTcat(STORE_ID, ORDER_NO, null, null, "請按電鈴");

            ArgumentCaptor<CallTcatRequest> req = ArgumentCaptor.forClass(CallTcatRequest.class);
            verify(shippingClient).callTcat(eq("HAWK2685914682"), req.capture());
            assertThat(req.getValue().isContact()).as("電聯預設 Y").isEqualTo("Y");
            assertThat(req.getValue().isTrolley()).as("推車預設 N").isEqualTo("N");
            assertThat(req.getValue().memo()).isEqualTo("請按電鈴");
        }

        /** 反向：入參明確指定時照送，不被預設值蓋掉。 */
        @Test
        @DisplayName("明確指定 N／Y 時照送（預設值不得蓋掉入參）")
        void explicitOptionsWin() {
            CallTcatResponse ok = new CallTcatResponse(success(), "2026-09-10 11:22:33");
            when(shippingClient.callTcat(anyString(), any())).thenReturn(ok);

            shippingService.callTcat(STORE_ID, ORDER_NO, "N", "Y", null);

            ArgumentCaptor<CallTcatRequest> req = ArgumentCaptor.forClass(CallTcatRequest.class);
            verify(shippingClient).callTcat(anyString(), req.capture());
            assertThat(req.getValue().isContact()).isEqualTo("N");
            assertThat(req.getValue().isTrolley()).isEqualTo("Y");
        }

        /**
         * provider 的受理時間透過中性的 {@code DispatchOutcome.dispatchedAt()} 回到呼叫端，
         * 再原樣回給 gRPC。少了這個欄位，provider 呼叫就搬不進 adapter。
         */
        @Test
        @DisplayName("成功：回傳 provider 的受理時間，並轉 210 TCAT_CALLED")
        void successReturnsProviderCallTimeAndAdvancesStatus() {
            when(shippingClient.callTcat(anyString(), any()))
                    .thenReturn(new CallTcatResponse(success(), "2026-09-10 11:22:33"));

            String callTime = shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null);

            assertThat(callTime).isEqualTo("2026-09-10 11:22:33");
            verify(shipmentMapper).markTcatCallSucceeded(
                    eq(1L), any(), eq(ShippingStatus.TCAT_CALLED.value()));
        }

        /**
         * 🔴 <b>平台流程留在 service 的證據</b>：搶佔失敗（另一個執行緒已在叫件）時，
         * provider 一次都不能被呼叫。若有人把搶佔一起搬進 adapter，這一條會轉紅。
         */
        @Test
        @DisplayName("搶佔失敗：provider 完全不被呼叫，回「處理中」")
        void busyNeverTouchesProvider() {
            when(shipmentMapper.acquireTcatCallRequested(anyLong(), any(), any(), any()))
                    .thenReturn(0);

            assertThatThrownBy(() -> shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null))
                    .isInstanceOf(ShippingBusinessException.class);
            verify(shippingClient, never()).callTcat(anyString(), any());
        }

        /**
         * 收斂規範 §8.1：同步 retry 最多<b>一次</b>（總共兩次呼叫）。
         * 兩次都失敗 ⇒ 走 markFailed 並落 provider 明細，然後對外拋業務例外。
         *
         * <p>⚠️ 斷言 {@code times(2)} 而不是 {@code atLeastOnce()}：無上限的重試對 provider
         * 是放大攻擊，而「retry 一次」正是這裡要釘住的約束。
         */
        @Test
        @DisplayName("兩次都失敗：retry 恰好一次、落 provider 明細、走 markFailed")
        void retriesExactlyOnceThenMarksFailed() {
            when(shippingClient.callTcat(anyString(), any())).thenReturn(
                    new CallTcatResponse(
                            new PayUniApiResponse("TCAT09", "交易失敗",
                                    Map.of("ErrorMsg", "取件地址不在服務範圍")),
                            null));

            assertThatThrownBy(() -> shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null))
                    .isInstanceOf(ShippingBusinessException.class);

            verify(shippingClient, org.mockito.Mockito.times(2)).callTcat(anyString(), any());
            verify(shipmentMapper).markTcatCallFailed(eq(1L), eq("TCAT09"), eq("取件地址不在服務範圍"));
            verify(shipmentMapper, never()).markTcatCallSucceeded(anyLong(), any(), anyString());
        }

        /**
         * 第一次失敗、retry 成功 ⇒ 視為成功。
         * 這一條與上一條合起來才釘得住「retry 存在且真的會被用到」。
         */
        @Test
        @DisplayName("第一次失敗、retry 成功 ⇒ 成功")
        void secondAttemptSucceeds() {
            when(shippingClient.callTcat(anyString(), any()))
                    .thenReturn(new CallTcatResponse(
                            new PayUniApiResponse("TCAT09", "暫時失敗", Map.of()), null))
                    .thenReturn(new CallTcatResponse(success(), "2026-09-10 11:22:33"));

            assertThat(shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null))
                    .isEqualTo("2026-09-10 11:22:33");
            verify(shipmentMapper).markTcatCallSucceeded(eq(1L), any(), anyString());
        }

        /**
         * 🔴 <b>ErrorMsg 優先於泛用 message</b>：PayUni 的 {@code message} 常常只是「交易失敗」，
         * 真正能讓賣家修正的字在 {@code ErrorMsg}。哨兵值 {@code "-"} 要當成沒有。
         */
        @Test
        @DisplayName("ErrorMsg 是哨兵值 \"-\" ⇒ 退回泛用 message，不是把 - 寫進 DB")
        void sentinelErrorMsgFallsBackToMessage() {
            when(shippingClient.callTcat(anyString(), any())).thenReturn(
                    new CallTcatResponse(
                            new PayUniApiResponse("TCAT09", "交易失敗", Map.of("ErrorMsg", "-")),
                            null));

            assertThatThrownBy(() -> shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null))
                    .isInstanceOf(ShippingBusinessException.class);
            verify(shipmentMapper).markTcatCallFailed(eq(1L), eq("TCAT09"), eq("交易失敗"));
        }

        /**
         * 🔴 <b>fail-closed</b>：provider 沒有派車能力（Uber 的 dispatch capability 為 empty）
         * 時必須擋，不得靜默略過。少了這一條，{@code requireDispatchCapability} 改成
         * 「查不到就跳過」也會綠——而那會讓賣家按了叫件、司機永遠不來。
         */
        @Test
        @DisplayName("provider 無派車能力（Uber）⇒ 擋下，provider 不被呼叫")
        void providerWithoutDispatchIsRejected() {
            ShipmentEntity uber = taqbinShipment();
            uber.setShippingMethod(ShippingMethod.UBER_INSTANT.value());
            when(shipmentMapper.selectOne(any())).thenReturn(uber);

            assertThatThrownBy(() -> shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null))
                    .isInstanceOf(ShippingBusinessException.class);
            verify(shippingClient, never()).callTcat(anyString(), any());
            verify(shipmentMapper, never()).acquireTcatCallRequested(anyLong(), any(), any(), any());
        }

        /** 前置守衛留在 service：尚未取號不得叫件。 */
        @Test
        @DisplayName("尚未取號 ⇒ 擋下，provider 不被呼叫")
        void labelRequiredBeforeDispatch() {
            ShipmentEntity e = taqbinShipment();
            e.setLabelPrintStatus(LabelPrintStatus.NONE.value());
            when(shipmentMapper.selectOne(any())).thenReturn(e);

            assertThatThrownBy(() -> shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null))
                    .isInstanceOf(ShippingBusinessException.class);
            verify(shippingClient, never()).callTcat(anyString(), any());
        }

        private CallTcatRequest dispatchWithGoodsType(String goodsType) {
            ShipmentEntity e = taqbinShipment();
            e.setGoodsType(goodsType);
            when(shipmentMapper.selectOne(any())).thenReturn(e);
            when(shippingClient.callTcat(anyString(), any()))
                    .thenReturn(new CallTcatResponse(success(), "2026-09-10 11:22:33"));

            shippingService.callTcat(STORE_ID, ORDER_NO, "Y", "N", null);

            ArgumentCaptor<CallTcatRequest> req = ArgumentCaptor.forClass(CallTcatRequest.class);
            verify(shippingClient).callTcat(eq("HAWK2685914682"), req.capture());
            return req.getValue();
        }

        private ShipmentEntity taqbinShipment() {
            ShipmentEntity e = new ShipmentEntity();
            e.setId(1L);
            e.setShipmentNo("SH1");
            e.setOrderNo(ORDER_NO);
            e.setStoreId(STORE_ID);
            e.setBuyerId(555L);
            e.setMerId("HAWK2685914682");
            e.setShipTradeNo("ST1");
            e.setShippingMethod(ShippingMethod.TAQBIN.value());
            e.setStatus(ShippingStatus.UNPROCESSED.value());
            e.setGoodsType("1");
            e.setLabelPrintStatus(LabelPrintStatus.SUCCEEDED.value());
            e.setTcatCallStatus(TcatCallStatus.NONE.value());
            return e;
        }

        private SellerShipmentSettingEntity senderSetting() {
            SellerShipmentSettingEntity s = new SellerShipmentSettingEntity();
            s.setStoreId(STORE_ID);
            s.setSenderName("範例生鮮小舖");
            s.setSenderPhone("0912345678");
            s.setSenderCity("台北市");
            s.setSenderDistrict("大安區");
            s.setSenderAddressDetail("敦化南路一段100號");
            return s;
        }
    }

    // ==================================================================
    // ① ReturnCapability：逆物流建單
    // ==================================================================

    @Nested
    @DisplayName("① 逆物流建單走 adapter")
    class Returns {

        @BeforeEach
        void sellerHasReturnSetting() {
            SellerShipmentSettingEntity setting = new SellerShipmentSettingEntity();
            setting.setSenderName("賣家收件人");
            setting.setSenderPhone("0912345678");
            setting.setReturnStoreCode("252698");
            setting.setReturnStoreName("敦安門市");
            when(onboardingService.findSettingByMethod(anyLong(), anyString()))
                    .thenReturn(setting);
        }

        /**
         * 🔴 這一條的鑑別力在 <b>{@code LgsType} 來自 adapter 的常數</b>：
         * 遷移前它是 {@code LgsType.C2C.code()} 就地寫在 ShippingService 裡。
         * 把 {@code PayUniCvsAdapter.lgsTypeCode()} 改成 HOME，本條就轉紅。
         */
        @Test
        @DisplayName("超商退貨：provider 請求的參數來自 adapter，不是呼叫端")
        void cvsReturnUsesAdapterParams() {
            when(shippingClient.createCvsShipment(anyString(), any()))
                    .thenReturn(new CreateCvsShipmentResponse(success(), "ST-RETURN", "91"));

            // 成功路徑會在 commit 後排非同步列印（production 由 @Transactional 提供同步器；
            // 單元測試沒有 Spring proxy，要自己開一個，否則卡在 registerSynchronization）
            ShippingService.ReturnShipmentResult result;
            org.springframework.transaction.support.TransactionSynchronizationManager.initSynchronization();
            try {
                result = shippingService.createReturnCvsShipment(
                        "O400280260", 2685914682L, "HAWK2685914682",
                        "買家", "0987654321", ShippingTemperature.AMBIENT.value(), 350L);
            } finally {
                org.springframework.transaction.support.TransactionSynchronizationManager.clearSynchronization();
            }

            ArgumentCaptor<CreateCvsShipmentRequest> req =
                    ArgumentCaptor.forClass(CreateCvsShipmentRequest.class);
            verify(shippingClient).createCvsShipment(eq("HAWK2685914682"), req.capture());

            assertThat(req.getValue().lgsType())
                    .as("LgsType 必須來自 adapter 的常數（PayUniCvsAdapter.lgsTypeCode()）")
                    .isEqualTo("C2C");
            assertThat(req.getValue().merTradeNo())
                    .as("逆向 MerTradeNo 的後綴是 provider 約束，已搬進 AbstractPayUniAdapter")
                    .isEqualTo("O400280260-Refund");
            assertThat(req.getValue().senderName()).isEqualTo("買家");
            assertThat(req.getValue().receiverName()).isEqualTo("賣家收件人");
            assertThat(req.getValue().storeId()).isEqualTo("252698");
            assertThat(result.trackingCode()).isEqualTo("ST-RETURN");
        }

        /**
         * 🔴 失敗殼是收斂規範 §8.4 的硬要求：provider 拒絕時仍必須有一列落地，
         * 否則 Admin 的異常清單查不到這筆退貨——症狀是「退貨憑空消失」。
         *
         * <p>同時釘住殼的<b>欄位組成</b>：遷移前它是第二份手寫副本，已經與成功列漂移。
         */
        @Test
        @DisplayName("超商退貨被拒：落一列 CREATE_FAILED 殼＋帶 provider 狀態，然後才拋")
        void cvsReturnRejectedPersistsShell() {
            when(shippingClient.createCvsShipment(anyString(), any()))
                    .thenReturn(new CreateCvsShipmentResponse(
                            new PayUniApiResponse("ERR01", "門市代碼錯誤", Map.of()), null, null));

            assertThatThrownBy(() -> shippingService.createReturnCvsShipment(
                    "O400280260", 2685914682L, "HAWK2685914682",
                    "買家", "0987654321", ShippingTemperature.AMBIENT.value(), 350L))
                    .isInstanceOf(ShippingBusinessException.class);

            ArgumentCaptor<ShipmentEntity> shell = ArgumentCaptor.forClass(ShipmentEntity.class);
            verify(shipmentMapper).insert(shell.capture());
            assertThat(shell.getValue().getStatus()).isEqualTo(ShippingStatus.CREATE_FAILED.value());
            // 🔴 建單失敗是 provider 層**失敗**，落 provider_error_*，不佔用貨態證據欄（V12）。
            //    這一支原本斷言 getProviderStatus()，那是拆欄前的行為——把失敗代碼寫進
            //    provider_status 正是 ADM 異常判準只能寫成「有值即異常」的成因。
            assertThat(shell.getValue().getProviderErrorCode()).isEqualTo("ERR01");
            assertThat(shell.getValue().getProviderErrorMessage()).isEqualTo("門市代碼錯誤");
            assertThat(shell.getValue().getProviderErrorAt())
                    .as("失敗時間要與貨態證據時間分離")
                    .isNotNull();
            assertThat(shell.getValue().getProviderStatus())
                    .as("🔴 陰性：貨態欄不得被失敗代碼污染——否則正常單與失敗單在 ADM 眼中無法區分")
                    .isNull();
            assertThat(shell.getValue().getIsReturn()).isTrue();
            assertThat(shell.getValue().getId())
                    .as("殼也要有主鍵，否則 insert 會炸在一個看不出原因的地方")
                    .isNotNull();
            assertThat(shell.getValue().getCvsStoreCode())
                    .as("殼與成功列共用同一份欄位組裝：門市代碼不得只出現在成功路徑")
                    .isEqualTo("252698");
        }

        /**
         * 宅配逆向：取件日推算（D+2、週日順延、台北時區）已搬進 adapter。
         * 這裡釘住的是「呼叫端沒有再算一次」——請求裡的 shipDate 由 adapter 決定。
         */
        @Test
        @DisplayName("宅配退貨：規格沿用原單，取件日由 adapter 推算")
        void homeReturnUsesForwardSpecAndAdapterPickupDate() {
            ShipmentEntity forward = new ShipmentEntity();
            forward.setSpec("3");
            when(shipmentMapper.selectOne(any())).thenReturn(forward);
            when(shippingClient.createHomeRefund(anyString(), any()))
                    .thenReturn(new CreateHomeRefundResponse(success(), "ST-HOME", "OBT1", "MT1"));

            shippingService.createReturnHomeShipment(
                    "O400280260", 2685914682L, "HAWK2685914682",
                    "買家", "0987654321", "台北市", "大安區", "敦化南路一段",
                    "106", ShippingTemperature.FROZEN.value());

            ArgumentCaptor<CreateHomeRefundRequest> req =
                    ArgumentCaptor.forClass(CreateHomeRefundRequest.class);
            verify(shippingClient).createHomeRefund(eq("HAWK2685914682"), req.capture());

            assertThat(req.getValue().spec())
                    .as("規格沿用原單（forwardShipment.spec）")
                    .isEqualTo("3");
            assertThat(req.getValue().goodsType())
                    .as("溫層換算成 provider 的 GoodsType（FROZEN → 2）")
                    .isEqualTo("2");
            assertThat(req.getValue().shipDate())
                    .as("取件日由 adapter 推算，格式為 provider 要的 yyyyMMdd")
                    .matches("\\d{8}");
            assertThat(req.getValue().shipTradeNo())
                    .as("刻意不帶原單號：帶了 PayUni 會檢查原單狀態")
                    .isNull();
        }
    }

    // ==================================================================
    // ② LabelCapability：列印與標籤識別回查
    // ==================================================================

    @Nested
    @DisplayName("② 標籤能力走 adapter")
    class Label {

        /**
         * 🔴 鑑別力在 {@code lgsType}：遷移前這裡送的是 {@code entity.getLgsType()}
         * （一個平台欄位），現在送的是 adapter 的常數。fixture 刻意把 entity 的 lgsType
         * 設成一個錯的值——照舊實作會送出 "WRONG"。
         */
        @Test
        @DisplayName("超商列印：LgsType 來自 adapter 常數，不是 entity 欄位")
        void cvsPrintUsesAdapterLgsType() {
            ShipmentEntity entity = cvsShipment();
            entity.setLgsType("WRONG");

            registry.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                    .label(FulfillmentContext.forShipment(entity)).orElseThrow()
                    .requestLabel(entity, LabelCapability.LabelRequest.onShipDate("20260910"));

            verify(labelService).triggerCvsPrint(
                    eq("HAWK100"), eq("ST1"), eq("1"), eq("C2C"), eq("UNIMART"), eq("20260910"));
        }

        /**
         * 🔴 {@code shipDate} 是 provider 的必填請求參數，而 {@code ShipmentEntity} 上沒有它。
         * adapter 代填一個「今天」會直接送錯資料給 provider，而回應是 200。
         */
        @Test
        @DisplayName("超商列印缺 shipDate ⇒ 拋，不得代填")
        void cvsPrintRefusesToInventShipDate() {
            ShipmentEntity entity = cvsShipment();
            LabelCapability cap = registry.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                    .label(FulfillmentContext.forShipment(entity)).orElseThrow();

            assertThatThrownBy(() -> cap.requestLabel(entity, LabelCapability.LabelRequest.onShipDate("")))
                    .isInstanceOf(IllegalArgumentException.class);
            verify(labelService, never()).triggerCvsPrint(
                    anyString(), anyString(), any(), any(), any(), anyString());
        }

        /**
         * 批次約束若沒有<b>執行點</b>，{@code batchConstraints()} 只是一份被閱讀的文件。
         */
        @Test
        @DisplayName("批次分組鍵不一致 ⇒ 拋（約束有被執行，不只是宣告）")
        void batchConstraintsAreEnforced() {
            ShipmentEntity a = cvsShipment();
            ShipmentEntity b = cvsShipment();
            b.setGoodsType("3");   // 與 a 的溫層不同

            LabelCapability cap = registry.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                    .label(FulfillmentContext.forShipment(a)).orElseThrow();

            assertThatThrownBy(() -> cap.requestLabelBatch(List.of(a, b),
                    LabelCapability.LabelRequest.onShipDate("20260910")))
                    .isInstanceOf(IllegalArgumentException.class);
            verify(labelService, never()).triggerBatchCvsPrint(
                    anyString(), anyString(), any(), any(), any(), anyString());
        }

        /**
         * 🔴 <b>第二份 LgsType SSOT 已消滅</b>的直接證據：
         * 標籤識別回查現在打的是 adapter 組的請求，{@code LgsType} 與回查（track）同一個來源。
         * 遷移前這一段在 {@code ShipmentLabelReconcileScheduler.deriveLgsType}——
         * 那是一份會與 adapter 各自漂移的副本。
         */
        @Test
        @DisplayName("標籤識別回查：LgsType 來自 adapter；三段碼以中性 key→value 回傳")
        void fetchLabelIdentifiersUsesAdapterLgsTypeAndNeutralKeys() {
            ShipmentEntity entity = cvsShipment();
            entity.setLgsType("WRONG");
            when(shippingClient.queryShipment(anyString(), any())).thenReturn(
                    new QueryShipmentResponse(
                            new PayUniApiResponse("SUCCESS", "ok", Map.of(
                                    "PartnerId", "P1", "Odno", "O1", "ValidationNo", "V1")),
                            "ST1", "91", null, "MT1"));

            LabelCapability.LabelIdentifierLookup lookup =
                    registry.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                            .label(FulfillmentContext.forShipment(entity)).orElseThrow()
                            .fetchLabelIdentifiers(entity);

            ArgumentCaptor<QueryShipmentRequest> req =
                    ArgumentCaptor.forClass(QueryShipmentRequest.class);
            verify(shippingClient).queryShipment(eq("HAWK100"), req.capture());
            assertThat(req.getValue().lgsType())
                    .as("LgsType 只有 adapter 這一份來源")
                    .isEqualTo("C2C");

            assertThat(lookup.querySucceeded()).isTrue();
            assertThat(lookup.identifiersReady()).isTrue();
            assertThat(lookup.segmentCodes())
                    .containsEntry(LabelCapability.SEGMENT_PARTNER_ID, "P1")
                    .containsEntry(LabelCapability.SEGMENT_ODNO, "O1")
                    .containsEntry(LabelCapability.SEGMENT_VALIDATION_NO, "V1");
        }

        /**
         * 三態的鑑別力：「查詢失敗」與「查詢成功但還沒產生」處置完全不同
         * （前者是 provider 故障、後者是正常等待），壓成同一個訊號會讓補收排程分不出來。
         */
        @Test
        @DisplayName("三段碼缺一 ⇒ notReady（不是 queryFailed，也不是 ready）")
        void partialSegmentsAreNotReady() {
            ShipmentEntity entity = cvsShipment();
            when(shippingClient.queryShipment(anyString(), any())).thenReturn(
                    new QueryShipmentResponse(
                            new PayUniApiResponse("SUCCESS", "ok", Map.of(
                                    "PartnerId", "P1", "Odno", "-", "ValidationNo", "V1")),
                            "ST1", "91", null, "MT1"));

            LabelCapability.LabelIdentifierLookup lookup =
                    registry.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                            .label(FulfillmentContext.forShipment(entity)).orElseThrow()
                            .fetchLabelIdentifiers(entity);

            assertThat(lookup.querySucceeded())
                    .as("provider 有回應，只是還沒產生標籤——不是故障")
                    .isTrue();
            assertThat(lookup.identifiersReady()).isFalse();
            assertThat(lookup.segmentCodes()).isEmpty();
        }

        /**
         * 陰性面：宅配的查詢回應 by-design 不帶三段碼，
         * 但它<b>仍然要真的打一次查詢</b>——否則宅配單的 provider 故障永遠看不見。
         */
        @Test
        @DisplayName("宅配：仍然真的查一次（LgsType=HOME），但識別恆為 notReady")
        void homeStillQueriesButNeverReady() {
            ShipmentEntity entity = cvsShipment();
            entity.setShippingMethod(ShippingMethod.TAQBIN.value());
            when(shippingClient.queryShipment(anyString(), any())).thenReturn(
                    new QueryShipmentResponse(
                            new PayUniApiResponse("SUCCESS", "ok", Map.of()), "ST1", "91", null, "MT1"));

            LabelCapability.LabelIdentifierLookup lookup =
                    registry.require(ShippingMethod.TAQBIN.value())
                            .label(FulfillmentContext.forShipment(entity)).orElseThrow()
                            .fetchLabelIdentifiers(entity);

            ArgumentCaptor<QueryShipmentRequest> req =
                    ArgumentCaptor.forClass(QueryShipmentRequest.class);
            verify(shippingClient).queryShipment(anyString(), req.capture());
            assertThat(req.getValue().lgsType()).isEqualTo("HOME");
            assertThat(lookup.querySucceeded()).isTrue();
            assertThat(lookup.identifiersReady()).isFalse();
        }
    }

    // ==================================================================
    // ③ StoreLocatorCapability：門市地圖
    // ==================================================================

    @Nested
    @DisplayName("③ 門市地圖走 adapter")
    class StoreLocator {

        private StoreLocatorCapability capability() {
            return registry.require(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())
                    .storeLocator(FulfillmentContext.forStore(
                            0L, ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP))
                    .orElseThrow();
        }

        @Test
        @DisplayName("組參數：回中性 key→value，呼叫端不碰 provider 欄名")
        void buildRedirectParamsReturnsNeutralKeys() {
            when(storeMapService.buildStoreMapParams(eq(2600000001L), eq("buyer"), eq("Y")))
                    .thenReturn(new StoreMapService.StoreMapFormResult(
                            "https://provider/map", "HAWK1", "1.0", "ENC", "HASH", "https://gw/redirect?id=x"));

            Map<String, String> params = capability().buildRedirectParams(
                    new StoreLocatorCapability.StoreMapRequest(2600000001L, "buyer", true));

            assertThat(params)
                    .containsEntry(StoreLocatorCapability.PARAM_FORM_ACTION, "https://provider/map")
                    .containsEntry(StoreLocatorCapability.PARAM_PAYLOAD, "ENC")
                    .containsEntry(StoreLocatorCapability.PARAM_SIGNATURE, "HASH")
                    .containsEntry(StoreLocatorCapability.PARAM_REDIRECT_URL, "https://gw/redirect?id=x");
        }

        @Test
        @DisplayName("解析成功：門市代碼、離島註記、導回出口都帶回")
        void parseResultCarriesSelection() {
            when(storeMapService.decryptStoreMapResult("ENC", "HASH", "buyer"))
                    .thenReturn(new StoreMapService.StoreMapDecryptResult(
                            "916712", "敦安門市", "台北市大安區", "I", "1", "hawkrbuyer://x"));

            StoreLocatorCapability.StoreSelection selection = capability().parseResult(Map.of(
                    StoreLocatorCapability.RESULT_PAYLOAD, "ENC",
                    StoreLocatorCapability.RESULT_SIGNATURE, "HASH",
                    StoreLocatorCapability.RESULT_CLIENT_KIND, "buyer"));

            assertThat(selection.storeCode()).isEqualTo("916712");
            assertThat(selection.insularArea()).isEqualTo("I");
            assertThat(selection.redirectUrl()).isEqualTo("hawkrbuyer://x");
        }

        /**
         * 🔴 <b>fail-closed 的那一面</b>。遷移前解不出 {@code MapJson} 只記一行 error log，
         * 然後帶著空門市代碼組出「選店成功」的導回網址——錯誤要到出貨當天才現形。
         */
        @Test
        @DisplayName("解不出門市代碼 ⇒ 丟例外，不猜門市")
        void parseResultFailsClosedOnMissingStoreCode() {
            when(storeMapService.decryptStoreMapResult("ENC", "HASH", "buyer"))
                    .thenReturn(new StoreMapService.StoreMapDecryptResult(
                            "", "", "", "", "", "hawkrbuyer://x"));

            assertThatThrownBy(() -> capability().parseResult(Map.of(
                    StoreLocatorCapability.RESULT_PAYLOAD, "ENC",
                    StoreLocatorCapability.RESULT_SIGNATURE, "HASH",
                    StoreLocatorCapability.RESULT_CLIENT_KIND, "buyer")))
                    .isInstanceOf(IllegalArgumentException.class);
        }

        /** 宅配沒有門市——這一格是永久 empty，不是「還沒搬」。 */
        @Test
        @DisplayName("宅配沒有門市地圖能力")
        void homeHasNoStoreLocator() {
            assertThat(registry.require(ShippingMethod.TAQBIN.value())
                    .storeLocator(FulfillmentContext.forStore(0L, ShippingMethod.TAQBIN)))
                    .isEmpty();
        }
    }

    private static PayUniApiResponse success() {
        return new PayUniApiResponse("SUCCESS", "success", Map.of());
    }

    private static ShipmentEntity cvsShipment() {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setShipmentNo("SH1");
        e.setOrderNo("O1");
        e.setStoreId(100L);
        e.setBuyerId(555L);
        e.setMerId("HAWK100");
        e.setShipTradeNo("ST1");
        e.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        e.setStatus(ShippingStatus.UNPROCESSED.value());
        e.setGoodsType("1");
        e.setShipType("UNIMART");
        return e;
    }
}

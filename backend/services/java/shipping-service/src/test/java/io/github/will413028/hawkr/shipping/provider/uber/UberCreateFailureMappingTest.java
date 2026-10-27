package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.order.grpc.OrderShippingInfo;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.entity.UberQuoteEntity;
import io.github.will413028.hawkr.shipping.entity.UberStoreMappingEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.mapper.UberQuoteMapper;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.capability.CreateCapability;
import io.github.will413028.hawkr.shipping.provider.create.UberShipmentStrategy;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.ManifestItem;
import io.github.will413028.hawkr.shipping.provider.uber.UberApiModels.UberWaypoint;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.util.List;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * 🔴 <b>建單失敗的 400 / 5xx 分流——66 陷阱 A8 的防線本身。</b>
 *
 * <p>66 §4.5.4 逐字：「把 5xx 當 990 會導致『其實建成功了但我們以為失敗』⇒ Admin 重建
 * ⇒ <b>兩趟車、重複計費</b>。」兩者的差別<b>不是</b>嚴重程度，是「對方那邊到底有沒有產生副作用」：
 * <ul>
 *   <li>400 ＝ 請求被看懂了並拒絕 ⇒ <b>確定沒有建出單</b> ⇒ {@code 990 CREATE_FAILED}</li>
 *   <li>5xx／timeout ＝ <b>不知道</b>對方收到了沒 ⇒ {@code 991 CREATE_UNKNOWN} ＋ 回查收斂</li>
 * </ul>
 *
 * <p>⚠️ 每一格各一支，<b>不合併</b>：合成一支參數化測試之後，把某一格改錯只會讓
 * 「一支測試的其中一格」紅，而 surefire 報的是方法名——那會讓下一個人以為是同一個問題。
 */
@DisplayName("Uber 建單失敗分流（A8）")
class UberCreateFailureMappingTest {

    private static final String ORDER_NO = "O-UBER-A8";
    private static final String BASE_URL = "https://api.uber.test/v1";
    private static final String CUSTOMER_ID = "cus_test";

    @Nested
    @DisplayName("① provider 回應 → shipments.status")
    class CreateOutcome {

        @Test
        @DisplayName("🔴 400（其他）⇒ 990 CREATE_FAILED（確定性：重試同一 payload 只會再錯一次）")
        void deterministic400BecomesCreateFailed() {
            ShipmentEntity e = createWith(new FakeUberTransport()
                    .respondWith(400, "{\"code\":\"invalid_params\",\"message\":\"bad address\"}"));

            assertThat(e.getStatus()).isEqualTo(ShippingStatus.CREATE_FAILED.value());
            assertThat(e.getProviderErrorCode()).isEqualTo("invalid_params");
            assertThat(e.getUberDeliveryId()).isNull();
        }

        @Test
        @DisplayName("🔴 5xx ⇒ 991 CREATE_UNKNOWN，**不是** 990（可能已建成功，重建＝兩趟車）")
        void serverErrorBecomesCreateUnknown() {
            ShipmentEntity e = createWith(new FakeUberTransport()
                    .respondWith(503, "{\"code\":\"unavailable\"}"));

            assertThat(e.getStatus())
                    .as("🔴 這一格寫成 990 就是陷阱 A8：Admin 會重建，而 Uber 那邊可能已經有一張單")
                    .isEqualTo(ShippingStatus.CREATE_UNKNOWN.value());
            assertThat(e.getStatus()).isNotEqualTo(ShippingStatus.CREATE_FAILED.value());
        }

        @Test
        @DisplayName("🔴 timeout ⇒ 991 CREATE_UNKNOWN（逾時是非確定性：請求可能已經到達）")
        void timeoutBecomesCreateUnknown() {
            ShipmentEntity e = createWith(new FakeUberTransport().failWithTimeout());

            assertThat(e.getStatus()).isEqualTo(ShippingStatus.CREATE_UNKNOWN.value());
            assertThat(e.getStatus()).isNotEqualTo(ShippingStatus.CREATE_FAILED.value());
        }

        @Test
        @DisplayName("customer_blocked 403 ⇒ 990 ＋ 原始 code 落地（呼叫端據此標門市＋通知賣家）")
        void customerBlockedBecomesCreateFailedWithProviderCode() {
            ShipmentEntity e = createWith(new FakeUberTransport()
                    .respondWith(403, "{\"code\":\"customer_blocked\"}"));

            assertThat(e.getStatus()).isEqualTo(ShippingStatus.CREATE_FAILED.value());
            assertThat(e.getProviderErrorCode())
                    .as("N6 事後反推需要分辨得出「這個門市被封鎖」與「我方憑證有問題」")
                    .isEqualTo("customer_blocked");
        }

        @Test
        @DisplayName("201 ⇒ 91 UNPROCESSED ＋ delivery_id 落地；provider_status 留給 canonical ingress")
        void successBecomesUnprocessed() {
            ShipmentEntity e = createWith(new FakeUberTransport()
                    .respondWith(201, "{\"id\":\"del_abc\",\"status\":\"pending\"}"));

            assertThat(e.getStatus()).isEqualTo(ShippingStatus.UNPROCESSED.value());
            assertThat(e.getUberDeliveryId()).isEqualTo("del_abc");
            // 🔴 **建單成功不得寫 provider_status**（V12 的 C4 判準，static gate
            //    check-shipment-provider-status-semantics.py 擋）。
            //
            //    本斷言原本是 `isEqualTo("pending")`，2026-09-12 改判：
            //    `shipments.provider_status` 只有一條寫入路徑（mapper 的 updateProviderStatus），
            //    它同時是「未知 provider 原生值」統一 fail-closed 的落點。
            //    在建單這一格用 entity setter 塞一份等於開第二條寫入路徑，
            //    而第二條路徑**不會**經過未知值處置。
            //    ⚠️ 不寫沒有遺失資訊：`pending` 映射到的正是上一行斷言的 91，
            //    真正的原生值由 webhook／T-8 回查經 canonical ingress 補上。
            assertThat(e.getProviderStatus())
                    .as("建單路徑不得開第二條 provider_status 寫入路徑")
                    .isNull();
        }

        @Test
        @DisplayName("🔴 401（憑證問題）⇒ 拋例外，**不寫 990／991**：請求根本沒送出去")
        void authFailureThrowsInsteadOfWritingATerminalish() {
            // 寫 990 ＝ 因為一次暫時性認證失敗就把買家的錢退掉；
            // 寫 991 ＝ 讓回查排程去找一個本來就不存在的單，白跑三天才落 990。
            FakeUberTransport transport = new FakeUberTransport().tokenFailsAfter(0);

            assertThatThrownBy(() -> createWith(transport))
                    .isInstanceOf(UberApiException.class);
            assertThat(transport.businessCalls())
                    .as("token 拿不到就不該有任何業務呼叫送出去")
                    .isEmpty();
        }

        @Test
        @DisplayName("🔴 沒有已被結帳確認的報價 ⇒ 990（確定性：重試一百次都一樣）")
        void missingAcceptedQuoteIsDeterministic() {
            ShipmentEntity e = create(new FakeUberTransport()
                    .respondWith(201, "{\"id\":\"del_x\"}"), null);

            assertThat(e.getStatus()).isEqualTo(ShippingStatus.CREATE_FAILED.value());
            assertThat(e.getProviderErrorCode()).isEqualTo("UBER_QUOTE_NOT_ACCEPTED");
        }

        @Test
        @DisplayName("🔴 查無可用門市對應 ⇒ 990 ＋ 可辨識的 code，**不是**拋例外進 DLQ")
        void missingStoreMappingBecomesCreateFailed() {
            // 拋例外的話：買家付了錢、連一列 shipment 都沒有、訂單卡住，而 DLQ 沒有人盯。
            // 990 有既有出口（Admin 重建／Admin 取消＋退款鏈），且 Admin 看得到原因。
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(201, "{\"id\":\"del_x\"}");
            UberStoreResolver empty = mock(UberStoreResolver.class);
            when(empty.activeMapping(anyLong())).thenReturn(Optional.empty());

            ShipmentEntity e = createCapability(adapter(transport, acceptedQuote())
                    .storeResolver(empty).build())
                    .createFromOrder(ORDER_NO, orderInfo(), null);

            assertThat(e.getStatus()).isEqualTo(ShippingStatus.CREATE_FAILED.value());
            assertThat(e.getProviderErrorCode()).isEqualTo("UBER_STORE_NOT_MAPPED");
            assertThat(transport.businessCalls())
                    .as("沒有門市對應就沒有 customer_id，不得帶著猜出來的值去打 Uber")
                    .isEmpty();
        }

        @Test
        @DisplayName("🔴 總開關關著 ⇒ fail-closed，一個位元組都不送出去")
        void disabledProviderNeverHitsTheNetwork() {
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(201, "{\"id\":\"del_x\"}");
            UberDirectAdapter adapter = adapter(transport, acceptedQuote())
                    .availability(false, true).build();

            assertThatThrownBy(() -> createCapability(adapter)
                    .createFromOrder(ORDER_NO, orderInfo(), null))
                    .isInstanceOf(ShippingBusinessException.class);
            assertThat(transport.calls()).isEmpty();
        }
    }

    @Nested
    @DisplayName("② 重試：確定性不重試、非確定性才重試")
    class RetryPolicy {

        @Test
        @DisplayName("🔴 400 只送一次（重試有副作用的呼叫 ＝ 多一次建出第二張單的機會）")
        void deterministicFailureIsNotRetried() {
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(400, "{\"code\":\"invalid_params\"}");
            createWith(transport);

            assertThat(transport.businessCalls()).hasSize(1);
        }

        @Test
        @DisplayName("5xx 依 §0.3 重試 2 次（共 3 送），耗盡後才落 991")
        void nonDeterministicFailureIsRetriedTwice() {
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(503, "{\"code\":\"unavailable\"}");
            createWith(transport);

            assertThat(transport.businessCalls())
                    .as("66 §0.3：我方→Uber（Create）＝ 2 次／2s・6s")
                    .hasSize(3);
        }

        @Test
        @DisplayName("🔴 每一送都帶同一個我方 pre-commit 鍵（ADR-014：去重由我方 DB 保證，不依賴 Uber）")
        void everyAttemptCarriesTheSamePrecommitKey() {
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(503, "{\"code\":\"unavailable\"}");
            createWith(transport);

            assertThat(transport.businessCalls())
                    .allSatisfy(c -> assertThat(c.body())
                            .contains("\"external_id\":\"uber_create_delivery:" + ORDER_NO + ":forward\""));
        }

        @Test
        @DisplayName("E2 的 deadline 是 15s（66 §0.2 T-4／§4.5.2 端點表）")
        void createDeliveryUsesItsOwnDeadline() {
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(201, "{\"id\":\"del_x\"}");
            createWith(transport);

            assertThat(transport.businessCalls()).singleElement()
                    .satisfies(c -> assertThat(c.deadline()).isEqualTo(Duration.ofSeconds(15)));
        }
    }

    @Nested
    @DisplayName("③ UberApiClient 的分類本身")
    class Classification {

        @Test
        @DisplayName("noncancelable_delivery（400）不得與其他 400 混為一談（處置完全相反）")
        void noncancelableIsItsOwnKind() {
            UberApiClient client = client(new FakeUberTransport()
                    .respondWith(400, "{\"code\":\"noncancelable_delivery\"}"));

            assertThatThrownBy(() -> client.cancelDelivery(CUSTOMER_ID, "del_1"))
                    .isInstanceOf(UberApiException.class)
                    .satisfies(e -> assertThat(((UberApiException) e).kind())
                            .as("其他 400 會寫 990（停止履約＋全額退款），"
                                    + "而這一格的正確處置是什麼都不寫——貨還在送")
                            .isEqualTo(UberFailureKind.NONCANCELABLE));
        }

        @Test
        @DisplayName("404 自成一類（66 §7.7「查到不存在 → 990」；陷阱 B2 的訊號）")
        void notFoundIsItsOwnKind() {
            UberApiClient client = client(new FakeUberTransport().respondWith(404, "{}"));

            assertThatThrownBy(() -> client.getDelivery(CUSTOMER_ID, "del_1"))
                    .isInstanceOf(UberApiException.class)
                    .satisfies(e -> assertThat(((UberApiException) e).kind())
                            .isEqualTo(UberFailureKind.NOT_FOUND));
        }

        @Test
        @DisplayName("429 可重試、400 不可重試（retryable 的定義就是「值不值得再送一次」）")
        void retryabilityMatchesTheKind() {
            assertThat(UberFailureKind.NON_DETERMINISTIC.retryable()).isTrue();
            assertThat(UberFailureKind.RATE_LIMITED.retryable()).isTrue();
            assertThat(UberFailureKind.DETERMINISTIC.retryable()).isFalse();
            assertThat(UberFailureKind.NONCANCELABLE.retryable()).isFalse();
            assertThat(UberFailureKind.CUSTOMER_BLOCKED.retryable()).isFalse();
            assertThat(UberFailureKind.AUTH.retryable()).isFalse();
            assertThat(UberFailureKind.NOT_FOUND.retryable()).isFalse();
        }
    }

    // ==================== fixtures ====================

    private static ShipmentEntity createWith(FakeUberTransport transport) {
        return create(transport, acceptedQuote());
    }

    private static ShipmentEntity create(FakeUberTransport transport, UberQuoteEntity quote) {
        return createCapability(adapter(transport, quote).build())
                .createFromOrder(ORDER_NO, orderInfo(), null);
    }

    private static CreateCapability createCapability(UberDirectAdapter adapter) {
        return adapter.create(FulfillmentContext.forStore(2685914682L, ShippingMethod.UBER_INSTANT))
                .orElseThrow();
    }

    private static TestUberAdapters.Builder adapter(FakeUberTransport transport, UberQuoteEntity quote) {
        UberShipmentStrategy strategy = mock(UberShipmentStrategy.class);
        when(strategy.createShipmentFromOrder(anyString(), any(), any()))
                .thenAnswer(inv -> freshShipment());

        UberStoreResolver storeResolver = mock(UberStoreResolver.class);
        when(storeResolver.activeMapping(anyLong())).thenReturn(Optional.of(mapping()));

        UberQuoteMapper quoteMapper = mock(UberQuoteMapper.class);
        when(quoteMapper.selectList(any()))
                .thenReturn(quote == null ? List.of() : List.of(quote));

        return TestUberAdapters.builder()
                .strategy(strategy)
                .storeResolver(storeResolver)
                .quoteMapper(quoteMapper)
                .apiClient(client(transport));
    }

    /** 🔴 sleeper 換成 no-op：退避階梯是 2s／6s，真的睡會讓這支測試跑 8 秒。 */
    private static UberApiClient client(FakeUberTransport transport) {
        ObjectMapper mapper = new ObjectMapper();
        UberOAuthTokenProvider tokens = new UberOAuthTokenProvider(
                transport, mapper, TestUberAdapters.FIXED_CLOCK,
                "https://auth.uber.test/oauth/v2/token", "cid", "secret", "eats.deliveries");
        return new UberApiClient(transport, tokens, mapper, BASE_URL, millis -> { });
    }

    private static ShipmentEntity freshShipment() {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setOrderNo(ORDER_NO);
        e.setShipmentNo("S-UBER-A8");
        e.setStoreId(2685914682L);
        e.setBuyerId(2623900520L);
        e.setShippingMethod(ShippingMethod.UBER_INSTANT.value());
        e.setStatus(ShippingStatus.UNPROCESSED.value());
        e.setIsReturn(false);
        return e;
    }

    private static UberStoreMappingEntity mapping() {
        UberStoreMappingEntity m = new UberStoreMappingEntity();
        m.setId(1L);
        m.setSellerStoreId(2685914682L);
        m.setExternalStoreId("store_ext_1");
        m.setCustomerId(CUSTOMER_ID);
        m.setPrepSlaMinutes(15);
        m.setActive(true);
        m.setBlocked(false);
        m.setHealthStatus("HEALTHY");
        return m;
    }

    /**
     * 🔴 pickup／dropoff／items 三欄用 {@link UberPayloadCodec} 產生，不手寫 JSON 字面值。
     *
     * <p>那三欄是 JSONB、schema 沒有規定欄名；兩邊各自手寫「我覺得對方會寫成這樣」
     * 就是自己跟自己對。用同一支 codec 讀寫，欄名改了兩邊會一起改。
     */
    private static UberQuoteEntity acceptedQuote() {
        UberPayloadCodec codec = new UberPayloadCodec(new ObjectMapper());
        UberQuoteEntity q = new UberQuoteEntity();
        q.setId(1L);
        q.setQuoteId("dqt_test");
        q.setOrderNo(ORDER_NO);
        q.setStatus("CHECKOUT_ACCEPTED");
        q.setExternalStoreId("store_ext_1");
        q.setPickupJson(codec.waypointToJson(new UberWaypoint(
                "{\"street_address\":[\"台北市信義區信義路五段1號\"],\"city\":\"台北市\","
                        + "\"state\":\"台北市\",\"zip_code\":\"110\",\"country\":\"TW\"}",
                "範例生鮮測試店", "0912345678", 25.0330, 121.5654, null)));
        q.setDropoffJson(codec.waypointToJson(new UberWaypoint(
                "{\"street_address\":[\"台北市中正區重慶南路一段1號\"],\"city\":\"台北市\","
                        + "\"state\":\"台北市\",\"zip_code\":\"100\",\"country\":\"TW\"}",
                "測試買家", "0987654321", 25.0421, 121.5116, "3 樓")));
        q.setItemsJson(codec.itemsToJson(List.of(
                new ManifestItem("測試商品", 1, "small", 300L, "sku_1"))));
        return q;
    }

    private static OrderShippingInfo orderInfo() {
        return OrderShippingInfo.newBuilder()
                .setStoreId("2685914682")
                .setBuyerId("2623900520")
                .setTotalActualAmount("500")
                .setShippingMethod(ShippingMethod.UBER_INSTANT.value())
                .build();
    }
}

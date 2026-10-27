package io.github.will413028.hawkr.shipping.provider.uber;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.entity.UberStoreMappingEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.capability.CancelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.PodCapability;
import io.github.will413028.hawkr.shipping.provider.capability.TrackCapability;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * cancel／track／pod 三個 capability 的行為（66 §4.5.0 E4／E6／E7）。
 */
@DisplayName("Uber capability 行為")
class UberCapabilityBehaviourTest {

    private static final String CUSTOMER_ID = "cus_test";
    private static final String DELIVERY_ID = "del_test";
    private static final String BASE_URL = "https://api.uber.test/v1";

    // ==================================================================
    // cancel（E6）—— 四個 provider 裡唯一真的打 provider 的取消
    // ==================================================================

    @Nested
    @DisplayName("cancel（E6）")
    class Cancel {

        @Test
        @DisplayName("成功 ⇒ accepted ＋ terminal（呼叫端才落得了 41）")
        void successIsTerminal() {
            CancelCapability.CancelOutcome outcome = cancel(new FakeUberTransport()
                    .respondWith(200, "{\"id\":\"" + DELIVERY_ID + "\",\"status\":\"canceled\"}"));

            assertThat(outcome.accepted()).isTrue();
            assertThat(outcome.terminal())
                    .as("66 §4.5.0 E6：寫 992 → 打 E6 → 成功 41")
                    .isTrue();
        }

        @Test
        @DisplayName("🔴 noncancelable_delivery ⇒ 9522，**且不得**回一個會讓呼叫端落終態的 outcome")
        void noncancelableNeverYieldsATerminalOutcome() {
            // 66 §7.6 逐字：「不得直接寫 41——它是 T1 終態，provider 未確認前寫入
            // ＝ 對買家宣告取消而貨照送」。
            assertThatThrownBy(() -> cancel(new FakeUberTransport()
                    .respondWith(400, "{\"code\":\"noncancelable_delivery\"}")))
                    .isInstanceOf(ShippingBusinessException.class)
                    .satisfies(e -> assertThat(((ShippingBusinessException) e).code())
                            .isEqualTo(GrpcResponseCode.SHIPMENT_CANCEL_NOT_CANCELLABLE.code()));
        }

        @Test
        @DisplayName("沒有 delivery_id ⇒ 擋下，provider 不被呼叫")
        void withoutDeliveryIdProviderIsNotCalled() {
            FakeUberTransport transport = new FakeUberTransport().respondWith(200, "{}");
            ShipmentEntity e = shipment();
            e.setUberDeliveryId(null);

            assertThatThrownBy(() -> capability(transport).cancel(ctx()).orElseThrow()
                    .cancel(e, "seller"))
                    .isInstanceOf(ShippingBusinessException.class);
            assertThat(transport.businessCalls()).isEmpty();
        }

        @Test
        @DisplayName("E6 的 deadline 是 15s（有副作用，66 §4.5.2）")
        void deadlineIs15s() {
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(200, "{\"status\":\"canceled\"}");
            cancel(transport);

            assertThat(transport.businessCalls()).singleElement()
                    .satisfies(c -> assertThat(c.deadline()).isEqualTo(Duration.ofSeconds(15)));
        }

        private CancelCapability.CancelOutcome cancel(FakeUberTransport transport) {
            return capability(transport).cancel(ctx()).orElseThrow().cancel(shipment(), "seller");
        }
    }

    // ==================================================================
    // track（E4）
    // ==================================================================

    @Nested
    @DisplayName("track（E4）")
    class Track {

        @Test
        @DisplayName("🔴 沒有 delivery_id ⇒ **不打 E4**（陷阱 B2：每次 404、永不收斂）")
        void noDeliveryIdMeansNoE4Call() {
            FakeUberTransport transport = new FakeUberTransport().respondWith(404, "{}");
            ShipmentEntity e = shipment();
            e.setUberDeliveryId(null);
            e.setStatus(ShippingStatus.CREATE_UNKNOWN.value());

            TrackCapability.TrackResult r = capability(transport).track(ctx()).orElseThrow().query(e);

            assertThat(transport.businessCalls())
                    .as("991 的收斂走 E3 ＋ 我方 external_id 反查，不是 E4")
                    .isEmpty();
            assertThat(r.normalizedStatus()).isNull();
            assertThat(r.providerRawStatus()).isNull();
        }

        @Test
        @DisplayName("已知狀態 ⇒ 正規化；外送員電話一律遮罩後才落任何欄位")
        void knownStatusIsNormalizedAndPhoneIsMasked() {
            TrackCapability.TrackResult r = track(new FakeUberTransport().respondWith(200,
                    "{\"id\":\"" + DELIVERY_ID + "\",\"status\":\"pickup_complete\","
                            + "\"courier\":{\"name\":\"Ku\",\"phone_number\":\"+886912345678\"}}"));

            // 🔴 硬寫 "930"：期望值跟著 ShippingStatus 一起變的話這條就驗不到東西
            assertThat(r.normalizedStatus()).isEqualTo("930");
            assertThat(r.providerRawStatus()).isEqualTo("pickup_complete");
            assertThat(r.providerMessage())
                    .as("🔴 外送員電話是可撥打的臨時號碼（66 §8.6）⇒ 原值不落庫不落 log")
                    .doesNotContain("912345678")
                    .endsWith("5678");
        }

        @Test
        @DisplayName("🔴 未知狀態 ⇒ normalized 為 null、raw 照存（fail-closed，不猜）")
        void unknownStatusKeepsRaw() {
            TrackCapability.TrackResult r = track(new FakeUberTransport().respondWith(200,
                    "{\"id\":\"" + DELIVERY_ID + "\",\"status\":\"teleported\"}"));

            assertThat(r.normalizedStatus()).isNull();
            assertThat(r.providerRawStatus()).isEqualTo("teleported");
        }

        @Test
        @DisplayName("E4 的 deadline 是 10s，且走的是 delivery_id 的路徑")
        void deadlineAndPath() {
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(200, "{\"status\":\"dropoff\"}");
            track(transport);

            assertThat(transport.businessCalls()).singleElement().satisfies(c -> {
                assertThat(c.deadline()).isEqualTo(Duration.ofSeconds(10));
                assertThat(c.method()).isEqualTo("GET");
                assertThat(c.url()).endsWith("/customers/" + CUSTOMER_ID + "/deliveries/" + DELIVERY_ID);
            });
        }

        private TrackCapability.TrackResult track(FakeUberTransport transport) {
            return capability(transport).track(ctx()).orElseThrow().query(shipment());
        }
    }

    // ==================================================================
    // pod（E7）
    // ==================================================================

    @Nested
    @DisplayName("pod（E7）")
    class Pod {

        @Test
        @DisplayName("抓到 ⇒ 轉存我方儲存並只回 objectKey（禁存完整 URL／presigned URL）")
        void podIsPersistedAndOnlyObjectKeyIsReturned() {
            UberPodStorage storage = mock(UberPodStorage.class);
            when(storage.store(anyString(), any(), anyString())).thenReturn("secure/delivery_pod/abc.jpg");

            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(200, "{\"document\":\"aGVsbG8=\"}");

            Optional<PodCapability.PodResult> r = builder(transport).podStorage(storage).build()
                    .pod(ctx()).orElseThrow().fetchPod(shipment());

            assertThat(r).isPresent();
            assertThat(r.orElseThrow().imageObjectKey()).isEqualTo("secure/delivery_pod/abc.jpg");
        }

        @Test
        @DisplayName("404（窗口未到／沒有這一種 POD）⇒ empty，不是例外")
        void notFoundYieldsEmpty() {
            // 66 §7.8：leave_at_door 只有 picture、需簽收才有 signature
            // ⇒ INV-U5 是「≥1 筆」而不是「三種都要有」。
            Optional<PodCapability.PodResult> r = capability(new FakeUberTransport()
                    .respondWith(404, "{}")).pod(ctx()).orElseThrow().fetchPod(shipment());

            assertThat(r).isEmpty();
        }

        @Test
        @DisplayName("保留窗口是 7 天（T-12 取 U-F 照片 7 天與 U-G 簽名 30 天的最短者）")
        void retentionWindowIsSevenDays() {
            // ⚠️ 硬寫 168：拿 30 天當計畫的後果是爭議發生時照片剛好在第 7 天過期，
            //    而我們在那天才發現（66 §0.5）。
            assertThat(capability(new FakeUberTransport()).pod(ctx()).orElseThrow()
                    .retentionWindowHours()).isEqualTo(168);
        }

        @Test
        @DisplayName("E7 的 deadline 是 20s（二進位，66 §4.5.2）")
        void deadlineIs20s() {
            UberPodStorage storage = mock(UberPodStorage.class);
            when(storage.store(anyString(), any(), anyString())).thenReturn("secure/x.jpg");
            FakeUberTransport transport = new FakeUberTransport()
                    .respondWith(200, "{\"document\":\"aGVsbG8=\"}");

            builder(transport).podStorage(storage).build().pod(ctx()).orElseThrow()
                    .fetchPod(shipment());

            assertThat(transport.businessCalls()).singleElement()
                    .satisfies(c -> assertThat(c.deadline()).isEqualTo(Duration.ofSeconds(20)));
        }
    }

    // ==================================================================
    // onboard —— 沒有 provider 端點，回的是 uber_store_mapping 的狀態
    // ==================================================================

    @Test
    @DisplayName("🔴 onboard：Uber 沒有建店槽位 ⇒ shippingTypeValue() 為 null（不得硬塞既有值）")
    void onboardHasNoShippingTypeSlot() {
        // 硬塞一個既有 ShippingType 會讓 Uber 的建單去查別條線的 seller_shipment_settings，
        // 而 buildSlotKey 的鍵一撞就是靜默取到別人的設定。
        assertThat(capability(new FakeUberTransport()).onboard(ctx()).orElseThrow()
                .shippingTypeValue()).isNull();
    }

    @Test
    @DisplayName("onboard：回的是門市對應狀態，且 external_store_id 遮罩（憑證等級的值）")
    void onboardReportsMaskedMappingStatus() {
        io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity setting =
                new io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity();
        setting.setStoreId(2685914682L);

        var outcome = capability(new FakeUberTransport()).onboard(ctx()).orElseThrow().submit(setting);

        assertThat(outcome.submitStatus()).isEqualTo("HEALTHY");
        assertThat(outcome.providerReference())
                .as("66 §8.6：external_store_id 洩漏 ＋ token 洩漏 ＝ 可代賣家叫車")
                .doesNotContain("store_ext_1");
    }

    // ==================== fixtures ====================

    private static UberDirectAdapter capability(FakeUberTransport transport) {
        return builder(transport).build();
    }

    private static TestUberAdapters.Builder builder(FakeUberTransport transport) {
        UberStoreResolver storeResolver = mock(UberStoreResolver.class);
        when(storeResolver.activeMapping(anyLong())).thenReturn(Optional.of(mapping()));

        ObjectMapper mapper = new ObjectMapper();
        UberOAuthTokenProvider tokens = new UberOAuthTokenProvider(
                transport, mapper, TestUberAdapters.FIXED_CLOCK,
                "https://auth.uber.test/oauth/v2/token", "cid", "secret", "eats.deliveries");

        return TestUberAdapters.builder()
                .storeResolver(storeResolver)
                .apiClient(new UberApiClient(transport, tokens, mapper, BASE_URL, millis -> { }));
    }

    private static ShipmentEntity shipment() {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setOrderNo("O-UBER-CAP");
        e.setShipmentNo("S-UBER-CAP");
        e.setStoreId(2685914682L);
        e.setShippingMethod(ShippingMethod.UBER_INSTANT.value());
        e.setStatus(ShippingStatus.HANDED_OVER.value());
        e.setUberDeliveryId(DELIVERY_ID);
        e.setIsReturn(false);
        return e;
    }

    private static FulfillmentContext ctx() {
        return FulfillmentContext.forShipment(shipment());
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
}

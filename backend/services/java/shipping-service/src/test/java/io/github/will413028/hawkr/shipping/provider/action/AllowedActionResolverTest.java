package io.github.will413028.hawkr.shipping.provider.action;

import io.github.will413028.hawkr.common.utils.code.FulfillmentAction;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders;
import io.github.will413028.hawkr.shipping.service.OnboardingService;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

/**
 * {@code allowedActions[]} 的計算。
 *
 * <h2>🔴 為什麼用<b>真的</b> {@link ShipmentStatusMapper} 與<b>真的</b> adapter</h2>
 * 本類別的價值全在「它問的是不是真的那兩層」。餵 stub 的話，斷言驗的只是
 * 「resolver 有沒有照著我寫的 stub 走」——而 stub 的 capability 集合與狀態機規則
 * 正是待驗的那件事（自己跟自己對）。裝真貨之後，改壞
 * {@code UberDirectAdapter.track()} 或 {@code ShipmentStatusMapper.isValidTransition}
 * 這種變異才會讓本檔轉紅。
 */
@DisplayName("allowedActions：兩層模型的計算")
class AllowedActionResolverTest {

    private AllowedActionResolver resolver;

    @BeforeEach
    void setUp() {
        resolver = new AllowedActionResolver(
                TestFulfillmentProviders.realRegistry(
                        mock(ShippingClient.class),
                        new ShipmentStatusMapper(),
                        mock(OnboardingService.class)),
                new ShipmentStatusMapper());
    }

    private Map<String, ActionAvailability> resolve(ShipmentEntity shipment) {
        return resolver.resolve(shipment).stream()
                .collect(Collectors.toMap(ActionAvailability::action, Function.identity()));
    }

    // ==================================================================
    // ① 陽性：狀態允許 ＋ capability present ⇒ enabled
    // ==================================================================

    @Nested
    @DisplayName("① 狀態允許且 capability present ⇒ enabled")
    class Positive {

        @Test
        @DisplayName("超商 91 UNPROCESSED＋已取號：print_label／track／handover 都可用")
        void cvsFreshShipmentHasActions() {
            Map<String, ActionAvailability> actions = resolve(cvs(
                    ShippingStatus.UNPROCESSED, LabelPrintStatus.SUCCEEDED));

            assertThat(actions.get(FulfillmentAction.PRINT_LABEL.value()).enabled())
                    .as("已取號仍可重印（冪等取回代碼）")
                    .isTrue();
            assertThat(actions.get(FulfillmentAction.TRACK.value()).enabled())
                    .as("超商有 TrackCapability")
                    .isTrue();
            assertThat(actions.get(FulfillmentAction.HANDOVER.value()).enabled())
                    .as("標籤已就緒且 91 → 921 合法")
                    .isTrue();
            assertThat(actions.get(FulfillmentAction.VIEW_ONLY.value()).enabled()).isTrue();
        }

        /**
         * 🔴 這一條盯的是「這個 provider 有沒有派車能力」。
         *
         * <p>黑貓叫件（{@code callTcat}）在 production 今天就在運作，實作已於本波搬入
         * {@code PayUniHomeAdapter.dispatch()}。若它回 {@code empty}（＝遷移前的寫法），
         * 本條會算出「叫件不可用 ＋ 此配送服務尚未開放」，
         * 賣家看到的是一顆按不下去的按鈕，而那句話是假的。
         *
         * <p>⚠️ 與下一條是同一個判斷點的兩面：只留這一條，把 capability 閘門整個拿掉也會綠。
         */
        @Test
        @DisplayName("黑貓 91 UNPROCESSED：dispatch 可用")
        void taqbinCanDispatch() {
            ShipmentEntity e = cvs(ShippingStatus.UNPROCESSED, LabelPrintStatus.SUCCEEDED);
            e.setShippingMethod(ShippingMethod.TAQBIN.value());

            assertThat(resolve(e).get(FulfillmentAction.DISPATCH.value()).enabled()).isTrue();
        }

        /**
         * 🔴 <b>Uber 的 dispatch 只在 {@code 990 CREATE_FAILED} 時可用</b>。
         *
         * <p>⚠️ 本條原本斷言的是「capability 真的 empty ⇒ 不可用（9503）」。W2-1 把
         * {@code UberDirectAdapter.dispatch()} 換成 E2 的真實實作之後，那個前提不再成立
         * ——而「Uber 在 91 不該能按重新建單」這件事本身沒變，所以改成直接斷言它，
         * 並且斷言<b>它是被狀態閘門擋下的</b>（9520）而不是被 capability 閘門（9503）。
         *
         * <p>三格一起驗，缺一都會被某一類變異放過：
         * <ul>
         *   <li>{@code 91} 不可用：少了它，allowed-from 放寬成「任何非終態」也會綠</li>
         *   <li>{@code 990} 可用：少了它，把整格寫死成 disabled 也會綠（而那會讓建單失敗的單<b>永遠救不回來</b>）</li>
         *   <li>🔴 {@code 991} 不可用：<b>本檔對陷阱 A8 唯一的防線</b>。{@code 991} 在
         *       {@code ShipmentStatusMapper} 裡屬 {@code PRE_SHIPMENT}
         *       ⇒ {@code isValidTransition(991, 91)} 為 true ⇒ 用「轉移合法性」推導會<b>放行</b>，
         *       而 {@code 991} 的語意正是「可能已經建成功了」⇒ 再建一次＝兩趟車、重複計費</li>
         * </ul>
         */
        @Test
        @DisplayName("Uber dispatch：只有 990 可按；91 與 🔴 991 都不可（A8 防線）")
        void uberDispatchOnlyFromCreateFailed() {
            ActionAvailability at91 = uberDispatchAt(ShippingStatus.UNPROCESSED);
            assertThat(at91.enabled()).isFalse();
            assertThat(at91.disabledReasonCode())
                    .as("被狀態閘門擋下，不是被 capability 閘門——後者的文案是「此配送服務尚未開放」，而 Uber 已經接上了")
                    .isEqualTo(GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code());

            assertThat(uberDispatchAt(ShippingStatus.CREATE_FAILED).enabled())
                    .as("990 是唯一 enabled 的那一格（賣家重新建單的入口）")
                    .isTrue();

            ActionAvailability at991 = uberDispatchAt(ShippingStatus.CREATE_UNKNOWN);
            assertThat(at991.enabled())
                    .as("🔴 991 CREATE_UNKNOWN 可能已經建成功了——重建就是陷阱 A8：兩趟車、重複計費。"
                            + "它的收斂手段是 T-8 回查，不是重建")
                    .isFalse();
            assertThat(at991.disabledReasonCode())
                    .isEqualTo(GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code());
        }

        /**
         * ⚠️ 刻意<b>不</b>設 {@code labelPrintStatus}：Uber 沒有標籤概念，
         * 而舊的 DISPATCH 閘門會要求「標籤已取號」。若顯式 allowed-from 沒有取代掉那一格，
         * 本 helper 造出來的單會因為 {@code SHIPMENT_LABEL_REQUIRED} 而不可用，
         * 於是 {@code 990} 那一條斷言會轉紅——這是刻意留的鑑別力。
         */
        private ActionAvailability uberDispatchAt(ShippingStatus status) {
            ShipmentEntity e = cvs(status, LabelPrintStatus.NONE);
            e.setShippingMethod(ShippingMethod.UBER_INSTANT.value());
            return resolve(e).get(FulfillmentAction.DISPATCH.value());
        }
    }

    // ==================================================================
    // ② 🔴 requiresProviderCall=false 的動作不因 capability 為 empty 而消失
    // ==================================================================

    @Nested
    @DisplayName("② requiresProviderCall=false 的動作不受 capability 影響")
    class ProviderCallGateOnlyAppliesWhenRequired {

        /**
         * 🔴 <b>本檔最重要的一條。</b>
         *
         * <p>Uber adapter 幾乎所有 capability 都是 empty。若 resolver 把
         * {@code Optional.empty()} 直接接到「按鈕不可用」，{@code handover} 與 {@code view_only}
         * 會整條消失——而它們是使用者最常按的那幾顆。
         *
         * <p>這一條與下一條是<b>同一個判斷點的兩面</b>：只留這一條，把 capability 閘門整個拿掉也會綠。
         */
        @Test
        @DisplayName("Uber（capability 幾乎全 empty）：handover／view_only 照樣可用")
        void actionsWithoutProviderCallSurviveEmptyCapabilities() {
            ShipmentEntity e = cvs(ShippingStatus.UNPROCESSED, LabelPrintStatus.SUCCEEDED);
            // 🔴 用 3004（7-11 大宗寄倉）而不是 3003（Uber）：
            //    設計書 §4.4.1a 的例子就是 68 的 handover（交倉寫 930，provider 端無 API），
            //    而 66 的 Uber 交付確認走 POD、不走賣家確認出貨
            //    ⇒ handover@3003 已明列為「這條線上不存在這個動作」。
            //    本測試要守的是「requiresProviderCall=false 不受 capability 為 empty 影響」，
            //    3004 同樣沒有註冊 adapter，論證力相同而不與那條明列衝突。
            e.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE.value());

            Map<String, ActionAvailability> actions = resolve(e);

            assertThat(FulfillmentActionPolicy.requiresProviderCall(
                    FulfillmentAction.HANDOVER, ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE))
                    .as("前提：handover 本來就不需要打 provider")
                    .isFalse();
            assertThat(actions.get(FulfillmentAction.HANDOVER.value()).enabled())
                    .as("capability 為 empty 不得讓 handover 消失")
                    .isTrue();
            assertThat(actions.get(FulfillmentAction.VIEW_ONLY.value()).enabled()).isTrue();
        }

        /**
         * 陰性面：capability 閘門<b>還在</b>。
         *
         * <p>⚠️ 本條原本用 Uber 的 {@code track} 當第二個例子。W2-1 之後 Uber 的
         * {@code track()} 是 E4 的真實實作 ⇒ 那個例子不再成立，改用兩個<b>今天仍然</b>
         * 為 empty 的判斷點：
         * <ul>
         *   <li>{@code print_label@3003}：{@code label()} 是<b>永久</b> empty
         *       （即時配送無標籤概念）——這一格不會隨施工波次改變</li>
         *   <li>{@code track@3004}：7-11 大宗寄倉<b>沒有註冊 adapter</b>
         *       ⇒ {@code hasCapabilityFor} 走「查無 adapter」那一支。
         *       少了它，把 {@code adapter.isEmpty()} 改成 {@code return true} 也會綠</li>
         * </ul>
         */
        @Test
        @DisplayName("capability 閘門還在：print_label@3003（永久 empty）與 track@3004（無 adapter）")
        void actionsRequiringProviderCallAreGated() {
            // ⚠️ labelPrintStatus 刻意用 NONE：SUCCEEDED 會先撞上狀態閘門的
            //    「已取號且不支援重印 ⇒ 5003」而根本走不到 capability 閘門
            //    ——那樣這條就驗不到它宣稱要驗的東西（實測：期望 9503、實際 5003）。
            ShipmentEntity uber = cvs(ShippingStatus.UNPROCESSED, LabelPrintStatus.NONE);
            uber.setShippingMethod(ShippingMethod.UBER_INSTANT.value());

            ActionAvailability printLabel = resolve(uber).get(FulfillmentAction.PRINT_LABEL.value());
            assertThat(printLabel.enabled())
                    .as("UberDirectAdapter.label() 是永久 empty（即時配送無標籤概念）")
                    .isFalse();
            assertThat(printLabel.disabledReasonCode())
                    .isEqualTo(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED.code());

            ShipmentEntity bulk = cvs(ShippingStatus.UNPROCESSED, LabelPrintStatus.SUCCEEDED);
            bulk.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE.value());

            ActionAvailability track = resolve(bulk).get(FulfillmentAction.TRACK.value());
            assertThat(track.enabled())
                    .as("3004 沒有註冊 adapter ⇒ 需要打 provider 的動作一律不可用")
                    .isFalse();
            assertThat(track.disabledReasonCode())
                    .isEqualTo(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED.code());
        }

        /**
         * 陽性面（W2-1 新增）：Uber 的 {@code track} 現在<b>是</b> present。
         *
         * <p>🔴 與上一條是同一個判斷點的兩面。少了這一條，把 {@code UberDirectAdapter.track()}
         * 改回 {@code Optional.empty()} 不會讓任何東西轉紅——而那會讓買家與賣家的
         * 「查詢配送進度」按鈕整條消失，症狀是靜默的。
         */
        @Test
        @DisplayName("Uber：track 可用（W2-1 把 E4 接上之後）")
        void uberTrackIsAvailableAfterW21() {
            ShipmentEntity e = cvs(ShippingStatus.IN_TRANSIT, LabelPrintStatus.NONE);
            e.setShippingMethod(ShippingMethod.UBER_INSTANT.value());

            assertThat(resolve(e).get(FulfillmentAction.TRACK.value()).enabled()).isTrue();
        }
    }

    // ==================================================================
    // ③ 狀態閘門
    // ==================================================================

    @Nested
    @DisplayName("③ 狀態閘門")
    class StateGate {

        @Test
        @DisplayName("終態（11 已取貨）：dispatch／handover 不可用，track／view_only 仍可用")
        void terminalBlocksMutations() {
            Map<String, ActionAvailability> actions = resolve(cvs(
                    ShippingStatus.PICKED_UP, LabelPrintStatus.SUCCEEDED));

            assertThat(actions.get(FulfillmentAction.DISPATCH.value()).enabled()).isFalse();
            assertThat(actions.get(FulfillmentAction.DISPATCH.value()).disabledReasonCode())
                    .isEqualTo(GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code());
            assertThat(actions.get(FulfillmentAction.HANDOVER.value()).enabled()).isFalse();
            assertThat(actions.get(FulfillmentAction.TRACK.value()).enabled())
                    .as("查詢是唯讀的：終態也要查得到最後的物流結果")
                    .isTrue();
            assertThat(actions.get(FulfillmentAction.VIEW_ONLY.value()).enabled()).isTrue();
        }

        @Test
        @DisplayName("尚未取得標籤 ⇒ handover 不可用（沒有東西可交寄）")
        void handoverNeedsLabel() {
            ActionAvailability handover = resolve(cvs(
                    ShippingStatus.UNPROCESSED, LabelPrintStatus.NONE))
                    .get(FulfillmentAction.HANDOVER.value());

            assertThat(handover.enabled()).isFalse();
            assertThat(handover.disabledReasonCode())
                    .isEqualTo(GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED.code());
        }

        /**
         * 列印走的是<b>列印狀態機</b>而不是配送狀態機：列印中再按一次會撞上
         * {@code acquireLabelPrintRequested} 的 CAS 而 rows=0，對使用者只表現成「按了沒反應」。
         */
        @Test
        @DisplayName("列印中（4102）⇒ print_label 不可用，理由是「處理中」不是「狀態不允許」")
        void printLabelBlockedWhileInFlight() {
            ActionAvailability print = resolve(cvs(
                    ShippingStatus.UNPROCESSED, LabelPrintStatus.REQUESTED))
                    .get(FulfillmentAction.PRINT_LABEL.value());

            assertThat(print.enabled()).isFalse();
            assertThat(print.disabledReasonCode())
                    .isEqualTo(GrpcResponseCode.SHIPMENT_PROVIDER_CALL_IN_PROGRESS.code());
        }

        @Test
        @DisplayName("尚無 provider 單號 ⇒ print_label 不可用（沒有東西可印）")
        void printLabelNeedsProviderTradeNo() {
            ShipmentEntity e = cvs(ShippingStatus.CREATE_FAILED, LabelPrintStatus.NONE);
            e.setShipTradeNo(null);

            assertThat(resolve(e).get(FulfillmentAction.PRINT_LABEL.value()).enabled()).isFalse();
        }
    }

    // ==================================================================
    // ④ 契約：enabled=false 一律帶得出理由；action 清單與 Gateway 對齊
    // ==================================================================

    @Nested
    @DisplayName("④ 對外契約")
    class Contract {

        /**
         * 🔴 「不可用要說明原因、不得憑空消失」是前端規格的硬要求，
         * 而 Gateway 對缺原因的項目是 <b>fail-closed 丟棄</b>（{@code BuildAllowedActions} 內的
         * {@code missing_reason}）⇒ 少一個原因碼＝那顆按鈕在前端整條不見。
         */
        @Test
        @DisplayName("跨狀態矩陣掃一遍：每個 enabled=false 都有 disabledReasonCode")
        void everyDisabledActionCarriesAReason() {
            int disabledSeen = 0;
            for (ShippingMethod method : List.of(ShippingMethod.TAQBIN,
                    ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, ShippingMethod.UBER_INSTANT)) {
                for (ShippingStatus status : ShippingStatus.values()) {
                    for (LabelPrintStatus print : LabelPrintStatus.values()) {
                        ShipmentEntity e = cvs(status, print);
                        e.setShippingMethod(method.value());
                        for (ActionAvailability a : resolver.resolve(e)) {
                            if (!a.enabled()) {
                                disabledSeen++;
                                assertThat(a.disabledReasonCode())
                                        .as(method.value() + "/" + status.value() + "/" + print.value()
                                                + " 的 " + a.action() + " 不可用卻沒有原因碼")
                                        .isNotNull();
                            }
                        }
                    }
                }
            }
            assertThat(disabledSeen)
                    .as("矩陣裡必須真的出現過 disabled，否則本條恆真、零鑑別力")
                    .isGreaterThan(0);
        }

        /**
         * 本波支援的 action 必須與 Gateway 的 {@code actionRoute} 對齊
         * （{@code gateway/internal/handler/fulfillment_actions.go}）：
         * <ul>
         *   <li><b>多算</b> ⇒ Gateway fail-closed 丟棄 ⇒ 「後端算了、前端看不到」，比不算更難查</li>
         *   <li><b>少算</b> ⇒ 那顆按鈕根本不會出現</li>
         * </ul>
         * ⚠️ 兩份清單分屬兩個 repo 的兩種語言，沒有共同的機械來源；這裡把 Java 這一份釘住，
         * 讓「順手加一個」必須先看到這條註解。
         */
        @Test
        @DisplayName("支援的 action 清單被釘住（與 Gateway 路由表對齊）")
        void supportedActionsArePinned() {
            assertThat(AllowedActionResolver.SUPPORTED.stream()
                    .map(FulfillmentAction::value).toList())
                    .containsExactly("dispatch", "handover", "print_label", "track", "view_only");
        }

        /**
         * 配送方式無法解析（髒資料／已退役的值）時的兜底。
         * 空清單會讓前端沒有任何可跟隨的 href；拋例外會讓整個訂單詳情打不開。
         */
        @Test
        @DisplayName("配送方式無法解析 ⇒ 只回 view_only（唯一允許的 fallback）")
        void unknownMethodFallsBackToViewOnly() {
            ShipmentEntity e = cvs(ShippingStatus.UNPROCESSED, LabelPrintStatus.SUCCEEDED);
            e.setShippingMethod("9999");

            List<ActionAvailability> actions = resolver.resolve(e);
            assertThat(actions).hasSize(1);
            assertThat(actions.get(0).action()).isEqualTo(FulfillmentAction.VIEW_ONLY.value());
            assertThat(actions.get(0).enabled()).isTrue();
        }
    }

    private static ShipmentEntity cvs(ShippingStatus status, LabelPrintStatus printStatus) {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setShipmentNo("SH1");
        e.setOrderNo("O1");
        e.setStoreId(100L);
        e.setBuyerId(555L);
        e.setMerId("HAWK100");
        e.setShipTradeNo("ST1");
        e.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        e.setStatus(status.value());
        e.setLabelPrintStatus(printStatus.value());
        return e;
    }
}

package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.utils.code.FulfillmentAction;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.TcatCallStatus;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders;
import io.github.will413028.hawkr.shipping.provider.action.ActionAvailability;
import io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * <b>BLOCKER 3 的守衛：「可以做」與「真的做」必須是同一套規則。</b>
 *
 * <h2>這支測試在守什麼</h2>
 * QA 在 dev 實測到兩個方向都壞：
 * <ul>
 *   <li>契約說 {@code handover enabled=false}（9527），而 {@code POST /shipments/&#123;no&#125;/confirm}
 *       回 200 並把 920 推到 921 —— 前端改由 {@code allowedActions} 驅動後，超商賣家永遠無法出貨</li>
 *   <li>{@code callTcat} 前置守衛<b>沒有任何 status 檢查</b>，於是已取貨（11）的單被推回已叫件（210）</li>
 * </ul>
 *
 * <h2>M4 變異</h2>
 * 拿掉 {@code ShippingService.confirmCvsShipment} 或 {@code callTcat} 裡的
 * {@code allowedActionResolver.require(...)} ⇒ 對應的 {@code rejectsWhatTheContractSaysIsDisabled}
 * 轉紅（那一支動作會變成 200／或走到更後面的 NPE，總之不再是 deterministic reject）。
 *
 * <h2>為什麼不用字串／結構斷言</h2>
 * 「有沒有呼叫 require()」用 grep 或 AST 都驗得到，但那驗的是<b>寫法</b>不是<b>行為</b>：
 * 抽一層 helper、換個名字、或把它放在 mutation 之後，字串斷言照樣綠。
 * 這裡改成問行為：<b>契約說不能做的，端點必須用同一個碼拒絕</b>。
 */
class CanonicalActionRuleParityTest {

    private FulfillmentProviderRegistry registry;
    private AllowedActionResolver resolver;

    @BeforeEach
    void setUp() {
        registry = TestFulfillmentProviders.realRegistry(
                mock(ShippingClient.class), new ShipmentStatusMapper(), mock(OnboardingService.class));
        resolver = new AllowedActionResolver(registry, new ShipmentStatusMapper());
    }

    // ==================== 讀寫共用（parity） ====================

    @Test
    @DisplayName("超商單在 920＋標籤成功時：契約說 handover 可用，require 也放行")
    void handoverIsAllowedExactlyWhenTheContractSaysSo() {
        ShipmentEntity ready = cvsShipment(ShippingStatus.CVS_CODE_RECEIVED, LabelPrintStatus.SUCCEEDED);

        assertThat(availability(ready, FulfillmentAction.HANDOVER).enabled())
                .as("920 + 4103 是唯一該按確認出貨的狀態")
                .isTrue();
        resolver.require(FulfillmentAction.HANDOVER, ready);   // 不得拋
    }

    @Test
    @DisplayName("契約說不可用時，執行面用同一個碼 deterministic reject（不是 silent no-op）")
    void rejectsWhatTheContractSaysIsDisabled() {
        // 標籤還沒成功 ⇒ 沒有交貨便代碼可交寄
        ShipmentEntity noLabel = cvsShipment(ShippingStatus.CVS_CODE_RECEIVED, LabelPrintStatus.NONE);

        ActionAvailability contract = availability(noLabel, FulfillmentAction.HANDOVER);
        assertThat(contract.enabled()).isFalse();

        // 🔴 期望值**硬寫字面碼**，不是 contract.disabledReasonCode()。
        //    resolve 與 require 共用同一支 evaluate ⇒ 拿 contract 的值當期望值恆真、零鑑別力
        //    （那是「期望值來自被測物」的形狀）。硬寫字面值才抓得到「兩邊一起改錯」。
        assertThat(contract.disabledReasonCode())
                .as("契約對『920 但標籤未成功』的碼")
                .isEqualTo(GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED.code());
        assertThatThrownBy(() -> resolver.require(FulfillmentAction.HANDOVER, noLabel))
                .isInstanceOf(ShippingBusinessException.class)
                .satisfies(e -> assertThat(((ShippingBusinessException) e).code())
                        .as("執行面回的碼必須就是契約給前端的 disabledReasonCode")
                        .isEqualTo(GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED.code()));
    }

    @Test
    @DisplayName("已取貨（終態）的單不得再被叫件推回 210——QA 在 dev 實測到的狀態倒退")
    void pickedUpShipmentCannotBeDispatchedBackwards() {
        ShipmentEntity delivered = homeShipment(ShippingStatus.PICKED_UP);

        assertThat(availability(delivered, FulfillmentAction.DISPATCH).enabled()).isFalse();
        assertThatThrownBy(() -> resolver.require(FulfillmentAction.DISPATCH, delivered))
                .isInstanceOf(ShippingBusinessException.class);
    }

    @Test
    @DisplayName("已取消（終態）的單不得再列印——契約與端點都要拒絕")
    void cancelledShipmentCannotPrintLabel() {
        // 🔴 PRINT_LABEL 的 stateGate 原本**完全不看配送狀態**：
        //    一張已取消、label_print_status=4104 的單，契約會算出 enabled=true，
        //    而 printCvsLabel 也沒有任何 status 檢查 ⇒ CAS 4104→4102 成功、真的打到 provider。
        ShipmentEntity cancelled = cvsShipment(ShippingStatus.CANCELLED, LabelPrintStatus.FAILED);

        ActionAvailability contract = availability(cancelled, FulfillmentAction.PRINT_LABEL);
        assertThat(contract.enabled())
                .as("終態不得再列印")
                .isFalse();
        assertThat(contract.disabledReasonCode())
                .isEqualTo(GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code());

        ShippingService service = serviceWith(cancelled);
        assertThatThrownBy(() -> service.printCvsLabel(
                cancelled.getStoreId(), cancelled.getOrderNo(), "2026-09-11"))
                .isInstanceOf(ShippingBusinessException.class)
                .satisfies(e -> assertThat(((ShippingBusinessException) e).code())
                        .as("端點回的碼必須就是契約的 disabledReasonCode（硬寫字面值，見上方說明）")
                        .isEqualTo(GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code()));
    }

    @Test
    @DisplayName("陰性對照：非終態且取號失敗的單，列印仍可用（證明上一支不是恆 false）")
    void nonTerminalFailedLabelCanStillPrint() {
        ShipmentEntity retryable = cvsShipment(ShippingStatus.UNPROCESSED, LabelPrintStatus.FAILED);
        assertThat(availability(retryable, FulfillmentAction.PRINT_LABEL).enabled()).isTrue();
    }

    // ==================== provider 維度（BLOCKER 2） ====================

    @Test
    @DisplayName("超商單不得有可按的 dispatch——它的 href 指向黑貓端點")
    void cvsShipmentHasNoEnabledDispatch() {
        ShipmentEntity cvs = cvsShipment(ShippingStatus.UNPROCESSED, LabelPrintStatus.SUCCEEDED);

        ActionAvailability dispatch = availability(cvs, FulfillmentAction.DISPATCH);
        assertThat(dispatch.enabled())
                .as("7-11 C2C 沒有派車 API；修復前這裡是 true，因為 dispatch() 回了一個會丟例外的 present 物件")
                .isFalse();
        assertThat(dispatch.disabledReasonCode())
                .isEqualTo(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED.code());
    }

    @Test
    @DisplayName("陰性對照：黑貓單的 dispatch 在同一狀態下是可用的（證明上一支不是恆 false）")
    void taqbinShipmentDoesHaveDispatch() {
        ShipmentEntity home = homeShipment(ShippingStatus.UNPROCESSED);
        assertThat(availability(home, FulfillmentAction.DISPATCH).enabled()).isTrue();
    }

    // ==================== actor 維度（BLOCKER 2） ====================

    @Test
    @DisplayName("每個動作都帶得出 actor，且賣家動作不得標成買家")
    void everyActionCarriesAnActor() {
        ShipmentEntity cvs = cvsShipment(ShippingStatus.CVS_CODE_RECEIVED, LabelPrintStatus.SUCCEEDED);
        List<ActionAvailability> actions = resolver.resolve(cvs);

        assertThat(actions).isNotEmpty();
        assertThat(actions).allSatisfy(a ->
                assertThat(a.actorAccountTypes()).as("action=" + a.action()).isNotEmpty());

        // 🔴 期望值一律硬寫字面值，不呼叫 FulfillmentActionPolicy——那會讓斷言跟著被測物一起變。
        assertThat(actorOf(actions, FulfillmentAction.HANDOVER)).containsExactly("2");     // 僅賣家
        assertThat(actorOf(actions, FulfillmentAction.PRINT_LABEL)).containsExactly("2");  // 僅賣家
        assertThat(actorOf(actions, FulfillmentAction.DISPATCH)).containsExactly("2");     // 僅賣家
        // 🔴 唯讀動作是**雙身分**：買賣家都能做，只是 gateway 依 viewer 給不同 href。
        //    先前宣告成單一 BUYER，於是 gateway 只能拿自己的路由表 key 當資格判定
        //    ⇒ 領域這份宣告是死的，形成兩份 SSOT（R4）。
        assertThat(actorOf(actions, FulfillmentAction.TRACK)).containsExactly("1", "2");
        assertThat(actorOf(actions, FulfillmentAction.VIEW_ONLY)).containsExactly("1", "2");
    }

    // ==================== 真正的 write path（M4 的鑑別力來源） ====================

    /**
     * 🔴 <b>上面那些測試直接呼叫 {@code resolver.require(...)}，因此對
     * 「有人把 {@code require} 從 {@code ShippingService} 拿掉」完全沒有鑑別力。</b>
     * 那正是 06-no-small-tails §3.10.1 記載的形狀：判準修了一半，
     * 而剩下那一半的變異不會轉紅。這兩支打的是真的 write path。
     */
    @Test
    @DisplayName("write path：契約說 handover 不可用時，confirmCvsShipment 必須用同一個碼拒絕")
    void confirmCvsShipmentRejectsIllegalHandover() {
        // 921 已確認出貨 → 再按一次確認出貨（契約：920 才是唯一合法起點）
        ShipmentEntity alreadyConfirmed =
                cvsShipment(ShippingStatus.IN_TRANSIT, LabelPrintStatus.SUCCEEDED);
        ShippingService service = serviceWith(alreadyConfirmed);

        assertThat(availability(alreadyConfirmed, FulfillmentAction.HANDOVER).enabled()).isFalse();
        assertThatThrownBy(() -> service.confirmCvsShipment(
                alreadyConfirmed.getStoreId(), alreadyConfirmed.getOrderNo(), List.of()))
                .as("修復前這裡回 200 並成功推進狀態——契約說不能做，端點照做")
                .isInstanceOf(ShippingBusinessException.class)
                .satisfies(e -> assertThat(((ShippingBusinessException) e).code())
                        .isEqualTo(GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED.code()));
    }

    @Test
    @DisplayName("write path：已取貨（終態）的黑貓單，callTcat 必須拒絕——QA 實測的 11→210 倒退")
    void callTcatRejectsTerminalShipment() {
        ShipmentEntity delivered = homeShipment(ShippingStatus.PICKED_UP);
        ShippingService service = serviceWith(delivered);

        assertThatThrownBy(() -> service.callTcat(
                delivered.getStoreId(), delivered.getOrderNo(), "Y", "N", "memo"))
                .as("修復前 callTcat 的五條前置守衛沒有一條檢查 status")
                .isInstanceOf(ShippingBusinessException.class);
    }

    // ==================== helpers ====================

    /**
     * 組一個只餵得出指定配送單的 {@link ShippingService}。
     *
     * <p>⚠️ 其餘依賴全 mock：本測試要驗的是<b>守衛在不在</b>，
     * 走到守衛之後的行為由各自既有的測試負責。
     */
    private ShippingService serviceWith(ShipmentEntity entity) {
        io.github.will413028.hawkr.shipping.mapper.ShipmentMapper shipmentMapper =
                mock(io.github.will413028.hawkr.shipping.mapper.ShipmentMapper.class);
        when(shipmentMapper.selectOne(any())).thenReturn(entity);
        return new ShippingService(
                shipmentMapper,
                mock(io.github.will413028.hawkr.shipping.mapper.ShipmentStatusLogMapper.class),
                mock(io.github.will413028.hawkr.shipping.client.OrderGrpcClient.class),
                mock(io.github.will413028.hawkr.shipping.client.BuyerGrpcClient.class),
                mock(io.github.will413028.hawkr.shipping.client.MediaGrpcClient.class),
                mock(io.github.will413028.hawkr.shipping.client.SellerGrpcClient.class),
                new ShipmentStatusMapper(),
                mock(ShipmentLabelService.class),
                mock(io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher.class),
                mock(io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher.class),
                mock(OnboardingService.class),
                mock(HolidayService.class),
                mock(CvsPrintWaitingRegistry.class),
                mock(io.github.will413028.hawkr.common.providercall.ProviderCallExecutor.class),
                mock(org.springframework.transaction.PlatformTransactionManager.class),
                registry,
                resolver,
                new io.micrometer.core.instrument.simple.SimpleMeterRegistry());
    }

    private ActionAvailability availability(ShipmentEntity shipment, FulfillmentAction action) {
        return resolver.resolve(shipment).stream()
                .filter(a -> a.action().equals(action.value()))
                .findFirst()
                .orElseThrow(() -> new AssertionError(
                        "resolve() 沒有回 " + action.value() + "——SUPPORTED 清單被改動了？"));
    }

    private static List<String> actorOf(List<ActionAvailability> actions, FulfillmentAction action) {
        return actions.stream().filter(a -> a.action().equals(action.value()))
                .findFirst().orElseThrow().actorAccountTypes();
    }

    private static ShipmentEntity cvsShipment(ShippingStatus status, LabelPrintStatus labelStatus) {
        ShipmentEntity e = base(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP, status);
        e.setLabelPrintStatus(labelStatus.value());
        return e;
    }

    private static ShipmentEntity homeShipment(ShippingStatus status) {
        ShipmentEntity e = base(ShippingMethod.TAQBIN, status);
        e.setLabelPrintStatus(LabelPrintStatus.SUCCEEDED.value());
        e.setTcatCallStatus(TcatCallStatus.NONE.value());
        return e;
    }

    private static ShipmentEntity base(ShippingMethod method, ShippingStatus status) {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(1L);
        e.setOrderNo("O-PARITY-0001");
        e.setShipmentNo("S-PARITY-0001");
        e.setStoreId(2675237784L);
        e.setShippingMethod(method.value());
        e.setStatus(status.value());
        e.setShipTradeNo("SC-PARITY-0001");
        return e;
    }
}

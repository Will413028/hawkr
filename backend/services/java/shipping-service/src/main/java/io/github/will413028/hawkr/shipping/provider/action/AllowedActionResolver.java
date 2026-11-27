package io.github.will413028.hawkr.shipping.provider.action;

import io.github.will413028.hawkr.common.utils.code.FulfillmentAction;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ShippingStatusRules;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.exception.ShippingBusinessException;
import io.github.will413028.hawkr.shipping.provider.FulfillmentContext;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderAdapter;
import io.github.will413028.hawkr.shipping.provider.FulfillmentProviderRegistry;
import io.github.will413028.hawkr.shipping.service.ShipmentStatusMapper;
import org.springframework.stereotype.Component;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.function.Function;

/**
 * {@code allowedActions[]} 的計算——<b>兩層模型真正被執行的地方</b>。
 *
 * <h2>🔴 這個類別存在之前發生了什麼</h2>
 * proto 有 {@code ActionAvailability}、Gateway 有 action→href 的映射層，
 * 而<b>沒有任何地方在算它</b> ⇒ 對外契約上 {@code allowedActions} 永遠是空陣列。
 * 兩層模型的文件、常數、policy 表全部就位，卻少了把它們接起來的那一步；
 * 而「欄位存在但恆為空」與「欄位不存在」對前端來說是同一件事——都得自己從 status 推導。
 *
 * <h2>兩層怎麼銜接（順序不可換）</h2>
 * <ol>
 *   <li><b>狀態閘門</b>：這顆按鈕在<b>當下這個狀態</b>能不能按（{@link ShipmentStatusMapper} 的
 *       allowed-from，或列印狀態機）。不通過 ⇒ {@code enabled=false} ＋ 狀態類 reason code</li>
 *   <li><b>capability 閘門</b>：<b>只在</b>
 *       {@link FulfillmentActionPolicy#requiresProviderCall} 為 {@code true} 時才問。
 *       🔴 {@code false} 的動作（賣家確認出貨、商家自送的取消）<b>完全不受 capability 影響</b>
 *       ——把 {@code Optional.empty()} 直接接到「按鈕不可用」，會讓使用者最常按的那幾顆整條消失</li>
 * </ol>
 *
 * <h2>⚠️ 本波只算 Gateway 路由表已登錄的 5 個 action</h2>
 * {@code dispatch} / {@code handover} / {@code print_label} / {@code track} / {@code view_only}。
 * 多算的 action 會被 Gateway 的 fail-closed 丟棄（它不猜路徑），
 * 於是「後端算了、前端看不到」——那比不算更難查。要擴充時<b>兩邊一起加</b>。
 */
@Component
public class AllowedActionResolver {

    private final FulfillmentProviderRegistry fulfillmentProviders;
    private final ShipmentStatusMapper statusMapper;

    public AllowedActionResolver(FulfillmentProviderRegistry fulfillmentProviders,
                                 ShipmentStatusMapper statusMapper) {
        this.fulfillmentProviders = fulfillmentProviders;
        this.statusMapper = statusMapper;
    }

    /**
     * 本波支援的 action（順序即對外輸出順序）。
     *
     * <p>⚠️ 與 Gateway 的 {@code actionRoute} 是<b>兩份必須對齊的清單</b>，
     * 兩側各有單邊釘樁（Java 的 supportedActionsArePinned、Go 的 TestActionRouteTableCarriesProviderDimension），
     * 但**沒有共同的機械來源**——跨 repo 跨語言的兩份清單，今天靠 review 對齊（多算會被 Gateway 丟棄、少算則按鈕不會出現）。
     */
    static final List<FulfillmentAction> SUPPORTED = List.of(
            FulfillmentAction.DISPATCH,
            FulfillmentAction.HANDOVER,
            FulfillmentAction.PRINT_LABEL,
            FulfillmentAction.TRACK,
            FulfillmentAction.VIEW_ONLY);

    /**
     * 算出這一筆配送單當下的動作可用性。
     *
     * <p>⚠️ 配送方式無法解析（髒資料／已退役的值）時回<b>只有 {@code view_only}</b> 的清單，
     * 不是空清單也不是拋例外：空清單會讓前端沒有任何可跟隨的 href，而拋例外會讓
     * 整個訂單詳情打不開——{@code view_only} 是 {@code FulfillmentAction} 明訂的唯一 fallback。
     */
    public List<ActionAvailability> resolve(ShipmentEntity shipment) {
        if (shipment == null) {
            return List.of();
        }
        ShippingMethod method = parseMethod(shipment.getShippingMethod());
        if (method == null) {
            return List.of(ActionAvailability.enabled(FulfillmentAction.VIEW_ONLY.value(),
                    FulfillmentActionPolicy.actorValuesFor(FulfillmentAction.VIEW_ONLY)));
        }
        ShippingStatus status = parseStatus(shipment.getStatus());
        FulfillmentContext ctx = FulfillmentContext.forShipment(shipment);

        List<ActionAvailability> out = new ArrayList<>(SUPPORTED.size());
        for (FulfillmentAction action : SUPPORTED) {
            out.add(evaluate(action, shipment, status, method, ctx));
        }
        return List.copyOf(out);
    }

    /**
     * <b>執行面的守衛——與 {@link #resolve} 共用同一份判定。</b>
     *
     * <h2>🔴 BLOCKER 3：這支方法存在的唯一理由</h2>
     * 修復前，「可以做」與「真的做」是兩套規則：
     * <ul>
     *   <li>契約側走 {@link #stateGate}／{@link #hasCapabilityFor}（本類別）</li>
     *   <li>執行面走 {@code ShippingService} 各 write path 自己的 inline 守衛
     *       ＋ mapper SQL 的 CAS，<b>五支裡沒有一支問轉移合法性</b></li>
     * </ul>
     * 症狀 QA 兩個方向都實測到：契約說 {@code handover enabled=false}（9527）而
     * {@code POST /shipments/&#123;no&#125;/confirm} 回 200 並把 920 推到 921；
     * 反方向是 {@code callTcat} 沒有任何 status 檢查 ⇒ 已取貨（11）的單被推回已叫件（210）。
     *
     * <p>⚠️ <b>不做成 silent no-op</b>：非法動作要 deterministic reject，
     * 且用的是<b>與契約一模一樣的業務碼</b>——前端拿到的 {@code disabledReasonCode}
     * 與它硬打端點時收到的碼相同，維運與使用者看到的是同一個原因。
     *
     * @throws ShippingBusinessException 這個動作在當下不可執行；碼＝契約會回的 {@code disabledReasonCode}
     */
    public void require(FulfillmentAction action, ShipmentEntity shipment) {
        if (shipment == null) {
            throw new ShippingBusinessException(GrpcResponseCode.SHIPMENT_NOT_FOUND);
        }
        ShippingMethod method = parseMethod(shipment.getShippingMethod());
        if (method == null) {
            throw new ShippingBusinessException(GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED);
        }
        ActionAvailability availability = evaluate(action, shipment,
                parseStatus(shipment.getStatus()), method, FulfillmentContext.forShipment(shipment));
        if (!availability.enabled()) {
            throw new ShippingBusinessException(
                    GrpcResponseCode.fromCode(availability.disabledReasonCode()));
        }
    }

    private ActionAvailability evaluate(FulfillmentAction action, ShipmentEntity shipment,
                                        ShippingStatus status, ShippingMethod method,
                                        FulfillmentContext ctx) {
        java.util.List<String> actor = FulfillmentActionPolicy.actorValuesFor(action);

        Integer stateReason = stateGate(action, shipment, status, method, ctx);
        if (stateReason != null) {
            return ActionAvailability.disabled(action.value(), stateReason, actor);
        }

        if (FulfillmentActionPolicy.requiresProviderCall(action, method)
                && !hasCapabilityFor(action, ctx)) {
            return ActionAvailability.disabled(action.value(),
                    GrpcResponseCode.FULFILLMENT_PROVIDER_NOT_ENABLED.code(), actor);
        }
        return ActionAvailability.enabled(action.value(), actor);
    }

    /**
     * 狀態閘門：回 {@code null} 表示狀態允許，否則回不可用的業務碼。
     *
     * <p>每一格的判準都寫成「這個動作實際會把單子推到哪個狀態，那條邊合不合法」，
     * 而不是自己再列一份狀態白名單——後者就是狀態機的第二份真實來源。
     */
    private Integer stateGate(FulfillmentAction action, ShipmentEntity shipment,
                              ShippingStatus status, ShippingMethod method, FulfillmentContext ctx) {
        switch (action) {
            case VIEW_ONLY:
                return null;

            case TRACK:
                return null;

            case DISPATCH:
                java.util.Set<ShippingStatus> explicitFrom =
                        FulfillmentActionPolicy.explicitAllowedFrom(action, method);
                if (explicitFrom != null) {
                    return explicitFrom.contains(status)
                            ? null
                            : GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code();
                }
                if (status == null || !statusMapper.isValidTransition(status, ShippingStatus.TCAT_CALLED)) {
                    return GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code();
                }
                if (FulfillmentActionPolicy.requiresProviderCall(action, method)
                        && !LabelPrintStatus.SUCCEEDED.value().equals(shipment.getLabelPrintStatus())) {
                    return GrpcResponseCode.SHIPMENT_LABEL_REQUIRED.code();
                }
                return null;

            case HANDOVER:
                if (!FulfillmentActionPolicy.isDomainApplicable(action, method)) {
                    return GrpcResponseCode.SHIPMENT_PROVIDER_MISMATCH.code();
                }
                if (!LabelPrintStatus.SUCCEEDED.value().equals(shipment.getLabelPrintStatus())) {
                    return GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED.code();
                }
                if (status == null || !statusMapper.isValidTransition(status, ShippingStatus.SELLER_CONFIRMED)) {
                    return GrpcResponseCode.SHIPMENT_HANDOVER_NOT_ALLOWED.code();
                }
                return null;

            case PRINT_LABEL:
                if (LabelPrintStatus.REQUESTED.value().equals(shipment.getLabelPrintStatus())) {
                    return GrpcResponseCode.SHIPMENT_PROVIDER_CALL_IN_PROGRESS.code();
                }
                if (shipment.getShipTradeNo() == null || shipment.getShipTradeNo().isBlank()) {
                    return GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code();
                }
                if (LabelPrintStatus.SUCCEEDED.value().equals(shipment.getLabelPrintStatus())
                        && !supportsReprint(ctx)) {
                    return GrpcResponseCode.SHIPMENT_ALREADY_LABELED.code();
                }
                if (status == null || ShippingStatusRules.isTerminal(status)) {
                    return GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code();
                }
                return null;

            default:
                return GrpcResponseCode.SHIPMENT_STATE_TRANSITION_REJECTED.code();
        }
    }

    /**
     * 這個 provider 的標籤是否可重印（冪等取回）。
     *
     * <p>超商三段碼可以重複取回（{@code usesThreeSegmentCode} 為 true 的那一軸），
     * 黑貓的宅配託運單取號一次即定案、重取會拋 5003。
     * capability 查不到時回 {@code false}（fail-closed：寧可按鈕不出現）。
     */
    private boolean supportsReprint(FulfillmentContext ctx) {
        return fulfillmentProviders.find(ctx.method().value())
                .flatMap(a -> a.label(ctx))
                .map(io.github.will413028.hawkr.shipping.provider.capability.LabelCapability::usesThreeSegmentCode)
                .orElse(false);
    }

    /** 這個 action 對應的 capability 在這個 provider 上是不是 present。 */
    private boolean hasCapabilityFor(FulfillmentAction action, FulfillmentContext ctx) {
        Optional<FulfillmentProviderAdapter> adapter =
                fulfillmentProviders.find(ctx.method().value());
        if (adapter.isEmpty()) {
            return false;
        }
        Function<FulfillmentProviderAdapter, Optional<?>> lookup = switch (action) {
            case DISPATCH -> a -> a.dispatch(ctx);
            case PRINT_LABEL, REPRINT_LABEL -> a -> a.label(ctx);
            case TRACK, REQUERY -> a -> a.track(ctx);
            case CANCEL -> a -> a.cancel(ctx);
            case ARRANGE_RETURN -> a -> a.returns(ctx);
            case VIEW_POD -> a -> a.pod(ctx);
            default -> a -> Optional.empty();
        };
        return lookup.apply(adapter.get()).isPresent();
    }

    private static ShippingMethod parseMethod(String value) {
        if (value == null || !ShippingMethod.isValidValue(value)) {
            return null;
        }
        return ShippingMethod.fromValue(value);
    }

    private static ShippingStatus parseStatus(String value) {
        if (value == null || !ShippingStatus.isValidValue(value)) {
            return null;
        }
        return ShippingStatus.fromValue(value);
    }
}

package io.github.will413028.hawkr.shipping.provider.action;

import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.FulfillmentAction;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;

import java.util.Map;
import java.util.Set;

/**
 * 每個領域動作的 {@code requiresProviderCall} 宣告——<b>兩層模型的銜接處</b>。
 *
 * <pre>
 * 第 1 層 capability（adapter）：這個 provider 的 API 有沒有這個能力？ → Optional.of / empty
 * 第 2 層 domain action（本類）：這顆按鈕現在能不能按？
 * </pre>
 *
 * <table border="1">
 *   <caption>銜接規則</caption>
 *   <tr><th>{@code requiresProviderCall}</th><th>語意</th><th>capability 為 empty 時</th></tr>
 *   <tr><td>{@code true}</td><td>這個 action 一定要打 provider API 才算完成</td>
 *       <td>{@code enabled = false} ＋ {@code PROVIDER_UNSUPPORTED}</td></tr>
 *   <tr><td>{@code false}</td><td>只寫我方狀態，provider 那邊靠回調／人工／完全不參與</td>
 *       <td>🔴 <b>不受影響</b></td></tr>
 * </table>
 *
 * <p>🔴 <b>把 {@code Optional.empty()} 直接接到「按鈕不可用」是本模型要防的頭號錯誤</b>：
 * 四個 provider 中只有 Uber 有真正的 cancel API，其餘三條線的取消都是
 * {@code requiresProviderCall = false} 的 domain action（寫 {@code 992} 或本地終態）。
 * 直接接的話，7-11 大宗寄倉與商家自送的取消按鈕會<b>整條消失</b>，而那是使用者最常按的按鈕之一。
 *
 * <p>⚠️ 宣告<b>逐 provider 可覆寫</b>：{@code cancel} 對 Uber 是 {@code true}、
 * 對超商與自送 L0 是 {@code false}。這是 action 的屬性隨 provider 變動，不是兩個不同的 action
 * ——這也是為什麼設計把 {@code cancel_request} 併回 {@code cancel}：
 * 同一個領域動作，差別只在這個布林。
 */
public final class FulfillmentActionPolicy {

    /**
     * 預設需要打 provider 的動作。
     *
     * <p>判準：<b>這個動作不打 provider 就不算完成嗎？</b>
     * 是 → 進本集合；否（只寫我方狀態）→ 不進。
     */
    private static final Set<FulfillmentAction> DEFAULT_REQUIRES_PROVIDER_CALL = Set.of(
            FulfillmentAction.DISPATCH,        // 派車／叫件：不打 provider 就沒有司機
            FulfillmentAction.CANCEL,          // 預設要打；超商與自送 L0 由 OVERRIDES 改成 false
            FulfillmentAction.PRINT_LABEL,     // 標籤由 provider 產生
            FulfillmentAction.REPRINT_LABEL,
            FulfillmentAction.TRACK,           // 主動回查
            FulfillmentAction.REQUERY,
            FulfillmentAction.ARRANGE_RETURN,  // 逆向建單
            FulfillmentAction.VIEW_POD,        // 送達證明要向 provider 取
            FulfillmentAction.UPDATE,          // 修改配送資料
            FulfillmentAction.RESUBMIT         // 重新建單
    );

    // ⚠️ 刻意**不在**上面集合裡的，逐群寫下理由（免得下一個人以為是漏的）：
    //   handover                                   ── B2C 交倉／自送出發只寫我方狀態，provider 端沒有交倉 API
    //   assign_batch/remove_from_batch/scan_bind/close_batch ── 批次編排是我方 domain model，provider 端無對應物
    //   report_progress/report_delivered/report_exception    ── 賣家自送的回報，provider 就是賣家自己
    //   upload_pod                                 ── 賣家上傳，不是向 provider 取
    //   seller_pickup / reselect_store             ── 我方狀態轉換；門市重選的 provider 側同步走取消重建
    //   open_case / claim / admin_resolve / admin_close_case ── ADM 的案件流程，與 provider 無關
    //   view_only                                  ── 兜底值，本來就不打任何東西

    private FulfillmentActionPolicy() {
    }

    /**
     * 逐 provider 覆寫表：{@code (action, shippingMethod) → requiresProviderCall}。
     *
     * <p>只列<b>與預設不同</b>的格子；沒列到的走預設。
     */
    private static final Map<String, Boolean> OVERRIDES = Map.ofEntries(
            // 🔴 取消：四個 provider 中只有 Uber 有真正的 cancel API。
            //    其餘三條線寫 992 CANCEL_PENDING 或本地終態，由回調／人工補上 provider 側確認。
            //    ⚠️ 這幾格漏掉的後果不是「多打一次 API」，是那些取消按鈕被 capability 的 empty 整條 gate 掉。
            Map.entry(FulfillmentAction.CANCEL.value() + "@" + ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value(), false),
            Map.entry(FulfillmentAction.CANCEL.value() + "@" + ShippingMethod.SEVEN_ELEVEN_BULK_WAREHOUSE.value(), false),
            Map.entry(FulfillmentAction.CANCEL.value() + "@" + ShippingMethod.MERCHANT_DELIVERY.value(), false),
            Map.entry(FulfillmentAction.CANCEL.value() + "@" + ShippingMethod.TAQBIN.value(), false),

            // 商家自送沒有外部 provider：所有動作都只寫我方狀態。
            Map.entry(FulfillmentAction.DISPATCH.value() + "@" + ShippingMethod.MERCHANT_DELIVERY.value(), false),
            Map.entry(FulfillmentAction.TRACK.value() + "@" + ShippingMethod.MERCHANT_DELIVERY.value(), false),
            Map.entry(FulfillmentAction.VIEW_POD.value() + "@" + ShippingMethod.MERCHANT_DELIVERY.value(), false),

            // Uber 的「派車」就是建單本身（同一支呼叫、不同觸發者），仍需打 provider。
            Map.entry(FulfillmentAction.DISPATCH.value() + "@" + ShippingMethod.UBER_INSTANT.value(), true)
    );

    /**
     * 每個領域動作的<b>執行者</b>——第三個維度（BLOCKER 2）。
     *
     * <h2>為什麼這一格必須存在</h2>
     * 少了它，對外契約只回答「這顆按鈕能不能按」而不回答「誰能按」，於是同一份
     * {@code allowedActions[]} 會原樣出現在買家端與賣家端。QA 實測到的兩個症狀都源自這裡：
     * <ul>
     *   <li>買家端 {@code /app/orders/&#123;no&#125;/logistics} 回三條掛 seller capability 的 href
     *       ⇒ 買家跟隨即 {@code 403 seller access required}</li>
     *   <li>賣家在那支端點上拿不到自己該執行的動作</li>
     * </ul>
     *
     * <h2>值域沿用 {@code AccountType}，不另立 enum</h2>
     * Gateway 的 {@code x-account-type} 用的就是 {@code "1"}／{@code "2"}／{@code "4"}。
     * 再開一組語意重疊的「ActionActor」＝兩份值域要同步，而它們一定會漂移。
     *
     * <p>⚠️ <b>fail-closed</b>：沒有在本表列出的動作歸給 {@code ADMIN}——
     * 那是最窄的受眾，漏列的症狀是「按鈕沒出現」（可見、可回報），
     * 而不是「買家看到賣家的按鈕」（靜默、跨越授權邊界）。
     */
    private static final Set<AccountType> SELLER_ONLY = Set.of(AccountType.SELLER);
    private static final Set<AccountType> BUYER_AND_SELLER = Set.of(AccountType.BUYER, AccountType.SELLER);

    /**
     * 這個領域動作<b>哪些身分可以執行</b>——actor eligibility 的唯一 SSOT。
     *
     * <p>🔴 這是集合而非單值，因為「同一個動作買賣家都能做、只是看到的端點不同」
     * （{@code track} / {@code view_only}）是常態而非例外：65 的端點表裡，
     * {@code cancel}（買家 B-3 / 賣家 S-4）與未來的下鑽都是同一形狀。
     *
     * <p>先前這件事有<b>兩份</b>：本表宣告 {@code TRACK → BUYER}，而 gateway 的
     * {@code viewerScopedRoute} 用「這張路由表有沒有這個 viewer 的 key」當資格判定
     * （drop 理由字串逐字是 {@code actor_mismatch}），完全不讀本表 ⇒ 本表對那兩個
     * 動作是死的。收斂後 gateway 只做兩件事：viewer 在不在本集合內（純執行）、
     * 以及 {@code (action, provider, viewer) → href}（純路由）。
     *
     * <p>漏列的 default 是 {@code ADMIN}（fail-closed）：症狀是按鈕沒出現，
     * 而不是買家看到賣家的按鈕。
     */
    private static final Map<FulfillmentAction, Set<AccountType>> ACTORS = Map.ofEntries(
            // 賣家履約動作：全部掛在 requireSeller(CapOrderFulfill) 的路由上
            Map.entry(FulfillmentAction.DISPATCH, SELLER_ONLY),
            Map.entry(FulfillmentAction.HANDOVER, SELLER_ONLY),
            Map.entry(FulfillmentAction.PRINT_LABEL, SELLER_ONLY),
            Map.entry(FulfillmentAction.REPRINT_LABEL, SELLER_ONLY),
            Map.entry(FulfillmentAction.ARRANGE_RETURN, SELLER_ONLY),
            Map.entry(FulfillmentAction.UPDATE, SELLER_ONLY),
            Map.entry(FulfillmentAction.RESUBMIT, SELLER_ONLY),
            Map.entry(FulfillmentAction.UPLOAD_POD, SELLER_ONLY),
            // 唯讀動作：買賣雙方都有，各自指向自己的端點（買家 B-1 / 賣家 S-9）
            Map.entry(FulfillmentAction.TRACK, BUYER_AND_SELLER),
            Map.entry(FulfillmentAction.VIEW_ONLY, BUYER_AND_SELLER),
            // VIEW_POD 是賣家動作：65 唯一的 pod 端點 S-12
            // GET /app/shipments/:order_no/pod 掛在 sellerHandle 上；買家側的 POD 是
            // B-1 回應內嵌的 delivery.podImageKeys，不是一個 action。
            // （本表先前宣告 BUYER，與設計書矛盾；該格今天不在 SUPPORTED 內尚未引爆。）
            Map.entry(FulfillmentAction.VIEW_POD, SELLER_ONLY));

    /**
     * 這個領域動作在這個配送方式上<b>存在嗎</b>——第四個維度。
     *
     * <p>與 {@link #requiresProviderCall} 是兩個問題：
     * <ul>
     *   <li>{@code requiresProviderCall}：這個動作要不要打 provider API</li>
     *   <li>本表：這條產品線的履約流程裡<b>有沒有這一步</b></li>
     * </ul>
     * 黑貓是司機到府取件，賣家不需要再「確認出貨」——那不是「provider 不支援」，
     * 是這個動作在那條線上根本不存在。
     *
     * <p>🔴 <b>預設「適用」，只列不適用的格子</b>：漏列的症狀是按鈕多出來（可見、可回報），
     * 而預設不適用的症狀是按鈕消失（靜默，且會誤殺所有尚未宣告的 provider）。
     * ⚠️ 不要改用 adapter 的 {@code requiresSellerConfirmation()} 當判準——
     * 那個布林的預設值是 {@code false}，分不出「明確宣告沒有」與「還沒宣告」。
     */
    private static final Set<String> NOT_APPLICABLE = Set.of(
            // 黑貓宅配：司機到府取件，沒有「賣家確認出貨」這一步。
            FulfillmentAction.HANDOVER.value() + "@" + ShippingMethod.TAQBIN.value(),
            // Uber 即時配送：交付確認走 POD，不走我方的「賣家確認出貨」。
            // ⚠️ 66 尚未落地；列在這裡是因為端點側本來就拒絕它，而兩層必須給同一個答案。
            //    66 實作時若確認 Uber 有這一步，改的是這一行——不是在端點再加一個 if。
            FulfillmentAction.HANDOVER.value() + "@" + ShippingMethod.UBER_INSTANT.value());

    /**
     * 逐 (action, provider) 的<b>顯式 allowed-from</b>——只列「用目標狀態的轉移合法性推導會答錯」的格子。
     *
     * <h2>🔴 為什麼 {@code dispatch@3003} 必須顯式列出</h2>
     * 預設判準是「這個動作實際會把單子推到哪個狀態，那條邊合不合法」，而它對 Uber 答錯兩次：
     * <ul>
     *   <li><b>目標狀態不對</b>：預設目標是 {@code 210 TCAT_CALLED}（黑貓叫件），
     *       而 的 A-4 自檢逐字列出 3003 <b>不經過</b> {@code 210}。
     *       Uber 的「派車」就是<b>建單本身</b>，成功後回到 {@code 91 UNPROCESSED}</li>
     *   <li>🔴 <b>{@code 991} 會被放行</b>：{@code 991 CREATE_UNKNOWN} 在
     *       {@code ShipmentStatusMapper} 裡屬 {@code PRE_SHIPMENT} ⇒
     *       {@code isValidTransition(991, 91)} 回 {@code true} ⇒ 賣家按得下「重新建單」。
     *       而 {@code 991} 的語意正是「<b>可能已經建成功了</b>」——再建一次就是
     *       66 陷阱 A8：<b>兩趟車、重複計費</b>。它的收斂手段是 T-8 回查，不是重建</li>
     * </ul>
     * ⇒ {@code dispatch}「🔴 <b>只在 {@code 990 CREATE_FAILED} 時 enabled</b>」。
     *
     * <p>⚠️ 用表而不是在 {@code AllowedActionResolver} 裡寫 {@code method == UBER_INSTANT}：
     * 那個 if 正是整個 adapter 模型要消滅的東西，而本檔的其他三張表已經是這個形狀。
     */
    private static final Map<String, Set<ShippingStatus>> EXPLICIT_ALLOWED_FROM = Map.of(
            FulfillmentAction.DISPATCH.value() + "@" + ShippingMethod.UBER_INSTANT.value(),
            Set.of(ShippingStatus.CREATE_FAILED));

    /**
     * 這個 (action, provider) 有沒有顯式 allowed-from。
     *
     * @return {@code null} ＝ 沒有，走預設推導；非 null ＝ <b>這就是完整的狀態閘門</b>，
     *         呼叫端不得再疊加其他狀態條件（例如標籤已取號——Uber 沒有標籤）
     */
    public static Set<ShippingStatus> explicitAllowedFrom(FulfillmentAction action, ShippingMethod method) {
        return EXPLICIT_ALLOWED_FROM.get(action.value() + "@" + method.value());
    }

    /** 這個動作在這個配送方式上是否存在。未列在 {@link #NOT_APPLICABLE} 者一律視為存在。 */
    public static boolean isDomainApplicable(FulfillmentAction action, ShippingMethod method) {
        return !NOT_APPLICABLE.contains(action.value() + "@" + method.value());
    }

    /**
     * 誰可以執行這個動作。未列出者一律歸 {@code ADMIN}（fail-closed，見 {@link #ACTOR}）。
     */
    public static Set<AccountType> actorsFor(FulfillmentAction action) {
        return ACTORS.getOrDefault(action, Set.of(AccountType.ADMIN));
    }

    /**
     * {@link #actorsFor} 的 wire 形式：{@code AccountType} 的 value 集合，排序後回傳。
     *
     * <p>排序讓 proto 的 {@code repeated} 欄位與測試斷言都是確定性的。
     */
    public static java.util.List<String> actorValuesFor(FulfillmentAction action) {
        return actorsFor(action).stream().map(AccountType::value).sorted().toList();
    }

    /**
     * 這個動作在這個配送方式上，是否必須打 provider API 才算完成。
     *
     * @param action 領域動作
     * @param method 配送方式
     */
    public static boolean requiresProviderCall(FulfillmentAction action, ShippingMethod method) {
        String key = action.value() + "@" + method.value();
        Boolean override = OVERRIDES.get(key);
        if (override != null) {
            return override;
        }
        return DEFAULT_REQUIRES_PROVIDER_CALL.contains(action);
    }
}

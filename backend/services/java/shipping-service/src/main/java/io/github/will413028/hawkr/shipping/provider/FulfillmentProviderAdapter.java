package io.github.will413028.hawkr.shipping.provider;

import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.shipping.provider.capability.CancelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.CostCapability;
import io.github.will413028.hawkr.shipping.provider.capability.CreateCapability;
import io.github.will413028.hawkr.shipping.provider.capability.DispatchCapability;
import io.github.will413028.hawkr.shipping.provider.capability.InboundCapability;
import io.github.will413028.hawkr.shipping.provider.capability.LabelCapability;
import io.github.will413028.hawkr.shipping.provider.capability.OnboardCapability;
import io.github.will413028.hawkr.shipping.provider.capability.PodCapability;
import io.github.will413028.hawkr.shipping.provider.capability.ReturnCapability;
import io.github.will413028.hawkr.shipping.provider.capability.StoreLocatorCapability;
import io.github.will413028.hawkr.shipping.provider.capability.TrackCapability;

import java.util.Optional;
import java.util.Set;

/**
 * 履約 provider 的能力邊界。取代只抽象了「建單」的舊 {@code ShipmentStrategy}。
 *
 * <h2>三條硬規則（adapter 不可以做什麼）</h2>
 * <ol>
 *   <li><b>adapter 只做翻譯，不做業務判斷。</b>「這個狀態要不要推進訂單」是狀態機的事。</li>
 *   <li><b>未知的 provider 原生值一律 fail-closed</b>（不猜測、不預設放行）。落點是 {@link #inbound}。</li>
 *   <li><b>adapter 不得持有憑證。</b> 憑證解析走既有的 secret 掛載，adapter 只拿到「已解析好的 client」。</li>
 * </ol>
 *
 * <h2>🔴 兩層模型：capability ≠ 按鈕可用性</h2>
 * <pre>
 * 第 1 層 capability（本介面）：這個 provider 的 API 有沒有這個能力？ → Optional.of / empty
 * 第 2 層 domain action（狀態機）：這顆按鈕現在能不能按？          → allowedActions[].enabled
 * </pre>
 * 銜接兩層的是每個 action 自己宣告的 {@code requiresProviderCall}（且逐 provider 可覆寫）：
 * <ul>
 *   <li>{@code true}（例：Uber 的 cancel）→ capability 為 empty 時 {@code enabled=false}</li>
 *   <li>{@code false}（例：賣家確認出貨、商家自送的取消——只寫我方狀態）→ <b>完全不受 capability 影響</b></li>
 * </ul>
 * ⚠️ 把 {@code Optional.empty()} 直接接到「按鈕不可用」，會讓 7-11 店到店的取消與商家自送的取消
 * 整條消失，而那是使用者最常按的按鈕之一。
 *
 * <h2>🔴 為什麼是 Optional 而不是丟例外</h2>
 * 呼叫端要能「<b>查得到</b>不支援」而不是「炸給你看」——{@code allowedActions[]} 的計算需要在
 * 不觸發任何副作用的情況下問出「這個 provider 支不支援取消」。用例外表達，這個問題就問不出來。
 *
 * <h2>本波刻意不納入 adapter 的三類</h2>
 * <ul>
 *   <li><b>批次編排</b>（建批／關批／掃描綁定）——provider 端無對應 API，那是我方 domain model</li>
 *   <li><b>provider 帳單匯入／對帳明細</b>——唯一寫入者是人工匯入，加一個恆為 empty 的 capability 是憑空的對稱</li>
 *   <li><b>賣家自送的服務範圍／時段／承載</b>——那是我方的資料，{@code MERCHANT} 沒有外部 API 可問</li>
 * </ul>
 *
 * <h2>驗收條件（可機械執行）</h2>
 * <ol>
 *   <li>{@code ShippingService} 內以 {@code shipping_method} 字面值分支的地方歸零
 *       （實查基準：15 處，非設計書所寫的 5 處）</li>
 *   <li>{@code Optional.empty()} 是可查詢狀態，不是例外</li>
 *   <li>adapter 不得回傳 provider 原生型別——{@code LgsType}／{@code ShipType}／{@code GoodsType}
 *       等 PayUni enum <b>不得越過本介面</b>；回 normalized 值 ＋ {@code providerRawStatus} 字串</li>
 * </ol>
 */
public interface FulfillmentProviderAdapter {

    /** 本 adapter 服務哪一種配送方式。registry 以此建索引。 */
    ShippingMethod supportedMethod();

    /** 建單。⚠️ 對 {@code 3005} 回 empty 不代表建不出單——那是 {@code requiresProviderCall=false} 的 domain action。 */
    Optional<CreateCapability> create(FulfillmentContext ctx);

    /** 取標籤／重印／批次。約束（單批上限、分組鍵）由 capability 物件自己攜帶，呼叫端不得寫死數字。 */
    Optional<LabelCapability> label(FulfillmentContext ctx);

    /** 叫件／派車／交運確認。 */
    Optional<DispatchCapability> dispatch(FulfillmentContext ctx);

    /** 取消。🔴 四個 provider 中目前只有 Uber 是 present——PayUni 的 11 個端點常數裡沒有 cancel。 */
    Optional<CancelCapability> cancel(FulfillmentContext ctx);

    /** 主動回查（狀態同步與對帳排程共用同一支 provider 呼叫，不另立 capability）。 */
    Optional<TrackCapability> track(FulfillmentContext ctx);

    /** 逆物流建單。 */
    Optional<ReturnCapability> returns(FulfillmentContext ctx);

    /** 送達證明。 */
    Optional<PodCapability> pod(FulfillmentContext ctx);

    /** 建店／開通。⚠️ 7-11 的擁有者跨到 seller-service，本介面只負責 provider 側的送審。 */
    Optional<OnboardCapability> onboard(FulfillmentContext ctx);

    /**
     * provider → 我方回調的解析與正規化（webhook／Notify／檔案交換）。
     *
     * <p>🔴 <b>本介面是「未知 provider 原生值 fail-closed」唯一的落點。</b>
     * 沒有它，各 provider 的映射表會各自寫在自己的 consumer 裡，而硬規則 2 沒有任何東西承載。
     */
    Optional<InboundCapability> inbound(FulfillmentContext ctx);

    /** 門市地圖（買家選店）。只有超商軸有。 */
    Optional<StoreLocatorCapability> storeLocator(FulfillmentContext ctx);

    /**
     * provider 成本（這條 leg 已發生多少物流成本）。
     *
     * <p>🔴 <b>它是 {@code logistics.cost.incurred} 得以 provider-neutral 的原因</b>：
     * 事件契約與觸發條件都不分 provider（70 §7.1／§3.1.1），而「成本怎麼取得」必然分
     * ——有的 provider 在報價時就給、有的只在月結帳單給。差異收斂在各自的 adapter 裡，
     * producer 不得改用 {@code if (provider == …)} 分支。
     *
     * <p>⚠️ 不透過 API 提供成本的 provider 回 {@link Optional#empty()}，
     * 🔴 <b>不是回 0</b>——0 是合法金額，會讓賣家應收變成 0 而沒有訊號。
     */
    Optional<CostCapability> cost(FulfillmentContext ctx);

    /**
     * 本波尚未搬移實作、但 provider <b>確實具備</b>的 capability 名稱集合。
     *
     * <p>🔴 <b>為什麼需要這個方法</b>：{@code Optional.empty()} 在遷移期間有兩種完全不同的原因——
     * <ul>
     *   <li><b>provider 不支援</b>（例：PayUni 沒有 cancel 端點）＝ <b>永久</b>的 empty</li>
     *   <li><b>我們還沒搬過來</b>（例：宅配標籤，實作仍在既有 service）＝ <b>暫時</b>的 empty</li>
     * </ul>
     * 型別上兩者一模一樣。分不出來的話，下一個人會把「我們還沒搬」讀成「這個 provider 做不到」，
     * 然後在別的地方重新實作一份——而那正是本 adapter 要消滅的東西。
     *
     * <p>⚠️ 這個集合<b>應該隨施工波次縮小到空集合</b>。它不是設計的一部分，是遷移狀態的誠實標註；
     * 留著不縮小就會變成「留著沒人敢刪」的技術債。
     *
     * @return capability 的方法名（例 {@code "label"}、{@code "dispatch"}）；已完成或 provider 不支援的不列入
     */
    Set<String> pendingMigration();

    /**
     * 這個 provider 的履約流程裡有沒有「賣家確認出貨」這一步（{@code handover}）。
     *
     * <p>🔴 <b>本方法從 {@code DispatchCapability} 搬上來（2026-09-10 BLOCKER 2 修復）。</b>
     * 原本它掛在 {@code DispatchCapability} 上，於是 7-11 C2C 為了「問得出這個布林」
     * 必須讓 {@code dispatch()} 回 {@code Optional.of(一個一呼叫就丟 UnsupportedOperationException 的物件)}。
     * 後果是<b>上游只看得到 {@code isPresent()}</b>：`allowedActions` 判定超商單「可以派車」，
     * 而 href 指向黑貓端點——那是 QA 實測到的跨 provider 誤導。
     *
     * <p>語意上它本來就不屬於 dispatch：它問的是「這條線的出貨流程長什麼樣」，
     * 不是「provider 有沒有派車 API」。放在 adapter 上，
     * {@code dispatch()} 就能誠實地對沒有派車 API 的 provider 回 {@code Optional.empty()}。
     *
     * <p>預設 {@code false}——新 provider 不寫這一格時走「不需要賣家確認」，
     * 那是保守值：它讓 {@code handover} 按鈕不出現，而不是讓一個不存在的步驟變成必經。
     */
    default boolean requiresSellerConfirmation() {
        return false;
    }
}

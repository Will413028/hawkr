package io.github.will413028.hawkr.shipping.provider.capability;

import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;

import java.util.Optional;

/**
 * provider 成本能力：這條 fulfillment leg <b>已經產生</b>多少物流成本。
 *
 * <h2>為什麼是一個 capability，而不是在 producer 裡寫 {@code if (provider == UBER)}</h2>
 * {@code logistics.cost.incurred} 的契約是 <b>provider-neutral</b> 的（70 §7.1）——
 * Uber 只是第一個 producer path。成本<b>怎麼取得</b>卻必然是 provider-specific：
 * 有的 provider 在報價時就給、有的只在月結帳單給、有的根本不透過 API 給。
 * 把這個差異放進既有的 capability 抽象，新 provider 接入時改的是<b>它自己的 adapter</b>，
 * 而不是去 producer 裡加一條 {@code if}。
 *
 * <p>⚠️ <b>不支援的 provider 回 {@link Optional#empty()}</b>（與其餘 capability 同慣例）
 * ——那代表「本 provider 不透過此管道提供成本」，<b>不是</b>「成本是 0」。
 * 🔴 兩者不得混為一談：0 會讓賣家應收變成 0，而 empty 是「不發這個事件」。
 *
 * <h2>🔴 語意：incurred（已發生），不是 quoted（報價）也不是 billed（最終帳單）</h2>
 * <ul>
 *   <li><b>已發生</b>：provider 這一趟確實會向我方收費——送達／取件後取消／
 *       買家無法投遞／返程 leg 四種情形都算（70 §3.1.1），<b>不是只有送達</b>。</li>
 *   <li><b>當下權威估計</b>：本值落進 {@code logistics_receivable.estimated_cost}。
 *       provider 月結帳單到達後走 {@code actual_cost} true-up <b>同一筆</b> receivable
 *       （70 §6.2、§3.1.3）——🔴 <b>不得為 true-up 建立第二筆</b>。</li>
 * </ul>
 *
 * <h2>🔴 單位：新台幣「元」整數</h2>
 * provider 的 minor unit <b>不得滲透出 adapter 邊界</b>。
 * 實作者若拿到的是 cents／分，換算必須在自己的 adapter 內完成
 * （Uber 走 {@code UberFieldFormat.fromMinorUnits}）。
 * ⚠️ 這不是假設性風險：Uber 入站曾經直接取值不換算 ⇒ <b>金額差 100 倍</b>，
 * 而這個值正是要向賣家收的錢。
 */
public interface CostCapability {

    /**
     * 這條 leg 已發生的 provider 成本，新台幣「元」整數。
     *
     * <p>回 {@link Optional#empty()} 的情形（呼叫端一律解讀為「不發成本事件」）：
     * <ul>
     *   <li>provider 側查無這條 leg 的成本事實</li>
     *   <li>本 provider 不透過 API 提供成本（只在月結帳單出現）</li>
     * </ul>
     * 🔴 <b>不得用 0 表達「查不到」</b>——0 是一個合法金額，會讓賣家應收變成 0 而沒有任何訊號。
     */
    Optional<Long> incurredCost(ShipmentEntity shipment);
}

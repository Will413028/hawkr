package io.github.will413028.hawkr.shipping.service.dto;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

/**
 * {@code logistics.cost.incurred} 的 wire payload（逐欄契約 ＝ 70 §7.1）。
 *
 * <h2>語意：provider 這條 fulfillment leg 已產生成本</h2>
 * 🔴 <b>不是「送達完成」。</b> 送達／取件後取消／買家無法投遞／返程 leg
 * 四種情形都算（70 §3.1.1）——把觸發條件讀成送達，會漏掉其中三類，
 * 而那三類正是「provider 已經收了錢、我方卻沒有任何應收紀錄」的情形。
 *
 * <h2>🔴 Cost Fact ≠ Responsibility Decision</h2>
 * 本 payload <b>刻意沒有</b> {@code responsibilityCause}。兩者是<b>兩個獨立的 business fact</b>：
 * <ul>
 *   <li>「這條 leg 產生了多少成本」—— shipping domain 的事實，載體是<b>本事件</b></li>
 *   <li>「責任終局 ＋ settlement counterparty」—— payment domain 的責任判定案件，
 *       載體是 <b>N16</b>（{@code fulfillment.responsibility.finalized}）</li>
 * </ul>
 * 🔴 <b>唯一的 responsibility authority 是 N16。</b> 讓成本事件攜帶 cause，等於讓一個 domain
 * 去斷言另一個 domain 的事實 ⇒ 雙 SSOT ⇒ 「向賣家收錯錢」會變成一個<b>沒有訊號</b>的事件。
 *
 * <p>⚠️ <b>舊版 70 §7.1 曾要求本欄必填且「未判定就不發」</b>，與同書 §6.3／SD-15 的
 * 「成本已發生的事實<b>先記錄</b>，歸屬未定」直接矛盾——照舊版施工，SD-15 圖裡那一步
 * 在物理上不可能發生。兩個事實如何 durable join（且<b>抵達順序不影響最終結果</b>）見 70 §3.1.4。
 *
 * <h2>🔴 型別：id 與金額一律 {@code String}</h2>
 * 65 通則；36 已於 2026-09-16 對 N16 統一。<b>理由不是風格</b>：
 * notification-service 把 payload 解進 {@code map[string]interface{}}，
 * 而 {@code encoding/json} 對 {@code any} 用 <b>float64</b>
 * ⇒ 超過 2^53 的 TSID 會被<b>靜默截斷</b>。
 *
 * <h2>🔴 金額單位：新台幣「元」整數</h2>
 * provider 的 minor unit <b>不得滲透進本 wire</b>。Uber 官方 {@code fee} 是 <b>cents</b>，
 * 換算只在 uber-adapter 邊界做（{@code UberFieldFormat.fromMinorUnits}，66）。
 * ⚠️ 這不是假設性風險：入站曾經直接取值不換算 ⇒ <b>金額差 100 倍</b>，
 * 而這個值正是要向賣家收的錢。
 *
 * <h2>⚠️ 投遞可靠性：shipping-service 沒有 outbox</h2>
 * 本服務的 publisher 是交易外的 {@code ObsRabbitPublisher}（實查：全服務無 outbox 表）。
 * ⇒ 發送失敗時這一筆成本事實<b>會丟</b>，而不是延後送達。
 * 🔴 <b>因此發送端必須 fail-loud</b>（拋出，讓呼叫端看得見），不得吞例外。
 * ⚠️ <b>最終兜底不在本條路徑上</b>：provider 月結帳單匯入（70 §6.1）會帶來該期間的<b>全部</b>
 * 成本事實並走 true-up（§6.2）⇒ 事件丟失是<b>可偵測、可補回</b>的，不是永久損失。
 * 把 outbox 加進 shipping-service 是獨立的工程，不在本波射程內。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
@JsonIgnoreProperties(ignoreUnknown = true)
public record LogisticsCostIncurredPayload(
        String eventId,
        String eventType,
        String eventVersion,
        /**
         * 🔴 <b>成本事實發生的業務時間</b>，不得用發送時間；與 DB audit time 不可互相冒充。
         * ISO 8601 UTC 帶 {@code Z}。
         */
        String occurredAt,
        String traceId,
        String correlationId,
        /** {@code ShippingMethod} 碼值（{@code 3003}…）。🔴 provider-neutral，本事件不限 Uber。 */
        String provider,
        /**
         * 🔴 <b>identity 的錨</b>（70 §3.1.2）。為 null／空 ⇒ 沒有 provider leg
         * ⇒ <b>不得發這個事件</b>（沒有 leg 就沒有 provider cost incurred）。
         *
         * <p>⚠️ consumer 的冪等鍵由 {@code provider} ＋ 本欄組出，形式由 DB 的
         * {@code chk_logrecv_idem_anchor} 強制：
         * {@code logistics_receivable:{provider}:{provider_delivery_ref}}。
         * 用別的格式寫入會在 INSERT 當下 {@code 23514}。
         */
        String providerDeliveryRef,
        /** 賣家業務鍵的 TSID 十進位字串（🔴 <b>不是 {@code account_id}</b>）。 */
        String storeId,
        /** 溯源。 */
        String orderNo,
        /**
         * TSID 的十進位字串；⚠️ 可為 {@code null}——<b>取消可能發生在 shipment 建立之前</b>，
         * 這個事件真的可能沒有 shipment。
         * 🔴 <b>正因為它可以是 null，它不能當冪等鍵</b>（70 §7.1 逐字）。
         */
        String shipmentId,
        /**
         * 這條 leg 已發生成本的<b>當下權威估計</b>，新台幣「元」整數的十進位字串。
         *
         * <p>🔴 <b>不是 provider 的最終帳單</b>：後續月結帳單 → {@code actual_cost}
         * → <b>true-up 同一筆 receivable</b>（70 §6.2、§3.1.3）。
         * 🔴 <b>不得為 true-up 建立第二筆 receivable。</b>
         */
        String estimatedCost) {
}

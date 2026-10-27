package io.github.will413028.hawkr.payment.logistics.mq.payload;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

/**
 * {@code N16 fulfillment.responsibility.finalized} 的 wire payload（36 §3.3 逐欄）。
 *
 * <p>語意：責任判定 <b>financially final</b>——不再有任何會改變 {@code refund_to_buyer} 的
 * 待覆核提案。🔴 只有 resolution {@code APPROVED} <b>且</b> case {@code RESOLVED} 的那一次
 * 交易才會產生它（65 §5.5.2 E0.1 ②③ 兩道 CAS 都 rows=1）。
 *
 * <p>🔴 <b>{@code responsibilityCause} 只能是四個 resolved 值</b>；
 * {@code RESPONSIBILITY_UNRESOLVED} 不會出現在本事件——未 final 就不發。
 * 平台／系統原因一律歸 {@code PLATFORM_CAUSED}，<b>不存在 {@code SYSTEM_CAUSED}</b>。
 *
 * <p>🔴 <b>不得把整包 evidence 塞進 payload</b>：只傳足以指認 final decision 的 reference
 * （{@code caseNo} ＋ {@code resolutionId}），完整 evidence 由 consumer 需要時經既有查詢取得。
 *
 * <p>⚠️ <b>{@code caseNo}／{@code resolutionId} 不是去重鍵</b>：{@code outbox_events} 沒有
 * {@code idempotency_key} 欄、也沒有對應 UNIQUE。producer 側的冪等是 <b>guarded finalization
 * transition 本身</b>（CAS {@code rows=0} ⇒ 不再 INSERT 第二筆），那是 DB 層的事實，
 * 不需要額外索引，也不會因為有人改了鍵的組成字串而失效。
 *
 * <p>🔴 <b>{@code shipmentId} 是 nullable {@code String}（TSID 十進位字串），不是 JSON 數字。</b>
 * 65 §6A.1.3 的跨語言通則逐字「新事件的 id 與金額一律 {@code string}」，理由不是風格：
 * notification-service 的 {@code matchesFilter(filter, payload map[string]interface{})} 把 payload
 * 解進 {@code map[string]interface{}}，而 {@code encoding/json} 對 {@code any} 用 <b>float64</b>
 * ⇒ 超過 2^53 的 TSID 會被靜默截斷，而本事件的 consumer 正好包含 notification-service。
 *
 * <p>⚠️ <b>「要能表達沒有 shipment」不是用數字型別的理由</b>：36 §3.3 當初選 boxed {@code Long}
 * 是為了讓「取消發生在 shipment 建立之前」能寫成 {@code null}（primitive 會把「沒有」靜默寫成 0，
 * 而 0 看起來像一個合法 TSID）——但 <b>nullable {@code String} 一樣表達得了 {@code null}</b>，
 * 兩個需求從來不衝突。WP2A-R1 依此統一為 {@code String}，36 的逐欄表已同批回灌。
 *
 * <p>⚠️ <b>internal 型別不跟著改</b>：{@code OpenUnresolvedCommand}／{@code FinalizeCommand} 的
 * {@code shipmentId} 保持 {@code Long}（與 DB／domain 一致），只在組 payload 的邊界轉字串
 * （{@code null → null}／{@code Long → Long.toString()}）。不為了 wire contract 重構 internal ID 型別。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
@JsonIgnoreProperties(ignoreUnknown = true)
public record FulfillmentResponsibilityFinalizedPayload(
        String eventId,
        String eventType,
        String eventVersion,
        /** 🔴 finality 成立的<b>業務時間</b>，不得用發送時間；與 DB audit time 不可互相冒充。 */
        String occurredAt,
        String traceId,
        String correlationId,
        /** Order 軸錨點，consumer 的 CAS 依據。 */
        String orderNo,
        /**
         * TSID 的十進位字串；⚠️ 可為 {@code null}（取消可能發生在 shipment 建立之前）。
         * 🔴 wire 上必須是 <b>quoted string 或 null</b>，不得是 JSON 數字——見類別 javadoc。
         */
        String shipmentId,
        /** {@code ShippingMethod} 碼值（{@code 3003}…）；provider-neutral，本事件不限 Uber。 */
        String shippingMethod,
        /** fulfillment leg 識別；無 provider leg 時為 null。 */
        String providerDeliveryRef,
        /** 🔴 四個 resolved 的 {@code ResponsibilityCause} 之一。 */
        String responsibilityCause,
        /**
         * 🔴 恆為 {@code true}。保留此欄是為了讓 consumer 的契約測試<b>斷言得到它</b>，
         * 而不是靠「這個 routing key 應該只在 final 時發」的紀律。
         */
        Boolean financiallyFinal,
        String caseNo,
        /** {@code fulfillment_case_resolutions.id}（TSID，<b>字串</b>）。 */
        String resolutionId,
        String resolvedAt
) {
}

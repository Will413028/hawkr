package io.github.will413028.hawkr.payment.logistics.mq.payload;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

/**
 * {@code N17 fulfillment.responsibility.unresolved} 的 wire payload。
 *
 * <p>語意：<b>責任仍未定案</b>——配送失敗但成因證據不足，退不退款<b>尚未決定</b>。
 * 🔴 只有建案 {@code ON CONFLICT DO NOTHING} 真的寫進一列的那一次交易才會產生它
 * （36 §3.3 的 L2：{@code rows=0} ⇒ 不發第二筆）。
 *
 * <p>🔴 <b>本事件不得攜帶任何 financially-final 語意</b>：不帶 Bearer、不帶 settlement
 * counterparty、不帶退款或不退款的決定、不帶 resolved 的責任成因。N1
 * （{@code order.fulfillment.failed}）的模板逐字承諾「已停止履約並<b>全額退款</b>」，
 * 而此時「退不退」尚未決定；N16 的語意是「已經 financially final」。兩條都借不到
 * ⇒「正在確認原因」這件事沒有任何既有 key 表達得了，所以 N17 必須是獨立的一條。
 *
 * <p>⚠️ <b>欄位集合由 WP2A 定案</b>：36 §3.3 與 65 §6A.1.3 當時只給了 class 名、envelope 三欄
 * 與冪等層級，<b>沒有逐欄表</b>（實查 65 §6A.1.3 只有 N1／N2／N3-N6／N7-N9 四張）。
 * 本實作取 N16 的欄位集合<b>減去全部 final-only 欄位</b>（{@code responsibilityCause}／
 * {@code resolutionId}／{@code resolvedAt}），並保留 {@code financiallyFinal} 恆 {@code false}
 * ——與 N16 保留該欄的理由對稱：讓 consumer 的契約測試斷言得到它，而不是靠紀律。
 * 逐欄表已同批回灌 36 §3.3。
 *
 * <p>⚠️ 刻意<b>不帶</b> {@code category}：它恆為 {@code RESPONSIBILITY_UNRESOLVED}，與 routing key
 * 同義，放進 payload 只會多一個可能與 routing key 不一致的來源。
 *
 * <p>⚠️ 刻意<b>不帶</b> {@code notifyBuyerAccountId}／{@code notifySellerAccountId}：那兩欄需要
 * 派送時 enrich 反解析，而本事件必須在業務交易內寫 outbox（交易內零 gRPC）。N16 同樣沒有它們。
 *
 * <p>🔴 {@code shipmentId} 是 nullable {@code String}（TSID 十進位字串）——理由與 internal 型別的邊界轉換見 {@link FulfillmentResponsibilityFinalizedPayload}（同一個決定）。
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
@JsonIgnoreProperties(ignoreUnknown = true)
public record FulfillmentResponsibilityUnresolvedPayload(
        String eventId,
        String eventType,
        String eventVersion,
        /** 🔴 建案的<b>業務時間</b>（來源端 {@code occurred_at}），不得用發送時間。 */
        String occurredAt,
        String traceId,
        String correlationId,
        String orderNo,
        /**
         * TSID 的十進位字串；⚠️ 可為 {@code null}。
         * 🔴 wire 上必須是 <b>quoted string 或 null</b>，不得是 JSON 數字。
         */
        String shipmentId,
        String shippingMethod,
        /** fulfillment leg 識別 ＝ 責任判定案件的 {@code subject_ref}。 */
        String providerDeliveryRef,
        /** 責任案件編號；notification 的 {@code reference_id} 必須含它，否則會被 dedup index 靜默吞掉。 */
        String caseNo,
        /**
         * 🔴 恆為 {@code false}。與 N16 的同名欄對稱——讓 consumer 可以<b>斷言</b>「這一則不是終局」，
         * 而不是靠 routing key 的紀律。
         */
        Boolean financiallyFinal
) {
}

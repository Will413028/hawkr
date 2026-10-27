package io.github.will413028.hawkr.payment.logistics.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCostFactJoinService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.Optional;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

/**
 * {@code N16 fulfillment.responsibility.finalized} 的 <b>payment-service（Logistics Finance）</b>
 * consumer（36 §3.3.1／70 §3.1.1／§6.3）。
 *
 * <p>🔴 <b>payment-service 同時是這條 key 的 producer 與 consumer，而那不是迴圈</b>：
 * producer 側是 {@code ResponsibilityCaseService.finalizeResponsibility}（責任案件 domain，
 * 寫 {@code fulfillment_exception_cases}）；consumer 側是本類別（物流財務 domain，
 * 讀 {@code logistics_receivables}）。兩者之間刻意<b>不做同進程直呼</b>——
 * <ul>
 *   <li>直呼會把財務側的失敗拖進責任定案的交易，讓「ADM 按下核准」因為一筆應收對不上而 500</li>
 *   <li>而 36 §3.3.1 的 atomic boundary 明訂那個交易裡只能有 ⓪①②③ 四件事</li>
 * </ul>
 * 走 MQ 讓兩邊各自重試、各自進自己的 DLQ。
 *
 * <h2>本 consumer 做什麼、不做什麼</h2>
 *
 * <b>做</b>：把「settlement counterparty 已 resolved」這個結論交給
 * {@link LogisticsCostFactJoinService#converge} —— <b>兩個獨立事實的唯一收斂點</b>。
 *
 * <p>🔴 <b>本事件只帶來 70 §3.1.1 連言的右半</b>（counterparty resolved）；
 * 左半（provider cost incurred）的權威金額在 shipping-service，載體是
 * {@code logistics.cost.incurred}，成本側落在 {@code logistics_cost_facts}（V16）。
 *
 * <p>⚠️ <b>因此「本 consumer 會不會建立應收」取決於成本事實到了沒</b>：
 * <ul>
 *   <li>成本事實<b>已在</b> 且 {@code bearerOf(cause) == SELLER}
 *       ⇒ 這一次 convergence <b>就會建立應收</b>（後到的一方觸發，抵達順序不影響結果）</li>
 *   <li>成本事實<b>還沒到</b> ⇒ 只落責任側，回 {@code AWAITING_COST_FACT}，不建立任何列</li>
 * </ul>
 *
 * <p>🔴 <b>不做</b>：
 * <ul>
 *   <li><b>不自行估算成本、不持有第二套建立邏輯</b>——金額一律取自成本事實，
 *       而「兩個事實齊備 ⇒ 建立應收」只有 {@code converge} 一個實作。
 *       ⚠️ 本段早期寫的是「<b>不建立應收</b>」，那在 durable join（Batch A-R1）之後
 *       已<b>不精確</b>：它不建立的是「自己另開一條路徑」，不是「應收」本身。
 *       理由全文見 {@link LogisticsCostFactJoinService} 的 class javadoc。</li>
 *   <li><b>不推進訂單狀態、不建撥款任務</b>——那是 order-service 那條 queue 的事
 *       （{@code order.fulfillment.responsibility_finalized}）。兩條 queue 對同一件事做
 *       兩件不同的副作用，是刻意的。</li>
 *   <li><b>不發起退款</b>——非 {@code BUYER_CAUSED} 的全額退款走既有 AfterSales closure，
 *       在這裡再開一條就是第二套退款引擎。</li>
 * </ul>
 *
 * <h2>🔴 fail-loud：與 order-service 那支<b>完全相同</b>的三道守衛</h2>
 *
 * 缺欄位／未知成因／{@code financially_final != true} ⇒ 一律進 DLQ。
 * <p>⚠️ <b>兩個 consumer 各寫一份守衛不是重複</b>：它們各自獨立訂閱，其中一個被改壞時
 * 另一個仍然成立。而且兩邊的「吞掉」後果不同——order 側是訂單卡 35，
 * 這一側是<b>錢收到錯的人身上</b>或<b>該收的沒收</b>。
 */
@Component
public class ResponsibilityFinalizedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ResponsibilityFinalizedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final LogisticsCostFactJoinService join;

    public ResponsibilityFinalizedConsumer(LogisticsCostFactJoinService join) {
        this.join = join;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_FULFILLMENT_RESPONSIBILITY_FINALIZED,
            containerFactory = "manualAckContainerFactory")
    public void onResponsibilityFinalized(org.springframework.amqp.core.Message message,
                                          Channel channel,
                                          @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag)
            throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        try {
            JsonNode payload = MAPPER.readTree(body);

            // wire 是 snake_case（producer record 帶 @JsonNaming(SnakeCaseStrategy)）。
            String causeValue = payload.path("responsibility_cause").asText("");
            JsonNode finalFlag = payload.path("financially_final");
            // 🔴 providerDeliveryRef 可為 null（沒有 provider leg）——那是合法的，
            //    代表這條 leg 從來沒有產生 provider 成本 ⇒ §3.1.1 條件 ① 不成立。
            //    不得因為它為空就 fail-loud，那會把一整類正常事件推進 DLQ。
            String legRef = payload.path("provider_delivery_ref").asText("");
            String caseNo = payload.path("case_no").asText("");

            if (!finalFlag.isBoolean() || !finalFlag.booleanValue()) {
                throw new IllegalArgumentException(
                        "N16 的 financially_final 不是 true（收到 " + finalFlag + "）——"
                                + "counterparty 在判定 final 之前並未 resolved，"
                                + "此時動任何一筆應收都是在猜");
            }
            // 🔴 仍然要 parse：未知成因必須在這裡 fail-loud，而不是等 converge 讀 DB 時
            //    才發現。事件帶了不存在的成因代表 producer 契約壞了，那不是「稍後再說」。
            ResponsibilityCause cause = parseCause(causeValue);

            // 🔴 走**收斂點**而不是責任側的私有邏輯（70 §3.1.4）：
            //    本事件只是兩個獨立事實的其中一個，判斷一律交給 converge。
            //    ⚠️ 成本事實可能**已經先到**——此時這一次呼叫就會建立應收；
            //    也可能還沒到，那就 durably wait（案件列已是 RESOLVED，成本側到達時會回頭查它）。
            //    兩條路徑呼叫同一個方法，是「抵達順序不影響最終結果」的機械形式。
            LogisticsCostFactJoinService.Outcome outcome =
                    join.converge(legRef.isBlank() ? null : legRef);

            if (outcome == LogisticsCostFactJoinService.Outcome.CONFLICT) {
                // 🔴 §27：任何 financial inconsistency 不得 log.warn + ack 吞掉。
                //    進 DLQ 讓它在 payment-service 自己的 DLX 上被看見。
                throw new IllegalStateException(
                        "N16 與既有物流應收牴觸，已拒絕處置（詳見前一行 error log）"
                                + " caseNo=" + caseNo + " providerDeliveryRef=" + legRef
                                + " cause=" + cause.value());
            }

            log.info("ResponsibilityFinalizedConsumer.done",
                    "物流財務側已套用責任判定 outcome=" + outcome
                            + " caseNo=" + caseNo + " ref=" + legRef);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("ResponsibilityFinalizedConsumer.error",
                    "處理責任判定 final 事件失敗（物流財務側），進 DLQ", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "PaymentResponsibilityFinalizedConsumer", e);
        }
    }

    /**
     * 🔴 fail-loud 解析成因。
     *
     * <p>{@code RESPONSIBILITY_UNRESOLVED} 不在 {@link ResponsibilityCause} 值域內
     * （它是 {@code FulfillmentExceptionCategory} 的一格）⇒ 它出現在本事件時必然落到這裡。
     * {@code SYSTEM_CAUSED} 同理——它<b>不存在</b>，平台／系統原因一律歸 {@code PLATFORM_CAUSED}。
     *
     * <p>⚠️ 用 {@code Optional} 而不是讓 {@code valueOf} 丟 {@code IllegalArgumentException}：
     * 後者的訊息是 JVM 產的，讀的人看不出「這個值為什麼不合法」。
     */
    private static ResponsibilityCause parseCause(String value) {
        return Optional.ofNullable(value)
                .flatMap(v -> Arrays.stream(ResponsibilityCause.values())
                        .filter(c -> c.value().equals(v))
                        .findFirst())
                .orElseThrow(() -> new IllegalArgumentException(
                        "N16 帶了未知的 responsibility_cause='" + value + "'（合法值 "
                                + Arrays.stream(ResponsibilityCause.values())
                                        .map(ResponsibilityCause::value).toList()
                                + "）。🔴 RESPONSIBILITY_UNRESOLVED 是案件 category 不是成因——"
                                + "未 final 就不該發本事件；SYSTEM_CAUSED 不存在，"
                                + "平台／系統原因一律歸 PLATFORM_CAUSED"));
    }
}
